"use client";

import { useRef, useState } from "react";
import { apiFetch, assignmentURL, type AssignmentRef } from "@/lib/api";
import AssignmentHeader from "./AssignmentHeader";
import Autocomplete, { inputClass } from "./Autocomplete";
import ErrorBanner from "./ErrorBanner";
import Field from "./Field";
import NoAccessPopup from "./NoAccessPopup";
import { acceptedTypes, downloadFile } from "./documents";
import { unavailableMessage } from "./names";
import type { DocumentItem, FieldError, Option, PipelinePage } from "./types";
import styles from "./case.module.css";

// The SUBMISSION_TYPE code of a Take Over (FR-APP-004 rule 2, DEC-116).
const takeOver = "Take Over";

// UI-024 Pipeline Data Entry (Gambar 45-47, FR-APP-004). Jenis Pengajuan
// and Produk are saved on Submit, which returns to the Dashboard and leaves
// the next assignment new (agreed); every pick of Jenis Pengajuan, the same
// one again included, clears Nama Bank and deletes the letter on the server
// at once (rule 3); the letter is stored when chosen (rule 6). Changing a
// field clears its error. Close leaves without saving (agreed). An Authors
// look is read-only.
export default function PipelineDataEntry({
  page,
  assignment,
  onFilesChanged,
}: {
  page: PipelinePage;
  assignment: AssignmentRef;
  onFilesChanged: () => void;
}) {
  const readOnly = !page.editable;
  const [submissionType, setSubmissionType] = useState(page.submissionType);
  const [productCode, setProductCode] = useState(page.productCode);
  const [bankName, setBankName] = useState(page.bankName);
  const [letter, setLetter] = useState<DocumentItem | null>(page.bankLetter);
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [message, setMessage] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);
  const [noAccess, setNoAccess] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  const isTakeOver = submissionType === takeOver;
  const errorOf = (key: string) => errors.find((e) => e.section === "form" && e.key === key)?.message;
  const clearError = (...keys: string[]) => setErrors((all) => all.filter((e) => !(e.section === "form" && keys.includes(e.key))));

  async function changeSubmissionType(next: string) {
    if (busy) {
      return;
    }
    const before = { submissionType, bankName };
    clearError("submissionType", "bankName");
    // Cleared at once; Nama Bank is locked until the server has answered.
    setSubmissionType(next);
    setBankName("");
    setBusy(true);
    setMessage(undefined);
    const res = await apiFetch(assignmentURL(assignment, "submission-type-change"), { method: "POST" }).catch(() => null);
    if (res?.ok) {
      if (letter) {
        setLetter(null);
        onFilesChanged();
      }
    } else {
      // The delete failed: the choice goes back (agreed; its message is TBD-052).
      setSubmissionType(before.submissionType);
      setBankName(before.bankName);
      setMessage(unavailableMessage);
    }
    setBusy(false);
  }

  async function upload(file: File) {
    setBusy(true);
    setMessage(undefined);
    setErrors([]);
    const form = new FormData();
    form.append("file", file);
    const res = await apiFetch(assignmentURL(assignment, "bank-letter"), { method: "POST", body: form }).catch(() => null);
    const body = res ? await res.json().catch(() => null) : null;
    if (res?.status === 201) {
      setLetter(body as DocumentItem);
      onFilesChanged();
    } else if (res?.status === 422) {
      setErrors((body as { errors: FieldError[] }).errors);
    } else {
      // ERR-025 comes with its message (502); anything else is ERR-003.
      setMessage((body as { errors?: { message: string }[] } | null)?.errors?.[0]?.message ?? unavailableMessage);
    }
    setBusy(false);
  }

  async function download(item: DocumentItem) {
    const result = await downloadFile(assignment.endpointPath, assignment.caseId, { fileId: item.fileId, fileName: item.fileName });
    setNoAccess(result === "forbidden");
    setMessage(result === "failed" ? unavailableMessage : undefined);
  }

  async function submit() {
    if (busy || readOnly) {
      return;
    }
    setBusy(true);
    setMessage(undefined);
    const res = await apiFetch(assignmentURL(assignment, "pipeline"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ submissionType, productCode, bankName: isTakeOver ? bankName : "" }),
    }).catch(() => null);
    if (res?.ok) {
      window.location.assign(assignment.endpointPath);
      return;
    }
    const invalid = res?.status === 422 ? ((await res.json().catch(() => null)) as { errors?: FieldError[] } | null) : null;
    setErrors(invalid?.errors ?? []);
    setMessage(invalid?.errors ? undefined : unavailableMessage);
    setBusy(false);
  }

  return (
    <div className={styles.assignmentArea}>
      <ErrorBanner errors={errors} message={message} />
      <section className={styles.card}>
        <AssignmentHeader title="Pipeline Data Entry" />
        <h3 className={styles.sectionTitle}>Inisiasi</h3>
        <div className={`${styles.formPanel} ${styles.pipelinePanel}`}>
          <div className={styles.pipelineGrid}>
            <Field label="Kode Branch" error={errorOf("branch")}>
              <input className={styles.inputDisabled} value={page.branchCode} disabled readOnly aria-label="Kode Branch" />
            </Field>
            <Field label="Nama Branch">
              <input className={styles.inputDisabled} value={page.branchName} disabled readOnly aria-label="Nama Branch" />
            </Field>

            <div className={styles.pipelineColumn}>
              <Field label="Jenis Pengajuan" required error={errorOf("submissionType")}>
                <PickList
                  label="Jenis Pengajuan"
                  placeholder="Pilih Jenis Pengajuan"
                  options={page.submissionTypeOptions}
                  value={submissionType}
                  invalid={!!errorOf("submissionType")}
                  disabled={readOnly || busy}
                  onPick={changeSubmissionType}
                />
              </Field>
              {isTakeOver && (
                <Field label="Nama Bank" required error={errorOf("bankName")}>
                  <Autocomplete
                    label="Nama Bank"
                    source={assignmentURL(assignment, "banks")}
                    text={bankName}
                    onType={(v) => (setBankName(v), clearError("bankName"))}
                    onChoose={(o) => (setBankName(o.description), clearError("bankName"))}
                    invalid={!!errorOf("bankName")}
                    disabled={readOnly || busy}
                  />
                </Field>
              )}
              <Field label="Produk" required error={errorOf("product")}>
                <select
                  aria-label="Produk"
                  className={inputClass(readOnly, !!errorOf("product"))}
                  value={productCode}
                  disabled={readOnly}
                  onChange={(e) => (setProductCode(e.target.value), clearError("product"))}
                >
                  <option value="" disabled>
                    Pilih Produk
                  </option>
                  {page.productOptions.map((o) => (
                    <option key={o.code} value={o.code}>
                      {o.description}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Tipe Nasabah" required>
                <select aria-label="Tipe Nasabah" className={styles.inputDisabled} value={page.customerType} disabled>
                  <option value={page.customerType}>{page.customerType}</option>
                </select>
              </Field>
            </div>

            {isTakeOver ? (
              <div className={styles.bankLetter}>
                <span className={styles.bankLetterLabel}>Bukti Surat dari Bank</span>
                {!readOnly && (
                  <>
                    <button type="button" className={styles.attachmentButton} disabled={busy} onClick={() => fileInput.current?.click()}>
                      Attachment
                    </button>
                    <input
                      ref={fileInput}
                      type="file"
                      accept={acceptedTypes}
                      hidden
                      aria-label="Bukti Surat dari Bank file"
                      onChange={(e) => {
                        const file = e.target.files?.[0];
                        e.target.value = "";
                        if (file) {
                          upload(file);
                        }
                      }}
                    />
                  </>
                )}
                {/* Gambar 46: a clip under the button until a file is stored (Gambar 47). */}
                {letter ? (
                  <FileItem item={letter} onDownload={download} />
                ) : (
                  <svg width="20" height="22" viewBox="0 0 20 22" aria-hidden="true" className={styles.clipIcon}>
                    <path
                      d="M16.5 10.5l-6.8 6.8a4.2 4.2 0 01-6-6l7.6-7.6a2.8 2.8 0 014 4l-7.4 7.4a1.4 1.4 0 01-2-2l6.6-6.6"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="1.6"
                      strokeLinecap="round"
                    />
                  </svg>
                )}
              </div>
            ) : (
              <div />
            )}
          </div>

          <div className={styles.identity}>
            <span className={styles.identityLabel}>{page.identity.label} *</span>
            {page.identity.file && <FileItem item={page.identity.file} onDownload={download} />}
          </div>
        </div>

        <footer className={styles.buttons}>
          <button type="button" className={styles.close} onClick={() => window.location.assign(assignment.endpointPath)}>
            Close
          </button>
          {!readOnly && (
            <button type="button" className={styles.next} disabled={busy} onClick={submit}>
              Submit
            </button>
          )}
        </footer>
      </section>
      {noAccess && <NoAccessPopup onClose={() => setNoAccess(false)} />}
    </div>
  );
}

// A dropdown that reports every pick, the current value again included,
// which a native select does not (FR-APP-004 rule 3). It looks like the
// other dropdowns: placeholder, chevron, list below; arrows, Enter, Escape.
function PickList({
  label,
  placeholder,
  options,
  value,
  invalid,
  disabled,
  onPick,
}: {
  label: string;
  placeholder: string;
  options: Option[];
  value: string;
  invalid: boolean;
  disabled: boolean;
  onPick: (code: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const chosen = options.find((o) => o.code === value);

  function show() {
    setActive(Math.max(0, options.findIndex((o) => o.code === value)));
    setOpen(true);
  }

  function pick(o: Option) {
    setOpen(false);
    onPick(o.code);
  }

  return (
    <div className={styles.autocomplete}>
      <button
        type="button"
        role="combobox"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        className={`${inputClass(disabled, invalid)} ${styles.pickButton}`}
        disabled={disabled}
        onClick={() => (open ? setOpen(false) : show())}
        onBlur={() => setOpen(false)}
        onKeyDown={(e) => {
          if (!open) {
            if (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              show();
            }
            return;
          }
          if (e.key === "ArrowDown") {
            e.preventDefault();
            setActive((i) => Math.min(i + 1, options.length - 1));
          } else if (e.key === "ArrowUp") {
            e.preventDefault();
            setActive((i) => Math.max(i - 1, 0));
          } else if ((e.key === "Enter" || e.key === " ") && options[active]) {
            e.preventDefault();
            pick(options[active]);
          } else if (e.key === "Escape") {
            setOpen(false);
          }
        }}
      >
        <span>{chosen?.description ?? placeholder}</span>
        <svg width="12" height="8" viewBox="0 0 12 8" aria-hidden="true">
          <path d="M1 1.5l5 5 5-5" fill="none" stroke="currentColor" strokeWidth="1.6" />
        </svg>
      </button>
      {open && (
        <ul className={styles.autocompleteList} role="listbox" aria-label={label}>
          {options.map((o, i) => (
            <li
              key={o.code}
              role="option"
              aria-selected={o.code === value}
              className={i === active ? styles.autocompleteActive : styles.autocompleteOption}
              // Chosen before the button's blur closes the list.
              onMouseDown={(e) => {
                e.preventDefault();
                pick(o);
              }}
            >
              {o.description}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

// A file item: grey file icon, blue link that downloads (no Display Image,
// rule 9), and the category caption.
function FileItem({ item, onDownload }: { item: DocumentItem; onDownload: (item: DocumentItem) => void }) {
  return (
    <div className={styles.fileItem}>
      <svg width="16" height="20" viewBox="0 0 16 20" aria-hidden="true" className={styles.fileItemIcon}>
        <path d="M1 1h9l5 5v13H1z" fill="none" stroke="currentColor" strokeWidth="1.4" />
        <path d="M10 1v5h5" fill="none" stroke="currentColor" strokeWidth="1.4" />
      </svg>
      <span className={styles.fileItemText}>
        <button type="button" className={styles.fileItemLink} onClick={() => onDownload(item)}>
          {item.name}
        </button>
        <span>{item.caption}</span>
      </span>
    </div>
  );
}
