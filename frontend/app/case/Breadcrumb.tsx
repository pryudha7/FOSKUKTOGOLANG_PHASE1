import styles from "./case.module.css";

// The two assignments of the Pre-Screening Submission tabbed flow (FR-APP-003).
export const preScreeningSteps = ["Pre - Screening Submission", "Document Underlying Upload"];

// The three assignments of the Prospect Data Entry Perorangan tabbed flow
// (FR-APP-012).
export const prospectSteps = ["Data Pribadi", "Data Pasangan", "Data Keluarga Dekat"];

// The six assignments of the Hot Prospect Data Entry tabbed flow (FR-APP-015).
export const hotProspectSteps = ["Data Usaha Nasabah", "Data Pembiayaan Lain", "Data Permohonan Fasilitas", "QCA Data", "Collateral Data", "Document Upload"];

// UI-010 breadcrumb: steps joined by a line, the active one filled dark;
// clicking another step asks to move there (validated by the server). With
// progress (UI-026 to UI-028) the line is dark up to the active step and
// the steps before it have dark rings.
export default function Breadcrumb({
  steps,
  active,
  onSelect,
  progress = false,
}: {
  steps: string[];
  active: number;
  onSelect: (index: number) => void;
  progress?: boolean;
}) {
  const line = { "--edge": `${50 / steps.length}%`, "--done": progress ? active / (steps.length - 1) : 0 } as React.CSSProperties;
  return (
    <ol className={styles.breadcrumb} style={line}>
      {steps.map((label, i) => (
        <li key={label} className={i === active ? styles.crumbActive : progress && i < active ? styles.crumbDone : styles.crumb}>
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
