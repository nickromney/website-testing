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

clear_mock_calls() {
  local cmd="$1"
  rm -f "$MOCK_CALLS_DIR/$cmd.calls"
}

teardown_mocks() {
  export PATH="${PATH#"$MOCK_BIN_DIR:"}"
  rm -rf "$MOCK_BIN_DIR" "$MOCK_CALLS_DIR"
}
