"use client";

import { useEffect, useRef, useState } from "react";
import {
  compare,
  daysInMonth,
  firstColumn,
  isoToLong,
  isoToTyped,
  monthsShort,
  parseISO,
  toISO,
  typedToISO,
  weekdays,
  type YMD,
} from "./dates";
import styles from "./case.module.css";

// A date field (BR-UI-011): typed as dd/MM/yyyy while focused, shown in
// the long Indonesian form otherwise, with the UI-010 date picker behind the
// calendar icon. value and onChange use yyyy-mm-dd. The input always holds
// the typed form; the long form is an overlay shown while unfocused, so
// focusing never rewrites the input (selection and typing stay intact).
export default function DateField({
  value,
  onChange,
  invalid,
  min,
  max,
  today,
  disabled = false,
  ariaLabel,
}: {
  value: string;
  onChange: (iso: string) => void;
  invalid?: boolean;
  // Dates the picker offers, yyyy-mm-dd (typing is validated by the server).
  min?: string;
  max?: string;
  // The operator's today, yyyy-mm-dd, for "Today" and the opening month.
  today: string;
  disabled?: boolean;
  ariaLabel: string;
}) {
  const [focused, setFocused] = useState(false);
  const [typed, setTyped] = useState(isoToTyped(value));
  const [picking, setPicking] = useState(false);

  useEffect(() => {
    if (!focused) {
      setTyped(isoToTyped(value));
    }
  }, [value, focused]);

  const showLong = !focused && value !== "";
  return (
    <div className={styles.dateField}>
      <input
        className={`${disabled ? styles.inputDisabled : invalid ? styles.inputInvalid : styles.input} ${showLong ? styles.dateHidden : ""}`}
        aria-label={ariaLabel}
        value={typed}
        disabled={disabled}
        onFocus={() => setFocused(true)}
        onChange={(e) => {
          setTyped(e.target.value);
          onChange(e.target.value === "" ? "" : typedToISO(e.target.value));
        }}
        onBlur={() => setFocused(false)}
      />
      {showLong && (
        <span className={styles.dateLong} aria-hidden="true">
          {isoToLong(value)}
        </span>
      )}
      <button
        type="button"
        className={styles.calendarButton}
        aria-label="Pilih tanggal"
        disabled={disabled}
        onClick={() => setPicking(!picking)}
      >
        <CalendarIcon />
      </button>
      {picking && (
        <DatePicker
          selected={parseISO(value)}
          min={min ? parseISO(min) : null}
          max={max ? parseISO(max) : null}
          today={parseISO(today)!}
          onPick={(d) => {
            onChange(toISO(d));
            setPicking(false);
          }}
          onClose={() => setPicking(false)}
        />
      )}
    </div>
  );
}

// UI-010 date picker (Gambar 12): month and year selectors, a Monday-first
// grid with neighbouring months' days in grey, the selection in blue, and
// "Today" / "Close" in the footer.
function DatePicker({
  selected,
  min,
  max,
  today: now,
  onPick,
  onClose,
}: {
  selected: YMD | null;
  min: YMD | null;
  max: YMD | null;
  today: YMD;
  onPick: (d: YMD) => void;
  onClose: () => void;
}) {
  const [view, setView] = useState({ y: selected?.y ?? now.y, m: selected?.m ?? now.m });
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const close = (e: MouseEvent) => {
      if (!ref.current?.parentElement?.contains(e.target as Node)) {
        onClose();
      }
    };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [onClose]);

  const cells: { day: YMD; inMonth: boolean }[] = [];
  const lead = firstColumn(view.y, view.m);
  const prev = view.m === 1 ? { y: view.y - 1, m: 12 } : { y: view.y, m: view.m - 1 };
  const next = view.m === 12 ? { y: view.y + 1, m: 1 } : { y: view.y, m: view.m + 1 };
  const prevDays = daysInMonth(prev.y, prev.m);
  for (let i = lead - 1; i >= 0; i--) {
    cells.push({ day: { ...prev, d: prevDays - i }, inMonth: false });
  }
  for (let d = 1; d <= daysInMonth(view.y, view.m); d++) {
    cells.push({ day: { ...view, d }, inMonth: true });
  }
  for (let d = 1; cells.length % 7 !== 0 || cells.length < 42; d++) {
    cells.push({ day: { ...next, d }, inMonth: false });
  }
  const disabled = (d: YMD) => (min !== null && compare(d, min) < 0) || (max !== null && compare(d, max) > 0);
  const years = Array.from({ length: 121 }, (_, i) => now.y - 110 + i);

  return (
    <div className={styles.datePicker} ref={ref} role="dialog" aria-label="Date picker">
      <div className={styles.dpHeader}>
        <select aria-label="Bulan" value={view.m} onChange={(e) => setView({ ...view, m: Number(e.target.value) })}>
          {monthsShort.map((name, i) => (
            <option key={name} value={i + 1}>
              {name}
            </option>
          ))}
        </select>
        <select aria-label="Tahun" value={view.y} onChange={(e) => setView({ ...view, y: Number(e.target.value) })}>
          {years.map((y) => (
            <option key={y} value={y}>
              {y}
            </option>
          ))}
        </select>
      </div>
      <div className={styles.dpGrid}>
        {weekdays.map((w) => (
          <span key={w} className={styles.dpWeekday}>
            {w}
          </span>
        ))}
        {cells.map(({ day, inMonth }) => {
          const isSelected = selected !== null && compare(day, selected) === 0;
          return (
            <button
              key={toISO(day)}
              type="button"
              disabled={disabled(day)}
              className={`${styles.dpDay} ${inMonth ? "" : styles.dpOther} ${isSelected ? styles.dpSelected : ""}`}
              onClick={() => onPick(day)}
            >
              {day.d}
            </button>
          );
        })}
      </div>
      <div className={styles.dpFooter}>
        <button type="button" className={styles.dpToday} disabled={disabled(now)} onClick={() => onPick(now)}>
          Today
        </button>
        <button type="button" className={styles.dpClose} onClick={onClose}>
          Close
        </button>
      </div>
    </div>
  );
}

function CalendarIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#444b52" strokeWidth="1.6" aria-hidden="true">
      <rect x="3" y="5" width="18" height="16" rx="2" />
      <path d="M3 10h18M8 3v4M16 3v4" />
      <path d="M8 14h.01M12 14h.01M16 14h.01M8 18h.01M12 18h.01" strokeWidth="2.4" strokeLinecap="round" />
    </svg>
  );
}
