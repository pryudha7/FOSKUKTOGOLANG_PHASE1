import styles from "./shell.module.css";

// BR-UI-001: no image; a circle with the first letter of the displayed
// name, upper case, white. Operator circles are ≈ #2C96DF.
export default function Avatar({ name }: { name: string }) {
  return (
    <span className={styles.avatar} aria-hidden="true">
      {name.trim().charAt(0).toUpperCase()}
    </span>
  );
}
