"use client";

import {FormEvent, useEffect, useState} from "react";

type User = {user_id: string; email: string; display_name: string; organization_id: string; organization: string; role: string};
type Employee = {id: string; name: string; role: string; department: string; location: string; status: string};
type LeaveRequest = {id: string; employee_id: string; employee: string; type: string; start: string; end: string; days: number; reason?: string; status: string};
type AttendanceEntry = {id: string; employee_id: string; employee: string; work_date: string; check_in?: string; check_out?: string; work_mode: string; status: string};
type Dashboard = {employees: number; present_today: number; on_leave: number; open_roles: number; pending_leave_requests: number};
type PortalData = {user: User | null; employees: Employee[]; leave: LeaveRequest[]; attendance: AttendanceEntry[]; dashboard: Dashboard | null};

const emptyData: PortalData = {user: null, employees: [], leave: [], attendance: [], dashboard: null};

async function requestJSON<T = unknown>(input: RequestInfo | URL, init?: RequestInit): Promise<T> {
  const response = await fetch(input, {...init, cache: "no-store"});
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body?.error?.message || `Request failed (${response.status})`);
  return body as T;
}

export default function Home() {
  const [data, setData] = useState<PortalData>(emptyData);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [email, setEmail] = useState("admin@advancehris.local");
  const [password, setPassword] = useState("");
  const [organization, setOrganization] = useState("northstar");
  const [leaveEmployee, setLeaveEmployee] = useState("");
  const [leaveType, setLeaveType] = useState("annual");
  const [leaveStart, setLeaveStart] = useState("");
  const [leaveEnd, setLeaveEnd] = useState("");
  const [leaveReason, setLeaveReason] = useState("");
  const [attendanceEmployee, setAttendanceEmployee] = useState("");
  const [workMode, setWorkMode] = useState("office");

  async function loadPortal() {
    setLoading(true);
    try {
      const me = await requestJSON<{user: User}>("/api/hr/auth/me");
      const role = me.user.role;
      const [leaveResponse, attendanceResponse, dashboardResponse, employeeResponse] = await Promise.all([
        requestJSON<{data: LeaveRequest[]}>("/api/hr/leave/requests"),
        requestJSON<{data: AttendanceEntry[]}>("/api/hr/attendance"),
        role === "employee" ? Promise.resolve(null) : requestJSON<Dashboard>("/api/hr/dashboard"),
        role === "employee" ? Promise.resolve({data: [] as Employee[]}) : requestJSON<{data: Employee[]}>("/api/hr/employees"),
      ]);
      const employees = employeeResponse.data ?? [];
      setData({user: me.user, leave: leaveResponse.data ?? [], attendance: attendanceResponse.data ?? [], dashboard: dashboardResponse, employees});
      setLeaveEmployee((current) => current || employees[0]?.id || "");
      setAttendanceEmployee((current) => current || employees[0]?.id || "");
      setError("");
    } catch (err) {
      const message = String(err);
      if (/sign in|unauthorized|401/i.test(message)) setData(emptyData);
      else setError(message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void loadPortal(); }, []);

  async function login(event: FormEvent) {
    event.preventDefault(); setBusy("login"); setError("");
    try {
      await requestJSON("/api/session/login", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({email, password, organization_slug: organization})});
      setPassword(""); await loadPortal();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function logout() { await fetch("/api/session/logout", {method: "POST"}); setData(emptyData); }

  async function createLeave(event: FormEvent) {
    event.preventDefault(); if (!data.user) return; setBusy("leave-create");
    try {
      const elevated = data.user.role === "admin" || data.user.role === "hr";
      await requestJSON("/api/hr/leave/requests", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({employee_id: elevated ? leaveEmployee : "", leave_type: leaveType, start_date: leaveStart, end_date: leaveEnd, reason: leaveReason})});
      setLeaveStart(""); setLeaveEnd(""); setLeaveReason(""); await loadPortal();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function decideLeave(id: string, decision: "approved" | "rejected") {
    setBusy(`leave-${id}`);
    try {
      await requestJSON(`/api/hr/leave/requests/${id}/decision`, {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({decision, note: `${decision === "approved" ? "Approved" : "Rejected"} from web portal`})});
      await loadPortal();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function attendanceAction(action: "check-in" | "check-out") {
    if (!data.user) return; setBusy(action);
    try {
      const elevated = data.user.role === "admin" || data.user.role === "hr";
      await requestJSON(`/api/hr/attendance/${action}`, {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({employee_id: elevated ? attendanceEmployee : "", work_mode: workMode})});
      await loadPortal();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  if (loading && !data.user) return <div className="portal-loading">Loading Advance HRIS…</div>;
  if (!data.user) return <LoginScreen {...{email, setEmail, password, setPassword, organization, setOrganization, login, busy, error}} />;

  const user = data.user;
  const elevated = user.role === "admin" || user.role === "hr";
  const manager = user.role === "manager";
  const pending = data.leave.filter((item) => item.status === "pending");
  const selectedAttendanceID = elevated ? attendanceEmployee : undefined;
  const attendance = selectedAttendanceID ? data.attendance.find((item) => item.employee_id === selectedAttendanceID) : data.attendance[0];
  const approvable = pending.filter((item) => elevated || (manager && item.employee !== user.display_name));
  const directory = data.employees.slice(0, 8);
  const metrics: Array<[string, string | number, string]> = data.dashboard ? [
    ["People", data.dashboard.employees, "Active workforce"], ["Present today", data.dashboard.present_today, "Recorded attendance"], ["On leave", data.dashboard.on_leave, "Current leave status"], ["Pending leave", data.dashboard.pending_leave_requests, "Needs review"],
  ] : [
    ["My requests", data.leave.length, "Leave history"], ["Pending", pending.length, "Awaiting decision"], ["Attendance", attendance ? "Recorded" : "Not yet", "Today"], ["Work mode", attendance?.work_mode || "—", "Today"],
  ];

  return <main className="portal-shell">
    <aside className="portal-sidebar">
      <div className="portal-brand"><span className="brand-mark">A</span><div><strong>Advance HRIS</strong><small>{user.organization}</small></div></div>
      <nav><button className="portal-nav active">Overview</button><button className="portal-nav">Leave</button><button className="portal-nav">Attendance</button>{(elevated || manager) && <button className="portal-nav">People</button>}<button className="portal-nav">Documents</button></nav>
      <div className="portal-account"><span className="avatar">{initials(user.display_name)}</span><div><strong>{user.display_name}</strong><small>{user.role}</small></div><button onClick={logout}>Sign out</button></div>
    </aside>

    <section className="portal-content">
      <header className="portal-topbar"><div><span className="portal-kicker">{user.role.toUpperCase()} PORTAL</span><h1>Good {greeting()}, {firstName(user.display_name)}.</h1><p>{roleMessage(user.role)}</p></div><button className="secondary-button" onClick={loadPortal}>Refresh data</button></header>
      {error && <div className="portal-error">{error}</div>}
      <div className="portal-metrics">{metrics.map(([label, value, meta]) => <article className="metric-card" key={label}><div className="metric-label">{label}</div><div className="metric-value">{value}</div><div className="metric-change">{meta}</div></article>)}</div>

      <div className="portal-grid">
        <article className="panel portal-card"><div className="panel-heading"><div><h2>Request leave</h2><p>Submit leave or a remote-work request.</p></div></div>
          <form className="portal-form" onSubmit={createLeave}>
            {elevated && <label>Employee<select required value={leaveEmployee} onChange={(e) => setLeaveEmployee(e.target.value)}>{data.employees.map((employee) => <option value={employee.id} key={employee.id}>{employee.name}</option>)}</select></label>}
            <div className="portal-fields"><label>Type<select value={leaveType} onChange={(e) => setLeaveType(e.target.value)}><option value="annual">Annual leave</option><option value="sick">Sick leave</option><option value="remote">Remote work</option><option value="unpaid">Unpaid leave</option><option value="parental">Parental leave</option><option value="other">Other</option></select></label><label>Start<input type="date" required value={leaveStart} onChange={(e) => setLeaveStart(e.target.value)} /></label><label>End<input type="date" required value={leaveEnd} onChange={(e) => setLeaveEnd(e.target.value)} /></label></div>
            <label>Reason<textarea value={leaveReason} onChange={(e) => setLeaveReason(e.target.value)} placeholder="Optional context" /></label><button className="primary-button" disabled={busy === "leave-create"}>{busy === "leave-create" ? "Submitting…" : "Submit request"}</button>
          </form>
        </article>

        <article className="panel portal-card"><div className="panel-heading"><div><h2>Attendance</h2><p>Record today’s work session.</p></div><span className={`portal-pill ${attendance ? "ok" : ""}`}>{attendance ? "Recorded" : "Not recorded"}</span></div>
          {elevated && <label className="portal-control">Employee<select value={attendanceEmployee} onChange={(e) => setAttendanceEmployee(e.target.value)}>{data.employees.map((employee) => <option value={employee.id} key={employee.id}>{employee.name}</option>)}</select></label>}
          <label className="portal-control">Work mode<select value={workMode} onChange={(e) => setWorkMode(e.target.value)}><option value="office">Office</option><option value="remote">Remote</option><option value="hybrid">Hybrid</option><option value="field">Field</option></select></label>
          <div className="attendance-actions"><button onClick={() => attendanceAction("check-in")} disabled={Boolean(busy)}>Check in</button><button onClick={() => attendanceAction("check-out")} disabled={Boolean(busy)}>Check out</button></div>
          <div className="attendance-summary"><span>Today</span><strong>{formatTime(attendance?.check_in)} → {formatTime(attendance?.check_out)}</strong><small>{attendance?.work_mode || "No work mode recorded"}</small></div>
        </article>
      </div>

      {(elevated || manager) && <article className="panel portal-card portal-wide"><div className="panel-heading"><div><h2>Leave approvals</h2><p>{manager ? "Pending requests from your team." : "Pending requests across the organization."}</p></div><span className="portal-pill">{approvable.length} pending</span></div><div className="approval-list">{approvable.length === 0 ? <div className="portal-empty">Nothing needs approval.</div> : approvable.map((item) => <div className="approval-row" key={item.id}><div><strong>{item.employee}</strong><p>{item.type} · {item.start} → {item.end} · {item.days} day{item.days === 1 ? "" : "s"}</p></div><div className="approval-actions"><button onClick={() => decideLeave(item.id, "approved")} disabled={busy === `leave-${item.id}`}>Approve</button><button className="danger" onClick={() => decideLeave(item.id, "rejected")} disabled={busy === `leave-${item.id}`}>Reject</button></div></div>)}</div></article>}

      <div className="portal-grid lower-grid">
        <article className="panel portal-card"><div className="panel-heading"><div><h2>Leave history</h2><p>Your visible request history.</p></div></div><div className="history-list">{data.leave.slice(0, 7).map((item) => <div className="history-row" key={item.id}><div><strong>{item.employee}</strong><p>{item.type} · {item.start} → {item.end}</p></div><span className={`status-badge ${item.status}`}>{item.status}</span></div>)}</div></article>
        {(elevated || manager) && <article className="panel portal-card"><div className="panel-heading"><div><h2>People</h2><p>Visible workforce directory.</p></div><span>{data.employees.length}</span></div><div className="directory-list">{directory.map((employee) => <div className="directory-row" key={employee.id}><span className="avatar soft">{initials(employee.name)}</span><div><strong>{employee.name}</strong><p>{employee.role} · {employee.department}</p></div><span className={`status-badge ${employee.status}`}>{employee.status}</span></div>)}</div></article>}
      </div>
    </section>
  </main>;
}

function LoginScreen(props: {email: string; setEmail: (v: string) => void; password: string; setPassword: (v: string) => void; organization: string; setOrganization: (v: string) => void; login: (event: FormEvent) => void; busy: string; error: string}) {
  return <main className="web-login"><section className="web-login-copy"><div className="portal-brand"><span className="brand-mark">A</span><div><strong>Advance HRIS</strong><small>People. Progress. Together.</small></div></div><div><span className="portal-kicker">SECURE PEOPLE OPERATIONS</span><h1>One HR system for every role.</h1><p>Employees manage their work life, managers handle team approvals, and HR operates through the web or the native local-first console.</p></div><div className="web-login-features"><span>Employee self-service</span><span>Manager approvals</span><span>Tenant-scoped access</span><span>HttpOnly web sessions</span></div></section><section className="web-login-form-wrap"><form className="web-login-form" onSubmit={props.login}><span className="portal-kicker">WELCOME BACK</span><h2>Sign in to your workspace</h2><p>Your role determines what you can see and do.</p>{props.error && <div className="portal-error">{props.error}</div>}<label>Email<input type="email" required value={props.email} onChange={(e) => props.setEmail(e.target.value)} /></label><label>Password<input type="password" required value={props.password} onChange={(e) => props.setPassword(e.target.value)} /></label><label>Organization<input required value={props.organization} onChange={(e) => props.setOrganization(e.target.value)} /></label><button className="primary-button login-submit" disabled={props.busy === "login"}>{props.busy === "login" ? "Signing in…" : "Sign in"}</button><small>Session tokens are stored in an HttpOnly cookie and are not exposed to browser JavaScript.</small></form></section></main>;
}

function initials(name: string) { return name.split(" ").filter(Boolean).map((part) => part[0]).slice(0, 2).join("").toUpperCase(); }
function firstName(name: string) { return name.split(" ")[0] || name; }
function greeting() { const hour = new Date().getHours(); return hour < 12 ? "morning" : hour < 18 ? "afternoon" : "evening"; }
function roleMessage(role: string) { if (role === "manager") return "Review your team’s requests and stay on top of attendance."; if (role === "employee") return "Manage your leave, attendance and day-to-day employee tasks."; return "Manage workforce operations with live tenant-scoped data."; }
function formatTime(value?: string) { return value ? new Date(value).toLocaleTimeString([], {hour: "2-digit", minute: "2-digit"}) : "—"; }
