# OpenChoreo deployment model

Advance HRIS is structured as a realistic multi-component workload: a public web experience, a protected Go application API, PostgreSQL, and an optional native HR/admin desktop client using the same API.

## Components

### `hris-web`

- Source: `apps/web`
- Runtime: Node.js / Next.js
- Port: `3000`
- Environment: `NEXT_PUBLIC_API_URL`

### `hris-api`

- Source: `apps/api`
- Runtime: Go
- Port: `8080`
- Health endpoint: `/health`
- Required secrets/configuration:
  - `DATABASE_URL`
  - `JWT_SECRET`
  - `WEB_ORIGIN`
- Optional first-environment bootstrap:
  - `BOOTSTRAP_ORG_NAME`
  - `BOOTSTRAP_ORG_SLUG`
  - `BOOTSTRAP_ADMIN_NAME`
  - `BOOTSTRAP_ADMIN_EMAIL`
  - `BOOTSTRAP_ADMIN_PASSWORD`
  - `SEED_DEMO_DATA`

`JWT_SECRET`, database credentials and bootstrap passwords must be injected as secrets rather than committed to a deployment manifest.

### PostgreSQL

Use managed PostgreSQL or an environment-specific data-plane resource. Apply `db/schema.sql` for the current development foundation; production should move to versioned migrations before the first stable release.

### Desktop

`apps/desktop` is distributed to HR/admin workstations rather than deployed as an OpenChoreo component. It connects to the environment's public/protected API, stores HR data encrypted locally, and keeps its bearer session in the operating-system keychain.

## Security boundaries

- Organization ID is derived from the authenticated token, never trusted from request payloads.
- Employee reads/writes are scoped by organization in SQL.
- `admin` and `hr` may mutate employee records and use desktop sync.
- `manager` currently has directory/dashboard read access only.
- `employee` is authenticated but has no broad workforce-directory access yet.
- Audit events capture employee create/update/archive/sync actions.

## Suggested environments

- `development` — automatic feature-branch deployment, optional demo data
- `staging` — real authentication and integration testing; no demo credentials
- `production` — controlled promotion, managed secrets, no bootstrap/demo seeding

## Recommended OpenChoreo demo flow

1. Deploy web and API as separate components.
2. Attach managed PostgreSQL to the API through a secret `DATABASE_URL`.
3. Inject `JWT_SECRET` and environment-specific CORS origin.
4. Demonstrate login and tenant-scoped employee API calls.
5. Demonstrate the Wails desktop synchronizing through the same API.
6. Promote development to staging without rebuilding application architecture.
7. Add logs, traces and security/audit dashboards before production promotion.
