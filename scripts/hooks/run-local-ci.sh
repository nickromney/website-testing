#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/hooks/lib.sh
source "${SCRIPT_DIR}/lib.sh"

HOOK_DRY_RUN_MESSAGE="would run pre-push local CI gate"
hook_parse_execute_flag "$@"

if hook_skip_requested; then
  hook_fail "skip_requested: verification did not execute"
  exit 1
fi

if [[ "${WEBSITE_TESTING_LOCAL_CI_IN_PROGRESS:-}" == "1" ]]; then
  hook_fail "recursive_gate: verification did not execute"
  exit 1
fi

cd "${HOOKS_REPO_ROOT}"

# Resolve only installed toolchains during verification.
export GOTOOLCHAIN=local
if [[ "$(go env GOVERSION)" != "go1.26.9" ]]; then
  hook_fail "Use the reviewed Go 1.26.9 pin in .mise.toml (mise exec -- go ...); local gates never fetch a toolchain"
  exit 1
fi

cat << 'EOF'
website-testing pre-push local CI gate

Running:
  make precommit
  make test-bash
  go test ./...
  go test -race ./...
  make test-cover
  make vet
  make vuln
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./cmd/smoke-go
  CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./cmd/smoke-go
  make lint

Full acceptance requires every configured check.
Explicit skip and recursive execution requests refuse verification.
EOF

export WEBSITE_TESTING_LOCAL_CI_IN_PROGRESS=1
export LEFTHOOK=0

commands=(
  "make precommit"
  "make test-bash"
  "go test ./..."
  "go test -race ./..."
  "make test-cover"
  "make vet"
  "make vuln"
  "CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./cmd/smoke-go"
  "CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./cmd/smoke-go"
  "make lint"
)

for command in "${commands[@]}"; do
  if ! bash -c "${command}"; then
    hook_fail "pre-push gate failed: ${command}"
    exit 1
  fi
done

hook_ok "pre-push gate passed"
