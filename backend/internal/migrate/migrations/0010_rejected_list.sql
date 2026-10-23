-- Ticket 10: Rejected List (SoT §9.16) and the DHN and Rejected List
-- settings that are the same in every environment (§9.25). The DHN base URL
-- is set per environment (DB-RULE-085).

CREATE TABLE fos.rejected_list (
    rejected_id   UUID PRIMARY KEY,
    id_number     VARCHAR NOT NULL,
    full_name     VARCHAR NOT NULL,
    birth_date    DATE NOT NULL,
    status        VARCHAR NOT NULL,                                          -- ACTIVE or INACTIVE (DB-RULE-051)
    notes         TEXT,
    customer_type VARCHAR NOT NULL,
    reason        TEXT NOT NULL,                                             -- DB-RULE-052
    created_at    TIMESTAMP NOT NULL,
    created_by    VARCHAR NOT NULL REFERENCES fos.operator (user_id),
    updated_at    TIMESTAMP NOT NULL,
    updated_by    VARCHAR NOT NULL REFERENCES fos.operator (user_id),
    UNIQUE (id_number, customer_type)                                        -- DB-RULE-053
);

INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, description, updated_at, updated_by)
SELECT 'LOAN_APPLICATION', s.purpose, s.value, s.value_type, s.description, now() AT TIME ZONE 'UTC', 'system'
FROM (VALUES
    ('api/dhn/path', '/v1/customer/dhn/check/{IdentityNumber}', 'STRING', 'Path API DHN; {IdentityNumber} diganti NIK atau NPWP (FR-PSAPI-006).'),
    ('api/dhn/timeout_ms', '30000', 'INTEGER', 'Timeout panggilan DHN, milidetik.'),
    ('api/dhn/retry_max', '3', 'INTEGER', 'Maksimal retry DHN (total 4 percobaan).'),
    ('api/dhn/retry_gap_ms', '5000', 'INTEGER', 'Jarak retry DHN, milidetik.'),
    ('api/dhn/simulate', 'false', 'BOOLEAN', 'Saklar simulasi DHN.'),
    ('api/dhn/simulate_response', '{"code":"DN-00","message":"Data tidak ditemukan","data":{"blacklisted":false}}', 'JSON', 'Response simulasi DHN (data tidak ditemukan).'),
    ('secret/dhn/apikey_ref', 'DHN_APIKEY', 'STRING', 'Nama variabel environment berisi API key DHN (DEC-060).'),
    ('process/rejectedlist/query_timeout_ms', '5000', 'INTEGER', 'Batas waktu query Rejected List, milidetik (FR-PSAPI-007).')
) AS s (purpose, value, value_type, description);
