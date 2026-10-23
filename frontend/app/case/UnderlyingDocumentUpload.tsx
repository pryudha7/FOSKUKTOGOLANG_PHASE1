"use client";

import { useState } from "react";
import { moveAssignment, type AssignmentRef } from "@/lib/api";
import AssignmentHeader from "./AssignmentHeader";
import Breadcrumb, { preScreeningSteps } from "./Breadcrumb";
import ErrorBanner from "./ErrorBanner";
import NoAccessPopup from "./NoAccessPopup";
import UploadDocumentModal from "./UploadDocumentModal";
import { downloadFile } from "./documents";
import { customerTypeIndividual, unavailableMessage } from "./names";
import type { DocumentCategory, StoredFile, UnderlyingPage } from "./types";
import styles from "./case.module.css";

type UploadTarget = { category: DocumentCategory; documentType: string };

// UI-011 Document Underlying Upload (Gambar 15, 16, 23): the document list
// with upload and download, and SLIK Request Data with the Nomor Memo. The
// other SLIK fields and Create arrive in ticket 33.
export default function UnderlyingDocumentUpload({
  page,
  assignment,
  onFilesChanged,
}: {
  page: UnderlyingPage;
  assignment: AssignmentRef;
  onFilesChanged: () => void;
}) {
  const [moving, setMoving] = useState(false);
  const [failed, setFailed] = useState(false);
  const [uploading, setUploading] = useState<UploadTarget | null>(null);
  const [noAccess, setNoAccess] = useState(false);

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

  async function download(file: StoredFile) {
    const result = await downloadFile(assignment.endpointPath, assignment.caseId, file);
    setNoAccess(result === "forbidden");
    setFailed(result === "failed");
  }

  return (
    <div className={styles.assignmentArea}>
      <ErrorBanner errors={[]} message={page.listError?.message ?? (failed ? unavailableMessage : undefined)} />
      <section className={styles.card}>
        <AssignmentHeader title="Document Underlying Upload" />
        <Breadcrumb steps={preScreeningSteps} active={1} onSelect={(i) => i === 0 && page.editable && back()} />
        <h3 className={styles.sectionTitle}>Document Underlying Upload</h3>
        <div className={styles.formPanel}>
          <div className={styles.infoBanner}>
            <span>
              <span className={styles.infoBannerIcon} aria-hidden="true">
                ⚠
              </span>
              Total ukuran file upload: {page.totalLimitMb} MB
            </span>
            <span>
              <span className={styles.infoBannerIcon} aria-hidden="true">
                ⚠
              </span>
              Hanya file yang diperbolehkan: gambar, pdf, excel, rar, dan zip
            </span>
          </div>
          {page.categories.map((c) => (
            <CategoryAccordion
              key={c.code}
              category={c}
              editable={page.editable}
              onUpload={(documentType) => setUploading({ category: c, documentType })}
              onDownload={download}
            />
          ))}
        </div>
        {/* Badan Usaha shows SLIK Worst Collect instead (ticket 31). */}
        {page.customerType === customerTypeIndividual && (
          <>
            <h3 className={styles.sectionTitle}>SLIK Request Data</h3>
            <div className={styles.formPanel}>
              <div className={styles.field}>
                <span className={styles.fieldLabel}>Nomor Memo</span>
                <span className={styles.readOnlyValue}>{page.memoNumber}</span>
              </div>
            </div>
          </>
        )}
        <footer className={styles.buttons}>
          <span className={styles.buttonGroup}>
            <button type="button" className={styles.close} onClick={() => window.location.assign(assignment.endpointPath)}>
              Close
            </button>
            {page.editable && (
              <button type="button" className={styles.back} disabled={moving} onClick={back}>
                Back
              </button>
            )}
          </span>
        </footer>
      </section>
      {uploading && (
        <UploadDocumentModal
          assignment={assignment}
          category={uploading.category}
          documentType={uploading.documentType}
          onClose={() => setUploading(null)}
          onUploaded={() => {
            setUploading(null);
            onFilesChanged();
          }}
        />
      )}
      {noAccess && <NoAccessPopup onClose={() => setNoAccess(false)} />}
    </div>
  );
}

// One accordion, closed at first (AC-UI-011-02). A clicked row is
// highlighted (AC-UI-011-04); only the document icon downloads
// (FR-FILE-003, AC-FR-FILE-003-06).
function CategoryAccordion({
  category,
  editable,
  onUpload,
  onDownload,
}: {
  category: DocumentCategory;
  editable: boolean;
  onUpload: (documentType: string) => void;
  onDownload: (file: StoredFile) => void;
}) {
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  return (
    <div className={styles.accordion}>
      <button type="button" className={styles.accordionHeader} aria-expanded={open} onClick={() => setOpen(!open)}>
        <span className={open ? styles.chevronOpen : styles.chevron} aria-hidden="true">
          ›
        </span>
        {category.label}
      </button>
      {open && (
        <table className={styles.documentTable}>
          <thead>
            <tr>
              <th>Kategori Dokumen</th>
              <th>Keterangan</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {category.documents.map((d) => (
              <tr
                key={d.documentType}
                className={selected === d.documentType ? styles.documentRowSelected : undefined}
                onClick={() => setSelected(d.documentType)}
              >
                <td>
                  <span className={styles.documentName}>
                    {d.file && (
                      <button
                        type="button"
                        className={styles.documentIcon}
                        aria-label={`Download ${d.documentType}`}
                        onClick={(e) => {
                          e.stopPropagation();
                          setSelected(d.documentType);
                          onDownload(d.file!);
                        }}
                      >
                        <svg width="14" height="16" viewBox="0 0 14 16" aria-hidden="true">
                          <path d="M1 0h8l5 5v11H1z" fill="currentColor" />
                          <path d="M7 6v6M4.5 9.5L7 12l2.5-2.5" stroke="#ffffff" strokeWidth="1.4" fill="none" />
                        </svg>
                      </button>
                    )}
                    {d.documentType}
                  </span>
                </td>
                <td className={styles.documentNote}>{d.mandatory ? "Mandatory" : "Conditional"}</td>
                <td>
                  {editable && (
                    <button
                      type="button"
                      className={styles.uploadIcon}
                      aria-label={`Upload ${d.documentType}`}
                      onClick={(e) => {
                        e.stopPropagation();
                        setSelected(d.documentType);
                        onUpload(d.documentType);
                      }}
                    >
                      <svg width="12" height="14" viewBox="0 0 12 14" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.4">
                        <path d="M6 10V1M2.5 4.5L6 1l3.5 3.5M1 13h10" />
                      </svg>
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
