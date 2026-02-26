# loom

`loom` is a fast CLI for triaging and actioning GitHub PR review comments.
It is designed for high-volume review loops (find/sift/sort/reply/resolve).

## What it does

- Lists PR review threads (default: unresolved only)
- Filters by path/author/severity/text
- Sorts by updated/created/path/line/author/severity
- Replies to review comments by DB comment ID
- Resolves/unresolves review threads by GraphQL thread ID

## Requirements

- Go 1.24+ (tested with Go 1.26)
- GitHub CLI (`gh`) authenticated with `repo` scope

## Build

```bash
cd tools/loom
go mod tidy
go build -o loom
```

## Install to PATH

This installs to `~/.local/bin` (already in PATH on this machine):

```bash
cd tools/loom
./install.sh
```

Then verify:

```bash
loom help
```

## Install Codex/Claude Skill

The repository ships a reusable skill at `.claude/skills/loom-pr-comments`.
Install it into both global skill locations:

```bash
cd tools/loom
./install-skill.sh
```

This installs:

- `~/.codex/skills/loom-pr-comments/SKILL.md`
- `~/.claude/skills/loom-pr-comments/SKILL.md`

## Usage

List unresolved threads:

```bash
loom list --repo ryuvel/tacara --pr 24
```

Find by keyword, narrow to one file, sort newest first:

```bash
loom find --repo ryuvel/tacara --pr 24 --query "stale rows" --path tacara-indexer/src/main.rs --sort updated --desc
```

Include grouped stats:

```bash
loom list --repo ryuvel/tacara --pr 24 --state unresolved --stats
```

Reply to a review comment:

```bash
loom reply --repo ryuvel/tacara --pr 24 --comment 2857259586 --body "Addressed in <commit-url>"
```

Resolve / unresolve a thread:

```bash
loom resolve --thread PRRT_kwDORR607s5w3N_2
loom unresolve --thread PRRT_kwDORR607s5w3N_2
```

JSON output (for scripting):

```bash
loom list --repo ryuvel/tacara --pr 24 --state all --json
```
