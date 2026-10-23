"use client";

import type { CaseFrame, Person } from "./types";
import { relativeTime } from "./relativeTime";
import styles from "./case.module.css";

// FR-CASE-005 / UI-013 panel Case Info: header, fields and menus. Its hide
// button shows on hover; once hidden, CaseHeader stays above the main
// column with a show button (Gambar 82, 83).
export default function CaseInfoPanel({ frame, onHide }: { frame: CaseFrame; onHide: () => void }) {
  return (
    <aside className={styles.info}>
      <CaseHeader frame={frame} />
      <button type="button" className={styles.hidePanel} aria-label="Hide case information" onClick={onHide}>
        ‹
      </button>
      {/* "Actions" is TBD-033: shown, inert. */}
      <div className={styles.infoActions}>Actions ▾</div>
      <dl className={styles.fields}>
        <dt>Priority</dt>
        <dd className={styles.priority}>{frame.priority}</dd>
        <dt>Status</dt>
        <dd>{frame.status && <span className={styles.badge}>{frame.status}</span>}</dd>
        <dt>Created</dt>
        <dd>
          <PersonTime person={frame.created} />
        </dd>
        <dt>Updated</dt>
        <dd>
          <PersonTime person={frame.updated} />
        </dd>
      </dl>
      {/* History Details content (TBD-035) and the Details page (FR-CASE-010)
          arrive in later tickets; the menus are shown as on the evidence. */}
      <nav className={styles.infoMenus}>
        <span className={styles.infoMenuActive}>History Details</span>
        <span className={styles.infoMenu}>Details</span>
      </nav>
    </aside>
  );
}

// The case header: briefcase, Case ID and case type label.
export function CaseHeader({ frame }: { frame: CaseFrame }) {
  return (
    <div className={styles.infoHeader}>
      <BriefcaseIcon />
      <div className={styles.infoHeaderText}>
        <span className={styles.infoCaseId}>{frame.caseId}</span>
        <span className={styles.infoCaseType}>{frame.caseTypeLabel}</span>
      </div>
      {/* The star's meaning is TBD-033: shown, inert. */}
      <span className={styles.infoStar} aria-hidden="true">
        ☆
      </span>
    </div>
  );
}

function PersonTime({ person }: { person: Person }) {
  return (
    <>
      <span className={styles.personName}>{person.name}</span> {relativeTime(person.at)}
    </>
  );
}

function BriefcaseIcon() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="#ffffff" strokeWidth="2" aria-hidden="true">
      <rect x="3" y="7" width="18" height="13" rx="2" />
      <path d="M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M3 12h18" />
    </svg>
  );
}
