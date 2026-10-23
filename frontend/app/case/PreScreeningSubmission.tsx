"use client";

import { useState, type ReactNode } from "react";
import { moveAssignment, type AssignmentRef } from "@/lib/api";
import type { FieldError, PersonForm, PreScreeningPage, Section } from "./types";
import AssignmentHeader from "./AssignmentHeader";
import DateField from "./DateField";
import ErrorBanner from "./ErrorBanner";
import Breadcrumb, { preScreeningSteps } from "./Breadcrumb";
import { customerTypeIndividual, unavailableMessage } from "./names";
import styles from "./case.module.css";

const emptyPerson: PersonForm = { nik: "", name: "", birthDate: "" };

type ErrorOf = (section: Section, key: string) => FieldError | undefined;

// UI-010 Pre - Screening Submission, Perorangan form (FR-APP-002). Next and
// the breadcrumb submit the form; errors show only then (AC-FR-APP-002-20).
// The Badan Usaha form arrives in ticket 31. An Authors look at someone
// else's assignment is read-only (agreed).
export default function PreScreeningSubmission({ page, assignment }: { page: PreScreeningPage; assignment: AssignmentRef }) {
  const customerTypeLocked = page.customerType !== ""; // after the first submit (FR-APP-002)
  const readOnly = !page.editable;
  const [customerType, setCustomerType] = useState(page.customerType);
  const [customer, setCustomer] = useState<PersonForm>(page.customer);
  const [spouse, setSpouse] = useState<PersonForm>(page.spouse);
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [failed, setFailed] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  // Changing Tipe Nasabah before submit clears every data field and error.
  function changeType(t: string) {
    setCustomerType(t);
    setCustomer(emptyPerson);
    setSpouse(emptyPerson);
    setErrors([]);
  }

  async function submit() {
    if (submitting || readOnly) {
      return;
    }
    setSubmitting(true);
    setFailed(false);
    const result = await moveAssignment<FieldError>(assignment, "submit", { customerType, customer, spouse });
    if (result.kind === "moved") {
      return;
    }
    setErrors(result.kind === "invalid" ? result.errors : []);
    setFailed(result.kind === "failed");
    setSubmitting(false);
  }

  const errorOf: ErrorOf = (section, key) => errors.find((e) => e.section === section && e.key === key);
  const branchError = errorOf("form", "branch")?.message ?? page.branchError?.message;
  const typeError = errorOf("form", "customerType");

  return (
    <div className={styles.assignmentArea}>
      <ErrorBanner errors={errors} message={failed ? unavailableMessage : undefined} />
      <section className={styles.card}>
        <AssignmentHeader title="Pre - Screening Submission" />

        <Breadcrumb steps={preScreeningSteps} active={0} onSelect={(i) => i === 1 && submit()} />

        <div className={styles.formPanel}>
          <Field label="Nama Branch" error={branchError}>
            <input className={styles.inputDisabled} value={page.branchName} disabled readOnly aria-label="Nama Branch" />
          </Field>
          <Field label="Tipe Nasabah" required error={typeError?.message}>
            <select
              aria-label="Tipe Nasabah"
              className={customerTypeLocked || readOnly ? styles.inputDisabled : typeError ? styles.inputInvalid : styles.input}
              value={customerType}
              disabled={customerTypeLocked || readOnly}
              onChange={(e) => changeType(e.target.value)}
            >
              <option value="" disabled>
                Pilih Tipe Nasabah
              </option>
              {/* Only the Perorangan form exists yet (agreed): other types are
                  listed but cannot be chosen until ticket 31 / TBD-029a. */}
              {page.customerTypeOptions.map((o) => (
                <option key={o.code} value={o.description} disabled={o.description !== customerTypeIndividual}>
                  {o.description}
                </option>
              ))}
            </select>
          </Field>

          {customerType === customerTypeIndividual && (
            <>
              <PersonSection title="Data Pribadi" section="customer" required value={customer} onChange={setCustomer} errorOf={errorOf} page={page} />
              <PersonSection title="Data Pasangan" section="spouse" value={spouse} onChange={setSpouse} errorOf={errorOf} page={page} />
            </>
          )}
        </div>

        <footer className={styles.buttons}>
          {/* Close returns to the Dashboard without saving (TBD-023b/c, agreed). */}
          <button type="button" className={styles.close} onClick={() => window.location.assign(assignment.endpointPath)}>
            Close
          </button>
          {!readOnly && (
            <button type="button" className={styles.next} disabled={submitting} onClick={submit}>
              Next
            </button>
          )}
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
  page,
}: {
  title: string;
  section: Section;
  required?: boolean;
  value: PersonForm;
  onChange: (p: PersonForm) => void;
  errorOf: ErrorOf;
  page: PreScreeningPage;
}) {
  const readOnly = !page.editable;
  const nik = errorOf(section, "nik");
  const name = errorOf(section, "name");
  const born = errorOf(section, "birthDate");
  const inputClass = (invalid: boolean) => (readOnly ? styles.inputDisabled : invalid ? styles.inputInvalid : styles.input);
  return (
    <>
      <h3 className={styles.sectionTitle}>{title}</h3>
      <Field label="NIK" required={required} error={nik?.message}>
        <input
          aria-label={`${title} NIK`}
          className={inputClass(!!nik)}
          value={value.nik}
          disabled={readOnly}
          onChange={(e) => onChange({ ...value, nik: e.target.value })}
        />
      </Field>
      <Field label="Nama" required={required} error={name?.message}>
        <input
          aria-label={`${title} Nama`}
          className={inputClass(!!name)}
          value={value.name}
          disabled={readOnly}
          onChange={(e) => onChange({ ...value, name: e.target.value })}
        />
      </Field>
      <Field label="Tanggal Lahir" required={required} error={born?.message}>
        <DateField
          ariaLabel={`${title} Tanggal Lahir`}
          value={value.birthDate}
          invalid={!!born}
          disabled={readOnly}
          min={page.earliestBirthDate}
          max={page.today}
          today={page.today}
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
