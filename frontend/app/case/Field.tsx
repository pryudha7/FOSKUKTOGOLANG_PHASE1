import type { ReactNode } from "react";
import styles from "./case.module.css";

// A labelled field: label above, asterisk after a required label, and on
// error a red label and the message under the input (UI-010 State Error).
export default function Field({
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
