---
name: memory-updater
description: Keep `/memory.md` current after each commit by maintaining project decisions and a commit journal.
---

# Memory Updater

## Purpose

Maintain `memory.md` as a stable project context file and append commit-level changes after each commit.

## Setup

Run once per clone:

```bash
./scripts/setup_hooks.sh
```

This configures `core.hooksPath=.githooks` and enables the post-commit updater.

## Workflow

1. Keep high-signal sections in `memory.md` accurate:
- Project identity
- Locked decisions
- RBAC/tenancy model
- Security defaults
- Non-goals
- Repo conventions

2. Allow automatic commit journaling:
- `.githooks/post-commit` runs `scripts/update_memory_after_commit.sh`.
- Script inserts latest commit summary under `## Commit Journal`.

3. When commit changes architecture or requirements:
- Update relevant memory sections manually in the same commit.
- Do not store secrets in `memory.md`.

## Guardrails

- Never remove the markers:
  - `<!-- COMMIT_JOURNAL_START -->`
  - `<!-- COMMIT_JOURNAL_END -->`
- Keep entries concise and factual.
- Record decisions, not speculation.
