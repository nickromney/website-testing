#!/usr/bin/env bats

load helpers/mocks.bash

setup() {
  setup_mocks
}

teardown() {
  if declare -F _smoke_cleanup > /dev/null 2>&1; then
    _smoke_cleanup
  fi
  teardown_mocks
}

make_curl_mock() {
  local http_headers="$1"
  local body="$2"

  mock_command_with_script "curl" "
dump_header=''
data_arg=''

while [[ \$# -gt 0 ]]; do
  case \"\$1\" in
    --dump-header)
      dump_header=\"\$2\"
      shift 2
      ;;
    --data)
      data_arg=\"\$2\"
      shift 2
      ;;
    --cookie|--cookie-jar)
      shift 2
      ;;
    --location|--silent)
      shift
      ;;
    -H)
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [[ -n \"\$dump_header\" ]]; then
  cat >\"\$dump_header\" <<'HDR'
$http_headers
HDR
fi

printf '%s\n' \"$body\"
"
}

make_dig_mock() {
  local output="$1"

  mock_command_with_script "dig" "
while [[ \$# -gt 0 ]]; do
  shift
done

printf '%s\n' \"$output\"
"
}

make_nc_mock() {
  local exit_code="$1"

  mock_command_with_script "nc" "exit $exit_code"
}

make_telnet_mock() {
  local output="$1"

  mock_command_with_script "telnet" "printf '%s\n' \"$output\""
}

@test "smoke_url_ok extracts the last HTTP status code" {
  make_curl_mock $'HTTP/1.1 302 Found\nHTTP/2 200 OK\n' "hello"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  smoke_url_ok "http://example.test/"

  run smoke_response_code
  [ "$status" -eq 0 ]
  [ "$output" -eq 200 ]
  [ "$SMOKE_TESTS_FAILED" -eq 0 ]
}

@test "smoke_header passes custom request headers to curl" {
  make_curl_mock $'HTTP/1.1 200 OK\n' "ok"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  smoke_header "X-Test: 1"
  smoke_url "http://example.test/"

  assert_mock_called "curl" "-H X-Test: 1"
}

@test "remove_smoke_headers clears previously added headers" {
  make_curl_mock $'HTTP/1.1 200 OK\n' "ok"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  smoke_header "X-Test: 1"
  remove_smoke_headers
  smoke_url "http://example.test/"

  if grep -qF -- "-H X-Test: 1" "$MOCK_CALLS_DIR/curl.calls"; then
    echo "Expected curl to be called without -H X-Test: 1" >&2
    cat "$MOCK_CALLS_DIR/curl.calls" >&2
    return 1
  fi
}

@test "smoke_form substitutes CSRF token in form data" {
  make_curl_mock $'HTTP/1.1 200 OK\n' "ok"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  formdata="$BATS_TEST_TMPDIR/form.txt"
  printf 'csrf=__SMOKE_CSRF_TOKEN__\n' > "$formdata"

  smoke_csrf "abc123"
  smoke_form "http://example.test/login" "$formdata"

  data_arg=$(awk '{for (i=1;i<=NF;i++) if ($i=="--data") print $(i+1)}' "$MOCK_CALLS_DIR/curl.calls" | tail -n 1)
  [[ "$data_arg" == @* ]]
  data_file=${data_arg#@}
  [[ -f "$data_file" ]]
  run cat "$data_file"
  [ "$status" -eq 0 ]
  [[ "$output" == *"csrf=abc123"* ]]
}

@test "smoke_assert_headers matches response headers" {
  make_curl_mock $'HTTP/1.1 200 OK\nContent-Type: text/plain\n' "ok"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  smoke_url "http://example.test/"
  smoke_assert_headers "Content-Type: text/plain"

  [ "$SMOKE_TESTS_FAILED" -eq 0 ]
}

@test "smoke_assert_body_absent passes when string is missing" {
  make_curl_mock $'HTTP/1.1 200 OK\n' "hello"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  smoke_url "http://example.test/"
  smoke_assert_body_absent "does-not-exist"

  [ "$SMOKE_TESTS_FAILED" -eq 0 ]
}

@test "smoke_dig_domain writes dig output and assertions work" {
  make_dig_mock "1.2.3.4"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  smoke_dig_domain "example.test" a
  smoke_assert_dig "1.2.3.4"
  smoke_assert_dig_absent "5.6.7.8"

  [ "$SMOKE_TESTS_FAILED" -eq 0 ]
}

@test "smoke_assert_ssl_expiry works on macOS without GNU date" {
  make_curl_mock $'HTTP/1.1 200 OK\n' "ok"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  # shellcheck disable=SC2034
  DOMAIN="example.test"

  expiry=$(
    python3 - << 'PY'
import datetime
dt = datetime.datetime.utcnow() + datetime.timedelta(days=60)
print(dt.strftime('%b %d %H:%M:%S %Y GMT'))
PY
  )

  printf '%s\n' "$expiry" > "$SMOKE_SSL_EXPIRY"
  smoke_assert_ssl_expiry

  [ "$SMOKE_TESTS_FAILED" -eq 0 ]
}

@test "smoke_report exits 1 when there are failures" {
  run bash -c 'source ./smoke.sh; SMOKE_TESTS_FAILED=1; SMOKE_TESTS_RUN=1; smoke_report'
  [ "$status" -eq 1 ]
}

@test "smoke_tcp_ok uses nc when available" {
  make_curl_mock $'HTTP/1.1 200 OK\n' "ok"
  make_nc_mock 0

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  smoke_tcp_ok "example.test" 443

  run smoke_response_body
  [ "$status" -eq 0 ]
  [[ "$output" == *"Connected"* ]]
  assert_mock_called "nc" "-z -w 5 example.test 443"
}

@test "smoke_tcp_ok falls back to telnet when nc not on PATH" {
  make_curl_mock $'HTTP/1.1 200 OK\n' "ok"
  make_telnet_mock "Connected"
  mock_passthrough_command "grep" "/usr/bin/grep"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  PATH="$MOCK_BIN_DIR:/bin"
  smoke_tcp_ok "example.test" 443

  run smoke_response_body
  [ "$status" -eq 0 ]
  [[ "$output" == *"Connected"* ]]
  assert_mock_called "telnet" "example.test 443"
  assert_mock_not_called "nc"
}

@test "SMOKE_AFTER_RESPONSE callback runs after smoke_url" {
  make_curl_mock $'HTTP/1.1 200 OK\n' "ok"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  after_called=0
  after() {
    after_called=1
  }
  # shellcheck disable=SC2034
  SMOKE_AFTER_RESPONSE=after

  smoke_url "http://example.test/"

  [ "$after_called" -eq 1 ]
}

@test "missing HTTP status in headers yields response code 000" {
  make_curl_mock $'Content-Type: text/plain\n' "ok"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  smoke_url "http://example.test/"
  run smoke_response_code
  [ "$status" -eq 0 ]
  [ "$output" -eq 0 ]
}

@test "curl is called with cookie jar options" {
  make_curl_mock $'HTTP/1.1 200 OK\n' "ok"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  smoke_url "http://example.test/"

  run cat "$MOCK_CALLS_DIR/curl.calls"
  [ "$status" -eq 0 ]
  [[ "$output" == *"--cookie "* ]]
  [[ "$output" == *"--cookie-jar "* ]]
}

@test "smoke_form fails fast when formdata file is missing" {
  run bash -c 'source ./smoke.sh; smoke_form "http://example.test/" "/no/such/file"'
  [ "$status" -eq 1 ]
  [[ "$output" == *"No formdata file"* ]]
}

@test "failing assertion increments failed count" {
  make_curl_mock $'HTTP/1.1 200 OK\n' "hello"

  source "${BATS_TEST_DIRNAME}/../smoke.sh"

  smoke_url_ok "http://example.test/"
  smoke_assert_body "does-not-exist"

  [ "$SMOKE_TESTS_FAILED" -eq 1 ]
  [ "$SMOKE_TESTS_RUN" -eq 2 ]
}
