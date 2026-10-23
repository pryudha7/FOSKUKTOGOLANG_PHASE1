"use client";

import { useState } from "react";
import type { LifecycleStage, StageItem } from "./types";
import styles from "./case.module.css";

// FR-CASE-004 Stage Bar: current stage bold, entered stages with a check;
// clicking a stage opens the Case lifecycle popup (FR-CASE-007, UI-014).
export default function StageBar({ stages, lifecycle }: { stages: StageItem[]; lifecycle: LifecycleStage[] }) {
  const [open, setOpen] = useState(false);
  return (
    <div className={styles.stageBarWrap}>
      <ol className={styles.stageBar}>
        {stages.map((s) => (
          <li key={s.name} className={styles.stage}>
            <button
              type="button"
              className={`${styles.stageButton} ${s.current ? styles.stageCurrent : ""}`}
              aria-current={s.current ? "step" : undefined}
              title={s.name}
              onClick={() => setOpen(true)}
            >
              {s.entered && <CheckIcon />}
              <span className={styles.stageLabel}>{s.name}</span>
            </button>
          </li>
        ))}
      </ol>
      {open && <LifecyclePopup lifecycle={lifecycle} onClose={() => setOpen(false)} />}
    </div>
  );
}

function LifecyclePopup({ lifecycle, onClose }: { lifecycle: LifecycleStage[]; onClose: () => void }) {
  return (
    <div className={styles.lifecycle} role="dialog" aria-labelledby="lifecycle-title">
      <div className={styles.lifecycleHeader}>
        <h3 id="lifecycle-title" className={styles.lifecycleTitle}>
          Case lifecycle
        </h3>
        <button type="button" className={styles.lifecycleClose} aria-label="Close" onClick={onClose}>
          ×
        </button>
      </div>
      <div className={styles.lifecycleColumns}>
        {lifecycle.map((s) => (
          <section key={s.stage} className={styles.lifecycleColumn}>
            <h4 className={styles.lifecycleStage}>
              {s.entered && <CheckIcon />}
              {s.stage}
            </h4>
            <ul className={styles.processes}>
              {s.processes.map((p, i) => (
                <li key={`${p.name}-${i}`} className={styles.process}>
                  <span className={p.started ? styles.processStarted : styles.processPending} aria-hidden="true">
                    {p.started ? "✓" : ""}
                  </span>
                  {p.name}
                  {p.started && <span className="sr-only"> (started)</span>}
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </div>
  );
}

function CheckIcon() {
  return (
    <svg className={styles.check} width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#444b52" strokeWidth="3" aria-label="entered">
      <path d="M5 12l5 5 9-10" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
