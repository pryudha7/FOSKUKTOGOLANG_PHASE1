"use client";

import { useState } from "react";
import { moveAssignment, type AssignmentRef } from "@/lib/api";
import AssignmentHeader from "./AssignmentHeader";
import Breadcrumb, { preScreeningSteps } from "./Breadcrumb";
import ErrorBanner from "./ErrorBanner";
import { unavailableMessage } from "./names";
import styles from "./case.module.css";

// UI-011 Document Underlying Upload (the Underlying Document upload). This
// ticket provides the way back to Pre-Screening Submission (no validation
// before Back: TBD-027a, agreed); documents, SLIK Request Data and Create
// arrive in ticket 07.
export default function UnderlyingDocumentUpload({ assignment }: { assignment: AssignmentRef }) {
  const [moving, setMoving] = useState(false);
  const [failed, setFailed] = useState(false);

  async function back() {
    if (moving) {
      return;
    }
    setMoving(true);
    const result = await moveAssignment(assignment, "back");
    if (result.kind !== "moved") {
      setFailed(true);
      setMoving(false);
    }
  }

  return (
    <div className={styles.assignmentArea}>
      <ErrorBanner errors={[]} message={failed ? unavailableMessage : undefined} />
      <section className={styles.card}>
        <AssignmentHeader title="Document Underlying Upload" />
        <Breadcrumb steps={preScreeningSteps} active={1} onSelect={(i) => i === 0 && back()} />
        <footer className={styles.buttons}>
          <span className={styles.buttonGroup}>
            <button type="button" className={styles.close} onClick={() => window.location.assign(assignment.endpointPath)}>
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
