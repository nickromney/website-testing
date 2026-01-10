#!/usr/bin/env bash
set -euo pipefail

if ! command -v bats >/dev/null 2>&1; then
  echo "Error: bats is not installed" >&2
  echo "Install: brew install bats-core" >&2
  exit 1
fi

cd "$(dirname "${BASH_SOURCE[0]}")/.."

bats -r _test
