import { sectionNumber, type FieldError } from "./types";
import styles from "./case.module.css";

// Banner prefixes that tell sections apart: "Data Pasangan - " on
// Pre-Screening Submission (agreed), "e-KTP - " and "Domisili - " on Data
// Pribadi (FR-APP-012 Validation, PROPOSED), "Pengurus <n> - " for card n
// of Data Pengurus (FR-APP-002, FR-APP-005).
const sectionPrefixes: Partial<Record<FieldError["section"], string>> = { spouse: "Data Pasangan - ", ektp: "e-KTP - ", domicile: "Domisili - " };

function sectionPrefix(section: FieldError["section"]): string {
  const officer = sectionNumber("company-officer", section);
  if (officer !== null) {
    return `Pengurus ${officer} - `;
  }
  // Card n of Data Pembiayaan Lain (agreed for ticket 18; not in the SoT).
  const financing = sectionNumber("other-financing", section);
  return financing !== null ? `Pembiayaan ${financing} - ` : (sectionPrefixes[section] ?? "");
}

// UI-010 State Error banner (Gambar 13): a red block with a white warning
// sign, then one bullet per error "<Nama Field> : <pesan>", field in bold.
// A message of its own (e.g. a failed request) shows as a single bullet.
// ERR-020 reads "Notes: <pesan>" (Gambar 24); an error without a field
// shows its message alone.
export default function ErrorBanner({ errors, message }: { errors: FieldError[]; message?: string }) {
  if (errors.length === 0 && !message) {
    return null;
  }
  return (
    <div className={styles.errorBanner} role="alert">
      <span className={styles.errorBannerIcon} aria-hidden="true">
        ⚠
      </span>
      <ul className={styles.errorBannerList}>
        {errors.map((e, i) => (
          <li key={`${e.section}-${e.key}-${i}`}>
            {e.code === "ERR-020" ? (
              <strong>{e.field}:</strong>
            ) : (
              e.field && (
                <strong>
                  {sectionPrefix(e.section)}
                  {e.field} :
                </strong>
              )
            )}
            {e.field && " "}
            {e.message}
          </li>
        ))}
        {message && <li>{message}</li>}
      </ul>
    </div>
  );
}
