#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/hooks/lib.sh
source "${SCRIPT_DIR}/lib.sh"

HOOK_DRY_RUN_MESSAGE="would run pre-push local CI gate"
hook_parse_execute_flag "$@"

if hook_skip_requested; then
  hook_print_skip_and_exit
fi

if [[ "${WEBSITE_TESTING_LOCAL_CI_IN_PROGRESS:-}" == "1" ]]; then
  hook_warn "WEBSITE_TESTING_LOCAL_CI_IN_PROGRESS=1; skipping run-local-ci.sh to avoid recursive local CI"
  exit 0
fi

cd "${HOOKS_REPO_ROOT}"

cat <<'EOF'
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

Skip only when you have a reason:
  LEFTHOOK=0 git push
  WEBSITE_TESTING_SKIP_HOOKS=1 git push
  git push --no-verify
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
