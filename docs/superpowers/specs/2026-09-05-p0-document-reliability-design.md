# P0 Document Reliability Design

**Date:** 2026-09-05
**Status:** Approved Design
**Branch:** `feat/cold-start-tier1-tier2-fixes`
**Implements:** Audit findings T-1..T-8 from `2026-09-05 04:24:51–04:25:21` log analysis (npm `2211700006`, `POST /api/v1/lms/khs` burst 6 requests in 7 seconds: 5×200 then 1×500 `context deadline exceeded`)

## Overview

The LMS document family (`POST /api/v1/lms/khs`, `/khs/semesters`, `/krs`, plus `/khs/extract`) is unstable under same-NPM bursts and misreports infrastructure failures. This P0 package fixes four root-cause clusters without changing any `port` interface:

1. **Same-NPM concurrency hang** — 2 concurrent `DownloadKHS` for the same NPM run `Navigate` in parallel on the same Chrome `--user-data-dir` process. Burst 6 in 7 seconds saturates the per-browser CDP queue → the 6th `Navigate` fails `context deadline exceeded` after 3 attempts (4.8s in the log, not 180s — CDP hung instantly). `Page close` also hangs (`Page close error: context deadline exceeded`).

2. **Wrong status code** — `DocumentHandler.DownloadKHS` and `GetKHSSemesters` wrap every error as `apperror.Internal` → HTTP 500. Infrastructure failures (`deadline`/`EOF`/`timeout`/`DNS check`/`open page after 3 attempts`) should be `503 Service Unavailable` with the same public message as `Login`, so the Flutter client can retry. Credential failures should be `401`. `extraction_service.verifySession` is worse: it maps *all* `GetOrCreate` errors to `401`, turning infra failures into "wrong password".

3. **Unescaped query params** — `lms_service.DownloadKHS` builds `detailURL` with `fmt.Sprintf("...&tahun_ajaran=%s&semester=%s", tahunAjaran, semester)` where `tahunAjaran` is `2025/2026` containing `/`. Without `url.QueryEscape` the slash is not encoded (`%2F`), which can trigger a 302/400 and a `WaitLoad` that never completes. `GANJIL` happened to be tolerated, `GENAP` was not.

4. **Data races** — `activeCount = 1` without lock, `activeCount--` in `Close` without lock, `touchLastUsed` locks `rodSession.mu` while readers lock `cachedSess.mu` (different mutex, same field), `newSession` failure leaks `activeCount` (incremented but never decremented, zombie until the 2h hard limit), and `pageCount--` without a `>0` guard.

Non-goals for this P0: `singleflight` dedup, `fmt.Printf [SESSION]` → `slog` with `trace_id`, Fiber request timeout middleware, single-decode PDF, `time.LoadLocation` caching — all deferred to P1 to keep the P0 diff reviewable in one PR.

## Architecture

Reuse the existing Clean Architecture — no new layers, no `port` interface changes.

```
handler/document_handler.go ──┐
                              ├──→ service/lms_service.go  ← per-NPM lock (phase-limited) + QueryEscape
handler/extraction_handler.go ┘         │
                                        ├──→ port.SessionManager.GetOrCreate (reuse existing npmLocks)
                                        └──→ port.BrowserSession.Navigate / ElementHref / DownloadPDF
                              handler: classifyDocumentError(err) → 503 / 401 / 500

manager.go / session.go  ← race fixes in place (no interface change)
```

Chosen approach: **service-local** (approved Approach 1). The per-NPM lock lives in `lmsDocumentService` as its own `sync.Map` (distinct from `Manager.npmLocks`). It serializes only `lmsDocumentService.DownloadKHS/GetKHSSemesters/DownloadKRS` for the same NPM; it does not lock `student_profile_service` or `extraction_service` for the same NPM (they use different browser phases and are not part of the burst that caused T-1). This keeps `port` unchanged while avoiding two maps contending for the same key.

## Components

### 1. Per-NPM Browser-Phase Lock (`internal/application/service/lms_service.go`)

Lock only the browser-heavy phase for the **same NPM**; different NPMs stay fully parallel, and local reads (`POST /khs/data`, `POST /khs/extract` via `findKHSFile`/`ParseKHS`) stay parallel even for the same NPM.

```go
// lms_service.go — new field on lmsDocumentService
type lmsDocumentService struct {
    cfg       *config.Config
    sessions  port.SessionManager
    npmLocks  sync.Map // map[string]*sync.Mutex, per-NPM, phase-limited
}

func (s *lmsDocumentService) getNPMLock(npm string) *sync.Mutex {
    v, _ := s.npmLocks.LoadOrStore(npm, &sync.Mutex{})
    return v.(*sync.Mutex)
}

// DownloadKHS — lock only Navigate + ElementHref + DownloadPDF
func (s *lmsDocumentService) DownloadKHS(req entity.KHSDownloadRequest) (*entity.KHSDownloadResult, error) {
    req.Semester = strings.ToUpper(strings.TrimSpace(req.Semester))
    req.TahunAjaran = strings.TrimSpace(req.TahunAjaran)
    // TrimSpace must precede ToUpper + ValidSemester so " ganjil " is accepted
    if !entity.ValidSemester(req.Semester) {
        return nil, fmt.Errorf("semester must be GANJIL or GENAP")
    }
    if req.TahunAjaran == "" {
        return nil, apperror.BadRequest("tahun_ajaran is required")
    }

    mu := s.getNPMLock(req.NPM)
    mu.Lock()
    defer mu.Unlock()

    session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
    if err != nil {
        return nil, fmt.Errorf("get session: %w", err)
    }
    defer session.Close()

    // QueryEscape via url.Values (slash → %2F)
    q := url.Values{}
    q.Set("tahun_ajaran", req.TahunAjaran)
    q.Set("semester", req.Semester)
    detailURL := s.cfg.App.LMSBaseURL + port.KHSDetailPath + "&" + q.Encode()

    if err := session.Navigate(detailURL); err != nil {
        return nil, fmt.Errorf("navigate to KHS detail: %w", err)
    }
    href, err := session.ElementHref(port.SelKHSCetakBtn)
    if err != nil {
        return nil, fmt.Errorf("find CETAK KHS button: %w", err)
    }
    pdfURL := s.cfg.App.LMSBaseURL + "/admin/" + href
    savePath := filepath.Join(s.cfg.App.DownloadDir, req.NPM, "khs", entity.KHSFilename(req.TahunAjaran, req.Semester))
    filename, size, err := session.DownloadPDF(pdfURL, savePath)
    // ...
}
```

Apply the same pattern to `GetKHSSemesters` (lock around `Navigate` + `Eval`) and `DownloadKRS` (around `Navigate` + `ElementAttribute` + `DownloadPDF`). Do **not** lock `DownloadKHSFile` (local `os.Stat`), `GetKHSExtraction`/`GetKRSExtraction`, or `ExtractKHS`/`ExtractKRS` beyond the existing `verifySession` call.

`sync`, `net/url`, and `strings` are already imported in `lms_service.go` or are trivial to add. `apperror` already imported.

### 2. Error Classification (`internal/interfaces/http/handler/document_handler.go`, `internal/application/service/extraction_service.go`, `internal/apperror` or `internal/application/service`)

Add a shared helper that reuses the existing Login classifier keywords and the browser/DNS transient predicates.

```go
// handler/document_handler.go (or apperror/classify.go — either location is acceptable;
// reuse the existing loginErrorKeyword + IsTransientBrowserError / IsTransientDNSError)
func classifyDocumentError(err error) *apperror.AppError {
    if err == nil {
        return apperror.Internal("unknown document error", fmt.Errorf("nil error"))
    }
    // Pass through explicit AppError first — 400/404 from validation must not be
    // reclassified as 503/401 when their message happens to contain "timeout" etc.
    var appErr *apperror.AppError
    if errors.As(err, &appErr) {
        return appErr
    }
    msg := err.Error()
    kind := classifyLoginErrorKind(msg) // reuse from lms_service.go — extract to apperror if needed
    // Also check IsTransientBrowserError / IsTransientDNSError for wrapped errors
    if kind == loginErrorKindInfrastructure || browser.IsTransientBrowserError(err) || session.IsTransientDNSError(err) {
        return &apperror.AppError{
            StatusCode: fiber.StatusServiceUnavailable,
            PublicMsg:  "Layanan LMS tidak dapat diakses saat ini. Silakan coba lagi dalam beberapa saat.",
            Internal:   err,
        }
    }
    if kind == loginErrorKindCredential {
        return apperror.Unauthorized("Username atau password salah")
    }
    return apperror.Internal("KHS download failed", err)
}
```

Wire in handlers:

```go
// document_handler.go
func (h *DocumentHandler) DownloadKHS(c fiber.Ctx) error {
    // ... validate NPM/password/tahunAjaran/semester (400) ...
    result, err := h.docService.DownloadKHS(req)
    if err != nil {
        return classifyDocumentError(err)
    }
    return response.Success(c, fiber.StatusOK, result, result.Message)
}
func (h *DocumentHandler) GetKHSSemesters(c fiber.Ctx) error {
    // ... validate ...
    result, err := h.docService.GetKHSSemesters(req)
    if err != nil {
        return classifyDocumentError(err)
    }
    return response.Success(c, fiber.StatusOK, result, result.Message)
}
// DownloadKHSFile keeps its own 404 path (os.IsNotExist → 404), not classified as infra.

// extraction_service.go — internal/application/service/extraction_service.go:46
func (s *extractionService) verifySession(npm, password string) error {
    session, err := s.sessions.GetOrCreate(npm, password)
    if err != nil {
        // Distinguish infra from credential — do not blindly return 401
        if browser.IsTransientBrowserError(err) || session.IsTransientDNSError(err) ||
            strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") ||
            strings.Contains(strings.ToLower(err.Error()), "dns check") {
            return &apperror.AppError{
                StatusCode: fiber.StatusServiceUnavailable,
                PublicMsg:  "Layanan LMS tidak dapat diakses saat ini. Silakan coba lagi dalam beberapa saat.",
                Internal:   err,
            }
        }
        return apperror.Unauthorized("Username atau password salah")
    }
    session.Close()
    return nil
}
```

If `classifyLoginErrorKind` / `loginErrorKeyword` are unexported in `lms_service.go`, extract them to `internal/apperror/classify.go` so both `lms_service.go` and handlers can share them. No behavior change for `Login` — it keeps its existing classifier.

### 3. URL QueryEscape + Validation (`internal/application/service/lms_service.go`)

All places that interpolate `tahun_ajaran` / `semester` / `npm` into a URL query must encode:

- `DownloadKHS`: `detailURL` construction (primary — observed in the log).
- `GetKHSSemesters`: if it ever interpolates query params (currently `KHSListPath` is static, no change needed unless a filter is added).
- `DownloadKHSFile`: `savePath` via `KHSFilename` already replaces `/` with `_`; no URL, no change, but validate `tahunAjaran` non-empty before building the path.

Add `strings.TrimSpace` on `tahunAjaran` and `semester` before validation and before `q.Encode()`. Return `apperror.BadRequest` for empty `tahunAjaran` after trimming (before acquiring the per-NPM lock, to avoid holding the lock for a validation failure).

### 4. Data-Race Fixes (`internal/infrastructure/session/manager.go`, `internal/infrastructure/session/session.go`)

Four targeted fixes (no interface changes, no new goroutines):

```go
// manager.go — createNewSession: guard activeCount with sess.mu
m.mu.Lock()
m.sessions[npm] = sess
m.mu.Unlock()
sess.mu.Lock()
sess.activeCount = 1
sess.mu.Unlock()

// manager.go — GetOrCreate fast path + double-check: rollback on newSession failure
sess.mu.Lock()
if time.Since(sess.lastUsed) < m.ttl {
    sess.lastUsed = time.Now()
    sess.activeCount++
    sess.mu.Unlock()
    newSess, err := newSession(sess)
    if err != nil {
        sess.mu.Lock()
        if sess.activeCount > 0 {
            sess.activeCount--
        }
        sess.mu.Unlock()
        m.evictSession(npm)
        return m.createNewSession(npm, password)
    }
    return newSess, nil
}
sess.mu.Unlock()

// session.go — touchLastUsed: lock cachedSess.mu, not rodSession.mu
func (s *rodSession) touchLastUsed() {
    s.cachedSess.mu.Lock()
    s.cachedSess.lastUsed = time.Now()
    s.cachedSess.mu.Unlock()
}
// Callers (Navigate/Eval/Element*/Download*) no longer need to hold s.mu for this call;
// they still hold s.mu for page ops.

// session.go — Close: guard activeCount with cachedSess.mu, guard pageCount decrement
if s.page != nil {
    // ... goroutine page.Close with 5s select, logging ...
    s.cachedSess.pageMu.Lock()
    if s.cachedSess.pageCount > 0 {
        s.cachedSess.pageCount--
    }
    fmt.Printf("[SESSION] Page closed, pageCount=%d\n", s.cachedSess.pageCount)
    s.cachedSess.pageMu.Unlock()
}
s.cachedSess.mu.Lock()
if s.cachedSess.activeCount > 0 {
    s.cachedSess.activeCount--
}
s.cachedSess.mu.Unlock()
```

No change to `pageTimeout` (60s), `Browser.Page` backoff `[0, 1s, 2s]`, `DNSTimeout` (5s), or `SessionTTL` (24h) — they remain as shipped in the current branch.

## Error Handling

| Source error | Detection | HTTP status | Public message | Internal log |
|---|---|---|---|---|
| `context deadline exceeded` / `EOF` / `timeout` / `open page after 3 attempts` / `wait login page load` / `browser connect` | `IsTransientBrowserError` or `strings.Contains(deadline/timeout/EOF/open page)` | 503 | `Layanan LMS tidak dapat diakses saat ini. Silakan coba lagi dalam beberapa saat.` | `internal` preserves original `navigate ... after 3 attempts: ...` |
| `DNS check` / `cannot resolve LMS host` / `i/o timeout` | `IsTransientDNSError` or `strings.Contains(DNS check/no such host)` | 503 | same 503 message | same |
| `username dan password` / `login gagal` / `page did not redirect` / `session expired` | `classifyLoginErrorKind == credential` | 401 | `Username atau password salah` | same |
| `npm is required` / `password is required` / `tahun_ajaran is required` / `semester must be GANJIL or GENAP` | handler validation before service call | 400 | validation message | none |
| `KHS PDF not found` (local `os.IsNotExist` in `DownloadKHSFile`) | `os.IsNotExist` | 404 | `KHS PDF not found for the specified year and semester` | wrapped `os.PathError` |
| unknown / programmer error | fallback | 500 | `KHS download failed` (or `fetch KHS semesters failed`) | `unhandled error` via `fibererror` |
| `nil` | defensive | 500 | `internal error: classifyDocumentError called with nil error` | `fmt.Errorf` with context |

`DownloadKRS` follows the same table. `verifySession` in `extraction_service` follows the same table instead of always returning 401.

`fibererror.New()` already logs `slog.Error("request error", method, path, status, internal)` for `*apperror.AppError` with `Internal != nil` — no change needed there.

## Testing

### Unit tests (new)

- `tests/service/lms_document_classifier_test.go` — table-driven. Each case constructs an error with a known substring (`context deadline exceeded`, `open page after 3 attempts`, `DNS check: cannot resolve`, `username dan password`, `page did not redirect to dashboard`, plain validation error) and asserts `classifyDocumentError` returns the expected `StatusCode` (503 / 401 / 400 / 500) and `PublicMsg`. Also asserts `QueryEscape`: `tahunAjaran="2025/2026"` → encoded URL contains `tahun_ajaran=2025%2F2026`.

- `tests/infrastructure/session/per_npm_lock_test.go` — concurrency. Mock `SessionManager` where `BrowserSession.Navigate` sleeps 200ms and counts concurrent calls via `atomic.Int32`. Case 1: 6 goroutines same NPM calling `DownloadKHS` → max concurrency == 1 and total wall-clock ≈ 6×200ms (sequential). Case 2: 2 goroutines different NPM → max concurrency == 2 and wall-clock ≈ 200ms (parallel). Case 3: `POST /khs/data` path (no browser lock) stays parallel even for same NPM.

- `tests/infrastructure/session/race_test.go` — race + leak. Subcases under `go test -race`: (a) `newSession` failure after `activeCount++` — assert `activeCount` rolled back to 0; (b) concurrent `touchLastUsed` vs `cleanup` read of `lastUsed` — no race; (c) concurrent `Close` `activeCount--` vs `GetOrCreate` `activeCount++` — no race; (d) double `Close` is idempotent (`closed` guard).

### Existing tests (must stay green)

- `go vet ./...`, `go test -race ./...` (all packages, especially `tests/infrastructure/session`, `tests/infrastructure/browser`, `tests/service`).

### Manual verification

```bash
go run cmd/server/main.go &
# Burst same NPM — expect sequential, no 500 hang
seq 6 | xargs -P 6 -I {} curl -s -X POST http://127.0.0.1:3000/api/v1/lms/khs \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"<real>","tahun_ajaran":"2025/2026","semester":"GENAP"}' \
  -w " %{http_code}\n" | sort | uniq -c
# Expect: all 200 (or 503 if LMS down), no 500 context deadline in slog

# Slash encoding
curl -s -X POST http://127.0.0.1:3000/api/v1/lms/khs \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"<real>","tahun_ajaran":"2025/2026","semester":"GANJIL"}' \
  | grep -q 200 && echo ok
```

## Migration

### Phase A — Production patch (single commit)

Apply in one commit on `feat/cold-start-tier1-tier2-fixes`:

- `internal/application/service/lms_service.go` — per-NPM `sync.Map` + `getNPMLock` + lock around `Navigate/ElementHref/DownloadPDF` in `DownloadKHS`/`GetKHSSemesters`/`DownloadKRS` + `url.Values` + `TrimSpace` + `tahunAjaran` empty check.
- `internal/interfaces/http/handler/document_handler.go` — `classifyDocumentError` + wire `DownloadKHS`/`GetKHSSemesters` through it.
- `internal/application/service/extraction_service.go` — `verifySession` infra→503 branch (the method lives at `extraction_service.go:46`, not in the handler).
- `internal/apperror/classify.go` (new, optional) — extract `loginErrorKeyword`/`classifyLoginErrorKind` if sharing across packages is needed; otherwise keep in `lms_service.go` and import predicates.
- `internal/infrastructure/session/manager.go` — `activeCount=1` under lock + rollback on `newSession` failure.
- `internal/infrastructure/session/session.go` — `touchLastUsed` under `cachedSess.mu`, `Close` `activeCount--` under `cachedSess.mu`, `pageCount--` guard `>0`.

Verify: `go build ./...` succeeds, `go vet ./...` clean, `go test ./tests/...` green.

### Phase B — Tests

Add 3 new test files, run `go test -race ./...` — all pass.

### Phase C — Deploy

```bash
docker build -t ghcr.io/mochizzan/lonceng_unman_be:latest .
docker compose -f compose.yml up -d --force-recreate lonceng-api
```

No `.env` change, no `compose.yml` change, no `port` interface change. `data/` volumes preserved.

### Phase D — Verify

Run the burst curl above; check `docker logs lonceng-api` no `[SESSION] Page close error: context deadline exceeded` under burst, and HTTP 503 (not 500) for infra failures.

### Phase E — Rollback

```bash
git revert <commit>
docker build -t ghcr.io/mochizzan/lonceng_unman_be:latest .
docker compose -f compose.yml up -d --force-recreate lonceng-api
```

Existing behavior restored (burst may hang again, but no new failure mode introduced).

## Backwards Compatibility

- `port.SessionManager` and `port.BrowserSession` signatures unchanged — all 6 existing callers and all mocks in `tests/` unaffected.
- `Browser.Page` backoff `[0, 1s, 2s]`, `pageTimeout` 60s, `DNSTimeout` 5s, `SessionTTL` 24h unchanged.
- HTTP response shapes unchanged — only `status` code changes for infra paths (500 → 503) and `PublicMsg` for those paths now matches the existing Login 503 message (already documented). Clients that already handle 503 from `/lms/login` handle it here identically.
- `KHSFilename` slash→underscore for filesystem unchanged; only the LMS query URL is now encoded.

## Risks

- **Low**: Per-NPM lock adds latency for same-NPM bursts (6 sequential × 3.8s ≈ 23s wall-clock vs 4s parallel). Mitigation: lock is phase-limited — only `Navigate+href+DownloadPDF` is serialized; validation and response marshaling stay outside the lock. P1 `singleflight` will reduce this to ~3.8s for identical `tahun:semester`.
- **Low**: Extracting `classifyLoginErrorKind` to `apperror` touches Login code. Mitigation: move is pure refactor, covered by existing `tests/service/lms_login_classifier_test.go`; keep Login behavior identical.
- **Negligible**: `TrimSpace` on `tahunAjaran` changes behavior for inputs with leading/trailing spaces (previously treated as distinct filenames/URLs). This is a fix, not a regression — spaces in `tahunAjaran` were always a bug.

## Files Modified

| File | Change | LOC |
|------|--------|-----|
| `internal/application/service/lms_service.go` | per-NPM `sync.Map` + lock around browser phase + `url.Values` + `TrimSpace` | +70 |
| `internal/interfaces/http/handler/document_handler.go` | `classifyDocumentError` + wire for `DownloadKHS`/`GetKHSSemesters` | +40 |
| `internal/application/service/extraction_service.go` | `verifySession` 503 branch (`:46`) | +20 |
| `internal/apperror/classify.go` | NEW (optional) — shared classifier extraction | +30 |
| `internal/infrastructure/session/manager.go` | `activeCount=1` guard + rollback | +15 |
| `internal/infrastructure/session/session.go` | `touchLastUsed` + `Close` guards | +15 |
| `tests/service/lms_document_classifier_test.go` | NEW: 6+ table cases | +120 |
| `tests/infrastructure/session/per_npm_lock_test.go` | NEW: 3 concurrency cases | +120 |
| `tests/infrastructure/session/race_test.go` | NEW: 4 race/leak cases | +100 |

**Total**: 5–6 production files modified, 1 optional new file, 3 new test files. Estimated ~510 LOC.

## References

- Audit: `2026-09-05` log analysis (T-1..T-8), `internal/infrastructure/session/session.go:74-246`, `internal/infrastructure/session/manager.go:24-636`, `internal/infrastructure/browser/browser.go:247-274`, `internal/application/service/lms_service.go:97-420`, `internal/interfaces/http/handler/document_handler.go:88-113`, `internal/infrastructure/fibererror/handler.go`, `internal/apperror/apperror.go`
- Log: `POST /api/v1/lms/khs` 04:24:51–04:25:21 burst, `[SESSION] Page close error: context deadline exceeded`, `navigate to KHS detail ... failed after 3 attempts: context deadline exceeded`
- Prior spec: `docs/superpowers/specs/2026-09-04-login-reliability-fixes-design.md` (branch `feat/cold-start-tier1-tier2-fixes`, commits `4763df5`, `6361f44`)
- Prior plan: `docs/superpowers/specs/2026-08-30-eval-page-performance-implementation-plan.md` (reference layout)
