# Advance HRIS documentation

This directory is the operator and developer handbook for Advance HRIS. The same core guide is also exposed inside the web application at `/docs`.

## Start here

- [Architecture](architecture.md) — components, trust boundaries, data flows, tenancy, local-first desktop model.
- [Development and local operation](development.md) — one-command Docker startup, manual development, environment variables, test commands and reset procedures.
- [API reference](api-reference.md) — authentication, roles, endpoint catalog, examples and response conventions.
- [Production readiness](production-readiness.md) — deployment checklist, security expectations, backups, migrations, observability and known product boundaries.
- [OpenChoreo deployment](openchoreo.md) — component split and OpenChoreo-specific deployment guidance.

## Product surface

Advance HRIS currently includes:

- multi-tenant organizations and role-based access;
- secure login through signed JWT access tokens;
- HttpOnly web sessions through a Next.js BFF;
- employee directory management for HR/admin;
- invitation-based onboarding;
- policy-backed leave with balances, carry-over, holidays and approvals;
- schedule-aware attendance with corrections and payroll-ready time summaries;
- audit history;
- a Wails desktop console with encrypted local employee storage and a durable sync queue.

The following are deliberately treated as future product layers rather than partially implemented screens:

- payroll and compensation;
- performance management;
- recruiting pipeline;
- employee document storage/notifications;
- refresh-token revocation and enterprise SSO/OIDC;
- desktop conflict-resolution UI;
- permission-aware AI copilot.

## Fastest MVP start

```bash
docker compose up --build
```

Then open:

- Web: http://localhost:3000
- Docs: http://localhost:3000/docs
- API health: http://localhost:8080/health

Demo credentials are documented in [development.md](development.md). They are for local use only.
