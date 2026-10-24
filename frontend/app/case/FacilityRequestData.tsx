"use client";

import { useEffect, useState } from "react";
import { apiFetch, assignmentURL, type AssignmentRef } from "@/lib/api";
import Field from "./Field";
import { CodeRadios, CodeSelect, NumberField, PercentField, TextField, save, useMove } from "./FormFields";
import { hotProspectSteps } from "./Breadcrumb";
import { HotProspectCard, leave } from "./HotProspectData";
import type { FinancingSimulation, FacilityRequestForm, FacilityRequestPage } from "./types";
import styles from "./case.module.css";

// Next on this step also checks the product limit and records the
// recommendation (agreed comment 8).
const next = "next";

// Tipe Margin Flat (BMV MARGINTYPE, DEC-145): the typed margin is flat.
const marginFlat = "FLAT";

const noSimulation: FinancingSimulation = { ready: false, installmentFlat: "0", installmentEffective: "0", marginFlat: "0.00000", marginEffective: "0.00000" };

// rupiah is "Rp100.000.000" (BR-UI-009); cents adds ",00" (FR-APP-018 rule 3).
function rupiah(digits: string, cents = false): string {
  return `Rp${digits.replace(/\B(?=(\d{3})+(?!\d))/g, ".")}${cents ? ",00" : ""}`;
}

// percent5 is "12,00000%" (BR-UI-014).
const percent5 = (v: string) => `${v.replace(".", ",")}%`;

// useFinancingSimulation asks the server for the simulation of the four fields
// whenever one changes (agreed comment 11); stale while a newer answer is
// due.
function useFinancingSimulation(assignment: AssignmentRef, form: FacilityRequestForm) {
  const { endpointPath, caseId, assignmentId } = assignment;
  const { requestedAmount, tenureMonths, marginTypeCode, marginRate } = form;
  const key = [requestedAmount, tenureMonths, marginTypeCode, marginRate].join("|");
  const [answer, setAnswer] = useState<{ key: string; simulation: FinancingSimulation }>({ key: "", simulation: noSimulation });

  useEffect(() => {
    let live = true;
    const timer = setTimeout(() => {
      apiFetch(assignmentURL({ endpointPath, caseId, assignmentId }, "facility-simulation"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ requestedAmount, tenureMonths, marginTypeCode, marginRate }),
      })
        .then((res) => (res.ok ? (res.json() as Promise<FinancingSimulation>) : Promise.reject()))
        .catch(() => noSimulation)
        .then((simulation) => live && setAnswer({ key, simulation }));
    }, 200);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [endpointPath, caseId, assignmentId, key, requestedAmount, tenureMonths, marginTypeCode, marginRate]);

  return { simulation: answer.key === key ? answer.simulation : noSimulation, stale: answer.key !== key };
}

// A computed value: bold label and value when bold (Total Obligor).
function Computed({ label, value, bold = false }: { label: string; value: string; bold?: boolean }) {
  return (
    <div className={styles.field}>
      <span className={bold ? styles.computedLabelBold : styles.computedLabel}>{label}</span>
      <span aria-label={label} className={bold ? styles.computedBold : undefined}>
        {value}
      </span>
    </div>
  );
}

// UI-032 Data Permohonan Fasilitas (Gambar 74-77, FR-APP-018): the request,
// the existing obligor and the approval side follow the typed values at
// once; Hitung Simulasi Angsuran opens the Financing Simulation. Back leaves as
// Close does; Back to Previous Step saves and opens Data Pembiayaan Lain
// (agreed comment 7).
export default function FacilityRequestData({ page, assignment }: { page: FacilityRequestPage; assignment: AssignmentRef }) {
  const readOnly = !page.editable;
  const [form, setForm] = useState<FacilityRequestForm>(page.values);
  const [simulationOpen, setSimulationOpen] = useState(false);
  const flow = useMove(assignment, "facility-request");
  const { simulation, stale } = useFinancingSimulation(assignment, form);
  const opts = page.options;
  const failed = (key: string) => page.failedOptions.includes(key);
  const err = (key: string) => flow.errorOf("form", key);
  const set = (key: keyof FacilityRequestForm, v: string) => {
    setForm((f) => ({ ...f, [key]: v }));
    flow.clear("form", key);
  };

  // FR-APP-018 Process 3-6: one request line, so the total is its amount;
  // the approval side follows the request.
  const totalObligor = (BigInt(form.existingFinancingAmount || "0") + BigInt(form.requestedAmount || "0")).toString();
  const flat = form.marginTypeCode === marginFlat;
  const installment = flat ? simulation.installmentFlat : simulation.installmentEffective;
  const contractType = page.contractType;

  return (
    <HotProspectCard
      title="Data Permohonan Fasilitas"
      step={2}
      flow={flow}
      body={() => form}
      editable={page.editable}
      facilityRequested={page.facilityRequested}
      failedOptions={page.failedOptions}
      buttons={
        <>
          <button type="button" className={styles.back} onClick={() => leave(assignment)}>
            Back
          </button>
          {!readOnly && (
            <div className={styles.buttonGroup}>
              <button type="button" className={styles.back} disabled={flow.busy} onClick={() => flow.move(save, form)}>
                Save
              </button>
              <button type="button" className={styles.back} disabled={flow.busy} onClick={() => flow.move(hotProspectSteps[1], form)}>
                Back to Previous Step
              </button>
              <button type="button" className={styles.next} disabled={flow.busy} onClick={() => flow.move(next, form)}>
                Next
              </button>
            </div>
          )}
        </>
      }
    >
      <h3 className={styles.sectionTitle}>Fasilitas Pemohonan Nasabah</h3>
      <div className={styles.formPanel}>
        <div className={styles.facilityCard}>
          <div className={styles.formGrid3}>
            <CodeRadios label="Tujuan Pembiayaan" options={opts.purpose} failedItem={failed("purpose")} value={form.purposeCode} onChange={(v) => set("purposeCode", v)} error={err("purposeCode")} disabled={readOnly} />
            <Field label="Tipe Akad">
              <select aria-label="Tipe Akad" className={styles.inputDisabled} value={contractType} disabled>
                <option value={contractType}>{contractType}</option>
              </select>
            </Field>
            <NumberField label="Pembiayaan yang di ajukan" money maxDigits={15} value={form.requestedAmount} onChange={(v) => set("requestedAmount", v)} error={err("requestedAmount")} disabled={readOnly} />
          </div>
          <Computed label="Total Requested Amount" value={form.requestedAmount ? rupiah(form.requestedAmount) : "--"} />
          <NumberField label="Jangka Waktu (Bulan)" placeholder="0" maxDigits={4} value={form.tenureMonths} onChange={(v) => set("tenureMonths", v)} error={err("tenureMonths")} disabled={readOnly} />
        </div>
      </div>

      <h3 className={styles.sectionTitle}>Total Existing Obligor Bank Nano Syariah</h3>
      <div className={styles.formPanel}>
        <div className={styles.facilityCard}>
          <NumberField label="Jumlah Pembiayaan Eksisting di Bank Nano" money maxDigits={15} value={form.existingFinancingAmount} onChange={(v) => set("existingFinancingAmount", v)} error={err("existingFinancingAmount")} disabled={readOnly} />
          <Computed label="Total Obligor" value={rupiah(totalObligor)} bold />
        </div>
      </div>

      <h3 className={styles.sectionTitle}>Fasilitas Persetujuan Pembiayaan</h3>
      <div className={styles.formPanel}>
        <NumberField label="Pembiayaan Yang Disetujui" money readOnly maxDigits={15} value={form.requestedAmount} onChange={() => {}} disabled={false} />
        <div className={styles.formGrid}>
          <NumberField label="Jangka Waktu (Bulan)" readOnly placeholder="0" maxDigits={4} value={form.tenureMonths} onChange={() => {}} disabled={false} />
          <Computed label="Angsuran Per Bulan" value={rupiah(installment, true)} />
          <div className={styles.marginPair}>
            <CodeSelect label="Tipe Margin" placeholder="Pilih Tipe Margin" required options={opts.marginType} failedItem={failed("marginType")} value={form.marginTypeCode} onChange={(v) => set("marginTypeCode", v)} error={err("marginTypeCode")} disabled={readOnly} />
            <PercentField label="Margin / Ujrah (%)" decimals={5} required value={form.marginRate} onChange={(v) => set("marginRate", v)} error={err("marginRate")} disabled={readOnly} />
          </div>
          {/* The other side of the margin, once a type is chosen (agreed comment 14). */}
          {form.marginTypeCode ? (
            <Computed label={flat ? "Margin Efektif (%)" : "Margin Flat (%)"} value={percent5(flat ? simulation.marginEffective : simulation.marginFlat)} />
          ) : (
            <span />
          )}
          <div className={styles.wide}>
            <TextField label="Catatan Deviasi" area maxLength={1000} value={form.deviationNote} onChange={(v) => set("deviationNote", v)} error={err("deviationNote")} disabled={readOnly} />
          </div>
          <div className={styles.wide}>
            <TextField label="Catatan Mitigasi" area maxLength={1000} value={form.mitigationNote} onChange={(v) => set("mitigationNote", v)} error={err("mitigationNote")} disabled={readOnly} />
          </div>
        </div>
      </div>

      <div className={styles.sectionHeader}>
        <h3 className={styles.sectionTitle}>Catatan Petugas Bank</h3>
        <button type="button" className={styles.addCompanyOfficer} disabled={!simulation.ready || stale} onClick={() => setSimulationOpen(true)}>
          Hitung Simulasi Angsuran
        </button>
      </div>
      <div className={styles.formPanel}>
        <TextField label="Notes" required area maxLength={1000} value={form.notes} onChange={(v) => set("notes", v)} error={err("notes")} disabled={readOnly} />
      </div>

      {simulationOpen && (
        <FinancingSimulationModal
          rows={[
            ["Produk Name", page.productName],
            ["Jenis Pembiayaan", opts.purpose.find((o) => o.code === form.purposeCode)?.description ?? ""],
            ["Periode (jangka waktu)", `${form.tenureMonths} Bulan`],
            ["Nama Debitur", page.applicantName],
            ["Plafon Pembiayaan", rupiah(form.requestedAmount)],
            ["Satuan Kurs", "Rupiah ( Rp. )"],
            ["Rate Indikasi(Effektif) p.a. (%)", percent5(simulation.marginEffective)],
            ["Rate Indikasi(Flat) p.a. (%)", percent5(simulation.marginFlat)],
            ["Angsuran per Bulan (Rate Efektif)", rupiah(simulation.installmentEffective, true)],
            ["Angsuran per Bulan (Rate Flat)", rupiah(simulation.installmentFlat, true)],
          ]}
          onClose={() => setSimulationOpen(false)}
        />
      )}
    </HotProspectCard>
  );
}

// The Financing Simulation modal, titled "Credit Simulation" (Gambar 77):
// two columns of label and value;
// Home and X only close it (FR-APP-018 Process 7).
function FinancingSimulationModal({ rows, onClose }: { rows: [string, string][]; onClose: () => void }) {
  return (
    <div className={styles.overlay} role="presentation">
      <div className={`${styles.modal} ${styles.simulationModal}`} role="dialog" aria-label="Credit Simulation">
        <div className={styles.modalHeader}>
          <h2 className={styles.modalTitle}>Credit Simulation</h2>
          <button type="button" className={styles.modalX} aria-label="Tutup" onClick={onClose}>
            ×
          </button>
        </div>
        <h3 className={styles.simulationTitle}>Credit Simulation</h3>
        <dl className={styles.simulationPanel}>
          {rows.map(([label, value]) => (
            <div key={label}>
              <dt>{label}</dt>
              <dd>{value}</dd>
            </div>
          ))}
        </dl>
        <button type="button" className={styles.homeButton} onClick={onClose}>
          <svg width="13" height="12" viewBox="0 0 13 12" aria-hidden="true">
            <path d="M6.5 0 0 5.5h1.8V12h3.4V8h2.6v4h3.4V5.5H13z" fill="currentColor" />
          </svg>
          Home
        </button>
      </div>
    </div>
  );
}
