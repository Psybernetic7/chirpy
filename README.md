# Chirpy

Chirpy is a Go-powered microblogging API that demonstrates an end-to-end user and content workflow: secure account management, JWT-based sessions with refresh tokens, content moderation, webhook-driven upgrades, and a sprinkling of simple admin tooling. It is designed as a compact but realistic reference for building production-style HTTP services in Go.

## Features
- RESTful JSON API for registering users, publishing short-form *chirps*, listing feeds, and managing content deletion.
- Secure authentication flow using Argon2 password hashing, short-lived JWT access tokens, and database-backed refresh tokens.
- Content filtering that refuses banned terms (`kerfuffle`, `sharbert`, `fornax`) before sending chirps.
- A hypothetical Pokla webhook handler that upgrades accounts to **Chirpy Red** when it receives a `user.upgraded` event authenticated with an API key.
- Development-friendly admin endpoints for metrics and database resets, protected by the `PLATFORM` environment flag.
- Static asset hosting at `/app/*`, backed by a hit counter surfaced through `/admin/metrics`.

## Project Layout
- `main.go` – HTTP server, routing, and request handlers.
- `internal/auth` – Password hashing, JWT helpers, refresh-token utilities, and authorization header parsing.
- `internal/database` – `sqlc`-generated PostgreSQL queries and models.
- `sql/schema` – Goose-compatible database migrations for users, chirps, refresh tokens, and premium status.
- `sql/queries` – Parameterized SQL used by `sqlc`.

## Requirements
- Go **1.22+** (the module declares `go 1.25.2` to pick up the newer `http.ServeMux` patterns).
- PostgreSQL 14+.
- [`sqlc`](https://docs.sqlc.dev/en/latest/overview/install.html) v1.30.0 (only needed if you regenerate query code).
- [`goose`](https://github.com/pressly/goose) or an equivalent migration runner for applying the SQL in `sql/schema`.

## Configuration
Chirpy reads configuration from environment variables (loaded automatically from `.env` during local development via `github.com/joho/godotenv`):

| Variable     | Description                                                                                   | Example                                                           |
|--------------|-----------------------------------------------------------------------------------------------|-------------------------------------------------------------------|
| `DB_URL`     | PostgreSQL connection string.                                                                 | `postgres://postgres:postgres@localhost:5432/chirpy?sslmode=disable` |
| `JWT_SECRET` | HS256 signing secret for access tokens. Prefer a 256-bit base64 string.                       | `y/JXKdwj+3XMrbGV6UdHosXDB96cnlLsEuw29Btv5zeM9oPlGVJBSJkCKzizLLyqzYK/pAFb/XtEARgWwAf1ew==` |
| `POLKA_KEY`  | Shared secret used to authenticate incoming Polka webhook requests via the `ApiKey` header.   | `f271c81ff7084ee5b99a5091b42d486e`                                |
| `PLATFORM`   | Controls development-only behaviour (`dev` enables `/admin/reset`).                           | `dev`                                                             |

## Database Setup
1. Create a PostgreSQL database:
   ```sh
   createdb chirpy
   ```
   or by container:
   ```sh
   docker run --name chirpy-db -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres:15
   ```
2. Apply migrations (with goose):
   ```sh
   goose postgres "$DB_URL" up
   ```
   Alternatively, apply the SQL files in `sql/schema` manually using `psql`.

## Running Locally
```sh
go run .
```

The server listens on `:8080`. Static assets are available under `http://localhost:8080/app/` (e.g. `/app/index.html`), and the API is served from `/api/*`.

To rebuild the database access layer after modifying `sql/queries`, run:
```sh
sqlc generate
```

## API Overview
Authorization headers use the standard `Authorization: Bearer <token>` format unless noted.

| Method | Route                         | Description                                                                                       | Auth |
|--------|-------------------------------|---------------------------------------------------------------------------------------------------|------|
| GET    | `/api/healthz`                | Liveness/readiness probe returning `OK`.                                                         | None |
| POST   | `/api/users`                  | Register a new user. Passwords are hashed with Argon2 before storage.                            | None |
| PUT    | `/api/users`                  | Update the authenticated user's email and password.                                              | Bearer access token |
| POST   | `/api/login`                  | Exchange email/password for a JWT access token and a refresh token.                              | None |
| POST   | `/api/refresh`                | Exchange a valid refresh token (sent as a bearer token) for a fresh access token.                | Bearer refresh token |
| POST   | `/api/revoke`                 | Revoke the supplied refresh token.                                                               | Bearer refresh token |
| POST   | `/api/chirps`                 | Publish a chirp (`body` ≤ 140 chars). Banned words are replaced with `****` prior to persistence. | Bearer access token |
| GET    | `/api/chirps`                 | List chirps. Supports `author_id=<uuid>` and `sort=desc` query params.                           | None |
| GET    | `/api/chirps/{chirpID}`       | Fetch a single chirp by ID.                                                                      | None |
| DELETE | `/api/chirps/{chirpID}`       | Delete a chirp you authored.                                                                     | Bearer access token |
| POST   | `/api/polka/webhooks`         | Handle Polka events. Only `user.upgraded` upgrades `is_chirpy_red`. Requires `Authorization: ApiKey <POLKA_KEY>`. | ApiKey header |
| GET    | `/admin/metrics`              | Simple HTML dashboard showing file-server hit counts.                                            | None |
| POST   | `/admin/reset`                | Development-only endpoint that truncates user data and resets metrics when `PLATFORM=dev`.       | None |

### Authentication Flow
1. **Register** via `/api/users`.
2. **Log in** via `/api/login` to receive:
   - A short-lived JWT access token (`expires_in = 1 hour`).
   - A long-lived refresh token stored server-side with an expiry of 60 days.
3. **Authorize API calls** by including the access token in the `Authorization: Bearer` header.
4. **Refresh** the access token by calling `/api/refresh` with the refresh token in the `Authorization` header.
5. **Revoke** compromised or unused refresh tokens through `/api/revoke`.

## Development Notes
- `go test ./...` runs the available unit tests (currently covering `internal/auth`).
- `handlerPolkaWebhooks` is intentionally strict: any unknown event or malformed payload results in `204 No Content` or `400/401` responses to prevent spurious upgrades.
- Static content requests (`/app/*`) increment an in-memory counter stored on the `apiConfig`, surfaced by `/admin/metrics`. The `/admin/reset` endpoint resets both the counter and user data for clean local testing.
- All mutable handlers rely on the `internal/database` package generated by `sqlc`. Avoid hand-editing generated files; update the SQL instead and run `sqlc generate`.

## Useful Commands
- Run tests: `go test ./...`
- Generate SQL stubs: `sqlc generate`
- Build a binary: `go build -o chirpy`

Happy chirping!
