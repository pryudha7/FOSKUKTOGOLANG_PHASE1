-- Ticket 09: Connect API Log (SoT §9.14) and Prescreening Run (§9.15), and
-- the Dukcapil settings that are the same in every environment (§9.25).

CREATE TABLE fos.connect_api_log (
    log_id                UUID PRIMARY KEY,
    case_id               VARCHAR REFERENCES fos.work (case_id),
    api_type              VARCHAR NOT NULL,
    api_name              VARCHAR,
    submitted_at          TIMESTAMP NOT NULL,
    api_status            VARCHAR NOT NULL,
    request               TEXT,                                               -- DB-RULE-042: kept whole
    response              TEXT,
    http_method           VARCHAR,
    request_url           TEXT,
    http_status_code      INTEGER,
    request_headers       JSONB,                                              -- DB-RULE-066: secrets masked
    response_headers      JSONB,
    client_ip             VARCHAR,
    duration_milliseconds INTEGER,
    attempt_number        SMALLINT,
    auth_username         VARCHAR,
    event_id              VARCHAR,
    pefindo_id            VARCHAR,
    inquiry_id            VARCHAR
);

-- DB-RULE-043, DB-RULE-069
CREATE INDEX connect_api_log_case_api_idx ON fos.connect_api_log (case_id, api_name, submitted_at DESC);
CREATE INDEX connect_api_log_case_time_idx ON fos.connect_api_log (case_id, submitted_at);
CREATE INDEX connect_api_log_time_idx ON fos.connect_api_log (submitted_at DESC);

CREATE TABLE fos.prescreening_runs (
    run_id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id          VARCHAR NOT NULL REFERENCES fos.work (case_id),
    state            VARCHAR NOT NULL,
    current_step     VARCHAR NOT NULL,
    attempt          INTEGER NOT NULL,
    owner_id         VARCHAR,
    heartbeat_at     TIMESTAMP NOT NULL,
    wait_deadline_at TIMESTAMP,
    started_at       TIMESTAMP NOT NULL,
    updated_at       TIMESTAMP NOT NULL
);

-- DB-RULE-046
CREATE UNIQUE INDEX prescreening_runs_one_active_idx ON fos.prescreening_runs (case_id)
    WHERE state IN ('RUNNING', 'WAITING_SLIK');

-- §9.25 catalog values for Dukcapil (FR-PSAPI-002, FR-PSAPI-003). The base
-- URL differs per environment and is set in each database (DB-RULE-085).
-- x-Gateway-APIKey is added to the masked headers (DEC-060; agreed for
-- ticket 09); simulate_response is agreed for TBD-042.
INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, description, updated_at, updated_by)
SELECT 'LOAN_APPLICATION', s.purpose, s.value, s.value_type, s.description, now() AT TIME ZONE 'UTC', 'system'
FROM (VALUES
    ('api/dukcapil/path', '/internal/kyc/dukcapil/v1.0/checkEktp', 'STRING', 'Path API Dukcapil (FR-PSAPI-003).'),
    ('api/dukcapil/timeout_ms', '30000', 'INTEGER', 'Timeout panggilan Dukcapil, milidetik.'),
    ('api/dukcapil/retry_max', '3', 'INTEGER', 'Maksimal retry Dukcapil (total 4 percobaan).'),
    ('api/dukcapil/retry_gap_ms', '5000', 'INTEGER', 'Jarak retry Dukcapil, milidetik.'),
    ('api/dukcapil/simulate', 'false', 'BOOLEAN', 'Saklar simulasi Dukcapil.'),
    ('api/dukcapil/simulate_response', '{"NIK":"Sesuai (100)","NAMA_LGKP":"Sesuai (100)","TGL_LHR":"Sesuai","NO_KK":"Sesuai","TMPT_LHR":"Sesuai (100)","PROP_NAME":"Sesuai","KAB_NAME":"Sesuai","KEC_NAME":"Sesuai","KEL_NAME":"Sesuai","NO_RT":"Sesuai","NO_RW":"Sesuai","ALAMAT":"Sesuai (100)","NIK_IBU":"Sesuai","NAMA_LGKP_IBU":"Sesuai (100)","AGAMA":"Sesuai","STATUS_KAWIN":"Sesuai","JENIS_PKRJN":"Sesuai","JENIS_KLMIN":"Sesuai","PDDK_AKH":"Sesuai","NO_PROP":"Sesuai","NO_KAB":"Sesuai","NO_KEC":"Sesuai","NO_KEL":"Sesuai"}', 'JSON', 'Response simulasi Dukcapil (data ditemukan).'),
    ('api/log/masked_headers', 'x-api-key, Authorization, Proxy-Authorization, Cookie, Set-Cookie, x-Gateway-APIKey', 'STRING', 'Header yang disamarkan pada Connect API Log (DB-RULE-066).'),
    ('secret/dukcapil/apikey_ref', 'DUKCAPIL_APIKEY', 'STRING', 'Nama variabel environment berisi API key Dukcapil (DEC-060).')
) AS s (purpose, value, value_type, description);
