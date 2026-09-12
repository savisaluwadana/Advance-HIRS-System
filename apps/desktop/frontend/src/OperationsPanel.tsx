import { useEffect, useMemo, useState } from "react";

type Employee = { id: string; name: string; department: string };
type LeaveRequest = {
  id: string;
  employee_id: string;
  employee: string;
  type: string;
  start: string;
  end: string;
  days: number;
  reason?: string;
  status: string;
};
type AttendanceEntry = {
  id: string;
  employee_id: string;
  employee: string;
  work_date: string;
  check_in?: string;
  check_out?: string;
  work_mode: string;
  status: string;
};
type AuditEvent = {
  id: string;
  actor?: string;
  action: string;
  resource_type: string;
  created_at: string;
};
type OperationsState = {
  leave_requests: LeaveRequest[];
  attendance: AttendanceEntry[];
  audit: AuditEvent[];
};
type WorkflowBridge = {
  GetOperationsState?: () => Promise<OperationsState>;
  DecideLeaveRequest?: (id: string, decision: string, note: string) => Promise<LeaveRequest>;
  CheckInEmployee?: (employeeID: string, workMode: string) => Promise<AttendanceEntry>;
  CheckOutEmployee?: (employeeID: string) => Promise<AttendanceEntry>;
};

export default function OperationsPanel({employees, role, onError}: {employees: Employee[]; role: string; onError: (message: string) => void}) {
  const [state, setState] = useState<OperationsState>({leave_requests: [], attendance: [], audit: []});
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [employeeID, setEmployeeID] = useState("");
  const [workMode, setWorkMode] = useState("office");
  const api = (window as unknown as {go?: {main?: {App?: WorkflowBridge}}}).go?.main?.App;

  async function refresh() {
    if (!api?.GetOperationsState) return;
    setLoading(true);
    try {
      const next = await api.GetOperationsState();
      setState(next);
      if (!employeeID && employees[0]?.id) setEmployeeID(employees[0].id);
    } catch (err) {
      onError(String(err));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  const pending = useMemo(() => state.leave_requests.filter((request) => request.status === "pending"), [state.leave_requests]);
  const attendanceByEmployee = useMemo(() => new Map(state.attendance.map((entry) => [entry.employee_id, entry])), [state.attendance]);
  const canDecide = role === "admin" || role === "hr" || role === "manager";
  const canOverrideAttendance = role === "admin" || role === "hr";

  async function decide(id: string, decision: "approved" | "rejected") {
    if (!api?.DecideLeaveRequest) return;
    setBusy(id);
    try {
      await api.DecideLeaveRequest(id, decision, decision === "approved" ? "Approved from desktop HR console" : "Rejected from desktop HR console");
      await refresh();
    } catch (err) {
      onError(String(err));
    } finally {
      setBusy("");
    }
  }

  async function attendanceAction(action: "in" | "out") {
    if (!employeeID || !api) return;
    setBusy(`attendance-${employeeID}`);
    try {
      if (action === "in" && api.CheckInEmployee) await api.CheckInEmployee(employeeID, workMode);
      if (action === "out" && api.CheckOutEmployee) await api.CheckOutEmployee(employeeID);
      await refresh();
    } catch (err) {
      onError(String(err));
    } finally {
      setBusy("");
    }
  }

  return (
    <section className="operations-section">
      <div className="operations-heading">
        <div><p className="eyebrow">LIVE HR OPERATIONS</p><h2>Approvals, attendance & audit</h2><p className="subtle">Cloud-backed workflows scoped to your signed-in role and organization.</p></div>
        <button className="ghost operation-refresh" onClick={refresh} disabled={loading}>{loading ? "Loading…" : "Refresh"}</button>
      </div>

      <div className="operations-grid">
        <article className="panel workflow-panel">
          <div className="panel-heading"><div><p className="eyebrow">LEAVE</p><h2>Pending approvals</h2></div><span>{pending.length} pending</span></div>
          <div className="workflow-list">
            {pending.length === 0 && <div className="empty-state">No leave requests need attention.</div>}
            {pending.slice(0, 5).map((request) => (
              <div className="leave-row" key={request.id}>
                <div className="leave-main"><strong>{request.employee}</strong><span>{request.type} · {request.start} → {request.end} · {request.days} day{request.days === 1 ? "" : "s"}</span>{request.reason && <small>{request.reason}</small>}</div>
                {canDecide && <div className="decision-actions"><button disabled={busy === request.id} className="approve" onClick={() => decide(request.id, "approved")}>Approve</button><button disabled={busy === request.id} className="reject" onClick={() => decide(request.id, "rejected")}>Reject</button></div>}
              </div>
            ))}
          </div>
        </article>

        <article className="panel workflow-panel">
          <div className="panel-heading"><div><p className="eyebrow">ATTENDANCE</p><h2>Today</h2></div><span>{state.attendance.length} recorded</span></div>
          {canOverrideAttendance && <div className="attendance-control">
            <select value={employeeID} onChange={(event) => setEmployeeID(event.target.value)}>{employees.map((employee) => <option value={employee.id} key={employee.id}>{employee.name}</option>)}</select>
            <select value={workMode} onChange={(event) => setWorkMode(event.target.value)}><option value="office">Office</option><option value="remote">Remote</option><option value="hybrid">Hybrid</option><option value="field">Field</option></select>
            <button onClick={() => attendanceAction("in")} disabled={!employeeID || Boolean(busy)}>Check in</button>
            <button onClick={() => attendanceAction("out")} disabled={!employeeID || Boolean(busy)}>Check out</button>
          </div>}
          <div className="attendance-list">
            {employees.slice(0, 6).map((employee) => {
              const entry = attendanceByEmployee.get(employee.id);
              return <div className="attendance-row" key={employee.id}><span><strong>{employee.name}</strong><small>{employee.department || "—"}</small></span><span className={entry ? "attendance-state present" : "attendance-state"}>{entry ? `${entry.work_mode} · ${entry.check_out ? "done" : "working"}` : "not recorded"}</span></div>;
            })}
          </div>
        </article>

        {(role === "admin" || role === "hr") && <article className="panel workflow-panel audit-panel">
          <div className="panel-heading"><div><p className="eyebrow">AUDIT</p><h2>Recent changes</h2></div><span>immutable trail</span></div>
          <div className="audit-list">
            {state.audit.length === 0 && <div className="empty-state">No audit events yet.</div>}
            {state.audit.slice(0, 8).map((event) => <div className="audit-row" key={event.id}><span className="audit-mark">{event.action.split(".")[0].slice(0, 2).toUpperCase()}</span><div><strong>{event.action.replaceAll(".", " ")}</strong><small>{event.actor || "System"} · {new Date(event.created_at).toLocaleString()}</small></div></div>)}
          </div>
        </article>}
      </div>
    </section>
  );
}
