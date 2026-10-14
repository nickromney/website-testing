#!/usr/bin/env bash
# Verify the smoke CLI against a local fixture. Bash 3.2 compatible.
# shellcheck disable=SC2034 # VK_* settings are read by the sourced verify-lib.sh
set -euo pipefail

VK_SCRIPT="$0"
VK_APP="website-testing"
VK_DESCRIPTION="Verification harness for the smoke CLI using an isolated local HTTP fixture."
# shellcheck source=verify-lib.sh disable=SC1091
. "$(cd "$(dirname "$0")" && pwd)/verify-lib.sh"
VK_PORT=38679

vk_commands_help() {
  cat <<'HELP'
  launch                 build smoke and start the local HTTP fixture
  doctor                 confirm the owned fixture answers on localhost
  quick-check            exercise CLI status, body and header assertions
  cleanup                stop the owned fixture and remove run state; keep evidence
HELP
}
vk_sample_commands() { printf '%s\n' verify launch doctor quick-check cleanup; }
vk_paths() {
  STATE="$VK_BASE_DIR/$VK_RUN_ID/state"
  EVIDENCE="$VK_EVIDENCE_ROOT/$VK_RUN_ID"
  APP_BIN="$STATE/smoke"
  FIXTURE_PID="$STATE/fixture.pid"
  FIXTURE_LOG="$EVIDENCE/fixture.log"
  HEALTH_URL="http://127.0.0.1:$VK_PORT/health"
}
vk_plan() {
  case "$1" in
    verify) vk_plan_line "build smoke, start owned localhost fixture on port $VK_PORT, assert HTTP status/body/header, stop fixture; evidence in $EVIDENCE" ;;
    launch) vk_plan_line "build cmd/smoke-go into $APP_BIN and start local fixture on port $VK_PORT" ;;
    doctor) vk_plan_line "check that the owned fixture serves $HEALTH_URL" ;;
    quick-check) vk_plan_line "run smoke quick mode against $HEALTH_URL and assert JSON pass, status, body and header" ;;
    cleanup) vk_plan_line "stop only the fixture process group owned by $FIXTURE_PID; remove $STATE; keep $EVIDENCE" ;;
    *) vk_unknown_command ;;
  esac
}
cmd_launch() {
  mkdir -p "$STATE" "$EVIDENCE"
  vk_mark_current
  vk_record_revision "$EVIDENCE/revision.txt"
  (cd "$VK_ROOT" && GOMAXPROCS=2 GOCACHE="$STATE/go-cache" go build -trimpath -o "$APP_BIN" ./cmd/smoke-go)
  if vk_port_busy "$VK_PORT"; then vk_fail "port $VK_PORT is already in use; refusing to touch its listener"; return 1; fi
  vk_spawn_group "$FIXTURE_PID" "$FIXTURE_LOG" python3 "$VK_SKILL_DIR/scripts/fixture_server.py" "$VK_PORT"
  if ! vk_wait_for_alive "$FIXTURE_PID" 10 curl -fsS "$HEALTH_URL" -o "$EVIDENCE/fixture-health.txt"; then
    vk_fail "local fixture did not become healthy; inspect $FIXTURE_LOG"
    return 1
  fi
  vk_say "built smoke and started owned localhost fixture at $HEALTH_URL"
}
cmd_doctor() {
  if [ ! -f "$FIXTURE_PID" ] || ! vk_group_alive "$FIXTURE_PID"; then
    vk_fail "owned fixture is not running; launch first"
    return 1
  fi
  curl -fsS "$HEALTH_URL" > "$EVIDENCE/doctor-response.txt"
  vk_expect_contains "$EVIDENCE/doctor-response.txt" 'smoke fixture healthy'
}
cmd_quick_check() {
  [ -x "$APP_BIN" ] || { vk_fail "smoke CLI is not built; launch first"; return 1; }
  "$APP_BIN" --json "$HEALTH_URL" --status 200 \
    --body-contains 'smoke fixture healthy' \
    --header-contains 'Content-Type: text/plain' > "$EVIDENCE/quick-check.json"
  vk_expect_contains "$EVIDENCE/quick-check.json" '"passed": true'
  vk_expect_contains "$EVIDENCE/quick-check.json" '"status_code": 200'
  vk_expect_contains "$EVIDENCE/quick-check.json" 'smoke fixture healthy'
}
cmd_cleanup() {
  if [ -f "$FIXTURE_PID" ]; then
    if vk_group_alive "$FIXTURE_PID"; then
      vk_stop_group "$FIXTURE_PID"
    fi
  fi
  rm -rf "${STATE:?}"
  rmdir "$VK_BASE_DIR/$VK_RUN_ID" 2>/dev/null || true
  vk_clear_current
  vk_say "stopped owned fixture and removed run state; evidence kept at $EVIDENCE"
}
vk_verify_recipes() { cmd_quick_check; }
vk_dispatch() {
  local cmd="$1"
  shift
  case "$cmd" in
    launch) cmd_launch ;;
    doctor) cmd_doctor ;;
    quick-check) cmd_quick_check ;;
    cleanup) cmd_cleanup ;;
    *) vk_unknown_command ;;
  esac
}
vk_main "$@"
