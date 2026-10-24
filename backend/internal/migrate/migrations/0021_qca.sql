-- Ticket 20: Hot Prospect Data Entry, QCA Data (FR-APP-020, UI-033).

-- §9.47 Case QCA Answer (ENT-059): one row per answered question; Next
-- replaces the case's rows with all ten (DB-RULE-126). The questions are
-- fixed and not stored; BMV QCA1 to QCA10 hold the answers (none seeded:
-- the SoT gives no options).
CREATE TABLE fos.case_qca_answer (
    case_id         VARCHAR NOT NULL REFERENCES fos.work (case_id),
    question_number SMALLINT NOT NULL,
    answer_code     VARCHAR NOT NULL,                                     -- BMV QCA<question_number> code
    PRIMARY KEY (case_id, question_number)
);
