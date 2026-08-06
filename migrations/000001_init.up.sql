CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE user_role AS ENUM ('superadmin', 'admin', 'operator');
CREATE TYPE disbursement_status AS ENUM ('PENDING', 'APPROVED', 'REJECTED', 'FAILED');
CREATE TYPE idem_state AS ENUM ('PROCESSING', 'COMPLETED');

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(64) NOT NULL UNIQUE,
    password_hash VARCHAR(60) NOT NULL,
    role          user_role NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    token_hash CHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

CREATE SEQUENCE disbursement_id_seq;
CREATE SEQUENCE audit_log_id_seq;

CREATE TABLE disbursements (
    id             VARCHAR(16) PRIMARY KEY
                       DEFAULT 'DSB-' || lpad(nextval('disbursement_id_seq')::text, 6, '0'),
    recipient_name VARCHAR(255) NOT NULL,
    account_number VARCHAR(64) NOT NULL,
    amount         BIGINT NOT NULL CHECK (amount >= 10000),
    admin_fee      BIGINT NOT NULL DEFAULT 0,
    status         disbursement_status NOT NULL DEFAULT 'PENDING',
    created_by     UUID NOT NULL REFERENCES users(id),
    approved_by    UUID REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);

CREATE INDEX idx_disbursements_status ON disbursements(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_disbursements_created_by ON disbursements(created_by) WHERE deleted_at IS NULL;
CREATE INDEX idx_disbursements_created_at ON disbursements(created_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_disbursements_recipient_name_trgm ON disbursements USING GIN (recipient_name gin_trgm_ops) WHERE deleted_at IS NULL;

CREATE TABLE idempotency_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    idempotency_key VARCHAR(64) NOT NULL,
    request_hash    CHAR(64) NOT NULL,
    state           idem_state NOT NULL DEFAULT 'PROCESSING',
    response_status INT,
    response_body   BYTEA,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '24 hours'),
    UNIQUE (user_id, idempotency_key)
);

CREATE TABLE audit_logs (
    id          VARCHAR(16) PRIMARY KEY
                    DEFAULT 'LOG-' || lpad(nextval('audit_log_id_seq')::text, 6, '0'),
    actor_id    UUID NOT NULL REFERENCES users(id),
    action      VARCHAR(64) NOT NULL,
    entity_type VARCHAR(64) NOT NULL,
    entity_id   VARCHAR(64) NOT NULL,
    metadata    JSONB,
    request_id  UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
