import styles from "./case.module.css";

// inputClass styles an input, select or pick list by its state.
export function inputClass(disabled: boolean, invalid: boolean): string {
  return disabled ? styles.inputDisabled : invalid ? styles.inputInvalid : styles.input;
}

// ERR-038: a dropdown or autocomplete list failed to load (BR-UI-004).
export const listFailedMessage = "Gagal mendapatkan data, harap refresh halaman";
