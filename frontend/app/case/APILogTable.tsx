"use client";

import { useState } from "react";
import type { APILogEntry } from "./types";
import styles from "./case.module.css";

// Request and Response are cut on display at 200 characters or 5 lines,
// whichever comes first (UI-020, PROPOSED); storage stays whole.
const cutChars = 200;
const cutLines = 5;

function cut(text: string): string | null {
  const lines = text.split("\n");
  let short = lines.length > cutLines ? lines.slice(0, cutLines).join("\n") : text;
  if (short.length > cutChars) {
    short = short.slice(0, cutChars);
  }
  return short.length < text.length ? short : null;
}

// pretty shows valid JSON with line breaks and 2-space indentation, other
// text (error messages) as it is (UI-020).
function pretty(text: string): string {
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}

// UI-020 API Log History table, without the sort and filter menus (ticket 14).
export default function APILogTable({ logs }: { logs: APILogEntry[] }) {
  return (
    <table className={styles.logTable}>
      <thead>
        <tr>
          <th>Nama API</th>
          <th>Waktu Connect</th>
          <th>Status</th>
          <th>Request</th>
          <th>Response</th>
        </tr>
      </thead>
      <tbody>
        {logs.map((l, i) => (
          <tr key={i}>
            <td>{l.name}</td>
            <td>{l.submittedAt}</td>
            <td>{l.status}</td>
            <td>
              <LongText text={l.request} />
            </td>
            <td>
              <LongText text={l.response} />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// A cell cut with "...", opened inline by "Lihat semua" and cut again by
// "Sembunyikan".
function LongText({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  const short = cut(text);
  if (short === null) {
    return <span className={styles.logText}>{text}</span>;
  }
  return (
    <span>
      {open ? <pre className={styles.logFull}>{pretty(text)}</pre> : <span className={styles.logText}>{short}...</span>}{" "}
      <button type="button" className={styles.logToggle} onClick={() => setOpen(!open)}>
        {open ? "Sembunyikan" : "Lihat semua"}
      </button>
    </span>
  );
}
