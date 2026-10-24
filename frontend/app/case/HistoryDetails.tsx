"use client";

import { useEffect, useState } from "react";
import { apiFetch, assignmentURL, type AssignmentRef } from "@/lib/api";
import APILogTable from "./APILogTable";
import NoAccessPopup from "./NoAccessPopup";
import { downloadFile } from "./documents";
import type { APIDataField, APIDataProcess, HistoryCard } from "./types";
import styles from "./case.module.css";

// ERR-034, the Reject banner of a sub-accordion (FR-PSAPI-005 rule 9).
const rejectedMessage = "Pengajuan ditolak karena tidak sesuai dengan ketentuan Pembiayaan.";

// The RESPONSE button of a process with a file (UI-021 Varian SLIK, Pefindo).
const documentButton = { slik: "View SLIK", pefindo: "Download PDF" } as const;

// History Details (UI-013) under the assignment card: the API Log History
// (UI-020, Gambar 36) and Pre-Screening API Data (UI-021, Gambar 38-40)
// accordions, closed at first. The other accordions come with their
// tickets (TBD-033/035).
export default function HistoryDetails({ assignment }: { assignment: AssignmentRef }) {
  const [card, setCard] = useState<HistoryCard | null>(null);
  const { endpointPath, caseId, assignmentId } = assignment;

  // Read once per assignment, not on every render of the page.
  useEffect(() => {
    let live = true;
    apiFetch(assignmentURL({ endpointPath, caseId, assignmentId }, "history"))
      .then((res) => (res.ok ? (res.json() as Promise<HistoryCard>) : null))
      .then((body) => live && setCard(body))
      .catch(() => live && setCard(null));
    return () => {
      live = false;
    };
  }, [endpointPath, caseId, assignmentId]);

  if (!card) {
    return null;
  }
  return (
    <section className={styles.historyCard} aria-label="History Details">
      <Accordion title="API Log History">
        <div className={styles.formPanel}>
          <h3 className={styles.panelTitle}>API Log History</h3>
          <APILogTable logs={card.logs} limit={{ chars: card.truncateChars, lines: card.truncateLines }} />
        </div>
      </Accordion>
      <Accordion title="Pre-Screening API Data">
        <div className={styles.apiDataPanel}>
          {card.processes.length === 0 ? (
            // UI-013 History Details (keadaan kosong) 3.
            <p className={styles.emptyText}>No pre-screening list found</p>
          ) : (
            card.processes.map((p) => <ProcessAccordion key={p.name} process={p} assignment={assignment} />)
          )}
        </div>
      </Accordion>
    </section>
  );
}

function Accordion({ title, children }: { title: string; children: React.ReactNode }) {
  const [open, setOpen] = useState(false);
  return (
    <div className={styles.historyAccordion}>
      <button type="button" className={styles.historyAccordionHeader} aria-expanded={open} onClick={() => setOpen(!open)}>
        <span className={open ? styles.chevronOpen : styles.chevron} aria-hidden="true">
          ›
        </span>
        {title}
      </button>
      {open && children}
    </div>
  );
}

// One process (UI-021): REQUEST and RESPONSE tabs, REQUEST first; the Reject
// banner on REQUEST only; View SLIK or Download PDF above the RESPONSE
// fields when the file exists. The Rejected List variant is the banner only.
function ProcessAccordion({ process: p, assignment }: { process: APIDataProcess; assignment: AssignmentRef }) {
  const [open, setOpen] = useState(false);
  const [tab, setTab] = useState<"request" | "response">("request");
  const [noAccess, setNoAccess] = useState(false);

  async function download() {
    if (!p.documentId) {
      return;
    }
    const result = await downloadFile(assignment.endpointPath, assignment.caseId, { fileId: p.documentId, fileName: p.documentName ?? "" });
    if (result === "forbidden") {
      setNoAccess(true);
    }
  }

  return (
    <div className={styles.processAccordion}>
      <button type="button" className={styles.processHeader} aria-expanded={open} onClick={() => setOpen(!open)}>
        <span className={open ? styles.chevronOpen : styles.chevron} aria-hidden="true">
          ›
        </span>
        {p.name}
      </button>
      {open &&
        (p.rejectedOnly ? (
          <div className={styles.processBody}>
            <p className={styles.rejectBanner}>{rejectedMessage}</p>
          </div>
        ) : (
          <>
            <div className={styles.processTabs} role="tablist">
              <button type="button" role="tab" aria-selected={tab === "request"} className={tab === "request" ? styles.tabActive : styles.tab} onClick={() => setTab("request")}>
                REQUEST
              </button>
              <button type="button" role="tab" aria-selected={tab === "response"} className={tab === "response" ? styles.tabActive : styles.tab} onClick={() => setTab("response")}>
                RESPONSE
              </button>
            </div>
            <div className={styles.processBody}>
              {tab === "request" ? (
                <>
                  {p.rejected && <p className={styles.rejectBanner}>{rejectedMessage}</p>}
                  <Fields fields={p.request} />
                </>
              ) : (
                <>
                  {p.documentId && p.documentKind && (
                    <div className={styles.checkRow}>
                      <button type="button" className={styles.viewDocument} onClick={download}>
                        {documentButton[p.documentKind]}
                      </button>
                    </div>
                  )}
                  <Fields fields={p.response} />
                  {p.errors.length > 0 && (
                    <div className={styles.apiField}>
                      <span className={styles.apiLabel}>Errors</span>
                      {p.errors.map((e, i) => (
                        <span key={i}>{e}</span>
                      ))}
                    </div>
                  )}
                  {p.results.length > 0 && (
                    <div className={styles.apiField}>
                      <span className={styles.apiLabel}>data</span>
                      {p.results.map((r, i) => (
                        <div key={i} className={styles.apiResult}>
                          <Fields fields={r} />
                        </div>
                      ))}
                    </div>
                  )}
                </>
              )}
            </div>
          </>
        ))}
      {noAccess && <NoAccessPopup onClose={() => setNoAccess(false)} />}
    </div>
  );
}

// Read-only fields: grey label above black value; Params' fields indented.
function Fields({ fields }: { fields: APIDataField[] }) {
  return (
    <>
      {fields.map((f, i) => (
        <div key={i} className={f.indent ? styles.apiFieldIndent : styles.apiField}>
          <span className={styles.apiLabel}>{f.label}</span>
          {f.value && <span>{f.value}</span>}
        </div>
      ))}
    </>
  );
}
