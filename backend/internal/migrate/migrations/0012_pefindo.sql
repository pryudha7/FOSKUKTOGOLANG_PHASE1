-- Ticket 13: Pefindo Report (SoT §9.22), Pefindo Scoring (§9.23),
-- Integration Token (§9.24) and Pefindo Scoring Reason (§9.40); the Pefindo
-- settings (§9.25). The report's other children (§9.35-9.39) are ticket 35.

-- The Pefindo API running inside a Pefindo step; attempt counts per API
-- (PROPOSED, agreed for ticket 13). NULL on steps that are one API.
ALTER TABLE fos.prescreening_runs ADD COLUMN current_api VARCHAR;

CREATE TABLE fos.pefindo_report (
    pefindo_report_id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id                     VARCHAR NOT NULL REFERENCES fos.work (case_id),
    subject_type                VARCHAR NOT NULL,               -- CUSTOMER or SPOUSE
    event_id                    VARCHAR NOT NULL,
    inquiry_id                  VARCHAR NOT NULL,
    id_pefindo                  VARCHAR NOT NULL,
    report_code                 VARCHAR NOT NULL,               -- 01 or 36
    report_json                 JSONB,                          -- personal data (DB-RULE-077)
    latest_period               DATE,                           -- DB-RULE-076
    latest_risk_grade           VARCHAR,
    latest_score                INTEGER,
    header_username             VARCHAR,
    header_id_report            VARCHAR,
    header_inquiry_date         TIMESTAMP,
    management_json             JSONB,                          -- DB-RULE-114
    other_data_json             JSONB,
    other_data_source_json      JSONB,
    debtor_history_summary_json JSONB,
    current_contract_json       JSONB,
    inquiries_analysis_json     JSONB,
    past_due_information_json   JSONB,
    repayment_information_json  JSONB,
    received_at                 TIMESTAMP NOT NULL,
    created_at                  TIMESTAMP NOT NULL,
    updated_at                  TIMESTAMP NOT NULL,
    UNIQUE (case_id, subject_type)                              -- DB-RULE-074
);

CREATE TABLE fos.pefindo_scoring (
    pefindo_scoring_id     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pefindo_report_id      BIGINT NOT NULL REFERENCES fos.pefindo_report (pefindo_report_id),
    id_pefindo             VARCHAR,
    period                 DATE NOT NULL,
    score                  INTEGER,
    risk_grade             VARCHAR,
    risk_grade_description VARCHAR,
    risk_description       VARCHAR,
    probability_of_default NUMERIC
);

CREATE INDEX pefindo_scoring_report_period ON fos.pefindo_scoring (pefindo_report_id, period DESC); -- DB-RULE-078

CREATE TABLE fos.pefindo_scoring_reason (
    reason_id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pefindo_scoring_id BIGINT NOT NULL REFERENCES fos.pefindo_scoring (pefindo_scoring_id),
    sequence_number    SMALLINT NOT NULL,
    reason_code        VARCHAR,
    reason_description VARCHAR,
    UNIQUE (pefindo_scoring_id, sequence_number)                -- DB-RULE-115
);

CREATE TABLE fos.integration_token (
    provider         VARCHAR PRIMARY KEY,
    token_ciphertext BYTEA,                                     -- AES-256-GCM (DEC-099)
    token_nonce      BYTEA,
    key_version      VARCHAR,
    obtained_at      TIMESTAMP,
    updated_at       TIMESTAMP NOT NULL
);

-- One row per provider, locked when a token is written (DB-RULE-080).
INSERT INTO fos.integration_token (provider, updated_at) VALUES ('PEFINDO', now() AT TIME ZONE 'UTC');

-- api/pefindo/baseurl and product_id differ per environment and are set on
-- each environment's database (DB-RULE-085); simulate_response is agreed
-- for TBD-042j.
INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, description, updated_at, updated_by)
SELECT 'LOAN_APPLICATION', s.purpose, s.value, s.value_type, s.description, now() AT TIME ZONE 'UTC', 'system'
FROM (VALUES
    ('api/pefindo/path', '/api/v1/product/search', 'STRING', 'Path API Pefindo Search (FR-PSAPI-014).'),
    ('api/pefindo/timeout_ms', '30000', 'INTEGER', 'Timeout Pefindo Search, milidetik.'),
    ('api/pefindo/retry_max', '3', 'INTEGER', 'Maksimal retry Pefindo Search (total 4 percobaan).'),
    ('api/pefindo/retry_gap_ms', '5000', 'INTEGER', 'Jarak retry Pefindo Search, milidetik.'),
    ('api/pefindo/simulate', 'false', 'BOOLEAN', 'Saklar simulasi Pefindo Search.'),
    ('api/pefindo/simulate_response', '{"code":"31","status":"Success","message":"Data tidak ditemukan","data":[]}', 'JSON', 'Response simulasi Pefindo Search (data tidak ditemukan).'),
    ('secret/pefindo/apikey_ref', 'PEFINDO_APIKEY', 'STRING', 'Nama variabel environment berisi x-gateway-apikey Pefindo (DEC-060).'),
    ('api/pefindotoken/path', '/api/v1/getToken', 'STRING', 'Path API getToken Pefindo (FR-PSAPI-015).'),
    ('api/pefindotoken/timeout_ms', '30000', 'INTEGER', 'Timeout getToken Pefindo, milidetik.'),
    ('secret/pefindotoken/authorization_ref', 'PEFINDO_TOKEN_AUTHORIZATION', 'STRING', 'Nama variabel environment berisi Authorization getToken Pefindo (DEC-060).'),
    ('secret/encryption/integration_token_key_ref', 'INTEGRATION_TOKEN_KEY', 'STRING', 'Nama variabel environment berisi kunci AES-256-GCM token integrasi, base64 32 byte (DEC-099).'),
    ('api/pefindogeneratereport/path', '/api/v1/product/generateReport', 'STRING', 'Path API Pefindo Generate Report (FR-PSAPI-016).'),
    ('api/pefindogeneratereport/timeout_ms', '30000', 'INTEGER', 'Timeout Pefindo Generate Report, milidetik.'),
    ('api/pefindogeneratereport/retry_max', '3', 'INTEGER', 'Maksimal retry Pefindo Generate Report.'),
    ('api/pefindogeneratereport/retry_gap_ms', '5000', 'INTEGER', 'Jarak retry Pefindo Generate Report, milidetik.'),
    ('api/pefindogeneratereport/simulate', 'false', 'BOOLEAN', 'Saklar simulasi Pefindo Generate Report.'),
    ('api/pefindogeneratereport/simulate_response', '{"code":"01","status":"Success","message":"Proses membuat report sedang dikerjakan"}', 'JSON', 'Response simulasi Pefindo Generate Report.'),
    ('api/pefindogeneratereport/language', '01', 'STRING', 'Field language Pefindo Generate Report (FR-PSAPI-016).'),
    ('process/pefindo/generate_to_download_gap_ms', '3500', 'INTEGER', 'Jarak setelah Generate Report sebelum Download PDF, milidetik (FR-PSAPI-016).'),
    ('api/pefindodownloadpdf/path', '/api/v1/product/downloadPdfReport/eventId/{EventID}', 'STRING', 'Path API Pefindo Download PDF (FR-PSAPI-018).'),
    ('api/pefindodownloadpdf/timeout_ms', '60000', 'INTEGER', 'Timeout Pefindo Download PDF, milidetik.'),
    ('api/pefindodownloadpdf/retry_max', '3', 'INTEGER', 'Maksimal retry Pefindo Download PDF.'),
    ('api/pefindodownloadpdf/retry_gap_ms', '60000', 'INTEGER', 'Jarak retry Pefindo Download PDF, milidetik (code 32).'),
    ('api/pefindodownloadpdf/simulate', 'false', 'BOOLEAN', 'Saklar simulasi Pefindo Download PDF.'),
    ('api/pefindodownloadpdf/simulate_response', '{"code":"31","status":"Success","message":"Proses membuat report sedang dikerjakan"}', 'JSON', 'Response simulasi Pefindo Download PDF (tanpa PDF).'),
    ('api/pefindogetreport/path', '/api/v1/product/getReport/eventId/{EventID}', 'STRING', 'Path API Pefindo Get Report (FR-PSAPI-017).'),
    ('api/pefindogetreport/timeout_ms', '30000', 'INTEGER', 'Timeout Pefindo Get Report, milidetik.'),
    ('api/pefindogetreport/retry_max', '3', 'INTEGER', 'Maksimal retry Pefindo Get Report.'),
    ('api/pefindogetreport/retry_gap_ms', '60000', 'INTEGER', 'Jarak retry Pefindo Get Report, milidetik (code 32).'),
    ('api/pefindogetreport/simulate', 'false', 'BOOLEAN', 'Saklar simulasi Pefindo Get Report.'),
    ('api/pefindogetreport/simulate_response', '{"code":"31","status":"Success","message":"Data tidak ditemukan"}', 'JSON', 'Response simulasi Pefindo Get Report (data tidak ditemukan).')
) AS s (purpose, value, value_type, description);
