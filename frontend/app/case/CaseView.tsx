"use client";

import { useCallback, useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import type { Assignment, CaseFrame, PreScreeningPage, ReviewPage, UnderlyingPage, WaitingSLIKPage } from "./types";
import CaseInfoPanel, { CaseHeader } from "./CaseInfoPanel";
import StageBar from "./StageBar";
import AttachmentsPanel from "./AttachmentsPanel";
import PreScreeningSubmission from "./PreScreeningSubmission";
import UnderlyingDocumentUpload from "./UnderlyingDocumentUpload";
import PreScreeningReview from "./PreScreeningReview";
import WaitingSLIK from "./WaitingSLIK";
import HistoryDetails from "./HistoryDetails";
import { assignmentNames } from "./names";
import styles from "./case.module.css";

// UI-013 Case View, mode Assignment: case info on the left, Stage Bar and
// the opened assignment in the middle, attachments on the right.
export default function CaseView({
  endpointPath,
  caseId,
  assignmentId,
}: {
  endpointPath: string;
  caseId: string;
  assignmentId: number;
}) {
  const [frame, setFrame] = useState<CaseFrame | null>(null);
  const [assignment, setAssignment] = useState<Assignment | null>(null);
  const [failed, setFailed] = useState<number | null>(null);
  // The panel shows each time Case View opens; hiding is not remembered.
  const [infoHidden, setInfoHidden] = useState(false);

  // load reads the frame and the assignment; a stored file changes both
  // (the document row icon and the attachments panel).
  const load = useCallback(() => {
    const base = `${endpointPath}/api/cases/${encodeURIComponent(caseId)}`;
    Promise.all([apiFetch(base), apiFetch(`${base}/assignments/${assignmentId}`)])
      .then(async ([f, a]) => {
        if (!f.ok || !a.ok) {
          setFailed(f.ok ? a.status : f.status);
          return;
        }
        setFrame((await f.json()) as CaseFrame);
        setAssignment((await a.json()) as Assignment);
      })
      .catch(() => setFailed(0));
  }, [endpointPath, caseId, assignmentId]);

  useEffect(load, [load]);

  if (failed !== null) {
    // No SoT screen exists for a case or assignment that cannot be opened.
    return <p className={styles.unavailable}>Case {caseId} cannot be opened.</p>;
  }
  if (!frame || !assignment) {
    return <div aria-busy="true" />;
  }
  const ref = { endpointPath, caseId, assignmentId };
  return (
    <div className={styles.caseView}>
      {!infoHidden && <CaseInfoPanel frame={frame} onHide={() => setInfoHidden(true)} />}
      <div className={styles.mainColumn}>
        {infoHidden && (
          <div className={styles.hiddenHeader}>
            <button type="button" className={styles.showPanel} aria-label="Show case information" onClick={() => setInfoHidden(false)}>
              ›
            </button>
            <CaseHeader frame={frame} />
          </div>
        )}
        <StageBar stages={frame.stages} lifecycle={frame.lifecycle} />
        <div className={styles.main}>
          {assignment.page && assignment.name === assignmentNames.preScreeningSubmission ? (
            <PreScreeningSubmission page={assignment.page as PreScreeningPage} assignment={ref} />
          ) : assignment.page && assignment.name === assignmentNames.underlyingDocumentUpload ? (
            <UnderlyingDocumentUpload page={assignment.page as UnderlyingPage} assignment={ref} onFilesChanged={load} />
          ) : assignment.page && assignment.name === assignmentNames.preScreeningReview ? (
            <PreScreeningReview page={assignment.page as ReviewPage} assignment={ref} />
          ) : assignment.page && assignment.name === assignmentNames.waitingSLIK ? (
            <WaitingSLIK page={assignment.page as WaitingSLIKPage} assignment={ref} />
          ) : (
            <section className={styles.card}>
              <h2 className={styles.cardTitle}>{assignment.name}</h2>
            </section>
          )}
          <HistoryDetails assignment={ref} />
        </div>
      </div>
      <AttachmentsPanel attachments={frame.attachments} />
    </div>
  );
}
