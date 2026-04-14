# LLM Integration Guide

`loom` is designed for deterministic automation loops where an LLM must:

1. Discover PR review feedback
2. Filter/sort and prioritize
3. Post top-level or inline PR comments when needed
4. Edit or delete mistaken comments without leaving the PR context
5. Post replies with resolution notes
6. Resolve review threads
7. Merge once the review state is clear
8. Re-check for remaining unresolved threads

## Recommended invocation style

Always pass explicit flags and prefer JSON output when available:

- `loom list --repo <owner/repo> --pr <n> --state unresolved --json`
- `loom find --repo <owner/repo> --pr <n> --query "<needle>" --json`
- `loom comment --repo <owner/repo> --pr <n> --body "<text>" --json`
- `loom comment --repo <owner/repo> --pr <n> --path <file> --line <n> --side RIGHT --body "<text>" --json`
- `loom comment --repo <owner/repo> --pr <n> --path <file> --start-line <n> --start-side RIGHT --line <n> --side RIGHT --body "<text>" --json`
- `loom comment --repo <owner/repo> --pr <n> --path <file> --subject file --body "<text>" --json`
- `loom edit --repo <owner/repo> --comment <id> --body "<text>" --json`
- `loom delete --repo <owner/repo> --comment <id> --json`
- `loom reply --repo <owner/repo> --pr <n> --comment <id> --body "<text>" --json`
- `loom resolve --thread <PRRT_...> --json`
- `loom merge --repo <owner/repo> --pr <n> --method squash --json`

## Automation-friendly flow

```bash
# 1) Fetch unresolved threads
loom list --repo ryuvel/tacara --pr 24 --state unresolved --json > /tmp/threads.json

# 2) Narrow to critical findings in one path
loom list --repo ryuvel/tacara --pr 24 --state unresolved --severity critical --path tacara-core/src --json

# 3) Leave a top-level PR note
loom comment --repo ryuvel/tacara --pr 24 --body "Overall review note" --json

# 4) Leave an inline review comment
loom comment --repo ryuvel/tacara --pr 24 --path README.md --line 14 --side RIGHT --body "This line needs clarification." --json

# 5) Correct a mistaken comment in place
loom edit --repo ryuvel/tacara --comment 2857259586 --body "This is the corrected wording." --json

# 6) Reply with a commit URL
loom reply --repo ryuvel/tacara --pr 24 --comment 2857259586 --body "Addressed in https://github.com/owner/repo/commit/<sha>" --json

# 7) Resolve by thread node ID
loom resolve --thread PRRT_kwDORR607s5w3N_2 --json

# 8) Delete a stray top-level or review comment if needed
loom delete --repo ryuvel/tacara --comment 2857259586 --json

# 9) Merge when review state is clean
loom merge --repo ryuvel/tacara --pr 24 --method squash --json

# 10) Verify empty unresolved queue
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
- `comment not found in owner/repo`
  - confirm the comment ID and pass `--type review` or `--type top-level` if auto-detection is ambiguous.
- `--method must be "merge", "squash", or "rebase"`
  - pass a supported merge method.
- auth failures
  - run `gh auth status` and re-authenticate.

## Determinism notes

- `list` defaults to unresolved state.
- `list`/`find` default sort is `updated` descending.
- repo inference uses upstream git remote first, then `go-gh` current repo detection.
