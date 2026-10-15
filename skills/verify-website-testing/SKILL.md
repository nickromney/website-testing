---
name: verify-website-testing
description: Verify smoke's CLI against a local HTTP fixture.
---

# Verify website-testing

From the project root, run `make verify-dry-run`, `make verify-test`, and `make verify`.
The harness builds the real Go `smoke` CLI into isolated run state, starts an owned local
HTTP fixture, and exercises quick mode with status, body and header assertions. It preserves
the CLI output and fixture log under `${TMPDIR}/verify-website-testing-evidence/<run-id>`.
Cleanup stops only the fixture process group started by the run and deletes that run's state.

This verifies the CLI's HTTP request and assertion path against localhost. It does not test
public network reachability, DNS/TLS probes, or the interactive terminal UI.
