"use client";

import { useEffect, useState, type ReactNode } from "react";
import { apiFetch } from "@/lib/api";
import type { Option } from "./types";
import styles from "./case.module.css";

// inputClass styles an input, select or pick list by its state.
export function inputClass(disabled: boolean, invalid: boolean): string {
  return disabled ? styles.inputDisabled : invalid ? styles.inputInvalid : styles.input;
}

// ERR-038: the suggestion query failed.
const suggestionsFailed = "Gagal mendapatkan data, harap refresh halaman";

// An autocomplete field (BR-UI-005): each pause of 300 ms in typing asks
// source?q=<text> (the server limits and orders the list); matching letters
// are bold; the first item is highlighted; arrows and Enter or a click
// choose. text is what the field shows; onType reports typing, onChoose a
// pick. renderOption lays out an item (the description by default).
export default function Autocomplete({
  label,
  source,
  text,
  onType,
  onChoose,
  renderOption = (o, match) => <Highlighted text={o.description} match={match} />,
  invalid,
  disabled,
}: {
  label: string;
  source: string;
  text: string;
  onType: (text: string) => void;
  onChoose: (o: Option) => void;
  renderOption?: (o: Option, match: string) => ReactNode;
  invalid: boolean;
  disabled: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [options, setOptions] = useState<Option[]>([]);
  const [failed, setFailed] = useState(false);
  const [active, setActive] = useState(0);

  useEffect(() => {
    if (!open) {
      return;
    }
    let live = true;
    const timer = setTimeout(() => {
      apiFetch(`${source}?q=${encodeURIComponent(text)}`)
        .then((res) => (res.ok ? (res.json() as Promise<{ options: Option[] }>) : Promise.reject()))
        .then((body) => {
          if (live) {
            setOptions(body.options);
            setFailed(false);
            setActive(0);
          }
        })
        .catch(() => live && setFailed(true));
    }, 300);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [source, text, open]);

  function choose(o: Option) {
    onChoose(o);
    setOpen(false);
  }

  return (
    <div className={styles.autocomplete}>
      <input
        aria-label={label}
        role="combobox"
        aria-expanded={open}
        aria-autocomplete="list"
        className={inputClass(disabled, invalid)}
        value={text}
        disabled={disabled}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onChange={(e) => {
          onType(e.target.value);
          setOpen(true);
        }}
        onKeyDown={(e) => {
          if (!open || failed || options.length === 0) {
            return;
          }
          if (e.key === "ArrowDown") {
            e.preventDefault();
            setActive((i) => Math.min(i + 1, options.length - 1));
          } else if (e.key === "ArrowUp") {
            e.preventDefault();
            setActive((i) => Math.max(i - 1, 0));
          } else if (e.key === "Enter") {
            e.preventDefault();
            choose(options[active]);
          } else if (e.key === "Escape") {
            setOpen(false);
          }
        }}
      />
      {open && (failed || options.length > 0) && (
        <ul className={styles.autocompleteList} role="listbox" aria-label={label}>
          {failed ? (
            <li className={styles.autocompleteFailed} aria-disabled="true">
              {suggestionsFailed}
            </li>
          ) : (
            options.map((o, i) => (
              <li
                key={o.code}
                role="option"
                aria-selected={i === active}
                className={i === active ? styles.autocompleteActive : styles.autocompleteOption}
                // Chosen before the input's blur closes the list.
                onMouseDown={(e) => {
                  e.preventDefault();
                  choose(o);
                }}
              >
                {renderOption(o, text)}
              </li>
            ))
          )}
        </ul>
      )}
    </div>
  );
}

// Highlighted shows text with each case-insensitive occurrence of match bold.
export function Highlighted({ text, match }: { text: string; match: string }) {
  if (match === "") {
    return <>{text}</>;
  }
  const parts: ReactNode[] = [];
  const lower = text.toLowerCase();
  const needle = match.toLowerCase();
  let at = 0;
  for (let i = lower.indexOf(needle); i >= 0; i = lower.indexOf(needle, at)) {
    parts.push(text.slice(at, i), <strong key={i}>{text.slice(i, i + needle.length)}</strong>);
    at = i + needle.length;
  }
  parts.push(text.slice(at));
  return <>{parts}</>;
}
