"use client";

import { useEffect, useState, type ReactNode } from "react";
import { apiFetch, assignmentURL, type AssignmentRef } from "@/lib/api";
import AssignmentHeader from "./AssignmentHeader";
import Breadcrumb, { hotProspectSteps } from "./Breadcrumb";
import DropdownsFailedPopup from "./DropdownsFailedPopup";
import ErrorBanner from "./ErrorBanner";
import { CodeSelect, NumberField, PercentField, RegionRows, save, TextField, useMove, useRegion } from "./FormFields";
import { unavailableMessage } from "./names";
import {
  otherFinancingSection,
  sectionNumber,
  type BusinessDataForm,
  type BusinessDataPage,
  type OtherFinancingCard,
  type OtherFinancingPage,
  type SLIKFacilityGroup,
} from "./types";
import styles from "./case.module.css";

// The card of a Hot Prospect step (UI-030, UI-031): the assignment name as
// title (agreed comment 6), the six-step tabbed flow, the banner, the body
// and the buttons. A breadcrumb step saves and opens it (agreed comment 5).
export function HotProspectCard({
  title,
  step,
  flow,
  body,
  editable,
  failedOptions = [],
  children,
  buttons,
}: {
  title: string;
  step: number;
  flow: ReturnType<typeof useMove>;
  body: () => unknown;
  editable: boolean;
  failedOptions?: string[];
  children: ReactNode;
  buttons: ReactNode;
}) {
  return (
    <div className={styles.assignmentArea}>
      <ErrorBanner errors={flow.errors} message={flow.failed ? unavailableMessage : undefined} />
      <DropdownsFailedPopup failed={failedOptions} />
      <section className={styles.card}>
        <AssignmentHeader title={title} />
        <Breadcrumb steps={hotProspectSteps} active={step} progress onSelect={(i) => editable && flow.move(hotProspectSteps[i], body())} />
        {children}
        <footer className={styles.buttons}>{buttons}</footer>
      </section>
    </div>
  );
}

// Close (or Back, in its place) returns to the Dashboard without saving
// (agreed comment 4).
export function leave(assignment: AssignmentRef) {
  window.location.assign(assignment.endpointPath);
}

// UI-030 Data Usaha Nasabah (Gambar 71, FR-APP-016). Back to Previous Step
// shows but is disabled: this is the first Hot Prospect step (agreed
// comment 3).
export function BusinessData({ page, assignment }: { page: BusinessDataPage; assignment: AssignmentRef }) {
  const readOnly = !page.editable;
  const [form, setForm] = useState<BusinessDataForm>(page.values);
  const flow = useMove(assignment, "business-data");
  const opts = page.options;
  const failed = (key: string) => page.failedOptions.includes(key);
  const err = (key: string) => flow.errorOf("form", key);
  const set = (key: keyof BusinessDataForm, errorKey: string, v: string) => {
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

  return (
    <HotProspectCard
      title="Data Usaha Nasabah"
      step={0}
      flow={flow}
      body={() => form}
      editable={page.editable}
      failedOptions={page.failedOptions}
      buttons={
        <>
          <button type="button" className={styles.close} onClick={() => leave(assignment)}>
            Close
          </button>
          {!readOnly && (
            <div className={styles.buttonGroup}>
              <button type="button" className={styles.back} disabled={flow.busy} onClick={() => flow.move(save, form)}>
                Save
              </button>
              <button type="button" className={styles.back} disabled>
                Back to Previous Step
              </button>
              <button type="button" className={styles.next} disabled={flow.busy} onClick={() => flow.move(hotProspectSteps[1], form)}>
                Next
              </button>
            </div>
          )}
        </>
      }
    >
      <h3 className={styles.sectionTitle}>Data Usaha Nasabah</h3>
      <div className={styles.formPanel}>
        <div className={styles.formGrid}>
          <TextField label="Jenis Bidang Usaha" required maxLength={100} value={form.businessField} onChange={(v) => set("businessField", "businessField", v)} error={err("businessField")} disabled={readOnly} />
          <CodeSelect label="Kategori Bidang Usaha" placeholder="Pilih Kategori Bidang Usaha" required options={opts.businessCategory} failedItem={failed("businessCategory")} value={form.businessCategoryCode} onChange={(v) => set("businessCategoryCode", "businessCategory", v)} error={err("businessCategory")} disabled={readOnly} />
          <NumberField label="Lama Usaha (Bulan)" maxDigits={5} value={form.businessAgeMonths} onChange={(v) => set("businessAgeMonths", "businessAgeMonths", v)} error={err("businessAgeMonths")} disabled={readOnly} />
          <NumberField label="Jumlah Karyawan" maxDigits={7} value={form.employeeCount} onChange={(v) => set("employeeCount", "employeeCount", v)} error={err("employeeCount")} disabled={readOnly} />
          <CodeSelect label="Jenis Tempat Usaha" placeholder="Pilih Jenis Tempat Usaha" required options={opts.businessLocation} failedItem={failed("businessLocation")} value={form.businessLocationCode} onChange={(v) => set("businessLocationCode", "businessLocation", v)} error={err("businessLocation")} disabled={readOnly} />
          <span />
          <TextField label="Alamat Usaha" required area maxLength={350} value={form.address} onChange={(v) => set("address", "address", v)} error={err("address")} disabled={readOnly} />
          <CodeSelect label="Status Kepemilikan Tempat Usaha" placeholder="Pilih Data" required options={opts.houseOwnership} failedItem={failed("houseOwnership")} value={form.houseOwnershipCode} onChange={(v) => set("houseOwnershipCode", "houseOwnership", v)} error={err("houseOwnership")} disabled={readOnly} />
          <NumberField label="Lama Tempat Usaha (Tahun)" maxDigits={3} value={form.yearsOfStay} onChange={(v) => set("yearsOfStay", "yearsOfStay", v)} error={err("yearsOfStay")} disabled={readOnly} />
          <NumberField label="Lama Tempat Usaha (Bulan)" maxDigits={2} value={form.monthsOfStay} onChange={(v) => set("monthsOfStay", "monthsOfStay", v)} error={err("monthsOfStay")} disabled={readOnly} />
          <TextField label="Nomor Telepon" maxLength={20} value={form.phone} onChange={(v) => set("phone", "phone", v)} error={err("phone")} disabled={readOnly} />
          <TextField label="RT" required maxLength={3} value={form.rt} onChange={(v) => set("rt", "rt", v)} error={err("rt")} disabled={readOnly} />
          <TextField label="RW" required maxLength={3} value={form.rw} onChange={(v) => set("rw", "rw", v)} error={err("rw")} disabled={readOnly} />
          <span />
          <RegionRows fields={region} />
        </div>
      </div>
    </HotProspectCard>
  );
}

const emptyCard: OtherFinancingCard = {
  bankName: "",
  financingType: "",
  financingAmount: "",
  outstandingAmount: "",
  tenorMonths: "",
  marginRate: "",
  slikCollect: "",
  purpose: "",
};

// UI-031 Data Pembiayaan Lain (Gambar 72, 73, FR-APP-017): cards, none
// required; all-empty cards are dropped by the server; Isi Data dari SLIK
// replaces them with the facilities ticked. Back leaves as Close does
// (agreed comment 4); Back to Previous Step saves and opens Data Usaha
// Nasabah.
export function OtherFinancingData({ page, assignment }: { page: OtherFinancingPage; assignment: AssignmentRef }) {
  const readOnly = !page.editable;
  const [cards, setCards] = useState<OtherFinancingCard[]>(page.values.otherFinancings);
  const [slikOpen, setSLIKOpen] = useState(false);
  const flow = useMove(assignment, "other-financing");
  const body = () => ({ otherFinancings: cards });
  // The cards' errors go with a change of cards: their numbers no longer hold.
  const dropCardErrors = () => flow.dropErrors((e) => sectionNumber("other-financing", e.section) !== null);
  const setCard = (i: number, key: keyof OtherFinancingCard, v: string) => {
    setCards((all) => all.map((c, j) => (j === i ? { ...c, [key]: v } : c)));
    flow.clear(otherFinancingSection(i + 1), key);
  };
  const removeCard = (i: number) => {
    setCards((all) => all.filter((_, j) => j !== i));
    dropCardErrors();
  };

  return (
    <HotProspectCard
      title="Data Pembiayaan Lain"
      step={1}
      flow={flow}
      body={body}
      editable={page.editable}
      buttons={
        <>
          <button type="button" className={styles.back} onClick={() => leave(assignment)}>
            Back
          </button>
          {!readOnly && (
            <div className={styles.buttonGroup}>
              <button type="button" className={styles.back} disabled={flow.busy} onClick={() => flow.move(save, body())}>
                Save
              </button>
              <button type="button" className={styles.back} disabled={flow.busy} onClick={() => flow.move(hotProspectSteps[0], body())}>
                Back to Previous Step
              </button>
              <button type="button" className={styles.next} disabled={flow.busy} onClick={() => flow.move(hotProspectSteps[2], body())}>
                Next
              </button>
            </div>
          )}
        </>
      }
    >
      <div className={styles.sectionHeader}>
        <h3 className={styles.sectionTitle}>Data Pembiayaan Lain</h3>
        {!readOnly && (
          <div className={styles.buttonGroup}>
            {page.slikAvailable && (
              <button type="button" className={styles.addCompanyOfficer} onClick={() => setSLIKOpen(true)}>
                <span aria-hidden="true">+</span> Isi Data dari SLIK
              </button>
            )}
            <button type="button" className={styles.addCompanyOfficer} onClick={() => (setCards((all) => [...all, emptyCard]), dropCardErrors())}>
              <span aria-hidden="true">+</span> Tambah Pembiayaan
            </button>
          </div>
        )}
      </div>
      <div className={styles.formPanel}>
        {cards.map((c, i) => {
          const cardError = (key: string) => flow.errorOf(otherFinancingSection(i + 1), key);
          return (
            <div key={i} className={styles.financingCard} role="group" aria-label={`Pembiayaan ${i + 1}`}>
              {!readOnly && (
                // Deleted without confirmation (PROPOSED); every card may go.
                <button type="button" className={styles.removeCompanyOfficer} aria-label={`Hapus Pembiayaan ${i + 1}`} onClick={() => removeCard(i)}>
                  <svg width="12" height="14" viewBox="0 0 12 14" aria-hidden="true">
                    <path d="M1 3h10M4 3V1h4v2M2 3l1 10h6l1-10" fill="currentColor" stroke="currentColor" strokeWidth="1" />
                  </svg>
                </button>
              )}
              <div className={styles.formGrid}>
                <TextField label="Nama Bank" maxLength={100} value={c.bankName} onChange={(v) => setCard(i, "bankName", v)} error={cardError("bankName")} disabled={readOnly} />
                <TextField label="Jenis Pembiayaan" maxLength={100} value={c.financingType} onChange={(v) => setCard(i, "financingType", v)} error={cardError("financingType")} disabled={readOnly} />
                <NumberField label="Jumlah Pembiayaan" money required={false} maxDigits={15} value={c.financingAmount} onChange={(v) => setCard(i, "financingAmount", v)} error={cardError("financingAmount")} disabled={readOnly} />
                <span />
                <NumberField label="Jumlah Outstanding" money required={false} maxDigits={15} value={c.outstandingAmount} onChange={(v) => setCard(i, "outstandingAmount", v)} error={cardError("outstandingAmount")} disabled={readOnly} />
                <NumberField label="Jangka Waktu (Bulan)" required={false} placeholder="0" maxDigits={4} value={c.tenorMonths} onChange={(v) => setCard(i, "tenorMonths", v)} error={cardError("tenorMonths")} disabled={readOnly} />
                <PercentField label="Margin / Ujrah (%)" value={c.marginRate} onChange={(v) => setCard(i, "marginRate", v)} error={cardError("marginRate")} disabled={readOnly} />
                <NumberField label="SLIK Collect" required={false} placeholder="0" maxDigits={1} value={c.slikCollect} onChange={(v) => setCard(i, "slikCollect", v)} error={cardError("slikCollect")} disabled={readOnly} />
                <div className={styles.wide}>
                  <TextField label="Tujuan" area maxLength={500} value={c.purpose} onChange={(v) => setCard(i, "purpose", v)} error={cardError("purpose")} disabled={readOnly} />
                </div>
              </div>
            </div>
          );
        })}
      </div>
      {slikOpen && (
        <SLIKFacilitiesModal
          assignment={assignment}
          onCancel={() => setSLIKOpen(false)}
          onSubmit={(picked) => {
            // Submit with nothing ticked changes nothing (PROPOSED).
            if (picked.length > 0) {
              setCards(picked);
              dropCardErrors();
            }
            setSLIKOpen(false);
          }}
        />
      )}
    </HotProspectCard>
  );
}

// slikMoney shows an amount as SLIK has it: "Rp7.000.000,00".
function slikMoney(v: string): string {
  if (v === "") {
    return "";
  }
  const [whole, decimals = "00"] = v.split(".");
  return `Rp${whole.replace(/\B(?=(\d{3})+(?!\d))/g, ".")},${decimals}`;
}

// The Tambah Pembiayaan Lain dari Data SLIK modal (Gambar 73): one accordion
// per condition, closed at first, header and facility checkboxes in step;
// Submit hands back the cards of the facilities ticked.
function SLIKFacilitiesModal({
  assignment,
  onCancel,
  onSubmit,
}: {
  assignment: AssignmentRef;
  onCancel: () => void;
  onSubmit: (cards: OtherFinancingCard[]) => void;
}) {
  const [groups, setGroups] = useState<SLIKFacilityGroup[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [open, setOpen] = useState<number[]>([]);
  const [ticked, setTicked] = useState<Set<string>>(new Set());
  const { endpointPath, caseId, assignmentId } = assignment;

  // Read once when the modal opens.
  useEffect(() => {
    let live = true;
    apiFetch(assignmentURL({ endpointPath, caseId, assignmentId }, "slik-facilities"))
      .then((res) => (res.ok ? (res.json() as Promise<{ groups: SLIKFacilityGroup[] }>) : Promise.reject()))
      .then((body) => live && setGroups(body.groups))
      .catch(() => live && setFailed(true));
    return () => {
      live = false;
    };
  }, [endpointPath, caseId, assignmentId]);

  const key = (g: number, f: number) => `${g}/${f}`;
  const allTicked = (g: number) => groups![g].facilities.every((_, f) => ticked.has(key(g, f)));
  const tickGroup = (g: number, on: boolean) =>
    setTicked((all) => {
      const next = new Set(all);
      groups![g].facilities.forEach((_, f) => (on ? next.add(key(g, f)) : next.delete(key(g, f))));
      return next;
    });
  const tick = (g: number, f: number, on: boolean) =>
    setTicked((all) => {
      const next = new Set(all);
      if (on) {
        next.add(key(g, f));
      } else {
        next.delete(key(g, f));
      }
      return next;
    });
  const submit = () =>
    onSubmit((groups ?? []).flatMap((g, gi) => g.facilities.filter((_, fi) => ticked.has(key(gi, fi))).map((f) => f.card)));

  return (
    <div className={styles.overlay} role="presentation">
      <div className={styles.modal} role="dialog" aria-label="Tambah Pembiayaan Lain dari Data SLIK">
        <div className={styles.modalHeader}>
          <h2 className={styles.modalTitle}>Tambah Pembiayaan Lain dari Data SLIK</h2>
          <button type="button" className={styles.modalX} aria-label="Tutup" onClick={onCancel}>
            ×
          </button>
        </div>
        <div className={styles.slikGroups}>
          {failed && <p className={styles.emptyText}>{unavailableMessage}</p>}
          {groups?.map((g, gi) => (
            <div key={g.title} className={styles.slikGroup}>
              <div className={styles.slikGroupHeader}>
                <button
                  type="button"
                  className={styles.slikGroupToggle}
                  aria-expanded={open.includes(gi)}
                  onClick={() => setOpen((all) => (all.includes(gi) ? all.filter((x) => x !== gi) : [...all, gi]))}
                >
                  <span className={open.includes(gi) ? styles.chevronOpen : styles.chevron} aria-hidden="true">
                    ›
                  </span>
                  {g.title}
                </button>
                <input type="checkbox" aria-label={`Pilih semua ${g.title}`} checked={allTicked(gi)} onChange={(e) => tickGroup(gi, e.target.checked)} />
              </div>
              {open.includes(gi) &&
                g.facilities.map((f, fi) => (
                  <div key={fi} className={styles.slikFacility}>
                    <input
                      type="checkbox"
                      className={styles.slikFacilityTick}
                      aria-label={`Pilih ${f.card.bankName || `fasilitas ${fi + 1}`}`}
                      checked={ticked.has(key(gi, fi))}
                      onChange={(e) => tick(gi, fi, e.target.checked)}
                    />
                    <div className={styles.formGrid}>
                      <ShownField label="Nama Bank" value={f.card.bankName} />
                      <ShownField label="Jenis Pembiayaan" value={f.card.financingType} />
                      <ShownField label="Jumlah Pembiayaan" value={slikMoney(f.financingAmount)} />
                      <span />
                      <ShownField label="Jumlah Outstanding" value={slikMoney(f.outstandingAmount)} />
                      <ShownField label="Jangka Waktu (Bulan)" value={f.card.tenorMonths} />
                      <ShownField label="Margin / Ujrah (%)" value={f.marginRate.replace(".", ",")} />
                      <ShownField label="SLIK Collect" value={f.card.slikCollect} />
                      <ShownField label="Tujuan" value={f.card.purpose} />
                    </div>
                  </div>
                ))}
            </div>
          ))}
        </div>
        <div className={styles.modalButtons}>
          <button type="button" className={styles.modalCancel} onClick={onCancel}>
            Cancel
          </button>
          <button type="button" className={styles.modalSubmit} onClick={submit}>
            Submit
          </button>
        </div>
      </div>
    </div>
  );
}

// A read-only field of the modal: grey label, black value.
function ShownField({ label, value }: { label: string; value: string }) {
  return (
    <div className={styles.field}>
      <span className={styles.fieldLabel}>{label}</span>
      <span>{value}</span>
    </div>
  );
}
