# Go Router Frontend (Control Layer)

This folder now contains a Next.js control console for the current backend state.

## What it does now

- Shows control/router health from:
  - `GET /v1/control/healthz`
  - `GET /v1/router/healthz`
- Supports full control-plane mutation workflows currently implemented:
  - Org bootstrap
  - Magic-link request/exchange
  - Team creation and member management
  - Team admin scope assignment
  - User-team API key create/revoke
  - Org provider key creation
  - Org/team model policy upserts
- Stores and reuses key IDs (`org_id`, `team_id`, `user_id`, `api_key_id`) in UI context.
- Shows recent activity and per-action response payloads for operational visibility.

## Architecture

The frontend uses same-origin route handlers as a proxy layer:

- `app/v1/control/[...path]/route.ts`
- `app/v1/router/[...path]/route.ts`

These forward to Go services and preserve `Set-Cookie`, so control-session cookies work in browser without CORS changes.

Defaults:

- `CONTROL_API_BASE=http://127.0.0.1:8080`
- `ROUTER_API_BASE=http://127.0.0.1:8081`

Override with env vars when needed.

## Run locally

```bash
cd frontend
npm install
npm run dev
```

Then open `http://localhost:3000`.

## Build check

```bash
cd frontend
npm run lint
npm run build
```

## Backend gaps for a full visibility dashboard

For richer visibility (table/list dashboards instead of action console cards), backend APIs still needed in the separate backend repo include:

- Read/list endpoints for orgs, teams, memberships, admin scopes, API keys, provider keys, and policies.
- Usage/metrics endpoints for router traffic and model-level request stats.
- Audit/event feed endpoint for control-plane mutation history.
- Router `POST /v1/router/chat/completions` implementation (currently not exposed in router handler).
