"use client";

import {FormEvent, useEffect, useState} from "react";
import styles from "./admin.module.css";

type User = {display_name: string; organization: string; role: string};
type Employee = {id: string; name: string; department: string};
type Schedule = {id: string; code: string; name: string; timezone: string; start_time: string; end_time: string; break_minutes: number; grace_minutes: number; overtime_threshold_minutes: number; work_days: number[]; is_default: boolean};
type Correction = {id: string; employee: string; work_date: string; requested_check_in?: string; requested_check_out?: string; requested_work_mode: string; reason: string; status: string};
type Summary = {employee: string; from: string; to: string; scheduled_days: number; recorded_days: number; worked_hours: number; overtime_hours: number; late_minutes: number; early_leave_minutes: number};

async function requestJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {...init, cache: "no-store"});
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body?.error?.message || `Request failed (${response.status})`);
  return body as T;
}

const weekdayOptions = [{v:1,l:"Mon"},{v:2,l:"Tue"},{v:3,l:"Wed"},{v:4,l:"Thu"},{v:5,l:"Fri"},{v:6,l:"Sat"},{v:0,l:"Sun"}];

export default function AttendanceAdmin() {
  const [user, setUser] = useState<User | null>(null);
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [schedules, setSchedules] = useState<Schedule[]>([]);
  const [corrections, setCorrections] = useState<Correction[]>([]);
  const [selectedEmployee, setSelectedEmployee] = useState("");
  const [selectedSchedule, setSelectedSchedule] = useState("");
  const [effectiveFrom, setEffectiveFrom] = useState(new Date().toISOString().slice(0,10));
  const [code, setCode] = useState("standard");
  const [name, setName] = useState("Standard workday");
  const [timezone, setTimezone] = useState("UTC");
  const [startTime, setStartTime] = useState("09:00");
  const [endTime, setEndTime] = useState("17:00");
  const [breakMinutes, setBreakMinutes] = useState("60");
  const [graceMinutes, setGraceMinutes] = useState("10");
  const [overtimeThreshold, setOvertimeThreshold] = useState("15");
  const [workDays, setWorkDays] = useState<number[]>([1,2,3,4,5]);
  const [isDefault, setIsDefault] = useState(true);
  const [from, setFrom] = useState(monthRange().from);
  const [to, setTo] = useState(monthRange().to);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [busy, setBusy] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  async function load() {
    setLoading(true);
    try {
      const me = await requestJSON<{user: User}>("/api/hr/auth/me");
      if (me.user.role !== "admin" && me.user.role !== "hr") throw new Error("HR or admin access is required.");
      const [scheduleResponse, employeeResponse, correctionResponse] = await Promise.all([
        requestJSON<{data: Schedule[]}>("/api/hr/attendance/schedules"),
        requestJSON<{data: Employee[]}>("/api/hr/employees"),
        requestJSON<{data: Correction[]}>("/api/hr/attendance/corrections"),
      ]);
      setUser(me.user);
      setSchedules(scheduleResponse.data ?? []);
      setEmployees(employeeResponse.data ?? []);
      setCorrections(correctionResponse.data ?? []);
      const firstSchedule = scheduleResponse.data?.[0];
      const firstEmployee = employeeResponse.data?.[0];
      if (!selectedSchedule && firstSchedule) { setSelectedSchedule(firstSchedule.id); chooseSchedule(firstSchedule); }
      if (!selectedEmployee && firstEmployee) setSelectedEmployee(firstEmployee.id);
      setError("");
    } catch (err) { setError(String(err)); }
    finally { setLoading(false); }
  }

  useEffect(() => { void load(); }, []);

  function chooseSchedule(schedule: Schedule) {
    setSelectedSchedule(schedule.id); setCode(schedule.code); setName(schedule.name); setTimezone(schedule.timezone);
    setStartTime(schedule.start_time); setEndTime(schedule.end_time); setBreakMinutes(String(schedule.break_minutes));
    setGraceMinutes(String(schedule.grace_minutes)); setOvertimeThreshold(String(schedule.overtime_threshold_minutes));
    setWorkDays(schedule.work_days); setIsDefault(schedule.is_default);
  }

  function toggleDay(day: number) { setWorkDays((current) => current.includes(day) ? current.filter((value) => value !== day) : [...current, day]); }

  async function saveSchedule(event: FormEvent) {
    event.preventDefault(); setBusy("schedule"); setError("");
    try {
      const saved = await requestJSON<Schedule>("/api/hr/attendance/schedules", {
        method: "PUT", headers: {"Content-Type":"application/json"},
        body: JSON.stringify({code,name,timezone,start_time:startTime,end_time:endTime,break_minutes:Number(breakMinutes),grace_minutes:Number(graceMinutes),overtime_threshold_minutes:Number(overtimeThreshold),work_days:workDays,is_default:isDefault}),
      });
      setSelectedSchedule(saved.id); await load();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function assignSchedule(event: FormEvent) {
    event.preventDefault(); setBusy("assign"); setError("");
    try {
      await requestJSON(`/api/hr/attendance/schedules/${selectedSchedule}/assignments`, {method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({employee_id:selectedEmployee,effective_from:effectiveFrom,effective_to:""})});
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function decide(id: string, decision: "approved"|"rejected") {
    setBusy(id); setError("");
    try {
      await requestJSON(`/api/hr/attendance/corrections/${id}/decision`, {method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({decision,note:`${decision} by HR`})});
      await load();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function loadSummary(event?: FormEvent) {
    event?.preventDefault(); if (!selectedEmployee) return; setBusy("summary"); setError("");
    try { setSummary(await requestJSON<Summary>(`/api/hr/attendance/timesheet?from=${from}&to=${to}&employee_id=${encodeURIComponent(selectedEmployee)}`)); }
    catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  if (loading && !user) return <main className={styles.state}>Loading attendance administration…</main>;
  if (!user) return <main className={styles.state}><h1>Attendance administration unavailable</h1><p>{error}</p><a href="/">Return to overview</a></main>;

  const pending = corrections.filter((item) => item.status === "pending");
  return <main className={styles.shell}>
    <header><div><p>ADVANCE HRIS · ATTENDANCE ADMIN</p><h1>Schedules, exceptions and payroll time.</h1><span>{user.organization} · {user.role}</span></div><nav><a href="/attendance">Attendance</a><a href="/leave/admin">Leave admin</a><a href="/">Overview</a></nav></header>
    {error && <div className={styles.error}>{error}</div>}

    <section className={styles.grid}>
      <article className={styles.panel}>
        <div className={styles.heading}><div><p>SCHEDULE POLICY</p><h2>Create or update a schedule</h2></div></div>
        <div className={styles.scheduleTabs}>{schedules.map((schedule) => <button type="button" className={selectedSchedule===schedule.id?styles.active:""} key={schedule.id} onClick={() => chooseSchedule(schedule)}>{schedule.name}{schedule.is_default?" · default":""}</button>)}</div>
        <form onSubmit={saveSchedule}>
          <div className={styles.fields}><label>Code<input required value={code} onChange={(e)=>setCode(e.target.value)} /></label><label>Name<input required value={name} onChange={(e)=>setName(e.target.value)} /></label></div>
          <label>Timezone<input required value={timezone} onChange={(e)=>setTimezone(e.target.value)} placeholder="Asia/Colombo" /></label>
          <div className={styles.fields}><label>Start<input required type="time" value={startTime} onChange={(e)=>setStartTime(e.target.value)} /></label><label>End<input required type="time" value={endTime} onChange={(e)=>setEndTime(e.target.value)} /></label></div>
          <div className={styles.fields}><label>Break minutes<input type="number" min="0" value={breakMinutes} onChange={(e)=>setBreakMinutes(e.target.value)} /></label><label>Grace minutes<input type="number" min="0" value={graceMinutes} onChange={(e)=>setGraceMinutes(e.target.value)} /></label></div>
          <label>Overtime threshold minutes<input type="number" min="0" value={overtimeThreshold} onChange={(e)=>setOvertimeThreshold(e.target.value)} /></label>
          <div className={styles.days}>{weekdayOptions.map((day)=><label key={day.v}><input type="checkbox" checked={workDays.includes(day.v)} onChange={()=>toggleDay(day.v)} />{day.l}</label>)}</div>
          <label className={styles.check}><input type="checkbox" checked={isDefault} onChange={(e)=>setIsDefault(e.target.checked)} />Organization default schedule</label>
          <button disabled={busy==="schedule"}>{busy==="schedule"?"Saving…":"Save schedule"}</button>
        </form>
      </article>

      <article className={styles.panel}>
        <div className={styles.heading}><div><p>ASSIGNMENT</p><h2>Employee schedule</h2></div></div>
        <form onSubmit={assignSchedule}>
          <label>Employee<select value={selectedEmployee} onChange={(e)=>setSelectedEmployee(e.target.value)}>{employees.map((employee)=><option key={employee.id} value={employee.id}>{employee.name} · {employee.department}</option>)}</select></label>
          <label>Schedule<select value={selectedSchedule} onChange={(e)=>{const schedule=schedules.find((item)=>item.id===e.target.value); if(schedule) chooseSchedule(schedule);}}>{schedules.map((schedule)=><option key={schedule.id} value={schedule.id}>{schedule.name}</option>)}</select></label>
          <label>Effective from<input type="date" required value={effectiveFrom} onChange={(e)=>setEffectiveFrom(e.target.value)} /></label>
          <button disabled={busy==="assign" || !selectedSchedule || !selectedEmployee}>Assign schedule</button>
        </form>
      </article>

      <article className={styles.panel}>
        <div className={styles.heading}><div><p>PAYROLL PERIOD</p><h2>Time summary</h2></div></div>
        <form onSubmit={loadSummary}>
          <label>Employee<select value={selectedEmployee} onChange={(e)=>setSelectedEmployee(e.target.value)}>{employees.map((employee)=><option key={employee.id} value={employee.id}>{employee.name}</option>)}</select></label>
          <div className={styles.fields}><label>From<input type="date" value={from} onChange={(e)=>setFrom(e.target.value)} /></label><label>To<input type="date" value={to} onChange={(e)=>setTo(e.target.value)} /></label></div>
          <button disabled={busy==="summary"}>Build summary</button>
        </form>
        {summary && <div className={styles.summary}><strong>{summary.employee}</strong><div><span>{summary.worked_hours}h worked</span><span>{summary.overtime_hours}h overtime</span><span>{summary.late_minutes}m late</span><span>{summary.recorded_days}/{summary.scheduled_days} days</span></div></div>}
      </article>

      <article className={styles.panel}>
        <div className={styles.heading}><div><p>CORRECTIONS</p><h2>Review queue</h2></div><span>{pending.length} pending</span></div>
        <div className={styles.list}>{pending.length===0?<div className={styles.empty}>Nothing needs review.</div>:pending.map((item)=><div className={styles.row} key={item.id}><div><strong>{item.employee}</strong><span>{item.work_date} · {item.reason}</span><small>{formatTime(item.requested_check_in)} → {formatTime(item.requested_check_out)}</small></div><div><button onClick={()=>void decide(item.id,"approved")} disabled={busy===item.id}>Approve</button><button className={styles.danger} onClick={()=>void decide(item.id,"rejected")} disabled={busy===item.id}>Reject</button></div></div>)}</div>
      </article>
    </section>
  </main>;
}

function formatTime(value?: string){return value?new Date(value).toLocaleString([], {month:"short",day:"numeric",hour:"2-digit",minute:"2-digit"}):"—";}
function monthRange(){const now=new Date();const first=new Date(now.getFullYear(),now.getMonth(),1);const last=new Date(now.getFullYear(),now.getMonth()+1,0);const f=(d:Date)=>`${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,"0")}-${String(d.getDate()).padStart(2,"0")}`;return{from:f(first),to:f(last)};}
