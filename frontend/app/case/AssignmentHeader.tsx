"use client";

import { useOperator } from "@/app/session/SessionContext";
import styles from "./case.module.css";

// Assignment card header: the logged-in operator's initial in a blue
// circle and the assignment title (UI-010, UI-011).
export default function AssignmentHeader({ title }: { title: string }) {
  const me = useOperator();
  return (
    <header className={styles.assignmentHeader}>
      <span className={styles.assignmentAvatar} aria-hidden="true">
        {me.fullName.trim().charAt(0).toUpperCase()}
      </span>
      <h2 className={styles.cardTitle}>{title}</h2>
    </header>
  );
}
