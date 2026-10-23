"use client";

import { useState, type FormEvent } from "react";
import styles from "./login.module.css";

// Shown when the backend cannot be reached or gives no SoT error (§12).
const unavailableMessage = "Layanan sedang tidak tersedia. Silakan coba beberapa saat lagi.";

// UI-001: Logo, title, [error banner], Username, Password, Login. Nothing else.
// The password lives only in component state: never in the URL or browser
// storage (FR-LOGIN-004).
export default function LoginForm({ loginPath, endpointPath }: { loginPath: string; endpointPath: string }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    // FR-LOGIN-001: with either field empty, no request is sent.
    if (username === "" || password === "" || submitting) {
      return;
    }
    setSubmitting(true);
    try {
      // API-001/API-002: POST to the login path with username and password only.
      const res = await fetch(loginPath, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });
      if (res.ok) {
        window.location.assign(endpointPath);
        return;
      }
      const body = (await res.json().catch(() => null)) as { message?: string } | null;
      fail(body?.message ?? unavailableMessage);
    } catch {
      fail(unavailableMessage);
    }
  }

  // FR-LOGIN-005: show the banner and clear both fields.
  function fail(message: string) {
    setError(message);
    setUsername("");
    setPassword("");
    setSubmitting(false);
  }

  return (
    <main className={styles.page}>
      <form className={styles.form} onSubmit={onSubmit} noValidate autoComplete="off">
        {/* Logo asset is TBD-014; the wordmark stands in until it is supplied. */}
        <div className={styles.logo}>nanobank syariah</div>
        <h1 className={styles.title}>Financing Origination System</h1>
        {error && (
          <div className={styles.banner} role="alert">
            {error}
          </div>
        )}
        <div className={styles.field}>
          <span className={styles.icon} aria-hidden="true">
            <UserIcon />
          </span>
          <input
            className={styles.input}
            type="text"
            name="username"
            placeholder="Username"
            aria-label="Username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
        </div>
        <div className={`${styles.field} ${styles.passwordField}`}>
          <span className={styles.icon} aria-hidden="true">
            <LockIcon />
          </span>
          <input
            className={styles.input}
            type="password"
            name="password"
            placeholder="Password"
            aria-label="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        <button className={styles.button} type="submit" disabled={submitting}>
          Login
        </button>
      </form>
    </main>
  );
}

function UserIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="#ffffff">
      <circle cx="8" cy="5" r="3.2" />
      <path d="M1.8 15c0-3.4 2.8-5.6 6.2-5.6s6.2 2.2 6.2 5.6z" />
    </svg>
  );
}

function LockIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="#ffffff">
      <path d="M4.5 7V5a3.5 3.5 0 0 1 7 0v2h-1.6V5a1.9 1.9 0 0 0-3.8 0v2z" />
      <rect x="2.5" y="7" width="11" height="8" rx="1" />
    </svg>
  );
}
