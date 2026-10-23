# FOS KUK

Bank Nano Syariah Financing Origination System for the KUK segment, rebuilt from Pega in Go. What to build is defined by `docs/FOS_KUK_SoT_v1.docx`; vocabulary is in `GLOSSARY.md`; work is tracked under `.scratch/`.

- `backend/`: Go API, PostgreSQL (schema `fos`), migrations applied at startup.
- `frontend/`: Next.js UI. Its middleware maps the configured paths (`LOGIN_PATH_DB`, `ENDPOINT_PATH_DESKTOP`) to pages and proxies `<ENDPOINT_PATH_DESKTOP>/api/*` to the backend.

## Running locally

Requirements: Go 1.26, Node 22, Docker.

```sh
docker run -d --name fos-pg -e POSTGRES_USER=fos -e POSTGRES_PASSWORD=fos -e POSTGRES_DB=fos -p 5432:5432 postgres:16-alpine

cd backend && cp .env.example .env    # export these variables, then:
go run ./cmd/fos

cd frontend && cp .env.example .env.local
npm install && npm run dev            # serves https://localhost:3000 with a self-signed certificate
```

Open `https://localhost:3000/PRWebDB1` (Database login) or `/PRWebLDAP1` (LDAP login). There is no operator administration yet: insert a role into `fos.roles` and an operator into `fos.operator` with a bcrypt `password_hash`.

Every configuration key is required; nothing has a default (SoT ENV-006). On Windows, start servers from PowerShell or set `MSYS_NO_PATHCONV=1`: Git Bash rewrites values such as `/PRWebDB1` into Windows paths.

## Tests

```sh
cd backend && go test ./...           # starts PostgreSQL in Docker via testcontainers
cd frontend && npm run typecheck && npm run build
```

Backend tests run at the HTTP API seam against a real PostgreSQL; external systems are faked at the network boundary.

## Deployment requirement: TLS

External services call the backend directly under `/api/NBS/` (for example the RPA's `POST /api/NBS/v1/PBSResponse`, FR-PSAPI-009). The proxy must route that prefix to the backend, not through Next.js. The RPA account is an operator with role `RPA` and a bcrypt password; every hit on the prefix is written to the Connect API Log.

The backend accepts requests only over HTTPS (FR-LOGIN-004): directly over TLS, or from a proxy listed in `TRUSTED_PROXY_CIDRS` that sends `X-Forwarded-Proto: https`. The Next.js server forwards that header from its client unchanged, so the TLS-terminating proxy in front of Next.js must **overwrite** `X-Forwarded-Proto` and `X-Forwarded-For` on every request.
