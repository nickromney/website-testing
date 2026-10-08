# website-testing

This repository now has two layers:

- `smoke`: the active Go CLI/TUI experiment for HTTP smoke checks.
- `smoke.sh`: the original Bash helper library kept for legacy context and parity work.

The Go path is the primary direction. The Bash scripts still ship, still have test coverage, and still document the historical API, but they are no longer the center of the repo.

## smoke

`smoke` is the modern runner in this repository. It aims to make website smoke tests easier to run locally, easier to package, and easier to evolve into a richer TUI-driven workflow.

Current direction:

- YAML-driven test specs
- shell-style ad hoc HTTP checks with flags
- HTTP, DNS, TLS, and TCP step kinds
- Script-friendly CLI execution
- Bubble Tea TUI for multi-step spec editing, YAML preview, save, and runs
- GoReleaser-based binary packaging

### Install

Tagged releases are intended to publish binaries to GitHub Releases via GoReleaser. Until a release is cut, build from source.

### Quickstart

Build locally:

```bash
make build
./bin/smoke --help
```

On macOS, an unsigned downloaded binary may be quarantined on first launch. If that happens, remove the quarantine xattr and retry:

```bash
xattr -d com.apple.quarantine ./smoke
./smoke --help
```

### Build From Source

```bash
make build
./bin/smoke --help
```

### Run

Current command surface:

```bash
./bin/smoke google.com
./bin/smoke https://example.com/health --status 200 --body-contains Example
./bin/smoke http://localhost:3000 --header "Host: example.test"
./bin/smoke run path/to/spec.yaml
./bin/smoke run path/to/spec.yaml --json
./bin/smoke tui
./bin/smoke tui path/to/spec.yaml
./bin/smoke version
```

Quick CLI mode is intended to feel more like the original Bash script: pass a target and optional assertion flags, and it performs a default HTTP smoke check without requiring YAML first.

`./bin/smoke tui` now opens a multi-step builder. It can compose HTTP, DNS, TLS, and TCP steps, shows a live YAML preview, saves to a path you choose with `s`, runs the generated spec with `r`, and lets you return from the run view back to the builder with `b`.

Historical repo checks still map directly into quick mode:

```bash
./bin/smoke google.com
./bin/smoke https://www.theregister.com/security --body-absent "Sorry, this page doesn't exist!"
```

### Sample Spec

Checked-in examples:

- `examples/http.yaml` for HTTP-only checks
- `examples/network.yaml` for mixed HTTP/DNS/TLS/TCP checks
- `examples/smoke-google.yaml` mirroring `smoke-google`
- `examples/smoke-theregister.yaml` mirroring `smoke-theregister`
- `examples/smoke-dig.yaml` mirroring `smoke-dig`
- `examples/smoke-ssl.yaml` mirroring `smoke-ssl`

Supported step kinds:

- `kind: http` with `request` and HTTP expectations
- `kind: dns` with `dns.name`, `dns.type`, optional `dns.server`, and `expect.answer_contains`
- `kind: tls` with `tls.address`, optional `tls.server_name`, and `expect.days_remaining_at_least`
- `kind: tcp` with `tcp.address`

Inline mixed example:

```yaml
steps:
  - name: homepage
    kind: http
    request:
      method: GET
      url: https://example.org/
      headers:
        User-Agent: smoke
    expect:
      status: 200
      body_contains:
        - Example Domain
      body_absent:
        - Exception
      header_contains:
        - Content-Type: text/html

  - name: dns lookup
    kind: dns
    dns:
      name: example.org
      type: A

  - name: tls expiry
    kind: tls
    tls:
      address: example.org:443
    expect:
      days_remaining_at_least: 7

  - name: tcp connect
    kind: tcp
    tcp:
      address: example.org:443
```

### Development

```bash
make test
make vet
make vuln
make precommit
```

### Local Validation

Install local hooks:

```bash
make hooks
# or: lefthook install
```

The pre-commit hook runs fast staged-file checks. The pre-push hook runs the same validation that CI used to run automatically.

Skip hooks only when needed:

```bash
LEFTHOOK=0 git commit ...
LEFTHOOK=0 git push
git commit --no-verify
git push --no-verify
```

The full gate runs locally on pre-push through `scripts/hooks/run-local-ci.sh` (lefthook). To run it directly:

```bash
uv run --locked scripts/hooks/run-local-ci.sh --execute
```

Release is still a GitHub Actions workflow:

```bash
gh workflow run release.yml
```

## Legacy Bash Library

The original Bash implementation is still here if you need it:

- `smoke.sh`
- `smoke-google`
- `smoke-dig`
- `smoke-ssl`
- `smoke-theregister`

Typical usage still looks like this:

```bash
#!/bin/bash

. smoke.sh

smoke_url_ok "https://example.org/"
  smoke_assert_body "Example Domain"

smoke_report
```

The Bash helper still supports:

- response code checks
- body contains / absent checks
- header checks
- GET and POST flows
- DNS checks
- SSL expiry checks
- CSRF token substitution

If you are touching the Bash path, keep it working. If you are investing in new product direction, do it in `smoke`.

## Agent operation and plan status

For the current ownership, action-effect and evidence contracts, use [the operating model](docs/agent-system.md). Its implemented plan covers agent navigation and documentation. Feature proposals below remain proposals until their own acceptance evidence is recorded; dated observations retain their original scope.

## Offline and live result acceptance

| Boundary | Command / owner | Claim |
| --- | --- | --- |
| Spec validation | `internal/spec` tests | YAML grammar and field semantics before effects |
| Offline runner contract | `internal/runner/offline_contract_test.go`; `make test-go` | Local deterministic HTTP/DNS/TLS/TCP behavior |
| Legacy API parity | `make test-bash`; `_test` | Promised Bash helpers against fixtures |
| Live selected spec | `./bin/smoke run path/to/spec.yaml --json` | Each assertion against its effective target at run time |

An evidence record carries spec path/digest, binary/source revision, effective
target, timestamp and per-step result. Invalid specs and local prerequisites
remain distinct from DNS/TLS/connectivity/assertion failures. The current JSON
output is the implementation contract; this document does not add fields to it.
Record extra provenance alongside output rather than invent CLI flags. Checked-in
internet examples are attended live probes, not an offline test gate. A pass
proves the selected assertion only and should not become a generic site-readiness
claim. Preserve newly confirmed protocol cases in the existing runner fixtures.
