# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd prime` for the current workflow context.

## Quick Reference

```bash
bd ready                            # Find available work
bd show <id>                        # View issue details
bd update <id> --status in_progress # Claim work
bd close <id>                       # Complete work
bd export -o .beads/issues.jsonl    # Refresh tracked issue snapshot
```

## Beads Setup

Latest `bd` expects a working Dolt-backed setup. If the local server state looks wrong, start with:

```bash
bd doctor --server
bd ready
```

## Branch Hygiene

```bash
# First push for a new branch must set upstream
git push -u origin HEAD

# Keep working tree clean before pulls/rebases
git status
git pull --rebase
```

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd export -o .beads/issues.jsonl || true
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
