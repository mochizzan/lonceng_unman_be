# Always-Fresh-on-Error (MarkStale) — Design

**Date:** 2026-09-10
**Status:** Approved (brainstorming 1-4 ok + Testing A-G)
**Prev:** `2026-09-09-fresh-browser-ttl15-design.md` pure ephemeral TTL15m
**Goal:** Jika ANY LMS op error (`ErrLMSExpired` + `IsTransient`), langsung buat sesi browser baru walaupun TTL 15m belum habis — semua endpoint LMS bawa `npm+password` jadi bisa re-login.

---

## 1. Context

- `14:22:50-14:23:56` `POST /lms/student-profile` `2211700006` `1m13s 500` `wait element "#nim": context deadline exceeded (not found for 15s)` 3× + `probe eval failed context deadline exceeded`. `session reused` (lastUsed <15m) tapi `PHPSESSID` PHP menit sudah GC — LMS return `200` shell tanpa `#nim`/`alert-danger` (200, bukan 302). `isLMSExpiredProbe` `hasAlert==true && !hasExpected` → false-negative, `Navigate` tidak `ErrLMSExpired` → `student_profile_service Scrape 77` & `lms_service navigateWithLMSRetry 242` hanya retry `ErrLMSExpired`, jadi retry 3×64s → `500`. Semua LMS-bound (`login/profile/photo/KRS/KHS/semesters/detail/file`) bawa `npm+password`
- Investigasi `codegraph` 3 subagent `AuditSessionReuse-2` `R1 273/ R2 318` fast-path reuse tanpa `validateSession`, `R4 touchLastUsed` perpanjang TTL, `R5` scraper reuse; `AuditServiceErrors-2` semua service `staleReuseRisk HIGH` untuk generic `deadline/closed network` (`ElementAttribute/Eval/DownloadPDF` return tanpa evict); `DesignFreshOnError-2` rekom `MarkStale` `port 28`

---

## 2. Architecture

- `port.SessionManager` `GetOrCreate/Close/CloseAll` + `MarkStale(npm) bool / Invalidate` (`port/session.go:28`, `manager.go:446` `delete(m.sessions)+browser.Close` luar `m.mu`, idempotent, tanpa `getNPMLock`). Sudah di branch `b455558` `70 lines` diff `HEAD`, `go vet` sebagian pass, mocks `tests/service/*` perlu `MarkStale` stub
- `Browser` pure ephemeral `Connect` no `UserDataDir` (`browser.go:131`), `IsTransientBrowserError` (`eof/deadline/timeout/closed network/closed/target closed/detached`) dipakai service untuk decide `MarkStale`
- `session.go:214-222` fast-fail `sel=="#nim" && isTimeout(err) → ErrLMSExpired` attempt 1 + `240 probeExpired||isWaitTimeout → ErrLMSExpired` exhausted dipertahankan (build `22614b` belum commit). `MarkStale` menambah guarantee untuk error lain (`input[name='semester']`, `.table-bordered`, `a[href*='khs_pdf.php']`, `DownloadPDF`, `Eval`)
- Service eksplisit: untuk `ErrLMSExpired` → `MarkStale+GetOrCreate retry once` (existing); untuk `IsTransient` → `MarkStale + return err` (next request fresh, tidak retry langsung untuk hindari 2× login race). `401 credential` (`login gagal`, `username dan password`) **tidak** `MarkStale`

---

## 3. Data Flow

`Handler` bind `npm+password` → `Service` `getNPMLock` per-NPM (`lms_service.go:270`) → `Manager.GetOrCreate` (`273` RLock hit `<15m` reuse `newSession(Page about:blank)` atau miss → `createNewSession CheckDNS → Browser.New().Connect → Page LMSBaseURL → login Race .wrapper/.alert-danger` → `cachedSession` `lastUsed=now`) → `Service session.Navigate(url)` Method-D (`#nim` viewupdate, `input[name='semester']` KRS, `.table-bordered` KHS list, `a[href*='khs_pdf.php']` KHS detail, `dashboard` photo) → `ElementAttribute/Eval/DownloadPDF/DownloadImage` → `MarkStale` pada `ErrLMSExpired/IsTransient` sebelum return → `Handler ClassifyDocumentError 503/401/500`

LMS-bound (fresh-on-error berlaku): `POST /lms/login`, `POST /lms/krs` (`konversi_upd_mhs` + `krs_pdf.php`), `POST /lms/khs/semesters`, `POST /lms/khs` (`cetak_detail` + `khs_pdf.php`), `POST /lms/khs/file` (local-first fallback), `POST /lms/student-profile`, `POST /lms/student-profile/photo`. FS-only (`krs/extract`, `khs/extract`, `krs/data`, `khs/data`, `student-profile/data`, `health`, `eval`) tidak hit LMS, tidak `MarkStale`

---

## 4. Error Handling

- `ErrLMSExpired` (`session.go:222/248` probe `hasAlert||#nim timeout` atau exhausted `wait element`) → `Service MarkStale(npm); fresh:=GetOrCreate(npm,password); fresh.Navigate(url) once` → `200` total `~25s (15s wait+6s login+probes)` bukan `1m13s 500`; safe to call `MarkStale` concurrently, next `GetOrCreate` fresh `Browser`
- `IsTransient` (`eof/deadline/timeout/closed network/closed/target closed/detached` dari `Page`/`Navigate`/`waitElementReady`/`DownloadPDF` `closed network connection`) → `Service MarkStale(npm); return err` → `handler 503` `Layanan LMS tidak dapat diakses`; next request fresh, tidak spin 3×
- `401 credential` (`Login gagal`, `username dan password`, `session expired` credential) → tidak `MarkStale`, return `401`, browser tetap cached (password salah bukan browser usang)
- `500 generic` (probe miss `hasExpected==false` tanpa alert, `404/500` non-transient) → sebelum: fast-fail `#nim` sudah jadi `ErrLMSExpired`, jadi tidak `500`; selain `#nim` generic `IsTransient false` → `MarkStale` anyway karena `ANY BrowserSession error` (opsi A strict: hanya `ErrLMSExpired+IsTransient` jadi `404` tidak `MarkStale` — spec pakai A)
- `MarkStale` idempotent `bool` return, `Close` tetap per-NPM lock, tidak `evictSession` holding `pageMu`

---

## 5. Testing

Semua harness `single file fetch only` `node tmp/<file> --help ==0` `go vet 0` `no otel/debug/cloud`

**A Existing (tetap pass):** `tmp/concurrent-evidence.mjs` 8 `login+profile` A→B + A‖B `0×503 per-req<15s wall<60s`; `tmp/lms-full-coverage.mjs` 16 `login/krs/semesters/khs` seq+par `0×503`; `tmp/e2e-full-flow.mjs` 61 steps/phase `login→data×2→photo→krs dl/extract/get→semesters→loop khs dl/extract/get` seq+par `122/122 200`; Prod mirror `tmp/prod/*.prod.mjs` 4 files default `https://lonceng-unman-api.miproduction.web.id`

**B Baru `tmp/stale-eviction-evidence.mjs` (induksi MarkStale):** `login A 200` → `profile A 200 reuse` → induksi error via `SESSION_TTL=1m` expiry alami (atau `chaos-stale` burst induksi IsTransient) → `profile A` next hit harus `creating new session` (MarkStale dari IsTransient) bukan `reused` `<30s` `0×503`. Tanpa `MarkStale` fail

**C Baru `tmp/chaos-stale.mjs` (burst):** `10× POST /lms/student-profile` paralel `5×A ‖ 5×B` tiap 200ms burst → `0×503` `per-req<15s` (kecuali 1× `~25s` jika expiry) `MarkStale` tidak deadlock `pageMu/npmLock`

**D Unit `tests/infrastructure/session`:** `TestMarkStale_EvictsAndNextGetOrCreateFresh` `InjectTestSession` → `MarkStale true` → `SessionCount 0` → `GetOrCreate fresh`; `TestMarkStale_Idempotent` 2×; `TestAlwaysFreshOnError_NavigateTimeout` mock `Navigate timeout` → `Service` `MarkStale` → `SessionCount 0`; `TestIsTransient_ClosedNetwork` `closed network true` (sudah)

**E Smoke prod checklist:** `go vet ./... 0` `go test ./tests/service ./tests/infrastructure/session 0` `docker build ghcr 0` `down && up -d` healthy 127.0.0.1:3000 → `BASE_URL=https://... node tmp/prod/stale-eviction-evidence.prod.mjs --json` pass

**F Baru `tmp/account-switch-stale.mjs` — repro idle ganti akun (sebab 14:22, `--idle SEC --ttl SEC`):** `login A 200 → profile A 200 → krs A 200` (panaskan `lastUsed=now`) → idle `A` sementara `loop 5× khs/semesters B 200` durasi `IDLE_SEC` → `profile A` lagi
- `F1 idle < TTL` (`--ttl 900 --idle 600` prod `TTL 15m IDLE 10m`) → `A` masih `<15m` `session reused` tapi `PHPSESSID` GC menit → `wait #nim 15s` → `ErrLMSExpired` → `MarkStale+GetOrCreate` → `200 <30s` bukan `500 1m13s` (before `500`)
- `F2 idle > TTL` (`--ttl 900 --idle 960` prod `TTL 15m IDLE 16m` atau dev `SESSION_TTL=1m` + `--ttl 60 --idle 70`) → `A` `>15m` evict → fresh `creating new session 6s` → `200`
Assert `status 200` `0×503` `per-req<30s` (expiry `25s` ok) `docker logs` `lms expired fast-fail`/`session marked stale`/`creating new session` bukan `session reused`. Prod mirror `tmp/prod/account-switch-stale.prod.mjs`

**G Baru `tmp/ttl-boundary-evidence.mjs`:** `login A → profile A reuse` → `sleep TTL-10s` → `profile A` `reused 200` → `sleep 20s` (lewat `TTL+10s`) → `profile A` `fresh creating new session 200`

---

## 6. Acceptance

- `docker build -t ghcr.io/mochizzan/lonceng_unman_be:latest .` `0` `docker compose down && up -d` healthy `127.0.0.1:3000` + `docker push` `prod pull`
- `go vet ./internal/... 0` `go test ./tests/infrastructure/session ./tests/service -run TestMarkStale -v 0`
- `tmp/concurrent-evidence.mjs` `8/8 200 0×503 per-req<15s` `tmp/lms-full-coverage.mjs` `16/16 200 per-req<15s` `tmp/e2e-full-flow.mjs` `122/122 200 per-req<15s` (fresh path); `tmp/account-switch-stale.mjs --idle 600` (prod) atau `--idle 70 --ttl 60` (dev) `F1 <30s 200` (`15s wait+6s login`) `F2 fresh 200 per-req<15s` `tmp/ttl-boundary-evidence.mjs` `reused per-req<15s → fresh per-req<15s` — expiry `25s` allowed `<30s`, not `<15s`
