# Architecture

## System overview

```text
Employees / Managers                       HR / Admins
        |                                      |
        v                                      v
Next.js web portal                    Wails + React desktop
        |                                      |
        |                              encrypted local SQLite
        |                                      |
        |                               durable sync queue
        +------------------+-------------------+
                           |
                           v
                      Go HTTP API
                auth + RBAC + tenant scope
                           |
                           v
                       PostgreSQL
```

PostgreSQL is the authoritative cloud system of record. The desktop application is local-first for HR/admin employee data, but synchronization still passes through the same authenticated API and tenant rules as the web application.

## Components

### Web

Path: `apps/web`

Technology: Next.js 16 / React 19.

Responsibilities:

- interactive employee/manager/HR portal;
- login and invitation onboarding;
- server-side BFF routes under `/api/*`;
- HttpOnly session cookie storage;
- leave and attendance workspaces;
- HR/admin administration surfaces;
- public product/developer documentation at `/docs`.

The browser does not need to hold the bearer access token. The Next.js route handlers read the HttpOnly cookie and attach the bearer token when proxying to the Go API.

### API

Path: `apps/api`

Technology: Go 1.26, net/http, pgx.

Responsibilities:

- authentication and token issuance;
- authorization by role;
- organization/tenant scoping;
- employee CRUD;
- invitation workflows;
- leave policy/balance/request workflows;
- attendance schedules, timekeeping, corrections and summaries;
- audit events;
- desktop sync endpoints;
- readiness/health endpoint.

### PostgreSQL

Path: `db`

PostgreSQL stores organizations, users, memberships, employees, leave, attendance, jobs and audit data.

Important identity rule:

```text
employee identity = (organization_id, employee_id)
```

Employee IDs are intentionally reusable in separate organizations. Queries and foreign keys must keep organization ID in the boundary.

### Desktop

Path: `apps/desktop`

Technology: Wails + React + Go.

Responsibilities:

- HR/admin native workspace;
- encrypted local employee storage;
- OS-keychain-backed session token;
- queued offline employee mutations;
- sync push/pull;
- cloud leave/attendance/audit operations.

Managers and employees should use the web portal rather than the native HR console.

## Authentication

1. User posts email/password/organization slug to the API.
2. API resolves a membership and validates bcrypt password hash.
3. API issues an HS256 JWT with user, organization and role claims.
4. Web BFF stores that JWT in an HttpOnly, SameSite=Lax cookie.
5. Web API proxy reads the cookie server-side and forwards the bearer token.
6. API middleware validates issuer, expiration, signature and role.

Production requires a secret `JWT_SECRET` with at least 32 characters. It must come from a secret manager, not source control.

## Authorization model

Roles:

| Role | Main permissions |
| --- | --- |
| admin | organization-wide HR actions, access administration, employee management, audit, leave/attendance admin |
| hr | employee management, employee/manager invitations, leave/attendance admin, audit |
| manager | self service plus direct-report leave/attendance review |
| employee | self-service leave and attendance |

Organization ID is always derived from the authenticated principal. Request payloads are not allowed to choose a tenant.

## Leave model

Leave requests resolve to a policy. Tracked policies use an append-only balance ledger.

Approval is transactional:

1. lock/validate the request;
2. calculate chargeable working days;
3. evaluate balance;
4. post debit when required;
5. mark request approved;
6. emit audit event.

Cancellation of approved tracked leave posts a reversing ledger credit rather than rewriting balance history.

## Attendance model

Attendance is based on effective-dated work schedules.

At check-in the resolved schedule is snapshotted onto the attendance entry, including scheduled timestamps, break and overtime threshold. Historical records therefore do not change when a schedule is edited later.

Corrections are approval-based. Approved corrections recompute time metrics in the same transaction.

## Local-first synchronization

The desktop stores employee records locally and queues mutations.

A queued mutation carries the base server version. During sync:

- safe updates advance the server version;
- a stale base version is returned as a conflict;
- pending local edits are not silently overwritten by a pull.

The current desktop foundation detects conflicts; a full human conflict-resolution UI remains a future layer.

## Trust boundaries

- Browser: untrusted; bearer tokens are not exposed to browser JavaScript.
- Next.js BFF: trusted web-session boundary.
- API: authorization and tenant enforcement boundary.
- PostgreSQL: authoritative state.
- Desktop local store: encrypted cache, not a replacement for tenant validation.
- Demo credentials: development only.
