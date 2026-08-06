-- DSB-01 needs two columns that 000001 did not create: bank_code (a mandatory
-- input) and note (optional free text).
--
-- bank_code is stored verbatim. It is not validated against a bank registry and
-- carries no foreign key — the disbursement records what the caller supplied,
-- and verifying that the code names a real, reachable bank belongs to the
-- payment rail, not to this table.
--
-- The column is added NOT NULL with a transient DEFAULT so existing rows get a
-- value; the default is then dropped so every future INSERT must supply one.
-- On an empty table this is equivalent to a plain NOT NULL add.
ALTER TABLE disbursements ADD COLUMN bank_code VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE disbursements ALTER COLUMN bank_code DROP DEFAULT;

ALTER TABLE disbursements ADD COLUMN note TEXT;
