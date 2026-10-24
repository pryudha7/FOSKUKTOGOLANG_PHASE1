"use client";

// Form pieces shared by the Prospect Data Entry pages (UI-025 to UI-028):
// moving a form, fields, BMV dropdowns and radios, numbers, and the region
// cascade.

import { useEffect, useState, type ReactNode } from "react";
import { apiFetch, assignmentPath, assignmentURL, type AssignmentRef } from "@/lib/api";
import Field from "./Field";
import { inputClass, listFailedMessage } from "./fields";
import type { FieldError, Option, Region, Section } from "./types";
import styles from "./case.module.css";

// The endpoint of each form and what moves it: a step name, or Create.
export type FormPath = "individual-data" | "spouse" | "close-relative" | "business-entity" | "business-data" | "other-financing" | "facility-request" | "qca";
export const create = "create";
// save is Save on Hot Prospect: the form is kept and the page stays.
export const save = "save";

// useMove holds a form's errors and moves it (FR-APP-012 Process 4, agreed
// comment 3): the server validates and saves the form, then the browser
// opens the target step, or the Dashboard after Create (agreed comment 5).
export function useMove(assignment: AssignmentRef, form: FormPath) {
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [failed, setFailed] = useState(false);
  const [busy, setBusy] = useState(false);

  async function move(to: string, body: unknown) {
    if (busy) {
      return;
    }
    setBusy(true);
    setFailed(false);
    const res = await apiFetch(`${assignmentURL(assignment, form)}?to=${encodeURIComponent(to)}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }).catch(() => null);
    if (res?.ok && to === save) {
      setErrors([]);
      setBusy(false);
      return;
    }
    if (res?.ok) {
      const next = (await res.json()) as { assignmentId: number };
      window.location.assign(to === create ? assignment.endpointPath : assignmentPath(assignment.endpointPath, assignment.caseId, next.assignmentId));
      return;
    }
    const invalid = res?.status === 422 ? ((await res.json().catch(() => null)) as { errors?: FieldError[] } | null) : null;
    setErrors(invalid?.errors ?? []);
    setFailed(!invalid?.errors);
    setBusy(false);
  }

  const errorOf = (section: Section, key: string) => errors.find((e) => e.section === section && e.key === key)?.message;
  // A changed field drops its error.
  const clear = (section: Section, key: string) => setErrors((all) => all.filter((e) => !(e.section === section && e.key === key)));
  // dropErrors drops every error a test picks (e.g. all Pengurus cards).
  const dropErrors = (test: (e: FieldError) => boolean) => setErrors((all) => all.filter((e) => !test(e)));
  return { errors, failed, busy, move, errorOf, clear, dropErrors };
}

// A free text field; maxLength mirrors the server limit (ERR-040). Person
// names have none, so ERR-013 shows as on Pre-Screening.
export function TextField({
  label,
  value,
  onChange,
  error,
  required = false,
  disabled = false,
  maxLength,
  area = false,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  error?: string;
  required?: boolean;
  disabled?: boolean;
  maxLength?: number;
  area?: boolean;
}) {
  const props = {
    "aria-label": label,
    className: `${inputClass(disabled, !!error)} ${area ? styles.textArea : ""}`,
    value,
    disabled,
    maxLength,
  };
  return (
    <Field label={label} required={required} error={error}>
      {area ? (
        <textarea {...props} onChange={(e) => onChange(e.target.value)} />
      ) : (
        <input {...props} onChange={(e) => onChange(e.target.value)} />
      )}
    </Field>
  );
}

// A BMV dropdown with its placeholder; the code is the value (BR-UI-012).
export function CodeSelect({
  label,
  placeholder,
  options,
  value,
  onChange,
  error,
  required = false,
  disabled = false,
  failedItem = false,
}: {
  label: string;
  placeholder: string;
  options: Option[];
  value: string;
  onChange: (code: string) => void;
  error?: string;
  required?: boolean;
  disabled?: boolean;
  failedItem?: boolean;
}) {
  return (
    <Field label={label} required={required} error={error}>
      <select aria-label={label} className={inputClass(disabled, !!error)} value={value} disabled={disabled} onChange={(e) => onChange(e.target.value)}>
        <option value="" disabled>
          {placeholder}
        </option>
        {/* ERR-038: a failed list shows one item that cannot be chosen. */}
        {failedItem && (
          <option value="__failed" disabled>
            {listFailedMessage}
          </option>
        )}
        {options.map((o) => (
          <option key={o.code} value={o.code}>
            {o.description}
          </option>
        ))}
      </select>
    </Field>
  );
}

// BMV radios (Jenis Kelamin, Perjanjian Pisah Harta), none chosen at first.
export function CodeRadios({
  label,
  options,
  value,
  onChange,
  error,
  disabled,
  failedItem = false,
}: {
  label: string;
  options: Option[];
  value: string;
  onChange: (code: string) => void;
  error?: string;
  disabled: boolean;
  failedItem?: boolean;
}) {
  return (
    <Field label={label} required error={error}>
      <div className={styles.radioRow} role="radiogroup" aria-label={label}>
        {failedItem && <span className={styles.notMarried}>{listFailedMessage}</span>}
        {options.map((o) => (
          <label key={o.code} className={styles.radioLabel}>
            <input type="radio" name={label} checked={value === o.code} disabled={disabled} onChange={() => onChange(o.code)} />
            {o.description}
          </label>
        ))}
      </div>
    </Field>
  );
}

// thousands puts a dot every three digits (BR-UI-009).
function thousands(digits: string): string {
  return digits.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
}

// A whole number typed with thousands dots, kept as digits: no minus, no
// decimals (BR-UI-009); "Rp" is a fixed prefix for money. A readOnly field
// cannot be typed in yet looks like an active one (AC-FR-APP-018-03).
export function NumberField({
  label,
  value,
  onChange,
  error,
  disabled,
  maxDigits,
  money = false,
  required = true,
  readOnly = false,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (digits: string) => void;
  error?: string;
  disabled: boolean;
  maxDigits: number;
  money?: boolean;
  required?: boolean;
  readOnly?: boolean;
  placeholder?: string;
}) {
  const input = (
    <input
      aria-label={label}
      inputMode="numeric"
      className={money ? styles.moneyInput : inputClass(disabled, !!error)}
      value={thousands(value)}
      disabled={disabled}
      readOnly={readOnly}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value.replace(/\D/g, "").slice(0, maxDigits))}
    />
  );
  return (
    <Field label={label} required={required} error={error}>
      {money ? (
        <span className={`${inputClass(disabled, !!error)} ${styles.money}`}>
          <span aria-hidden="true">Rp</span>
          {input}
        </span>
      ) : (
        input
      )}
    </Field>
  );
}

// A percentage (BR-UI-013): typed with digits and one comma, at most two
// decimals, shown "21,00" with a fixed "%" once the field is left; a margin
// of Data Permohonan Fasilitas has five (BR-UI-014, "12,00000"). value is
// the API form, with a dot ("21.00").
export function PercentField({
  label,
  value,
  onChange,
  error,
  disabled,
  decimals = 2,
  required = false,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  error?: string;
  disabled: boolean;
  decimals?: number;
  required?: boolean;
}) {
  const typed = value.replace(".", ",");
  const pattern = new RegExp(`^[0-9]*(,[0-9]{0,${decimals}})?$`);
  return (
    <Field label={label} required={required} error={error}>
      <span className={`${inputClass(disabled, !!error)} ${styles.money}`}>
        <input
          aria-label={label}
          inputMode="decimal"
          className={`${styles.moneyInput} ${styles.percentInput}`}
          value={typed}
          disabled={disabled}
          onChange={(e) => {
            const v = e.target.value.replace(/[^0-9,]/g, "");
            if (pattern.test(v)) {
              onChange(v.replace(",", "."));
            }
          }}
          onBlur={() => value !== "" && !Number.isNaN(Number(value)) && onChange(Number(value).toFixed(decimals))}
        />
        <span aria-hidden="true">%</span>
      </span>
    </Field>
  );
}

// "--" until a value exists (Dati II, Kode Pos).
export function ShownValue({ label, value }: { label: string; value: string }) {
  return (
    <div className={styles.field}>
      <span className={styles.fieldLabel}>{label}</span>
      <span aria-label={label}>{value || "--"}</span>
    </div>
  );
}

export type RegionOption = Option & { value1: string };
const regionKeys = ["provinceCode", "cityCode", "districtCode", "subdistrictCode"] as const;

// cascade sets one unit and resets every unit below it (FR-APP-005 rule 6).
function cascade<R extends Region>(r: R, key: (typeof regionKeys)[number], code: string): R {
  const next = { ...r, [key]: code };
  for (const below of regionKeys.slice(regionKeys.indexOf(key) + 1)) {
    next[below] = "";
  }
  return next;
}

// useRegion loads the units under each chosen one (FR-APP-005 rules 5 and
// 6) and returns the fields to place: a unit is disabled until the one above
// is chosen; Dati II is the Kota's Value1, Kode Pos the Kelurahan's.
export function useRegion({
  assignment,
  region,
  onChange,
  provinces,
  provincesFailed,
  section,
  flow,
  required,
  disabled,
}: {
  assignment: AssignmentRef;
  region: Region;
  onChange: (r: Region) => void;
  provinces: Option[];
  provincesFailed: boolean;
  section: Section;
  flow: Pick<ReturnType<typeof useMove>, "errorOf" | "clear">;
  required: boolean;
  disabled: boolean;
}) {
  const { endpointPath, caseId, assignmentId } = assignment;
  const source = assignmentURL({ endpointPath, caseId, assignmentId }, "regions");
  const cities = useList(source, "CITY", region.provinceCode);
  const districts = useList(source, "DISTRICT", region.cityCode);
  const subdistricts = useList(source, "SUBDISTRICT", region.districtCode);
  const postal = useList(source, "POSTALCODE", region.subdistrictCode);
  const regency = cities.options.find((c) => c.code === region.cityCode)?.value1 ?? "";
  const unit = (label: string, key: (typeof regionKeys)[number], errorKey: string, options: Option[], failed: boolean, above: string | null) => (
    <CodeSelect
      label={label}
      placeholder={`Pilih ${label}`}
      options={options}
      value={region[key]}
      onChange={(code) => (onChange(cascade(region, key, code)), flow.clear(section, errorKey))}
      error={flow.errorOf(section, errorKey)}
      required={required}
      disabled={disabled || above === ""}
      failedItem={failed}
    />
  );
  return {
    province: unit("Provinsi", "provinceCode", "province", provinces, provincesFailed, null),
    city: unit("Kota", "cityCode", "city", cities.options, cities.failed, region.provinceCode),
    district: unit("Kecamatan", "districtCode", "district", districts.options, districts.failed, region.cityCode),
    subdistrict: unit("Kelurahan", "subdistrictCode", "subdistrict", subdistricts.options, subdistricts.failed, region.districtCode),
    regency: <ShownValue label="Dati II" value={regency} />,
    postal: <ShownValue label="Kode Pos" value={postal.options[0]?.code ?? ""} />,
  };
}

// useList reads the units of a type under parent; nothing without a parent.
function useList(source: string, type: string, parent: string) {
  const [state, setState] = useState<{ key: string; options: RegionOption[]; failed: boolean }>({ key: "", options: [], failed: false });
  const key = `${type}/${parent}`;
  useEffect(() => {
    if (!parent) {
      return;
    }
    let live = true;
    apiFetch(`${source}?type=${type}&parent=${encodeURIComponent(parent)}`)
      .then((res) => (res.ok ? (res.json() as Promise<{ options: RegionOption[] }>) : Promise.reject()))
      .then((body) => live && setState({ key, options: body.options, failed: false }))
      .catch(() => live && setState({ key, options: [], failed: true }));
    return () => {
      live = false;
    };
  }, [source, type, parent, key]);
  return state.key === key && parent ? state : { options: [] as RegionOption[], failed: false };
}

// Two fields side by side within one column (e.g. RT and RW).
export function Pair({ children }: { children: ReactNode }) {
  return <div className={styles.pair}>{children}</div>;
}

// The region rows of Data Pasangan, Data Keluarga Dekat and Domisili:
// Provinsi, Kota | Kecamatan, Kelurahan; Dati II under Kota, Kode Pos under
// Kelurahan (Gambar 52 bottom, 54, 56).
export function RegionRows({ fields }: { fields: ReturnType<typeof useRegion> }) {
  return (
    <>
      <Pair>
        {fields.province}
        {fields.city}
      </Pair>
      <Pair>
        {fields.district}
        {fields.subdistrict}
      </Pair>
      <Pair>
        <span />
        {fields.regency}
      </Pair>
      <Pair>
        <span />
        {fields.postal}
      </Pair>
    </>
  );
}
