-- Ticket 12: Manual SLIK Entry after the SLIK SLA timeout or a bypass
-- (FR-PSAPI-011, FR-PSAPI-019). Agreed interim decisions on PROPOSED items:
-- the System operator, the Input Manual decision and the bypass setting.

-- The System operator records the Input Manual Recommendation (FR-PSAPI-011
-- rule 4). It cannot log in: its password hash is no bcrypt hash, so no
-- password matches, and it is not in the directory.
INSERT INTO fos.roles (role_code, role_name, is_active, created_at, updated_at)
VALUES ('System', 'System', TRUE, now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC')
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO fos.operator (user_id, password_hash, full_name, role_code, created_at, updated_at)
VALUES ('System', '!', 'System', 'System', now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC');

-- DB-RULE-065. Inactive: the Team Lead's Keputusan does not offer it; the
-- banner and history read descriptions without a status filter.
INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, status, created_at, created_by, updated_at, updated_by)
VALUES ('INPUTMANUAL', 'FAC_DECISION', '', 'Input Manual', 0, 'Inactive', now() AT TIME ZONE 'UTC', 'system', now() AT TIME ZONE 'UTC', 'system');

INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, description, updated_at, updated_by)
VALUES ('LOAN_APPLICATION', 'process/slik/bypass_enabled', 'false', 'BOOLEAN',
        'true: retry otomatis SLIK yang gagal diganti Bypass dan case lanjut ke input SLIK manual (FR-PSAPI-019).',
        now() AT TIME ZONE 'UTC', 'system');
