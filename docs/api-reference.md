# API reference

Base path: `/api/v1`

The API returns JSON. Protected endpoints require:

```http
Authorization: Bearer <access_token>
```

The web application normally reaches protected endpoints through its same-origin BFF path `/api/hr/*`.

## Error shape

Typical error:

```json
{
  "error": {
    "code": "validation_error",
    "message": "..."
  }
}
```

Clients should use HTTP status for broad behavior and `error.code` for workflow-specific handling.

## Authentication

### POST /auth/login

```json
{
  "email": "admin@advancehris.local",
  "password": "local-admin-change-me",
  "organization_slug": "northstar"
}
```

Returns access token, expiry and principal.

### GET /auth/me

Roles: admin, hr, manager, employee.

Returns the authenticated principal.

## Health

### GET /health

Public. Returns service/database readiness.

## Dashboard and employees

### GET /dashboard

Roles: admin, hr, manager.

### GET /employees

Roles: admin, hr, manager.

### POST /employees

Roles: admin, hr.

Required fields include first name, last name and work email.

### PATCH /employees/{id}

Roles: admin, hr.

### DELETE /employees/{id}

Roles: admin, hr.

This archives the employee rather than physically deleting the record.

## Access invitations

### GET /access/invitations

Roles: admin, hr.

### POST /access/invitations

Roles: admin, hr.

HR may grant employee/manager access. Admin is required to grant HR/admin access.

### POST /access/invitations/{id}/revoke

Roles: admin, hr.

### POST /access/invitations/inspect

Public token-bound endpoint.

### POST /access/invitations/accept

Public token-bound endpoint.

Raw invitation tokens are never persisted. PostgreSQL stores the SHA-256 digest.

## Leave

### GET /leave/policies

Roles: all authenticated roles, scope enforced.

### PUT /leave/policies

Roles: admin, hr.

### POST /leave/policies/{id}/assignments

Roles: admin, hr.

### GET /leave/balances

Roles: all authenticated roles, scope enforced.

### GET /leave/balances/ledger

Roles: all authenticated roles, scope enforced.

### POST /leave/balances/adjustments

Roles: admin, hr.

### GET /leave/holidays

Roles: all authenticated roles.

### POST /leave/holidays

Roles: admin, hr.

### DELETE /leave/holidays/{id}

Roles: admin, hr.

### GET /leave/requests

Roles: all authenticated roles.

Scope:

- admin/hr: organization;
- manager: self + direct reports;
- employee: self.

### POST /leave/requests

Roles: all authenticated roles.

Non-HR roles may submit only for their own linked employee profile.

### POST /leave/requests/{id}/decision

Roles: admin, hr, manager.

Managers can decide only direct-report requests.

### POST /leave/requests/{id}/cancel

Roles: all authenticated roles with ownership/scope checks.

## Attendance

### GET /attendance/schedules

Roles: all authenticated roles.

### PUT /attendance/schedules

Roles: admin, hr.

### POST /attendance/schedules/{id}/assignments

Roles: admin, hr.

### GET /attendance?date=YYYY-MM-DD

Roles: all authenticated roles.

Scope follows self/direct-report/organization rules.

### POST /attendance/check-in

Roles: all authenticated roles.

Example:

```json
{
  "employee_id": "emp_001",
  "work_mode": "remote"
}
```

For manager/employee self-service, the server ignores attempts to act for another employee. HR/admin must supply the employee ID.

### POST /attendance/check-out

Roles: all authenticated roles.

Checkout closes only the relevant current-day open record.

### GET /attendance/corrections

Roles: all authenticated roles.

### POST /attendance/corrections

Roles: all authenticated roles.

At least one corrected timestamp and a reason are required.

### POST /attendance/corrections/{id}/decision

Roles: admin, hr, manager.

### GET /attendance/timesheet

Roles: all authenticated roles, scoped.

Used for payroll-ready period summaries.

## Audit

### GET /audit?limit=50

Roles: admin, hr.

## Desktop sync

### POST /sync/push

Roles: admin, hr.

Pushes queued employee mutations with base server versions.

### GET /sync/pull

Roles: admin, hr.

Returns server state for reconciliation.

## Security expectations for API consumers

- Never trust organization IDs from clients.
- Do not store access tokens in localStorage.
- Treat invitation tokens as secrets.
- Use HTTPS outside local development.
- Rotate `JWT_SECRET` through a secret-management process.
- Enforce network/database backups separately from application authorization.
