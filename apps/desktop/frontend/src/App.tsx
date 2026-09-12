import { useEffect, useMemo, useState } from "react";

type Employee = {
  id: string;
  name: string;
  role: string;
  department: string;
  location: string;
  status: string;
  server_version: number;
  updated_at: string;
};

type DesktopState = {
  employees: Employee[];
  pending_sync: number;
  last_sync_at: string;
  offline: boolean;
  api_base_url: string;
  storage_ready: boolean;
};

type SyncResult = {
  pushed: number;
  pulled: number;
  pending: number;
  conflicts: number;
  offline: boolean;
  message: string;
};

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          GetDesktopState: () => Promise<DesktopState>;
          SaveEmployee: (employee: Employee) => Promise<void>;
          SyncNow: () => Promise<SyncResult>;
        };
      };
    };
  }
}

const blankEmployee = (): Employee => ({
  id: "",
  name: "",
  role: "",
  department: "",
  location: "",
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

  const employees = useMemo(() => {
    const list = state?.employees ?? [];
    const needle = query.trim().toLowerCase();
    if (!needle) return list;
    return list.filter((employee) =>
      [employee.name, employee.role, employee.department, employee.location]
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
    await api.SaveEmployee(editing);
    setEditing(null);
    await refresh();
  }

  const active = state?.employees.filter((employee) => employee.status === "active").length ?? 0;
  const departments = new Set(state?.employees.map((employee) => employee.department)).size;

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
      </aside>

      <main>
        <header className="topbar">
          <div>
            <p className="eyebrow">HR ADMIN CONSOLE</p>
            <h1>Workforce command center</h1>
            <p className="subtle">Fast local access, secure cloud synchronization, one shared HR system.</p>
          </div>
          <div className="top-actions">
            <input aria-label="Search employees" placeholder="Search people, teams, roles…" value={query} onChange={(event) => setQuery(event.target.value)} />
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
                    <span><strong>{employee.name}</strong><small>{employee.role}</small></span>
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
                <Health label="OS keychain protected key" ok={Boolean(state?.storage_ready)} />
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
      </main>

      {editing && (
        <div className="modal-backdrop" onMouseDown={() => setEditing(null)}>
          <form className="modal" onSubmit={saveEmployee} onMouseDown={(event) => event.stopPropagation()}>
            <div className="panel-heading">
              <div><p className="eyebrow">LOCAL EDIT</p><h2>{editing.id ? "Update employee" : "Add employee"}</h2></div>
              <button type="button" className="ghost" onClick={() => setEditing(null)}>Close</button>
            </div>
            <label>Name<input required value={editing.name} onChange={(e) => setEditing({...editing, name: e.target.value})} /></label>
            <label>Role<input required value={editing.role} onChange={(e) => setEditing({...editing, role: e.target.value})} /></label>
            <div className="field-grid">
              <label>Department<input required value={editing.department} onChange={(e) => setEditing({...editing, department: e.target.value})} /></label>
              <label>Location<input required value={editing.location} onChange={(e) => setEditing({...editing, location: e.target.value})} /></label>
            </div>
            <label>Status<select value={editing.status} onChange={(e) => setEditing({...editing, status: e.target.value})}><option value="active">Active</option><option value="leave">On leave</option><option value="inactive">Inactive</option></select></label>
            <div className="modal-note">This change is saved locally first and queued for secure cloud synchronization.</div>
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
