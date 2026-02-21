# go-router
Building an open-router clone in golang, This is inspired by [Harkirat Singh's Building Open Router in 4 hours...](https://youtu.be/AuN2KlBqli4?si=6NxEX5xLIVhtflAe)

This repository started with the goal of building an OpenRouter-style system in Go.

Before implementation, multiple monorepo strategies were considered:
	•	Multiple Go modules (one per service)
	•	Workspace-based setup using go work
	•	External monorepo tooling (Nx, Turborepo)
	•	Separate repositories for each backend

After evaluation, the decision was:

Single Go module + multiple binaries under cmd/

Rationale:
	•	Router and control APIs share core domain logic (DB access, auth, billing).
	•	Independent versioning is not required at this stage.
	•	Simpler setup reduces cognitive overhead.
	•	Go modules already provide clean dependency boundaries.
	•	As a solo developer, iteration speed matters more than tooling complexity.

Structure:
```
cmd/
  router/    → OpenAI-compatible routing API
  control/   → Dashboard/admin API
internal/    → Shared domain logic
client/      → Next.js dashboard
```
