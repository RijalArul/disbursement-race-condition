ALTER TABLE audit_logs
    DROP COLUMN metadata,
    ADD COLUMN before JSONB,
    ADD COLUMN after JSONB;
