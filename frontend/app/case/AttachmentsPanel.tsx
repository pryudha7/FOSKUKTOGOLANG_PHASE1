"use client";

import { useState } from "react";
import styles from "./case.module.css";

// FR-CASE-006 attachments panel, closed by default. Files arrive with the
// Case Document table in ticket 07; until then the case has none.
export default function AttachmentsPanel({ count }: { count: number }) {
  const [open, setOpen] = useState(false);
  if (!open) {
    return (
      <aside className={styles.attachmentsClosed}>
        <button type="button" className={styles.attachmentsToggle} aria-label="Open files and documents" onClick={() => setOpen(true)}>
          ⇤
        </button>
        <span className={styles.paperclip} aria-hidden="true">
          📎
        </span>
        <span className={styles.attachmentsBadge}>{count}</span>
      </aside>
    );
  }
  return (
    <aside className={styles.attachmentsOpen}>
      <div className={styles.attachmentsHeader}>
        <span className={styles.attachmentsTitle}>
          <span aria-hidden="true">📎</span> Files &amp; documents ({count})
        </span>
        <button type="button" className={styles.attachmentsToggle} aria-label="Close files and documents" onClick={() => setOpen(false)}>
          ⇥
        </button>
      </div>
    </aside>
  );
}
