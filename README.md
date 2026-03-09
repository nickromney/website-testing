# website-testing

This repository now has two layers:

- `smoke-go`: the active Go CLI/TUI experiment for HTTP smoke checks.
- `smoke.sh`: the original Bash helper library kept for legacy context and parity work.

The Go path is the primary direction. The Bash scripts still ship, still have test coverage, and still document the historical API, but they are no longer the center of the repo.

## smoke-go

`smoke-go` is the modern runner in this repository. It aims to make website smoke tests easier to run locally, easier to package, and easier to evolve into a richer TUI-driven workflow.

Current direction:

- YAML-driven test specs
- Script-friendly CLI execution
- Bubble Tea TUI for interactive runs
- GoReleaser-based binary packaging

### Install

Tagged releases are intended to publish binaries to GitHub Releases via GoReleaser. Until a release is cut, build from source.

### Build From Source

```bash
go build -o bin/smoke-go ./cmd/smoke-go
./bin/smoke-go --help
```

### Run

Current command surface:

```bash
./bin/smoke-go run path/to/spec.yaml
./bin/smoke-go run path/to/spec.yaml --json
./bin/smoke-go tui path/to/spec.yaml
./bin/smoke-go version
```

### Sample Spec

There is a checked-in sample spec at `examples/http.yaml`. Start there, or use this inline example:

```yaml
steps:
  - name: homepage
    request:
      method: GET
      url: https://example.org/
      headers:
        User-Agent: smoke-go
    expect:
      status: 200
      body_contains:
        - Example Domain
      body_absent:
        - Exception
      header_contains:
        - Content-Type: text/html
```

### Development

```bash
go test ./...
bats -r _test
pre-commit run -a
```

The GitHub Actions checks keep both the legacy Bash surface and the Go experiment under test.

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

If you are touching the Bash path, keep it working. If you are investing in new product direction, do it in `smoke-go`.
