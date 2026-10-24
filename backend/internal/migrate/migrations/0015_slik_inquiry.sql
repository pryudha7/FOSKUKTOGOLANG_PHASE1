-- Ticket 36: the SLIKInquiry settings (§9.25, FR-PSAPI-013). The base URL
-- (api/slik/baseurl) and the key (secret/slik/apikey_ref) are SLIK's; the
-- simulated answer is the SoT's sample (agreed for ticket 36).
INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, description, updated_at, updated_by)
SELECT 'LOAN_APPLICATION', s.purpose, s.value, s.value_type, s.description, now() AT TIME ZONE 'UTC', 'system'
FROM (VALUES
    ('api/slikinquiry/path', '/v1/slik/status/{KodeReff}', 'STRING', 'Path API cek status SLIK (FR-PSAPI-013).'),
    ('api/slikinquiry/timeout_ms', '30000', 'INTEGER', 'Timeout cek status SLIK, milidetik.'),
    ('api/slikinquiry/retry_max', '0', 'INTEGER', 'Cek status SLIK tanpa retry (FR-PSAPI-013 aturan 3).'),
    ('api/slikinquiry/simulate', 'false', 'BOOLEAN', 'Saklar simulasi cek status SLIK.'),
    ('api/slikinquiry/simulate_response', '{"code":"4040","message":"data not found"}', 'JSON', 'Response simulasi cek status SLIK.')
) AS s (purpose, value, value_type, description);
