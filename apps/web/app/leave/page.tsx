"use client";

import {FormEvent, useEffect, useMemo, useState} from "react";
import styles from "./leave.module.css";

type User = {user_id: string; email: string; display_name: string; organization: string; role: string};
type Employee = {id: string; name: string; role: string; department: string; location: string; status: string};
type LeaveRequest = {id: string; employee_id: string; employee: string; type: string; start: string; end: string; days: number; reason?: string; status: string};
type Balance = {employee_id: string; policy_id: string; policy_name: string; leave_type: string; year: number; entitlement: number; carry_over: number; adjustments: number; used: number; available: number; track_balance: boolean};
type Policy = {id: string; name: string; leave_type: string; annual_entitlement: number; carry_over_limit: number; track_balance: boolean};
type Holiday = {id: string; date: string; name: string; location?: string};

async function requestJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {...init, cache: "no-store"});
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(payload?.error?.message || `Request failed (${response.status})`);
  return payload as T;
}

export default function LeaveWorkspace() {
  const [user, setUser] = useState<User | null>(null);
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [requests, setRequests] = useState<LeaveRequest[]>([]);
  const [balances, setBalances] = useState<Balance[]>([]);
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [holidays, setHolidays] = useState<Holiday[]>([]);
  const [selectedEmployee, setSelectedEmployee] = useState("");
  const [leaveType, setLeaveType] = useState("annual");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [partialDay, setPartialDay] = useState("full");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const elevated = user?.role === "admin" || user?.role === "hr";
  const manager = user?.role === "manager";

  async function loadBase() {
    setLoading(true);
    try {
      const me = await requestJSON<{user: User}>("/api/hr/auth/me");
      const canSeeDirectory = me.user.role !== "employee";
      const [leaveResponse, policyResponse, holidayResponse, employeeResponse] = await Promise.all([
        requestJSON<{data: LeaveRequest[]}>("/api/hr/leave/requests"),
        requestJSON<{data: Policy[]}>("/api/hr/leave/policies"),
        requestJSON<{data: Holiday[]}>(`/api/hr/leave/holidays?year=${new Date().getFullYear()}`),
        canSeeDirectory ? requestJSON<{data: Employee[]}>("/api/hr/employees") : Promise.resolve({data: [] as Employee[]}),
      ]);
      setUser(me.user);
      setRequests(leaveResponse.data ?? []);
      setPolicies(policyResponse.data ?? []);
      setHolidays(holidayResponse.data ?? []);
      setEmployees(employeeResponse.data ?? []);
      const target = me.user.role === "admin" || me.user.role === "hr" ? (selectedEmployee || employeeResponse.data?.[0]?.id || "") : "";
      if ((me.user.role === "admin" || me.user.role === "hr") && target) setSelectedEmployee(target);
      await loadBalances(me.user, target);
      setError("");
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }

  async function loadBalances(currentUser = user, employeeID = selectedEmployee) {
    if (!currentUser) return;
    const elevatedUser = currentUser.role === "admin" || currentUser.role === "hr";
    const query = elevatedUser ? `?employee_id=${encodeURIComponent(employeeID)}&year=${new Date().getFullYear()}` : `?year=${new Date().getFullYear()}`;
    if (elevatedUser && !employeeID) return;
    const response = await requestJSON<{data: Balance[]}>(`/api/hr/leave/balances${query}`);
    setBalances(response.data ?? []);
  }

  useEffect(() => { void loadBase(); }, []);

  async function changeEmployee(employeeID: string) {
    setSelectedEmployee(employeeID);
    if (user) {
      try { await loadBalances(user, employeeID); } catch (err) { setError(String(err)); }
    }
  }

  async function submitLeave(event: FormEvent) {
    event.preventDefault();
    if (!user) return;
    setBusy("create"); setError("");
    try {
      const finalEnd = partialDay === "full" ? endDate : startDate;
      await requestJSON("/api/hr/leave/requests", {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify({
          employee_id: elevated ? selectedEmployee : "",
          leave_type: leaveType,
          start_date: startDate,
          end_date: finalEnd,
          partial_day: partialDay,
          reason,
        }),
      });
      setStartDate(""); setEndDate(""); setPartialDay("full"); setReason("");
      await loadBase();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function decide(id: string, decision: "approved" | "rejected") {
    setBusy(id); setError("");
    try {
      await requestJSON(`/api/hr/leave/requests/${id}/decision`, {
        method: "POST", headers: {"Content-Type": "application/json"},
        body: JSON.stringify({decision, note: `${decision} from leave workspace`}),
      });
      await loadBase();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function cancelRequest(id: string) {
    setBusy(id); setError("");
    try {
      await requestJSON(`/api/hr/leave/requests/${id}/cancel`, {
        method: "POST", headers: {"Content-Type": "application/json"},
        body: JSON.stringify({note: "Cancelled from leave workspace"}),
      });
      await loadBase();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  const visibleBalances = useMemo(() => balances.filter((balance) => balance.track_balance), [balances]);
  const pending = useMemo(() => requests.filter((request) => request.status === "pending"), [requests]);
  const canApprove = elevated || manager;

  if (loading && !user) return <main className={styles.state}>Loading leave workspace…</main>;
  if (!user) return <main className={styles.state}><h1>Leave workspace unavailable</h1><p>{error || "Sign in first to continue."}</p><a href="/">Return to sign in</a></main>;

  return <main className={styles.shell}>
    <header className={styles.topbar}>
      <div><p>ADVANCE HRIS · LEAVE</p><h1>Time off, without spreadsheet accounting.</h1><span>Policy-backed balances, company holidays and auditable approvals for {user.organization}.</span></div>
      <div className={styles.headerActions}><a href="/">Overview</a>{elevated && <a href="/access">Access</a>}<button onClick={loadBase}>Refresh</button></div>
    </header>

    {error && <div className={styles.error}>{error}</div>}

    {elevated && <section className={styles.employeeSelector}><label>Viewing employee<select value={selectedEmployee} onChange={(event) => void changeEmployee(event.target.value)}>{employees.map((employee) => <option value={employee.id} key={employee.id}>{employee.name} · {employee.department}</option>)}</select></label></section>}

    <section className={styles.balanceGrid}>
      {visibleBalances.length === 0 && <article className={styles.emptyCard}><strong>No tracked balances</strong><span>This role or leave policy does not currently use an entitlement balance.</span></article>}
      {visibleBalances.map((balance) => <article className={styles.balanceCard} key={balance.policy_id}>
        <div><span>{balance.policy_name}</span><small>{balance.year}</small></div>
        <strong>{formatDays(balance.available)}</strong>
        <p>available</p>
        <footer><span>{formatDays(balance.entitlement)} entitlement</span><span>{formatDays(balance.used)} used</span>{balance.carry_over > 0 && <span>{formatDays(balance.carry_over)} carried</span>}</footer>
      </article>)}
    </section>

    <section className={styles.mainGrid}>
      <article className={styles.panel}>
        <div className={styles.panelHeading}><div><p>NEW REQUEST</p><h2>Request time away</h2></div><span>Weekends + holidays excluded</span></div>
        <form className={styles.form} onSubmit={submitLeave}>
          {elevated && <label>Employee<select required value={selectedEmployee} onChange={(event) => void changeEmployee(event.target.value)}>{employees.map((employee) => <option value={employee.id} key={employee.id}>{employee.name}</option>)}</select></label>}
          <div className={styles.fieldGrid}>
            <label>Leave type<select value={leaveType} onChange={(event) => setLeaveType(event.target.value)}>{policies.map((policy) => <option key={policy.id} value={policy.leave_type}>{policy.name}</option>)}</select></label>
            <label>Day type<select value={partialDay} onChange={(event) => setPartialDay(event.target.value)}><option value="full">Full day(s)</option><option value="first_half">First half</option><option value="second_half">Second half</option></select></label>
          </div>
          <div className={styles.fieldGrid}>
            <label>Start<input required type="date" value={startDate} onChange={(event) => { setStartDate(event.target.value); if (partialDay !== "full") setEndDate(event.target.value); }} /></label>
            <label>End<input required type="date" disabled={partialDay !== "full"} value={partialDay === "full" ? endDate : startDate} onChange={(event) => setEndDate(event.target.value)} /></label>
          </div>
          <label>Reason<textarea value={reason} onChange={(event) => setReason(event.target.value)} placeholder="Optional context for your approver" /></label>
          <button className={styles.primary} disabled={busy === "create" || !startDate || (partialDay === "full" && !endDate)}>{busy === "create" ? "Submitting…" : "Submit request"}</button>
        </form>
      </article>

      <article className={styles.panel}>
        <div className={styles.panelHeading}><div><p>CALENDAR</p><h2>Company holidays</h2></div><span>{holidays.length} this year</span></div>
        <div className={styles.holidayList}>{holidays.length === 0 ? <div className={styles.empty}>No holidays configured yet.</div> : holidays.slice(0, 8).map((holiday) => <div className={styles.holidayRow} key={holiday.id}><time>{new Date(`${holiday.date}T00:00:00`).toLocaleDateString(undefined, {month: "short", day: "numeric"})}</time><div><strong>{holiday.name}</strong><span>{holiday.location || "All locations"}</span></div></div>)}</div>
      </article>
    </section>

    {canApprove && <section className={styles.panel}>
      <div className={styles.panelHeading}><div><p>APPROVAL QUEUE</p><h2>{manager ? "Your team’s pending leave" : "Pending organization leave"}</h2></div><span>{pending.length} pending</span></div>
      <div className={styles.requestList}>{pending.length === 0 ? <div className={styles.empty}>Nothing needs approval.</div> : pending.map((request) => <div className={styles.requestRow} key={request.id}><div><strong>{request.employee}</strong><span>{labelType(request.type)} · {request.start} → {request.end} · {formatDays(request.days)}</span>{request.reason && <small>{request.reason}</small>}</div><div className={styles.actions}><button onClick={() => void decide(request.id, "approved")} disabled={busy === request.id}>Approve</button><button className={styles.danger} onClick={() => void decide(request.id, "rejected")} disabled={busy === request.id}>Reject</button></div></div>)}</div>
    </section>}

    <section className={styles.panel}>
      <div className={styles.panelHeading}><div><p>HISTORY</p><h2>Leave requests</h2></div><span>{requests.length} visible</span></div>
      <div className={styles.requestList}>{requests.length === 0 ? <div className={styles.empty}>No leave requests yet.</div> : requests.slice(0, 20).map((request) => <div className={styles.requestRow} key={request.id}><div><strong>{request.employee}</strong><span>{labelType(request.type)} · {request.start} → {request.end} · {formatDays(request.days)}</span></div><div className={styles.historyActions}><span className={`${styles.status} ${styles[request.status] || ""}`}>{request.status}</span>{(elevated || (user.role === "employee" && (request.status === "pending" || request.status === "approved"))) && (request.status === "pending" || request.status === "approved") && <button onClick={() => void cancelRequest(request.id)} disabled={busy === request.id}>Cancel</button>}</div></div>)}</div>
    </section>
  </main>;
}

function labelType(value: string) { return value.replaceAll("_", " ").replace(/^./, (letter) => letter.toUpperCase()); }
function formatDays(value: number) { return `${Number(value).toLocaleString(undefined, {maximumFractionDigits: 1})} day${value === 1 ? "" : "s"}`; }
