# Disbursement API

API disbursement dengan idempotency, approval yang aman terhadap race condition, soft delete, audit trail terpisah, dan structured logging dengan propagasi request ID.

Keputusan desain dan trade-off-nya dibahas di [ARCHITECTURE.md](ARCHITECTURE.md).

---

## Stack

| Komponen | Pilihan | Alasan |
|---|---|---|
| Bahasa | Go 1.26 | — |
| HTTP framework | Gin | ekosistem middleware paling matang, routing dan binding ringkas |
| Database | PostgreSQL 16 | `SELECT ... FOR UPDATE`, partial index, `pg_trgm` untuk partial search |
| ORM | GORM | produktif untuk CRUD, tetap memungkinkan raw SQL dan locking clause saat dibutuhkan |
| Migration | golang-migrate | migration berupa SQL mentah, sehingga index dan constraint eksplisit di repo |
| Logging | `log/slog` (stdlib) | JSON handler tersedia di standard library, tidak menambah dependency |
| JWT | `golang-jwt/jwt/v5` | implementasi standar, mendukung validasi algoritma eksplisit |
| Validasi | `go-playground/validator` | terintegrasi dengan binding Gin |
| Testing | `testify` | assertion ringkas; dependency di-fake lewat interface, bukan mock generator |
| Dokumentasi | `swaggo/swag` | anotasi di handler, spec ikut ter-review bersama kode |

Dependency tambahan sengaja dijaga minimal. Rate limiting, worker pool, dan idempotency ditulis sendiri karena logikanya kecil dan menjadi bagian dari hal yang dinilai.

---

## Menjalankan

### Docker (disarankan)

```bash
cp .env.example .env
docker compose up --build
```

Compose menjalankan PostgreSQL, menunggu healthcheck-nya lolos, menjalankan migration, menjalankan seed, lalu menyalakan API. Tidak ada langkah manual di antaranya.

API tersedia di `http://localhost:8080`, Swagger UI di `http://localhost:8080/swagger/index.html`. Klik "Authorize" dan masukkan `Bearer <access_token>` untuk mencoba endpoint yang butuh autentikasi langsung dari UI.

File di `docs/` (`docs.go`, `swagger.json`, `swagger.yaml`) hasil generate dari anotasi swaggo di handler — jangan diedit manual. Setelah mengubah anotasi atau menambah endpoint, jalankan `make swagger` lalu commit ulang `docs/`.

### Lokal

Butuh Go 1.26 dan PostgreSQL 16 yang sudah berjalan.

```bash
cp .env.example .env      # sesuaikan kredensial database
make migrate-up
make seed
make run
```

### Perintah lain

```bash
make test          # unit test
make test-race     # unit test dengan race detector
make migrate-down  # rollback migration
make swagger       # regenerate dokumentasi OpenAPI
```

---

## Konfigurasi

Seluruh konfigurasi lewat environment variable. Aplikasi gagal saat boot dengan pesan eksplisit kalau variabel wajib kosong — tidak ada nilai default yang diam-diam dipakai untuk hal sensitif.

| Variabel | Default | Keterangan |
|---|---|---|
| `APP_PORT` | `8080` | |
| `APP_ENV` | `development` | memengaruhi format log dan detail error |
| `DB_HOST` | — | wajib |
| `DB_PORT` | `5432` | |
| `DB_USER` | — | wajib |
| `DB_PASSWORD` | — | wajib |
| `DB_NAME` | — | wajib |
| `DB_SSLMODE` | `disable` | |
| `DB_MAX_OPEN_CONNS` | `25` | |
| `DB_MAX_IDLE_CONNS` | `5` | |
| `DB_CONN_MAX_LIFETIME` | `5m` | |
| `JWT_SECRET` | — | wajib, aplikasi menolak boot kalau kosong |
| `JWT_ACCESS_TTL` | `15m` | |
| `JWT_REFRESH_TTL` | `168h` | 7 hari |
| `IDEMPOTENCY_TTL` | `24h` | |
| `AUDIT_WORKER_COUNT` | `4` | |
| `AUDIT_BUFFER_SIZE` | `1024` | |
| `RATE_LIMIT_CREATE` | `30` | per menit per user |
| `RATE_LIMIT_DEFAULT` | `120` | per menit per user |
| `RATE_LIMIT_LOGIN_IP` | `10` | per menit per IP, khusus `POST /auth/login` |
| `RATE_LIMIT_LOGIN_USERNAME` | `5` | per menit per username, khusus `POST /auth/login` |
| `LOG_LEVEL` | `info` | |

---

## Kredensial Seed

| Username | Password | Role |
|---|---|---|
| `superadmin` | `superadmin123` | superadmin |
| `admin` | `admin123` | admin |
| `operator` | `operator123` | operator |

Kredensial ini ada di repo karena ditentukan oleh spesifikasi test. Untuk deployment sungguhan, seed harus mengambil nilai dari environment dan password awal wajib diganti pada login pertama.

---

## Role

| Kemampuan | operator | admin | superadmin |
|---|:--:|:--:|:--:|
| Membuat disbursement | ✅ | ✅ | ✅ |
| Melihat disbursement | ✅ | ✅ | ✅ |
| Mengubah status | ❌ | ✅ | ✅ |
| Menghapus (soft delete) | ❌ | ❌ | ✅ |
| Melihat audit log | ❌ | ❌ | ✅ |

Role yang tidak mencukupi dijawab `403`, bukan `401`. `401` khusus untuk masalah autentikasi: token tidak ada, malformed, signature salah, atau kedaluwarsa.

---

## Format Response

Seluruh endpoint memakai envelope yang sama.

```json
// sukses
{ "success": true, "data": { }, "meta": { } }

// gagal
{ "success": false, "error": { "code": "CONFLICT", "message": "..." } }
```

Setiap response membawa header `X-Request-ID`.

---

## Endpoint

### Autentikasi

| Method | Path | Role | Keterangan |
|---|---|---|---|
| POST | `/auth/login` | publik | menukar kredensial dengan access + refresh token |
| POST | `/auth/refresh` | publik | menukar refresh token dengan access token baru (dengan rotasi) |
| POST | `/auth/logout` | publik | mencabut refresh token |

Semua endpoint di luar `/auth/*` dan `/health` memerlukan header `Authorization: Bearer <access_token>`.

<details>
<summary><code>POST /auth/login</code></summary>

```bash
curl -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'
```

```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOi...",
    "refresh_token": "9f2c1a...",
    "token_type": "Bearer",
    "expires_in": 900
  }
}
```

Kredensial salah dijawab `401` dengan pesan generik. Pesannya sengaja tidak membedakan "user tidak ditemukan" dari "password salah", agar endpoint ini tidak bisa dipakai untuk menebak username yang terdaftar.
</details>

<details>
<summary><code>POST /auth/refresh</code></summary>

```bash
curl -X POST http://localhost:8080/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"9f2c1a..."}'
```

Refresh token lama dicabut dan token baru diterbitkan pada setiap pemakaian. Refresh token yang sudah dicabut lalu dipakai lagi memicu pencabutan seluruh token milik user tersebut, karena itu indikasi token bocor.
</details>

<details>
<summary><code>POST /auth/logout</code></summary>

```bash
curl -X POST http://localhost:8080/auth/logout \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"9f2c1a..."}'
```

Idempotent — logout dua kali tetap `200`. Access token yang sudah terbit tetap berlaku sampai kedaluwarsa; lihat ARCHITECTURE.md untuk alasannya.
</details>

### Disbursement

| Method | Path | Role | Keterangan |
|---|---|---|---|
| GET | `/disbursements` | semua | daftar dengan filter, pencarian, sorting, pagination |
| GET | `/disbursements/:id` | semua | detail |
| POST | `/disbursements` | semua | mendukung `Idempotency-Key` |
| POST | `/disbursements/batch` | semua | partial success |
| PATCH | `/disbursements/:id/status` | admin, superadmin | concurrency-safe |
| DELETE | `/disbursements/:id` | superadmin | soft delete |

<details>
<summary><code>GET /disbursements</code></summary>

| Parameter | Tipe | Ketentuan |
|---|---|---|
| `page` | number | default `1` |
| `limit` | number | default `20`, maksimum `100` |
| `search` | string | partial match pada `recipient_name` |
| `status` | string | `PENDING` \| `APPROVED` \| `REJECTED` |
| `date_from` | date | `YYYY-MM-DD` |
| `date_to` | date | `YYYY-MM-DD` |
| `sort_by` | string | `created_at` \| `amount` (default `created_at`) |
| `sort_order` | string | `asc` \| `desc` (default `desc`) |

```bash
curl -G http://localhost:8080/disbursements \
  -H "Authorization: Bearer $TOKEN" \
  -d 'status=PENDING' -d 'search=budi' -d 'sort_by=amount' -d 'limit=20'
```

```json
{
  "success": true,
  "data": [ ],
  "meta": { "page": 1, "limit": 20, "total": 284, "total_pages": 15 }
}
```

`sort_by` dan `sort_order` divalidasi terhadap whitelist, tidak pernah disambung langsung ke SQL. Baris yang sudah di-soft-delete tidak pernah muncul.
</details>

<details>
<summary><code>POST /disbursements</code></summary>

```bash
curl -X POST http://localhost:8080/disbursements \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000' \
  -d '{
    "recipient_name": "Budi Santoso",
    "account_number": "1234567890",
    "bank_code": "BCA",
    "amount": 1250000,
    "note": "Pembayaran supplier"
  }'
```

Aturan:

- `recipient_name`, `account_number`, `bank_code`, `amount` wajib
- `amount` bilangan bulat positif, minimal `10000`
- Status awal selalu `PENDING`
- `admin_fee` dihitung otomatis: `2500` bila `amount < 5.000.000`, `5000` bila `amount >= 5.000.000`
- `created_by` diambil dari token, bukan dari body

Header `Idempotency-Key` opsional. Tanpa header, endpoint tetap berfungsi normal tanpa jaminan idempotency.
</details>

<details>
<summary><code>PATCH /disbursements/:id/status</code></summary>

```bash
curl -X PATCH http://localhost:8080/disbursements/DSB-000001/status \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"status":"APPROVED","note":"Sudah diverifikasi"}'
```

| Kondisi | Status |
|---|---|
| Berhasil | `200` |
| Role bukan admin/superadmin | `403` |
| `status` selain `APPROVED`/`REJECTED` | `400` |
| Disbursement sudah `APPROVED`/`REJECTED` | `409` |
| Tidak ditemukan atau sudah dihapus | `404` |

`approved_by` diambil dari token. Detail mekanisme locking ada di ARCHITECTURE.md.
</details>

<details>
<summary><code>DELETE /disbursements/:id</code></summary>

```bash
curl -X DELETE http://localhost:8080/disbursements/DSB-000001 \
  -H "Authorization: Bearer $SUPERADMIN_TOKEN"
```

Hanya superadmin, dan hanya untuk disbursement berstatus `PENDING` (`409` jika bukan). Baris tidak dihapus dari database — hanya `deleted_at` yang diisi.
</details>

### Audit log

| Method | Path | Role |
|---|---|---|
| GET | `/audit-logs` | superadmin |

| Parameter | Tipe | Ketentuan |
|---|---|---|
| `page` | number | default `1` |
| `limit` | number | default `20`, maksimum `100` |
| `entity_id` | string | exact match |
| `action` | string | `created` \| `status_changed` \| `deleted` |
| `date_from` | date | `YYYY-MM-DD` |
| `date_to` | date | `YYYY-MM-DD` |
| `sort_by` | string | `created_at` (satu-satunya kolom yang diizinkan) |
| `sort_order` | string | `asc` \| `desc` (default `desc`) |

Envelope `meta` sama persis dengan `GET /disbursements`. `sort_by`/`sort_order` divalidasi terhadap whitelist yang sama seperti endpoint lain, tidak pernah disambung langsung ke SQL.

```json
{
  "id": "LOG-000001",
  "entity_id": "DSB-000001",
  "action": "status_changed",
  "actor": "33333333-3333-3333-3333-333333333333",
  "before": { "status": "PENDING", "note": null },
  "after": { "status": "APPROVED", "note": "Sudah diverifikasi" },
  "created_at": "2025-06-12T08:00:00Z"
}
```

`actor` adalah `user_id` (UUID) dari JWT, apa adanya — tidak di-resolve menjadi username, supaya `GET /audit-logs` tidak menambah query join di setiap pemanggilan.

Aksi yang dicatat: `created`, `status_changed`, `deleted`. Untuk `created`, `before` selalu `null` (tidak ada state sebelumnya). Operasi baca dan aktivitas autentikasi tidak masuk tabel ini — cukup di structured log — agar tabel audit tetap kecil dan query-nya tetap cepat.

### Lain-lain

| Method | Path | Role |
|---|---|---|
| GET | `/health` | publik |
| GET | `/swagger/index.html` | publik |

`/health` mengembalikan `503` bila database tidak terjangkau.

---

## Schema Database

```sql
CREATE TYPE user_role           AS ENUM ('operator','admin','superadmin');
CREATE TYPE disbursement_status AS ENUM ('PENDING','APPROVED','REJECTED');
CREATE TYPE idem_state          AS ENUM ('PROCESSING','COMPLETED');

CREATE TABLE users (
  id            BIGSERIAL    PRIMARY KEY,
  username      VARCHAR(50)  NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  role          user_role    NOT NULL,
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
  id         BIGSERIAL   PRIMARY KEY,
  user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash CHAR(64)    NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rt_user_active ON refresh_tokens(user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_rt_expires     ON refresh_tokens(expires_at);

CREATE SEQUENCE disbursement_seq;
CREATE TABLE disbursements (
  id             VARCHAR(20)  PRIMARY KEY
                 DEFAULT 'DSB-' || lpad(nextval('disbursement_seq')::text, 6, '0'),
  recipient_name VARCHAR(255) NOT NULL,
  account_number VARCHAR(50)  NOT NULL,
  bank_code      VARCHAR(20)  NOT NULL,
  amount         BIGINT       NOT NULL CHECK (amount >= 10000),
  admin_fee      BIGINT       NOT NULL,
  note           TEXT,
  status         disbursement_status NOT NULL DEFAULT 'PENDING',
  created_by     VARCHAR(50)  NOT NULL,
  approved_by    VARCHAR(50),
  created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
  deleted_at     TIMESTAMPTZ
);
CREATE INDEX idx_dsb_created ON disbursements(created_at DESC)         WHERE deleted_at IS NULL;
CREATE INDEX idx_dsb_amount  ON disbursements(amount)                  WHERE deleted_at IS NULL;
CREATE INDEX idx_dsb_status  ON disbursements(status, created_at DESC) WHERE deleted_at IS NULL;

CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_dsb_recipient ON disbursements USING gin (recipient_name gin_trgm_ops);

CREATE TABLE idempotency_keys (
  id              BIGSERIAL   PRIMARY KEY,
  user_id         BIGINT      NOT NULL REFERENCES users(id),
  idempotency_key UUID        NOT NULL,
  request_hash    CHAR(64)    NOT NULL,
  state           idem_state  NOT NULL DEFAULT 'PROCESSING',
  response_status INT,
  response_body   JSONB,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at      TIMESTAMPTZ NOT NULL,
  UNIQUE (user_id, idempotency_key)
);
CREATE INDEX idx_idem_expires ON idempotency_keys(expires_at);

CREATE SEQUENCE audit_log_id_seq;
CREATE TABLE audit_logs (
  id          VARCHAR(16) PRIMARY KEY
              DEFAULT 'LOG-' || lpad(nextval('audit_log_id_seq')::text, 6, '0'),
  actor_id    UUID        NOT NULL REFERENCES users(id),
  action      VARCHAR(64) NOT NULL,
  entity_type VARCHAR(64) NOT NULL,
  entity_id   VARCHAR(64) NOT NULL,
  before      JSONB,
  after       JSONB,
  request_id  UUID        NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_entity      ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_created_at  ON audit_logs(created_at);
```

Penjelasan tiap keputusan index ada di [ARCHITECTURE.md](ARCHITECTURE.md#skema-database). Ringkasnya:

- Index pada `disbursements` bersifat **partial** (`WHERE deleted_at IS NULL`) karena soft delete membuat baris mati menumpuk permanen.
- `recipient_name` memakai **GIN trigram**, karena `ILIKE '%term%'` tidak bisa memakai btree.
- `amount` bertipe `BIGINT` — nilai uang selalu integer, tidak pernah float.
- Idempotency key unik **per user**, bukan global.

---

## Structured Logging

Setiap request menghasilkan satu baris log JSON:

```json
{
  "level": "info",
  "timestamp": "2025-06-12T08:00:00Z",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "method": "PATCH",
  "path": "/disbursements/DSB-000001/status",
  "status_code": 200,
  "latency_ms": 42,
  "user": "admin"
}
```

`request_id` dibuat di middleware, disimpan di `context.Context`, dan dikembalikan ke client lewat header `X-Request-ID`. Seluruh method service dan repository menerima `context.Context` sebagai argumen pertama — itulah jalur propagasinya, sehingga log dari handler, service, dan repository dalam satu request membawa `request_id` yang sama.

Kolom `request_id` juga disimpan di `audit_logs`, sehingga sebuah baris audit bisa ditelusuri balik ke request HTTP yang memicunya.

Password, token, dan header `Authorization` tidak pernah masuk ke log.

---

## Testing

```bash
make test
make test-race
```

Test berfokus pada logika bisnis kritis, bukan mengejar angka coverage. Dependency di-fake lewat interface, bukan mock yang sekadar memverifikasi bahwa sebuah fungsi terpanggil.

Cakupan:

- **Kalkulasi `admin_fee`** — termasuk boundary `5.000.000` dari kedua sisi, dan penolakan `amount` di bawah `10000`
- **Transisi status** — setiap kombinasi transisi yang valid dan tidak valid, termasuk disbursement yang sudah di-soft-delete
- **RBAC** — matriks role terhadap setiap aksi
- **Idempotency handler** — key baru, replay, in-flight, body berbeda dengan key sama, key kedaluwarsa, kegagalan service di tengah, serta key milik user lain
- **Token** — access token kedaluwarsa, signature salah, refresh setelah logout, refresh token yang di-reuse setelah rotasi

### Test integrasi: race condition pada approval

`internal/repository/disbursement/disbursement_repository_integration_test.go` menjalankan dua `UpdateStatus` bersamaan (APPROVED vs REJECTED) ke row `PENDING` yang sama lewat repository asli, bukan fake — membuktikan `SELECT ... FOR UPDATE` benar-benar menyerialkan kedua transaksi. Butuh Postgres asli, jadi di-skip otomatis kalau `DB_HOST` tidak di-set:

```bash
docker-compose up -d postgres migrate

DB_HOST=localhost DB_PORT=5432 DB_USER=postgres DB_PASSWORD=postgres DB_NAME=disbursement DB_SSLMODE=disable \
  go test ./internal/repository/disbursement/... -run TestUpdateStatusConcurrentApprovalIsRace -v
```

Hasil yang diharapkan: tepat satu goroutine sukses, satu lagi `ErrConflict`, dan status akhir baris tidak pernah PENDING atau tercampur.

---

## Verifikasi Manual

### Idempotency

```bash
KEY=$(uuidgen)

# request pertama
curl -i -X POST http://localhost:8080/disbursements \
  -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: $KEY" \
  -H 'Content-Type: application/json' \
  -d '{"recipient_name":"Budi","account_number":"1234567890","bank_code":"BCA","amount":1250000}'

# request kedua, key sama
curl -i -X POST http://localhost:8080/disbursements \
  -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: $KEY" \
  -H 'Content-Type: application/json' \
  -d '{"recipient_name":"Budi","account_number":"1234567890","bank_code":"BCA","amount":1250000}'
```

Response kedua identik dengan yang pertama, membawa header `X-Idempotent-Replayed: true`, dan tidak ada baris disbursement baru.

### Race condition pada approval

```bash
ID=DSB-000001

curl -s -o /dev/null -w '%{http_code}\n' -X PATCH http://localhost:8080/disbursements/$ID/status \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"status":"APPROVED"}' &

curl -s -o /dev/null -w '%{http_code}\n' -X PATCH http://localhost:8080/disbursements/$ID/status \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"status":"REJECTED"}' &

wait
```

Keluarannya satu `200` dan satu `409`.

---

## Batasan yang Diketahui

Hal-hal berikut adalah pilihan sadar untuk scope test ini, bukan kelalaian. Alasan lengkapnya di ARCHITECTURE.md.

| Kondisi sekarang | Untuk production |
|---|---|
| Refresh token di PostgreSQL | Redis — TTL native menghapus kebutuhan cleanup, dan bebannya lepas dari database bisnis |
| Rate limiting in-memory per instance | Redis — batas in-memory tidak akurat begitu aplikasi di-scale horizontal |
| `POST /auth/login` dibatasi IP+username, bukan user_id | Endpoint ini pre-auth — belum ada JWT untuk dijadikan kunci. IP saja gampang dilewati (ganti IP/proxy pool); username saja bisa disalahgunakan buat lock-out akun orang lain. Kombinasi keduanya menahan brute-force satu IP maupun credential stuffing terdistribusi ke satu akun |
| Audit lewat buffered channel | Transactional outbox — event tidak hilang meski proses crash |
| ID sekuensial `DSB-000001` | ULID berprefix — menghilangkan kebocoran volume transaksi dan enumerabilitas |
| Access token tidak bisa dicabut sebelum kedaluwarsa | Blacklist `jti`, bila revocation instan memang dibutuhkan |
| Baris idempotency kedaluwarsa dibersihkan lewat job berkala | Partisi berdasarkan waktu bila volumenya tumbuh besar |
