-- Ticket 14: how API Log History cuts long Request and Response on display
-- (UI-020, §9.25; PROPOSED values agreed for ticket 14).
INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, description, updated_at, updated_by)
SELECT 'LOAN_APPLICATION', s.purpose, s.value, s.value_type, s.description, now() AT TIME ZONE 'UTC', 'system'
FROM (VALUES
    ('ui/apilog/truncate_chars', '200', 'INTEGER', 'Request/Response API Log History dipotong pada jumlah karakter ini (UI-020).'),
    ('ui/apilog/truncate_lines', '5', 'INTEGER', 'Request/Response API Log History dipotong pada jumlah baris ini (UI-020).')
) AS s (purpose, value, value_type, description);
