"use client";

import {FormEvent, useEffect, useState} from "react";
import styles from "./accept.module.css";

type Preview = {
  email: string;
  role: string;
  employee_id?: string;
  employee?: string;
  organization: string;
  expires_at: string;
  existing_user: boolean;
};

async function requestJSON<T>(url: string, body: unknown): Promise<T> {
  const response = await fetch(url, {
    method: "POST",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify(body),
    cache: "no-store",
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(payload?.error?.message || `Request failed (${response.status})`);
  return payload as T;
}

export default function AcceptClient({token}: {token: string}) {
  const [preview, setPreview] = useState<Preview | null>(null);
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) {
      setError("This invitation link is missing its token.");
      setLoading(false);
      return;
    }
    // Keep the secret out of the address bar/history after the initial page load.
    window.history.replaceState(null, "", "/accept");
    void requestJSON<Preview>("/api/invitations/inspect", {token})
      .then((next) => {
        setPreview(next);
        setDisplayName(next.employee || "");
      })
      .catch((err) => setError(String(err)))
      .finally(() => setLoading(false));
  }, [token]);

  async function accept(event: FormEvent) {
    event.preventDefault();
    if (!preview) return;
    setBusy(true);
    setError("");
    try {
      await requestJSON("/api/invitations/accept", {
        token,
        display_name: displayName,
        password: preview.existing_user ? "" : password,
      });
      window.location.assign("/");
    } catch (err) {
      setError(String(err));
      setBusy(false);
    }
  }

  return <main className={styles.shell}>
    <section className={styles.hero}>
      <div className={styles.brand}><span>A</span><div><strong>Advance HRIS</strong><small>Secure workspace onboarding</small></div></div>
      <div className={styles.copy}>
        <p>ORGANIZATION INVITATION</p>
        <h1>Join your team without compromising account security.</h1>
        <span>The invitation is one-time, expires automatically, and is exchanged for a normal role-scoped HRIS session after acceptance.</span>
      </div>
      <div className={styles.security}><span>One-time token</span><span>Role-scoped access</span><span>HttpOnly web session</span></div>
    </section>

    <section className={styles.formWrap}>
      <div className={styles.card}>
        {loading && <div className={styles.state}><strong>Checking invitation…</strong><span>Validating the one-time token.</span></div>}
        {!loading && error && !preview && <div className={styles.state}><strong>Invitation unavailable</strong><span>{error}</span><a href="/">Return to sign in</a></div>}
        {!loading && preview && <form onSubmit={accept}>
          <p className={styles.kicker}>YOU’RE INVITED</p>
          <h2>Join {preview.organization}</h2>
          <p className={styles.intro}>You’ll receive <strong>{preview.role}</strong> access{preview.employee ? ` linked to ${preview.employee}` : ""}.</p>

          <div className={styles.summary}>
            <div><span>Email</span><strong>{preview.email}</strong></div>
            <div><span>Role</span><strong>{preview.role}</strong></div>
            <div><span>Expires</span><strong>{new Date(preview.expires_at).toLocaleString()}</strong></div>
          </div>

          {error && <div className={styles.error}>{error}</div>}
          {preview.existing_user ? <div className={styles.existing}>An Advance HRIS account already exists for this email. Accepting adds this organization to that account without changing its password.</div> : <>
            <label>Display name<input required value={displayName} onChange={(event) => setDisplayName(event.target.value)} autoComplete="name" /></label>
            <label>Create password<input type="password" required minLength={12} value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="new-password" /><small>Use at least 12 characters.</small></label>
          </>}
          <button disabled={busy}>{busy ? "Joining workspace…" : preview.existing_user ? "Accept & join workspace" : "Create account & join"}</button>
          <small className={styles.footnote}>The invitation token is consumed after successful acceptance and cannot be reused.</small>
        </form>}
      </div>
    </section>
  </main>;
}
