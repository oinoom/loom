# loom

`loom` is a fast CLI for triaging and actioning GitHub PR comments and lightweight issue/PR workflows.
It is designed for high-volume review loops across top-level PR comments and review threads, including inline comments on files and diff lines.
It is also designed to be automation/LLM-friendly.

## What it does

- Lists PR review threads (default: unresolved only)
- Filters by path/author/severity/text
- Sorts by updated/created/path/line/author/severity
- Posts top-level PR conversation comments
- Posts inline PR review comments on files, lines, or line ranges
- Offers explicit comment mode aliases for top-level, inline, and file comments
- Edits or deletes existing PR comments by comment ID
- Creates GitHub issues
- Closes GitHub issues
- Opens pull requests
- Edits pull request title/body/base
- Merges pull requests from the CLI
- Replies to review comments by DB comment ID or comment URL
- Resolves/unresolves review threads by GraphQL thread ID, or by review comment ID/URL when `--repo` and `--pr` are provided

## LLM-first docs

- [llms.txt](./llms.txt)
- [llms-full.txt](./llms-full.txt)
- [docs/LLM_GUIDE.md](./docs/LLM_GUIDE.md)

## Requirements

- Go 1.24+ (tested with Go 1.26)
- GitHub CLI (`gh`) authenticated with `repo` scope

## Build

```bash
cd /path/to/loom
go mod tidy
go build -o loom
```

## Install to PATH

This installs to `~/.local/bin` (already in PATH on this machine):

```bash
cd /path/to/loom
./install.sh
```

Then verify:

```bash
loom help
```

Command discovery:

```bash
loom help list
loom help comment inline
loom thread resolve --help
```

## Install Agent Skill

The repository ships a reusable Loom skill at `skills/loom`.
Tell your agent to install this skill.

If you want to install it locally into common skill directories, run:

```bash
cd /path/to/loom
./install-skill.sh
```

This installs:

- `~/.codex/skills/loom/SKILL.md`
- `~/.claude/skills/loom/SKILL.md`

## Usage

List unresolved threads in a stable machine-friendly way:

```bash
loom list --repo <owner/repo> --pr <pr-number> --format json
```

Find by keyword, narrow to one file, sort newest first:

```bash
loom list --repo <owner/repo> --pr <pr-number> --query "<text>" --path <path/to/file> --sort updated --desc
```

Include grouped stats:

```bash
loom list --repo <owner/repo> --pr <pr-number> --state unresolved --stats --format table
```

Comment on a PR:

```bash
loom comment top --repo <owner/repo> --pr <pr-number> --body "Top-level PR note"
```

Comment on one diff line:

```bash
loom comment inline --repo <owner/repo> --pr <pr-number> --path <path/to/file> --line <line-number> --side RIGHT --body "Please rename this."
```

Comment on a diff line range:

```bash
loom comment inline --repo <owner/repo> --pr <pr-number> --path <path/to/file> --start-line <start-line> --start-side RIGHT --line <end-line> --side RIGHT --body "This section needs more detail."
```

Comment on a whole file in the PR:

```bash
loom comment file --repo <owner/repo> --pr <pr-number> --path <path/to/file> --body "This file needs an inline usage example."
```

Edit an existing PR comment:

```bash
loom comment edit --repo <owner/repo> --comment-id <comment-id> --body "Updated wording"
```

Delete an existing PR comment:

```bash
loom comment delete --repo <owner/repo> --comment-id <comment-id>
```

Create an issue:

```bash
loom issue create --repo <owner/repo> --title "Tracking bug" --body "Details"
```

Close an issue:

```bash
loom issue close --repo <owner/repo> --issue <issue-number> --reason completed
```

Open a pull request:

```bash
loom pr create --repo <owner/repo> --head <head-branch> --base <base-branch> --title "Ship it" --body "Summary"
```

Edit a pull request:

```bash
loom pr edit --repo <owner/repo> --pr <pr-number> --title "Updated title" --body "Updated summary"
```

Merge a PR:

```bash
loom pr merge --repo <owner/repo> --pr <pr-number> --method squash
```

Reply to a review comment:

```bash
loom comment reply --repo <owner/repo> --pr <pr-number> --comment-id <comment-id> --body "Addressed in <commit-url>"
```

Resolve / unresolve a thread:

```bash
loom thread resolve --thread-id <thread-id>
loom thread unresolve --thread-id <thread-id>
```

Resolve a thread from a review comment URL:

```bash
loom thread resolve --repo <owner/repo> --pr <pr-number> --comment-id "https://github.com/<owner>/<repo>/pull/<pr-number>#discussion_r<comment-id>"
```

Machine-readable output:

```bash
loom list --repo <owner/repo> --pr <pr-number> --state all --format jsonl
loom comment top --repo <owner/repo> --pr <pr-number> --body "Top-level PR note" --json
loom comment inline --repo <owner/repo> --pr <pr-number> --path <path/to/file> --line <line-number> --side RIGHT --body "Please rename this." --json
loom comment edit --repo <owner/repo> --comment-id <comment-id> --body "Updated wording" --json
loom comment delete --repo <owner/repo> --comment-id <comment-id> --json
loom issue create --repo <owner/repo> --title "Tracking bug" --body "Details" --json
loom issue close --repo <owner/repo> --issue <issue-number> --reason completed --json
loom pr create --repo <owner/repo> --head <head-branch> --base <base-branch> --title "Ship it" --body "Summary" --json
loom pr edit --repo <owner/repo> --pr <pr-number> --title "Updated title" --body "Updated summary" --json
loom pr merge --repo <owner/repo> --pr <pr-number> --method squash --json
loom comment reply --repo <owner/repo> --pr <pr-number> --comment-id <comment-id> --body "Addressed in <commit-url>" --json
loom thread resolve --thread-id <thread-id> --json
```

Notes:

- `loom find` still works, but `loom list --query ...` is the preferred search form.
- `--comment-id` and `--thread-id` are the preferred flag names; `--comment` and `--thread` remain supported.
- Nested subcommands like `loom comment inline` and `loom pr merge` are the preferred command shape; legacy single-token aliases remain supported.
- `--stats` prints to stderr so it can be used with `--format json` or `--format jsonl` safely.
