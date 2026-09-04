# Login Reliability Fixes Design

**Date:** 2026-09-04
**Status:** Approved Design
**Implements:** Telemetry findings from `tmp/edge-case-aggressive-eof.mjs` and `tmp/edge-case-stress.mjs` (2026-09-04)

## Overview

Add defensive retry logic to two browser-related operations that produce
intermittent HTTP 503 responses during cold-start and high concurrent load:

1. **`Browser.Page()` cold-start race** — `Browser.Page()` calls
   `rod.Page(proto.TargetCreateTarget{URL: url})` immediately after the
   browser launches. The CDP websocket may not be fully ready to handle
   `Target.createTarget`, returning `EOF` on the first attempt. The
   second or third attempt typically succeeds because the websocket
   has had time to flush its buffers.

2. **`Manager.checkDNS()` Docker-internal DNS overload** — Docker's
   embedded DNS resolver at `127.0.0.11:53` becomes overloaded under
   high concurrent load (10+ parallel login requests), causing
   `lookup ... i/o timeout`. The DNS cache usually warms within a
   second, so a retry succeeds.

The existing `Navigate()` method already implements this pattern (3
retries with exponential backoff for transient errors). This design
extends the same pattern to `Page()` and `checkDNS()`.

The previously-shipped login error classifier (commit history: `feat`
branch `feat/cold-start-tier1-tier2-fixes`) correctly classifies these
as infrastructure failures with HTTP 503 — that work is preserved.

## Architecture

```
Login request
  POST /api/v1/lms/login {npm, password}
            ↓
LMSHandler.Login (validates NPM, calls lmsService.Login)
            ↓
LMSService.Login → Manager.GetOrCreate
            ↓
Manager.createNewSession
  ├── checkDNS (NEW: 3x retry for transient DNS errors)
  ├── createSessionWithRestore  ─┐
  └── OR createSession (full)    │ BOTH call:
                                 │ br.Page(url) (NEW: 3x retry for transient CDP errors)
  └── cache session
```

The retry logic is implemented at the lowest layer where the transient
error originates (`Browser.Page()` and `Manager.checkDNS()`). All callers
inherit the improvement without code changes.

## Components

### 1. `Browser.Page()` Retry (`internal/infrastructure/browser/browser.go`)

Modify the existing `Page()` method (currently at line 217) to retry on
transient CDP errors. Add a new helper `isTransientBrowserError()`.

```go
// isTransientBrowserError reports whether err is a transient CDP/browser
// failure worth retrying (EOF, context deadline, timeout). Non-transient
// errors (e.g. "context canceled", invalid URL) fail immediately.
func isTransientBrowserError(err error) bool {
    if err == nil {
        return false
    }
    msg := strings.ToLower(err.Error())
    return strings.Contains(msg, "eof") ||
        strings.Contains(msg, "deadline") ||
        strings.Contains(msg, "timeout")
}

// Page opens a new browser tab. Retries 3x with exponential backoff on
// transient CDP errors (EOF, deadline, timeout) — addresses the cold-start
// race where Page() is called before the CDP websocket is fully ready.
//
// On non-transient errors (context canceled, invalid args), returns
// immediately — no retry, no backoff.
func (b *Browser) Page(url string) (*rod.Page, error) {
    const maxAttempts = 3
    backoffs := []time.Duration{0, 500 * time.Millisecond, 1 * time.Second}

    var lastErr error
    for attempt := 0; attempt < maxAttempts; attempt++ {
        if backoffs[attempt] > 0 {
            time.Sleep(backoffs[attempt])
        }
        page, err := b.rod.Page(proto.TargetCreateTarget{URL: url})
        if err == nil {
            if attempt > 0 {
                slog.Debug("Page() succeeded after retry",
                    "url", url, "attempt", attempt+1)
            }
            return page, nil
        }
        lastErr = err
        if !isTransientBrowserError(err) {
            // Non-transient error — don't retry.
            break
        }
        slog.Warn("Page() transient failure, will retry",
            "url", url, "attempt", attempt+1, "error", err)
    }
    return nil, fmt.Errorf("open page %s after %d attempts: %w",
        url, maxAttempts, lastErr)
}
```

**New imports needed in `browser.go`**: `strings` (already imported).

### 2. `Manager.checkDNS()` Retry (`internal/infrastructure/session/manager.go`)

Modify the existing `checkDNS()` method (currently at line 436) to retry
on transient DNS errors. Add a new helper `isTransientDNSError()`.

```go
// isTransientDNSError reports whether err is a transient DNS failure
// worth retrying (timeout, no such host, network unreachable). Parse
// errors and protocol errors fail immediately.
func isTransientDNSError(err error) bool {
    if err == nil {
        return false
    }
    msg := strings.ToLower(err.Error())
    return strings.Contains(msg, "timeout") ||
        strings.Contains(msg, "no such host") ||
        strings.Contains(msg, "network is unreachable") ||
        strings.Contains(msg, "connection refused")
}

// checkDNS verifies that the LMS hostname resolves before browser launch.
// Retries up to 3 times on transient DNS failures (i/o timeout, no such
// host, network unreachable) — addresses the Docker-internal DNS resolver
// (127.0.0.11:53) becoming overloaded under high concurrent load.
//
// The first attempt uses the configured DNSTimeout; subsequent attempts
// use the same timeout (no exponential backoff — DNS cache usually warms
// fast).
func (m *Manager) checkDNS(rawURL string) error {
    u, err := url.Parse(rawURL)
    if err != nil {
        return fmt.Errorf("DNS check: invalid LMS URL %q: %w", rawURL, err)
    }
    host := u.Hostname()

    dnsTimeout := m.cfg.App.DNSTimeout
    if dnsTimeout == 0 {
        dnsTimeout = 5 * time.Second
    }

    const maxAttempts = 3
    var lastErr error
    for attempt := 0; attempt < maxAttempts; attempt++ {
        ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
        resolver := &net.Resolver{}
        addrs, err := resolver.LookupHost(ctx, host)
        cancel()
        if err == nil {
            if attempt > 0 {
                slog.Debug("DNS resolved after retry",
                    "host", host, "addrs", addrs, "attempt", attempt+1)
            } else {
                slog.Debug("DNS resolved", "host", host, "addrs", addrs)
            }
            return nil
        }
        lastErr = err
        if !isTransientDNSError(err) {
            break
        }
        slog.Warn("DNS check transient failure, will retry",
            "host", host, "attempt", attempt+1, "error", err)
    }
    return fmt.Errorf(
        "DNS check: cannot resolve LMS host %q after %d attempts: %w. "+
            "Verify your DNS settings (try setting DNS to 8.8.8.8)",
        host, maxAttempts, lastErr)
}
```

**No new imports needed in `manager.go`** (`strings`, `context`, `net`,
`url`, `time`, `log/slog` already imported).

### 3. `DNSTimeout` Default Config (`internal/config/config.go`)

Change the default value of `DNSTimeout` from 2s to 5s, and update the
comment to reflect the new operational rationale.

```go
// DNS_TIMEOUT default raised from 2s → 5s: telemetry from
// tmp/edge-case-stress.mjs shows Docker internal DNS (127.0.0.11:53)
// needs 3-5s during heavy concurrent load before the retry (handled
// in manager.checkDNS) can succeed. Operators can still override with
// DNS_TIMEOUT env var.
DNSTimeout: getEnvDuration("DNS_TIMEOUT", 5*time.Second),
```

## Error Handling

| Layer | Error | Before | After |
|-------|-------|--------|-------|
| `Browser.Page()` | EOF (transient) | Fail with HTTP 503 (cold-start race) | Retry 3x → success on retry #2 or #3 (typical) |
| `Browser.Page()` | context canceled | Fail immediately | Same — fail immediately (non-transient) |
| `Browser.Page()` | Invalid URL | Fail immediately | Same — fail immediately (non-transient) |
| `Browser.Page()` | All retries exhausted | n/a | Wrap with `fmt.Errorf("open page %s after %d attempts: %w", ...)` — preserves original error |
| `Manager.checkDNS()` | i/o timeout (transient) | Fail with HTTP 503 | Retry 3x → success on retry #2 or #3 (typical) |
| `Manager.checkDNS()` | no such host (transient) | Fail with HTTP 503 | Retry 3x → success on retry #2 or #3 (typical) |
| `Manager.checkDNS()` | Parse error | Fail immediately | Same — fail immediately (non-transient) |
| `Manager.checkDNS()` | All retries exhausted | n/a | Wrap with `cannot resolve ... after 3 attempts: ...` |
| `LMSLogin.Login()` | `Browser.Page()` or `checkDNS()` failed | HTTP 503 with infra-failure message | Same — classifier (already shipped) maps to 503 unchanged |

The HTTP response shape and status codes to clients are unchanged.
The classifier in `internal/application/service/lms_service.go` (from
the previously-shipped cold-start fixes) continues to map these errors
correctly.

## Testing

### Unit Tests

#### New: `tests/infrastructure/browser/page_retry_test.go`

5 tests using the existing manual-mock pattern (function-field structs):

1. **`TestPage_SuccessOnFirstCall`** — mock returns `*rod.Page, nil` on
   first call. Verify `Page()` returns page, no retry log emitted.
2. **`TestPage_RetryOnEOF_SuccessOnSecondCall`** — mock returns
   `EOF` on first call, `*rod.Page, nil` on second. Verify page
   returned, no error.
3. **`TestPage_RetryExhausted_ReturnsWrappedError`** — mock returns
   `EOF` on all 3 calls. Verify error contains `"after 3 attempts"`
   and wraps the original error.
4. **`TestPage_NonTransientError_NoRetry`** — mock returns
   `"context canceled"` on first call. Verify error returned
   immediately, mock called exactly once.
5. **`TestPage_BackoffTiming`** — all 3 calls return EOF. Verify
   total elapsed time ≥ 500ms + 1s = 1.5s (cumulative backoff).

#### New: `tests/infrastructure/session/check_dns_retry_test.go`

5 tests using mock session manager pattern:

1. **`TestCheckDNS_SuccessOnFirstCall`** — mock `net.Resolver` returns
   addrs on first call. Verify `checkDNS()` returns nil.
2. **`TestCheckDNS_RetryOnTimeout_SuccessOnSecondCall`** — mock returns
   timeout error on first call, addrs on second. Verify returns nil.
3. **`TestCheckDNS_RetryExhausted_ReturnsWrappedError`** — mock returns
   timeout on all 3 calls. Verify error contains `"after 3 attempts"`.
4. **`TestCheckDNS_ParseError_FailsImmediately`** — pass malformed URL.
   Verify returns parse error, no retry attempt.
5. **`TestCheckDNS_DefaultTimeout_WhenZero`** — set `DNSTimeout = 0`,
   verify default of 5s is used internally.

For mocking DNS, use a test-injectable `dnsLookupFunc` field on `Manager`
(only set in tests) OR use a real DNS lookup against a localhost stub.
The former is cleaner.

#### Update: `tests/config/dns_timeout_test.go`

Update existing tests for new default:

1. Existing tests with no env var → expect default `5 * time.Second`
   (was 2s).
2. Existing test for `DNS_TIMEOUT=10s` env override → unchanged, still
   10s.

### Telemetry Verification

Existing scripts in `tmp/` continue to work and provide post-patch
verification:

1. **`tmp/edge-case-aggressive-eof.mjs`** — 6 aggressive scenarios.
   Pre-patch: 0 fresh EOF triggered (cached log only). Post-patch:
   expect same result, but if EOF was triggered, expect retry to
   succeed before HTTP 503 is returned.
2. **`tmp/edge-case-stress.mjs`** — 4 stress scenarios. Pre-patch:
   Skenario J shows 20/20 HTTP 503 from DNS timeout. Post-patch:
   expect reduced 503 count (DNS retry succeeds on most attempts).

Manual verification:

```bash
curl -X POST http://127.0.0.1:3000/api/v1/lms/login \
  -H "Content-Type: application/json" \
  -d '{"npm":"2211700006","password":"Izzan027"}' \
  -w "\nHTTP %{http_code} | time=%{time_total}s\n"
```

Pre-patch: 503 with infra-failure message (cold-start EOF).
Post-patch: 200 (login success) or 401 (classifier still works
correctly for credential failures).

## Migration

### Phase A — Implementation (atomic, single commit)

Apply all 3 production patches:
- `internal/infrastructure/browser/browser.go`
- `internal/infrastructure/session/manager.go`
- `internal/config/config.go`

Verify:
- `go build ./cmd/... ./internal/...` succeeds
- All existing tests pass (`go test ./tests/...`)

### Phase B — Unit Tests

- Add 2 new test files (`page_retry_test.go`, `check_dns_retry_test.go`)
- Update 1 existing test (`dns_timeout_test.go`)
- Run `go test ./...` — all new tests pass

### Phase C — Image Build & Deploy

- `docker build -t ghcr.io/mochizzan/lonceng_unman_be:latest .`
- `docker compose -f compose.yml up -d --force-recreate lonceng-api`
- Container restart preserves `data/` volumes (downloaded PDFs,
  session profiles)

### Phase D — Verification

- Run `node tmp/edge-case-aggressive-eof.mjs` — expect reduced EOF 503
- Run `node tmp/edge-case-stress.mjs` — expect reduced DNS 503
- Manual `curl /api/v1/lms/login` — expect 200 or 401 (not 503)

### Phase E — Rollback (if Phase D fails)

```bash
git checkout HEAD~1 -- internal/infrastructure/browser/browser.go \
  internal/infrastructure/session/manager.go internal/config/config.go
docker build -t ghcr.io/mochizzan/lonceng_unman_be:latest .
docker compose -f compose.yml up -d --force-recreate lonceng-api
```

Existing behavior is restored.

## Backwards Compatibility

- `Browser.Page()` signature unchanged
- `Manager.checkDNS()` signature unchanged (private method)
- `Config.AppConfig.DNSTimeout` type unchanged (`time.Duration`)
- Existing `DNS_TIMEOUT` env var still respected (operators who set
  their own value won't be affected by the new 5s default)
- All 14 existing callers of `Page()` automatically inherit retry benefit
- HTTP response shapes and status codes to clients are unchanged
  (classifier still maps to 503/401/200 correctly)

## Risks

- **Low**: Retry logic could mask underlying issues (transient errors
  become silent). Mitigation: `slog.Warn` on every retry attempt with
  attempt number and error message.
- **Low**: Retry adds latency worst-case (cold-start EOF + 3x retry +
  backoff adds ~3.5s to login). Mitigation: exponential backoff caps
  at 2s; first retry at 500ms catches the common case quickly.
- **Negligible**: Existing tests don't cover retry path. Mitigation:
  new unit tests cover happy & failure paths.

## Files Modified

| File | Change | LOC |
|------|--------|-----|
| `internal/infrastructure/browser/browser.go` | `Page()` retry + helper | +30 |
| `internal/infrastructure/session/manager.go` | `checkDNS()` retry + helper | +30 |
| `internal/config/config.go` | `DNSTimeout` default 2s → 5s + comment | ±5 |
| `tests/infrastructure/browser/page_retry_test.go` | NEW: 5 unit tests | +120 |
| `tests/infrastructure/session/check_dns_retry_test.go` | NEW: 5 unit tests | +120 |
| `tests/config/dns_timeout_test.go` | UPDATE: 2 tests for new default | ±20 |

**Total**: 3 production files modified, 2 new test files, 1 updated
test file. Estimated ~325 LOC.

## References

- Telemetry findings: `tmp/edge-case-aggressive-eof.mjs`,
  `tmp/edge-case-stress.mjs`, `tmp/edge-case-cdp-race.mjs`,
  `tmp/debug-login-timing.mjs` (all dated 2026-09-04)
- Existing retry pattern: `rodSession.Navigate()` in
  `internal/infrastructure/session/session.go:75-115`
- Error classifier: `internal/application/service/lms_service.go`
  (login infrastructure failure → HTTP 503 mapping, shipped
  2026-09-04)
- Branch: `feat/cold-start-tier1-tier2-fixes` (working branch)
