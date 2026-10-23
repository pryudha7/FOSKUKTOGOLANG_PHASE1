import type { FieldError } from "./types";
import styles from "./case.module.css";

// Banner prefixes that tell sections apart, like "Pengurus <n> - " in the
// SoT (agreed: "Data Pasangan - "; Data Pribadi has none).
const sectionPrefix: Partial<Record<FieldError["section"], string>> = { spouse: "Data Pasangan - " };

// UI-010 State Error banner (Gambar 13): a red block with a white warning
// sign, then one bullet per error "<Nama Field> : <pesan>", field in bold.
// A message of its own (e.g. a failed request) shows as a single bullet.
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
            <strong>
              {sectionPrefix[e.section] ?? ""}
              {e.field} :
            </strong>{" "}
            {e.message}
          </li>
        ))}
        {message && <li>{message}</li>}
      </ul>
    </div>
  );
}
