"use client";

import {FormEvent, useEffect, useState} from "react";
import styles from "./admin.module.css";

type User = {display_name: string; organization: string; role: string};
type Employee = {id: string; name: string; department: string};
type Policy = {id: string; code: string; name: string; leave_type: string; annual_entitlement: number; carry_over_limit: number; track_balance: boolean; allow_negative: boolean; requires_approval: boolean; is_default: boolean};
type Holiday = {id: string; date: string; name: string; location?: string};

async function requestJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {...init, cache: "no-store"});
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body?.error?.message || `Request failed (${response.status})`);
  return body as T;
}

export default function LeaveAdmin() {
  const [user, setUser] = useState<User | null>(null);
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [holidays, setHolidays] = useState<Holiday[]>([]);
  const [selectedPolicy, setSelectedPolicy] = useState("");
  const [selectedEmployee, setSelectedEmployee] = useState("");
  const [entitlement, setEntitlement] = useState("20");
  const [carryOver, setCarryOver] = useState("5");
  const [allowNegative, setAllowNegative] = useState(false);
  const [newPolicyCode, setNewPolicyCode] = useState("");
  const [newPolicyName, setNewPolicyName] = useState("");
  const [newPolicyType, setNewPolicyType] = useState("annual");
  const [newPolicyEntitlement, setNewPolicyEntitlement] = useState("20");
  const [newPolicyCarryOver, setNewPolicyCarryOver] = useState("0");
  const [newPolicyTrackBalance, setNewPolicyTrackBalance] = useState(true);
  const [holidayDate, setHolidayDate] = useState("");
  const [holidayName, setHolidayName] = useState("");
  const [holidayLocation, setHolidayLocation] = useState("");
  const [adjustment, setAdjustment] = useState("");
  const [adjustmentNote, setAdjustmentNote] = useState("");
  const [effectiveFrom, setEffectiveFrom] = useState(new Date().toISOString().slice(0, 10));
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    try {
      const me = await requestJSON<{user: User}>("/api/hr/auth/me");
      if (me.user.role !== "admin" && me.user.role !== "hr") throw new Error("HR or admin access is required.");
      const [policyResponse, employeeResponse, holidayResponse] = await Promise.all([
        requestJSON<{data: Policy[]}>("/api/hr/leave/policies"),
        requestJSON<{data: Employee[]}>("/api/hr/employees"),
        requestJSON<{data: Holiday[]}>(`/api/hr/leave/holidays?year=${new Date().getFullYear()}`),
      ]);
      const nextPolicies = policyResponse.data ?? [];
      setUser(me.user);
      setPolicies(nextPolicies);
      setEmployees(employeeResponse.data ?? []);
      setHolidays(holidayResponse.data ?? []);
      const chosen = nextPolicies.find((policy) => policy.id === selectedPolicy) ?? nextPolicies[0];
      if (chosen) {
        setSelectedPolicy(chosen.id);
        setEntitlement(String(chosen.annual_entitlement));
        setCarryOver(String(chosen.carry_over_limit));
        setAllowNegative(chosen.allow_negative);
      }
      if (!selectedEmployee && employeeResponse.data?.[0]) setSelectedEmployee(employeeResponse.data[0].id);
      setError("");
    } catch (err) { setError(String(err)); }
    finally { setLoading(false); }
  }

  useEffect(() => { void load(); }, []);

  function choosePolicy(policyID: string) {
    setSelectedPolicy(policyID);
    const policy = policies.find((item) => item.id === policyID);
    if (policy) {
      setEntitlement(String(policy.annual_entitlement));
      setCarryOver(String(policy.carry_over_limit));
      setAllowNegative(policy.allow_negative);
    }
  }

  async function createPolicy(event: FormEvent) {
    event.preventDefault(); setBusy("create-policy"); setError("");
    try {
      const created = await requestJSON<Policy>("/api/hr/leave/policies", {
        method: "PUT", headers: {"Content-Type": "application/json"},
        body: JSON.stringify({
          code: newPolicyCode.trim().toLowerCase(),
          name: newPolicyName.trim(),
          leave_type: newPolicyType,
          annual_entitlement: Number(newPolicyEntitlement),
          carry_over_limit: Number(newPolicyCarryOver),
          track_balance: newPolicyTrackBalance,
          allow_negative: false,
          requires_approval: true,
        }),
      });
      setNewPolicyCode(""); setNewPolicyName(""); setNewPolicyEntitlement("20"); setNewPolicyCarryOver("0");
      await load();
      choosePolicy(created.id);
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function savePolicy(event: FormEvent) {
    event.preventDefault();
    const policy = policies.find((item) => item.id === selectedPolicy);
    if (!policy) return;
    setBusy("policy"); setError("");
    try {
      await requestJSON("/api/hr/leave/policies", {
        method: "PUT", headers: {"Content-Type": "application/json"},
        body: JSON.stringify({
          code: policy.code,
          name: policy.name,
          leave_type: policy.leave_type,
          annual_entitlement: Number(entitlement),
          carry_over_limit: Number(carryOver),
          track_balance: policy.track_balance,
          allow_negative: allowNegative,
          requires_approval: true,
        }),
      });
      await load();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function assignPolicy(event: FormEvent) {
    event.preventDefault(); setBusy("assign"); setError("");
    try {
      await requestJSON(`/api/hr/leave/policies/${selectedPolicy}/assignments`, {
        method: "POST", headers: {"Content-Type": "application/json"},
        body: JSON.stringify({employee_id: selectedEmployee, effective_from: effectiveFrom, effective_to: ""}),
      });
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function addHoliday(event: FormEvent) {
    event.preventDefault(); setBusy("holiday"); setError("");
    try {
      await requestJSON("/api/hr/leave/holidays", {
        method: "POST", headers: {"Content-Type": "application/json"},
        body: JSON.stringify({date: holidayDate, name: holidayName, location: holidayLocation}),
      });
      setHolidayDate(""); setHolidayName(""); setHolidayLocation(""); await load();
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function removeHoliday(id: string) {
    setBusy(id); setError("");
    try { await requestJSON(`/api/hr/leave/holidays/${id}`, {method: "DELETE"}); await load(); }
    catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  async function adjustBalance(event: FormEvent) {
    event.preventDefault(); setBusy("adjust"); setError("");
    try {
      await requestJSON("/api/hr/leave/balances/adjustments", {
        method: "POST", headers: {"Content-Type": "application/json"},
        body: JSON.stringify({employee_id: selectedEmployee, policy_id: selectedPolicy, year: new Date().getFullYear(), amount: Number(adjustment), note: adjustmentNote}),
      });
      setAdjustment(""); setAdjustmentNote("");
    } catch (err) { setError(String(err)); }
    finally { setBusy(""); }
  }

  if (loading && !user) return <main className={styles.state}>Loading leave administration…</main>;
  if (!user) return <main className={styles.state}><h1>Leave administration unavailable</h1><p>{error}</p><a href="/">Return to overview</a></main>;

  return <main className={styles.shell}>
    <header><div><p>ADVANCE HRIS · LEAVE ADMIN</p><h1>Policies, calendars and balance controls.</h1><span>{user.organization} · {user.role}</span></div><nav><a href="/leave">Leave workspace</a><a href="/">Overview</a></nav></header>
    {error && <div className={styles.error}>{error}</div>}

    <section className={styles.grid}>
      <article className={styles.panel}>
        <div className={styles.heading}><div><p>NEW POLICY</p><h2>Create a policy variant</h2></div><span>Employee-specific</span></div>
        <form onSubmit={createPolicy}>
          <div className={styles.fields}><label>Policy code<input required value={newPolicyCode} onChange={(event) => setNewPolicyCode(event.target.value)} placeholder="annual-executive" /></label><label>Name<input required value={newPolicyName} onChange={(event) => setNewPolicyName(event.target.value)} placeholder="Executive annual leave" /></label></div>
          <div className={styles.fields}><label>Leave type<select value={newPolicyType} onChange={(event) => setNewPolicyType(event.target.value)}><option value="annual">Annual</option><option value="sick">Sick</option><option value="parental">Parental</option><option value="unpaid">Unpaid</option><option value="remote">Remote</option><option value="other">Other</option></select></label><label>Annual entitlement<input type="number" min="0" step="0.5" value={newPolicyEntitlement} onChange={(event) => setNewPolicyEntitlement(event.target.value)} /></label></div>
          <label>Carry-over cap<input type="number" min="0" step="0.5" value={newPolicyCarryOver} onChange={(event) => setNewPolicyCarryOver(event.target.value)} /></label>
          <label className={styles.check}><input type="checkbox" checked={newPolicyTrackBalance} onChange={(event) => setNewPolicyTrackBalance(event.target.checked)} />Track an entitlement balance for this policy</label>
          <button disabled={busy === "create-policy"}>{busy === "create-policy" ? "Creating…" : "Create policy"}</button>
        </form>
      </article>

      <article className={styles.panel}>
        <div className={styles.heading}><div><p>POLICY</p><h2>Entitlements & carry-over</h2></div></div>
        <form onSubmit={savePolicy}>
          <label>Policy<select value={selectedPolicy} onChange={(event) => choosePolicy(event.target.value)}>{policies.map((policy) => <option key={policy.id} value={policy.id}>{policy.name}{policy.is_default ? " · default" : ""}</option>)}</select></label>
          <div className={styles.fields}><label>Annual entitlement<input type="number" min="0" step="0.5" value={entitlement} onChange={(event) => setEntitlement(event.target.value)} /></label><label>Carry-over cap<input type="number" min="0" step="0.5" value={carryOver} onChange={(event) => setCarryOver(event.target.value)} /></label></div>
          <label className={styles.check}><input type="checkbox" checked={allowNegative} onChange={(event) => setAllowNegative(event.target.checked)} />Allow negative balance</label>
          <button disabled={busy === "policy"}>{busy === "policy" ? "Saving…" : "Save policy"}</button>
        </form>
      </article>

      <article className={styles.panel}>
        <div className={styles.heading}><div><p>ASSIGNMENT</p><h2>Employee policy assignment</h2></div></div>
        <form onSubmit={assignPolicy}>
          <label>Employee<select value={selectedEmployee} onChange={(event) => setSelectedEmployee(event.target.value)}>{employees.map((employee) => <option key={employee.id} value={employee.id}>{employee.name} · {employee.department}</option>)}</select></label>
          <label>Policy<select value={selectedPolicy} onChange={(event) => choosePolicy(event.target.value)}>{policies.map((policy) => <option key={policy.id} value={policy.id}>{policy.name}{policy.is_default ? " · default" : ""}</option>)}</select></label>
          <label>Effective from<input type="date" value={effectiveFrom} onChange={(event) => setEffectiveFrom(event.target.value)} /></label>
          <button disabled={busy === "assign"}>Assign policy</button>
        </form>
      </article>

      <article className={styles.panel}>
        <div className={styles.heading}><div><p>BALANCE LEDGER</p><h2>Manual adjustment</h2></div></div>
        <form onSubmit={adjustBalance}>
          <label>Employee<select value={selectedEmployee} onChange={(event) => setSelectedEmployee(event.target.value)}>{employees.map((employee) => <option key={employee.id} value={employee.id}>{employee.name}</option>)}</select></label>
          <label>Policy<select value={selectedPolicy} onChange={(event) => choosePolicy(event.target.value)}>{policies.filter((policy) => policy.track_balance).map((policy) => <option key={policy.id} value={policy.id}>{policy.name}</option>)}</select></label>
          <div className={styles.fields}><label>Days (+ / -)<input required type="number" step="0.5" value={adjustment} onChange={(event) => setAdjustment(event.target.value)} /></label><label>Reason<input required value={adjustmentNote} onChange={(event) => setAdjustmentNote(event.target.value)} /></label></div>
          <button disabled={busy === "adjust"}>Post adjustment</button>
        </form>
      </article>

      <article className={styles.panel}>
        <div className={styles.heading}><div><p>CALENDAR</p><h2>Company holidays</h2></div></div>
        <form onSubmit={addHoliday}>
          <div className={styles.fields}><label>Date<input required type="date" value={holidayDate} onChange={(event) => setHolidayDate(event.target.value)} /></label><label>Name<input required value={holidayName} onChange={(event) => setHolidayName(event.target.value)} /></label></div>
          <label>Location (optional)<input value={holidayLocation} onChange={(event) => setHolidayLocation(event.target.value)} placeholder="Blank applies company-wide" /></label>
          <button disabled={busy === "holiday"}>Add holiday</button>
        </form>
        <div className={styles.list}>{holidays.map((holiday) => <div key={holiday.id}><span><strong>{holiday.name}</strong><small>{holiday.date} · {holiday.location || "All locations"}</small></span><button type="button" onClick={() => void removeHoliday(holiday.id)} disabled={busy === holiday.id}>Remove</button></div>)}</div>
      </article>
    </section>
  </main>;
}
