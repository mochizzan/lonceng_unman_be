# Design: POST /api/v1/lms/khs/file — Local-first + inline fallback

- Date: 2026-09-05
- Status: Approved (brainstorming gate passed)
- Scope: Single endpoint `POST /api/v1/lms/khs/file` — no merge with `POST /api/v1/lms/khs`
- Approach chosen: **A) Local-first + inline fallback (reuse DownloadKHS internals)**

## 1. Context & Problem

`POST /api/v1/lms/khs/file` currently always hits LMS even when the PDF is already on disk:

- `handler.DownloadKHSFile` → `lmsDocumentService.DownloadKHSFile` (`lms_service.go:428`)
- Validates `tahun_ajaran`/`semester` → `sessions.GetOrCreate(npm, password)` → browser login (cold-start 60s, DNS 5s, 401/503 classification) → `os.Stat(downloads/{npm}/khs/{TA}_{Sem}.pdf)` → `SendFile` or 404.
- Comment `Local filesystem only — no browser lock needed` is misleading: per-NPM lock is not taken, but `GetOrCreate` is still called on every request.
- In contrast, `POST /api/v1/lms/khs` always re-downloads (navigate detail → find CETAK KHS button → `DownloadPDF` with per-NPM lock + `url.QueryEscape`).

Impact: every `/khs/file` cache hit pays cold-start cost and can return 401/503 even though the file is locally available. Caller inventory: Flutter FE (`lonceng_unman_fe`), eval pipeline (`/eval`), internal tools.

Recent reliability work (commits `c6813dd`, `6361f44`, `2d3f361`) already hardened DNS/browser retries; this change builds on that without touching those layers.

## 2. Goals & Non-Goals

### Goals

- Cache hit: serve PDF from disk with **zero LMS hit** (no `GetOrCreate`, no browser, no lock).
- Cache miss: download first via the same flow as `DownloadKHS`, then serve the PDF as binary.
- Keep the two endpoints separate: `POST /api/v1/lms/khs` keeps its "always download" semantics.
- Keep request/response contract stable (PDF binary + same headers).

### Non-Goals

- No change to `POST /api/v1/lms/khs`, extraction (`/lms/khs/extract`, `/lms/khs/data`), or inventory.
- No new endpoint, no async/polling, no `?fallback=` flag (YAGNI — Approach B/C deferred).
- No change to Flutter FE or eval callers beyond the latency/error improvement.
- No change to `config`, `session/manager` retry, or `entity/document.go`.

## 3. Architecture

### Current flow (before)

```
POST /api/v1/lms/khs/file {npm, password, tahun_ajaran, semester}
  → handler.DownloadKHSFile (validate)
  → lmsDocumentService.DownloadKHSFile
      → normalize semester/tahun_ajaran
      → ValidateSemester
      → GetOrCreate(npm, password)          ← ALWAYS hits LMS
      → os.Stat(downloads/{npm}/khs/{TA}_{Sem}.pdf)
      → 404 or SendFile
```

### New flow (after)

```
POST /api/v1/lms/khs/file {npm, password, tahun_ajaran, semester}
  → handler.DownloadKHSFile (validate npm/year/semester, password required)
  → lmsDocumentService.DownloadKHSFile
      → normalize: ToUpper(TrimSpace(semester)), TrimSpace(tahun_ajaran)
      → if !ValidSemester → 400 (no LMS hit)
      → filePath = cfg.App.DownloadDir/{npm}/khs/KHSFilename(tahun_ajaran, semester)
      → os.Stat(filePath)
        → hit  → return path,size              ← NO GetOrCreate, NO lock, NO LMS
        → miss → getNPMLock(npm).Lock()
                 → GetOrCreate(npm, password)  ← LMS hit only here
                 → Navigate(detailURL = LMSBaseURL + port.KHSDetailPath + "&" + url.Values{q tahun_ajaran, semester}.Encode())
                 → ElementHref(port.SelKHSCetakBtn)
                 → pdfURL = LMSBaseURL + "/admin/" + href
                 → DownloadPDF(pdfURL, filePath)
                 → os.Stat(filePath) → return
  → handler: c.SendFile with PDF headers, or classified error
```

Reuse surface from `DownloadKHS` (`lms_service.go:363`): `url.Values.Encode` for `tahun_ajaran` slash handling, `SelKHSCetakBtn`, `DownloadPDF`, per-NPM lock via `getNPMLock`, overwrite semantics. No new dependency.

## 4. Files & Blast Radius

| File | Change | Risk |
|------|--------|------|
| `internal/application/service/lms_service.go` | Refactor `DownloadKHSFile` to local-first + inline fallback; fix misleading comment; reuse `DownloadKHS` download block | Low — isolated method, same error helpers |
| `internal/interfaces/http/handler/document_handler.go` | No logic change; keep `SendFile` + headers; ensure `ClassifyDocumentError` is applied to fallback errors (already done for other doc handlers) | Low — header contract unchanged |
| `internal/interfaces/http/router/router.go` | No change (path stays `POST /lms/khs/file`) | None |
| `internal/domain/entity/document.go` | No change (`KHSFilename` reused) | None |
| `tests/service/khs_file_service_test.go` | Add cases: hit-no-LMS, miss-fallback-success, miss-401, miss-503 | Test-only |
| `tests/handler/khs_file_handler_test.go` | Add cases: fallback error mapping | Test-only |
| `docs/api/05_KHS_DOWNLOAD.md` (and new file doc if exists) | Clarify cache-hit-no-LMS vs miss-fallback | Docs |

No migration, no env var, no schema change.

## 5. API Contract

### Request

```
POST /api/v1/lms/khs/file
Content-Type: application/json

{ "npm": "2211700006", "password": "izzan027", "tahun_ajaran": "2022/2023", "semester": "GANJIL" }
```

- `npm`: `^[0-9]+$`, 8–12 chars (existing `validateNPM`)
- `password`: required (non-empty) — needed only for fallback; not validated against LMS on cache hit.
- `tahun_ajaran`: required, trimmed.
- `semester`: required, `GANJIL`|`GENAP` case-insensitive input, normalized to upper.

### Response — success (both hit & miss)

```
200 OK
Content-Type: application/pdf
Content-Disposition: attachment; filename="KHS_2211700006_2022_2023_GANJIL.pdf"
Content-Length: <bytes>
Accept-Ranges: bytes

<PDF bytes>
```

`filename` uses `strings.ReplaceAll(tahunAjaran, "/", "_")` (existing handler) — matches `KHSFilename`.

### Response — errors

| Status | When | Public message |
|--------|------|----------------|
| 400 | invalid JSON, npm invalid, `tahun_ajaran` empty, `semester` not GANJIL/GENAP | `npm is required` / `tahun_ajaran is required` / `semester must be GANJIL or GENAP` |
| 404 | file not on disk **and** fallback did not produce it (still missing after download attempt) | `KHS PDF not found for the specified year and semester` |
| 401 | fallback `GetOrCreate` classified as credential (`login gagal` / `username dan password` / redirect check) | `Username atau password salah` |
| 503 | fallback classified as infrastructure (`DNS check`, `browser connect`, `timeout`, `EOF`, `context deadline exceeded`, …) | `Layanan LMS tidak dapat diakses saat ini. Silakan coba lagi dalam beberapa saat.` |
| 500 | fallback failed for other reason | `KHS download failed` (or current fallbackMsg) |

Cache hit **never** returns 401/503 — it does not contact LMS.

Error classification reuses `apperror.IsInfrastructureError` / `IsCredentialError` / `ClassifyDocumentError` (`internal/apperror/classify.go`) — same keyword sets as `loginErrorKeyword` in `lms_service.go`.

## 6. Detailed Design

### 6.1 Service: `DownloadKHSFile`

Pseudocode for the new method:

```go
func (s *lmsDocumentService) DownloadKHSFile(req entity.KHSDownloadRequest) (string, int64, error) {
    req.Semester = strings.ToUpper(strings.TrimSpace(req.Semester))
    req.TahunAjaran = strings.TrimSpace(req.TahunAjaran)
    if !entity.ValidSemester(req.Semester) {
        return "", 0, apperror.BadRequest("semester must be GANJIL or GENAP")
    }
    if req.TahunAjaran == "" {
        return "", 0, apperror.BadRequest("tahun_ajaran is required")
    }

    filePath := filepath.Join(s.cfg.App.DownloadDir, req.NPM, "khs",
        entity.KHSFilename(req.TahunAjaran, req.Semester))

    // Fast path: serve if already on disk — no LMS, no lock.
    if info, err := os.Stat(filePath); err == nil {
        slog.Info("serving KHS file (cache hit)", "npm", req.NPM, "path", filePath, "size", info.Size())
        return filePath, info.Size(), nil
    } else if !os.IsNotExist(err) {
        return "", 0, apperror.Internal("failed to stat PDF file", err)
    }

    // Cache miss: fallback download — same steps as DownloadKHS, under per-NPM lock.
    mu := s.getNPMLock(req.NPM)
    mu.Lock()
    defer mu.Unlock()

    // Re-check after acquiring lock (another goroutine may have downloaded).
    if info, err := os.Stat(filePath); err == nil {
        slog.Info("serving KHS file (cache hit after lock)", "npm", req.NPM, "path", filePath)
        return filePath, info.Size(), nil
    }

    session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
    if err != nil {
        return "", 0, fmt.Errorf("get session: %w", err) // classified in handler
    }
    defer session.Close()

    q := url.Values{}
    q.Set("tahun_ajaran", req.TahunAjaran)
    q.Set("semester", req.Semester)
    detailURL := s.cfg.App.LMSBaseURL + port.KHSDetailPath + "&" + q.Encode()
    slog.Info("navigating to KHS detail (fallback)", "url", detailURL)

    if err := session.Navigate(detailURL); err != nil {
        return "", 0, fmt.Errorf("navigate to KHS detail: %w", err)
    }

    href, err := session.ElementHref(port.SelKHSCetakBtn)
    if err != nil {
        return "", 0, fmt.Errorf("find CETAK KHS button: %w", err)
    }

    pdfURL := s.cfg.App.LMSBaseURL + "/admin/" + href
    slog.Info("downloading KHS PDF (fallback)", "url", pdfURL)

    filename, size, err := session.DownloadPDF(pdfURL, filePath)
    _ = filename
    if err != nil {
        return "", 0, fmt.Errorf("download KHS PDF: %w", err)
    }

    info, err := os.Stat(filePath)
    if err != nil {
        return "", 0, apperror.Internal("failed to stat PDF file", err)
    }
    _ = size
    slog.Info("serving KHS file (fallback complete)", "npm", req.NPM, "path", filePath, "size", info.Size())
    return filePath, info.Size(), nil
}
```

Notes:
- Normalize before `KHSFilename` and before `ValidSemester` — matches `DownloadKHS` (`strings.ToUpper`).
- Fast-path `os.Stat` outside lock for latency; double-check inside lock to avoid duplicate downloads.
- Per-NPM lock (`sync.Map` in `lmsDocumentService`) — distinct from `session.Manager.npmLocks`; prevents two concurrent fallbacks for same NPM+semester from corrupting the same `DownloadPDF` target. `DownloadKHS` already uses the same pattern.
- `url.Values.Encode` handles `tahun_ajaran` slash (`2022/2023` → `2022%2F2023`) — matches prior P0 fix (`c6813dd`).
- Errors from fallback are wrapped with context (`get session:`, `navigate to KHS detail:`, `download KHS PDF:`) so `apperror.ClassifyDocumentError` can match `infraKeywords` / `credentialKeywords`. `AppError` (400/404) passes through via `errors.As` check first.

### 6.2 Handler: `DownloadKHSFile`

No behavioral change beyond error propagation. Current handler already:
- Validates `npm`, `password`, `tahun_ajaran`, `semester`.
- Calls `docService.DownloadKHSFile` and on error `return err` (Fiber error handler maps `*AppError` to JSON; wrapped `infra` errors become 500 unless classified).
- Should ensure fallback errors get `ClassifyDocumentError` mapping to 401/503. Options:
  - (Chosen) Keep handler as `return err` and let service-level wrapped errors be classified by the global `fibererror` / `ClassifyDocumentError` call if the handler adds `return apperror.ClassifyDocumentError(err, "KHS download failed")` on non-AppError. Check current `document_handler.go:134` — it does `return err` raw; the other handlers (`DownloadKHS`, `GetKHSSemesters`) do `return apperror.ClassifyDocumentError(err, ...)`. Align `DownloadKHSFile` fallback path with that pattern: if the cache-hit path already handles `*AppError`, the fallback error path should be `return apperror.ClassifyDocumentError(err, "KHS download failed")` so infra→503 and credential→401 are surfaced correctly. This is a one-line handler fix.

Success path unchanged:
```go
c.Set("Content-Type", "application/pdf")
c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="KHS_%s_%s_%s.pdf"`, req.NPM, strings.ReplaceAll(req.TahunAjaran, "/", "_"), req.Semester))
c.Set("Content-Length", strconv.FormatInt(size, 10))
c.Set("Accept-Ranges", "bytes")
return c.SendFile(filePath)
```

### 6.3 Concurrency & Idempotency

- Cache hit: lock-free, safe for concurrent readers (`os.Stat` + `SendFile`).
- Cache miss: per-NPM lock serializes fallback downloads for the same NPM. Different NPMs download in parallel.
- Double-check after lock prevents two concurrent misses from both downloading.
- `DownloadPDF` overwrites the target — last writer wins, which matches `DownloadKHS` semantics. No partial-file serve: handler only `Stat`s after `DownloadPDF` returns.

### 6.4 Observability

- Cache hit: `slog.Info("serving KHS file (cache hit)", ...)` — already exists, keep.
- Fallback: `slog.Info("navigating to KHS detail (fallback)", ...)` + `slog.Info("downloading KHS PDF (fallback)", ...)` + completion log — mirrors `DownloadKHS` logs, makes the two paths distinguishable.
- Error logs: fallback errors are wrapped and classified; 503 vs 401 is visible via `PublicMsg`.

## 7. Alternatives Considered

| Approach | Trade-off | Verdict |
|----------|-----------|---------|
| B) `?fallback=auto|never` flag | Explicit control; useful if eval pipeline must never hit LMS | Deferred — YAGNI now; adds contract/docs/tests for little value. Can be added later without breaking A. |
| C) Async 202 + poll | Avoids blocking FE for 60s on miss | Overkill for "sedikit perubahan"; changes sync PDF contract, requires job store and FE changes. Deferred. |

## 8. Testing Plan

### Unit — `tests/service/khs_file_service_test.go`

- `cache hit serves without LMS` — create file on disk, mock `SessionManager` that fails if `GetOrCreate` is called, assert `DownloadKHSFile` returns path/size and `GetOrCreate` was not invoked.
- `cache miss fallback success` — no file on disk, mock session `Navigate`/`ElementHref`/`DownloadPDF` to create the file, assert fallback creates and returns it.
- `cache miss re-check after lock` — pre-create file between outer Stat and lock (via mock), assert no download occurs.
- `cache miss invalid semester` → `*AppError` 400, no LMS hit.
- `cache miss credential failure` → wrapped error classified as 401 (assert `ClassifyDocumentError` in handler test).
- `cache miss infra failure` → wrapped error classified as 503.
- Semester normalization: `ganjil` lowercase succeeds.

Reuse existing `mockSessionManager` / temp `downloadDir` helpers; add `mockBrowserSession` with `Navigate`/`ElementHref`/`DownloadPDF` stubs as in `khs` download tests.

### Handler — `tests/handler/khs_file_handler_test.go`

- Valid request with existing PDF → 200 PDF.
- Valid request with missing PDF but fallback mock returns file → 200 PDF.
- Missing PDF + service returns `NotFound` → 404.
- Missing PDF + service returns wrapped `EOF`/`timeout` → 503 via `ClassifyDocumentError`.
- Missing PDF + service returns `login gagal` → 401.
- Invalid semester → 400 (handler validation before service).

### Manual smoke

- `curl -X POST /api/v1/lms/khs/file -d '{"npm":"...","password":"...","tahun_ajaran":"2022/2023","semester":"GANJIL"}'` — (a) semester already downloaded → <50ms, no `navigating to KHS detail` log; (b) new semester → log shows `navigating to KHS detail (fallback)` then PDF served.

## 9. Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Fallback downloads with wrong `tahun_ajaran`/`semester` encoding | Reuse `url.Values.Encode` and `KHSFilename` — already proven in `DownloadKHS` |
| Cold-start still hits on first miss | Accepted — miss is rare; P0 retries (`Browser.Page` backoff `[0,1s,2s]`, DNS 5s) already mitigate |
| Stale/wrong PDF served on hit (user changed password, PDF rotated on LMS) | Hit path intentionally skips LMS — if freshness is needed, caller should use `POST /lms/khs` (always re-download). Document this in API docs. |
| Disk `os.Stat` permission error | Return `Internal` (500) — not 404 — via `apperror.Internal` |

## 10. Rollout

- No config change, no migration.
- Backward compatible: callers sending `{npm, password, tahun_ajaran, semester}` and expecting PDF binary are unaffected; only latency/error-rate improves.
- Docs: update `docs/api/05_KHS_DOWNLOAD.md` (or add `KHS_FILE` doc) to describe cache-hit-no-LMS vs miss-fallback.

## 11. Open Questions (resolved)

- Q: Should `password` be optional on cache hit? A: No — keep it required at handler level to keep the contract simple and to allow fallback without a second request. Future optimization could make it optional, but not in this change.
- Q: Should hit path return 304 / ETag? A: No — out of scope; `SendFile` already handles `Accept-Ranges`.
