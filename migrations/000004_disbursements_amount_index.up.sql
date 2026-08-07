-- sort_by menerima created_at dan amount (whitelist di orderClause). created_at
-- sudah ter-index sejak 000001; amount belum, sehingga ORDER BY amount menyortir
-- seluruh baris hidup setiap request.
--
-- Partial (WHERE deleted_at IS NULL) mengikuti index disbursements lainnya:
-- baris yang sudah di-soft-delete tidak pernah muncul di query manapun, jadi
-- tidak perlu ikut menghuni index.
CREATE INDEX idx_disbursements_amount ON disbursements(amount) WHERE deleted_at IS NULL;
