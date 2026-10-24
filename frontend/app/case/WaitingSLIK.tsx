"use client";

import { useState } from "react";
import { apiFetch, assignmentURL, type AssignmentRef } from "@/lib/api";
import AssignmentHeader from "./AssignmentHeader";
import ErrorBanner from "./ErrorBanner";
import { unavailableMessage } from "./names";
import type { SLIKStatus, WaitingSLIKPage } from "./types";
import styles from "./case.module.css";

// UI-023 Waiting SLIK (Gambar 44): the time left before the SLIK SLA, Check
// SLIK Status (FR-PSAPI-013) and its Message panel, hidden until the first
// check and showing the last; with a Spouse, one panel per subject. Close
// leaves without changes; there is no Submit (DEC-086). Once the system has
// completed the assignment (409), the button goes away (agreed for
// TBD-048d).
export default function WaitingSLIK({ page, assignment }: { page: WaitingSLIKPage; assignment: AssignmentRef }) {
  const [statuses, setStatuses] = useState<SLIKStatus[] | null>(null);
  const [checking, setChecking] = useState(false);
  const [failed, setFailed] = useState(false);
  const [completed, setCompleted] = useState(false);

  async function check() {
    if (checking) {
      return;
    }
    setChecking(true);
    setFailed(false);
    const res = await apiFetch(assignmentURL(assignment, "slik-status"), { method: "POST" }).catch(() => null);
    const body = res?.ok ? ((await res.json().catch(() => null)) as { statuses?: SLIKStatus[] } | null) : null;
    if (body?.statuses) {
      setStatuses(body.statuses);
    } else {
      // An old message is not shown beside the failure.
      setStatuses(null);
      setFailed(true);
      setCompleted(res?.status === 409);
    }
    setChecking(false);
  }

  // Titled panels when the case has a Spouse (AC-FR-PSAPI-013-03).
  const titled = statuses?.some((s) => s.subject === "SPOUSE") ?? false;
  return (
    <div className={styles.assignmentArea}>
      <ErrorBanner errors={[]} message={failed ? unavailableMessage : undefined} />
      <section className={styles.card}>
        <AssignmentHeader title="SLIKWQFlowAct" />
        <p className={styles.dueText}>Due {page.dueMinutes} minutes from now</p>
        {page.editable && !completed && (
          <div className={styles.checkRow}>
            <button type="button" className={styles.checkSLIK} disabled={checking} onClick={check}>
              Check SLIK Status
            </button>
          </div>
        )}
        {statuses?.map((s) => (
          <div key={s.subject} className={styles.messagePanel}>
            <span className={styles.messageLabel}>
              {titled ? (s.subject === "SPOUSE" ? "Message (Pasangan)" : "Message (Calon Nasabah)") : "Message"}
            </span>
            <span>{s.message}</span>
          </div>
        ))}
        <footer className={styles.buttons}>
          <button type="button" className={styles.close} onClick={() => window.location.assign(assignment.endpointPath)}>
            Close
          </button>
        </footer>
      </section>
    </div>
  );
}
