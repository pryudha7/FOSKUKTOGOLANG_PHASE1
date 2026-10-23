-- Ticket 30: customer (SoT §9.28), every party of a case in one table:
-- Nasabah, Badan Usaha, Pasangan, Keluarga (one row each per case) and
-- Pengurus (one row per officer). Columns unused by a customer_type stay
-- NULL; required-ness is enforced by each form, not NOT NULL (DB-RULE-104).

CREATE TABLE fos.customer (
    customer_id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id                      VARCHAR NOT NULL REFERENCES fos.work (case_id),
    customer_type                VARCHAR NOT NULL,
    sort_order                   INTEGER,
    id_number                    VARCHAR,
    full_name                    VARCHAR,
    birth_date                   DATE,
    establishment_date           DATE,
    role_code                    VARCHAR,
    is_existing_customer         BOOLEAN,
    birth_place                  VARCHAR,
    id_lifetime                  BOOLEAN,
    nationality                  VARCHAR,
    npwp                         VARCHAR,
    email                        VARCHAR,
    gender_code                  VARCHAR,
    marital_status_code          VARCHAR,
    marital_status_pbs_code      VARCHAR,
    dependents_code              VARCHAR,
    mother_maiden_name           VARCHAR,
    mailing_address_code         VARCHAR,
    same_address_as_ektp         BOOLEAN,
    job_code                     VARCHAR,
    education_code               VARCHAR,
    workplace                    VARCHAR,
    workplace_address            VARCHAR,
    work_duration                VARCHAR,
    gross_income_yearly          NUMERIC(18, 0),
    income_source_code           VARCHAR,
    workplace_sector_code        VARCHAR,
    workplace_business_type_code VARCHAR,
    relationship_with_bank_code  VARCHAR,
    customer_category_code       VARCHAR,
    prenup_code                  VARCHAR,
    mobile_phone                 VARCHAR,
    relationship_code            VARCHAR,
    business_entity_code         VARCHAR,
    place_of_establishment       VARCHAR,
    deed_number                  VARCHAR,
    last_deed_number             VARCHAR,
    last_deed_date               DATE,
    economic_sector_code         VARCHAR,
    business_type_code           VARCHAR,
    go_public                    VARCHAR,
    business_group_name          VARCHAR
);

CREATE INDEX customer_case_id_idx ON fos.customer (case_id);

-- §9.28: one row per case for Nasabah, Badan Usaha, Pasangan and Keluarga.
CREATE UNIQUE INDEX customer_one_per_case_idx ON fos.customer (case_id, customer_type)
    WHERE customer_type <> 'Pengurus';
