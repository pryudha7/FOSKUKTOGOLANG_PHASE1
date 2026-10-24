"use client";

import { useState } from "react";
import { assignmentURL, type AssignmentRef } from "@/lib/api";
import AssignmentHeader from "./AssignmentHeader";
import Autocomplete, { Highlighted } from "./Autocomplete";
import DateField from "./DateField";
import DropdownsFailedPopup from "./DropdownsFailedPopup";
import ErrorBanner from "./ErrorBanner";
import Field from "./Field";
import { unavailableMessage } from "./names";
import { CodeSelect, create, Pair, RegionRows, TextField, useMove, useRegion } from "./ProspectDataEntry";
import type { BusinessEntityForm, BusinessEntityPage, Officer, Option, Section } from "./types";
import styles from "./case.module.css";

// Data Pengurus holds 1 to 10 cards (FR-APP-002).
const maxOfficers = 10;
const emptyOfficer: Officer = { roleCode: "", nik: "", name: "", birthDate: "" };

const officerSection = (i: number) => `officer-${i + 1}` as Section;

// An autocomplete item in two lines: code, then the description (Gambar 50).
const twoLines = (o: Option, match: string) => (
  <span className={styles.twoLineOption}>
    <Highlighted text={o.code} match={match} />
    <span className={styles.optionNote}>
      <Highlighted text={o.description} match={match} />
    </span>
  </span>
);

// UI-025 Prospect Data Entry, Badan Usaha (Gambar 48-50, FR-APP-005): the
// entity and its Data Pengurus; Next ends Prospect Data Entry and returns
// to the Dashboard, Close leaves without saving (agreed comments 1-2).
export default function BusinessEntityData({ page, assignment }: { page: BusinessEntityPage; assignment: AssignmentRef }) {
  const readOnly = !page.editable;
  const [form, setForm] = useState<BusinessEntityForm>(page.values);
  const [sectorText, setSectorText] = useState(page.sector?.description ?? "");
  const [categoryText, setCategoryText] = useState(page.category?.description ?? "");
  const flow = useMove(assignment, "business-entity");
  const opts = page.options;
  const failed = (key: string) => page.failedOptions.includes(key);
  const err = (key: string) => flow.errorOf("form", key);
  const set = <K extends keyof BusinessEntityForm>(key: K, errorKey: string, v: BusinessEntityForm[K]) => {
    setForm((f) => ({ ...f, [key]: v }));
    flow.clear("form", errorKey);
  };
  const region = useRegion({
    assignment,
    region: form,
    onChange: (r) => setForm((f) => ({ ...f, ...r })),
    provinces: opts.province,
    provincesFailed: failed("province"),
    section: "form",
    flow,
    required: true,
    disabled: readOnly,
  });
  const suggestions = (type: string) => `${assignmentURL(assignment, "suggestions")}?type=${type}`;

  const setOfficer = (i: number, key: keyof Officer, errorKey: string, v: string) => {
    setForm((f) => ({ ...f, officers: f.officers.map((o, j) => (j === i ? { ...o, [key]: v } : o)) }));
    flow.clear(officerSection(i), errorKey);
  };
  // Tambah does nothing at 10 cards, delete nothing at 1 (FR-APP-002). The
  // cards' errors go with a change of cards: their numbers no longer hold.
  const dropOfficerErrors = () => flow.dropErrors((e) => e.section.startsWith("officer-"));
  const addOfficer = () => {
    setForm((f) => (f.officers.length >= maxOfficers ? f : { ...f, officers: [...f.officers, emptyOfficer] }));
    dropOfficerErrors();
  };
  const removeOfficer = (i: number) => {
    setForm((f) => (f.officers.length <= 1 ? f : { ...f, officers: f.officers.filter((_, j) => j !== i) }));
    dropOfficerErrors();
  };

  return (
    <div className={styles.assignmentArea}>
      <ErrorBanner errors={flow.errors} message={flow.failed ? unavailableMessage : undefined} />
      <DropdownsFailedPopup failed={page.failedOptions} />
      <section className={styles.card}>
        <AssignmentHeader title="Prospect Data Entry" />
        <h3 className={styles.sectionTitle}>Badan Usaha</h3>
        <div className={styles.formPanel}>
          <div className={styles.formGrid}>
            <Field label="Nomor NPWP">
              <input aria-label="Nomor NPWP" className={styles.inputDisabled} value={page.npwp} disabled readOnly />
            </Field>
            <div className={styles.customerStatus} role="status">
              <span aria-hidden="true" className={styles.customerStatusIcon}>
                ▲
              </span>
              {page.existingCustomer ? "Nasabah Eksisting" : "Nasabah Baru"}
            </div>
            <Field label="Nama Badan Usaha">
              <input aria-label="Nama Badan Usaha" className={styles.inputDisabled} value={page.name} disabled readOnly />
            </Field>
            <CodeSelect label="Kode Badan Usaha" placeholder="Pilih Kode Badan Usaha" options={opts.businessEntity} failedItem={failed("businessEntity")} value={form.businessEntityCode} onChange={(v) => set("businessEntityCode", "businessEntity", v)} error={err("businessEntity")} disabled={readOnly} />
            <TextField label="Tempat Pendirian" required maxLength={50} value={form.placeOfEstablishment} onChange={(v) => set("placeOfEstablishment", "placeOfEstablishment", v)} error={err("placeOfEstablishment")} disabled={readOnly} />
            <TextField label="Nomor Akta Pendirian" required maxLength={50} value={form.deedNumber} onChange={(v) => set("deedNumber", "deedNumber", v)} error={err("deedNumber")} disabled={readOnly} />
            <Field label="Tanggal Akta Pendirian">
              <DateField ariaLabel="Tanggal Akta Pendirian" value={page.establishmentDate} today={page.today} disabled onChange={() => undefined} />
            </Field>
            <TextField label="Nomor Akta Perubahan Terakhir" maxLength={50} value={form.lastDeedNumber} onChange={(v) => set("lastDeedNumber", "lastDeedNumber", v)} error={err("lastDeedNumber")} disabled={readOnly} />
            <Field label="Tanggal Akta Perubahan Terakhir" error={err("lastDeedDate")}>
              {/* Before today only: the picker ends yesterday (FR-APP-005 rule 7). */}
              <DateField ariaLabel="Tanggal Akta Perubahan Terakhir" value={form.lastDeedDate} invalid={!!err("lastDeedDate")} disabled={readOnly} max={page.latestDeedDate} today={page.today} onChange={(v) => set("lastDeedDate", "lastDeedDate", v)} />
            </Field>
            <TextField label="Nomor Telepon" required maxLength={20} value={form.phone} onChange={(v) => set("phone", "phone", v)} error={err("phone")} disabled={readOnly} />
            <TextField label="Nomor HP" maxLength={20} value={form.mobilePhone} onChange={(v) => set("mobilePhone", "mobilePhone", v)} error={err("mobilePhone")} disabled={readOnly} />
            <TextField label="Alamat Email" maxLength={254} value={form.email} onChange={(v) => set("email", "email", v)} error={err("email")} disabled={readOnly} />
            <Field label="Kode Bidang Usaha" required error={err("economicSector")}>
              <span className={styles.cornerMark}>
                <Autocomplete
                  label="Kode Bidang Usaha"
                  source={suggestions("ECONOMIC_SECTOR")}
                  text={sectorText}
                  // Typed text is sent as it is; the server refuses what is not a code (ERR-037).
                  onType={(v) => (setSectorText(v), set("economicSectorCode", "economicSector", v))}
                  onChoose={(o) => (setSectorText(o.description), set("economicSectorCode", "economicSector", o.code))}
                  renderOption={twoLines}
                  invalid={!!err("economicSector")}
                  disabled={readOnly}
                />
              </span>
            </Field>
            <CodeSelect label="Hubungan dengan Bank" placeholder="Pilih Hubungan dengan Bank" options={opts.relationshipWithBank} failedItem={failed("relationshipWithBank")} value={form.relationshipWithBankCode} onChange={(v) => set("relationshipWithBankCode", "relationshipWithBank", v)} error={err("relationshipWithBank")} disabled={readOnly} />
            {/* Tipe Bisnis has no placeholder (UI-025). */}
            <CodeSelect label="Tipe Bisnis" placeholder="" required options={opts.businessType} failedItem={failed("businessType")} value={form.businessTypeCode} onChange={(v) => set("businessTypeCode", "businessType", v)} error={err("businessType")} disabled={readOnly} />
            <Field label="Golongan Nasabah" required error={err("customerCategory")}>
              <span className={styles.cornerMark}>
                <Autocomplete
                  label="Golongan Nasabah"
                  source={suggestions("BUSINESSENTITY_CATEGORY")}
                  text={categoryText}
                  onType={(v) => (setCategoryText(v), set("customerCategoryCode", "customerCategory", v))}
                  onChoose={(o) => (setCategoryText(o.description), set("customerCategoryCode", "customerCategory", o.code))}
                  renderOption={twoLines}
                  invalid={!!err("customerCategory")}
                  disabled={readOnly}
                />
              </span>
            </Field>
            <TextField label="Alamat" required maxLength={350} value={form.address} onChange={(v) => set("address", "address", v)} error={err("address")} disabled={readOnly} />
            <CodeSelect label="Go Public" placeholder="Pilih Go Public" required options={opts.goPublic} failedItem={failed("goPublic")} value={form.goPublic} onChange={(v) => set("goPublic", "goPublic", v)} error={err("goPublic")} disabled={readOnly} />
            <RegionRows fields={region} />
            <div className={styles.wide}>
              <TextField label="Nama Group Usaha Nasabah" maxLength={100} value={form.businessGroupName} onChange={(v) => set("businessGroupName", "businessGroupName", v)} error={err("businessGroupName")} disabled={readOnly} />
            </div>
          </div>
        </div>

        <div className={styles.officersHeader}>
          <h3 className={styles.sectionTitle}>Data Pengurus</h3>
          {!readOnly && (
            <button type="button" className={styles.addOfficer} onClick={addOfficer}>
              <span aria-hidden="true">+</span> Tambah
            </button>
          )}
        </div>
        {form.officers.map((o, i) => {
          const section = officerSection(i);
          const oErr = (key: string) => flow.errorOf(section, key);
          return (
            <div key={i} className={`${styles.formPanel} ${styles.officerCard}`} aria-label={`Pengurus ${i + 1}`} role="group">
              {!readOnly && (
                <button type="button" className={styles.removeOfficer} aria-label={`Hapus Pengurus ${i + 1}`} onClick={() => removeOfficer(i)}>
                  <svg width="12" height="14" viewBox="0 0 12 14" aria-hidden="true">
                    <path d="M1 3h10M4 3V1h4v2M2 3l1 10h6l1-10" fill="currentColor" stroke="currentColor" strokeWidth="1" />
                  </svg>
                </button>
              )}
              <div className={styles.officerRole}>
                <CodeSelect label="Jabatan" placeholder="Pilih Jabatan" required options={opts.businessRole} failedItem={failed("businessRole")} value={o.roleCode} onChange={(v) => setOfficer(i, "roleCode", "role", v)} error={oErr("role")} disabled={readOnly} />
              </div>
              <TextField label="NIK" required maxLength={16} value={o.nik} onChange={(v) => setOfficer(i, "nik", "nik", v)} error={oErr("nik")} disabled={readOnly} />
              <TextField label="Nama" required maxLength={30} value={o.name} onChange={(v) => setOfficer(i, "name", "name", v)} error={oErr("name")} disabled={readOnly} />
              <Field label="Tanggal Lahir" required error={oErr("birthDate")}>
                <DateField
                  ariaLabel="Tanggal Lahir"
                  value={o.birthDate}
                  invalid={!!oErr("birthDate")}
                  disabled={readOnly}
                  min={page.earliestBirthDate}
                  max={page.today}
                  today={page.today}
                  onChange={(v) => setOfficer(i, "birthDate", "birthDate", v)}
                />
              </Field>
            </div>
          );
        })}

        <footer className={styles.buttons}>
          <button type="button" className={styles.close} onClick={() => window.location.assign(assignment.endpointPath)}>
            Close
          </button>
          {!readOnly && (
            <button type="button" className={styles.next} disabled={flow.busy} onClick={() => flow.move(create, form)}>
              Next
            </button>
          )}
        </footer>
      </section>
    </div>
  );
}
