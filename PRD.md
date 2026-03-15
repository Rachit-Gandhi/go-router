# Go Router PRD (v1)

## 1) Product Summary

Build an OpenRouter-like multi-tenant gateway in Go with two binaries:

- `control` plane: org, team, member, auth, key and policy management.
- `router` plane: OpenAI-compatible inference gateway (`/chat/completions`) with provider/model routing.

Primary priorities for v1 are correctness and extensibility.

## 2) Goals and Non-Goals

### Goals

- Multi-tenant tenancy model centered on `org`.
- Role hierarchy: `org_owner -> team_admin -> member`.
- Magic-link sign-in for all users, exchanging link code for secure session cookie.
- Team-scoped user API keys for router access.
- Org-level provider/model allowlists with team-level reductions.
- OpenAI-compatible `/v1/router/chat/completions` endpoint.
- Clear implementation plan that an automated agent can execute.

### Non-Goals (v1)

- Billing
- Audit trails
- SSO
- Compliance frameworks
- Observability layer (deferred to future phase)

## 3) Locked Architecture and Repo Structure

Monorepo with backend-first implementation.

```text
/
  PRD.md
  memory.md
  frontend/                  # empty placeholder (Next.js/React later)
  cmd/
    control/
      main.go
    router/
      main.go
  internal/
    auth/
    rbac/
    control/
    router/
    store/
    crypto/
    policy/
  db/
    migrations/              # goose migrations
    query/                   # sqlc queries
    sqlc.yaml
  scripts/
    update_memory_after_commit.sh
    setup_hooks.sh
  .githooks/
    post-commit
  skills/
    memory-updater/
      SKILL.md
```

Tech choices:

- Backend: Go (`net/http`)
- DB: PostgreSQL
- Cache/coordination: Redis only where critically required
- Migrations: Goose
- Queries: SQLC

## 4) Domain Model and Access Model

### 4.1 Role and Scope Model

Global org role per user:

- `org_owner`: full control in org.
- `team_admin`: manage members and policy reductions only for assigned teams.
- `member`: invoke router with own team API key.

Important locked interpretation:

- User has one org-level role.
- Team-admin scope is constrained by explicit team mappings (`team_admin_scopes`).
- `org_owner` is implicitly a member/admin of all teams.

### 4.2 Core Entities

- `orgs`
- `users`
- `org_memberships` (role)
- `teams`
- `team_memberships`
- `team_admin_scopes`
- `auth_magic_links`
- `auth_refresh_tokens`
- `user_team_api_keys`
- `org_provider_keys`
- `org_model_policies`
- `team_model_policies`
- `usage_logs`

## 5) Data Model (PostgreSQL)

All tenant-owned tables include `org_id` and enforce tenant-safe constraints.

### 5.1 Suggested Tables (v1)

1. `orgs`
- `id` (pk, text/uuid)
- `name`
- `owner_user_id` (fk users.id)
- `created_at`, `updated_at`

2. `users`
- `id`
- `email` (unique)
- `name`
- `created_at`, `updated_at`

3. `org_memberships`
- `org_id` (fk)
- `user_id` (fk)
- `role` (`org_owner|team_admin|member`)
- `created_at`
- `PRIMARY KEY (org_id, user_id)`

4. `teams`
- `id`
- `org_id` (fk)
- `name`
- `profile_jsonb`
- `rate_limit_per_minute` (nullable)
- `created_at`, `updated_at`
- `UNIQUE (org_id, name)`

5. `team_memberships`
- `org_id`
- `team_id`
- `user_id`
- `created_at`
- `PRIMARY KEY (team_id, user_id)`
- `UNIQUE (org_id, team_id, user_id)`

6. `team_admin_scopes`
- `org_id`
- `team_id`
- `admin_user_id`
- `created_at`
- `PRIMARY KEY (team_id, admin_user_id)`

7. `auth_magic_links`
- `id`
- `org_id`
- `email`
- `code_hash`
- `expires_at`
- `consumed_at` (nullable)
- `created_at`

8. `auth_refresh_tokens`
- `id`
- `org_id`
- `user_id`
- `token_hash`
- `session_id`
- `device_info` (nullable)
- `expires_at`
- `revoked_at` (nullable)
- `last_used_at`
- `created_at`

9. `user_team_api_keys`
- `id`
- `org_id`
- `team_id`
- `user_id`
- `key_hash`
- `key_prefix`
- `revoked_at` (nullable)
- `is_active`
- `last_used_at` (nullable)
- `created_at`
- `UNIQUE (org_id, team_id, user_id, is_active) WHERE is_active=true`

10. `org_provider_keys`
- `id`
- `org_id`
- `provider` (`claude|codex|gemini|...`)
- `key_ciphertext`
- `key_nonce`
- `key_kek_id` (or static key label)
- `is_active`
- `created_at`, `updated_at`

11. `org_model_policies`
- `org_id`
- `provider`
- `model`
- `is_allowed`
- `created_at`
- `PRIMARY KEY (org_id, provider, model)`

12. `team_model_policies`
- `org_id`
- `team_id`
- `provider`
- `model`
- `is_allowed`
- `created_at`
- `PRIMARY KEY (org_id, team_id, provider, model)`

13. `usage_logs`
- `id`
- `org_id`
- `team_id`
- `user_id`
- `api_key_id`
- `provider`
- `model`
- `request_tokens`
- `response_tokens`
- `latency_ms`
- `status_code`
- `request_fingerprint`
- `created_at`

## 6) Security and Isolation Requirements

Hard no-fail conditions:

- No cross-tenant data leak.
- No API key plaintext storage or accidental exposure.

### 6.1 Mandatory Security Controls (v1)

1. Tenant isolation
- Every query in control/router paths must scope by `org_id`.
- API key auth resolves `(org_id, team_id, user_id)` before request handling.

2. Key security
- Store only API key hash for user keys.
- Store provider keys encrypted at rest (`key_ciphertext`), decrypt only in memory at call time.
- Return only one-time plaintext value on key creation.

3. Session security
- Signed and encrypted HTTP-only `SameSite` cookie.
- Sliding refresh window target: 15 minutes.
- Refresh token rotation in DB (`auth_refresh_tokens`).
- Revoke chain on suspicious reuse.

4. Auth flow hardening
- Magic-link code stored as hash.
- One-time consumption and short expiry.
- Rate-limit login initiation and code exchange.

5. Input/output controls
- Validate model/provider permissions before upstream calls.
- Reject disallowed provider/model combos with explicit 403.

## 7) API Contract (v1)

Base path split:

- Control: `/v1/control/...`
- Router: `/v1/router/...`

### 7.1 Control Plane APIs

1. Auth
- `POST /v1/control/auth/magic-link/request`
- `POST /v1/control/auth/magic-link/exchange`
- `POST /v1/control/auth/refresh`
- `POST /v1/control/auth/logout`

2. Org and team management
- `POST /v1/control/orgs` (signup flow creates org + owner membership)
- `POST /v1/control/orgs/{org_id}/teams`
- `POST /v1/control/orgs/{org_id}/teams/{team_id}/members`
- `POST /v1/control/orgs/{org_id}/teams/{team_id}/admins/{user_id}` (scope mapping)

3. API keys
- `POST /v1/control/orgs/{org_id}/teams/{team_id}/users/{user_id}/api-keys`
- `POST /v1/control/orgs/{org_id}/api-keys/{key_id}/revoke`

4. Provider and model policies
- `POST /v1/control/orgs/{org_id}/providers/{provider}/keys`
- `PUT /v1/control/orgs/{org_id}/policies/models`
- `PUT /v1/control/orgs/{org_id}/teams/{team_id}/policies/models` (reduce-only)

### 7.2 Router Plane API

OpenAI-compatible request/response shape:

- `POST /v1/router/chat/completions`

Auth:

- `Authorization: Bearer <user_team_api_key>`

Routing:

- v1 complexity heuristic: prompt size/token estimate.
- Short prompts -> faster/cheaper models.
- Long prompts -> stronger models.
- Must stay within org/team policy intersection.

## 8) Policy Evaluation Rules

Effective model allowlist:

1. Start from org allowlist (`org_model_policies is_allowed=true`).
2. Intersect with team policy:
- Team admin can only reduce.
- Team policy cannot add model/provider absent at org level.
3. Router chooses model from effective allowlist only.

## 9) Functional Requirements with Explicit Tests

### FR-1: Org signup and owner bootstrap
- Requirement: Signup creates org, owner user, owner membership.
- Tests:
1. Integration: `POST /orgs` creates `orgs + users + org_memberships(role=org_owner)`.
2. Security: owner has access to all team management routes.

### FR-2: Team creation by owner only
- Requirement: Only owner can create teams.
- Tests:
1. Authorization: team_admin/member get 403 on create-team.
2. Data: new team row has correct `org_id`.

### FR-3: Team member management
- Requirement: owner can add members to any team; team_admin only within scoped teams.
- Tests:
1. team_admin can add member to scoped team -> 200.
2. team_admin add to non-scoped team -> 403.
3. owner add any team -> 200.

### FR-4: Magic-link auth + session
- Requirement: login by magic-link code exchange and secure cookie session with refresh.
- Tests:
1. request link creates hashed code with expiry.
2. exchange consumes code once and sets HTTP-only encrypted cookie.
3. refresh rotates DB refresh token and extends session window.

### FR-5: Per-user per-team API key
- Requirement: key generated per user/team and usable for router auth.
- Tests:
1. create key returns plaintext once; DB stores hash only.
2. bearer auth resolves active key and org/team/user identity.
3. revoked key rejected (401).

### FR-6: Org provider key management
- Requirement: owner stores provider keys and org model allowlist.
- Tests:
1. ciphertext persists, plaintext never returned after creation.
2. team policy update cannot include model not in org allowlist.

### FR-7: Team-level reduce-only policy
- Requirement: team_admin can restrict allowed providers/models for scoped team.
- Tests:
1. remove model succeeds.
2. add model absent at org level fails (400/403).

### FR-8: OpenAI-compatible chat completions
- Requirement: `/v1/router/chat/completions` accepts OpenAI shape and proxies upstream.
- Tests:
1. contract tests for payload fields and response shape.
2. policy denial yields deterministic forbidden error.

### FR-9: Router complexity strategy (v1)
- Requirement: route by prompt complexity heuristic (length/token estimate).
- Tests:
1. short prompt selects fast tier model.
2. long prompt selects stronger tier model.
3. selected model must be in effective allowlist.

### FR-10: Usage logging
- Requirement: persist usage for each routed request.
- Tests:
1. successful call creates usage row with tokens and model.
2. failed upstream call logs status and latency.

### FR-11: No cross-tenant leakage
- Requirement: tenant scoping enforced across control/router flows.
- Tests:
1. API key from org A cannot access org B resources.
2. all SQL queries include tenant predicate in query tests/review.

### FR-12: No key exposure
- Requirement: provider and user keys are never logged/plaintext at rest.
- Tests:
1. unit test sanitizer redacts secrets.
2. DB inspection test verifies hashed/ciphertext storage only.

## 10) Agent-Executable Delivery Plan

### Milestone 0: Bootstrap
- Initialize module, directories, lint/test baseline, `frontend/` placeholder.
- Done when:
1. `go test ./...` passes with scaffold tests.
2. `sqlc generate` and migration tooling wired.

### Milestone 1: Schema and queries
- Create goose migrations for all v1 tables and constraints.
- Add sqlc queries for CRUD/auth/policy/router lookup paths.
- Done when:
1. migrations up/down cleanly on local Postgres.
2. sqlc compile succeeds.

### Milestone 2: Auth and RBAC (control)
- Implement magic-link request/exchange, cookie session, refresh rotation.
- Implement RBAC middleware and team admin scope checks.
- Done when:
1. auth integration tests pass.
2. authorization matrix tests pass.

### Milestone 3: Key and policy management (control)
- Implement per-user/team API key issuance + revocation.
- Implement org provider keys + org/team model policy APIs.
- Done when:
1. key security tests pass.
2. policy inheritance tests pass.

### Milestone 4: Router `/chat/completions`
- Implement OpenAI-compatible endpoint and upstream adapters.
- Add complexity heuristic model selector.
- Done when:
1. contract tests pass.
2. routing policy tests pass.

### Milestone 5: Usage and hardening
- Implement usage logs and mandatory security guards.
- Done when:
1. cross-tenant leak tests pass.
2. key exposure tests pass.

## 11) Explicit Open Questions (Tracked Assumptions)

1. Cookie crypto implementation detail (library and key rotation cadence) is deferred to implementation design doc.
2. Provider adapter set for v1 defaults to placeholders with one real provider integration first.
3. Rate limiting strategy is team-level and can use Redis when needed.

---

This PRD is the source-of-truth for v1 implementation.
