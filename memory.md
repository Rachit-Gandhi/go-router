# Project Memory

## Project Identity
- Name: `go-router`
- Path: `$REPO_ROOT`
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
- 2026-03-15T21:50:16+05:30 5f25456: Add model pricing sync and estimated usage cost metrics [files: Makefile,cmd/pricing-sync/main.go,db/migrations/000005_model_pricing.sql,db/query/model_pricing.sql,db/query/model_pricing.sql.go,db/query/models.go,frontend/README.md,internal/control/httpapi/read_endpoints.go,internal/control/httpapi/visibility_test.go,internal/pricing/data/static_prices.json,internal/pricing/static.go,internal/pricing/sync.go,internal/pricing/sync_test.go]
- 2026-03-15T21:11:10+05:30 d94b48b: Address CodeRabbit follow-ups for redaction and fingerprinting [files: internal/httputil/redact.go,internal/httputil/redact_test.go,internal/router/httpapi/handler.go,internal/router/httpapi/handler_test.go]
- 2026-03-15T19:45:18+05:30 952bd71: Implement milestone 5 usage logging and hardening [files: internal/httputil/redact.go,internal/httputil/redact_test.go,internal/router/httpapi/handler.go,internal/router/httpapi/milestone4_test.go,internal/router/httpapi/postgres_test.go]
- 2026-03-15T19:15:53+05:30 2a3b008: Resolve follow-up CodeRabbit review comments [files: internal/control/httpapi/read_endpoints.go,internal/control/httpapi/read_endpoints_test.go,internal/control/httpapi/visibility_test.go,internal/router/httpapi/handler.go,internal/router/httpapi/milestone4_test.go]
- 2026-03-15T18:40:14+05:30 d3f01cc: Resolve CodeRabbit review findings for router and visibility endpoints [files: db/migrations/000004_usage_logs_partition_maintenance.sql,internal/control/httpapi/read_endpoints.go,internal/control/httpapi/read_endpoints_test.go,internal/control/httpapi/visibility_test.go,internal/router/httpapi/handler.go,internal/router/httpapi/handler_test.go,internal/router/httpapi/milestone4_test.go,internal/router/httpapi/postgres_test.go]
- 2026-03-15T18:24:56+05:30 b7f3672: Implement control-plane visibility read endpoints [files: internal/control/httpapi/handler.go,internal/control/httpapi/read_endpoints.go,internal/control/httpapi/visibility_test.go]
- 2026-03-15T17:53:19+05:30 b50bbdd: Implement Milestone 4 router chat completions with policy-aware routing [files: cmd/router/main.go,internal/router/httpapi/handler.go,internal/router/httpapi/handler_test.go,internal/router/httpapi/milestone4_test.go,internal/router/httpapi/postgres_test.go]
- 2026-03-15T17:22:31+05:30 830092c: Address CodeRabbit review follow-ups for milestone auth/rbac [files: db/query/memberships.sql,db/query/memberships.sql.go,db/query/teams.sql,db/query/teams.sql.go,internal/bootstrap/schema_m1_review_test.go,internal/control/httpapi/handler.go,internal/control/httpapi/handler_test.go,internal/control/httpapi/milestone2_test.go,internal/control/httpapi/milestone3_test.go,internal/control/httpapi/postgres_test.go,internal/store/postgres.go]
- 2026-03-15T12:55:21+05:30 ef7b649: docs: update memory commit journal [files: memory.md]
- 2026-03-15T12:55:08+05:30 5a2a7cd: chore: bootstrap project scaffold and tooling [files: .githooks/post-commit,Makefile,PRD.md,cmd/control/main.go,cmd/router/main.go,db/migrations/000001_init.sql,db/query/db.go,db/query/health_checks.sql,db/query/health_checks.sql.go,db/query/models.go,db/sqlc.yaml,frontend/.gitkeep,go.mod,internal/auth/doc.go,internal/bootstrap/layout_test.go,internal/bootstrap/tooling_test.go,internal/control/httpapi/handler.go,internal/control/httpapi/handler_test.go,internal/crypto/doc.go,internal/policy/doc.go,internal/rbac/doc.go,internal/router/httpapi/handler.go,internal/router/httpapi/handler_test.go,internal/store/doc.go,memory.md,scripts/setup_hooks.sh,scripts/update_memory_after_commit.sh,skills/memory-updater/SKILL.md]
- 2026-03-14T20:42:43+05:30 6d0d280: chore: reset repository to empty initial state [files: none]
<!-- COMMIT_JOURNAL_END -->
