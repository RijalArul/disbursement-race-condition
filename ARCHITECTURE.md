# Architecture Decisions

Dokumen ini menjawab Bagian 1 dari coding test, ditambah beberapa keputusan pendukung yang memengaruhi implementasi.

---

## 1.1 Idempotency

`POST /disbursements` menerima header opsional `Idempotency-Key` berisi UUID v4 yang **digenerate oleh client**. Kunci harus datang dari client, bukan server: kalau server yang membuat kunci setiap request masuk, retry akibat timeout tetap dianggap request baru dan duplikasi tetap terjadi. Yang membuat sebuah retry dikenali sebagai request yang sama adalah client memakai ulang kunci yang persis sama.

State disimpan di tabel `idempotency_keys` pada PostgreSQL yang sama dengan data bisnis, dengan constraint `UNIQUE (user_id, idempotency_key)`. Menyimpannya di memori proses bukan pilihan, karena hilang saat restart dan tidak terbagi antar instance.

Alurnya **reserve-then-complete**. Server menjalankan `INSERT ... ON CONFLICT DO NOTHING` untuk mengklaim kunci dengan state `PROCESSING` beserta SHA-256 dari request body. Kalau baris berhasil dibuat, request diproses, lalu baris di-update menjadi `COMPLETED` beserta response body dan status code-nya. Kalau `INSERT` tidak menghasilkan baris, berarti kunci sudah dipakai: state `COMPLETED` mengembalikan response tersimpan apa adanya plus header `X-Idempotent-Replayed: true`; state `PROCESSING` berarti request pertama masih berjalan sehingga dijawab `409`; `request_hash` yang berbeda berarti kunci dipakai untuk body lain sehingga dijawab `422`.

Pola cek-dulu-baru-insert sengaja dihindari karena tidak atomic — dua request paralel bisa sama-sama lolos pengecekan lalu sama-sama menulis. `ON CONFLICT` memindahkan penjaminan itu ke unique index database. Scoping per user mencegah satu user membaca response milik user lain yang kebetulan memakai kunci sama. Baris kedaluwarsa setelah 24 jam lewat kolom `expires_at`.

---

## 1.2 Concurrency & Locking

Dua admin yang menekan "Approve" bersamaan ditangani dengan **pessimistic locking**. `PATCH /disbursements/:id/status` membuka transaksi, membaca baris dengan `SELECT ... FOR UPDATE`, memeriksa status, lalu menulis perubahan dan commit. Request kedua terblokir di baris `SELECT` sampai transaksi pertama commit. Setelah lock dilepas, request kedua membaca status yang sudah `APPROVED`, gagal pengecekan, dan menerima `409` dengan pesan bahwa disbursement sudah final. Hasilnya persis yang diminta: satu berhasil, satu mendapat error yang jelas.

Optimistic locking dengan kolom `version` sengaja tidak dipakai. Optimistic bertumpu pada asumsi bahwa penulis yang kalah akan **mengulang** operasinya. Untuk approval asumsi itu salah — approval bukan operasi yang boleh diulang, dan admin kedua justru harus diberi tahu bahwa keputusan sudah diambil orang lain. Optimistic juga memaksa retry loop di sisi client untuk sesuatu yang bisa diselesaikan database dalam satu transaksi.

Trade-off pessimistic adalah lock ditahan selama transaksi berlangsung. Karena itu transaksi dijaga sesempit mungkin: hanya `SELECT`, validasi, dan `UPDATE`. Penulisan audit log dikirim ke worker asinkron **setelah** commit, tidak pernah di dalam transaksi, supaya lock tidak ditahan menunggu I/O. Karena lock hanya mengunci satu baris berdasarkan primary key, kontensi hanya terjadi antar admin yang menyentuh disbursement yang sama — bukan antrean global.

Alternatif setara adalah `UPDATE ... WHERE id = ? AND status = 'PENDING'` lalu memeriksa `RowsAffected`. Itu lock-free dan hanya satu roundtrip, tapi kurang eksplisit saat dibaca dan lebih sulit diperluas ketika nanti ada aturan validasi tambahan sebelum penulisan.

---

## Keputusan Pendukung

### Penyimpanan refresh token: PostgreSQL, di-hash

Requirement `POST /auth/logout` mewajibkan refresh token bisa diinvalidasi. JWT bersifat stateless — sekali diterbitkan ia valid sampai `exp`, dan tidak ada cara mencabutnya tanpa menyimpan state di server. Jadi state ini wajib ada; pertanyaannya hanya di mana.

Dipilih tabel `refresh_tokens` di PostgreSQL. Yang disimpan adalah **SHA-256 hash** dari token, bukan token mentah — perlakuan yang sama seperti password, sehingga kebocoran database tidak langsung memberi attacker token yang bisa dipakai. Logout mengisi `revoked_at`; endpoint refresh menolak baris yang sudah di-revoke atau kedaluwarsa.

Selain itu diterapkan **rotasi**: setiap kali refresh dipakai, token lama di-revoke dan token baru diterbitkan. Kalau token yang sudah di-revoke muncul lagi, itu indikasi token dicuri, dan seluruh token milik user tersebut ikut di-revoke.

Alasan memilih database dibanding Redis untuk scope ini: PostgreSQL sudah menjadi dependency wajib, sehingga tidak menambah service baru; token durable melewati restart; dan operasi revoke cukup satu `UPDATE`.

**Untuk skala production, Redis adalah pilihan yang lebih tepat.** TTL native menghilangkan kebutuhan cron cleanup baris kedaluwarsa, lookup in-memory jauh lebih cepat daripada index scan ketika tabel token tumbuh besar, dan bebannya lepas dari database bisnis yang seharusnya fokus melayani query disbursement dan audit. Migrasi ke sana tidak mengubah desain — hanya mengganti implementasi `RefreshTokenRepository`, karena layer service hanya bergantung pada interface-nya.

Konsekuensi yang perlu disadari: access token yang sudah diterbitkan tetap valid sampai `exp` meskipun user sudah logout. Ini sifat bawaan JWT stateless. Mitigasi yang dipakai adalah TTL pendek (15 menit). Kalau revocation instan dibutuhkan, perlu blacklist `jti` yang dicek di setiap request — dan itu mengembalikan biaya lookup ke jalur panas, yang justru alasan JWT dipakai sejak awal.

### Audit log: worker asinkron berbatas

Requirement menyatakan audit log tidak boleh memblokir operasi utama, dan kegagalan penulisannya tidak boleh menggagalkan operasi disbursement.

Implementasinya adalah buffered channel dengan sejumlah worker goroutine tetap. Layer service mengirim event setelah transaksi bisnis commit. `Enqueue` memakai `select` dengan `default`, sehingga saat buffer penuh event di-drop dan dicatat sebagai warning — request tidak pernah menunggu.

Beberapa detail yang tidak boleh dilewat:

- Context request sudah di-cancel begitu handler selesai. Event membawa `context.WithoutCancel(ctx)` agar `request_id` tetap terbawa tanpa ikut ter-cancel di tengah `INSERT`.
- Goroutine tidak dibuat per request. Spike traffic akan melahirkan ribuan goroutine yang berebut connection pool, dan mekanisme yang dimaksudkan agar non-blocking justru membuat request utama gagal menunggu koneksi.
- Panic di dalam goroutine tidak bisa di-recover dari luar. `recover` dipasang terpusat di satu tempat di dalam worker.
- Saat `SIGTERM`, channel ditutup dan worker di-drain sebelum proses keluar.

**Kenapa bukan trigger database.** Trigger berjalan di dalam transaksi yang sama dengan perubahan datanya. Kalau penulisan audit gagal, seluruh transaksi abort dan operasi disbursement ikut gagal — kebalikan persis dari yang diminta. Trigger juga tidak mengetahui `actor` dan `request_id`, karena keduanya konsep aplikasi yang tidak ada di baris manapun; menyelundupkannya lewat `SET LOCAL` tetap menuntut aplikasi bekerja setiap transaksi, hanya lewat jalur yang tidak terlihat di kode. Terakhir, trigger kehilangan intent bisnis: soft delete dan perubahan status sama-sama `UPDATE`, dan trigger harus menebak maksudnya dari kolom mana yang berubah.

**Kenapa bukan CDC.** Membaca WAL lewat Debezium punya masalah `actor` yang persis sama, kehilangan intent bisnis yang sama, dan menambah Kafka beserta Connect untuk mengaudit tiga jenis aksi. CDC adalah jawaban yang tepat untuk replikasi ke data warehouse atau search index, bukan untuk audit trail yang inti nilainya justru pada identitas pelaku.

Kelemahan jujur dari pendekatan yang dipilih: event yang masih di buffer hilang kalau proses crash, dan perubahan yang dilakukan langsung lewat `psql` tidak tercatat. Mitigasi untuk yang kedua adalah kebijakan akses database, bukan trigger. Untuk yang pertama, langkah berikutnya adalah **transactional outbox** — menulis event ke tabel `audit_outbox` di dalam transaksi yang sama (atomic, tidak pernah hilang), lalu relay asinkron memindahkannya ke `audit_logs`. Itu pola yang lebih kuat, dan dipilih tidak dipakai sekarang hanya karena requirement menekankan sifat non-blocking secara harfiah.

### Skema database

Beberapa keputusan skema yang perlu penjelasan:

**Partial index `WHERE deleted_at IS NULL`.** Soft delete berarti baris mati menumpuk selamanya. Tanpa partial index, index ikut tumbuh membawa baris yang tidak pernah muncul di query manapun. Semua index utama pada `disbursements` dibatasi ke baris hidup.

**GIN trigram pada `recipient_name`.** Parameter `search` melakukan partial match, yang diterjemahkan menjadi `ILIKE '%term%'`. Pola dengan wildcard di depan tidak bisa memakai btree index dan akan berakhir sebagai sequential scan. Extension `pg_trgm` dengan index GIN membuat pencarian ini terindeks.

**`amount BIGINT`.** Nilai uang selalu integer dalam satuan terkecil, tidak pernah floating point. Di sisi Go dipetakan ke `int64` di seluruh jalur, termasuk DTO, agar tidak ada titik di mana JSON number ter-decode menjadi `float64`.

**`UNIQUE (user_id, idempotency_key)`.** Dijelaskan di bagian 1.1.

**Format ID sekuensial.** `DSB-000001` dan `LOG-000001` dihasilkan dari sequence PostgreSQL, mengikuti contoh pada spesifikasi. Kelemahannya nyata dan perlu disebut: ID sekuensial membocorkan volume transaksi kepada siapa pun yang membuat dua disbursement pada waktu berbeda, dan mudah dienumerasi untuk probing IDOR. Untuk production, ULID dengan prefix (`DSB-01H...`) memberi keunggulan yang sama — bisa dibaca manusia, terurut secara waktu — tanpa membocorkan volume.

### Read caching untuk GET /disbursements (belum diimplementasikan)

`GET /disbursements` dan `GET /disbursements/:id` saat ini selalu membaca dari PostgreSQL, tidak ada layer cache. Ini keputusan sadar, bukan kelalaian.

Singleflight (menggabungkan request-request identik yang datang bersamaan supaya hanya satu yang benar-benar menyentuh database) sengaja tidak dipasang sekarang. Singleflight bekerja per proses — begitu aplikasi discale ke lebih dari satu instance, tiap instance punya singleflight sendiri-sendiri dan request identik yang jatuh ke instance berbeda tetap sama-sama menembus database. Dengan kata lain manfaatnya hilang tepat pada skenario yang justru butuh cache paling banyak. Alasan kedua: belum ada bukti beban baca yang menuntutnya — index parsial dan GIN trigram pada `recipient_name` seharusnya cukup menopang performa list/search di skala sekarang.

Stack ini juga tidak memakai Redis, jadi tidak ada tempat alami untuk cache yang dibagi antar instance. Kalau nanti beban baca terbukti jadi bottleneck, langkah berikutnya adalah menambah Redis sebagai shared cache (TTL pendek, invalidasi saat ada write lewat jalur audit/event yang sudah ada) — bukan singleflight in-process, karena singleflight tidak bertahan begitu ada lebih dari satu instance.
