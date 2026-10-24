"use client";

import { useState } from "react";
import { apiFetch, assignmentURL, type AssignmentRef } from "@/lib/api";
import APILogTable from "./APILogTable";
import AssignmentHeader from "./AssignmentHeader";
import ErrorBanner from "./ErrorBanner";
import Field from "./Field";
import { unavailableMessage } from "./names";
import type { FieldError, Recommendation, ReviewPage } from "./types";
import styles from "./case.module.css";

// Notes' limit; the server checks it again (ERR-030).
const notesMaxLength = 1000;

// UI-017 Pre Screening (Gambar 27, 28): the Team Lead's Keputusan and Notes,
// with History Pre-screening Notes once the case has a decision. Submit
// passes back to the Dashboard (FR-APP-010); Close leaves without saving
// (TBD-040a, agreed). Errors show as on UI-010 (agreed). After a Check
// Failure it is the Retry view (Gambar 35): the failed API's log and Submit
// to call it again (FR-PSAPI-001 rule 5). After Manual SLIK Entry the SLIK
// log rows show above the history, without the Retry text (Varian Timeout
// SLIK, FR-PSAPI-011 Process 6).
export default function PreScreeningReview({ page, assignment }: { page: ReviewPage; assignment: AssignmentRef }) {
  const [decisionCode, setDecisionCode] = useState("");
  const [notes, setNotes] = useState("");
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [failed, setFailed] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  async function submit() {
    if (submitting || !page.editable) {
      return;
    }
    setSubmitting(true);
    setFailed(false);
    const res = await (page.retry
      ? apiFetch(assignmentURL(assignment, "retry"), { method: "POST" })
      : apiFetch(assignmentURL(assignment, "review"), {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ decisionCode, notes }),
        })
    ).catch(() => null);
    if (res?.ok) {
      window.location.assign(assignment.endpointPath);
      return;
    }
    const invalid = res?.status === 422 ? ((await res.json().catch(() => null)) as { errors?: FieldError[] } | null) : null;
    setErrors(invalid?.errors ?? []);
    setFailed(!invalid?.errors);
    setSubmitting(false);
  }

  const errorOf = (key: string) => errors.find((e) => e.key === key);
  const decisionError = errorOf("decision");
  const notesError = errorOf("notes");
  return (
    <div className={styles.assignmentArea}>
      <ErrorBanner errors={errors} message={failed ? unavailableMessage : undefined} />
      <section className={styles.card}>
        <AssignmentHeader title="Pre Screening" />
        {page.retry && (
          <div className={styles.formPanel}>
            <h3 className={styles.panelTitle}>API Log History</h3>
            <p className={styles.retryText}>
              Server failed for this API:
              <br />
              Please retry by click on submit button
            </p>
            <APILogTable logs={page.retry.logs} limit={{ chars: page.truncateChars, lines: page.truncateLines }} />
          </div>
        )}
        {page.slikTimeout && (
          <div className={styles.formPanel}>
            <h3 className={styles.panelTitle}>API Log History</h3>
            <APILogTable logs={page.slikTimeout.logs} limit={{ chars: page.truncateChars, lines: page.truncateLines }} />
          </div>
        )}
        {page.history.length > 0 && (
          <>
            <h3 className={styles.sectionTitle}>History Pre-screening Notes</h3>
            <div>
              {page.history.map((r, i) => (
                <HistoryEntry key={i} entry={r} />
              ))}
            </div>
          </>
        )}
        {page.editable && !page.retry && (
          <div className={styles.formPanel}>
            <Field label="Keputusan" required error={decisionError?.message}>
              <select
                aria-label="Keputusan"
                className={`${decisionError ? styles.inputInvalid : styles.input} ${styles.decisionSelect}`}
                value={decisionCode}
                onChange={(e) => setDecisionCode(e.target.value)}
              >
                <option value="" disabled>
                  Pilih Keputusan
                </option>
                {page.decisionOptions.map((o) => (
                  <option key={o.code} value={o.code}>
                    {o.description}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Notes" required error={notesError?.message}>
              <textarea
                aria-label="Notes"
                className={notesError ? styles.textareaInvalid : styles.textarea}
                maxLength={notesMaxLength}
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
              />
            </Field>
          </div>
        )}
        <footer className={styles.buttons}>
          <button type="button" className={styles.close} onClick={() => window.location.assign(assignment.endpointPath)}>
            Close
          </button>
          {page.editable && (
            <button type="button" className={styles.next} disabled={submitting} onClick={submit}>
              Submit
            </button>
          )}
        </footer>
      </section>
    </div>
  );
}

// One decision, closed at first (FR-APP-010 rule 5).
function HistoryEntry({ entry }: { entry: Recommendation }) {
  const [open, setOpen] = useState(false);
  return (
    <div className={styles.accordion}>
      <button type="button" className={styles.accordionHeader} aria-expanded={open} onClick={() => setOpen(!open)}>
        <span className={open ? styles.chevronOpen : styles.chevron} aria-hidden="true">
          ›
        </span>
        {entry.decidedBy}
      </button>
      {open && (
        <dl className={styles.historyBody}>
          <dt>Tanggal &amp; Waktu Submit</dt>
          <dd>{entry.submittedAt}</dd>
          <dt>Keputusan</dt>
          <dd>{entry.decision}</dd>
          <dt>Notes</dt>
          <dd>{entry.notes}</dd>
        </dl>
      )}
    </div>
  );
}
