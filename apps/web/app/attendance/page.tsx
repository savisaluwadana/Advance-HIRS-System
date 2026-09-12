"use client";

import {FormEvent, useEffect, useMemo, useState} from "react";
import styles from "./attendance.module.css";

type User = {user_id: string; display_name: string; organization: string; role: string};
type Employee = {id: string; name: string; department: string};
type Attendance = {id: string; employee_id: string; employee: string; work_date: string; check_in?: string; check_out?: string; work_mode: string; schedule_name?: string; late_minutes: number; early_leave_minutes: number; worked_minutes: number; overtime_minutes: number; source: string};
type Correction = {id: string; employee_id: string; employee: string; work_date: string; requested_check_in?: string; requested_check_out?: string; requested_work_mode: string; reason: string; status: string; review_note?: string};
type Timesheet = {employee_id: string; employee: string; from: string; to: string; scheduled_days: number; recorded_days: number; worked_minutes: number; overtime_minutes: number; late_minutes: number; early_leave_minutes: number; worked_hours: number; overtime_hours: number};

async function requestJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {...init, cache: "no-store"});
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body?.error?.message || `Request failed (${response.status})`);
  return body as T;
}

export default function AttendanceWorkspace() {
  const [user, setUser] = useState<User | null>(null);
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [entries, setEntries] = useState<Attendance[]>([]);
  const [corrections, setCorrections] = useState<Correction[]>([]);
  const [summary, setSummary] = useState<Timesheet | null>(null);
  const [selectedEmployee, setSelectedEmployee] = useState("");
  const [workMode, setWorkMode] = useState("office");
  const [correctionDate, setCorrectionDate] = useState("");
  const [correctionIn, setCorrectionIn] = useState("");
  const [correctionOut, setCorrectionOut] = useState("");
  const [correctionReason, setCorrectionReason] = useState("");
  const [busy, setBusy] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const elevated = user?.role === "admin" || user?.role === "hr";
  const manager = user?.role === "manager";
  const today = localDateString(new Date());

  async function load(targetEmployee = selectedEmployee) {
    setLoading(true);
    try {
      const me = await requestJSON<{user: User}>("/api/hr/auth/me");
      const canSeeEmployees = me.user.role === "admin" || me.user.role === "hr";
      const employeeResponse = canSeeEmployees ? await requestJSON<{data: Employee[]}>("/api/hr/employees") : {data: [] as Employee[]};
      const target = canSeeEmployees ? (targetEmployee || employeeResponse.data?.[0]?.id || "") : "";
      if (canSeeEmployees && target) setSelectedEmployee(target);
      const [attendanceResponse, correctionResponse] = await Promise.all([
        requestJSON<{data: Attendance[]}>(`/api/hr/attendance?date=${today}`),
        requestJSON<{data: Correction[]}>("/api/hr/attendance/corrections"),
      ]);
      setUser(me.user);
      setEmployees(employeeResponse.data ?? []);
      setEntries(attendanceResponse.data ?? []);
      setCorrections(correctionResponse.data ?? []);
      const range = monthRange();
      if (!canSeeEmployees || target) {
        const employeeQuery = canSeeEmployees ? `&employee_id=${encodeURIComponent(target)}` : "";
        const sheet = await requestJSON<Timesheet>(`/api/hr/attendance/timesheet?from=${range.from}&to=${range.to}${employeeQuery}`);
        setSummary(sheet);
      }
      setError("");
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void load(); }, []);

  async function clock(action: "check-in" | "check-out") {
    if (!user) return;
    setBusy(action); setError("");
    try {
      await requestJSON(`/api/hr/attendance/${action}`, {
        method: "POST", headers: {"Content-Type": "application/json"},
        body: JSON.stringify({employee_id: elevated ? selectedEmployee : "", work_mode: workMode}),
      });
      await load();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function submitCorrection(event: FormEvent) {
    event.preventDefault();
    setBusy("correction"); setError("");
    try {
      await requestJSON("/api/hr/attendance/corrections", {
        method: "POST", headers: {"Content-Type": "application/json"},
        body: JSON.stringify({
          employee_id: elevated ? selectedEmployee : "",
          work_date: correctionDate,
          requested_check_in: correctionIn ? new Date(correctionIn).toISOString() : "",
          requested_check_out: correctionOut ? new Date(correctionOut).toISOString() : "",
          work_mode: workMode,
          reason: correctionReason,
        }),
      });
      setCorrectionDate(""); setCorrectionIn(""); setCorrectionOut(""); setCorrectionReason("");
      await load();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function decideCorrection(id: string, decision: "approved" | "rejected") {
    setBusy(id); setError("");
    try {
      await requestJSON(`/api/hr/attendance/corrections/${id}/decision`, {
        method: "POST", headers: {"Content-Type": "application/json"},
        body: JSON.stringify({decision, note: `${decision} from attendance workspace`}),
      });
      await load();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function changeEmployee(id: string) {
    setSelectedEmployee(id);
    await load(id);
  }

  const currentEntry = useMemo(() => {
    if (elevated) return entries.find((entry) => entry.employee_id === selectedEmployee);
    return entries.find((entry) => entry.employee === user?.display_name);
  }, [entries, selectedEmployee, elevated, user]);
  const pendingCorrections = corrections.filter((item) => item.status === "pending" && (!manager || item.employee !== user?.display_name));

  if (loading && !user) return <main className={styles.state}>Loading attendance…</main>;
  if (!user) return <main className={styles.state}><h1>Attendance unavailable</h1><p>{error || "Sign in first."}</p><a href="/">Return to overview</a></main>;

  return <main className={styles.shell}>
    <header className={styles.topbar}>
      <div><p>ADVANCE HRIS · ATTENDANCE</p><h1>Work time that explains itself.</h1><span>Schedules, corrections and payroll-ready time metrics for {user.organization}.</span></div>
      <nav><a href="/">Overview</a><a href="/leave">Leave</a>{elevated && <a href="/attendance/admin">Attendance admin</a>}<button onClick={() => void load()}>Refresh</button></nav>
    </header>

    {error && <div className={styles.error}>{error}</div>}
    {elevated && <section className={styles.selector}><label>Viewing employee<select value={selectedEmployee} onChange={(event) => void changeEmployee(event.target.value)}>{employees.map((employee) => <option key={employee.id} value={employee.id}>{employee.name} · {employee.department}</option>)}</select></label></section>}

    <section className={styles.metrics}>
      <Metric label="Worked this month" value={summary ? `${summary.worked_hours}h` : "—"} meta={summary ? `${summary.recorded_days}/${summary.scheduled_days} recorded days` : "No period data"} />
      <Metric label="Overtime" value={summary ? `${summary.overtime_hours}h` : "—"} meta="Approved/corrected attendance" />
      <Metric label="Late" value={summary ? `${summary.late_minutes}m` : "—"} meta="After schedule grace" />
      <Metric label="Early leave" value={summary ? `${summary.early_leave_minutes}m` : "—"} meta="Before scheduled end" />
    </section>

    <section className={styles.grid}>
      <article className={styles.panel}>
        <div className={styles.heading}><div><p>TODAY</p><h2>Clock state</h2></div><span className={currentEntry ? styles.good : styles.muted}>{currentEntry ? "Recorded" : "Not started"}</span></div>
        <label>Work mode<select value={workMode} onChange={(event) => setWorkMode(event.target.value)}><option value="office">Office</option><option value="remote">Remote</option><option value="hybrid">Hybrid</option><option value="field">Field</option></select></label>
        <div className={styles.clockActions}><button onClick={() => void clock("check-in")} disabled={Boolean(busy)}>Check in</button><button onClick={() => void clock("check-out")} disabled={Boolean(busy)}>Check out</button></div>
        <div className={styles.clockCard}><div><span>In</span><strong>{formatTime(currentEntry?.check_in)}</strong></div><div><span>Out</span><strong>{formatTime(currentEntry?.check_out)}</strong></div><div><span>Schedule</span><strong>{currentEntry?.schedule_name || "Default"}</strong></div></div>
        {currentEntry && <div className={styles.badges}><span>{currentEntry.late_minutes}m late</span><span>{formatMinutes(currentEntry.worked_minutes)} worked</span><span>{currentEntry.overtime_minutes}m overtime</span><span>{currentEntry.source}</span></div>}
      </article>

      <article className={styles.panel}>
        <div className={styles.heading}><div><p>CORRECTION</p><h2>Fix a time record</h2></div><span>Approval required</span></div>
        <form onSubmit={submitCorrection}>
          <label>Work date<input required type="date" value={correctionDate} onChange={(event) => setCorrectionDate(event.target.value)} /></label>
          <div className={styles.fields}><label>Correct check-in<input type="datetime-local" value={correctionIn} onChange={(event) => setCorrectionIn(event.target.value)} /></label><label>Correct check-out<input type="datetime-local" value={correctionOut} onChange={(event) => setCorrectionOut(event.target.value)} /></label></div>
          <label>Reason<textarea required value={correctionReason} onChange={(event) => setCorrectionReason(event.target.value)} placeholder="Why should this attendance record change?" /></label>
          <button disabled={busy === "correction" || (!correctionIn && !correctionOut)}>{busy === "correction" ? "Submitting…" : "Request correction"}</button>
        </form>
      </article>
    </section>

    {(elevated || manager) && <section className={styles.panel}>
      <div className={styles.heading}><div><p>REVIEW QUEUE</p><h2>Attendance corrections</h2></div><span>{pendingCorrections.length} pending</span></div>
      <div className={styles.list}>{pendingCorrections.length === 0 ? <div className={styles.empty}>Nothing needs review.</div> : pendingCorrections.map((item) => <div className={styles.row} key={item.id}><div><strong>{item.employee}</strong><span>{item.work_date} · {item.reason}</span><small>{formatTime(item.requested_check_in)} → {formatTime(item.requested_check_out)} · {item.requested_work_mode}</small></div><div><button onClick={() => void decideCorrection(item.id, "approved")} disabled={busy === item.id}>Approve</button><button className={styles.danger} onClick={() => void decideCorrection(item.id, "rejected")} disabled={busy === item.id}>Reject</button></div></div>)}</div>
    </section>}

    <section className={styles.panel}>
      <div className={styles.heading}><div><p>TODAY'S VISIBILITY</p><h2>{manager ? "You and your direct reports" : elevated ? "Organization attendance" : "Your attendance"}</h2></div><span>{entries.length} records</span></div>
      <div className={styles.list}>{entries.length === 0 ? <div className={styles.empty}>No attendance recorded for today.</div> : entries.map((entry) => <div className={styles.row} key={entry.id}><div><strong>{entry.employee}</strong><span>{entry.work_mode} · {entry.schedule_name || "Default schedule"}</span></div><div className={styles.rowMetrics}><span>{formatTime(entry.check_in)} → {formatTime(entry.check_out)}</span><small>{entry.late_minutes}m late · {entry.overtime_minutes}m OT</small></div></div>)}</div>
    </section>
  </main>;
}

function Metric({label, value, meta}: {label: string; value: string; meta: string}) { return <article><span>{label}</span><strong>{value}</strong><small>{meta}</small></article>; }
function formatTime(value?: string) { return value ? new Date(value).toLocaleTimeString([], {hour: "2-digit", minute: "2-digit"}) : "—"; }
function formatMinutes(value: number) { const h = Math.floor(value / 60); const m = value % 60; return `${h}h ${m}m`; }
function localDateString(date: Date) { return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`; }
function monthRange() { const now = new Date(); const from = new Date(now.getFullYear(), now.getMonth(), 1); const to = new Date(now.getFullYear(), now.getMonth() + 1, 0); return {from: localDateString(from), to: localDateString(to)}; }
