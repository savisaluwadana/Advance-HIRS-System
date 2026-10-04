import type { Metadata } from "next";
import styles from "./docs.module.css";

export const metadata: Metadata = {
  title: "Documentation | Advance HRIS",
  description: "Product, developer, API, Docker and production documentation for Advance HRIS.",
};

const endpointGroups = [
  {
    name: "Authentication & access",
    items: [
      "POST /api/v1/auth/login",
      "GET /api/v1/auth/me",
      "GET|POST /api/v1/access/invitations",
      "POST /api/v1/access/invitations/{id}/revoke",
      "POST /api/v1/access/invitations/inspect",
      "POST /api/v1/access/invitations/accept",
    ],
  },
  {
    name: "People & leave",
    items: [
      "GET|POST /api/v1/employees",
      "PATCH|DELETE /api/v1/employees/{id}",
      "GET|PUT /api/v1/leave/policies",
      "GET|POST /api/v1/leave/requests",
      "POST /api/v1/leave/requests/{id}/decision",
      "POST /api/v1/leave/requests/{id}/cancel",
      "GET /api/v1/leave/balances",
      "GET /api/v1/leave/balances/ledger",
      "POST /api/v1/leave/balances/adjustments",
      "GET|POST /api/v1/leave/holidays",
    ],
  },
  {
    name: "Attendance & operations",
    items: [
      "GET|PUT /api/v1/attendance/schedules",
      "POST /api/v1/attendance/schedules/{id}/assignments",
      "GET /api/v1/attendance",
      "POST /api/v1/attendance/check-in",
      "POST /api/v1/attendance/check-out",
      "GET|POST /api/v1/attendance/corrections",
      "POST /api/v1/attendance/corrections/{id}/decision",
      "GET /api/v1/attendance/timesheet",
      "GET /api/v1/audit",
      "POST /api/v1/sync/push",
      "GET /api/v1/sync/pull",
    ],
  },
];

const roles = [
  ["Admin", "Organization-wide HR operations, access administration, employee management, audit, leave and attendance administration."],
  ["HR", "Employee management, employee/manager invitations, leave and attendance administration, audit and desktop sync."],
  ["Manager", "Self service plus direct-report leave and attendance review."],
  ["Employee", "Self-service leave and attendance."],
];

const productionChecklist = [
  "Use HTTPS and keep secure HttpOnly cookies enabled.",
  "Inject JWT_SECRET and database credentials from a secret manager.",
  "Disable demo seeding and remove local demo credentials.",
  "Run database migrations as an explicit deployment gate.",
  "Use managed/backed-up PostgreSQL and test restores.",
  "Collect API, database, auth, workflow and container health telemetry.",
  "Keep tenant isolation and manager direct-report checks in every new workflow.",
];

export default function DocsPage() {
  return (
    <main className={styles.shell}>
      <header className={styles.hero}>
        <nav className={styles.topnav}>
          <a className={styles.brand} href="/"><span>A</span><strong>Advance HRIS</strong></a>
          <div><a href="#quickstart">Quick start</a><a href="#architecture">Architecture</a><a href="#api">API</a><a href="#production">Production</a><a href="/">Open app</a></div>
        </nav>
        <div className={styles.heroGrid}>
          <div>
            <p className={styles.kicker}>PRODUCT + DEVELOPER DOCUMENTATION</p>
            <h1>Build, run and operate Advance HRIS with one source of truth.</h1>
            <p className={styles.lead}>Advance HRIS is a multi-tenant HR platform with a Next.js employee portal, Go API, PostgreSQL system of record and a Wails local-first HR desktop console.</p>
            <div className={styles.actions}><a className={styles.primary} href="#quickstart">Run the MVP</a><a className={styles.secondary} href="https://github.com/savisaluwadana/Advance-HIRS-System">View repository</a></div>
          </div>
          <aside className={styles.commandCard}>
            <span>ONE-COMMAND MVP</span>
            <code>docker compose up --build</code>
            <small>PostgreSQL → migrations → API health → web health</small>
          </aside>
        </div>
      </header>

      <section className={styles.stats}>
        <article><strong>4</strong><span>roles</span></article>
        <article><strong>3</strong><span>runtime surfaces</span></article>
        <article><strong>2</strong><span>core HR workflows</span></article>
        <article><strong>1</strong><span>tenant-scoped API boundary</span></article>
      </section>

      <section className={styles.section} id="quickstart">
        <div className={styles.sectionHeading}><p>01 · QUICK START</p><h2>Run the full local MVP</h2></div>
        <div className={styles.twoCol}>
          <article className={styles.card}>
            <h3>Start</h3>
            <pre><code>{`docker compose up --build`}</code></pre>
            <p>The Compose stack waits for PostgreSQL, applies the current schema and SQL migrations, then starts the API only after migrations succeed. The web service starts only after the API passes its health check.</p>
            <div className={styles.links}><a href="http://localhost:3000">Web · localhost:3000</a><a href="http://localhost:8080/health">API health · localhost:8080/health</a></div>
          </article>
          <article className={styles.card}>
            <h3>Local demo accounts</h3>
            <div className={styles.table}>
              <div className={styles.tr}><strong>Admin</strong><code>admin@advancehris.local</code><code>local-admin-change-me</code></div>
              <div className={styles.tr}><strong>Manager</strong><code>sara@northstar.local</code><code>local-demo-password-2026</code></div>
              <div className={styles.tr}><strong>Employee</strong><code>ava@northstar.local</code><code>local-demo-password-2026</code></div>
            </div>
            <p className={styles.warning}>Local/demo credentials must never be reused in a shared, staging or production environment.</p>
          </article>
        </div>
        <div className={styles.note}><strong>Reset local data</strong><code>docker compose down -v && docker compose up --build</code></div>
      </section>

      <section className={styles.section} id="architecture">
        <div className={styles.sectionHeading}><p>02 · ARCHITECTURE</p><h2>Hybrid web + local-first HR operations</h2></div>
        <div className={styles.arch}>
          <div className={styles.node}><span>Employees / Managers</span><strong>Next.js web portal</strong><small>HttpOnly session · self service</small></div>
          <div className={styles.arrow}>→</div>
          <div className={styles.nodeMain}><span>Application boundary</span><strong>Go API</strong><small>Auth · RBAC · tenant scope · workflows</small></div>
          <div className={styles.arrow}>→</div>
          <div className={styles.node}><span>Authoritative state</span><strong>PostgreSQL</strong><small>Organizations · people · leave · attendance · audit</small></div>
        </div>
        <div className={styles.archSecondary}><div className={styles.node}><span>HR / Admin</span><strong>Wails desktop</strong><small>Encrypted SQLite · OS keychain · durable sync queue</small></div><div className={styles.arrow}>↗</div><p>The desktop uses the same API and tenant rules. Local storage is an encrypted working copy, not an authorization boundary.</p></div>
        <div className={styles.principles}>
          <article><h3>Tenant invariant</h3><p>Organization ID comes from the authenticated principal. Employee identity is scoped by <code>(organization_id, employee_id)</code>.</p></article>
          <article><h3>Web session boundary</h3><p>The browser does not need the bearer token. Next.js stores it in an HttpOnly cookie and proxies same-origin API calls.</p></article>
          <article><h3>Historical attendance</h3><p>Resolved schedules are snapshotted at check-in so later schedule edits do not change historical payroll calculations.</p></article>
        </div>
      </section>

      <section className={styles.section}>
        <div className={styles.sectionHeading}><p>03 · ROLES</p><h2>Permission model</h2></div>
        <div className={styles.roleGrid}>{roles.map(([role, text]) => <article className={styles.card} key={role}><h3>{role}</h3><p>{text}</p></article>)}</div>
      </section>

      <section className={styles.section}>
        <div className={styles.sectionHeading}><p>04 · PRODUCT MODULES</p><h2>What is implemented now</h2></div>
        <div className={styles.moduleGrid}>
          <article className={styles.card}><h3>People & access</h3><p>Employee create/update/archive, role-aware directory access, one-time invitations and organization memberships.</p><code>/access</code></article>
          <article className={styles.card}><h3>Leave Management v2</h3><p>Policies, balances, carry-over, holidays, half days, overlap prevention, manager approvals and auditable adjustments.</p><code>/leave · /leave/admin</code></article>
          <article className={styles.card}><h3>Attendance Management v2</h3><p>Schedules, effective-dated assignments, check-in/out, corrections, late/early/overtime metrics and payroll-ready summaries.</p><code>/attendance · /attendance/admin</code></article>
          <article className={styles.card}><h3>Desktop operations</h3><p>Encrypted local employee state, queued edits, cloud synchronization, leave approvals, attendance override and audit visibility.</p><code>apps/desktop</code></article>
        </div>
      </section>

      <section className={styles.section} id="api">
        <div className={styles.sectionHeading}><p>05 · API</p><h2>HTTP surface</h2><span>Protected endpoints use <code>Authorization: Bearer &lt;token&gt;</code>. The web portal normally calls them through <code>/api/hr/*</code>.</span></div>
        <div className={styles.endpointGrid}>{endpointGroups.map((group) => <article className={styles.card} key={group.name}><h3>{group.name}</h3><div className={styles.endpointList}>{group.items.map((item) => <code key={item}>{item}</code>)}</div></article>)}</div>
      </section>

      <section className={styles.section}>
        <div className={styles.sectionHeading}><p>06 · CONFIGURATION</p><h2>Environment variables</h2></div>
        <div className={styles.twoCol}>
          <article className={styles.card}><h3>API required</h3><div className={styles.endpointList}><code>DATABASE_URL</code><code>JWT_SECRET</code><code>WEB_ORIGIN</code></div><p><code>JWT_SECRET</code> must be at least 32 characters and should come from a secret manager.</p></article>
          <article className={styles.card}><h3>Web</h3><div className={styles.endpointList}><code>API_INTERNAL_URL</code><code>WEB_COOKIE_SECURE</code></div><p>Local HTTP Docker uses <code>WEB_COOKIE_SECURE=false</code>. HTTPS production should use <code>true</code> or the production default.</p></article>
        </div>
      </section>

      <section className={styles.section}>
        <div className={styles.sectionHeading}><p>07 · DATABASE</p><h2>Schema and migrations</h2></div>
        <article className={styles.card}>
          <pre><code>{`DATABASE_URL='postgres://...' sh db/migrate.sh`}</code></pre>
          <p>The migration entrypoint applies the current idempotent schema and every SQL migration. The Compose stack runs it before the API. Existing migration files remain the upgrade history for tenant-scoped employee IDs, Leave v2 and Attendance v2.</p>
        </article>
      </section>

      <section className={styles.section} id="production">
        <div className={styles.sectionHeading}><p>08 · PRODUCTION</p><h2>Release checklist</h2></div>
        <div className={styles.checklist}>{productionChecklist.map((item) => <div key={item}><span>✓</span><p>{item}</p></div>)}</div>
        <div className={styles.boundary}>
          <h3>Deliberately not presented as complete</h3>
          <p>Payroll/compensation, performance management, recruitment, employee document storage/notifications, enterprise SSO/OIDC, refresh-token revocation, full desktop conflict-resolution UX and the AI HR copilot remain future layers. Placeholder navigation should not be treated as shipped functionality.</p>
        </div>
      </section>

      <section className={styles.section}>
        <div className={styles.sectionHeading}><p>09 · TROUBLESHOOTING</p><h2>Common local issues</h2></div>
        <div className={styles.faq}>
          <details><summary>Login works but the next request is unauthorized</summary><p>For local HTTP, make sure <code>WEB_COOKIE_SECURE=false</code>. Production HTTPS should keep secure cookies enabled.</p></details>
          <details><summary>API cannot reach PostgreSQL in Docker</summary><p>Containers use the Compose service hostname <code>postgres</code>, not <code>localhost</code>.</p></details>
          <details><summary>Ports 3000, 8080 or 5432 are already used</summary><p>Set <code>HRIS_WEB_PORT</code>, <code>HRIS_API_PORT</code> or <code>HRIS_POSTGRES_PORT</code> before running Compose.</p></details>
          <details><summary>I want a completely fresh demo database</summary><p>Run <code>docker compose down -v</code>, then start again with <code>docker compose up --build</code>.</p></details>
        </div>
      </section>

      <footer className={styles.footer}><div><strong>Advance HRIS</strong><span>Documentation mirrors the repository guides under <code>docs/</code>.</span></div><a href="/">Back to application</a></footer>
    </main>
  );
}
