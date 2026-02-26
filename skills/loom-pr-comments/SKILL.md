---
name: loom-pr-comments
description: Use this skill when triaging, filtering, replying to, or resolving GitHub PR review comments. Trigger for requests like "address PR comments", "show unresolved review threads", "reply and resolve comments", or "find comments by file/severity/author". Uses the `loom` CLI for fast list/find/reply/resolve workflows.
---

# Loom PR Comments

Use `loom` to handle GitHub review comments quickly.

## Preconditions

- `loom` is installed and in PATH.
- `gh auth status` is healthy.
- You know repo + PR number.

## Core Commands

- List unresolved threads:
  - `loom list --repo <owner/repo> --pr <number>`
- Sift by file/author/severity and sort:
  - `loom list --repo <owner/repo> --pr <number> --path <path-substring> --author <login> --severity <critical|major|minor> --sort updated --desc`
- Find by text:
  - `loom find --repo <owner/repo> --pr <number> --query "<text>"`
- Include triage stats:
  - `loom list --repo <owner/repo> --pr <number> --stats`
- Reply to a review comment:
  - `loom reply --repo <owner/repo> --pr <number> --comment <database-comment-id> --body "<message>"`
- Resolve/unresolve a thread:
  - `loom resolve --thread <PRRT_...>`
  - `loom unresolve --thread <PRRT_...>`

## Typical Workflow

1. `loom list` unresolved comments.
2. Group with `--stats`, narrow with `--path`, `--author`, `--severity`.
3. Implement fixes.
4. Reply with commit links via `loom reply`.
5. Resolve threads with `loom resolve`.
6. Re-run `loom list --state unresolved` and ensure the table is empty.
