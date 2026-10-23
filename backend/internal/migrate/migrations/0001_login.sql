-- Ticket 01: roles (SoT §9.3), operator (ENT-005; agreed columns, TBD-018),
-- operator_session (agreed; not in SoT §9) and login_log (SoT §9.21).
-- Primary keys are BIGINT identity pending TBD-001. Timestamps are UTC (§9.2).

CREATE TABLE fos.roles (
    role_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    role_code  VARCHAR NOT NULL UNIQUE,
    role_name  VARCHAR NOT NULL,
    is_active  BOOLEAN NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    created_by VARCHAR,
    updated_by VARCHAR
);

CREATE TABLE fos.operator (
    user_id       VARCHAR PRIMARY KEY,
    password_hash VARCHAR NOT NULL,
    full_name     VARCHAR NOT NULL,
    role_code     VARCHAR NOT NULL REFERENCES fos.roles (role_code), -- DB-RULE-001: exactly one role
    branch_code   VARCHAR,
    report_to     VARCHAR,                                            -- User ID of the Team Lead
    timezone      VARCHAR NOT NULL DEFAULT 'GMT+7',                   -- format per ENT-005 (TBD-030 open)
    mobile_access BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMP NOT NULL,
    updated_at    TIMESTAMP NOT NULL
);

-- Usernames match case-insensitively (TBD-012, agreed).
CREATE UNIQUE INDEX operator_user_id_lower_idx ON fos.operator (lower(user_id));

CREATE TABLE fos.operator_session (
    session_id       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash       BYTEA NOT NULL UNIQUE,                           -- SHA-256 of the cookie token
    user_id          VARCHAR NOT NULL REFERENCES fos.operator (user_id),
    endpoint         VARCHAR NOT NULL,                                -- DESKTOP or MOBILE
    created_at       TIMESTAMP NOT NULL,
    last_activity_at TIMESTAMP NOT NULL
);

CREATE TABLE fos.login_log (
    login_log_id     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    login_id         VARCHAR NOT NULL,
    operator_user_id VARCHAR REFERENCES fos.operator (user_id),
    auth_method      VARCHAR NOT NULL,
    result           VARCHAR NOT NULL,
    fail_reason      VARCHAR,
    error_code       VARCHAR,
    client_ip        VARCHAR,
    user_agent       TEXT,
    attempted_at     TIMESTAMP NOT NULL
);

-- DB-RULE-070
CREATE INDEX login_log_login_id_attempted_at_idx ON fos.login_log (login_id, attempted_at DESC);
CREATE INDEX login_log_attempted_at_idx ON fos.login_log (attempted_at DESC);
