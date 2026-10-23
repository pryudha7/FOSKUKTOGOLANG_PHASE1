-- Ticket 11: SLIK Report (SoT §9.18), SLIK Facility (§9.19), SLIK Facility
-- History (§9.20), SLIK Debtor (§9.32), SLIK Collateral (§9.33) and SLIK
-- Guarantor (§9.34); work.slik_worst_collect (agreed for TBD-039b); the RPA
-- role (agreed for TBD-044) and the SLIK settings (§9.25).
--
-- zip_document_id and pdf_document_id reference Case Document, whose key is
-- a UUID (§9.7 file_id BIGINT/UUID), so they are UUID rather than BIGINT.

CREATE TABLE fos.slik_report (
    slik_report_id                          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id                                 VARCHAR NOT NULL REFERENCES fos.work (case_id),
    subject_type                            VARCHAR NOT NULL,               -- CUSTOMER or SPOUSE
    reference_code                          VARCHAR NOT NULL UNIQUE,        -- DB-RULE-057, DB-RULE-059
    identity_number                         VARCHAR NOT NULL,
    status                                  VARCHAR NOT NULL,               -- PENDING_REQUEST, REQUESTED, RECEIVED (DB-RULE-058)
    requested_at                            TIMESTAMP,
    received_at                             TIMESTAMP,
    zip_document_id                         UUID REFERENCES fos.case_document (file_id),
    pdf_document_id                         UUID REFERENCES fos.case_document (file_id),
    source_file_name                        VARCHAR,
    inquiry_id                              VARCHAR,
    report_number                           VARCHAR,
    latest_data_position                    VARCHAR,
    result_date                             VARCHAR,
    worst_quality                           SMALLINT,
    worst_quality_data_month                CHAR(6),
    worst_collect                           SMALLINT,
    total_effective_credit_limit            NUMERIC,
    total_outstanding_principal             NUMERIC,
    inquiry_date                            DATE,
    header_inquiry_date                     DATE,
    search_parameter_birth_date             DATE,
    search_parameter_debtor_name            VARCHAR,
    search_parameter_gender                 VARCHAR,
    search_parameter_gender_description     VARCHAR,
    search_parameter_npwp                   VARCHAR,
    search_parameter_birth_place            VARCHAR,
    effective_credit_limit_credit_financing NUMERIC,
    effective_credit_limit_letter_of_credit NUMERIC,
    effective_credit_limit_guarantee_given  NUMERIC,
    effective_credit_limit_securities       NUMERIC,
    effective_credit_limit_other            NUMERIC,
    outstanding_principal_credit_financing  NUMERIC,
    outstanding_principal_letter_of_credit  NUMERIC,
    outstanding_principal_guarantee_given   NUMERIC,
    outstanding_principal_securities        NUMERIC,
    outstanding_principal_other             NUMERIC,
    creditor_public_bank                    INTEGER,
    creditor_rural_bank                     INTEGER,
    creditor_financing_institution          INTEGER,
    creditor_others                         INTEGER,
    inquiry_usage_id                        VARCHAR,
    header_created_by                       VARCHAR,
    inquiry_financial_institution_code      VARCHAR,
    inquiry_branch_code                     VARCHAR,
    inquiry_purpose_code                    VARCHAR,
    total_parts                             INTEGER,
    part_number                             INTEGER,
    summary_json                            JSONB,
    letter_of_credit_json                   JSONB,                          -- DB-RULE-111
    guarantee_given_json                    JSONB,
    other_facility_json                     JSONB,
    created_at                              TIMESTAMP NOT NULL,
    updated_at                              TIMESTAMP NOT NULL
);
CREATE INDEX slik_report_case_subject_idx ON fos.slik_report (case_id, subject_type); -- DB-RULE-059

CREATE TABLE fos.slik_facility (
    facility_id                           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slik_report_id                        BIGINT NOT NULL REFERENCES fos.slik_report (slik_report_id) ON DELETE CASCADE,
    sequence_number                       SMALLINT NOT NULL,
    financial_institution                 VARCHAR,
    financial_institution_description     VARCHAR,
    branch_description                    VARCHAR,
    account_number                        VARCHAR,
    start_contract_number                 VARCHAR,
    end_contract_number                   VARCHAR,
    credit_financing_type                 VARCHAR,
    credit_financing_type_description     VARCHAR,
    contract_description                  VARCHAR,
    start_contract_date                   DATE,
    end_contract_date                     DATE,
    start_date                            DATE,
    due_date                              DATE,
    condition_date                        DATE,
    currency_code                         VARCHAR,
    initial_credit_limit                  NUMERIC,
    credit_limit                          NUMERIC,
    outstanding_principal                 NUMERIC,
    interest_rate                         NUMERIC,
    principal_arrears                     NUMERIC,
    interest_arrears                      NUMERIC,
    penalty                               NUMERIC,
    quality                               SMALLINT,
    arrears_days                          INTEGER,
    arrears_frequency                     INTEGER,
    condition                             VARCHAR,
    condition_description                 VARCHAR,
    branch                                VARCHAR,
    credit_financing_nature               VARCHAR,
    credit_financing_contract             VARCHAR,
    debtor_category_code                  VARCHAR,
    usage_type                            VARCHAR,
    economic_sector                       VARCHAR,
    government_program_credit             VARCHAR,
    project_location                      VARCHAR,
    interest_return_rate_type             VARCHAR,
    bad_debt_cause_code                   VARCHAR,
    restructuring_method_code             VARCHAR,
    credit_financing_nature_description   VARCHAR,
    debtor_category_description           VARCHAR,
    usage_type_description                VARCHAR,
    economic_sector_description           VARCHAR,
    government_program_credit_description VARCHAR,
    project_location_description          VARCHAR,
    interest_return_rate_type_description VARCHAR,
    restructuring_description             VARCHAR,
    bad_debt_cause_description            VARCHAR,
    quality_description                   VARCHAR,
    keterangan                            VARCHAR,
    extension_frequency                   INTEGER,
    restructuring_frequency               INTEGER,
    project_value                         NUMERIC,
    current_month_realization             NUMERIC,
    value_in_original_currency            NUMERIC,
    credit_start_date                     DATE,
    bad_debt_date                         DATE,
    restructuring_end_date                DATE,
    formed_date                           DATE,
    updated_date                          DATE,
    data_period                           CHAR(6),
    detail_json                           JSONB NOT NULL
);
CREATE INDEX slik_facility_report_idx ON fos.slik_facility (slik_report_id); -- DB-RULE-061

CREATE TABLE fos.slik_facility_history (
    history_id     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    facility_id    BIGINT NOT NULL REFERENCES fos.slik_facility (facility_id) ON DELETE CASCADE,
    month_index    SMALLINT NOT NULL,
    period         CHAR(6),
    kolektabilitas SMALLINT,
    arrears_days   INTEGER,
    UNIQUE (facility_id, month_index)                                       -- DB-RULE-062
);

CREATE TABLE fos.slik_debtor (
    debtor_id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slik_report_id           BIGINT NOT NULL REFERENCES fos.slik_report (slik_report_id) ON DELETE CASCADE,
    sequence_number          SMALLINT NOT NULL,
    debtor_name              VARCHAR,
    identity                 VARCHAR,
    identity_number          VARCHAR,
    npwp                     VARCHAR,
    gender                   VARCHAR,
    gender_description       VARCHAR,
    birth_place              VARCHAR,
    birth_date               DATE,
    formed_date              DATE,
    updated_date             DATE,
    pelapor                  VARCHAR,
    reporter_description     VARCHAR,
    address                  VARCHAR,
    village                  VARCHAR,
    subdistrict              VARCHAR,
    regency_city             VARCHAR,
    regency_city_description VARCHAR,
    postal_code              VARCHAR,
    country                  VARCHAR,
    country_description      VARCHAR,
    pekerjaan                VARCHAR,
    occupation_description   VARCHAR,
    workplace                VARCHAR,
    business_field           VARCHAR,
    business_field_description VARCHAR,
    debtor_title_code        VARCHAR,
    debtor_title_status      VARCHAR,
    UNIQUE (slik_report_id, sequence_number)                                -- DB-RULE-110
);

CREATE TABLE fos.slik_collateral (
    collateral_id                                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    facility_id                                   BIGINT NOT NULL REFERENCES fos.slik_facility (facility_id) ON DELETE CASCADE,
    sequence_number                               SMALLINT NOT NULL,
    collateral_type_description                   VARCHAR,
    collateral_number                             VARCHAR,
    binding_type                                  VARCHAR,
    binding_type_description                      VARCHAR,
    collateral_value_per_financial_institution    NUMERIC,
    collateral_njop_value                         NUMERIC,
    independent_collateral_value                  NUMERIC,
    pari_passu_percentage                         NUMERIC,
    updated_date                                  DATE,
    binding_date                                  DATE,
    reporter_appraisal_date                       DATE,
    independent_appraiser_appraisal_date          DATE,
    collateral_owner_name                         VARCHAR,
    collateral_address                            VARCHAR,
    collateral_location_regency_city              VARCHAR,
    collateral_location_regency_city_description  VARCHAR,
    collateral_rating                             VARCHAR,
    rating_agency_code                            VARCHAR,
    rating_agency                                 VARCHAR,
    ownership_proof                               VARCHAR,
    independent_appraiser_name                    VARCHAR,
    asuransi                                      VARCHAR,
    keterangan                                    VARCHAR,
    UNIQUE (facility_id, sequence_number)                                   -- DB-RULE-110
);

CREATE TABLE fos.slik_guarantor (
    guarantor_id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    facility_id                BIGINT NOT NULL REFERENCES fos.slik_facility (facility_id) ON DELETE CASCADE,
    sequence_number            SMALLINT NOT NULL,
    guarantor_name             VARCHAR,
    identity_number            VARCHAR,
    guarantor_type_code        VARCHAR,
    guarantor_type_description VARCHAR,
    guarantor_address          VARCHAR,
    keterangan                 VARCHAR,
    created_date               DATE,
    updated_date               DATE,
    UNIQUE (facility_id, sequence_number)                                   -- DB-RULE-110
);

-- SLIK Worst Collect of the case (FR-PSAPI-010 rule 6; agreed for TBD-039b).
ALTER TABLE fos.work ADD COLUMN slik_worst_collect SMALLINT;

-- The RPA account calling PBSResponse has this role (agreed for TBD-044).
INSERT INTO fos.roles (role_code, role_name, is_active, created_at, updated_at)
VALUES ('RPA', 'RPA', TRUE, now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC')
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, description, updated_at, updated_by)
SELECT 'LOAN_APPLICATION', s.purpose, s.value, s.value_type, s.description, now() AT TIME ZONE 'UTC', 'system'
FROM (VALUES
    ('api/slik/path', '/v1/slik/check', 'STRING', 'Path API SLIK dan SLIK Pasangan (FR-PSAPI-008).'),
    ('api/slik/timeout_ms', '30000', 'INTEGER', 'Timeout panggilan SLIK, milidetik.'),
    ('api/slik/retry_max', '3', 'INTEGER', 'Maksimal retry SLIK (total 4 percobaan).'),
    ('api/slik/retry_gap_ms', '5000', 'INTEGER', 'Jarak retry SLIK, milidetik.'),
    ('api/slik/simulate', 'false', 'BOOLEAN', 'Saklar simulasi SLIK.'),
    ('api/slik/simulate_response', '{"code":"1100","message":"success"}', 'JSON', 'Response simulasi request SLIK.'),
    ('secret/slik/apikey_ref', 'SLIK_APIKEY', 'STRING', 'Nama variabel environment berisi API key SLIK (DEC-060).'),
    ('process/slik/wait_sla_seconds', '3600', 'INTEGER', 'Batas waktu menunggu hasil SLIK, detik (FR-PSAPI-011).')
) AS s (purpose, value, value_type, description);
