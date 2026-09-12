# Advance HRIS

A modern, OpenChoreo-friendly Human Resources Information System for internal company operations and platform demos.

## Product direction

Advance HRIS is being built as a real modular HR platform rather than a static dashboard. The first foundation includes:

- People directory and employee lifecycle
- Attendance and time tracking
- Leave management
- Payroll overview
- Performance and goals
- Recruitment pipeline
- Organization roles and access foundations
- Workforce insights / AI-ready workflows

## Architecture

```text
Browser
  |
  v
Next.js Web App (apps/web)
  |
  v
Go HTTP API (apps/api)
  |
  v
PostgreSQL (db/schema.sql)
```

The web and API are intentionally independent deployable components so the system can be demonstrated cleanly on OpenChoreo.

## Run locally

### 1. API

```bash
cd apps/api
go run ./cmd/server
```

API: `http://localhost:8080`

### 2. Web

```bash
cd apps/web
npm install
npm run dev
```

Web: `http://localhost:3000`

Or run the full stack with Docker:

```bash
docker compose up --build
```

## Initial API

- `GET /health`
- `GET /api/v1/dashboard`
- `GET /api/v1/employees`
- `GET /api/v1/leave/requests`

The first API uses seeded demo data so the product is immediately runnable. `db/schema.sql` establishes the PostgreSQL model that the next implementation pass will wire to repositories and authentication.

## OpenChoreo

See `docs/openchoreo.md` for the intended component split and environment configuration.

## Roadmap

1. Auth, tenants and RBAC
2. PostgreSQL repositories + migrations
3. Employee CRUD and onboarding
4. Leave approval workflows and attendance ingestion
5. Payroll runs and compensation
6. Goals, reviews and recruiting workflows
7. Audit logs, notifications and integrations
8. AI HR copilot with permission-aware actions
