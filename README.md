# loom

`loom` is a fast CLI for triaging and actioning GitHub PR comments.
It is designed for high-volume review loops across top-level PR comments and review threads, including inline comments on files and diff lines.
It is also designed to be automation/LLM-friendly.

## What it does

- Lists PR review threads (default: unresolved only)
- Filters by path/author/severity/text
- Sorts by updated/created/path/line/author/severity
- Posts top-level PR conversation comments
- Posts inline PR review comments on files, lines, or line ranges
- Edits or deletes existing PR comments by comment ID
- Merges pull requests from the CLI
- Replies to review comments by DB comment ID
- Resolves/unresolves review threads by GraphQL thread ID

## LLM-first docs

- [llms.txt](./llms.txt)
- [llms-full.txt](./llms-full.txt)
- [docs/LLM_GUIDE.md](./docs/LLM_GUIDE.md)

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

Comment on a PR:

```bash
loom comment --repo ryuvel/tacara --pr 24 --body "Top-level PR note"
```

Comment on one diff line:

```bash
loom comment --repo ryuvel/tacara --pr 24 --path main.go --line 42 --side RIGHT --body "Please rename this."
```

Comment on a diff line range:

```bash
loom comment --repo ryuvel/tacara --pr 24 --path README.md --start-line 10 --start-side RIGHT --line 14 --side RIGHT --body "This section needs more detail."
```

Comment on a whole file in the PR:

```bash
loom comment --repo ryuvel/tacara --pr 24 --path docs/LLM_GUIDE.md --subject file --body "This file needs an inline usage example."
```

Edit an existing PR comment:

```bash
loom edit --repo ryuvel/tacara --comment 2857259586 --body "Updated wording"
```

Delete an existing PR comment:

```bash
loom delete --repo ryuvel/tacara --comment 2857259586
```

Merge a PR:

```bash
loom merge --repo ryuvel/tacara --pr 24 --method squash
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

JSON output for action commands:

```bash
loom comment --repo ryuvel/tacara --pr 24 --body "Top-level PR note" --json
loom comment --repo ryuvel/tacara --pr 24 --path main.go --line 42 --side RIGHT --body "Please rename this." --json
loom edit --repo ryuvel/tacara --comment 2857259586 --body "Updated wording" --json
loom delete --repo ryuvel/tacara --comment 2857259586 --json
loom merge --repo ryuvel/tacara --pr 24 --method squash --json
loom reply --repo ryuvel/tacara --pr 24 --comment 2857259586 --body "Addressed in <commit-url>" --json
loom resolve --thread PRRT_kwDORR607s5w3N_2 --json
```

JSON output (for scripting):

```bash
loom list --repo ryuvel/tacara --pr 24 --state all --json
```
