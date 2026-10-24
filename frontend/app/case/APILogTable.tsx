"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { APILogEntry } from "./types";
import styles from "./case.module.css";

// The cut limits come from Dynamic System Settings (ui/apilog/truncate_*,
// UI-020); storage stays whole.
export type LogCut = { chars: number; lines: number };

function cut(text: string, limit: LogCut): string | null {
  const lines = text.split("\n");
  let short = lines.length > limit.lines ? lines.slice(0, limit.lines).join("\n") : text;
  if (short.length > limit.chars) {
    short = short.slice(0, limit.chars);
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

type Column = { key: "name" | "submittedAt" | "status" | "request" | "response"; label: string };

const columns: Column[] = [
  { key: "name", label: "Nama API" },
  { key: "submittedAt", label: "Waktu Connect" },
  { key: "status", label: "Status" },
  { key: "request", label: "Request" },
  { key: "response", label: "Response" },
];

const noValue = "[No Value]";

type Sort = { key: Column["key"]; descending: boolean } | null;

// What is open over the table: one column's menu or its filter popup.
type Open = { kind: "menu" | "filter"; key: Column["key"] } | null;

// UI-020 API Log History table (Gambar 36, 37): a menu per column to sort
// all rows or filter them by the column's values (several columns combine
// with AND); long Request and Response are cut with "..." and opened inline.
// Sorting and filtering run here over all the case's rows (agreed for
// ticket 14: no pages).
export default function APILogTable({ logs, limit }: { logs: APILogEntry[]; limit: LogCut }) {
  const [sort, setSort] = useState<Sort>(null);
  const [filters, setFilters] = useState<Partial<Record<Column["key"], string[]>>>({});
  const [open, setOpen] = useState<Open>(null);
  const head = useRef<HTMLTableSectionElement>(null);

  // Escape or a click outside the open menu or popup closes it.
  useEffect(() => {
    if (!open) {
      return;
    }
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(null);
    const onClick = (e: MouseEvent) => {
      if (!head.current?.contains(e.target as Node)) {
        setOpen(null);
      }
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onClick);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onClick);
    };
  }, [open]);

  const clearFilter = (key: Column["key"]) => {
    const next = { ...filters };
    delete next[key];
    setFilters(next);
    setOpen(null);
  };

  const rows = useMemo(() => {
    const kept = logs.filter((l) =>
      Object.entries(filters).every(([key, values]) => !values || values.includes(l[key as Column["key"]] || noValue)),
    );
    if (!sort) {
      return kept;
    }
    // Waktu Connect sorts by the time itself, the others by their text.
    const value = (l: APILogEntry): number | string => (sort.key === "submittedAt" ? Date.parse(l.at) : l[sort.key]);
    return [...kept].sort((a, b) => {
      const order = value(a) < value(b) ? -1 : value(a) > value(b) ? 1 : 0;
      return sort.descending ? -order : order;
    });
  }, [logs, filters, sort]);

  return (
    <table className={styles.logTable}>
      <thead ref={head}>
        <tr>
          {columns.map((c) => (
            <th key={c.key} className={styles.logHead}>
              <span>{c.label}</span>
              <button
                type="button"
                className={styles.logMenuButton}
                aria-label={`${c.label} menu`}
                aria-haspopup="menu"
                aria-expanded={open?.kind === "menu" && open.key === c.key}
                onClick={() => setOpen(open?.kind === "menu" && open.key === c.key ? null : { kind: "menu", key: c.key })}
              >
                ⋮
              </button>
              {open?.kind === "menu" && open.key === c.key && (
                <ul className={styles.logMenu} role="menu">
                  <li>
                    <button type="button" role="menuitem" onClick={() => (setSort({ key: c.key, descending: false }), setOpen(null))}>
                      <span aria-hidden="true">↑</span> Sort ascending
                    </button>
                  </li>
                  <li>
                    <button type="button" role="menuitem" onClick={() => (setSort({ key: c.key, descending: true }), setOpen(null))}>
                      <span aria-hidden="true">↓</span> Sort descending
                    </button>
                  </li>
                  <li>
                    <button type="button" role="menuitem" onClick={() => setOpen({ kind: "filter", key: c.key })}>
                      <span aria-hidden="true">≡</span> Apply filter
                    </button>
                  </li>
                </ul>
              )}
              {open?.kind === "filter" && open.key === c.key && (
                <FilterPopup
                  logs={logs}
                  column={c.key}
                  checked={filters[c.key]}
                  limit={limit}
                  // Nothing ticked filters nothing: as Clear filter (agreed).
                  onApply={(values) => (values.length === 0 ? clearFilter(c.key) : (setFilters({ ...filters, [c.key]: values }), setOpen(null)))}
                  onClear={() => clearFilter(c.key)}
                  onCancel={() => setOpen(null)}
                />
              )}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((l, i) => (
          <tr key={`${l.at}-${l.name}-${i}`}>
            <td>{l.name}</td>
            <td>{l.submittedAt}</td>
            <td>{l.status}</td>
            <td>
              <LongText text={l.request} limit={limit} />
            </td>
            <td>
              <LongText text={l.response} limit={limit} />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// The "Apply filter" popup (Gambar 37): Clear filter, the column's distinct
// values ("[No Value]" first for an empty one), Search text narrowing the
// list, Apply and Cancel. Long values are cut as in the cells.
function FilterPopup({
  logs,
  column,
  checked,
  limit,
  onApply,
  onClear,
  onCancel,
}: {
  logs: APILogEntry[];
  column: Column["key"];
  checked?: string[];
  limit: LogCut;
  onApply: (values: string[]) => void;
  onClear: () => void;
  onCancel: () => void;
}) {
  const distinct = useMemo(() => {
    const seen = Array.from(new Set(logs.map((l) => l[column] || noValue)));
    return seen.sort((a, b) => (a === noValue ? -1 : b === noValue ? 1 : 0));
  }, [logs, column]);
  const [picked, setPicked] = useState<string[]>(checked ?? []);
  const [search, setSearch] = useState("");
  const shown = distinct.filter((v) => v.toLowerCase().includes(search.toLowerCase()));
  return (
    <div className={styles.filterPopup} role="dialog" aria-label="Apply filter">
      <button type="button" className={styles.filterClear} onClick={onClear}>
        Clear filter
      </button>
      <div className={styles.filterValues}>
        {shown.map((v) => (
          <label key={v} className={styles.filterValue}>
            <input
              type="checkbox"
              checked={picked.includes(v)}
              onChange={(e) => setPicked(e.target.checked ? [...picked, v] : picked.filter((p) => p !== v))}
            />
            <span>{shortened(v, limit)}</span>
          </label>
        ))}
      </div>
      <label className={styles.filterSearch}>
        <span>Search text</span>
        <input type="text" value={search} onChange={(e) => setSearch(e.target.value)} />
      </label>
      <div className={styles.filterButtons}>
        <button type="button" className={styles.pill} onClick={() => onApply(picked)}>
          Apply
        </button>
        <button type="button" className={styles.pill} onClick={onCancel}>
          Cancel
        </button>
      </div>
    </div>
  );
}

// shortened is a value cut with "..." when it is too long.
function shortened(text: string, limit: LogCut): string {
  const short = cut(text, limit);
  return short === null ? text : `${short}...`;
}

// A cell cut with "...", opened inline by "Lihat semua" and cut again by
// "Sembunyikan".
function LongText({ text, limit }: { text: string; limit: LogCut }) {
  const [open, setOpen] = useState(false);
  const short = cut(text, limit);
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
