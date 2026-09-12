import { useEffect, useMemo, useState } from "react";
import OperationsPanel from "./OperationsPanel";

type Employee = {
  id: string;
  employee_number?: string;
  first_name?: string;
  last_name?: string;
  name: string;
  work_email?: string;
  role: string;
  department: string;
  location: string;
  employment_type?: string;
  status: string;
  server_version: number;
  updated_at: string;
};

type AuthUser = {
  user_id: string;
  email: string;
  display_name: string;
  organization_id: string;
  organization: string;
  role: string;
};

type DesktopState = {
  employees: Employee[];
  pending_sync: number;
  last_sync_at: string;
  offline: boolean;
  api_base_url: string;
  storage_ready: boolean;
  authenticated: boolean;
  user: AuthUser;
};

type SyncResult = {
  pushed: number;
  pulled: number;
  pending: number;
  conflicts: number;
  offline: boolean;
  message: string;
};

type AuthSession = {
  authenticated: boolean;
  expires_at: string;
  user: AuthUser;
};

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          GetDesktopState: () => Promise<DesktopState>;
          Login: (email: string, password: string, organizationSlug: string) => Promise<AuthSession>;
          Logout: () => Promise<void>;
          SaveEmployee: (employee: Employee) => Promise<void>;
          SyncNow: () => Promise<SyncResult>;
        };
      };
    };
  }
}

const blankEmployee = (): Employee => ({
  id: "",
  employee_number: "",
  name: "",
  work_email: "",
  role: "",
  department: "",
  location: "",
  employment_type: "full_time",
  status: "active",
  server_version: 0,
  updated_at: new Date().toISOString()
});

export default function App() {
  const [state, setState] = useState<DesktopState | null>(null);
  const [error, setError] = useState("");
  const [syncing, setSyncing] = useState(false);
  const [query, setQuery] = useState("");
  const [editing, setEditing] = useState<Employee | null>(null);
  const [email, setEmail] = useState("admin@advancehris.local");
  const [password, setPassword] = useState("");
  const [organizationSlug, setOrganizationSlug] = useState("northstar");
  const [signingIn, setSigningIn] = useState(false);

  const api = window.go?.main?.App;

  async function refresh() {
    if (!api) {
      setError("Open this UI through Wails so the native bridge is available.");
      return;
    }
    try {
      const next = await api.GetDesktopState();
      setState(next);
      setError("");
    } catch (err) {
      setError(String(err));
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  async function signIn(event: React.FormEvent) {
    event.preventDefault();
    if (!api) return;
    setSigningIn(true);
    setError("");
    try {
      await api.Login(email, password, organizationSlug);
      setPassword("");
      await refresh();
    } catch (err) {
      setError(String(err));
    } finally {
      setSigningIn(false);
    }
  }

  async function signOut() {
    if (!api) return;
    await api.Logout();
    setEditing(null);
    setQuery("");
    await refresh();
  }

  const employees = useMemo(() => {
    const list = state?.employees ?? [];
    const needle = query.trim().toLowerCase();
    if (!needle) return list;
    return list.filter((employee) =>
      [employee.name, employee.work_email, employee.role, employee.department, employee.location]
        .join(" ")
        .toLowerCase()
        .includes(needle)
    );
  }, [query, state]);

  async function syncNow() {
    if (!api) return;
    setSyncing(true);
    try {
      const result = await api.SyncNow();
      await refresh();
      if (result.message && (result.offline || result.conflicts > 0)) setError(result.message);
    } catch (err) {
      setError(String(err));
    } finally {
      setSyncing(false);
    }
  }

  async function saveEmployee(event: React.FormEvent) {
    event.preventDefault();
    if (!api || !editing) return;
    try {
      await api.SaveEmployee(editing);
      setEditing(null);
      await refresh();
    } catch (err) {
      setError(String(err));
    }
  }

  if (state && !state.authenticated) {
    return (
      <div className="login-shell">
        <section className="login-visual">
          <div className="brand login-brand">
            <div className="brand-mark">A</div>
            <div><strong>Advance HRIS</strong><span>Secure HR desktop</span></div>
          </div>
          <div className="login-copy">
            <p className="eyebrow">HYBRID LOCAL-FIRST HRIS</p>
            <h1>Your HR workspace, secured on this device.</h1>
            <p>Sign in with an HR or administrator account to unlock encrypted local workforce data and synchronize approved changes with Advance HRIS Cloud.</p>
          </div>
          <div className="login-security">
            <span>Encrypted SQLite</span>
            <span>OS keychain session</span>
            <span>Tenant-scoped RBAC</span>
          </div>
        </section>
        <section className="login-panel-wrap">
          <form className="login-panel" onSubmit={signIn}>
            <div>
              <p className="eyebrow">HR ADMIN CONSOLE</p>
              <h2>Sign in</h2>
              <p className="subtle">The native console is restricted to HR and administrator accounts. Managers and employees use the web portal.</p>
            </div>
            {error && <div className="notice">{error}</div>}
            <label>Email<input type="email" required value={email} onChange={(event) => setEmail(event.target.value)} /></label>
            <label>Password<input type="password" required value={password} onChange={(event) => setPassword(event.target.value)} /></label>
            <label>Organization<input required value={organizationSlug} onChange={(event) => setOrganizationSlug(event.target.value)} /></label>
            <button className="primary login-button" type="submit" disabled={signingIn}>{signingIn ? "Signing in…" : "Unlock workspace"}</button>
            <small className="login-footnote">Cloud: {state.api_base_url}</small>
          </form>
        </section>
      </div>
    );
  }

  const active = state?.employees.filter((employee) => employee.status === "active").length ?? 0;
  const departments = new Set(state?.employees.map((employee) => employee.department).filter(Boolean)).size;

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">A</div>
          <div><strong>Advance HRIS</strong><span>People operating system</span></div>
        </div>

        <nav>
          <button className="nav-item active">Overview</button>
          <button className="nav-item">People</button>
          <button className="nav-item">Attendance</button>
          <button className="nav-item">Leave</button>
          <button className="nav-item">Payroll</button>
          <button className="nav-item">Performance</button>
          <button className="nav-item">Recruitment</button>
        </nav>

        <div className="sync-card">
          <div className="sync-row">
            <span className={state?.offline ? "status-dot offline" : "status-dot"} />
            <strong>{state?.offline ? "Offline mode" : "Cloud ready"}</strong>
          </div>
          <p>{state?.pending_sync ?? 0} change(s) waiting to sync</p>
          <button onClick={syncNow} disabled={syncing}>{syncing ? "Syncing…" : "Sync now"}</button>
        </div>

        <div className="desktop-user">
          <div><strong>{state?.user?.display_name || "HR user"}</strong><span>{state?.user?.role || "member"} · {state?.user?.organization || "workspace"}</span></div>
          <button className="ghost" onClick={signOut}>Sign out</button>
        </div>
      </aside>

      <main>
        <header className="topbar">
          <div>
            <p className="eyebrow">HR ADMIN CONSOLE</p>
            <h1>Workforce command center</h1>
            <p className="subtle">Fast local access, secure cloud synchronization, one shared HR system.</p>
          </div>
          <div className="top-actions">
            <input aria-label="Search employees" placeholder="Search people, email, teams, roles…" value={query} onChange={(event) => setQuery(event.target.value)} />
            <button className="primary" onClick={() => setEditing(blankEmployee())}>Add employee</button>
          </div>
        </header>

        {error && <div className="notice">{error}</div>}

        <section className="metrics">
          <Metric label="People" value={String(state?.employees.length ?? 0)} meta="Encrypted locally" />
          <Metric label="Active" value={String(active)} meta="Current workforce" />
          <Metric label="Departments" value={String(departments)} meta="Across this workspace" />
          <Metric label="Pending sync" value={String(state?.pending_sync ?? 0)} meta="Queued safely offline" />
        </section>

        <section className="content-grid">
          <div className="panel people-panel">
            <div className="panel-heading">
              <div><p className="eyebrow">WORKFORCE</p><h2>People directory</h2></div>
              <span>{employees.length} shown</span>
            </div>

            <div className="people-table">
              <div className="table-row table-head"><span>Employee</span><span>Team</span><span>Location</span><span>Status</span></div>
              {employees.map((employee) => (
                <button className="table-row person-row" key={employee.id} onClick={() => setEditing(employee)}>
                  <span className="person">
                    <span className="avatar">{employee.name.split(" ").map((part) => part[0]).slice(0, 2).join("")}</span>
                    <span><strong>{employee.name}</strong><small>{employee.role}{employee.work_email ? ` · ${employee.work_email}` : ""}</small></span>
                  </span>
                  <span>{employee.department}</span>
                  <span>{employee.location}</span>
                  <span><span className={`badge ${employee.status}`}>{employee.status}</span></span>
                </button>
              ))}
            </div>
          </div>

          <div className="right-stack">
            <div className="panel">
              <p className="eyebrow">LOCAL-FIRST</p>
              <h2>Desktop health</h2>
              <div className="health-list">
                <Health label="Encrypted local store" ok={Boolean(state?.storage_ready)} />
                <Health label="OS keychain protected session" ok={Boolean(state?.authenticated)} />
                <Health label="Cloud API" ok={!state?.offline} />
                <Health label="Sync queue" ok={(state?.pending_sync ?? 0) === 0} />
              </div>
              <div className="last-sync">Last sync<strong>{state?.last_sync_at ? new Date(state.last_sync_at).toLocaleString() : "Not synced yet"}</strong></div>
            </div>

            <div className="panel ai-card">
              <p className="eyebrow">AI HR COPILOT — FOUNDATION</p>
              <h2>Ask your workforce</h2>
              <p>Permission-aware analysis will sit on the same employee, leave, payroll and performance model.</p>
              <button disabled>Coming next</button>
            </div>
          </div>
        </section>

        {state?.authenticated && <OperationsPanel employees={state.employees} role={state.user.role} onError={setError} />}
      </main>

      {editing && (
        <div className="modal-backdrop" onMouseDown={() => setEditing(null)}>
          <form className="modal" onSubmit={saveEmployee} onMouseDown={(event) => event.stopPropagation()}>
            <div className="panel-heading">
              <div><p className="eyebrow">LOCAL EDIT</p><h2>{editing.id ? "Update employee" : "Add employee"}</h2></div>
              <button type="button" className="ghost" onClick={() => setEditing(null)}>Close</button>
            </div>
            <label>Name<input required value={editing.name} onChange={(e) => setEditing({...editing, name: e.target.value})} /></label>
            <div className="field-grid">
              <label>Employee number<input value={editing.employee_number || ""} onChange={(e) => setEditing({...editing, employee_number: e.target.value})} /></label>
              <label>Work email<input type="email" value={editing.work_email || ""} onChange={(e) => setEditing({...editing, work_email: e.target.value})} /></label>
            </div>
            <label>Role<input required value={editing.role} onChange={(e) => setEditing({...editing, role: e.target.value})} /></label>
            <div className="field-grid">
              <label>Department<input required value={editing.department} onChange={(e) => setEditing({...editing, department: e.target.value})} /></label>
              <label>Location<input required value={editing.location} onChange={(e) => setEditing({...editing, location: e.target.value})} /></label>
            </div>
            <div className="field-grid">
              <label>Employment type<select value={editing.employment_type || "full_time"} onChange={(e) => setEditing({...editing, employment_type: e.target.value})}><option value="full_time">Full time</option><option value="part_time">Part time</option><option value="contract">Contract</option><option value="intern">Intern</option><option value="temporary">Temporary</option></select></label>
              <label>Status<select value={editing.status} onChange={(e) => setEditing({...editing, status: e.target.value})}><option value="active">Active</option><option value="leave">On leave</option><option value="onboarding">Onboarding</option><option value="inactive">Inactive</option></select></label>
            </div>
            <div className="modal-note">This change is saved locally first and queued for secure, tenant-scoped cloud synchronization.</div>
            <button className="primary" type="submit">Save locally</button>
          </form>
        </div>
      )}
    </div>
  );
}

function Metric({label, value, meta}: {label: string; value: string; meta: string}) {
  return <div className="metric"><span>{label}</span><strong>{value}</strong><small>{meta}</small></div>;
}

function Health({label, ok}: {label: string; ok: boolean}) {
  return <div className="health-row"><span>{label}</span><strong className={ok ? "healthy" : "warning"}>{ok ? "Ready" : "Attention"}</strong></div>;
}
