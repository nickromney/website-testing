#!/usr/bin/env bats
# Full-gate refusals must not run any producer or masquerade as executed checks.
setup() {
  GATE_ROOT="$(git -C "$BATS_TEST_DIRNAME" rev-parse --show-toplevel)"
  GATE="$GATE_ROOT/scripts/hooks/run-local-ci.sh"
  export PRODUCER_MARKER="$BATS_TEST_TMPDIR/producer-called"
  mkdir -p "$BATS_TEST_TMPDIR/bin"
  for producer in make npm tofu uv go checkov yamllint shellcheck; do
    cat > "$BATS_TEST_TMPDIR/bin/$producer" << 'MOCK'
#!/usr/bin/env bash
echo invoked >> "$PRODUCER_MARKER"
exit 93
MOCK
    chmod +x "$BATS_TEST_TMPDIR/bin/$producer"
  done
  export PATH="$BATS_TEST_TMPDIR/bin:$PATH"
}

@test "explicit full-gate skip refuses without running producers" {
  run env WEBSITE_TESTING_SKIP_HOOKS=1 WEBSITE_TESTING_LOCAL_CI_IN_PROGRESS=0 "$GATE" --execute
  [ "$status" -ne 0 ]
  [ "${output#*skip_requested: verification did not execute}" != "$output" ]
  [ ! -e "$PRODUCER_MARKER" ]
}

@test "recursive full-gate request refuses without running producers" {
  run env WEBSITE_TESTING_SKIP_HOOKS=0 WEBSITE_TESTING_LOCAL_CI_IN_PROGRESS=1 "$GATE" --execute
  [ "$status" -ne 0 ]
  [ "${output#*recursive_gate: verification did not execute}" != "$output" ]
  [ ! -e "$PRODUCER_MARKER" ]
}

@test "explicit preview remains an observation without running producers" {
  run env WEBSITE_TESTING_SKIP_HOOKS=0 WEBSITE_TESTING_LOCAL_CI_IN_PROGRESS=0 "$GATE" --dry-run
  [ "$status" -eq 0 ]
  [ "$output" = 'WARN dry run: would run pre-push local CI gate' ]
  [ ! -e "$PRODUCER_MARKER" ]
}
