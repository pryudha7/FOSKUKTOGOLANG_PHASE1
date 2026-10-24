-- Ticket 21: Hot Prospect Data Entry, Collateral Data (FR-APP-021, UI-034).

-- §9.48 Case Collateral (ENT-060): one row per card, Agunan Utama (MAIN)
-- and Jaminan Pendukung (SUPPORT) together; Next and Save replace the
-- case's rows (DB-RULE-127, agreed comment 3). Columns not applying to the
-- collateral type, or hidden by Tipe Bukti Kepemilikan, stay NULL.
CREATE TABLE fos.case_collateral (
    collateral_id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id                VARCHAR NOT NULL REFERENCES fos.work (case_id),
    collateral_section     VARCHAR NOT NULL,                             -- MAIN or SUPPORT
    sequence_number        SMALLINT NOT NULL,
    collateral_type_code   VARCHAR NOT NULL,                             -- BMV MAINCOLLATERAL or COLLATERALSUPPORT
    building_form_code     VARCHAR,                                      -- BMV BENTUKTANAHBANGUNAN
    ownership_type_code    VARCHAR,                                      -- BMV OWNERSHIPTYPE
    owner_name             VARCHAR(100),
    shm_number             VARCHAR(50),
    shgb_number            VARCHAR(50),
    bpkb_number            VARCHAR(50),
    fiduciary_number       VARCHAR(50),
    shgb_valid_until       DATE,
    location               VARCHAR(255),
    land_area              VARCHAR(50),
    due_date               DATE,
    physical_value         NUMERIC(18, 0),
    object_value           NUMERIC(18, 0),
    deposit_amount         NUMERIC(18, 0),
    vehicle_brand          VARCHAR(100),
    vehicle_year           SMALLINT,
    remarks                VARCHAR(500),
    collateral_status_code VARCHAR,                                      -- BMV COLLATERAL_STATUS
    collateral_type_text   VARCHAR(100),
    binding_type           VARCHAR(100),
    binding_date           DATE,
    appraisal_date         DATE,
    kjpp_appraisal_date    DATE,
    njop_value             NUMERIC(18, 0),
    kjpp_value             NUMERIC(18, 0),
    kjpp_appraiser_name    VARCHAR(100),
    pari_passu_status_code VARCHAR,                                      -- BMV STATUS_PARIPASU
    pari_passu_percent     SMALLINT,                                     -- 0 when not Y; NULL on Deposito
    joint_financing_code   VARCHAR,                                      -- BMV COLLATERAL_JOINT_ACCOUNT
    insured_code           VARCHAR,                                      -- BMV COLLATERAL_ASURANSI
    UNIQUE (case_id, collateral_section, sequence_number)
);

-- §9.25 settings of FR-APP-021 (agreed comments 5 and 6).
INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, description, updated_at, updated_by)
VALUES
    ('LOAN_APPLICATION', 'validation/collateral/max_cards_per_section', '10', 'INTEGER',
     'Jumlah maksimum kartu pada seksi Agunan Utama dan pada seksi Jaminan Pendukung, masing-masing.', now() AT TIME ZONE 'UTC', 'system'),
    ('LOAN_APPLICATION', 'validation/collateral/main_required_above_amount', '100000000', 'INTEGER',
     'Batas Total Requested Amount dalam rupiah; lebih dari nilai ini, Collateral Data wajib memiliki minimal satu Agunan Utama.',
     now() AT TIME ZONE 'UTC', 'system');

-- §6 "Data saat ini" (agreed comment 9): the collateral types (description
-- = code), Tipe Bukti Kepemilikan and Status Paripasu.
INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, status, created_at, created_by, updated_at, updated_by)
VALUES
    ('Tanah dan Bangunan', 'MAINCOLLATERAL', '', 'Tanah dan Bangunan', 1, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('Deposito Bank Nano Syariah', 'MAINCOLLATERAL', '', 'Deposito Bank Nano Syariah', 2, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('Tanah Kosong Produktif', 'COLLATERALSUPPORT', '', 'Tanah Kosong Produktif', 1, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('Kendaraan', 'COLLATERALSUPPORT', '', 'Kendaraan', 2, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('1', 'OWNERSHIPTYPE', '', 'SHM', 1, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('2', 'OWNERSHIPTYPE', '', 'SHGB', 2, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('3', 'OWNERSHIPTYPE', '', 'BPKB', 3, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('4', 'OWNERSHIPTYPE', '', 'SHMRS', 4, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('5', 'OWNERSHIPTYPE', '', 'Akta Jaminan Fidusia', 5, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('Y', 'STATUS_PARIPASU', '', 'Ya', 1, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system'),
    ('T', 'STATUS_PARIPASU', '', 'Tidak', 2, 'Active', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system');
