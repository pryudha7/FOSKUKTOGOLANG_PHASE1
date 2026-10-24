"use client";

import { useEffect, useState, type ReactNode } from "react";
import { apiFetch, assignmentPath, assignmentURL, type AssignmentRef } from "@/lib/api";
import AssignmentHeader from "./AssignmentHeader";
import Autocomplete, { Highlighted } from "./Autocomplete";
import { inputClass, listFailedMessage } from "./fields";
import Breadcrumb, { prospectSteps } from "./Breadcrumb";
import DateField from "./DateField";
import ErrorBanner from "./ErrorBanner";
import Field from "./Field";
import DropdownsFailedPopup from "./DropdownsFailedPopup";
import { unavailableMessage } from "./names";
import type {
  AddressForm,
  CloseRelativeForm,
  CloseRelativePage,
  FieldError,
  Option,
  IndividualDataForm,
  IndividualDataPage,
  Region,
  Section,
  SpouseForm,
  SpousePage,
} from "./types";
import styles from "./case.module.css";

// The endpoint of each form and what moves it: a step name, or Create.
type FormPath = "individual-data" | "spouse" | "close-relative" | "business-entity";
export const create = "create";

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

// The card around each step: header, the three-step tabbed flow, the
// banner, the body and the buttons.
function ProspectCard({
  step,
  move,
  children,
  buttons,
}: {
  step: number;
  move: ReturnType<typeof useMove> & { body: () => unknown; editable: boolean; failedOptions: string[] };
  children: ReactNode;
  buttons: ReactNode;
}) {
  return (
    <div className={styles.assignmentArea}>
      <ErrorBanner errors={move.errors} message={move.failed ? unavailableMessage : undefined} />
      <DropdownsFailedPopup failed={move.failedOptions} />
      <section className={styles.card}>
        <AssignmentHeader title="Prospect Data Entry" />
        <Breadcrumb steps={prospectSteps} active={step} progress onSelect={(i) => move.editable && move.move(prospectSteps[i], move.body())} />
        {children}
        <footer className={styles.buttons}>{buttons}</footer>
      </section>
    </div>
  );
}

// A free text field; maxLength mirrors the server limit (ERR-040).
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
function CodeRadios({
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
// decimals (BR-UI-009); "Rp" is a fixed prefix for money.
function NumberField({
  label,
  value,
  onChange,
  error,
  disabled,
  maxDigits,
  money = false,
}: {
  label: string;
  value: string;
  onChange: (digits: string) => void;
  error?: string;
  disabled: boolean;
  maxDigits: number;
  money?: boolean;
}) {
  const input = (
    <input
      aria-label={label}
      inputMode="numeric"
      className={money ? styles.moneyInput : inputClass(disabled, !!error)}
      value={thousands(value)}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value.replace(/\D/g, "").slice(0, maxDigits))}
    />
  );
  return (
    <Field label={label} required error={error}>
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

// "--" until a value exists (Dati II, Kode Pos).
export function ShownValue({ label, value }: { label: string; value: string }) {
  return (
    <div className={styles.field}>
      <span className={styles.fieldLabel}>{label}</span>
      <span aria-label={label}>{value || "--"}</span>
    </div>
  );
}

type RegionOption = Option & { value1: string };
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

// The Data Pribadi address block of e-KTP (Gambar 52 top).
function EKTPBlock({
  value,
  onChange,
  regionFields,
  errorOf,
  options,
  failed,
  disabled,
}: {
  value: AddressForm;
  onChange: (key: keyof AddressForm, v: string) => void;
  regionFields: ReturnType<typeof useRegion>;
  errorOf: (key: string) => string | undefined;
  options: Record<string, Option[]>;
  failed: (key: string) => boolean;
  disabled: boolean;
}) {
  return (
    <>
      <TextField label="Alamat Rumah Tinggal (Sesuai e-KTP)" required maxLength={350} value={value.address} onChange={(v) => onChange("address", v)} error={errorOf("address")} disabled={disabled} />
      <TextField label="Nomor Handphone" required maxLength={20} value={value.mobilePhone} onChange={(v) => onChange("mobilePhone", v)} error={errorOf("mobilePhone")} disabled={disabled} />
      <TextField label="Nomor Telepon" required maxLength={20} value={value.phone} onChange={(v) => onChange("phone", v)} error={errorOf("phone")} disabled={disabled} />
      <Pair>
        <TextField label="RT" required maxLength={3} value={value.rt} onChange={(v) => onChange("rt", v)} error={errorOf("rt")} disabled={disabled} />
        <TextField label="RW" required maxLength={3} value={value.rw} onChange={(v) => onChange("rw", v)} error={errorOf("rw")} disabled={disabled} />
      </Pair>
      <NumberField label="Lama Tinggal (Tahun)" maxDigits={2} value={value.yearsOfStay} onChange={(v) => onChange("yearsOfStay", v)} error={errorOf("yearsOfStay")} disabled={disabled} />
      <NumberField label="Lama Tinggal (Bulan)" maxDigits={2} value={value.monthsOfStay} onChange={(v) => onChange("monthsOfStay", v)} error={errorOf("monthsOfStay")} disabled={disabled} />
      <CodeSelect label="Status Kepemilikan Rumah" placeholder="Pilih Status Kepemilikan Rumah" required options={options.houseOwnership} failedItem={failed("houseOwnership")} value={value.houseOwnershipCode} onChange={(v) => onChange("houseOwnershipCode", v)} error={errorOf("houseOwnership")} disabled={disabled} />
      <Pair>
        {regionFields.district}
        {regionFields.subdistrict}
      </Pair>
      <Pair>
        {regionFields.province}
        {regionFields.city}
      </Pair>
      <Pair>
        <span />
        {regionFields.postal}
      </Pair>
      <Pair>
        <span />
        {regionFields.regency}
      </Pair>
      <span />
    </>
  );
}

// UI-026 Data Pribadi (Gambar 51-53, FR-APP-012).
export function IndividualData({ page, assignment }: { page: IndividualDataPage; assignment: AssignmentRef }) {
  const readOnly = !page.editable;
  const failed = (key: string) => page.failedOptions.includes(key);
  const [form, setForm] = useState<IndividualDataForm>(page.values);
  const [sectorText, setSectorText] = useState(page.sector?.description ?? "");
  const flow = useMove(assignment, "individual-data");
  const opts = page.options;

  const set = <K extends keyof IndividualDataForm>(key: K, errorKey: string, v: IndividualDataForm[K]) => {
    setForm((f) => ({ ...f, [key]: v }));
    flow.clear("form", errorKey);
  };
  const setAddress = (block: "ektp" | "domicile") => (key: keyof AddressForm, v: string) => {
    setForm((f) => ({ ...f, [block]: { ...f[block], [key]: v } }));
    flow.clear(block === "ektp" ? "ektp" : "domicile", key.replace(/Code$/, ""));
  };
  const setRegion = (block: "ektp" | "domicile") => (r: Region) => setForm((f) => ({ ...f, [block]: { ...f[block], ...r } }));
  const ektpRegion = useRegion({ assignment, region: form.ektp, onChange: setRegion("ektp"), provinces: opts.province, provincesFailed: failed("province"), section: "ektp", flow, required: true, disabled: readOnly });
  const domicileRegion = useRegion({ assignment, region: form.domicile, onChange: setRegion("domicile"), provinces: opts.province, provincesFailed: failed("province"), section: "domicile", flow, required: true, disabled: readOnly });
  const err = (key: string) => flow.errorOf("form", key);
  const domErr = (key: string) => flow.errorOf("domicile", key);
  const dom = setAddress("domicile");

  // Business Rule 6: unchecking copies e-KTP into Domisili; Alamat Surat
  // Menyurat is left as it is either way.
  function toggleSame(same: boolean) {
    setForm((f) => ({ ...f, sameAsEktp: same, domicile: same ? f.domicile : { ...f.ektp } }));
  }

  const card = { ...flow, body: () => form, editable: page.editable };
  return (
    <ProspectCard
      step={0}
      move={{ ...card, failedOptions: page.failedOptions }}
      buttons={
        <>
          {/* Close leaves without saving (agreed comment 4). */}
          <button type="button" className={styles.close} onClick={() => window.location.assign(assignment.endpointPath)}>
            Close
          </button>
          {!readOnly && (
            <button type="button" className={styles.next} disabled={flow.busy} onClick={() => flow.move(prospectSteps[1], form)}>
              Next
            </button>
          )}
        </>
      }
    >
      <h3 className={styles.sectionTitle}>Data Pribadi</h3>
      <div className={styles.formPanel}>
        <div className={styles.formGrid}>
          <Field label="Nomor e-KTP" required>
            <input aria-label="Nomor e-KTP" className={styles.inputDisabled} value={page.nik} disabled readOnly />
          </Field>
          <div className={styles.customerStatus} role="status">
            <span aria-hidden="true" className={styles.customerStatusIcon}>
              ▲
            </span>
            {page.existingCustomer ? "Nasabah Eksisting" : "Nasabah Baru"}
          </div>
          <Field label="Nama Lengkap Nasabah (Sesuai e-KTP)" required>
            <input aria-label="Nama Lengkap Nasabah (Sesuai e-KTP)" className={styles.inputDisabled} value={page.name} disabled readOnly />
          </Field>
          <Pair>
            <Field label="Berlaku Sampai" required error={err("idLifetime")}>
              <label className={styles.checkboxLabel}>
                <input type="checkbox" checked={form.idLifetime} disabled={readOnly} onChange={(e) => set("idLifetime", "idLifetime", e.target.checked)} />
                Seumur Hidup
              </label>
            </Field>
            <TextField label="Kewarganegaraan" required maxLength={50} value={form.nationality} onChange={(v) => set("nationality", "nationality", v)} error={err("nationality")} disabled={readOnly} />
          </Pair>
          <TextField label="Alamat Email" maxLength={254} value={form.email} onChange={(v) => set("email", "email", v)} error={err("email")} disabled={readOnly} />
          <TextField label="Nomor NPWP" required maxLength={22} value={form.npwp} onChange={(v) => set("npwp", "npwp", v)} error={err("npwp")} disabled={readOnly} />
          <CodeSelect label="Pekerjaan" placeholder="Pilih Pekerjaan" required options={opts.job} failedItem={failed("job")} value={form.jobCode} onChange={(v) => set("jobCode", "job", v)} error={err("job")} disabled={readOnly} />
          <TextField label="Tempat Bekerja" required maxLength={100} value={form.workplace} onChange={(v) => set("workplace", "workplace", v)} error={err("workplace")} disabled={readOnly} />
          <div className={styles.stack}>
            <CodeRadios label="Jenis Kelamin" options={opts.gender} failedItem={failed("gender")} value={form.genderCode} onChange={(v) => set("genderCode", "gender", v)} error={err("gender")} disabled={readOnly} />
            <NumberField label="Penghasilan Kotor Pertahun" money maxDigits={15} value={form.grossIncome} onChange={(v) => set("grossIncome", "grossIncome", v)} error={err("grossIncome")} disabled={readOnly} />
            <CodeSelect label="Sumber Penghasilan" placeholder="Pilih Sumber Penghasilan" required options={opts.sourceIncome} failedItem={failed("sourceIncome")} value={form.incomeSourceCode} onChange={(v) => set("incomeSourceCode", "incomeSource", v)} error={err("incomeSource")} disabled={readOnly} />
            <CodeSelect label="Hubungan dengan Bank" placeholder="Pilih Hubungan dengan Bank" required options={opts.relationshipWithBank} failedItem={failed("relationshipWithBank")} value={form.relationshipWithBankCode} onChange={(v) => set("relationshipWithBankCode", "relationshipWithBank", v)} error={err("relationshipWithBank")} disabled={readOnly} />
            <CodeSelect label="Golongan Nasabah" placeholder="Pilih Golongan Nasabah" required options={opts.customerCategory} failedItem={failed("customerCategory")} value={form.customerCategoryCode} onChange={(v) => set("customerCategoryCode", "customerCategory", v)} error={err("customerCategory")} disabled={readOnly} />
          </div>
          <div>
            <TextField label="Alamat Tempat Kerja" required area maxLength={350} value={form.workplaceAddress} onChange={(v) => set("workplaceAddress", "workplaceAddress", v)} error={err("workplaceAddress")} disabled={readOnly} />
          </div>
          <Field label="Bidang Usaha Tempat Bekerja" required error={err("workplaceSector")}>
            <Autocomplete
              label="Bidang Usaha Tempat Bekerja"
              source={`${assignmentURL(assignment, "suggestions")}?type=ECONOMIC_SECTOR`}
              text={sectorText}
              // Typed text is sent as it is; the server refuses what is not a code (ERR-037).
              onType={(v) => (setSectorText(v), set("workplaceSectorCode", "workplaceSector", v))}
              onChoose={(o) => (setSectorText(o.description), set("workplaceSectorCode", "workplaceSector", o.code))}
              renderOption={(o, match) => (
                <span className={styles.twoLineOption}>
                  <Highlighted text={o.code} match={match} />
                  <span>
                    <Highlighted text={o.description} match={match} />
                  </span>
                </span>
              )}
              invalid={!!err("workplaceSector")}
              disabled={readOnly}
            />
          </Field>
          <Pair>
            <Field label="Tanggal Lahir" required>
              <DateField ariaLabel="Tanggal Lahir" value={page.birthDate} today={page.birthDate} disabled onChange={() => undefined} />
            </Field>
            <TextField label="Tempat Lahir" required maxLength={50} value={form.birthPlace} onChange={(v) => set("birthPlace", "birthPlace", v)} error={err("birthPlace")} disabled={readOnly} />
          </Pair>
          <CodeSelect label="Tipe Bisnis Tempat Bekerja" placeholder="Pilih Tipe Bisnis Tempat Bekerja" required options={opts.businessType} failedItem={failed("businessType")} value={form.workplaceBusinessTypeCode} onChange={(v) => set("workplaceBusinessTypeCode", "workplaceBusinessType", v)} error={err("workplaceBusinessType")} disabled={readOnly} />
          <span />
        </div>

        <div className={styles.formGrid}>
          <EKTPBlock value={form.ektp} onChange={setAddress("ektp")} regionFields={ektpRegion} errorOf={(k) => flow.errorOf("ektp", k)} options={opts} failed={failed} disabled={readOnly} />
        </div>

        <label className={styles.checkboxLabel}>
          <input type="checkbox" checked={form.sameAsEktp} disabled={readOnly} onChange={(e) => toggleSame(e.target.checked)} />
          Alamat Rumah Tinggal sama dengan e-KTP
        </label>

        {!form.sameAsEktp && (
          <div className={styles.formGrid}>
            <TextField label="Alamat Rumah Tinggal Sekarang (Jika berbeda dengan e-KTP)" required maxLength={350} value={form.domicile.address} onChange={(v) => dom("address", v)} error={domErr("address")} disabled={readOnly} />
            <TextField label="Nomor Handphone" required maxLength={20} value={form.domicile.mobilePhone} onChange={(v) => dom("mobilePhone", v)} error={domErr("mobilePhone")} disabled={readOnly} />
            <TextField label="Nomor Telepon" required maxLength={20} value={form.domicile.phone} onChange={(v) => dom("phone", v)} error={domErr("phone")} disabled={readOnly} />
            <NumberField label="Lama Tinggal (Bulan)" maxDigits={2} value={form.domicile.monthsOfStay} onChange={(v) => dom("monthsOfStay", v)} error={domErr("monthsOfStay")} disabled={readOnly} />
            <NumberField label="Lama Tinggal (Tahun)" maxDigits={2} value={form.domicile.yearsOfStay} onChange={(v) => dom("yearsOfStay", v)} error={domErr("yearsOfStay")} disabled={readOnly} />
            <CodeSelect label="Alamat Surat Menyurat" placeholder="Pilih Alamat" required options={opts.mailingAddress} failedItem={failed("mailingAddress")} value={form.mailingAddressCode} onChange={(v) => (setForm((f) => ({ ...f, mailingAddressCode: v })), flow.clear("domicile", "mailingAddress"))} error={domErr("mailingAddress")} disabled={readOnly} />
            <CodeSelect label="Status Kepemilikan Rumah" placeholder="Pilih Status Kepemilikan Rumah" required options={opts.houseOwnership} failedItem={failed("houseOwnership")} value={form.domicile.houseOwnershipCode} onChange={(v) => dom("houseOwnershipCode", v)} error={domErr("houseOwnership")} disabled={readOnly} />
            <Pair>
              <TextField label="RT" required maxLength={3} value={form.domicile.rt} onChange={(v) => dom("rt", v)} error={domErr("rt")} disabled={readOnly} />
              <TextField label="RW" required maxLength={3} value={form.domicile.rw} onChange={(v) => dom("rw", v)} error={domErr("rw")} disabled={readOnly} />
            </Pair>
            <RegionRows fields={domicileRegion} />
          </div>
        )}

        <div className={styles.formGrid}>
          <CodeSelect label="Pendidikan Terakhir" placeholder="Pilih Pendidikan Terakhir" required options={opts.education} failedItem={failed("education")} value={form.educationCode} onChange={(v) => set("educationCode", "education", v)} error={err("education")} disabled={readOnly} />
          <CodeSelect label="Status Perkawinan" placeholder="Pilih Status Perkawinan" required options={opts.maritalStatus} failedItem={failed("maritalStatus")} value={form.maritalStatusCode} onChange={(v) => set("maritalStatusCode", "maritalStatus", v)} error={err("maritalStatus")} disabled={readOnly} />
          <TextField label="Nama Gadis Ibu Kandung" required maxLength={30} value={form.motherMaidenName} onChange={(v) => set("motherMaidenName", "motherMaidenName", v)} error={err("motherMaidenName")} disabled={readOnly} />
          <CodeSelect label="Jumlah Tanggungan" placeholder="Pilih Jumlah Tanggungan" required options={opts.dependents} failedItem={failed("dependents")} value={form.dependentsCode} onChange={(v) => set("dependentsCode", "dependents", v)} error={err("dependents")} disabled={readOnly} />
        </div>
      </div>
    </ProspectCard>
  );
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

// UI-027 Data Pasangan (Gambar 54, 55, FR-APP-013): the form only for a
// married customer; otherwise the status alone, and Next saves nothing.
export function SpouseData({ page, assignment }: { page: SpousePage; assignment: AssignmentRef }) {
  const readOnly = !page.editable;
  const failed = (key: string) => page.failedOptions.includes(key);
  const [form, setForm] = useState<SpouseForm>(page.values);
  const flow = useMove(assignment, "spouse");
  const opts = page.options;
  const set = <K extends keyof SpouseForm>(key: K, errorKey: string, v: SpouseForm[K]) => {
    setForm((f) => ({ ...f, [key]: v }));
    flow.clear("form", errorKey);
  };
  const region = useRegion({ assignment, region: form, onChange: (r) => setForm((f) => ({ ...f, ...r })), provinces: opts.province, provincesFailed: failed("province"), section: "form", flow, required: false, disabled: readOnly });
  const err = (key: string) => flow.errorOf("form", key);
  const body = () => (page.married ? form : {});

  return (
    <ProspectCard
      step={1}
      move={{ ...flow, body, editable: page.editable, failedOptions: page.failedOptions }}
      buttons={
        !readOnly && (
          <>
            <button type="button" className={styles.back} disabled={flow.busy} onClick={() => flow.move(prospectSteps[0], body())}>
              Back
            </button>
            <button type="button" className={styles.next} disabled={flow.busy} onClick={() => flow.move(prospectSteps[2], body())}>
              Next
            </button>
          </>
        )
      }
    >
      {!page.married ? (
        <p className={styles.notMarried}>Status Nasabah Tidak Kawin</p>
      ) : (
        <>
          <h3 className={styles.sectionTitle}>Data Pasangan</h3>
          <div className={styles.formPanel}>
            <div className={styles.formGrid}>
              <div className={styles.wide}>
                <TextField label="Nama Pasangan (Sesuai e-KTP)" required maxLength={30} value={form.name} onChange={(v) => set("name", "name", v)} error={err("name")} disabled={readOnly} />
              </div>
              <TextField label="No. e-KTP" required maxLength={16} value={form.nik} onChange={(v) => set("nik", "nik", v)} error={err("nik")} disabled={readOnly} />
              <Pair>
                <Field label="Tanggal Lahir" required error={err("birthDate")}>
                  <DateField
                    ariaLabel="Tanggal Lahir"
                    value={form.birthDate}
                    invalid={!!err("birthDate")}
                    disabled={readOnly}
                    min={page.earliestBirthDate}
                    max={page.today}
                    today={page.today}
                    onChange={(v) => set("birthDate", "birthDate", v)}
                  />
                </Field>
                <TextField label="Tempat Lahir" required maxLength={50} value={form.birthPlace} onChange={(v) => set("birthPlace", "birthPlace", v)} error={err("birthPlace")} disabled={readOnly} />
              </Pair>
              <CodeSelect label="Pekerjaan" placeholder="Pilih Pekerjaan" required options={opts.job} failedItem={failed("job")} value={form.jobCode} onChange={(v) => set("jobCode", "job", v)} error={err("job")} disabled={readOnly} />
              <TextField label="Lama Bekerja" maxLength={50} value={form.workDuration} onChange={(v) => set("workDuration", "workDuration", v)} error={err("workDuration")} disabled={readOnly} />
              <div className={styles.wide}>
                <TextField label="Alamat Kantor" maxLength={350} value={form.officeAddress} onChange={(v) => set("officeAddress", "officeAddress", v)} error={err("officeAddress")} disabled={readOnly} />
              </div>
              <TextField label="Nomor Telepon" maxLength={20} value={form.phone} onChange={(v) => set("phone", "phone", v)} error={err("phone")} disabled={readOnly} />
              <TextField label="Nomor Handphone" required maxLength={20} value={form.mobilePhone} onChange={(v) => set("mobilePhone", "mobilePhone", v)} error={err("mobilePhone")} disabled={readOnly} />
              <RegionRows fields={region} />
              <CodeSelect label="Pendidikan Terakhir" placeholder="Pilih Pendidikan Terakhir" options={opts.education} failedItem={failed("education")} value={form.educationCode} onChange={(v) => set("educationCode", "education", v)} error={err("education")} disabled={readOnly} />
              <CodeRadios label="Perjanjian Pisah Harta" options={opts.prenup} failedItem={failed("prenup")} value={form.prenupCode} onChange={(v) => set("prenupCode", "prenup", v)} error={err("prenup")} disabled={readOnly} />
            </div>
          </div>
        </>
      )}
    </ProspectCard>
  );
}

// UI-028 Data Keluarga Dekat (Gambar 56, FR-APP-014): Create ends Prospect
// Data Entry.
export function CloseRelativeData({ page, assignment }: { page: CloseRelativePage; assignment: AssignmentRef }) {
  const readOnly = !page.editable;
  const failed = (key: string) => page.failedOptions.includes(key);
  const [form, setForm] = useState<CloseRelativeForm>(page.values);
  const flow = useMove(assignment, "close-relative");
  const opts = page.options;
  const set = <K extends keyof CloseRelativeForm>(key: K, errorKey: string, v: CloseRelativeForm[K]) => {
    setForm((f) => ({ ...f, [key]: v }));
    flow.clear("form", errorKey);
  };
  const region = useRegion({ assignment, region: form, onChange: (r) => setForm((f) => ({ ...f, ...r })), provinces: opts.province, provincesFailed: failed("province"), section: "form", flow, required: true, disabled: readOnly });
  const err = (key: string) => flow.errorOf("form", key);

  return (
    <ProspectCard
      step={2}
      move={{ ...flow, body: () => form, editable: page.editable, failedOptions: page.failedOptions }}
      buttons={
        !readOnly && (
          <>
            <button type="button" className={styles.back} disabled={flow.busy} onClick={() => flow.move(prospectSteps[1], form)}>
              Back
            </button>
            <button type="button" className={styles.next} disabled={flow.busy} onClick={() => flow.move(create, form)}>
              Create
            </button>
          </>
        )
      }
    >
      <h3 className={styles.sectionTitle}>Data Keluarga Dekat (Yang Tidak Tinggal Serumah)</h3>
      <div className={styles.formPanel}>
        <div className={styles.formGrid}>
          <TextField label="Nama Lengkap (Sesuai e-KTP)" required maxLength={30} value={form.name} onChange={(v) => set("name", "name", v)} error={err("name")} disabled={readOnly} />
          <CodeSelect label="Hubungan Keluarga" placeholder="Pilih Hubungan Keluarga" required options={opts.relationship} failedItem={failed("relationship")} value={form.relationshipCode} onChange={(v) => set("relationshipCode", "relationship", v)} error={err("relationship")} disabled={readOnly} />
          <TextField label="Alamat Tempat Tinggal Saat Ini" required maxLength={350} value={form.address} onChange={(v) => set("address", "address", v)} error={err("address")} disabled={readOnly} />
          <Pair>
            <TextField label="No. Telepon Rumah" maxLength={20} value={form.homePhone} onChange={(v) => set("homePhone", "homePhone", v)} error={err("homePhone")} disabled={readOnly} />
            <TextField label="No. Telepon Kantor" maxLength={20} value={form.officePhone} onChange={(v) => set("officePhone", "officePhone", v)} error={err("officePhone")} disabled={readOnly} />
          </Pair>
          <RegionRows fields={region} />
          <TextField label="No. HP" required maxLength={20} value={form.mobilePhone} onChange={(v) => set("mobilePhone", "mobilePhone", v)} error={err("mobilePhone")} disabled={readOnly} />
          <span />
          <CodeSelect label="Pendidikan Terakhir" placeholder="Pilih Pendidikan Terakhir" required options={opts.education} failedItem={failed("education")} value={form.educationCode} onChange={(v) => set("educationCode", "education", v)} error={err("education")} disabled={readOnly} />
        </div>
      </div>
    </ProspectCard>
  );
}
