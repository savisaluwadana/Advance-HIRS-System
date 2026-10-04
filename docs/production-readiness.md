# Production readiness

Advance HRIS has a solid MVP foundation, but production deployment requires operational controls around it.

## Required before production

### Secrets

- generate a high-entropy `JWT_SECRET` of at least 32 characters;
- use a secret manager;
- remove all demo bootstrap passwords;
- set `SEED_DEMO_DATA=false`;
- do not commit real database credentials.

### HTTPS and cookies

- terminate HTTPS at the ingress/load balancer;
- use `WEB_COOKIE_SECURE=true` or leave it on the production default;
- keep SameSite and HttpOnly protections;
- set `WEB_ORIGIN` to the exact deployed web origin.

### Database

- use managed PostgreSQL or a backed-up production cluster;
- run migrations as an explicit deployment step;
- test restore procedures;
- monitor storage growth and connection saturation;
- keep database access private to application/network boundaries.

The local Compose migration service is designed for deterministic MVP startup. Production promotion should run the same SQL through a controlled migration job rather than relying on an application container restart.

### Backups

At minimum:

- automated daily logical or provider snapshots;
- retention policy;
- encrypted backup storage;
- periodic restore drill;
- documented recovery point/recovery time objectives.

### Observability

Collect:

- API request count, latency and 5xx rate;
- PostgreSQL availability/latency;
- login failure rate;
- invitation failures;
- leave/attendance workflow errors;
- migration success/failure;
- container restarts;
- audit-event volume.

Add structured logs and traces before high-scale rollout.

### Security review

Validate:

- tenant-isolation queries;
- manager direct-report boundaries;
- invitation privilege escalation rules;
- brute-force/rate-limit controls at ingress;
- dependency/container vulnerability scans;
- database TLS where applicable;
- secret rotation procedure;
- session expiry behavior.

Current access tokens are stateless and expire after eight hours. Refresh-token revocation, user session management and SSO/OIDC are future layers.

## Deployment health gates

A deployment should not receive traffic until:

1. PostgreSQL is reachable;
2. migrations complete successfully;
3. API `/health` returns HTTP 200;
4. web server is healthy;
5. a smoke test can authenticate and call `/api/v1/auth/me`.

## Data and tenancy invariants

Do not break these invariants:

- tenant comes from authenticated membership;
- employee identity is composite `(organization_id, id)`;
- manager decisions are restricted to direct reports;
- HR cannot grant admin/HR access;
- invitation email must match linked employee email for employee/manager roles;
- approved leave ledger entries are append-only/reversible, not silently rewritten;
- attendance historical schedule snapshots remain stable.

## CI expectations

Pull requests should pass:

- Next.js production build;
- API Go tests/build against the API's declared Go version;
- PostgreSQL integration tests;
- migration idempotency check;
- desktop React build;
- desktop core tests;
- API container build.

## Known product boundaries

These are not complete product modules yet:

- payroll/compensation;
- performance goals/reviews;
- recruitment pipeline;
- employee document repository and notification delivery;
- enterprise SSO/OIDC and refresh-token revocation;
- full desktop conflict-resolution UI;
- AI HR copilot.

Do not expose placeholder navigation as if these features were implemented.

## Recommended next engineering sequence

1. refresh-token/session revocation and SSO/OIDC;
2. rate limiting and abuse controls;
3. structured logs/metrics/traces;
4. automated backup/restore validation;
5. payroll domain built on attendance period summaries;
6. document storage and notification provider abstraction;
7. desktop conflict-resolution UX;
8. performance/recruiting domains;
9. permission-aware AI copilot only after domain permissions are stable.
