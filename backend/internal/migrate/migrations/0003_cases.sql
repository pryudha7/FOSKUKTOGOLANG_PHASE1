-- Ticket 05: cases (SoT §9.4), assignments (§9.5), Business Mapping Value
-- (§9.6) and case lifecycle (§9.9). Primary keys BIGINT identity (TBD-001).

-- FR-CASE-001: one sequence per prefix, starting at 1; gaps are tolerated (TBD-022).
CREATE SEQUENCE fos.case_id_lf_seq START 1;
CREATE SEQUENCE fos.case_id_pv_seq START 1;

CREATE TABLE fos.work (
    case_id                  VARCHAR PRIMARY KEY,                             -- DB-RULE-008
    case_type                VARCHAR NOT NULL,
    parent_case_id           VARCHAR REFERENCES fos.work (case_id),           -- DB-RULE-009
    case_status              VARCHAR,
    current_stage            VARCHAR,
    branch_code              VARCHAR,
    urgency_work_class       INTEGER NOT NULL,
    branch_officer           VARCHAR NOT NULL,
    branch_officer_team_lead VARCHAR,
    is_check_bypassed        BOOLEAN NOT NULL DEFAULT FALSE,
    customer_code            VARCHAR,
    memo_number              VARCHAR,
    submission_type          VARCHAR,
    product_code             VARCHAR,                                          -- FK to product_management (§9.26) once it exists
    contract_type            VARCHAR,
    bank_name                VARCHAR,
    -- Not in §9.4: Tipe Nasabah as the BMV CUSTOMER_TYPE description (§9.6 usage
    -- table; agreed for ticket 05). Filled on submit of Pre-Screening Submission.
    customer_type            VARCHAR,
    created_at               TIMESTAMP NOT NULL,
    updated_at               TIMESTAMP NOT NULL,
    created_by               VARCHAR,
    updated_by               VARCHAR
);

CREATE TABLE fos.assignments (
    assignment_id     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id           VARCHAR NOT NULL REFERENCES fos.work (case_id),          -- DB-RULE-013
    assignment_name   VARCHAR NOT NULL,                                         -- no UNIQUE with case_id (DB-RULE-014)
    assignment_type   VARCHAR NOT NULL,
    assigned_user_id  VARCHAR,
    work_queue_id     BIGINT REFERENCES fos.work_queues (work_queue_id),
    assignment_status VARCHAR NOT NULL,
    created_at        TIMESTAMP NOT NULL,
    first_opened_at   TIMESTAMP,
    worked_by_user_id VARCHAR,
    completed_at      TIMESTAMP,
    -- DB-RULE-011
    CONSTRAINT assignments_target_chk CHECK (
        (assignment_type <> 'worklist' OR assigned_user_id IS NOT NULL) AND
        (assignment_type <> 'work queue' OR work_queue_id IS NOT NULL))
);

CREATE INDEX assignments_case_id_idx ON fos.assignments (case_id);

CREATE TABLE fos.business_mapping_value (
    code        VARCHAR NOT NULL,
    type        VARCHAR NOT NULL,
    sub_type    VARCHAR NOT NULL,                                               -- '' when unused (DB-RULE-021)
    description VARCHAR NOT NULL,
    sort_order  INTEGER NOT NULL,
    value1      VARCHAR,
    value2      VARCHAR,
    status      VARCHAR NOT NULL DEFAULT 'Active',                              -- DB-RULE-017
    created_at  TIMESTAMP NOT NULL,
    created_by  VARCHAR NOT NULL,
    updated_at  TIMESTAMP NOT NULL,
    updated_by  VARCHAR NOT NULL,
    PRIMARY KEY (type, sub_type, code)                                          -- DB-RULE-021
);

CREATE TABLE fos.lifecycle_process (
    process_id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    stage_name   VARCHAR NOT NULL,
    process_name VARCHAR NOT NULL,
    sort_order   INT NOT NULL,
    UNIQUE (stage_name, sort_order)
);

CREATE TABLE fos.case_stage_progress (
    case_id            VARCHAR NOT NULL REFERENCES fos.work (case_id),          -- DB-RULE-029
    stage_name         VARCHAR NOT NULL,
    first_entered_at   TIMESTAMP NOT NULL,
    entered_from_stage VARCHAR,
    PRIMARY KEY (case_id, stage_name)
);

CREATE TABLE fos.case_process_progress (
    case_id          VARCHAR NOT NULL REFERENCES fos.work (case_id),
    process_id       BIGINT NOT NULL REFERENCES fos.lifecycle_process (process_id),
    first_started_at TIMESTAMP NOT NULL,
    PRIMARY KEY (case_id, process_id)
);

-- DB-RULE-030: processes per stage as listed in UI-014 (WF-006 to WF-008: TBD-034).
INSERT INTO fos.lifecycle_process (stage_name, process_name, sort_order)
SELECT v.stage, v.process, v.ord FROM (VALUES
    ('Financing Submission', 'Pre-Screening Submission', 1),
    ('Financing Submission', 'Pipeline Data Entry', 2),
    ('Financing Submission', 'Badan Usaha', 3),
    ('Financing Submission', 'Mapping Value Scoring DataSet', 4),
    ('Financing Submission', 'Data Pribadi', 5),
    ('Financing Submission', 'Data Pasangan', 6),
    ('Financing Submission', 'Data Keluarga Dekat', 7),
    ('Financing Submission', 'Mapping Value Scoring DataSet', 8),
    ('Financing Application', 'Set Initial Data', 1),
    ('Financing Application', 'Data Usaha Nasabah', 2),
    ('Financing Application', 'Data Pembiayaan Lain', 3),
    ('Financing Application', 'Data Permohonan Fasilitas', 4),
    ('Financing Application', 'QCA Data', 5),
    ('Financing Application', 'Collateral Data', 6),
    ('Financing Application', 'Document Upload', 7),
    ('Financing Application', 'Mapping Value Scoring DataSet', 8),
    ('Financing Application', 'Sub Survey Account Officer', 9),
    ('Financing Application', 'Mapping Value Scoring DataSet', 10),
    ('Financing Application', 'RAC Check', 11),
    ('Financing Application', 'Appraisal', 12),
    ('Financing Application', 'Proceed Changes Stage From FAC', 13),
    ('Financing Checker', 'Financing Checker', 1),
    ('Financing Checker', 'Scoring Check', 2),
    ('Financing Checker', 'Proceed Changes Stage From FAC', 3),
    ('Survey Analyst', 'Create Phone Verification case', 1),
    ('Survey Analyst', 'Sub Survey Analyst', 2),
    ('Survey Analyst', 'Wait', 3),
    ('Recommendation', 'FA Recommendation', 1),
    ('Recommendation', 'Committee Recommendation', 2)
) AS v (stage, process, ord);

-- BMV CUSTOMER_TYPE as given in §9.6 (the full BMV data set is imported
-- separately; BRANCH rows come with it).
INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, status, created_at, created_by, updated_at, updated_by)
VALUES
    ('Perorangan', 'CUSTOMER_TYPE', '', 'Perorangan', 2, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('Badan Usaha', 'CUSTOMER_TYPE', '', 'Badan Usaha', 2, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system');
