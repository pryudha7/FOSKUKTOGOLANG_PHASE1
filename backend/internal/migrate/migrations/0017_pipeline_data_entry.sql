-- Ticket 15: Pipeline Data Entry (FR-APP-004, UI-024).

-- §9.26 Product Management (ENT-037), DB-RULE-088. Its rows come from the
-- Pega table; maintenance is TBD-052h, so none are seeded. Work.product_code
-- keeps no foreign key: §9.4 marks it "FK ?" and whether a product in use may
-- be deleted is TBD-052 (DB-RULE-091).
CREATE TABLE fos.product_management (
    code                  VARCHAR PRIMARY KEY,                                  -- DB-RULE-088
    name                  VARCHAR NOT NULL,
    notes                 TEXT,
    segment_code          VARCHAR,
    financing_limit_start NUMERIC,
    financing_limit_end   NUMERIC,
    tenure_limit          INTEGER,
    contract_type         VARCHAR NOT NULL,
    created_at            TIMESTAMP NOT NULL,
    created_by            VARCHAR NOT NULL,
    updated_at            TIMESTAMP NOT NULL,
    updated_by            VARCHAR NOT NULL
);

-- BR-UI-005: the most options one autocomplete search returns (PROPOSED §9.25,
-- agreed for ticket 15).
INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, description, updated_at, updated_by)
VALUES ('LOAN_APPLICATION', 'ui/autocomplete/max_results', '50', 'INTEGER',
        'Jumlah maksimum opsi per pencarian autocomplete; pencarian langsung ke database (BR-UI-005).',
        now() AT TIME ZONE 'UTC', 'system');

-- The Bukti Surat dari Bank document type (FR-APP-004 rule 6, DB-RULE-090),
-- the §9.10 example row.
INSERT INTO fos.document_mapping_value (category, sub_category, description, sort_order, is_mandatory, created_at, created_by, updated_at, updated_by)
VALUES ('Dokumen Inisiasi', 'TO', 'Bukti Surat Take Over dari Bank', 1, FALSE,
        now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system');
