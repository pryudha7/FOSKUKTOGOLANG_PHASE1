"use client";

import { useState, type ReactNode } from "react";
import { apiFetch, assignmentPath } from "@/lib/api";
import { useOperator } from "@/app/session/SessionContext";
import type { FieldError, PersonForm, PreScreeningPage } from "./types";
import DateField from "./DateField";
import ErrorBanner from "./ErrorBanner";
import Breadcrumb, { preScreeningSteps } from "./Breadcrumb";
import styles from "./case.module.css";

const emptyPerson: PersonForm = { nik: "", name: "", birthDate: "" };

// UI-010 Pre - Screening Submission, Perorangan form (FR-APP-002). Next and
// the breadcrumb submit the form; errors show only then (AC-FR-APP-002-20).
// The Badan Usaha form arrives in ticket 31.
export default function PreScreeningSubmission({
  page,
  endpointPath,
  caseId,
  assignmentId,
}: {
  page: PreScreeningPage;
  endpointPath: string;
  caseId: string;
  assignmentId: number;
}) {
  const me = useOperator();
  const fixed = page.customerType !== ""; // disabled after submit (FR-APP-002)
  const [customerType, setCustomerType] = useState(page.customerType);
  const [personal, setPersonal] = useState<PersonForm>(page.personal);
  const [spouse, setSpouse] = useState<PersonForm>(page.spouse);
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [submitting, setSubmitting] = useState(false);

  // Changing Tipe Nasabah before submit clears every data field and error.
  function changeType(t: string) {
    setCustomerType(t);
    setPersonal(emptyPerson);
    setSpouse(emptyPerson);
    setErrors([]);
  }

  async function submit() {
    if (submitting) {
      return;
    }
    setSubmitting(true);
    const res = await apiFetch(`${endpointPath}/api/cases/${encodeURIComponent(caseId)}/assignments/${assignmentId}/submit`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ customerType, personal, spouse }),
    }).catch(() => null);
    if (res?.ok) {
      const body = (await res.json()) as { assignmentId: number };
      window.location.assign(assignmentPath(endpointPath, caseId, body.assignmentId));
      return;
    }
    if (res?.status === 422) {
      setErrors(((await res.json()) as { errors: FieldError[] }).errors);
    }
    setSubmitting(false);
  }

  const errorOf = (section: string, field: string) => errors.find((e) => e.section === section && e.field === field);
  const branchError = errorOf("form", "Nama Branch") ?? (page.branchError ? { message: page.branchError.message } : undefined);

  return (
    <div className={styles.assignmentArea}>
      <ErrorBanner errors={errors} />
      <section className={styles.card}>
        <header className={styles.assignmentHeader}>
          {/* UI-010: the avatar shows the logged-in operator's initial. */}
          <span className={styles.assignmentAvatar} aria-hidden="true">
            {me.fullName.trim().charAt(0).toUpperCase()}
          </span>
          <h2 className={styles.cardTitle}>Pre - Screening Submission</h2>
        </header>

        <Breadcrumb steps={preScreeningSteps} active={0} onSelect={(i) => i === 1 && submit()} />

        <div className={styles.formPanel}>
          <Field label="Nama Branch" error={branchError?.message}>
            <input className={styles.inputDisabled} value={page.branchName} disabled readOnly aria-label="Nama Branch" />
          </Field>
          <Field label="Tipe Nasabah" required error={errorOf("form", "Tipe Nasabah")?.message}>
            <select
              aria-label="Tipe Nasabah"
              className={fixed ? styles.inputDisabled : errorOf("form", "Tipe Nasabah") ? styles.inputInvalid : styles.input}
              value={customerType}
              disabled={fixed}
              onChange={(e) => changeType(e.target.value)}
            >
              <option value="" disabled>
                Pilih Tipe Nasabah
              </option>
              {page.customerTypeOptions.map((o) => (
                <option key={o.code} value={o.description}>
                  {o.description}
                </option>
              ))}
            </select>
          </Field>

          {customerType === "Perorangan" && (
            <>
              <PersonSection title="Data Pribadi" section="personal" required value={personal} onChange={setPersonal} errorOf={errorOf} />
              <PersonSection title="Data Pasangan" section="spouse" value={spouse} onChange={setSpouse} errorOf={errorOf} />
            </>
          )}
        </div>

        <footer className={styles.buttons}>
          {/* Close returns to the Dashboard without saving (TBD-023b/c, agreed). */}
          <button type="button" className={styles.close} onClick={() => window.location.assign(endpointPath)}>
            Close
          </button>
          <button type="button" className={styles.next} disabled={submitting} onClick={submit}>
            Next
          </button>
        </footer>
      </section>
    </div>
  );
}

function PersonSection({
  title,
  section,
  required = false,
  value,
  onChange,
  errorOf,
}: {
  title: string;
  section: "personal" | "spouse";
  required?: boolean;
  value: PersonForm;
  onChange: (p: PersonForm) => void;
  errorOf: (section: string, field: string) => FieldError | undefined;
}) {
  const nik = errorOf(section, "NIK");
  const name = errorOf(section, "Nama");
  const born = errorOf(section, "Tanggal Lahir");
  return (
    <>
      <h3 className={styles.sectionTitle}>{title}</h3>
      <Field label="NIK" required={required} error={nik?.message}>
        <input
          aria-label={`${title} NIK`}
          className={nik ? styles.inputInvalid : styles.input}
          value={value.nik}
          onChange={(e) => onChange({ ...value, nik: e.target.value })}
        />
      </Field>
      <Field label="Nama" required={required} error={name?.message}>
        <input
          aria-label={`${title} Nama`}
          className={name ? styles.inputInvalid : styles.input}
          value={value.name}
          onChange={(e) => onChange({ ...value, name: e.target.value })}
        />
      </Field>
      <Field label="Tanggal Lahir" required={required} error={born?.message}>
        <DateField
          ariaLabel={`${title} Tanggal Lahir`}
          value={value.birthDate}
          invalid={!!born}
          onChange={(birthDate) => onChange({ ...value, birthDate })}
        />
      </Field>
    </>
  );
}

// A labelled field: label above, asterisk after a required label, and on
// error a red label and the message under the input (UI-010 State Error).
function Field({
  label,
  required = false,
  error,
  children,
}: {
  label: string;
  required?: boolean;
  error?: string;
  children: ReactNode;
}) {
  return (
    <div className={styles.field}>
      <span className={error ? styles.fieldLabelInvalid : styles.fieldLabel}>
        {label}
        {required && <span className={styles.required}> *</span>}
      </span>
      {children}
      {error && <span className={styles.fieldError}>{error}</span>}
    </div>
  );
}
