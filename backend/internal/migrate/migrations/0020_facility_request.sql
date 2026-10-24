-- Ticket 19: Hot Prospect Data Entry, Data Permohonan Fasilitas
-- (FR-APP-018, UI-032).

-- §9.41 Case Financing Request (ENT-053): one row per Tujuan Pembiayaan
-- line, overwritten by the form (DB-RULE-118); the UI has one line.
CREATE TABLE fos.case_financing_request (
    request_id       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id          VARCHAR NOT NULL REFERENCES fos.work (case_id),
    sort_order       INTEGER NOT NULL,
    purpose_code     VARCHAR NOT NULL,                                    -- BMV DPFHPDE code
    requested_amount NUMERIC(18, 0) NOT NULL,
    tenure_months    INTEGER NOT NULL
);

-- §9.42 Case Financing (ENT-054): totals and notes, one row per case;
-- the totals are computed by the server (DB-RULE-119).
CREATE TABLE fos.case_financing (
    case_id                   VARCHAR PRIMARY KEY REFERENCES fos.work (case_id),
    total_requested_amount    NUMERIC(18, 0) NOT NULL,
    existing_financing_amount NUMERIC(18, 0) NOT NULL,
    total_obligor             NUMERIC(18, 0) NOT NULL,
    deviation_note            VARCHAR,
    mitigation_note           VARCHAR,
    officer_notes             VARCHAR
);

-- §9.43 Case Credit Simulation (ENT-055): the latest simulation of the
-- case, overwritten (DB-RULE-120).
CREATE TABLE fos.case_credit_simulation (
    case_id               VARCHAR PRIMARY KEY REFERENCES fos.work (case_id),
    product_code          VARCHAR,
    total_obligor         NUMERIC(18, 0) NOT NULL,
    amount                NUMERIC(18, 0) NOT NULL,
    tenure_months         INTEGER NOT NULL,
    margin_type_code      VARCHAR NOT NULL,                               -- BMV MARGINTYPE code
    margin_flat           NUMERIC(8, 5) NOT NULL,
    margin_effective      NUMERIC(8, 5) NOT NULL,
    installment_flat      NUMERIC(18, 0) NOT NULL,
    installment_effective NUMERIC(18, 0) NOT NULL
);

-- §9.44 Recommendation Credit Simulation (ENT-056): the simulation submitted
-- with a Data Permohonan Fasilitas recommendation, never changed
-- (DB-RULE-121).
CREATE TABLE fos.recommendation_credit_simulation (
    simulation_id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    decision_id           BIGINT NOT NULL UNIQUE REFERENCES fos.case_pre_screening_decision (decision_id),
    purpose_code          VARCHAR NOT NULL,
    debtor_name           VARCHAR NOT NULL,
    product_code          VARCHAR,
    total_obligor         NUMERIC(18, 0) NOT NULL,
    amount                NUMERIC(18, 0) NOT NULL,
    tenure_months         INTEGER NOT NULL,
    margin_type_code      VARCHAR NOT NULL,
    margin_flat           NUMERIC(8, 5) NOT NULL,
    margin_effective      NUMERIC(8, 5) NOT NULL,
    installment_flat      NUMERIC(18, 0) NOT NULL,
    installment_effective NUMERIC(18, 0) NOT NULL
);

-- §6 "Data saat ini" (agreed comment 16): MARGINTYPE, RECOMMENDED (only
-- RECOMMEND has a known description) and RECOMMEND_LIMIT (never shown, the
-- description is the code). DPFHPDE's codes are not given, so none.
INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, status, created_at, created_by, updated_at, updated_by)
VALUES
    ('FLAT', 'MARGINTYPE', '', 'Flat', 0, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('EFFECTIVE', 'MARGINTYPE', '', 'Effective', 0, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('RECOMMEND', 'RECOMMENDED', '', 'Rekomendasi', 0, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('TURUNLMT', 'RECOMMEND_LIMIT', '', 'TURUNLMT', 0, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('SESUAILMT', 'RECOMMEND_LIMIT', '', 'SESUAILMT', 0, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system');
