export MOCK_BIN_DIR="$BATS_TEST_TMPDIR/mock_bin"

setup_mocks() {
  mkdir -p "$MOCK_BIN_DIR"
  export PATH="$MOCK_BIN_DIR:$PATH"

  export MOCK_CALLS_DIR="$BATS_TEST_TMPDIR/mock_calls"
  mkdir -p "$MOCK_CALLS_DIR"
}

mock_command_with_script() {
  local cmd="$1"
  local script="$2"

  cat > "$MOCK_BIN_DIR/$cmd" << EOF
#!/usr/bin/env bash
echo "\$@" >>"$MOCK_CALLS_DIR/$cmd.calls"

$script
EOF
  chmod +x "$MOCK_BIN_DIR/$cmd"
}

mock_passthrough_command() {
  local cmd="$1"
  local target="$2"

  mock_command_with_script "$cmd" "exec \"$target\" \"\$@\""
}

assert_mock_called() {
  local cmd="$1"
  local expected_args="${2:-}"

  if [[ ! -f "$MOCK_CALLS_DIR/$cmd.calls" ]]; then
    echo "Mock command '$cmd' was not called" >&2
    return 1
  fi

  if [[ -n "$expected_args" ]]; then
    if ! grep -qF -- "$expected_args" "$MOCK_CALLS_DIR/$cmd.calls"; then
      echo "Mock command '$cmd' was not called with expected args: $expected_args" >&2
      echo "Actual calls:" >&2
      cat "$MOCK_CALLS_DIR/$cmd.calls" >&2
      return 1
    fi
  fi
}

assert_mock_not_called() {
  local cmd="$1"

  if [[ -f "$MOCK_CALLS_DIR/$cmd.calls" ]]; then
    echo "Mock command '$cmd' was called but should not have been" >&2
    echo "Calls:" >&2
    cat "$MOCK_CALLS_DIR/$cmd.calls" >&2
    return 1
  fi
}

clear_mock_calls() {
  local cmd="$1"
  rm -f "$MOCK_CALLS_DIR/$cmd.calls"
}

teardown_mocks() {
  export PATH="${PATH#"$MOCK_BIN_DIR:"}"
  rm -rf "$MOCK_BIN_DIR" "$MOCK_CALLS_DIR"
}
