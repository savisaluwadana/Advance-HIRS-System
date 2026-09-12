# Advance HRIS

A hybrid local-first Human Resources Information System designed for real internal company operations and as a multi-component OpenChoreo demo.

## Product architecture

```text
Employees / Managers                 HR / Admins
        |                                  |
        v                                  v
Next.js Web Portal                Wails + React Desktop
        |                                  |
        |                         Encrypted local SQLite
        |                                  |
        |                          durable sync queue
        +---------------+------------------+
                        |
                        v
                 Go Application API
            Auth + RBAC + tenant scope
                        |
                        v
                    PostgreSQL
```

PostgreSQL is the authoritative cloud system of record. Employee/manager self-service runs through the web portal, while HR/admins can also use the native local-first desktop console.

## Current product foundation

- Responsive role-aware Next.js portal
- Native Wails + React HR/admin desktop console
- PostgreSQL persistence with organization/tenant isolation
- bcrypt password authentication + signed JWT access tokens
- Web sessions stored in HttpOnly/SameSite cookies through a Next.js BFF
- Roles: `admin`, `hr`, `manager`, `employee`
- Employee create, update, archive and directory workflows
- Policy-backed leave management with balances, carry-over and company holidays
- Employee-specific leave policy variants and manager approvals
- Work schedules with timezone, work days, breaks, grace periods and overtime thresholds
- Employee-specific effective-dated schedule assignments
- Schedule-aware attendance with persisted late, early-leave, worked and overtime minutes
- Attendance correction requests with manager/HR approval
- Payroll-period attendance summaries
- Organization audit history
- Secure one-time organization invitations
- HR/admin access administration at `/access`
- Invitation acceptance/onboarding at `/accept?token=...`
- AES-256-GCM encrypted desktop employee payloads
- OS-keychain-protected desktop session token
- Durable offline desktop sync queue with conflict detection
- CI integration tests against PostgreSQL for RBAC, invitations, leave accounting, attendance accounting, audit and tenant isolation

## Run locally with Docker

```bash
docker compose up --build
```

Local development bootstraps:

```text
Organization: northstar
Admin:        admin@advancehris.local
Password:     local-admin-change-me
Manager:      sara@northstar.local
Employee:     ava@northstar.local
Demo password: local-demo-password-2026
```

These accounts are local/demo-only. Never enable demo seeding or reuse these credentials in a shared or production environment.

Web: `http://localhost:3000`

Leave workspace: `http://localhost:3000/leave`

Leave administration: `http://localhost:3000/leave/admin`

Attendance workspace: `http://localhost:3000/attendance`

Attendance administration: `http://localhost:3000/attendance/admin`

Access administration: `http://localhost:3000/access`

API: `http://localhost:8080`

## Leave Management v2

Leave requests are policy-backed rather than being treated as standalone rows.

Tracked policies use an append-only balance ledger. Yearly entitlement and capped carry-over entries are provisioned idempotently. Approving a tracked leave request validates the employee's available balance and posts a debit inside the same PostgreSQL transaction as the approval. Cancelling approved leave posts a reversing credit and marks the request cancelled in the same transaction.

Default policies are provisioned per organization when first needed:

- Annual leave — 20 days, up to 5 days carry-over
- Sick leave — 10 days, no carry-over
- Parental leave — untracked by default
- Unpaid leave — untracked
- Remote work — untracked
- Other leave — untracked

HR/admins can customize entitlement, carry-over and negative-balance behavior from `/leave/admin`, create policy variants, assign policies to employees, post auditable manual adjustments, and manage company/location holidays.

The request engine excludes weekends and matching holidays from day calculations. Half-day requests count as `0.5` and must be a single calendar date. PostgreSQL prevents overlapping pending/approved leave for the same employee, including concurrent submissions.

## Attendance Management v2

Attendance is schedule-aware. Each organization gets a default work schedule when first needed, and HR/admins can create additional schedules with their own timezone, working days, shift times, break duration, late grace period and overtime threshold.

Employee-specific schedule assignments are effective-dated. When an employee checks in, Advance HRIS snapshots the resolved schedule onto that attendance row. This means changing a schedule later does not retroactively change historical payroll calculations.

Attendance rows persist:

- scheduled start/end
- check-in/check-out
- break minutes
- late minutes after grace
- early-leave minutes
- net worked minutes
- overtime minutes after the configured threshold
- work mode and source (`self_service`, `hr_override`, `correction`, or `sync`)

Employees can submit correction requests when a time record is wrong or missing. Managers may approve corrections only for direct reports; HR/admin can review organization-wide corrections. Approval rewrites the attendance record and recomputes schedule metrics in one PostgreSQL transaction.

`/attendance` provides employee/manager timekeeping, current-month metrics, correction submission and approval queues. `/attendance/admin` provides schedule configuration, employee assignments, HR correction review and payroll-period summary generation.

The period summary endpoint returns scheduled days, recorded days, worked minutes/hours, overtime, late minutes and early-leave minutes. This is the timekeeping boundary intended to feed the future payroll module.

## Secure invitation flow

1. Sign in as an `admin` or `hr` user.
2. Open `/access`.
3. Choose a role and, for employee/manager access, link an employee profile.
4. Advance HRIS creates a random one-time token that expires after 48 hours.
5. Only the SHA-256 hash is persisted in PostgreSQL; the raw token is returned once.
6. Share the generated `/accept?token=...` link through a trusted channel.
7. The acceptance page removes the token from the browser URL after capture and exchanges it through POST requests.
8. Successful acceptance consumes the invitation and creates the normal role-scoped HRIS web session.

Security boundaries:

- HR can grant `employee` or `manager` access.
- Only an admin can grant `hr` or `admin` access.
- Employee/manager invitation emails must match the linked employee work email.
- Reissuing an invitation invalidates the prior pending token for that email.
- Existing users can join another organization without having their password overwritten.

## Run the desktop app

Start PostgreSQL/API first, then:

```bash
cd apps/desktop
wails dev
```

The desktop application accepts HR/admin accounts only. Its cloud access token is stored in the operating-system keychain and is never written to SQLite or browser local storage.

## API auth and workflow endpoints

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

Core protected endpoints include:

- `GET /api/v1/auth/me`
- `GET /api/v1/dashboard`
- `GET|POST /api/v1/employees`
- `PATCH|DELETE /api/v1/employees/{id}`
- `GET|POST /api/v1/leave/requests`
- `POST /api/v1/leave/requests/{id}/decision`
- `POST /api/v1/leave/requests/{id}/cancel`
- `GET /api/v1/leave/policies`
- `PUT /api/v1/leave/policies`
- `POST /api/v1/leave/policies/{id}/assignments`
- `GET /api/v1/leave/balances`
- `GET /api/v1/leave/balances/ledger`
- `POST /api/v1/leave/balances/adjustments`
- `GET|POST /api/v1/leave/holidays`
- `DELETE /api/v1/leave/holidays/{id}`
- `GET /api/v1/attendance/schedules`
- `PUT /api/v1/attendance/schedules`
- `POST /api/v1/attendance/schedules/{id}/assignments`
- `GET /api/v1/attendance`
- `POST /api/v1/attendance/check-in`
- `POST /api/v1/attendance/check-out`
- `GET|POST /api/v1/attendance/corrections`
- `POST /api/v1/attendance/corrections/{id}/decision`
- `GET /api/v1/attendance/timesheet`
- `GET /api/v1/audit`
- `GET|POST /api/v1/access/invitations`
- `POST /api/v1/access/invitations/{id}/revoke`
- `POST /api/v1/sync/push`
- `GET /api/v1/sync/pull`

Public token-bound onboarding endpoints:

- `POST /api/v1/access/invitations/inspect`
- `POST /api/v1/access/invitations/accept`

## Database migrations

Fresh environments should initialize `db/schema.sql`.

Existing environments should apply migrations in order:

```bash
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/migrations/002_tenant_scoped_employee_ids.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/migrations/003_leave_management_v2.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/migrations/004_attendance_v2.sql
```

Migration 002 removes the legacy global uniqueness constraint from `employees.id`. Employee identity is scoped by `(organization_id, id)`.

Migration 003 introduces leave policies, policy assignments, holidays, the balance ledger, half-day/cancellation fields and the database overlap constraint.

Migration 004 introduces work schedules, effective-dated schedule assignments, attendance metric snapshots and attendance correction requests.

## Production configuration

The API uses Go 1.26. The Docker build and a dedicated CI workflow verify the same toolchain used by container deployment.

Required API configuration:

- `DATABASE_URL`
- `JWT_SECRET` — minimum 32 characters; inject from a secret store
- `WEB_ORIGIN`

Recommended web configuration:

- `API_INTERNAL_URL` — server-side route-handler URL for the Go API

Optional bootstrap/demo configuration:

- `BOOTSTRAP_ORG_NAME`
- `BOOTSTRAP_ORG_SLUG`
- `BOOTSTRAP_ADMIN_NAME`
- `BOOTSTRAP_ADMIN_EMAIL`
- `BOOTSTRAP_ADMIN_PASSWORD`
- `SEED_DEMO_DATA`
- `SEED_DEMO_PASSWORD`

## Next development layers

1. Automated/versioned production migration runner
2. Refresh-token/session revocation + SSO/OIDC
3. Payroll and compensation
4. Performance goals/reviews
5. Recruiting pipeline
6. Documents and notifications
7. Desktop conflict-resolution UI + delta sync
8. Permission-aware AI HR copilot

## OpenChoreo

See `docs/openchoreo.md` for the component split and environment/secret guidance.
