# Advance HRIS

A hybrid local-first Human Resources Information System built as a real product and as a reference application for OpenChoreo.

## Product architecture

```text
Employees / Managers
        |
        v
Next.js Web Portal -------------------+
                                      |
HR / Admins                           v
    |                          Go Cloud API
    v                                 |
Wails + React Desktop                 +---- PostgreSQL
    |                                 +---- Object storage
Encrypted SQLite                      +---- Audit / reporting
    |
Durable Sync Queue -------------------+
```

### Why hybrid

- Employees and managers get browser-based self service.
- HR/admin teams get a native desktop console with fast offline access.
- Desktop employee payloads are encrypted locally with AES-256-GCM.
- The local encryption key is stored in the OS keychain.
- Local edits are durable-first: SQLite commit, sync queue, then cloud reconciliation.
- Web and desktop share the same Go API and tenant-aware cloud model.
- The cloud API remains the authoritative system of record.

## Repository

- `apps/web` — Next.js employee/manager web experience
- `apps/api` — Go cloud API, sync contract and application services
- `apps/desktop` — Wails + React HR/admin desktop app
- `db/schema.sql` — PostgreSQL starter model
- `docs/openchoreo.md` — OpenChoreo deployment notes

## Run locally

### API

```bash
cd apps/api
go run ./cmd/server
```

API: `http://localhost:8080`

### Web

```bash
cd apps/web
npm install
npm run dev
```

Web: `http://localhost:3000`

### Desktop

Install Wails v2:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
```

Then:

```bash
cd apps/desktop
wails doctor
wails dev
```

To target another API:

```bash
ADVANCE_HRIS_API_URL=https://api.example.com wails dev
```

## API foundation

- `GET /health`
- `GET /api/v1/dashboard`
- `GET /api/v1/employees`
- `GET /api/v1/leave/requests`
- `POST /api/v1/sync/push`
- `GET /api/v1/sync/pull?since=<cursor>`

The current server still uses seeded in-memory data so the whole product can run immediately. PostgreSQL repositories and tenant/auth enforcement are the next backend milestone.

## Development roadmap

1. PostgreSQL repositories, migrations and tenant boundaries
2. Authentication + RBAC for employee, manager, HR and admin roles
3. Conflict-aware delta sync with per-entity cursors
4. Employee lifecycle + onboarding/offboarding
5. Leave approvals and attendance/timesheets
6. Payroll and compensation
7. Performance reviews and goals
8. Recruitment pipeline
9. Audit log, documents and notifications
10. Permission-aware AI HR copilot
