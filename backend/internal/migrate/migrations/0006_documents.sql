-- Ticket 07: Case Document (SoT §9.7), Document Mapping Value (§9.10), Case
-- Document Category and Case Document Requirement (§9.11) and Dynamic System
-- Settings (§9.25).

CREATE TABLE fos.case_document (
    file_id            UUID PRIMARY KEY,
    case_id            VARCHAR NOT NULL REFERENCES fos.work (case_id),          -- DB-RULE-024
    -- Codename, e.g. DokumenPreScreening (DB-RULE-087; agreed for TBD-052).
    document_category  VARCHAR,
    document_type      VARCHAR,
    original_file_name VARCHAR NOT NULL,
    file_name          VARCHAR NOT NULL,
    bucket_name        VARCHAR NOT NULL,                                        -- DB-RULE-023
    object_name        VARCHAR NOT NULL,
    content_type       VARCHAR,
    file_size_bytes    BIGINT,
    created_at         TIMESTAMP NOT NULL,
    created_by         VARCHAR NOT NULL,
    UNIQUE (case_id, document_category, document_type)                         -- DB-RULE-031
);

CREATE TABLE fos.document_mapping_value (
    row_uuid     UUID PRIMARY KEY DEFAULT gen_random_uuid(),                    -- DB-RULE-032
    category     VARCHAR NOT NULL,
    sub_category VARCHAR NOT NULL,
    description  VARCHAR NOT NULL,
    notes        VARCHAR,
    sort_order   INTEGER NOT NULL,
    is_mandatory BOOLEAN NOT NULL,
    created_at   TIMESTAMP NOT NULL,
    created_by   VARCHAR NOT NULL,
    updated_at   TIMESTAMP NOT NULL,
    updated_by   VARCHAR NOT NULL,
    UNIQUE (category, sub_category, description)                                -- DB-RULE-032 (PROPOSED)
);

CREATE TABLE fos.case_document_category (
    case_id           VARCHAR NOT NULL REFERENCES fos.work (case_id),
    document_category VARCHAR NOT NULL,
    sort_order        INTEGER NOT NULL,
    source_code       VARCHAR,
    list_source       VARCHAR NOT NULL,                                         -- PRESCREENING or DOCUMENT_UPLOAD
    created_at        TIMESTAMP NOT NULL,
    PRIMARY KEY (case_id, document_category)                                    -- DB-RULE-033
);

CREATE TABLE fos.case_document_requirement (
    requirement_id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id                   VARCHAR NOT NULL REFERENCES fos.work (case_id),
    document_category         VARCHAR NOT NULL,
    document_type             VARCHAR NOT NULL,
    notes                     VARCHAR,
    sort_order                INTEGER NOT NULL,
    is_mandatory              BOOLEAN NOT NULL,
    team_lead_checked         BOOLEAN NOT NULL DEFAULT FALSE,
    financing_analyst_checked BOOLEAN NOT NULL DEFAULT FALSE,
    list_source               VARCHAR NOT NULL,
    source_row_uuid           UUID,
    created_at                TIMESTAMP NOT NULL,
    FOREIGN KEY (case_id, document_category) REFERENCES fos.case_document_category (case_id, document_category),
    UNIQUE (case_id, document_category, document_type)                         -- DB-RULE-033
);

CREATE TABLE fos.dynamic_system_settings (
    setting_id  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    app_scope   VARCHAR NOT NULL,                                               -- LOAN_APPLICATION, GOLD or GLOBAL
    purpose     VARCHAR NOT NULL,
    value       TEXT NOT NULL,
    value_type  VARCHAR NOT NULL,                                               -- STRING, INTEGER, BOOLEAN or JSON
    description TEXT,
    updated_at  TIMESTAMP NOT NULL,
    updated_by  VARCHAR NOT NULL,
    UNIQUE (app_scope, purpose)                                                 -- DB-RULE-083
);

-- The per-file limit is set per database (§9.25); 10 MB agreed for TBD-036.
INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, description, updated_at, updated_by)
VALUES ('LOAN_APPLICATION', 'validation/upload/max_file_bytes', '10485760', 'INTEGER',
        'Batas ukuran maksimum per file dalam byte (FR-FILE-002, FR-APP-004).', now() AT TIME ZONE 'UTC', 'system');

-- Master data until the import file arrives (§9.10): the Perorangan
-- category and documents of Gambar 16 (agreed for ticket 07).
INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, status, created_at, created_by, updated_at, updated_by)
VALUES ('DOCPRE-2', 'DOCUMENTPRESCREENING', 'Perorangan', 'Dokumen Pre-Screening', 1, 'Active',
        now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system');

INSERT INTO fos.document_mapping_value (category, sub_category, description, sort_order, is_mandatory, created_at, created_by, updated_at, updated_by)
SELECT 'Dokumen Pre-Screening', 'Perorangan', d.description, d.sort_order, d.is_mandatory,
       now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'
FROM (VALUES
    ('KK', 1, TRUE),
    ('KTP', 2, TRUE),
    ('Buku Nikah', 3, FALSE),
    ('Form Aplikasi Pembiayaan', 4, TRUE),
    ('Memo Usulan Pembiayaan', 5, FALSE),
    ('Laporan Kunjungan Nasabah', 6, FALSE),
    ('Form Permohonan SLIK', 7, FALSE),
    ('SLIK', 8, FALSE)
) AS d (description, sort_order, is_mandatory);
