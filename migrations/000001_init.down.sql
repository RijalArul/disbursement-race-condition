DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS disbursements;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;

DROP SEQUENCE IF EXISTS audit_log_id_seq;
DROP SEQUENCE IF EXISTS disbursement_id_seq;

DROP TYPE IF EXISTS idem_state;
DROP TYPE IF EXISTS disbursement_status;
DROP TYPE IF EXISTS user_role;

DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS pgcrypto;
