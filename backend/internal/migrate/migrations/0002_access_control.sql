-- Ticket 03: access control master data (SoT §9.3) and per-session login path.
-- access_groups is not specified in §9.3 although role_access_mappings
-- references it; it follows the work_groups pattern (agreed).

CREATE TABLE fos.access_groups (
    access_group_id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    access_group_code VARCHAR NOT NULL UNIQUE,
    access_group_name VARCHAR NOT NULL,
    is_active         BOOLEAN NOT NULL,
    created_at        TIMESTAMP NOT NULL,
    updated_at        TIMESTAMP NOT NULL
);

CREATE TABLE fos.work_groups (
    work_group_id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    work_group_code VARCHAR NOT NULL UNIQUE,
    work_group_name VARCHAR NOT NULL,
    is_active       BOOLEAN NOT NULL,
    created_at      TIMESTAMP NOT NULL,
    updated_at      TIMESTAMP NOT NULL
);

CREATE TABLE fos.work_queues (
    work_queue_id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    work_queue_code VARCHAR NOT NULL UNIQUE,
    work_queue_name VARCHAR NOT NULL,
    work_group_id   BIGINT REFERENCES fos.work_groups (work_group_id), -- TBD in §9.3
    is_active       BOOLEAN NOT NULL,
    created_at      TIMESTAMP NOT NULL,
    updated_at      TIMESTAMP NOT NULL
);

CREATE TABLE fos.role_access_mappings (
    mapping_id      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    role_id         BIGINT NOT NULL REFERENCES fos.roles (role_id),
    access_group_id BIGINT NOT NULL REFERENCES fos.access_groups (access_group_id),
    work_group_id   BIGINT NOT NULL REFERENCES fos.work_groups (work_group_id),
    work_queue_id   BIGINT REFERENCES fos.work_queues (work_queue_id),      -- required? TBD-015
    is_active       BOOLEAN NOT NULL,
    created_at      TIMESTAMP NOT NULL,
    updated_at      TIMESTAMP NOT NULL
);

-- §9.3: at most one active mapping per role.
CREATE UNIQUE INDEX role_access_mappings_one_active_per_role_idx
    ON fos.role_access_mappings (role_id) WHERE is_active;

-- The login path a session came from, so the UI can return there (TBD-013, agreed).
ALTER TABLE fos.operator_session ADD COLUMN auth_method VARCHAR NOT NULL DEFAULT 'DATABASE';
ALTER TABLE fos.operator_session ALTER COLUMN auth_method DROP DEFAULT;

-- Initial master data. Codes equal names: the SoT defines no code convention.
INSERT INTO fos.access_groups (access_group_code, access_group_name, is_active, created_at, updated_at)
SELECT v.n, v.n, TRUE, now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC'
FROM (VALUES ('Authors'), ('Admin'), ('User')) AS v (n);                 -- §3.2

INSERT INTO fos.roles (role_code, role_name, is_active, created_at, updated_at)
SELECT v.code, v.name, TRUE, now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC'
FROM (VALUES
    ('Financing Admin & Service Dept Head', 'Financing Admin & Service Dept Head'),
    ('Financing Admin & Control Dept Head', 'Financing Admin & Control Dept Head'),
    ('Financing management Support Dept Head', 'Financing management Support Dept Head'),
    ('Relationship Consumer Financing Staff', 'Relationship Consumer Financing Staff'),
    ('Sales Consumer Financing Officer', 'Sales Consumer Financing Officer'),
    ('Consumer Financing Dept Head', 'Consumer Financing Dept Head'),
    ('Financing System & Tools', 'Financing System & Tools'),
    ('FADVMaker', 'FADVMaker'),
    ('Identity Management SectHead', 'Identity Management SectHead'),
    ('Identity Management Associate Analyst', 'Identity Management Associate Analyst'),
    ('KORWIL', 'KORWIL'),
    ('FAMD', 'FAMD'),
    ('FADV', 'FADV'),
    ('SPVFACVer', 'SPVFACVer'),
    ('SPVFACProc', 'SPVFACProc'),
    ('FACCustodyChecker', 'FACCustodyChecker'),
    ('FACCustody', 'FACCustody'),
    ('TL', 'TL'),
    ('RFGH', 'RFGH'),
    ('PD', 'PD'),
    ('FD', 'FD'),
    ('BD', 'BD'),
    ('PODH', 'PODH'),
    ('PODV', 'PODV'),
    ('BM', 'BM'),
    ('Committee', 'Committee'),
    ('UTKPChecker', 'UTKPChecker'),
    ('UTKP', 'UTKP'),
    ('SAO', 'SAO'),
    ('FAO/FADHMaker', 'FAO/FADHMaker'),
    ('FADH/FADVMaker', 'FADH/FADVMaker'),
    ('FADV/FAGHChecker', 'FADV/FAGHChecker'),
    ('FAO', 'FAO'),
    ('FA', 'FA'),
    ('FADH', 'FADH'),
    ('FAGH', 'FAGH'),
    ('FACProc', 'FACProc'),
    ('FACVer', 'FACVer'),
    ('AO', 'AO')
) AS v (code, name)                                                       -- §3.3
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO fos.work_groups (work_group_code, work_group_name, is_active, created_at, updated_at)
SELECT v.code, v.name, TRUE, now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC'
FROM (VALUES
    ('Default', 'Default'),
    ('FACProc', 'FACProc'),
    ('FACCustody', 'FACCustody'),
    ('FACVer', 'FACVer'),
    ('Identity Management', 'Identity Management'),
    ('AO', 'AO'),
    ('UTKP', 'UTKP'),
    ('Committee', 'Committee'),
    ('FA', 'FA'),
    ('Financing System & Tools', 'Financing System & Tools')
) AS v (code, name);                                                      -- §3.4

-- §3.6: the only complete mapping. BM's work queue, and the PODV/PODH access
-- groups, are TBD, so no other mapping exists.
INSERT INTO fos.role_access_mappings (role_id, access_group_id, work_group_id, work_queue_id, is_active, created_at, updated_at)
SELECT r.role_id, ag.access_group_id, wg.work_group_id, NULL, TRUE, now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC'
FROM fos.roles r, fos.access_groups ag, fos.work_groups wg
WHERE r.role_code = 'BM' AND ag.access_group_code = 'User' AND wg.work_group_code = 'Committee';
