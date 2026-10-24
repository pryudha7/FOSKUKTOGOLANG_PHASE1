"use client";

import styles from "./case.module.css";

// ERR-035 (Gambar 43): a popup in the middle of the page over a dark layer,
// titled with the application host and a globe icon, with one OK button.
// ERR-039 uses the same popup with its own message.
export const noAccessMessage = "Unable to open the attachment. You lack the required permissions.";

export default function NoAccessPopup({ onClose, message = noAccessMessage }: { onClose: () => void; message?: string }) {
  return (
    <div className={styles.overlay} role="presentation">
      <div className={styles.noAccess} role="alertdialog" aria-labelledby="no-access-host" aria-describedby="no-access-text">
        <div id="no-access-host" className={styles.noAccessHost}>
          <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8">
            <circle cx="12" cy="12" r="9.5" />
            <path d="M2.5 12h19M12 2.5c2.6 2.8 3.9 6 3.9 9.5S14.6 18.7 12 21.5M12 2.5C9.4 5.3 8.1 8.5 8.1 12s1.3 6.7 3.9 9.5" />
          </svg>
          {window.location.host}
        </div>
        <p id="no-access-text" className={styles.noAccessText}>
          {message}
        </p>
        <div className={styles.noAccessActions}>
          <button type="button" className={styles.noAccessOk} onClick={onClose} autoFocus>
            OK
          </button>
        </div>
      </div>
    </div>
  );
}
