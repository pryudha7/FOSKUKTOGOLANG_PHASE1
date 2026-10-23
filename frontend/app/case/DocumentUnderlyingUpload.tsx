"use client";

import { useState } from "react";
import { apiFetch, assignmentPath } from "@/lib/api";
import { useOperator } from "@/app/session/SessionContext";
import Breadcrumb, { preScreeningSteps } from "./Breadcrumb";
import styles from "./case.module.css";

// UI-011 Document Underlying Upload. This ticket provides the way back to
// Pre-Screening Submission (no validation before Back: TBD-027a, agreed);
// documents, SLIK Request Data and Create arrive in ticket 07.
export default function DocumentUnderlyingUpload({
  endpointPath,
  caseId,
  assignmentId,
}: {
  endpointPath: string;
  caseId: string;
  assignmentId: number;
}) {
  const me = useOperator();
  const [moving, setMoving] = useState(false);

  async function back() {
    if (moving) {
      return;
    }
    setMoving(true);
    const res = await apiFetch(`${endpointPath}/api/cases/${encodeURIComponent(caseId)}/assignments/${assignmentId}/back`, {
      method: "POST",
    }).catch(() => null);
    if (res?.ok) {
      const body = (await res.json()) as { assignmentId: number };
      window.location.assign(assignmentPath(endpointPath, caseId, body.assignmentId));
      return;
    }
    setMoving(false);
  }

  return (
    <div className={styles.assignmentArea}>
      <section className={styles.card}>
        <header className={styles.assignmentHeader}>
          <span className={styles.assignmentAvatar} aria-hidden="true">
            {me.fullName.trim().charAt(0).toUpperCase()}
          </span>
          <h2 className={styles.cardTitle}>Document Underlying Upload</h2>
        </header>
        <Breadcrumb steps={preScreeningSteps} active={1} onSelect={(i) => i === 0 && back()} />
        <footer className={styles.buttons}>
          <span className={styles.buttonGroup}>
            <button type="button" className={styles.close} onClick={() => window.location.assign(endpointPath)}>
              Close
            </button>
            <button type="button" className={styles.back} disabled={moving} onClick={back}>
              Back
            </button>
          </span>
        </footer>
      </section>
    </div>
  );
}
