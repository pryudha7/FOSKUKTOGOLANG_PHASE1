"use client";

import { useState } from "react";
import NoAccessPopup from "./NoAccessPopup";
import { downloadFile } from "./documents";
import { shortRelativeTime } from "./relativeTime";
import type { Attachment } from "./types";
import styles from "./case.module.css";

// How many items the compact list shows before "View all" (Gambar 18 shows
// three; the number is not stated in the SoT).
const compactCount = 3;

const imageName = /\.(jpe?g|png|gif|bmp)$/i;

// FR-CASE-006 attachments panel (Gambar 17, 18), closed by default. The
// open/close control sits at the top right of the page, level with the
// Stage Bar (UI-013). A file name downloads the file (DEC-114).
export default function AttachmentsPanel({
  endpointPath,
  caseId,
  attachments,
}: {
  endpointPath: string;
  caseId: string;
  attachments: Attachment[];
}) {
  const [open, setOpen] = useState(false);
  const [all, setAll] = useState(false);
  const [noAccess, setNoAccess] = useState(false);
  const count = attachments.length;
  if (!open) {
    return (
      <aside className={styles.attachmentsClosed}>
        <button type="button" className={styles.attachmentsToggle} aria-label="Open files and documents" onClick={() => setOpen(true)}>
          ⇤
        </button>
        <span className={styles.paperclipBox} aria-hidden="true">
          📎
        </span>
        <span className={styles.attachmentsBadge}>{count}</span>
      </aside>
    );
  }
  const fileURL = (a: Attachment) => `${endpointPath}/api/cases/${encodeURIComponent(caseId)}/documents/${encodeURIComponent(a.fileId)}`;
  const shown = all ? attachments : attachments.slice(0, compactCount);
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
      <ul className={styles.attachmentList}>
        {shown.map((a) => (
          <li key={a.fileId} className={styles.attachmentItem}>
            {imageName.test(a.fileName) ? (
              <img className={styles.attachmentThumb} src={fileURL(a)} alt="" />
            ) : (
              <span className={styles.attachmentThumbFile} aria-hidden="true" />
            )}
            <span className={styles.attachmentText}>
              <button
                type="button"
                className={styles.attachmentLink}
                title={a.fileName}
                onClick={async () => setNoAccess((await downloadFile(endpointPath, caseId, a)) === "forbidden")}
              >
                {a.fileName}
              </button>
              <span className={styles.attachmentMeta}>
                {a.categoryLabel} • {a.uploadedBy} • {shortRelativeTime(a.createdAt)}
              </span>
            </span>
          </li>
        ))}
      </ul>
      {count > 0 && (
        <button type="button" className={styles.viewAll} onClick={() => setAll(!all)}>
          {all ? "View less" : "View all"}
        </button>
      )}
      {noAccess && <NoAccessPopup onClose={() => setNoAccess(false)} />}
    </aside>
  );
}
