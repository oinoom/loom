# AGENTS.md

## Repo Purpose

`loom` is a CLI for triaging and actioning GitHub pull request comments.

## Working Rule

- Any user-visible feature added to `loom` must be tested on a real PR in `oinoom/loom` before the work is considered complete.
- New functionality should be dogfooded through `loom` itself whenever the feature makes that possible.
- For PR-comment features, prefer testing against a `loom` PR first rather than another repository.
- When making GitHub-side changes that `loom` is intended to cover, use `loom` rather than `gh`.
- If a needed GitHub mutation is missing from `loom`, add that capability to `loom` and dogfood it on a real `loom` PR before relying on it.
- Record the exact command path that was dogfooded and whether it succeeded.
- If a feature cannot be safely dogfooded on a `loom` PR, stop and explain the blocker before merging.

## Practical Expectation

- Implement the feature.
- Open or update a `loom` PR that contains the change.
- Use the new `loom` functionality against that PR.
- Verify the result through `loom` where possible.
- Only then treat the feature as finished.
