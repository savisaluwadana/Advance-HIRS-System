# Development and local operation

## Prerequisites

For the simplest path:

- Docker Engine / Docker Desktop;
- Docker Compose v2.

For manual development:

- Go 1.26 for `apps/api`;
- Node.js 22 for `apps/web`;
- Go 1.25 + Wails tooling for `apps/desktop`;
- PostgreSQL 17;
- npm.

## One-command MVP

From the repository root:

```bash
docker compose up --build
```

The Compose dependency chain is intentional:

```text
postgres healthy
      |
      v
schema + migrations complete
      |
      v
API /health returns OK
      |
      v
web starts
```

The database migration container runs `db/migrate.sh`. The script first applies the current idempotent schema and then every SQL migration in `db/migrations`. Running it repeatedly is supported.

### Local URLs

- Web portal: http://localhost:3000
- Documentation: http://localhost:3000/docs
- Leave: http://localhost:3000/leave
- Leave admin: http://localhost:3000/leave/admin
- Attendance: http://localhost:3000/attendance
- Attendance admin: http://localhost:3000/attendance/admin
- Access admin: http://localhost:3000/access
- API health: http://localhost:8080/health

### Demo accounts

Organization slug: `northstar`

| Role | Email | Password |
| --- | --- | --- |
| admin | admin@advancehris.local | local-admin-change-me |
| manager | sara@northstar.local | local-demo-password-2026 |
| employee | ava@northstar.local | local-demo-password-2026 |

These credentials exist only to make the local MVP immediately usable. Do not expose this configuration on a shared network or deployment.

## Compose host ports

Optional host-port overrides:

```bash
HRIS_WEB_PORT=3100 HRIS_API_PORT=8180 HRIS_POSTGRES_PORT=55432 docker compose up --build
```

## Stop

```bash
docker compose down
```

## Reset all local database data

```bash
docker compose down -v
docker compose up --build
```

The `-v` removes the named PostgreSQL data volume.

## Inspect health

```bash
curl http://localhost:8080/health
docker compose ps
docker compose logs -f api
docker compose logs -f web
docker compose logs -f migrate
```

## Manual API development

Start PostgreSQL, export configuration, then:

```bash
cd apps/api
go mod tidy
go test ./...
go run ./cmd/server
```

Required API variables:

- `DATABASE_URL`
- `JWT_SECRET`
- `WEB_ORIGIN`

Bootstrap variables are optional.

## Manual web development

```bash
cd apps/web
npm install
API_INTERNAL_URL=http://localhost:8080 WEB_COOKIE_SECURE=false npm run dev
```

`WEB_COOKIE_SECURE=false` is appropriate only for local HTTP. Production HTTPS should leave it unset or set it to `true`.

## Desktop development

Start PostgreSQL/API first.

```bash
cd apps/desktop
wails dev
```

The desktop application accepts only admin/HR sessions.

## Database migrations

Apply all current migrations with:

```bash
DATABASE_URL='postgres://...' sh db/migrate.sh
```

Existing individual migrations are retained for traceability.

## Tests

### API

```bash
cd apps/api
go test ./...
go build ./cmd/server
```

Integration tests require PostgreSQL.

### Web

```bash
cd apps/web
npm install
npm run build
```

### Desktop UI

```bash
cd apps/desktop/frontend
npm install
npm run build
```

### Desktop Go core

```bash
cd apps/desktop
go test ./internal/...
```

CI runs these surfaces separately.

## Common problems

### Login succeeds but subsequent web calls say unauthorized

Check `WEB_COOKIE_SECURE`. Local HTTP Docker runs must use `false`. HTTPS deployments should use `true`.

### API cannot reach PostgreSQL

In Docker, the database hostname is `postgres`, not `localhost`.

### Old local data behaves differently after a code update

Run:

```bash
docker compose up --build
```

The migration service runs before the API. If you intentionally want a fresh demo database, reset the volume with `docker compose down -v`.

### Port already in use

Override host ports with `HRIS_WEB_PORT`, `HRIS_API_PORT` and `HRIS_POSTGRES_PORT`.

### Desktop says the native bridge is unavailable

The React frontend must be opened through Wails, not as a standalone browser page.
