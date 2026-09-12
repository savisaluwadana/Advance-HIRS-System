# Advance HRIS

A hybrid local-first Human Resources Information System designed for real internal company operations and as a multi-component OpenChoreo demo.

## Product architecture

```text
Employees / Managers                 HR / Admins
        |                                  |
        v                                  v
Next.js Web Portal                Wails + React Desktop
                                           |
                                  Encrypted local SQLite
                                           |
                                   durable sync queue
        |                                  |
        +---------------+------------------+
                        |
                        v
                 Go Application API
              Auth + RBAC + tenant scope
                        |
                        v
                    PostgreSQL
```

PostgreSQL is the authoritative cloud system of record. The HR/admin desktop saves approved edits locally first and synchronizes them through the same tenant-scoped API.

## Current foundation

- Responsive Next.js HR dashboard
- Native Wails + React HR/admin desktop console
- AES-256-GCM encrypted local employee payloads
- OS keychain-protected encryption key and desktop access token
- Durable offline sync queue with conflict detection
- PostgreSQL persistence
- Organizations / tenant isolation
- Password authentication with bcrypt
- Signed JWT access tokens
- Server-side roles: `admin`, `hr`, `manager`, `employee`
- Employee create, list, update and archive endpoints
- Audit events for employee mutations
- PostgreSQL-backed dashboard, leave and desktop sync endpoints
- CI integration test against PostgreSQL

## Run locally with Docker

The fastest full-stack setup is:

```bash
docker compose up --build
```

Local development bootstraps a demo organization and admin account:

```text
Organization: northstar
Email:        admin@advancehris.local
Password:     local-admin-change-me
```

These credentials exist only for the local Docker configuration. Do not reuse them in a shared or production environment.

Web: `http://localhost:3000`

API: `http://localhost:8080`

## Run the desktop app

Start PostgreSQL/API first, then:

```bash
cd apps/desktop
wails dev
```

Sign in using an organization account. The desktop access token is stored in the operating-system keychain and is never written to SQLite or browser local storage.

## API auth flow

Login:

```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "admin@advancehris.local",
  "password": "local-admin-change-me",
  "organization_slug": "northstar"
}
```

Use the returned token as:

```http
Authorization: Bearer <access_token>
```

Protected endpoints currently include:

- `GET /api/v1/auth/me`
- `GET /api/v1/dashboard`
- `GET /api/v1/employees`
- `POST /api/v1/employees`
- `PATCH /api/v1/employees/{id}`
- `DELETE /api/v1/employees/{id}` — archives rather than hard-deletes
- `GET /api/v1/leave/requests`
- `POST /api/v1/sync/push`
- `GET /api/v1/sync/pull`

## Production configuration

Required API configuration:

- `DATABASE_URL`
- `JWT_SECRET` — minimum 32 characters; inject from a secret store
- `WEB_ORIGIN`

Optional bootstrap configuration for a new environment:

- `BOOTSTRAP_ORG_NAME`
- `BOOTSTRAP_ORG_SLUG`
- `BOOTSTRAP_ADMIN_NAME`
- `BOOTSTRAP_ADMIN_EMAIL`
- `BOOTSTRAP_ADMIN_PASSWORD`
- `SEED_DEMO_DATA`

Do not enable demo seeding or keep bootstrap credentials in production after initial provisioning.

## Next milestones

1. Schema migrations and automated production migration job
2. Refresh-token/session revocation and SSO/OIDC
3. Fine-grained manager/team permissions
4. Conflict-resolution UI + delta sync cursors
5. Employee onboarding/offboarding workflows
6. Leave approvals + attendance/timesheets
7. Payroll and compensation
8. Performance and recruiting workflows
9. Documents, notifications and richer audit reporting
10. Permission-aware AI HR copilot

## OpenChoreo

See `docs/openchoreo.md` for the component split and environment/secret guidance.
