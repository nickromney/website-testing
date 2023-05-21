#!/bin/bash
SMOKE_TMP_DIR=$(mktemp -d)

SMOKE_AFTER_RESPONSE=""

SMOKE_CURL_CODE="$SMOKE_TMP_DIR/smoke_curl_code"
SMOKE_CURL_HEADERS="$SMOKE_TMP_DIR/smoke_curl_headers"
SMOKE_CURL_BODY="$SMOKE_TMP_DIR/smoke_curl_body"
SMOKE_CURL_COOKIE_JAR="$SMOKE_TMP_DIR/smoke_curl_cookie_jar"

SMOKE_CSRF_TOKEN=""
SMOKE_CSRF_FORM_DATA="$SMOKE_TMP_DIR/smoke_csrf_form_data"

#SMOKE_DIG_GLOBAL_SERVER="8.8.8.8"
SMOKE_DIG_GLOBAL_SERVER="8.8.4.4"
#SMOKE_DIG_GLOBAL_SERVER="1.1.1.1"
SMOKE_DIG_RESULTS="$SMOKE_TMP_DIR/smoke_dig_results"

SMOKE_HEADERS=()
SMOKE_SSL_EXPIRY="$SMOKE_TMP_DIR/smoke_ssl_expiry"
SMOKE_SSL_EXPIRY_ALERT_DAYS=35
SMOKE_TESTS_FAILED=0
SMOKE_TESTS_RUN=0
SMOKE_URL_PREFIX=""

## "Public API"

remove_smoke_headers() {
    unset SMOKE_HEADERS
}

smoke_csrf() {
    SMOKE_CSRF_TOKEN="$1"
}

smoke_dig_domain() {
    DOMAIN="$1"
    SMOKE_DIG_Q_TYPE="$2"

    _dig_domain "$DOMAIN" "$SMOKE_DIG_Q_TYPE"
}

smoke_form() {
    URL="$1"
    FORMDATA="$2"

    if [[ ! -f "$FORMDATA" ]]; then
        _smoke_print_failure "No formdata file"
        _smoke_cleanup
        exit 1
    fi

    _curl_post "$URL" "$FORMDATA"
}

smoke_form_ok() {
    URL="$1"
    FORMDATA="$2"

    smoke_form "$URL" "$FORMDATA"
    smoke_assert_code_ok
}

smoke_header() {
    SMOKE_HEADERS+=("$1")
}

smoke_host() {
    smoke_header "Host: $1"
}

smoke_report() {
    _smoke_cleanup
    if [[ $SMOKE_TESTS_FAILED -ne 0 ]]; then
        _smoke_print_report_failure "FAIL ($SMOKE_TESTS_FAILED/$SMOKE_TESTS_RUN)"
        exit 1
    fi
    _smoke_print_report_success "OK ($SMOKE_TESTS_RUN/$SMOKE_TESTS_RUN)"
}

smoke_response_body() {
    cat "$SMOKE_CURL_BODY"
}

smoke_response_code() {
    cat "$SMOKE_CURL_CODE"
}

smoke_response_dig() {
    cat "$SMOKE_DIG_RESULTS"
}

smoke_response_headers() {
    cat "$SMOKE_CURL_HEADERS"
}

smoke_response_ssl_expiry() {
    cat "$SMOKE_SSL_EXPIRY"
}

smoke_ssl_expiry() {
    DOMAIN="$1"
    _ssl_expiry "$DOMAIN"
}

smoke_tcp_ok() {
    URL="$1 $2"
    _smoke_print_url "$URL"
    echo EOF | telnet "$URL" > "$SMOKE_CURL_BODY"
    smoke_assert_body "Connected"
}

smoke_url() {
    URL="$1"
    _curl_get "$URL"
}

smoke_url_ok() {
    URL="$1"
    smoke_url "$URL"
    smoke_assert_code_ok
}

smoke_url_prefix() {
    SMOKE_URL_PREFIX="$1"
}

## Assertions

smoke_assert_body() {
    STRING="$1"

    if smoke_response_body | grep --quiet "$STRING"; then
        _smoke_success "Body contains \"$STRING\""
    else
        _smoke_fail "Body does not contain \"$STRING\""
    fi
}

smoke_assert_body_absent() {
    STRING="$1"

    if smoke_response_body | grep --quiet "$STRING"; then
        _smoke_fail "(Assert absence of string): Body contains \"$STRING\""
    else
        _smoke_success "(Assert absence of string): Body does not contain \"$STRING\""
    fi
}

smoke_assert_code() {
    CODE=$(cat "$SMOKE_CURL_CODE")

    if [[ $CODE == "$1" ]]; then
        _smoke_success "$1 Response code"
    else
        _smoke_fail "$1 Response code"
    fi
}

smoke_assert_code_ok() {
    CODE=$(cat "$SMOKE_CURL_CODE")

    if [[ $CODE == 2* ]]; then
        _smoke_success "2xx Response code"
    else
        _smoke_fail "2xx Response code"
    fi
}

smoke_assert_dig() {
    STRING="$1"

    if smoke_response_dig | grep --quiet "$STRING"; then
        _smoke_success "Dig results contain \"$STRING\""
    else
        _smoke_fail "(Assert presence of string): Dig results do not contain \"$STRING\""
        smoke_response_dig
    fi
}

smoke_assert_dig_absent() {
    STRING="$1"

    if smoke_response_dig | grep --quiet "$STRING"; then
        _smoke_fail "(Assert absence of string): Dig results contain \"$STRING\""
        smoke_response_dig
    else
        _smoke_success "(Assert absence of string): Dig results do not contain \"$STRING\""
    fi
}

smoke_assert_ssl_expiry() {
    EXPIRY=$(smoke_response_ssl_expiry)
	EXPIRY_SIMPLE=$( date -d "$EXPIRY" +%F )
	EXPIRY_SEC=$(date -d "$EXPIRY" +%s)
	TODAY_SEC=$(date +%s)
	EXPIRY_CALC=$(echo "($EXPIRY_SEC-$TODAY_SEC)/86400" | bc )
	# Output
	if [ "$EXPIRY_CALC" -gt "$SMOKE_SSL_EXPIRY_ALERT_DAYS" ] ; then
        _smoke_success "$EXPIRY_SIMPLE - $DOMAIN certificate valid for $EXPIRY_CALC days"
    else
        _smoke_fail "$EXPIRY_SIMPLE - $DOMAIN certificate expires in $EXPIRY_CALC days."
    fi
}

smoke_assert_headers() {
    STRING="$1"

    if smoke_response_headers | grep --quiet "$STRING"; then
        _smoke_success "Headers contain \"$STRING\""
    else
        _smoke_fail "Headers do not contain \"$STRING\""
    fi
}

## Smoke "private" functions

_smoke_after_response() {
    $SMOKE_AFTER_RESPONSE
}

_smoke_cleanup() {
    rm -rf "$SMOKE_TMP_DIR"
}

_smoke_fail() {
    REASON="$1"
    (( ++SMOKE_TESTS_FAILED ))
    (( ++SMOKE_TESTS_RUN ))
    _smoke_print_failure "$REASON"
}

_smoke_prepare_formdata() {
    FORMDATA="$1"

    if [[ "" != "$SMOKE_CSRF_TOKEN" ]]; then
        < "$FORMDATA" sed "s/__SMOKE_CSRF_TOKEN__/$SMOKE_CSRF_TOKEN/" > "$SMOKE_CSRF_FORM_DATA"
        echo "$SMOKE_CSRF_FORM_DATA"
    else
        echo "$FORMDATA"
    fi
}

_smoke_success() {
    REASON="$1"
    _smoke_print_success "$REASON"
    (( ++SMOKE_TESTS_RUN ))
}

## Curl helpers
_curl() {
  local opt=(--cookie "$SMOKE_CURL_COOKIE_JAR" --cookie-jar "$SMOKE_CURL_COOKIE_JAR" --location --dump-header "$SMOKE_CURL_HEADERS" --silent)

  if (( ${#SMOKE_HEADERS[@]} )); then
    for header in "${SMOKE_HEADERS[@]}"
    do
        opt+=(-H "$header")
    done
  fi

  curl "${opt[@]}" "$@" > "$SMOKE_CURL_BODY"
}

_curl_get() {
    URL="$1"

    SMOKE_URL="$SMOKE_URL_PREFIX$URL"
    _smoke_print_url "$SMOKE_URL"

    _curl "$SMOKE_URL"

    grep -oE 'HTTP[^ ]+ [0-9]{3}' "$SMOKE_CURL_HEADERS" | tail -n1 | grep -oE '[0-9]{3}' > "$SMOKE_CURL_CODE"

    $SMOKE_AFTER_RESPONSE
}

_curl_post() {
    URL="$1"
    FORMDATA="$2"
    FORMDATA_FILE="@"$(_smoke_prepare_formdata "$FORMDATA")

    SMOKE_URL="$SMOKE_URL_PREFIX$URL"
    _smoke_print_url "$SMOKE_URL"

    _curl --data "$FORMDATA_FILE" "$SMOKE_URL"

    grep -oE 'HTTP[^ ]+ [0-9]{3}' "$SMOKE_CURL_HEADERS" | tail -n1 | grep -oE '[0-9]{3}' > "$SMOKE_CURL_CODE"

    $SMOKE_AFTER_RESPONSE
}

## Dig helpers
_dig_domain() {
    DOMAIN="$1"
    SMOKE_DIG_Q_TYPE="$2"

    _smoke_print_url "$DOMAIN"

    dig @"$SMOKE_DIG_GLOBAL_SERVER" "$DOMAIN" "$SMOKE_DIG_Q_TYPE" +short > "$SMOKE_DIG_RESULTS"

    $SMOKE_AFTER_RESPONSE
}

## SSL helpers
_ssl_expiry() {
    DOMAIN="$1"
    _smoke_print_url "$DOMAIN"
	echo | openssl s_client -servername "$DOMAIN" -connect "$DOMAIN":443 2>/dev/null | openssl x509 -noout -dates | grep notAfter | sed 's/notAfter=//' > "$SMOKE_SSL_EXPIRY"
}

## Print helpers

# test for color support, inspired by:
# http://unix.stackexchange.com/questions/9957/how-to-check-if-bash-can-print-colors
if [ -t 1 ]; then
    ncolors=$(tput colors)
    if test -n "$ncolors" && test "$ncolors" -ge 8; then
        bold="$(tput bold)"
        normal="$(tput sgr0)"
        red="$(tput setaf 1)"
        redbg="$(tput setab 1)"
        green="$(tput setaf 2)"
        greenbg="$(tput setab 2)"
    fi
fi

_smoke_print_failure() {
    TEXT="$1"
    echo "    [${red}${bold}FAIL${normal}] $TEXT"
}

_smoke_print_report_failure() {
    TEXT="$1"
    echo -e "${redbg}$TEXT${normal}"
}

_smoke_print_report_success() {
    TEXT="$1"
    echo -e "${greenbg}$TEXT${normal}"
}

_smoke_print_success() {
    TEXT="$1"
    echo "    [ ${green}${bold}OK${normal} ] $TEXT"
}

_smoke_print_url() {
    TEXT="$1"
    echo "> $TEXT"
}
