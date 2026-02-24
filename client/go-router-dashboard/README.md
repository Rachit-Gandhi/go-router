# Go Router Dashboard

Next.js (App Router, TypeScript, Tailwind) frontend for the Go auth backend in this repository.

## What this frontend includes

- OpenRouter-style landing page.
- Header with only a `Sign up` button.
- Signup/login modal form.
- Next API proxy routes:
  - `POST /api/signup` -> Go `POST /users`
  - `POST /api/login` -> Go `POST /login`
- Backend errors are shown as-is in the form UI.

## Environment setup

Create `client/go-router-dashboard/.env.local`:

```env
CONTROL_API_BASE_URL=http://localhost:8080
```

Replace the host/port with the values used by your Go control server.

## Run locally

1. Start Go control server from repo root:
   - `go run ./cmd/control`
2. Start frontend:
   - `cd client/go-router-dashboard`
   - `bun install`
   - `bun dev`
3. Open `http://localhost:3000`.

## Notes

- Backend code and models are not modified by this frontend.
- Login success is plain text (`Login successful`) and backend error messages are plain text from `http.Error`; this app surfaces those raw strings directly.
