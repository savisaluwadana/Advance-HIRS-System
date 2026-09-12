"use client";

import {FormEvent, useEffect, useMemo, useState} from "react";
import styles from "./access.module.css";

type User = {display_name: string; organization: string; role: string};
type Employee = {id: string; name: string; work_email?: string; role: string; department: string; status: string};
type Invitation = {id: string; email: string; role: string; employee_id?: string; employee?: string; expires_at: string; accepted_at?: string; revoked_at?: string; created_at: string};
type InvitationCreate = {invitation: Invitation; token: string; notice: string};

async function requestJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {...init, cache: "no-store"});
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(payload?.error?.message || `Request failed (${response.status})`);
  return payload as T;
}

export default function AccessPage() {
  const [user, setUser] = useState<User | null>(null);
  const [employees, setEmployees] = useState<Employee[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("employee");
  const [employeeID, setEmployeeID] = useState("");
  const [latestLink, setLatestLink] = useState("");
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    try {
      const me = await requestJSON<{user: User}>("/api/hr/auth/me");
      if (me.user.role !== "admin" && me.user.role !== "hr") throw new Error("Access administration is limited to HR and administrators.");
      const [employeeResponse, invitationResponse] = await Promise.all([
        requestJSON<{data: Employee[]}>("/api/hr/employees"),
        requestJSON<{data: Invitation[]}>("/api/hr/access/invitations"),
      ]);
      setUser(me.user);
      setEmployees(employeeResponse.data ?? []);
      setInvitations(invitationResponse.data ?? []);
      setEmployeeID((current) => current || employeeResponse.data?.[0]?.id || "");
      setError("");
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void load(); }, []);

  const selectedEmployee = useMemo(() => employees.find((item) => item.id === employeeID), [employees, employeeID]);
  const needsEmployee = role === "employee" || role === "manager";
  const pending = invitations.filter((item) => !item.accepted_at && !item.revoked_at && new Date(item.expires_at).getTime() > Date.now());

  function chooseEmployee(value: string) {
    setEmployeeID(value);
    const employee = employees.find((item) => item.id === value);
    if (employee?.work_email) setEmail(employee.work_email);
  }

  async function createInvitation(event: FormEvent) {
    event.preventDefault();
    setBusy("create");
    setError("");
    setLatestLink("");
    try {
      const result = await requestJSON<InvitationCreate>("/api/hr/access/invitations", {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify({email, role, employee_id: needsEmployee ? employeeID : ""}),
      });
      setLatestLink(`${window.location.origin}/accept?token=${encodeURIComponent(result.token)}`);
      await load();
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy("");
    }
  }

  async function revoke(id: string) {
    setBusy(id);
    setError("");
    try {
      await requestJSON(`/api/hr/access/invitations/${id}/revoke`, {method: "POST"});
      await load();
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy("");
    }
  }

  async function copyLink() {
    if (!latestLink) return;
    try { await navigator.clipboard.writeText(latestLink); } catch { setError("Could not copy automatically. Select the invitation link and copy it manually."); }
  }

  if (loading) return <main className={styles.loading}>Loading access administration…</main>;

  return <main className={styles.shell}>
    <header className={styles.header}>
      <div><a href="/">← HRIS overview</a><p>ACCESS ADMINISTRATION</p><h1>Control who can enter {user?.organization || "this workspace"}.</h1><span>Issue one-time access links, connect users to employee records and revoke pending invitations.</span></div>
      <div className={styles.identity}><strong>{user?.display_name || "HR user"}</strong><small>{user?.role}</small></div>
    </header>

    {error && <div className={styles.error}>{error}</div>}
    <section className={styles.grid}>
      <article className={styles.card}>
        <div className={styles.cardHeading}><div><p>NEW INVITATION</p><h2>Invite a workspace member</h2></div><span>48h expiry</span></div>
        <form onSubmit={createInvitation} className={styles.form}>
          <label>Role<select value={role} onChange={(event) => {setRole(event.target.value); setLatestLink("");}}>
            <option value="employee">Employee</option><option value="manager">Manager</option>{user?.role === "admin" && <><option value="hr">HR</option><option value="admin">Administrator</option></>}
          </select></label>
          {needsEmployee && <label>Employee profile<select required value={employeeID} onChange={(event) => chooseEmployee(event.target.value)}>{employees.filter((item) => item.status !== "inactive").map((employee) => <option value={employee.id} key={employee.id}>{employee.name} · {employee.department}</option>)}</select><small>Employee and manager invitations must use the employee’s work email.</small></label>}
          <label>Email<input type="email" required value={email} onChange={(event) => setEmail(event.target.value)} readOnly={needsEmployee && Boolean(selectedEmployee?.work_email)} /></label>
          <button disabled={busy === "create"}>{busy === "create" ? "Creating secure link…" : "Create invitation"}</button>
        </form>

        {latestLink && <div className={styles.oneTime}><strong>One-time invitation link</strong><p>This raw token is not stored by Advance HRIS. Copy it now; creating a new invite for the same email invalidates this one.</p><div><input readOnly value={latestLink} onFocus={(event) => event.currentTarget.select()} /><button onClick={copyLink}>Copy</button></div></div>}
      </article>

      <article className={styles.card}>
        <div className={styles.cardHeading}><div><p>PENDING ACCESS</p><h2>Active invitations</h2></div><span>{pending.length} pending</span></div>
        <div className={styles.list}>{pending.length === 0 ? <div className={styles.empty}>No pending invitations.</div> : pending.map((invite) => <div className={styles.row} key={invite.id}><div><strong>{invite.email}</strong><span>{invite.role}{invite.employee ? ` · ${invite.employee}` : ""}</span><small>Expires {new Date(invite.expires_at).toLocaleString()}</small></div><button disabled={busy === invite.id} onClick={() => revoke(invite.id)}>{busy === invite.id ? "Revoking…" : "Revoke"}</button></div>)}</div>
      </article>
    </section>

    <section className={styles.card}>
      <div className={styles.cardHeading}><div><p>ACCESS HISTORY</p><h2>Recent invitations</h2></div><button className={styles.refresh} onClick={load}>Refresh</button></div>
      <div className={styles.history}>{invitations.slice(0, 15).map((invite) => {
        const state = invite.accepted_at ? "Accepted" : invite.revoked_at ? "Revoked" : new Date(invite.expires_at).getTime() <= Date.now() ? "Expired" : "Pending";
        return <div className={styles.historyRow} key={invite.id}><div><strong>{invite.email}</strong><span>{invite.role}{invite.employee ? ` · ${invite.employee}` : ""}</span></div><span data-state={state.toLowerCase()}>{state}</span><small>{new Date(invite.created_at).toLocaleString()}</small></div>;
      })}</div>
    </section>
  </main>;
}
