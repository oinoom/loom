---
name: loom
description: Use this skill when triaging, filtering, commenting on, editing, deleting, replying to, resolving, or merging GitHub PR comments and review threads. Trigger for requests like "address PR comments", "show unresolved review threads", "leave a PR comment", "fix that comment", "delete that comment", "reply and resolve comments", or "find comments by file/severity/author". Uses the `loom` CLI for fast PR review workflows.
---

# Loom

Use `loom` to handle GitHub PR comments quickly.

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
  - `loom list --repo <owner/repo> --pr <number> --query "<text>"`
- Include triage stats:
  - `loom list --repo <owner/repo> --pr <number> --stats`
- Leave a top-level PR comment:
  - `loom comment-top --repo <owner/repo> --pr <number> --body "<message>"`
- Leave an inline PR review comment on one line:
  - `loom comment-inline --repo <owner/repo> --pr <number> --path <file> --line <n> --side RIGHT --body "<message>"`
- Leave an inline PR review comment on a line range:
  - `loom comment-inline --repo <owner/repo> --pr <number> --path <file> --start-line <n> --start-side RIGHT --line <n> --side RIGHT --body "<message>"`
- Leave a file-level PR review comment:
  - `loom comment-file --repo <owner/repo> --pr <number> --path <file> --body "<message>"`
- Edit an existing PR comment by database ID:
  - `loom edit --repo <owner/repo> --comment-id <database-comment-id-or-url> --body "<message>"`
- Delete an existing PR comment by database ID:
  - `loom delete --repo <owner/repo> --comment-id <database-comment-id-or-url>`
- Reply to a review comment:
  - `loom reply --repo <owner/repo> --pr <number> --comment-id <database-comment-id-or-url> --body "<message>"`
- Resolve/unresolve a thread:
  - `loom resolve --thread-id <PRRT_...>`
  - `loom unresolve --thread-id <PRRT_...>`
- Merge a PR after review is complete:
  - `loom merge --repo <owner/repo> --pr <number> --method squash`

## Typical Workflow

1. `loom list` unresolved comments.
2. Group with `--stats`, narrow with `--path`, `--author`, `--severity`.
3. Leave a top-level or inline PR note with `loom comment-top`, `loom comment-inline`, or `loom comment-file` when needed.
4. If you need to correct or remove your own comment, use `loom edit` or `loom delete`.
5. Implement fixes.
6. Reply with commit links via `loom reply`.
7. Resolve threads with `loom resolve`.
8. If the user asks to merge and the review state is clear, use `loom merge`.
9. Re-run `loom list --state unresolved` and ensure the table is empty.
