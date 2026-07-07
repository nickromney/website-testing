#!/usr/bin/env bash
set -euo pipefail

HOOKS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC2034 # Sourced hook scripts consume this shared root.
HOOKS_REPO_ROOT="$(cd "${HOOKS_DIR}/../.." && pwd)"

hook_skip_requested() {
  [[ "${WEBSITE_TESTING_SKIP_HOOKS:-}" == "1" ]]
}

hook_print_skip_and_exit() {
  echo "WARN WEBSITE_TESTING_SKIP_HOOKS=1; skipping ${0##*/}"
  exit 0
}

hook_ok() {
  echo "OK   $*"
}

hook_warn() {
  echo "WARN $*"
}

hook_fail() {
  echo "FAIL $*" >&2
}

hook_parse_execute_flag() {
  local dry_run_message="${HOOK_DRY_RUN_MESSAGE:-would run hook command}"

  if [[ "${1:-}" == "--execute" ]]; then
    shift
  elif [[ "${1:-}" == "--dry-run" ]]; then
    hook_warn "dry run: ${dry_run_message}"
    exit 0
  elif [[ "${1:-}" == "--" ]]; then
    shift
  elif [[ "${1:-}" == --* ]]; then
    hook_fail "unknown option: ${1}"
    exit 2
  fi

  # shellcheck disable=SC2034 # Sourced hook scripts consume this parsed argv.
  HOOK_ARGS=("$@")
}
