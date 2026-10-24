"use client";

import { useEffect, useState } from "react";
import { apiFetch, assignmentURL, moveAssignment, type AssignmentRef } from "@/lib/api";
import AssignmentHeader from "./AssignmentHeader";
import Breadcrumb, { preScreeningSteps } from "./Breadcrumb";
import ErrorBanner from "./ErrorBanner";
import Field from "./Field";
import NoAccessPopup from "./NoAccessPopup";
import UploadDocumentModal from "./UploadDocumentModal";
import { downloadFile } from "./documents";
import { customerTypeIndividual, unavailableMessage } from "./names";
import type { DocumentCategory, FieldError, Option, SLIKRequest, StoredFile, UnderlyingPage } from "./types";
import styles from "./case.module.css";

type UploadTarget = { category: DocumentCategory; documentType: string };

// Alamat Nasabah's limit; the server checks it again (ERR-029).
const addressMaxLength = 350;

// UI-011 Document Underlying Upload (Gambar 15, 16, 23, 24): the document
// list with upload and download, SLIK Request Data, and Create, which on
// success hands the case to the Team Lead and returns to the Dashboard
// (FR-FILE-004, FR-APP-009). Errors stay until Create is pressed again.
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
  const [slik, setSLIK] = useState<SLIKRequest>(
    page.slik?.values ?? { requestPurpose: "", facilityOption: "", address: "", searchType: "", segmentation: "" },
  );
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [creating, setCreating] = useState(false);
  // Counts Create results, so an accordion with ERR-026 opens after each one.
  const [attempt, setAttempt] = useState(0);

  async function create() {
    if (creating || !page.editable) {
      return;
    }
    setCreating(true);
    setFailed(false);
    const url = assignmentURL(assignment, "create");
    const res = await apiFetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(slik),
    }).catch(() => null);
    if (res?.ok) {
      window.location.assign(assignment.endpointPath);
      return;
    }
    const invalid = res?.status === 422 ? ((await res.json().catch(() => null)) as { errors?: FieldError[] } | null) : null;
    if (invalid?.errors) {
      setErrors(invalid.errors);
    } else {
      setErrors([]);
      setFailed(true);
    }
    setAttempt((n) => n + 1);
    setCreating(false);
  }

  const slikError = (key: string) => errors.find((e) => e.section === "slik" && e.key === key);

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
      <ErrorBanner
        errors={errors.filter((e) => e.section !== "documents")}
        message={page.listError?.message ?? (failed ? unavailableMessage : undefined)}
      />
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
              missing={errors.filter((e) => e.section === "documents" && e.key.startsWith(`${c.code}/`))}
              attempt={attempt}
              onUpload={(documentType) => setUploading({ category: c, documentType })}
              onDownload={download}
            />
          ))}
        </div>
        {/* Badan Usaha shows SLIK Worst Collect instead (ticket 31). */}
        {page.customerType === customerTypeIndividual && page.slik && (
          <>
            <h3 className={styles.sectionTitle}>SLIK Request Data</h3>
            <div className={styles.formPanel}>
              <SLIKSelect
                label="Tujuan Permintaan"
                placeholder="Pilih Tujuan Permintaan"
                options={page.slik.options.requestPurpose}
                value={slik.requestPurpose}
                error={slikError("requestPurpose")?.message}
                disabled={!page.editable}
                onChange={(v) => setSLIK({ ...slik, requestPurpose: v })}
              />
              <SLIKSelect
                label="Pilihan Fasilitas"
                placeholder="Pilih Fasilitas"
                options={page.slik.options.facilityOption}
                value={slik.facilityOption}
                error={slikError("facilityOption")?.message}
                disabled={!page.editable}
                onChange={(v) => setSLIK({ ...slik, facilityOption: v })}
              />
              <div className={styles.field}>
                <span className={styles.fieldLabel}>Nomor Memo</span>
                <span className={styles.readOnlyValue}>{page.memoNumber}</span>
              </div>
              <Field label="Alamat Nasabah" required error={slikError("address")?.message}>
                <textarea
                  aria-label="Alamat Nasabah"
                  className={!page.editable ? styles.textareaDisabled : slikError("address") ? styles.textareaInvalid : styles.textarea}
                  maxLength={addressMaxLength}
                  value={slik.address}
                  disabled={!page.editable}
                  onChange={(e) => setSLIK({ ...slik, address: e.target.value })}
                />
              </Field>
              <SLIKSelect
                label="Tipe Pencarian"
                placeholder="Pilih Tipe Pencarian"
                options={page.slik.options.searchType}
                value={slik.searchType}
                error={slikError("searchType")?.message}
                disabled={!page.editable}
                onChange={(v) => setSLIK({ ...slik, searchType: v })}
              />
              <SLIKSelect
                label="Segmentasi"
                placeholder="Pilih Segmentasi"
                options={page.slik.options.segmentation}
                value={slik.segmentation}
                error={slikError("segmentation")?.message}
                disabled={!page.editable}
                onChange={(v) => setSLIK({ ...slik, segmentation: v })}
              />
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
          {page.editable && (
            <button type="button" className={styles.next} disabled={creating} onClick={create}>
              Create
            </button>
          )}
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

// A SLIK dropdown: BMV descriptions shown, codes stored (UI-011).
function SLIKSelect({
  label,
  placeholder,
  options,
  value,
  error,
  disabled,
  onChange,
}: {
  label: string;
  placeholder: string;
  options: Option[];
  value: string;
  error?: string;
  disabled: boolean;
  onChange: (code: string) => void;
}) {
  return (
    <Field label={label} required error={error}>
      <select
        aria-label={label}
        className={disabled ? styles.inputDisabled : error ? styles.inputInvalid : styles.input}
        value={value}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value)}
      >
        <option value="" disabled>
          {placeholder}
        </option>
        {options.map((o) => (
          <option key={o.code} value={o.code}>
            {o.description}
          </option>
        ))}
      </select>
    </Field>
  );
}

// One accordion, closed at first (AC-UI-011-02). A clicked row is
// highlighted (AC-UI-011-04); only the document icon downloads
// (FR-FILE-003, AC-FR-FILE-003-06). After Create, rows missing a mandatory
// file show ERR-026 and the accordion opens with the error marker left of
// its chevron (Gambar 24, agreed over the written "right").
function CategoryAccordion({
  category,
  editable,
  missing,
  attempt,
  onUpload,
  onDownload,
}: {
  category: DocumentCategory;
  editable: boolean;
  missing: FieldError[]; // ERR-026 of this category's rows
  attempt: number;
  onUpload: (documentType: string) => void;
  onDownload: (file: StoredFile) => void;
}) {
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const hasErrors = missing.length > 0;
  // Opens after each Create result with errors here; the operator may close it again.
  useEffect(() => {
    if (hasErrors) {
      setOpen(true);
    }
  }, [attempt, hasErrors]);
  const missingMessage = (documentType: string) => missing.find((e) => e.field === documentType)?.message;
  return (
    <div className={styles.accordion}>
      <button type="button" className={styles.accordionHeader} aria-expanded={open} onClick={() => setOpen(!open)}>
        {hasErrors && (
          <span className={styles.accordionError} role="img" aria-label="Error">
            ▲
          </span>
        )}
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
                    {missingMessage(d.documentType) && <span className={styles.documentError}>{missingMessage(d.documentType)}</span>}
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
