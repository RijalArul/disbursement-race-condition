ALTER TABLE audit_logs
    DROP COLUMN after,
    DROP COLUMN before,
    ADD COLUMN metadata JSONB;
