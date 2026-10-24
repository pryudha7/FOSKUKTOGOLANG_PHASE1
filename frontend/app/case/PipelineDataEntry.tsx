"use client";

import { useEffect, useRef, useState } from "react";
import { apiFetch, assignmentURL, moveAssignment, type AssignmentRef } from "@/lib/api";
import AssignmentHeader from "./AssignmentHeader";
import ErrorBanner from "./ErrorBanner";
import Field from "./Field";
import NoAccessPopup from "./NoAccessPopup";
import { acceptedTypes, downloadFile } from "./documents";
import { unavailableMessage } from "./names";
import type { DocumentItem, FieldError, Option, PipelinePage } from "./types";
import styles from "./case.module.css";

// The SUBMISSION_TYPE code of a Take Over (FR-APP-004 rule 2).
const takeOver = "Take Over";

// The Bukti Surat dari Bank item once uploaded (FR-APP-004 rule 7).
const bankLetterName = "Bukti Surat Take Over dari Bank";
const bankLetterCaption = "DokumenInisiasi • Dokumen Inisiasi";

// ERR-025: storage could not be reached.
const storageFailed = "Upload dokumen gagal. Silakan coba lagi.";

// UI-024 Pipeline Data Entry (Gambar 45-47, FR-APP-004). Jenis Pengajuan
// and Produk are saved on Submit; changing Jenis Pengajuan clears Nama Bank
// and deletes the letter on the server at once (rule 3); the letter is
// stored when chosen (rule 6). Close leaves without saving (agreed). An
// Authors look is read-only.
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

  async function changeSubmissionType(next: string) {
    if (busy) {
      return;
    }
    const before = { submissionType, bankName };
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
    if (res?.status === 201) {
      const stored = (await res.json()) as { fileId: string; fileName: string };
      setLetter({ ...stored, name: bankLetterName, caption: bankLetterCaption });
      onFilesChanged();
    } else if (res?.status === 422) {
      setErrors(((await res.json()) as { errors: FieldError[] }).errors);
    } else {
      setMessage(res?.status === 502 ? storageFailed : unavailableMessage);
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
    const result = await moveAssignment<FieldError>(assignment, "pipeline", {
      submissionType,
      productCode,
      bankName: isTakeOver ? bankName : "",
    });
    if (result.kind === "moved") {
      return;
    }
    setErrors(result.kind === "invalid" ? result.errors : []);
    setMessage(result.kind === "failed" ? unavailableMessage : undefined);
    setBusy(false);
  }

  const inputClass = (invalid: boolean) => (readOnly ? styles.inputDisabled : invalid ? styles.inputInvalid : styles.input);

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
                <select
                  aria-label="Jenis Pengajuan"
                  className={inputClass(!!errorOf("submissionType"))}
                  value={submissionType}
                  disabled={readOnly || busy}
                  onChange={(e) => changeSubmissionType(e.target.value)}
                >
                  <option value="" disabled>
                    Pilih Jenis Pengajuan
                  </option>
                  <OptionList options={page.submissionTypeOptions} />
                </select>
              </Field>
              {isTakeOver && (
                <Field label="Nama Bank" required error={errorOf("bankName")}>
                  <BankAutocomplete assignment={assignment} value={bankName} invalid={!!errorOf("bankName")} disabled={readOnly || busy} onChange={setBankName} />
                </Field>
              )}
              <Field label="Produk" required error={errorOf("product")}>
                <select
                  aria-label="Produk"
                  className={inputClass(!!errorOf("product"))}
                  value={productCode}
                  disabled={readOnly}
                  onChange={(e) => setProductCode(e.target.value)}
                >
                  <option value="" disabled>
                    Pilih Produk
                  </option>
                  <OptionList options={page.productOptions} />
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

function OptionList({ options }: { options: Option[] }) {
  return (
    <>
      {options.map((o) => (
        <option key={o.code} value={o.code}>
          {o.description}
        </option>
      ))}
    </>
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

// ERR-038: the suggestion query failed.
const suggestionsFailed = "Gagal mendapatkan data, harap refresh halaman";

// The Nama Bank autocomplete (Field 6, BR-UI-005): each pause of 300 ms in
// typing queries the server; matching letters are bold; arrows and Enter or
// a click choose. Any typed text may stay as it is.
function BankAutocomplete({
  assignment,
  value,
  invalid,
  disabled,
  onChange,
}: {
  assignment: AssignmentRef;
  value: string;
  invalid: boolean;
  disabled: boolean;
  onChange: (v: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [options, setOptions] = useState<Option[]>([]);
  const [failed, setFailed] = useState(false);
  const [active, setActive] = useState(0);
  const { endpointPath, caseId, assignmentId } = assignment;

  useEffect(() => {
    if (!open) {
      return;
    }
    let live = true;
    const timer = setTimeout(() => {
      apiFetch(`${assignmentURL({ endpointPath, caseId, assignmentId }, "banks")}?q=${encodeURIComponent(value)}`)
        .then((res) => (res.ok ? (res.json() as Promise<{ options: Option[] }>) : Promise.reject()))
        .then((body) => {
          if (live) {
            setOptions(body.options);
            setFailed(false);
            setActive(0);
          }
        })
        .catch(() => live && setFailed(true));
    }, 300);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [endpointPath, caseId, assignmentId, value, open]);

  function choose(o: Option) {
    onChange(o.description);
    setOpen(false);
  }

  return (
    <div className={styles.autocomplete}>
      <input
        aria-label="Nama Bank"
        role="combobox"
        aria-expanded={open}
        aria-autocomplete="list"
        className={disabled ? styles.inputDisabled : invalid ? styles.inputInvalid : styles.input}
        value={value}
        disabled={disabled}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onChange={(e) => {
          onChange(e.target.value);
          setOpen(true);
        }}
        onKeyDown={(e) => {
          if (!open || failed || options.length === 0) {
            return;
          }
          if (e.key === "ArrowDown") {
            e.preventDefault();
            setActive((i) => Math.min(i + 1, options.length - 1));
          } else if (e.key === "ArrowUp") {
            e.preventDefault();
            setActive((i) => Math.max(i - 1, 0));
          } else if (e.key === "Enter") {
            e.preventDefault();
            choose(options[active]);
          } else if (e.key === "Escape") {
            setOpen(false);
          }
        }}
      />
      {open && (failed || options.length > 0) && (
        <ul className={styles.autocompleteList} role="listbox">
          {failed ? (
            <li className={styles.autocompleteFailed} aria-disabled="true">
              {suggestionsFailed}
            </li>
          ) : (
            options.map((o, i) => (
              <li
                key={o.code}
                role="option"
                aria-selected={i === active}
                className={i === active ? styles.autocompleteActive : styles.autocompleteOption}
                // Chosen before the input's blur closes the list.
                onMouseDown={(e) => {
                  e.preventDefault();
                  choose(o);
                }}
              >
                <Highlighted text={o.description} match={value} />
              </li>
            ))
          )}
        </ul>
      )}
    </div>
  );
}

// Highlighted shows text with each case-insensitive occurrence of match bold.
function Highlighted({ text, match }: { text: string; match: string }) {
  if (match === "") {
    return <>{text}</>;
  }
  const parts: React.ReactNode[] = [];
  const lower = text.toLowerCase();
  const needle = match.toLowerCase();
  let at = 0;
  for (let i = lower.indexOf(needle); i >= 0; i = lower.indexOf(needle, at)) {
    parts.push(text.slice(at, i), <strong key={i}>{text.slice(i, i + needle.length)}</strong>);
    at = i + needle.length;
  }
  parts.push(text.slice(at));
  return <>{parts}</>;
}
