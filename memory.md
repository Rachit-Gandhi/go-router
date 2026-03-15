# Project Memory

## Project Identity
- Name: `go-router`
- Path: `/Users/rachitgandhi/workspace/github.com/Rachit-Gandhi/go-router`
- Goal: Build an OpenRouter-like multi-tenant gateway in Go.
- Priority: correctness and extensibility.

## Locked Decisions
- Monorepo, backend first; keep `frontend/` placeholder only.
- Two binaries: `cmd/control` and `cmd/router`.
- Backend stack: Go `net/http`, PostgreSQL, Goose, SQLC.
- API split: `/v1/control/*` and `/v1/router/*`.
- Router contract: OpenAI-compatible `/chat/completions`.
- Auth: magic-link + signed encrypted HTTP-only same-site cookie + refresh token table.
- Sliding refresh target: 15 minutes.
- Redis: encouraged only where critically needed.
- Background workers: allowed.

## RBAC and Tenancy
- Role hierarchy: `org_owner -> team_admin -> member`.
- One org-level role per user.
- Team-admin authority is limited to assigned teams (`team_admin_scopes`).
- `org_owner` is implicitly member/admin across all teams.
- Hard constraints: no cross-tenant data leak, no key exposure.

## Data and Security Defaults
- All tenant tables carry `org_id`.
- User API keys: hash-only storage, plaintext shown once at creation.
- Provider keys: encrypted at rest (ciphertext + metadata), decrypted in memory only.
- Team policy is reduce-only against org model allowlist.

## Non-Goals (v1)
- Billing
- Audit trails
- SSO
- Compliance framework work
- Observability layer

## Repo Conventions
- SQL tables/columns: lowercase snake_case.
- Go naming: default Go conventions.
- Preferred abbreviation: `org` for organization.

## Commit Journal
<!-- COMMIT_JOURNAL_START -->
- 2026-03-14T20:42:43+05:30 6d0d280: chore: reset repository to empty initial state [files: ]
- Initialized project memory and PRD baseline.
<!-- COMMIT_JOURNAL_END -->
