# website-testing: agent operating model

Adopted 6 October 2026 from local source and command inspection.
Spec-driven Go HTTP/DNS/TLS/TCP smoke runner with TUI and legacy Bash parity.

## Read by intent

Start with the local agent guide and build manifest. For domain or behavior
changes, follow the owners below, then the relevant contract/test. These
documents retain product detail and historical evidence:

- [README.md](../README.md)
- [.beads/README.md](../.beads/README.md)

## System ownership

| Owner | Responsibility |
| --- | --- |
| [internal/spec](../internal/spec) | Declarative check specification and examples. |
| [examples](../examples) | Declarative check specification and examples. |
| [internal/runner](../internal/runner) | Check execution and result behavior. |
| [internal/adhoc](../internal/adhoc) | Check execution and result behavior. |
| [internal/cli](../internal/cli) | Script/TUI adapters and legacy Bash surface. |
| [internal/tui](../internal/tui) | Script/TUI adapters and legacy Bash surface. |
| [smoke.sh](../smoke.sh) | Script/TUI adapters and legacy Bash surface. |

Intent selects the owning policy; that policy produces decisions or artifacts;
adapters perform effects; verification establishes the result. Change the
owner once and keep alternate surfaces on that same contract.

## Invariants

- Example specs target real networks and are not offline unit tests.
- A step pass proves only its assertion at its run time.

## Existing action interfaces

These are inspected command surfaces, not a report that they ran. Read current
help and recipes for arguments, dependencies and lifecycle hooks before use.
Examples containing placeholder paths or bracketed options are grammar.

| Command | Effects and evidence |
| --- | --- |
| `make test-go` | Go race tests. |
| `make test-bash` | Legacy BATS tests. |
| `make build` | Builds local smoke binary. |
| `./bin/smoke run path/to/spec.yaml --json` | Executes specified checks and emits machine result; live targets can trigger network. |

## Observe, verify and retain

Establish source revision, dirty state and relevant input identity before
choosing an action. Keep intended settings, cached artifacts and observed
runtime state distinct. An existing artifact is not a freshness or readiness
claim. Use the smallest deterministic fixture at the changed seam first;
expand to process, browser, device or deployment checks only when that
claim needs them. Record unavailable evidence explicitly.

Retain the command/configuration, source and input identity, result, limitation
and next discriminating check. Reuse evidence only while its relevant inputs
remain applicable. Promote a reproducible failure to a regression fixture,
a design decision to its owning document, and a repeated operator correction
to one concise guide rule. Keep private observations in private artifacts.

## Implemented plan for this pass

- [x] Map current source ownership and existing interfaces.
- [x] Make command effects and evidence limits discoverable.
- [x] Route agent work here and retain detailed product plans at their owners.

Acceptance: owner paths and document links resolve; current instructions
match inspected source; catalog hashes bind this context to the reviewed
bytes. This is documentation/control navigation acceptance. Product runtime
checks retain their own scope and are not certified by this pass.

## Project decisions

Navigate YAML spec to parsed steps to runner outcome; CLI and TUI must share that execution core. Use internal/runner/offline_contract_test.go and the focused Go tests for deterministic HTTP/DNS/TLS/TCP semantics, preserving legacy Bash compatibility only where promised. A live result must identify spec/path or digest, effective target, source/binary revision, timestamp and each step outcome. Report reachability/TLS/assertion failure separately from invalid spec or local prerequisite failure. Do not run checked-in internet examples as an offline gate. Reuse local fixtures during iteration and retain newly confirmed protocol failures as runner regressions before broadening network checks.
