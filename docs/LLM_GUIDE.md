# LLM Integration Guide

`loom` is designed for deterministic automation loops where an LLM must:

1. Discover PR review feedback
2. Filter/sort and prioritize
3. Post top-level PR comments when needed
4. Post replies with resolution notes
5. Resolve review threads
6. Re-check for remaining unresolved threads

## Recommended invocation style

Always pass explicit flags and prefer JSON output when available:

- `loom list --repo <owner/repo> --pr <n> --state unresolved --json`
- `loom find --repo <owner/repo> --pr <n> --query "<needle>" --json`
- `loom comment --repo <owner/repo> --pr <n> --body "<text>" --json`
- `loom reply --repo <owner/repo> --pr <n> --comment <id> --body "<text>" --json`
- `loom resolve --thread <PRRT_...> --json`

## Automation-friendly flow

```bash
# 1) Fetch unresolved threads
loom list --repo ryuvel/tacara --pr 24 --state unresolved --json > /tmp/threads.json

# 2) Narrow to critical findings in one path
loom list --repo ryuvel/tacara --pr 24 --state unresolved --severity critical --path tacara-core/src --json

# 3) Leave a top-level PR note
loom comment --repo ryuvel/tacara --pr 24 --body "Overall review note" --json

# 4) Reply with a commit URL
loom reply --repo ryuvel/tacara --pr 24 --comment 2857259586 --body "Addressed in https://github.com/owner/repo/commit/<sha>" --json

# 5) Resolve by thread node ID
loom resolve --thread PRRT_kwDORR607s5w3N_2 --json

# 6) Verify empty unresolved queue
loom list --repo ryuvel/tacara --pr 24 --state unresolved --json
```

## Exit code contract

- `0`: command succeeded
- non-zero: command failed (arguments/API/auth/network/etc.)

## Failure modes and mitigations

- `GraphQL: Could not resolve to a PullRequest...`
  - repo or PR mismatch; pass explicit `--repo`.
- `reply body is empty`
  - pass `--body`, `--body-file`, or pipe stdin.
- `comment body is empty`
  - pass `--body`, `--body-file`, or pipe stdin.
- auth failures
  - run `gh auth status` and re-authenticate.

## Determinism notes

- `list` defaults to unresolved state.
- `list`/`find` default sort is `updated` descending.
- repo inference uses upstream git remote first, then `go-gh` current repo detection.
