-- Ticket 08: Case Pre-Screening Decision, the Recommendation history (SoT
-- §9.12). Append-only (DB-RULE-036). The Data Permohonan Fasilitas columns
-- stay NULL on Pre Screening rows (DB-RULE-122).

CREATE TABLE fos.case_pre_screening_decision (
    decision_id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id                      VARCHAR NOT NULL REFERENCES fos.work (case_id),
    recommendation_type          VARCHAR NOT NULL,
    decision_code                VARCHAR,                                   -- BMV FAC_DECISION code
    notes                        VARCHAR NOT NULL,
    decided_by                   VARCHAR NOT NULL,                          -- DB-RULE-116
    decided_by_role_code         VARCHAR NOT NULL,                          -- snapshot, DB-RULE-117
    decided_at                   TIMESTAMP NOT NULL,
    recommendation_status        VARCHAR,
    limit_action_code            VARCHAR,
    total_obligor                NUMERIC(18,0),
    requested_amount             NUMERIC(18,0),
    recommended_amount           NUMERIC(18,0),
    approved_amount              NUMERIC(18,0),
    requested_margin_flat        NUMERIC(8,5),
    requested_margin_effective   NUMERIC(8,5),
    recommended_margin_flat      NUMERIC(8,5),
    recommended_margin_effective NUMERIC(8,5),
    approved_margin_flat         NUMERIC(8,5),
    approved_margin_effective    NUMERIC(8,5),
    requested_tenure             INTEGER,
    recommended_tenure           INTEGER,
    approved_tenure              INTEGER
);

-- FR-APP-010 "Data saat ini"; sort_order is empty there, 0 here (NOT NULL),
-- so the options order by code.
INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, status, created_at, created_by, updated_at, updated_by)
VALUES
    ('SESUAI', 'FAC_DECISION', '', 'Sesuai', 0, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('TIDAKSESUAI', 'FAC_DECISION', '', 'Tidak Sesuai', 0, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system');
