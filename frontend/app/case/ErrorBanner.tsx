import type { FieldError } from "./types";
import styles from "./case.module.css";

// UI-010 State Error banner (Gambar 13): a red block with a white warning
// sign, then one bullet per error "<Nama Field> : <pesan>", field in bold.
export default function ErrorBanner({ errors }: { errors: FieldError[] }) {
  if (errors.length === 0) {
    return null;
  }
  return (
    <div className={styles.errorBanner} role="alert">
      <span className={styles.errorBannerIcon} aria-hidden="true">
        ⚠
      </span>
      <ul className={styles.errorBannerList}>
        {errors.map((e, i) => (
          <li key={`${e.section}-${e.field}-${i}`}>
            <strong>{e.field} :</strong> {e.message}
          </li>
        ))}
      </ul>
    </div>
  );
}
