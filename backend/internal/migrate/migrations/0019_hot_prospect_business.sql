-- Ticket 18: Hot Prospect Data Entry, Data Usaha Nasabah and Data
-- Pembiayaan Lain (FR-APP-016, FR-APP-017).

-- §9.30 Customer Business (ENT-042): the applicant's business, one row per
-- applicant customer (DB-RULE-106). The business address is customer_address
-- type "Usaha" (DB-RULE-105). customer_id is BIGINT as customer's key.
CREATE TABLE fos.customer_business (
    customer_id            BIGINT PRIMARY KEY REFERENCES fos.customer (customer_id),
    business_field         VARCHAR NOT NULL,
    business_category_code VARCHAR NOT NULL,
    business_age_months    INTEGER NOT NULL,
    employee_count         INTEGER NOT NULL,
    business_location_code VARCHAR NOT NULL
);

-- §9.31 Other Financing (ENT-043): zero or more rows per case, one per
-- card, replaced as a whole by Next (DB-RULE-107); no column is required and
-- nothing refers back to slik_facility (DB-RULE-108).
CREATE TABLE fos.other_financing (
    other_financing_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id            VARCHAR NOT NULL REFERENCES fos.work (case_id),
    sort_order         INTEGER NOT NULL,
    bank_name          VARCHAR,
    financing_type     VARCHAR,
    financing_amount   NUMERIC(18, 0),
    outstanding_amount NUMERIC(18, 0),
    tenor_months       INTEGER,
    margin_rate        NUMERIC(5, 2),
    slik_collect       SMALLINT,
    purpose            VARCHAR
);

CREATE INDEX other_financing_case_id_idx ON fos.other_financing (case_id);
