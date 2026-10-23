import styles from "./case.module.css";

// The two assignments of the Pre-Screening Submission tabbed flow (FR-APP-003).
export const preScreeningSteps = ["Pre - Screening Submission", "Document Underlying Upload"];

// UI-010 breadcrumb: steps joined by a line, the active one filled dark;
// clicking another step asks to move there (validated by the server).
export default function Breadcrumb({
  steps,
  active,
  onSelect,
}: {
  steps: string[];
  active: number;
  onSelect: (index: number) => void;
}) {
  return (
    <ol className={styles.breadcrumb}>
      {steps.map((label, i) => (
        <li key={label} className={i === active ? styles.crumbActive : styles.crumb}>
          <button
            type="button"
            className={styles.crumbButton}
            aria-current={i === active ? "step" : undefined}
            onClick={() => i !== active && onSelect(i)}
          >
            <span className={styles.crumbLabel}>{label}</span>
            <span className={styles.crumbDot} />
          </button>
        </li>
      ))}
    </ol>
  );
}
