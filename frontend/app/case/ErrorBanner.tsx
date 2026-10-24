import { sectionNumber, type FieldError, type NumberedSection } from "./types";
import styles from "./case.module.css";

// Banner prefixes that tell sections apart: "Data Pasangan - " on
// Pre-Screening Submission (agreed), "e-KTP - " and "Domisili - " on Data
// Pribadi (FR-APP-012 Validation, PROPOSED), and "<card> <n> - " for card
// n of a list: Data Pengurus (FR-APP-002, FR-APP-005), Data Pembiayaan Lain
// and the collateral sections (agreed for tickets 18 and 21; not in the SoT).
const sectionPrefixes: Partial<Record<FieldError["section"], string>> = { spouse: "Data Pasangan - ", ektp: "e-KTP - ", domicile: "Domisili - " };
const cardNames: [NumberedSection, string][] = [
  ["company-officer", "Pengurus"],
  ["other-financing", "Pembiayaan"],
  ["main-collateral", "Agunan Utama"],
  ["support-collateral", "Jaminan Pendukung"],
];

function sectionPrefix(section: FieldError["section"]): string {
  for (const [prefix, name] of cardNames) {
    const n = sectionNumber(prefix, section);
    if (n !== null) {
      return `${name} ${n} - `;
    }
  }
  return sectionPrefixes[section] ?? "";
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
