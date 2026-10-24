"use client";

import { useState } from "react";
import type { AssignmentRef } from "@/lib/api";
import { hotProspectSteps } from "./Breadcrumb";
import { CodeSelect, save, useMove } from "./FormFields";
import { HotProspectCard, leave } from "./HotProspectData";
import type { QCAPage } from "./types";
import styles from "./case.module.css";

// The ten fixed QCA questions (FR-APP-020 Field), as Gambar 79 words them;
// question n is answered from BMV QCA<n> (DEC-150).
const questions = [
  "Bagaimana hubungan pemilik dengan Bank Nano Syariah?",
  "Apa tujuan dari pembiayaan tersebut?",
  "Berapa lama usaha telah beroperasi?",
  "Bagaimana penampilan tempat tinggal pemohon?",
  "Bagaimana kondisi keseluruhan lokasi usaha?",
  "Apakah pemilik memiliki sertifikat atas lokasi usahanya?",
  "Jika tempat usaha adalah sewa, berapa persen biaya sewa dari pendapatan bulanan?",
  "Bagaimana jarak lokasi usaha/bisnis pemohon dengan cabang?",
  "Apa saja sumber pemasukan dalam keluarga selain dari pemasukan usaha pemohon?",
  "Berapa pemasukan pemilik usaha yang digunakan untuk biaya rumah tangga?",
];

// qcaKey is the field and options key of question i (from 0).
const qcaKey = (i: number) => `qca${i + 1}`;

// UI-033 QCA Data (Gambar 79, FR-APP-020): ten required dropdowns. Back
// leaves as Close does; Back to Previous Step saves and opens Data
// Permohonan Fasilitas (agreed comment 2).
export default function QCAData({ page, assignment }: { page: QCAPage; assignment: AssignmentRef }) {
  const readOnly = !page.editable;
  const [answers, setAnswers] = useState<string[]>(page.values.answers);
  const flow = useMove(assignment, "qca");
  const body = () => ({ answers });
  const set = (i: number, code: string) => {
    setAnswers((all) => all.map((a, j) => (j === i ? code : a)));
    flow.clear("form", qcaKey(i));
  };

  return (
    <HotProspectCard
      title="QCA Data"
      step={3}
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
              <button type="button" className={styles.back} disabled={flow.busy} onClick={() => flow.move(hotProspectSteps[2], body())}>
                Back to Previous Step
              </button>
              <button type="button" className={styles.next} disabled={flow.busy} onClick={() => flow.move(hotProspectSteps[4], body())}>
                Next
              </button>
            </div>
          )}
        </>
      }
    >
      <h3 className={styles.sectionTitle}>QCA Data</h3>
      <div className={styles.formPanel}>
        {questions.map((q, i) => {
          const key = qcaKey(i);
          return (
            <CodeSelect
              key={key}
              label={q}
              placeholder="Select"
              required
              options={page.options[key] ?? []}
              failedItem={page.failedOptions.includes(key)}
              value={answers[i]}
              onChange={(v) => set(i, v)}
              error={flow.errorOf("form", key)}
              disabled={readOnly}
            />
          );
        })}
      </div>
    </HotProspectCard>
  );
}
