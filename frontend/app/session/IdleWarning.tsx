"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { apiFetch, serverRequestEvent } from "@/lib/api";
import styles from "./idle.module.css";

const warnAfterMs = 14 * 60 * 1000; // FR-ACCESS-005: warning after 14 minutes
const countdownSeconds = 60; //         then 1:00 to log off (TBD-040: configurable?)

// UI-019 "Log off timeout warning". The server ends the session at 15
// minutes regardless (FR-ACCESS-005 rule 2); this popup only warns.
export default function IdleWarning({ endpointPath, loginPath }: { endpointPath: string; loginPath: string }) {
  const [remaining, setRemaining] = useState<number | null>(null);
  const warnTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const showing = remaining !== null;

  const restart = useCallback(() => {
    clearTimeout(warnTimer.current);
    warnTimer.current = setTimeout(() => setRemaining(countdownSeconds), warnAfterMs);
  }, []);

  // Each server request restarts the idle timer, except while the popup is
  // shown: then only OK does (FR-ACCESS-005 rule 1).
  useEffect(() => {
    restart();
    const onRequest = () => {
      if (!showing) {
        restart();
      }
    };
    window.addEventListener(serverRequestEvent, onRequest);
    return () => {
      window.removeEventListener(serverRequestEvent, onRequest);
      clearTimeout(warnTimer.current);
    };
  }, [restart, showing]);

  useEffect(() => {
    if (remaining === null) {
      return;
    }
    if (remaining <= 0) {
      window.location.assign(loginPath);
      return;
    }
    const tick = setTimeout(() => setRemaining(remaining - 1), 1000);
    return () => clearTimeout(tick);
  }, [remaining, loginPath]);

  if (!showing) {
    return null;
  }

  async function onOk() {
    setRemaining(null);
    // The OK click is a server request: it resets the server's idle timer.
    const res = await apiFetch(`${endpointPath}/api/me`).catch(() => null);
    if (!res?.ok) {
      window.location.assign(loginPath);
    }
  }

  const m = Math.floor(remaining / 60);
  const s = String(remaining % 60).padStart(2, "0");
  return (
    <div className={styles.overlay}>
      <div className={styles.dialog} role="alertdialog" aria-modal="true" aria-labelledby="idle-title">
        <div className={styles.header}>
          <span className={styles.headerTitle}>Log off timeout warning</span>
          {/* What X does is TBD-040; it is shown and does nothing (agreed). */}
          <button type="button" className={styles.close} aria-label="Close">
            ×
          </button>
        </div>
        <div className={styles.body}>
          <p id="idle-title" className={styles.title}>
            Log off timeout warning
          </p>
          <p>
            Timeout will occur in {m}:{s}
          </p>
          <p>Please click Ok to prevent log off</p>
          <button type="button" className={styles.ok} onClick={onOk} autoFocus>
            OK
          </button>
        </div>
      </div>
    </div>
  );
}
