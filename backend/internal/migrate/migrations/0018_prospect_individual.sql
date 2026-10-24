-- Ticket 17: Prospect Data Entry Perorangan (FR-APP-012 to FR-APP-014).

-- §9.27 Registered Customers and Existing Customers (ENT-038, ENT-039):
-- read to tell a new customer from an existing one (FR-APP-005 rule 1,
-- DB-RULE-095). Neither refers to a case (DEC-135); no rows are seeded.
CREATE TABLE fos.registered_customers (
    id_number     VARCHAR NOT NULL,
    customer_type VARCHAR NOT NULL,
    full_name     VARCHAR NOT NULL,
    birth_date    DATE,
    notes         TEXT,
    created_at    TIMESTAMP NOT NULL,
    created_by    VARCHAR NOT NULL,
    updated_at    TIMESTAMP NOT NULL,
    updated_by    VARCHAR NOT NULL,
    PRIMARY KEY (id_number, customer_type)                                      -- DB-RULE-095
);

CREATE TABLE fos.existing_customers (
    id_number     VARCHAR NOT NULL,
    customer_type VARCHAR NOT NULL,
    full_name     VARCHAR NOT NULL,
    birth_date    DATE,
    notes         TEXT,
    created_at    TIMESTAMP NOT NULL,
    created_by    VARCHAR NOT NULL,
    updated_at    TIMESTAMP NOT NULL,
    updated_by    VARCHAR NOT NULL,
    PRIMARY KEY (id_number, customer_type)                                      -- DB-RULE-094, DB-RULE-095
);

-- §9.29 Customer Address (ENT-041): one row per customer and address type
-- ("e-KTP", "Domisili", "Badan Usaha", "Kantor", "Keluarga", "Usaha").
-- customer_id is BIGINT as customer's key. Columns unused by a type stay NULL.
CREATE TABLE fos.customer_address (
    customer_id          BIGINT NOT NULL REFERENCES fos.customer (customer_id),
    address_type         VARCHAR NOT NULL,
    address              VARCHAR,
    phone                VARCHAR,
    office_phone         VARCHAR,
    mobile_phone         VARCHAR,
    rt                   VARCHAR,
    rw                   VARCHAR,
    years_of_stay        SMALLINT,
    months_of_stay       SMALLINT,
    house_ownership_code VARCHAR,
    province_code        VARCHAR,
    city_code            VARCHAR,
    regency_city_code    VARCHAR,
    district_code        VARCHAR,
    subdistrict_code     VARCHAR,
    postal_code          VARCHAR,
    PRIMARY KEY (customer_id, address_type)
);
