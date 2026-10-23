"use client";

import { useState } from "react";
import type { PreScreeningPage } from "./types";
import styles from "./case.module.css";

// UI-010 Pre - Screening Submission. This ticket shows Nama Branch and Tipe
// Nasabah; the Data Pribadi / Badan Usaha sections and Next arrive in
// tickets 30 and 31.
export default function PreScreeningSubmission({ page, endpointPath }: { page: PreScreeningPage; endpointPath: string }) {
  const [customerType, setCustomerType] = useState(page.customerType);
  const fixed = page.customerType !== ""; // disabled after submit (FR-APP-002)

  return (
    <section className={styles.card}>
      <header className={styles.assignmentHeader}>
        <span className={styles.assignmentAvatar} aria-hidden="true">
          P
        </span>
        <h2 className={styles.cardTitle}>Pre - Screening Submission</h2>
      </header>

      <ol className={styles.breadcrumb}>
        <li className={styles.crumbActive}>
          <span className={styles.crumbLabel}>Pre - Screening Submission</span>
          <span className={styles.crumbDot} />
        </li>
        <li className={styles.crumb}>
          <span className={styles.crumbLabel}>Document Underlying Upload</span>
          <span className={styles.crumbDot} />
        </li>
      </ol>

      <div className={styles.formPanel}>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Nama Branch</span>
          <input className={styles.inputDisabled} value={page.branchName} disabled readOnly />
          {page.branchError && <span className={styles.fieldError}>{page.branchError.message}</span>}
        </label>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>
            Tipe Nasabah <span className={styles.required}>*</span>
          </span>
          <select
            className={fixed ? styles.inputDisabled : styles.input}
            value={customerType}
            disabled={fixed}
            onChange={(e) => setCustomerType(e.target.value)}
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
        </label>
      </div>

      <footer className={styles.buttons}>
        {/* Close returns to the Dashboard; the case and assignment stay (TBD-023b, agreed). */}
        <button type="button" className={styles.close} onClick={() => window.location.assign(endpointPath)}>
          Close
        </button>
        <button type="button" className={styles.next}>
          Next
        </button>
      </footer>
    </section>
  );
}
