# LLM Integration Guide

`loom` is designed for deterministic automation loops where an LLM must:

1. Discover PR review feedback
2. Filter/sort and prioritize
3. Post top-level or inline PR comments when needed
4. Edit or delete mistaken comments without leaving the PR context
5. Post replies with resolution notes
6. Create tracking issues when a bug should outlive the current PR
7. Close tracking issues when they are done
8. Open pull requests without dropping to another CLI
9. Edit pull request title/body/base without dropping to another CLI
10. Resolve review threads
11. Merge once the review state is clear
12. Re-check for remaining unresolved threads

## Recommended invocation style

Always pass explicit flags and prefer JSON output when available:

- `loom list --repo <owner/repo> --pr <n> --state unresolved --format json`
- `loom list --repo <owner/repo> --pr <n> --query "<needle>" --format json`
- `loom comment-top --repo <owner/repo> --pr <n> --body "<text>" --json`
- `loom comment-inline --repo <owner/repo> --pr <n> --path <file> --line <n> --side RIGHT --body "<text>" --json`
- `loom comment-inline --repo <owner/repo> --pr <n> --path <file> --start-line <n> --start-side RIGHT --line <n> --side RIGHT --body "<text>" --json`
- `loom comment-file --repo <owner/repo> --pr <n> --path <file> --body "<text>" --json`
- `loom edit --repo <owner/repo> --comment-id <id-or-url> --body "<text>" --json`
- `loom delete --repo <owner/repo> --comment-id <id-or-url> --json`
- `loom issue --repo <owner/repo> --title "<text>" --body "<text>" --json`
- `loom issue-close --repo <owner/repo> --issue <n> [--reason completed|not_planned] --json`
- `loom pr-create --repo <owner/repo> --head <branch> --base <branch> --title "<text>" --body "<text>" --json`
- `loom pr-edit --repo <owner/repo> --pr <n> [--title "<text>"] [--body "<text>"] [--base <branch>] --json`
- `loom reply --repo <owner/repo> --pr <n> --comment-id <id-or-url> --body "<text>" --json`
- `loom resolve --thread-id <PRRT_...> --json`
- `loom resolve --repo <owner/repo> --pr <n> --comment-id <id-or-url> --json`
- `loom merge --repo <owner/repo> --pr <n> --method squash --json`

## Automation-friendly flow

```bash
# 1) Fetch unresolved threads
loom list --repo ryuvel/tacara --pr 24 --state unresolved --format json > /tmp/threads.json

# 2) Narrow to critical findings in one path
loom list --repo ryuvel/tacara --pr 24 --state unresolved --severity critical --path tacara-core/src --format json

# 3) Leave a top-level PR note
loom comment-top --repo ryuvel/tacara --pr 24 --body "Overall review note" --json

# 4) Leave an inline review comment
loom comment-inline --repo ryuvel/tacara --pr 24 --path README.md --line 14 --side RIGHT --body "This line needs clarification." --json

# 5) Correct a mistaken comment in place
loom edit --repo ryuvel/tacara --comment-id 2857259586 --body "This is the corrected wording." --json

# 6) Reply with a commit URL
loom reply --repo ryuvel/tacara --pr 24 --comment-id 2857259586 --body "Addressed in https://github.com/owner/repo/commit/<sha>" --json

# 7) Resolve by thread node ID
loom resolve --thread-id PRRT_kwDORR607s5w3N_2 --json

# 8) Delete a stray top-level or review comment if needed
loom delete --repo ryuvel/tacara --comment-id 2857259586 --json

# 9) Open a tracking issue if the bug belongs outside the PR
loom issue --repo ryuvel/tacara --title "Follow-up bug" --body "Detailed repro." --json

# 10) Close the issue when resolved
loom issue-close --repo ryuvel/tacara --issue 101 --reason completed --json

# 11) Open the PR itself from loom if needed
loom pr-create --repo ryuvel/tacara --head feat/work --base main --title "Ship it" --body "Summary" --json

# 12) Edit the PR if the title or body needs cleanup
loom pr-edit --repo ryuvel/tacara --pr 24 --title "Updated title" --body "Updated summary" --json

# 13) Merge when review state is clean
loom merge --repo ryuvel/tacara --pr 24 --method squash --json

# 14) Verify empty unresolved queue
loom list --repo ryuvel/tacara --pr 24 --state unresolved --format json
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
- `issue body is empty`
  - pass `--body`, `--body-file`, or pipe stdin.
- `pull request body is empty`
  - pass `--body`, `--body-file`, or pipe stdin.
- `comment not found in owner/repo`
  - confirm the comment ID and pass `--type review` or `--type top-level` if auto-detection is ambiguous.
- `review thread not found for comment ...`
  - confirm `--repo`, `--pr`, and the review comment reference when resolving by comment ID or URL.
- `--reason must be "completed" or "not_planned"`
  - pass a supported issue close reason or omit `--reason`.
- `--method must be "merge", "squash", or "rebase"`
  - pass a supported merge method.
- auth failures
  - run `gh auth status` and re-authenticate.

## Determinism notes

- `list` defaults to unresolved state.
- `list`/`find` default sort is `updated` descending.
- `list --format auto` selects `table` on a TTY and `json` otherwise.
- `--stats` writes to stderr, so stdout stays machine-readable when `--format json` or `--format jsonl` is used.
- repo inference uses upstream git remote first, then `go-gh` current repo detection.
