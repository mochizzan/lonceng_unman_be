# Fresh per NPM, In-Memory, TTL 15m — Design

**Date:** 2026-09-09
**Status:** Approved (brainstorming 1-4 incremental ok)
**Scope:** Remove disk-persisted Chromium profile; always fresh browser per NPM cache miss. Fixes `503 use of closed network connection` (reparse `Singleton*` on Windows bind-mount) and `context deadline exceeded` stale-page race.

---

## 1. Context & Problem

- **Symptom A (2026-09-04T04:19-04:22 log, 2211700006):** `Navigate ok 100-300ms` then `wait element "form": context deadline exceeded (not found for 15s)` ×3 → `total 1m4s → 500`. LMS returned 200 shell without form after idle 28m; probe `Anda Sudah Logout` miss → no `ErrLMSExpired` bubble.
- **Symptom B (2026-09-09T12:14 docker ghcr, 2211700006):** `Page() transient failure EOF` then `write tcp ... use of closed network connection after 3 attempts → 503`. `ls profiles/2211700006/Singleton* → Unsupported reparse point type` (Windows reparse via `./profiles:/data/profiles` bind-mount). `2211700009` parallel 200 proves not LMS throttle; per-NPM isolation works.
- **Root cause:** `ConnectWithProfile(UserDataDir)` + `cleanStaleLock` only `os.Remove(Singleton*)` without Windows reparse handling; `IsTransientBrowserError` misses `closed network`; `navigateReadySelector("viewupdate")="form"` too generic.
- **Constraint:** LMS PHP native (`PHPSESSID` cookie, curl-able parallel). `in-memory` sessions with `TTL 15m` are acceptable; login cost 6-9s negligible vs 64s hang / 503.

Investigation sources: `internal/infrastructure/session/manager.go`, `session.go`, `internal/infrastructure/browser/browser.go`, `internal/interfaces/http/router/router.go`, `handler/extraction_handler.go`, `service/extraction_service.go`, `compose.yml`, `tmp/phase1-evidence-browser-timeout.md`, `tmp/phase3-hypotheses.md`, `tmp/concurrent-evidence.mjs`, `tmp/stress-burst-parallel.mjs`.

---

## 2. Goals & Non-Goals

**Goals:**
- No disk profile → no `Singleton*` stale, no `tmpfs 512M` pressure, no Windows reparse.
- Always fresh `Browser` on cache miss; reuse in-memory `sessions[NPM]` while `TTL 15m` valid.
- Deterministic `8/8 200, 0×503` for 2 different NPMs sequential + parallel (same `ghcr` image).
- Prove TTL eviction via dedicated harness + unit tests.

**Non-Goals:**
- Survive `docker compose down` without re-login.
- Cookie-jar memory optimization (approach 2) — deferred.
- Request-scoped browser pool (approach 3) — not chosen.
- Changing LMS itself, adding `otel`/`debug`/`cloud` deps.

---

## 3. Architecture

### 3.1 Components

- **`config.Config`**: `SessionTTL` default `15m` (was `24h`), `BrowserLaunchTimeout 60s`, `BrowserTimeout 60s`, `DNSTimeout 5s`, `MaxSessions 20`. `ProfileBaseDir` unused when ephemeral (kept for compat, not mounted).
- **`session.Manager`** (`internal/infrastructure/session/manager.go`):
  - Fields: `sessions map[NPM]*cachedSession`, `npmLocks sync.Map`, `ttl = 15m`, `stopCh`, `browserFactory func()*browser.Browser`.
  - `cachedSession`: `browser *Browser`, `createdAt/lastUsed time.Time`, `mu sync.Mutex` guards `lastUsed/activeCount`, `pageMu sync.Mutex` serializes `Browser.Page()`, `activeCount/pageCount int32`.
  - Removed: `profileDir string`, `createSessionWithRestore`, `validateSession`, disk `Stat(profileDir)` branching.
  - Kept: `maxSessionLifetime 2h`, `gracePeriod 5m`, `maxPagesPerBrowser 10`, `cleanupLoop 1m`, `evictOldest`, `CheckDNS` with 3× retry.
- **`browser.Browser`** (`internal/infrastructure/browser/browser.go`):
  - `Connect(headless bool)` without `UserDataDir`; `ConnectWithProfile` deleted or deprecated.
  - `SetLaunchTimeout` + `CancelTimeout` after `Connect` retained.
  - `Page(url)` retries 3× `[0,1s,2s]` on `IsTransientBrowserError`; add `"closed network"` / `"closed"` to transient set.
  - `cleanStaleLock` disk helper deleted (no profile).
  - `Close()` calls `rod.Close()` only (no `launcher.Cleanup` of profile unless legacy flag).
- **`session.rodSession`** (`internal/infrastructure/session/session.go`):
  - `newSession`, `replacePage`, `touchLastUsed`, `Close` (page 0→1→0, activeCount--), `Navigate` (`about:blank` reset → `Navigate` → `waitReady`), `waitElementReady` / `waitDomReady`, `isLMSExpiredProbe`, `navigateReadySelector` (`#nim` for `viewupdate`, `input[name=semester]`, `.table-bordered`, `a[href*='khs_pdf.php']`), `isTimeout` / `isElementTimeout`.

### 3.2 Alternatives Considered

- **Approach 1 Pure Ephemeral (chosen):** simplest, no stale, one code path.
- **Approach 2 Memory Cookie Jar:** saves 6-9s per 15m but adds cookie state + validateSession.
- **Approach 3 Request-Scoped Pool:** freshest but high launch pressure.

---

## 4. Data Flow

### 4.1 All LMS-Bound Endpoints (fresh flow applies)

Through `Manager.GetOrCreate(npm,password)` → `rodSession.Navigate` → `browser.Scraper / DownloadPDF/Image` → `rodSession.Close`:

- `POST /api/v1/lms/login` (`LMSHandler.Login → LMSService.Login`)
- `POST /api/v1/lms/krs` (`DocumentHandler.DownloadKRS → DocumentService.DownloadKRS`: `op=master_mahasiswa&act=konversi_upd_mhs` then `DownloadPDF krs_pdf.php`)
- `POST /api/v1/lms/khs/semesters` (`GetKHSSemesters`: `op=mahasiswa_khs&act=cetak` list `.table-bordered`)
- `POST /api/v1/lms/khs` (`DownloadKHS`: `cetak_detail` + `DownloadPDF khs_pdf.php`)
- `POST /api/v1/lms/khs/file` (`DownloadKHSFile`: FS hit, fallback `DownloadPDF` via session if miss)
- `POST /api/v1/lms/student-profile`, `POST /api/v1/lms/student-profile/photo` (`StudentProfileService.Scrape/GetPhoto`: `viewupdate #nim`)

### 4.2 FS-Only Endpoints (no Browser, no Manager)

- `POST /api/v1/lms/krs/extract`, `POST /api/v1/lms/khs/extract` → `ExtractionService.ExtractKRS/KHS` reads `downloads/<npm>/krs|khs/*.pdf` → `PDFParser.Parse*` → `ExtractionCache.Set`. Only on `ErrPDFNotFound` calls `verifySession` (which will use fresh path but not the hot path).
- `POST /api/v1/lms/krs/data`, `POST /api/v1/lms/khs/data` → `GetKRSExtraction/GetKHSExtraction` reads `extracted/<npm>/*.json`.
- `POST /api/v1/lms/student-profile/data` → `StudentProfileService.Get` reads cache file.
- `GET /api/v1/health`, `GET|POST /eval/*`, `GET /eval/static/*`, `POST /api/v1/auth/*`: no LMS.

### 4.3 Sequence (miss vs hit, TTL 15m)

```
Hit (lastUsed <15m && hard <2h+5m, activeCount managed):
  RLock hit → mu.Lock activeCount++ → newSession(Page about:blank, pageMu) → rodSession{page} → Navigate → Eval/Download → Close(activeCount--, pageCount--)
Miss / expired / hard limit:
  RLock miss or TTL exceeded → per-NPM Lock → double-check → evict if expired (under m.mu) → createNewSession:
    CheckDNS(LMSBaseURL, 3×5s) → Browser.New().SetLaunchTimeout(60s).Connect(headless) → Page(LMSBaseURL) → waitElementReady(SelUsernameInput)
    → login(Race SelSuccessIndicator .wrapper vs SelErrorIndicator .alert-danger) → wrapper ok → cachedSession{now,now}
    → mu.Lock activeCount=1 → newSession → rodSession
Parallel NPM_A ‖ NPM_B: getNPMLock per-NPM + pageMu per-browser → no crosstalk; parallel wall <1.4× seq sum.
```

---

## 5. Error Handling

- **503 Infrastructure:** `CheckDNS` transient (`timeout|no such host|network unreachable|connection refused`) 3×, `Page EOF|deadline|timeout|closed network` 3×, `Navigate`/`WaitLoad` CDP page errors → `apperror.IsInfrastructureError` → `503 "Layanan LMS tidak dapat diakses..."`. Fixes `2211700006` 503 by adding `closed network` to transient.
- **401 Credential:** login `Race` matches `SelErrorIndicator` with `LOGIN GAGAL` → `401 Unauthorized`. `ClassifyLoginResult` preserves anti-enumeration.
- **500 Generic:** `waitReady #nim` timeout without expiry markers → `500 failed to scrape` with `probe raw` DEBUG + `probe miss but expected selector absent` WARN. Not retried as infra.
- **ErrLMSExpired bubble:** `isLMSExpiredProbe` (`hasAlert==true && hasExpected==false` or `alert-danger`) → wraps `port.ErrLMSExpired` → `navigateWithLMSRetry` / `StudentProfileService.Scrape` evicts and re-logins once. No 3×15s spin.
- **TTL behavior (15m):** soft TTL `15m` on `lastUsed`, hard `2h + 5m grace` on `createdAt`. `cleanupLoop 1m` evicts `activeCount==0` only; `activeCount>0` within `grace` skipped. Miss cost 6-9s fresh login, not 503.
- **Guard:** `SESSION_PERSISTENT` flag not introduced (approach 1). If later needed, add `PersistentProfile bool` behind env, but current spec is pure ephemeral.

---

## 6. Testing & Verification

### 6.1 Existing Harness (no change)

- `tmp/concurrent-evidence.mjs` (std `fetch` only, no `otel/debug/cloud`): sequential A→B + parallel A‖B (different NPMs, passwords `Izzan027` / `ilham122`), asserts `all 200`, `0×503`, `per-req <15s`, `wall <60s`, `parallel wall <1.4× seq sum`. `go vet ./internal/infrastructure/session ./internal/infrastructure/browser ./internal/application/service == 0`, `node tmp/concurrent-evidence.mjs --help == 0`.

### 6.2 New Harness: Session TTL Evidence

- File: `tmp/session-ttl-evidence.mjs` (single file, std `fetch`, `--help`, `--json`, `exit 1` on fail).
- Flow:
  1. `POST /login NPM_A` → `200 creating new session`.
  2. `POST /student-profile NPM_A` → `200 session reused` (`#nim` <15s).
  3. Wait `TTL+delta`: prod `15m+10s` (documented long wait) OR dev override `TEST_TTL_SEC` / `SESSION_TTL=1m` in compose test (wait `70s`). Alternatively evict is not via test endpoint (no `__test/evict` added for security).
  4. `POST /student-profile NPM_A` → must be `200 creating new session` (fresh), not `reused`, not `503/500`, wall `6-9s`. Fail if still `reused` (TTL not honored) or `503`.
- Parks with `tmp/concurrent-evidence.mjs` to prove no regression on parallel after TTL.

### 6.3 Unit Tests

- `tests/infrastructure/session/ttl_test.go::TestSessionTTL15m`: `SessionTTL=15m`, inject session with `lastUsed = now-16m` → `Cleanup()` → `SessionCount==0`; `now-14m` → still `1`.
- `tests/infrastructure/browser/transient_test.go::TestIsTransientBrowserError_ClosedNetwork`: `"closed network"` → `true`.
- `tests/infrastructure/session/cleanup_test.go::TestHardLimit2h_Grace5m`: `createdAt now-2h6m` → evict even if `activeCount>0`.
- `tests/infrastructure/session/maxsessions_test.go::TestMaxSessionsEvictOldest`.

### 6.4 Smoke Tests

- `tmp/smoke-fresh-browser.mjs`: sequential single NPM: `login → krs → khs/semesters → khs → profile` each `200` and `<15s` (linear, not parallel).
- `tmp/smoke-extract-fs.mjs`: `POST /lms/krs/extract` + `POST /lms/khs/extract` (FS-only) must not increment `docker logs | grep "creating new session"` beyond the single initial login.

All harnesses: `node tmp/<file> --help` prints usage, `go vet`/`go test` pass.

---

## 7. Deployment & Config

- `internal/config/config.go`: `SessionTTL` env `SESSION_TTL` default `15m` (change from `24h`); validate `APP_ENV ∈ {development,staging,production}`.
- `compose.yml`: remove `./profiles:/data/profiles` bind-mount; remove `PROFILE_BASE_DIR=/data/profiles` override (or set to `""`/tmpfs ephem); remove volumes for profiles; keep `/data/downloads`, `/data/extracted`, `/data/eval/ground_truth`, `tmpfs /tmp 512M`, `shm_size 1gb`, `dns 8.8.8.8/1.1.1.1`.
- `Dockerfile`: ensure no `mkdir /data/profiles` required (or keep as empty dir, unused).
- Migration: existing `./profiles/*` on host becomes orphaned (safe to `rm -rf ./profiles/*` after deploy); no data loss (sessions ephemeral).

---

## 8. Risks & Mitigations

- **Risk:** Login rate per 15m doubles vs 24h. **Mitig:** LMS handles concurrent logins (proven `A‖B` 200), cost 8s acceptable; `MaxSessions 20` caps browsers.
- **Risk:** Cold-start latency visible to first request after idle. **Mitig:** `Page` retry `[0,1s,2s]` + `DNS 3×`; `parallel wall` harness guards regression.
- **Risk:** No survive restart. **Mitig:** `docker compose restart` is rare; re-login is cheap; no stale to debug.
- **Risk:** `closed network` string miss. **Mitig:** unit test covers, also match `"closed"` substring.

---

## 9. Acceptance Criteria

- `docker build -t ghcr.io/mochizzan/lonceng_unman_be:latest .` succeeds.
- `docker compose down && docker compose up -d` healthy `127.0.0.1:3000` (port 3000 host killed before up).
- `BASE_URL=http://localhost:3000 NPM_A=2211700006 PASSWORD_A=Izzan027 NPM_B=2211700009 PASSWORD_B=ilham122 node tmp/concurrent-evidence.mjs` → `8/8 200, 0×503, per-req <15s, parallel wall <1.4× seq sum`. Pre-fix `17.54s` for `#nim` on cold docker is documented baseline; post-fix must be `<15s` via `#nim` poll (not `form` generic).
- `SESSION_TTL=1m docker compose up` (compose `environment: SESSION_TTL: "1m"` overrides `.env`) → `node tmp/session-ttl-evidence.mjs --json` passes TTL eviction (second hit `creating new session`, not `reused`).
- `go vet ./internal/infrastructure/session ./internal/infrastructure/browser ./internal/application/service` is `0`, `go test ./tests/infrastructure/session ./tests/infrastructure/browser` pass.
