# OpenChoreo deployment model

Advance HRIS is structured so the application demonstrates a realistic multi-component workload on OpenChoreo.

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
- Environment: `API_PORT`, `DATABASE_URL`

### PostgreSQL

Use a managed PostgreSQL database or an environment-specific data plane resource. Keep `DATABASE_URL` out of source control and inject it as a secret/configuration value.

## Suggested environments

- `development` — auto deploy from the feature branch for demos
- `staging` — integration testing and product review
- `production` — controlled promotion only

## Recommended next OpenChoreo demo flows

1. Deploy web and API as separate components.
2. Expose the API internally to the web component and expose only the web publicly.
3. Configure environment-specific API URLs and database credentials.
4. Demonstrate build/deploy promotion from development to staging.
5. Add logs, metrics and traces before production promotion.
6. Add an MCP-facing HRIS API only after RBAC and audit logging are enforced.
