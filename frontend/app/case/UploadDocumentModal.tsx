"use client";

import { useRef, useState, type DragEvent } from "react";
import { apiFetch, type AssignmentRef } from "@/lib/api";
import { acceptedTypes, uploadPath } from "./documents";
import styles from "./case.module.css";

// ERR-025 when the request itself fails (agreed interim text).
const uploadFailedMessage = "Upload dokumen gagal. Silakan coba lagi.";

// withoutExtension is Name's default: the file name without its extension.
function withoutExtension(fileName: string): string {
  const dot = fileName.lastIndexOf(".");
  return dot > 0 ? fileName.slice(0, dot) : fileName;
}

// UI-015 Upload Document (Gambar 21, 22): one file for one document row.
// Submit stores it at once (FR-FILE-002); Cancel and X close without
// saving. Closing by Esc or a click outside is TBD-036g, so neither closes.
export default function UploadDocumentModal({
  assignment,
  category,
  documentType,
  onClose,
  onUploaded,
}: {
  assignment: AssignmentRef;
  category: { code: string; label: string };
  documentType: string;
  onClose: () => void;
  onUploaded: () => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [name, setName] = useState("");
  const [errors, setErrors] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const input = useRef<HTMLInputElement>(null);

  // Only the first file is used; choosing again replaces it and refills Name
  // (FR-FILE-002 rules 1, 2). The banner clears on a file change (agreed).
  function choose(files: FileList | null) {
    const first = files?.[0];
    if (!first) {
      return;
    }
    setFile(first);
    setName(withoutExtension(first.name));
    setErrors([]);
  }

  function drop(e: DragEvent) {
    e.preventDefault();
    choose(e.dataTransfer.files);
  }

  function remove() {
    setFile(null);
    setName("");
    setErrors([]);
    if (input.current) {
      input.current.value = "";
    }
  }

  async function submit() {
    // Submit without a file is TBD-036a: nothing happens.
    if (!file || submitting) {
      return;
    }
    setErrors([]);
    setSubmitting(true);
    const form = new FormData();
    form.append("category", category.code);
    form.append("documentType", documentType);
    form.append("name", name);
    form.append("file", file);
    const res = await apiFetch(uploadPath(assignment.endpointPath, assignment.caseId, assignment.assignmentId), {
      method: "POST",
      body: form,
    }).catch(() => null);
    setSubmitting(false);
    if (res?.status === 201) {
      onUploaded();
      return;
    }
    const body = res ? ((await res.json().catch(() => null)) as { errors?: { message: string }[] } | null) : null;
    setErrors(body?.errors?.map((e) => e.message) ?? [uploadFailedMessage]);
  }

  return (
    <div className={styles.overlay} role="presentation">
      <div className={styles.modal} role="dialog" aria-modal="true" aria-labelledby="upload-title">
        <header className={styles.modalHeader}>
          <h2 id="upload-title" className={styles.modalTitle}>
            Upload Document
          </h2>
          <button type="button" className={styles.modalX} aria-label="Close" onClick={onClose}>
            ×
          </button>
        </header>
        {errors.length > 0 && (
          <ul className={styles.modalErrors} role="alert">
            {errors.map((m, i) => (
              <li key={i}>{m}</li>
            ))}
          </ul>
        )}
        <div className={styles.dropArea} onDragOver={(e) => e.preventDefault()} onDrop={drop}>
          <svg width="56" height="56" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="#3b3b3b" strokeWidth="2" strokeLinecap="round">
            <path d="M21.4 11.1l-8.5 8.5a5.5 5.5 0 0 1-7.8-7.8l8.5-8.5a3.7 3.7 0 0 1 5.2 5.2l-8.5 8.5a1.8 1.8 0 0 1-2.6-2.6l7.8-7.8" />
          </svg>
          <strong>Drag and drop files here</strong>
        </div>
        <span className={styles.modalOr}>or</span>
        <button type="button" className={styles.selectFiles} onClick={() => input.current?.click()}>
          Select file(s)
        </button>
        <input
          ref={input}
          type="file"
          multiple
          accept={acceptedTypes}
          hidden
          aria-label="Select file(s)"
          onChange={(e) => choose(e.target.files)}
        />
        {file && (
          <table className={styles.fileTable}>
            <thead>
              <tr>
                <th>
                  Name<span className={styles.fileRequired}>*</span>
                </th>
                <th>File</th>
                <th>Category</th>
                <th aria-label="Remove" />
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>
                  <input aria-label="Name" className={styles.fileName} value={name} onChange={(e) => setName(e.target.value)} />
                </td>
                <td className={styles.fileOriginal}>{file.name}</td>
                <td>
                  <select aria-label="Category" className={styles.fileCategory} disabled value={category.code}>
                    <option value={category.code}>{category.label}</option>
                  </select>
                </td>
                <td>
                  <button type="button" className={styles.fileRemove} aria-label="Remove file" onClick={remove}>
                    <svg width="16" height="16" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8">
                      <path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13M10 11v6M14 11v6" />
                    </svg>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        )}
        <footer className={styles.modalButtons}>
          <button type="button" className={styles.modalCancel} onClick={onClose}>
            Cancel
          </button>
          <button type="button" className={styles.modalSubmit} disabled={submitting} onClick={submit}>
            Submit
          </button>
        </footer>
      </div>
    </div>
  );
}
