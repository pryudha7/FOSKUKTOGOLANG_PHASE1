"use client";

import { useState, type ReactNode } from "react";
import type { AssignmentRef } from "@/lib/api";
import { hotProspectSteps } from "./Breadcrumb";
import DateField from "./DateField";
import Field from "./Field";
import { CodeSelect, NumberField, Pair, save, TextField, useMove } from "./FormFields";
import { inputClass } from "./fields";
import { HotProspectCard, leave } from "./HotProspectData";
import { numberedSection, sectionNumber, type CollateralCard, type CollateralPage, type NumberedSection, type Option } from "./types";
import styles from "./case.module.css";

// The collateral types (BMV MAINCOLLATERAL, COLLATERALSUPPORT codes) and
// Status Paripasu Ya (FR-APP-021 Field A-D, Process 5).
const landBuilding = "Tanah dan Bangunan";
const deposit = "Deposito Bank Nano Syariah";
const vacantLand = "Tanah Kosong Produktif";
const vehicle = "Kendaraan";
const pariPassuYes = "Y";

const emptyCard: CollateralCard = {
  typeCode: "",
  buildingFormCode: "",
  ownershipTypeCode: "",
  ownerName: "",
  shmNumber: "",
  shgbNumber: "",
  shgbValidUntil: "",
  bpkbNumber: "",
  fiduciaryNumber: "",
  location: "",
  landArea: "",
  dueDate: "",
  physicalValue: "",
  objectValue: "",
  depositAmount: "",
  vehicleBrand: "",
  vehicleYear: "",
  remarks: "",
  collateralStatusCode: "",
  collateralTypeText: "",
  bindingType: "",
  bindingDate: "",
  appraisalDate: "",
  kjppAppraisalDate: "",
  njopValue: "",
  kjppValue: "",
  kjppAppraiserName: "",
  pariPassuStatusCode: "",
  pariPassuPercent: "",
  jointFinancingCode: "",
  insuredCode: "",
};

const shownPariPassu = (c: CollateralCard): CollateralCard => (c.pariPassuStatusCode === pariPassuYes ? c : { ...c, pariPassuPercent: "" });

// The two sections: Agunan Utama then Jaminan Pendukung.
type SectionName = "main" | "support";
const sections: { name: SectionName; title: string; prefix: NumberedSection; typeOptions: string; errorKey: string }[] = [
  { name: "main", title: "Agunan Utama", prefix: "main-collateral", typeOptions: "mainCollateral", errorKey: "mainCollateral" },
  { name: "support", title: "Jaminan Pendukung", prefix: "support-collateral", typeOptions: "supportCollateral", errorKey: "supportCollateral" },
];

// UI-034 Collateral Data (Gambar 84-90, FR-APP-021): cards per section
// shaped by their collateral type. Back leaves as Close does; Back to
// Previous Step saves and opens QCA Data (agreed comment 4).
export default function CollateralData({ page, assignment }: { page: CollateralPage; assignment: AssignmentRef }) {
  const readOnly = !page.editable;
  // Presentase Paripasu shows empty unless Status Paripasu is Y (Process 5);
  // the stored 0 is the server's.
  const [cards, setCards] = useState(() => ({ main: page.values.main.map(shownPariPassu), support: page.values.support.map(shownPariPassu) }));
  const flow = useMove(assignment, "collateral");
  const body = () => cards;
  const opts = page.options;
  const failed = (key: string) => page.failedOptions.includes(key);

  // A change of cards moves their numbers: their errors go.
  const dropCardErrors = (prefix: NumberedSection) => flow.dropErrors((e) => sectionNumber(prefix, e.section) !== null);
  const setCard = (name: SectionName, prefix: NumberedSection, i: number, next: CollateralCard, changed: (keyof CollateralCard)[]) => {
    setCards((all) => ({ ...all, [name]: all[name].map((c, j) => (j === i ? next : c)) }));
    changed.forEach((key) => flow.clear(numberedSection(prefix, i + 1), key));
  };

  return (
    <HotProspectCard
      title="Collateral Data"
      step={4}
      flow={flow}
      body={body}
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
              <button type="button" className={styles.back} disabled={flow.busy} onClick={() => flow.move(save, body())}>
                Save
              </button>
              <button type="button" className={styles.back} disabled={flow.busy} onClick={() => flow.move(hotProspectSteps[3], body())}>
                Back to Previous Step
              </button>
              <button type="button" className={styles.next} disabled={flow.busy} onClick={() => flow.move(hotProspectSteps[5], body())}>
                Next
              </button>
            </div>
          )}
        </>
      }
    >
      <h3 className={styles.sectionTitle}>Kolateral Data</h3>
      {sections.map((s) => {
        const list = cards[s.name];
        const sectionError = flow.errorOf("form", s.errorKey);
        return (
          <div key={s.name} className={styles.collateralSection}>
            <div className={styles.sectionHeader}>
              <h3 className={styles.sectionTitle}>{s.title}</h3>
              {!readOnly && (
                // At most maxCards cards per section (Process 2, PROPOSED).
                <button
                  type="button"
                  className={styles.addCard}
                  disabled={list.length >= page.maxCards}
                  onClick={() => (setCards((all) => ({ ...all, [s.name]: [...all[s.name], emptyCard] })), dropCardErrors(s.prefix))}
                >
                  <span aria-hidden="true">+</span> Tambah {s.title}
                </button>
              )}
            </div>
            {sectionError && <span className={styles.fieldError}>{sectionError}</span>}
            {list.map((c, i) => (
              <CollateralCardFields
                key={i}
                title={s.title}
                number={i + 1}
                card={c}
                typeOptions={opts[s.typeOptions] ?? []}
                typeFailed={failed(s.typeOptions)}
                opts={opts}
                failed={failed}
                today={page.today}
                readOnly={readOnly}
                error={(key) => flow.errorOf(numberedSection(s.prefix, i + 1), key)}
                onChange={(next, changed) => setCard(s.name, s.prefix, i, next, changed)}
                onRemove={() => (setCards((all) => ({ ...all, [s.name]: all[s.name].filter((_, j) => j !== i) })), dropCardErrors(s.prefix))}
              />
            ))}
          </div>
        );
      })}
    </HotProspectCard>
  );
}

// The number field of the Tipe Bukti Kepemilikan (Field E): 1 and 4 Nomor
// SHM, 2 Nomor SHGB and Berlaku Sampai, 3 Nomor BPKB, 5 Nomor Fidusia.
const ownershipNumbers: (keyof CollateralCard)[] = ["shmNumber", "shgbNumber", "shgbValidUntil", "bpkbNumber", "fiduciaryNumber"];

function CollateralCardFields({
  title,
  number,
  card,
  typeOptions,
  typeFailed,
  opts,
  failed,
  today,
  readOnly,
  error,
  onChange,
  onRemove,
}: {
  title: string;
  number: number;
  card: CollateralCard;
  typeOptions: Option[];
  typeFailed: boolean;
  opts: Record<string, Option[]>;
  failed: (key: string) => boolean;
  today: string;
  readOnly: boolean;
  error: (key: string) => string | undefined;
  onChange: (next: CollateralCard, changed: (keyof CollateralCard)[]) => void;
  onRemove: () => void;
}) {
  const set = (key: keyof CollateralCard, v: string) => {
    // Process 5: Status Paripasu other than Y empties Presentase Paripasu.
    if (key === "pariPassuStatusCode" && v !== pariPassuYes) {
      onChange({ ...card, pariPassuStatusCode: v, pariPassuPercent: "" }, [key, "pariPassuPercent"]);
      return;
    }
    onChange({ ...card, [key]: v }, [key]);
  };
  // Process 3: another type empties the card.
  const setType = (v: string) => v !== card.typeCode && onChange({ ...emptyCard, typeCode: v }, Object.keys(emptyCard) as (keyof CollateralCard)[]);
  // Process 4: another Tipe Bukti Kepemilikan empties the number fields.
  const setOwnership = (v: string) => {
    if (v === card.ownershipTypeCode) {
      return;
    }
    const next = { ...card, ownershipTypeCode: v };
    ownershipNumbers.forEach((k) => (next[k] = ""));
    onChange(next, ["ownershipTypeCode", ...ownershipNumbers]);
  };
  const text = (key: keyof CollateralCard, label: string, max: number) => (
    <TextField label={label} maxLength={max} value={card[key]} onChange={(v) => set(key, v)} error={error(key)} disabled={readOnly} />
  );
  const money = (key: keyof CollateralCard, label: string) => (
    <NumberField label={label} money required={false} maxDigits={15} value={card[key]} onChange={(v) => set(key, v)} error={error(key)} disabled={readOnly} />
  );
  const date = (key: keyof CollateralCard, label: string) => (
    <Field label={label} error={error(key)}>
      <DateField ariaLabel={label} value={card[key]} invalid={!!error(key)} today={today} disabled={readOnly} onChange={(v) => set(key, v)} />
    </Field>
  );
  const code = (key: keyof CollateralCard, label: string, optionsKey: string, placeholder = "", required = false) => (
    <CodeSelect
      label={label}
      placeholder={placeholder}
      required={required}
      options={opts[optionsKey] ?? []}
      failedItem={failed(optionsKey)}
      value={card[key]}
      onChange={(v) => set(key, v)}
      error={error(key)}
      disabled={readOnly}
    />
  );
  const ownership = (
    <CodeSelect
      label="Tipe Bukti Kepemilikan"
      placeholder="Pilih Tipe Bukti Kepemilikan"
      options={opts.ownershipType ?? []}
      failedItem={failed("ownershipType")}
      value={card.ownershipTypeCode}
      onChange={setOwnership}
      error={error("ownershipTypeCode")}
      disabled={readOnly}
    />
  );
  const ownershipNumber: ReactNode = (() => {
    switch (card.ownershipTypeCode) {
      case "1":
      case "4":
        return text("shmNumber", "Nomor SHM", 50);
      case "2":
        return (
          <Pair>
            {text("shgbNumber", "Nomor SHGB", 50)}
            {date("shgbValidUntil", "Berlaku Sampai")}
          </Pair>
        );
      case "3":
        return text("bpkbNumber", "Nomor BPKB", 50);
      case "5":
        return text("fiduciaryNumber", "Nomor Fidusia", 50);
      default:
        return <span />;
    }
  })();

  return (
    <div className={styles.collateralCard} role="group" aria-label={`${title} ${number}`}>
      <div className={styles.collateralTypeRow}>
        <CodeSelect
          label={title}
          placeholder="Select"
          options={typeOptions}
          failedItem={typeFailed}
          value={card.typeCode}
          onChange={setType}
          error={error("typeCode")}
          disabled={readOnly}
        />
        {!readOnly && (
          // Removed without confirmation (CONFIRMED); saved on Next.
          <button type="button" className={styles.removeCollateral} aria-label={`Hapus ${title} ${number}`} onClick={onRemove}>
            <svg width="12" height="14" viewBox="0 0 12 14" aria-hidden="true">
              <path d="M1 3h10M4 3V1h4v2M2 3l1 10h6l1-10" fill="currentColor" stroke="currentColor" strokeWidth="1" />
            </svg>
          </button>
        )}
      </div>
      {card.typeCode === landBuilding && (
        <>
          <div className={styles.formGrid}>
            {code("buildingFormCode", "Tanah dan Bangunan Yang Berbentuk", "buildingForm", "Pilih bentuk Tanah dan Bangunan", true)}
            {ownership}
            {text("ownerName", "Nama Pemilik", 100)}
            {ownershipNumber}
            {money("physicalValue", "Nilai Aktual Fisik (LPA UTKP)")}
            {text("location", "Lokasi", 255)}
          </div>
          <Details card={card} set={set} text={text} money={money} date={date} code={code} pariPassuRequired readOnly={readOnly} error={error} />
          <h4 className={styles.collateralRemarksTitle}>Keterangan</h4>
          {text("remarks", "Agunan diperuntukan dan Jarak lokasi usaha dari kantor cabang Bank Nano Syariah", 500)}
        </>
      )}
      {card.typeCode === deposit && money("depositAmount", "Jumlah Deposito Bank Nano Syariah")}
      {card.typeCode === vacantLand && (
        <>
          <div className={styles.formGrid}>
            <div className={styles.narrowField}>{ownership}</div>
            {ownershipNumber}
            {text("ownerName", "Atas Nama", 100)}
            {text("location", "Lokasi", 255)}
            {text("landArea", "Luas Tanah", 50)}
            {date("dueDate", "Jatuh Tempo")}
          </div>
          <Details card={card} set={set} text={text} money={money} date={date} code={code} readOnly={readOnly} error={error} />
        </>
      )}
      {card.typeCode === vehicle && (
        <>
          <div className={styles.formGrid}>
            {text("bpkbNumber", "BPKB No.", 50)}
            {text("ownerName", "Atas Nama", 100)}
            {money("objectValue", "Nilai Objek Sebesar")}
            {text("vehicleBrand", "Jenis / Merek Kendaraan", 100)}
            {/* Four digits, no thousands dots (FR-APP-021 Field). */}
            <TextField
              label="Tahun Pembuatan"
              maxLength={4}
              value={card.vehicleYear}
              onChange={(v) => set("vehicleYear", v.replace(/\D/g, ""))}
              error={error("vehicleYear")}
              disabled={readOnly}
            />
          </div>
          <Details card={card} set={set} text={text} money={money} date={date} code={code} readOnly={readOnly} error={error} />
        </>
      )}
    </div>
  );
}

// The white card of collateral details (Field F). Presentase Paripasu is
// active and required only with Status Paripasu Y; otherwise grey and
// emptied (Process 5).
function Details({
  card,
  set,
  text,
  money,
  date,
  code,
  pariPassuRequired = false,
  readOnly,
  error,
}: {
  card: CollateralCard;
  set: (key: keyof CollateralCard, v: string) => void;
  text: (key: keyof CollateralCard, label: string, max: number) => ReactNode;
  money: (key: keyof CollateralCard, label: string) => ReactNode;
  date: (key: keyof CollateralCard, label: string) => ReactNode;
  code: (key: keyof CollateralCard, label: string, optionsKey: string, placeholder?: string, required?: boolean) => ReactNode;
  pariPassuRequired?: boolean;
  readOnly: boolean;
  error: (key: string) => string | undefined;
}) {
  const pariPassu = card.pariPassuStatusCode === pariPassuYes;
  const percentDisabled = readOnly || !pariPassu;
  return (
    <div className={styles.collateralDetails}>
      {code("collateralStatusCode", "Status Agunan", "collateralStatus")}
      {text("collateralTypeText", "Jenis Agunan", 100)}
      {text("bindingType", "Jenis Pengikatan", 100)}
      {date("bindingDate", "Tanggal Pengikatan")}
      {money("njopValue", "Nilai Agunan Sesuai NJOP / Nilai Wajar")}
      {date("appraisalDate", "Tanggal Penilaian")}
      {money("kjppValue", "Nilai Agunan KJPP")}
      {date("kjppAppraisalDate", "Tanggal Penilaian KJPP")}
      {text("kjppAppraiserName", "Nama Penilaian KJPP", 100)}
      <Field label="Presentase Paripasu" required={pariPassu} error={error("pariPassuPercent")}>
        <span className={`${inputClass(percentDisabled, !!error("pariPassuPercent"))} ${styles.money}`}>
          <input
            aria-label="Presentase Paripasu"
            inputMode="numeric"
            className={`${styles.moneyInput} ${styles.percentInput}`}
            value={card.pariPassuPercent}
            disabled={percentDisabled}
            onChange={(e) => set("pariPassuPercent", e.target.value.replace(/\D/g, "").slice(0, 3))}
          />
          {card.pariPassuPercent !== "" && <span aria-hidden="true">%</span>}
        </span>
      </Field>
      {code("jointFinancingCode", "Status Agunan Join Financing", "jointFinancing")}
      {code("pariPassuStatusCode", "Status Paripasu", "pariPassuStatus", "", pariPassuRequired)}
      {code("insuredCode", "Diasuransikan ?", "insured")}
    </div>
  );
}
