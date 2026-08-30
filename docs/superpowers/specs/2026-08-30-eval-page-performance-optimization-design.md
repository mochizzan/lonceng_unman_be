# /eval Page Performance Optimization Design Doc

**Date**: 2026-08-30
**Author**: Main Agent (with parallel scout subagents)
**Status**: Approved — Ready for Implementation Plan

---

## 1. Problem Statement

### Current State
- `/eval` (Index/Dashboard) page takes **>200ms** to load DOM in local environment
- `/eval/:npm` (Student detail) page has redundant I/O that doubles disk reads
- `/eval/pipeline` page has ~670 DOM nodes from 11 phase cards

### Target State
- **<100ms** DOM load time for `/eval` Index page
- Zero redundant I/O in Student detail page
- Reduced initial paint cost for pipeline page

### Hard Constraints (Non-negotiable)
| Constraint | Status |
|-----------|--------|
| Must remain SSR (no CSR) | ✅ |
| No stale data from caching | ✅ |
| Data must be fresh from disk | ✅ |
| No client-side rendering | ✅ |

---

## 2. Root Cause Analysis

### 2.1 Redundant I/O in `EvalService.Student()` — **HIGH IMPACT**

**Location**: `internal/application/service/eval_service.go:216-220` + `:260-417`

```go
// Student() loop (line 215-234):
if doc.Paired {
    metrics, err := s.evaluateDoc(npm, doc.DocType, doc.File)        // ReadFile GT + Extract
    docEval.CompareRows = s.buildCompareRows(npm, doc.DocType, doc.File)  // ReadFile GT + Extract AGAIN
}
```

Both `evaluateDoc()` and `buildCompareRows()` independently call `LoadGT()` + `LoadExtract()`, resulting in **4× ReadFile per paired doc** instead of 2×.

**Worst case**: 50 NPM × 5 paired docs × 4 reads = **1000 ReadFile calls** for a single Student page load.

### 2.2 No Cache Layer in evalstore — **HIGH IMPACT**

**Location**: `internal/infrastructure/evalstore/evalstore.go:130-147`

```go
func (s *Store) LoadGT(npm, docType, filename string) ([]byte, error) {
    path := filepath.Join(s.evalDir, npm, docType, filename)
    data, err := os.ReadFile(path)  // Always hits disk
    ...
}
```

Every `LoadGT`/`LoadExtract` call hits the disk directly. No in-memory cache, no request-scoped memoization.

### 2.3 Chart.js Loaded Globally — **MEDIUM IMPACT**

**Location**: `internal/interfaces/http/evalhtml/templates/base.html` scripts partial

Chart.js (~77KB gzipped) is loaded via `{{define "scripts"}}` partial on **all 9 pages**, but only `index.html` uses it (for doughnut + bar charts). The other 8 pages (pipeline, detail, krs, khs, student, edit_krs, edit_khs, login, 404) don't need Chart.js.

### 2.4 Cache-Control Too Short — **MEDIUM IMPACT**

**Location**: `internal/interfaces/http/router/router.go:127`

```go
c.Set("Cache-Control", "public, max-age=300")  // 5 minutes
```

Static assets are embedded in the binary via `embed.FS` — they **cannot change without a redeploy**. `max-age=300` (5 minutes) forces unnecessary re-validation/re-download.

### 2.5 Pipeline.html DOM Size — **MEDIUM IMPACT**

**Location**: `internal/interfaces/http/evalhtml/templates/pipeline.html`

11 phase cards in Formulas tab = ~560 DOM nodes rendered at initial load. Animation tab is hidden by default (not a bottleneck), but Formulas tab is fully rendered.

### 2.6 Index Name Lookup Full Unmarshal — **LOW IMPACT**

**Location**: `internal/application/service/eval_service.go:43-62`

Index page does full `KHSExtraction`/`KRSExtraction` JSON unmarshal for name lookup, but only needs `Mahasiswa.Nama`. The `fastNameLookup()` pattern (line 534) already exists for `StudentList()` but isn't reused in `Index()`.

---

## 3. Proposed Solutions

### Solution 1: Per-Request Scoped Cache in evalstore — **Priority #1**

**Impact**: ~50% reduction in disk I/O for Index() and Student()
**Effort**: Medium
**Stale Risk**: NONE (cache dies with request)
**CSR**: NOT REQUIRED

#### Design
Wrap `Store` with a request-scoped caching layer that memoizes `LoadGT`/`LoadExtract` results within a single HTTP request.

```go
// New type: request-scoped cache wrapper
type cachedStore struct {
    inner  port.EvalStore
    cache  map[string][]byte  // keyed by "npm|docType|filename"
}

func (c *cachedStore) LoadGT(npm, docType, filename string) ([]byte, error) {
    key := npm + "|" + docType + "|" + filename
    if data, ok := c.cache[key]; ok {
        return data, nil
    }
    data, err := c.inner.LoadGT(npm, docType, filename)
    if err == nil {
        c.cache[key] = data
    }
    return data, err
}
```

#### Files to Modify
- `internal/infrastructure/evalstore/evalstore.go` — Add `cachedStore` wrapper
- `internal/application/service/eval_service.go` — Inject cache scope per request

#### Trade-offs
- ✅ Eliminates redundant reads within a single request
- ✅ Zero stale risk — cache lives only for the duration of one request
- ✅ No interface changes to `port.EvalStore`
- ⚠️ Adds ~100 bytes of memory per request (map overhead)

---

### Solution 2: Merge evaluateDoc + buildCompareRows — **Priority #2**

**Impact**: ~50% reduction in disk I/O for Student() paired docs
**Effort**: Medium
**Stale Risk**: NONE
**CSR**: NOT REQUIRED

> **Relationship to Solution 1**: Solutions 1 and 2 are **alternatives for Student()** (both eliminate the evaluateDoc/buildCompareRows redundancy), but **complementary for Index()** — Solution 1 also caches GT reads between the name lookup loop and evaluateDoc() in Index(), while Solution 2 does not. Recommended: implement Solution 1 first; Solution 2 is optional cleanup.

#### Design
Refactor `evaluateDoc()` to return the parsed structs alongside metrics, eliminating the need for `buildCompareRows()` to re-read the same files.

```go
type evaluateResult struct {
    Metrics      entity.Metrics
    GTData       []byte
    ExtractData  []byte
}

func (s *EvalService) evaluateDocFull(npm, docType, filename string) (evaluateResult, error) {
    gtData, err := s.store.LoadGT(npm, docType, filename)
    if err != nil { return evaluateResult{}, err }
    extractData, err := s.store.LoadExtract(npm, docType, filename)
    if err != nil { return evaluateResult{}, err }
    
    // Parse once, return both metrics and raw data
    var metrics entity.Metrics
    if docType == "khs" {
        var gt, extract entity.KHSExtraction
        json.Unmarshal(gtData, &gt)
        json.Unmarshal(extractData, &extract)
        metrics = s.computeKHSMetrics(&gt, &extract)
    } else {
        var gt, extract entity.KRSExtraction
        json.Unmarshal(gtData, &gt)
        json.Unmarshal(extractData, &extract)
        metrics = s.computeKRSMetrics(&gt, &extract)
    }
    return evaluateResult{Metrics: metrics, GTData: gtData, ExtractData: extractData}, nil
}
```

#### Files to Modify
- `internal/application/service/eval_service.go` — Refactor `evaluateDoc()` + `buildCompareRows()`

#### Trade-offs
- ✅ Eliminates 4× ReadFile → 2× per paired doc
- ✅ Zero stale risk — data still read from disk once per request
- ⚠️ Requires interface change to internal service methods
- ⚠️ Slightly more memory per doc (retains []byte for both GT and Extract)

---

### Solution 3: Chart.js Conditional Loading — **Priority #3**

**Impact**: ~77KB savings per page (8 pages freed from Chart.js)
**Effort**: Low
**Stale Risk**: NONE
**CSR**: NOT REQUIRED

#### Design
Move Chart.js CDN script from `base.html` scripts partial to `index.html` only, with `defer` attribute.

```html
<!-- base.html: Remove chart.umd.min.js from scripts partial -->

<!-- index.html: Add at bottom of body -->
<script src="https://cdnjs.cloudflare.com/ajax/libs/Chart.js/4.4.1/chart.umd.min.js" defer></script>
<script src="/eval/static/js/index.js" type="module"></script>
```

#### Files to Modify
- `internal/interfaces/http/evalhtml/templates/base.html` — Remove Chart.js from scripts partial
- `internal/interfaces/http/evalhtml/templates/index.html` — Add Chart.js with defer

#### Trade-offs
- ✅ 8 pages no longer load 77KB unnecessary JS
- ✅ Render-blocking → non-blocking (via defer)
- ✅ Zero stale risk — same CDN URL
- ⚠️ Requires `index.js` to handle deferred Chart.js load (already uses DOMContentLoaded)

---

### Solution 4: Cache-Control 1 Year for Embedded Assets — **Priority #4**

**Impact**: Eliminates static asset re-downloads after first load
**Effort**: Low
**Stale Risk**: NONE (assets embedded in binary)
**CSR**: NOT REQUIRED

#### Design
Change `max-age=300` to `max-age=31536000` (1 year) with `immutable` directive for assets served from `embed.FS`.

```go
// router.go:127
c.Set("Cache-Control", "public, max-age=31536000, immutable")
```

#### Files to Modify
- `internal/interfaces/http/router/router.go` — Change cache header

#### Trade-offs
- ✅ Static assets cached for 1 year after first load
- ✅ Zero stale risk — assets compiled into binary, change only on redeploy
- ✅ Eliminates all static asset network requests after first load
- ⚠️ If assets change, need cache-busting (but embed.FS = compiled in, so redeploy = new binary)

---

### Solution 5: Native `<details>` Collapse for Pipeline.html — **Priority #5**

**Impact**: ~560 nodes saved from initial paint
**Effort**: Medium
**Stale Risk**: NONE
**CSR**: NOT REQUIRED

#### Design
Wrap each phase card (except Fase 1) in native HTML `<details>` element. Browser handles collapse/expand without JS.

```html
<details open>
  <summary><h4><i class="bi bi-file-earmark-pdf"></i> Fase 1: PDF Reading</h4></summary>
  <!-- phase content -->
</details>
<details>
  <summary><h4><i class="bi bi-arrow-down"></i> Fase 2: Coordinate Transform</h4></summary>
  <!-- phase content -->
</details>
<!-- ... Fase 3-11 ... -->
```

#### Files to Modify
- `internal/interfaces/http/evalhtml/templates/pipeline.html` — Wrap 10 phase cards in `<details>`

#### Trade-offs
- ✅ ~560 nodes removed from initial layout/paint
- ✅ Pure HTML, zero JS, zero CSR
- ✅ Browser lazy-renders expanded content
- ⚠️ Slightly different UX (collapsed by default)

---

### Solution 6: Index Name Lookup Partial Decode — **Priority #6**

**Impact**: CPU savings (minor)
**Effort**: Low
**Stale Risk**: NONE
**CSR**: NOT REQUIRED

#### Design
Reuse `fastNameLookup()` pattern (already exists at line 534) for Index page name lookup instead of full struct unmarshal.

#### Files to Modify
- `internal/application/service/eval_service.go` — Replace full unmarshal with partial decode

#### Trade-offs
- ✅ Reduces CPU time for name extraction
- ✅ Zero stale risk
- ⚠️ Minor impact (CPU, not I/O)

---

## 4. Implementation Phases

### Phase 1 — Quick Wins (Impact: 30-40%, Effort: Low)
| # | Solution | Files | Est. Latency Reduction |
|---|----------|-------|----------------------|
| 3 | Chart.js conditional loading | base.html, index.html | ~50ms (8 pages) |
| 4 | Cache-Control 31536000 | router.go:127 | ~20ms (repeat loads) |
| — | defer CDN scripts | base.html | ~10ms |

### Phase 2 — I/O Reduction (Impact: 40-50%, Effort: Medium)
| # | Solution | Files | Est. Latency Reduction |
|---|----------|-------|----------------------|
| 1 | Per-request scoped cache | evalstore.go, eval_service.go | ~50ms (Index) |
| 2 | Merge evaluateDoc + buildCompareRows | eval_service.go | ~30ms (Student) |
| 6 | Index name partial decode | eval_service.go | ~5ms |

### Phase 3 — Template Optimization (Impact: 10-20%, Effort: Medium)
| # | Solution | Files | Est. Latency Reduction |
|---|----------|-------|----------------------|
| 5 | `<details>` collapse | pipeline.html | ~30-40ms (pipeline) |

### Expected Outcome
| Phase | Cumulative Latency Reduction | Expected Load Time |
|-------|----------------------------|-------------------|
| Current | 0% | >200ms |
| Phase 1 | 30-40% | ~120-140ms |
| Phase 1+2 | 60-80% | ~40-80ms |
| Phase 1+2+3 | 70-90% | ~20-60ms |

**Target <100ms is achievable with Phase 1+2.**

---

## 5. Constraint Compliance Matrix

| Solution | SSR | Fresh Data | No Stale Cache | No CSR |
|----------|-----|------------|----------------|--------|
| 1. Per-request cache | ✅ | ✅ | ✅ (dies with request) | ✅ |
| 2. Merge evaluateDoc | ✅ | ✅ | ✅ (no cache) | ✅ |
| 3. Chart.js conditional | ✅ | ✅ | ✅ (same CDN) | ✅ |
| 4. Cache-Control 1yr | ✅ | ✅ | ✅ (embed.FS) | ✅ |
| 5. `<details>` collapse | ✅ | ✅ | ✅ (no cache) | ✅ |
| 6. Partial decode | ✅ | ✅ | ✅ (no cache) | ✅ |

---

## 6. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Per-request cache memory leak | Low | Low | Cache is scoped to request, GC'd after response |
| `<details>` UX regression | Low | Low | Fase 1 remains open; user can expand others |
| Chart.js defer breaks charts | Low | Medium | index.js already uses DOMContentLoaded |
| Cache-Control immutable issues | None | None | embed.FS assets can't change without redeploy |

---

## 7. Testing Strategy

### Unit Tests
- `TestCachedStoreLoadGT` — Verify cache hit returns same data as underlying store
- `TestCachedStoreLoadExtract` — Verify cache hit returns same data
- `TestEvaluateDocFullReturnsData` — Verify merged method returns both metrics and raw data
- `TestIndexNameLookupPartialDecode` — Verify partial decode returns correct name

### Integration Tests
- `TestEvalIndexPageLoadTime` — Benchmark Index page <100ms
- `TestEvalStudentPageLoadTime` — Benchmark Student page <100ms
- `TestStaticAssetsCacheHeader` — Verify Cache-Control: max-age=31536000

### Manual Verification
- Load `/eval` and verify Chart.js loads only on index page
- Load `/eval/pipeline` and verify phase cards are collapsed by default
- Load `/eval/:npm` and verify compare rows render correctly

---

## 8. Open Questions

1. **Cache key format**: Use `npm|docType|filename` or struct key? → Recommend string key for simplicity.
2. **Cache scope**: Per-request via handler injection or per-service-method? → Recommend per-request via handler.
3. **Pipeline.html UX**: Should Fase 1 be open or closed by default? → Recommend open (most important phase).

---

## 9. References

- `internal/application/service/eval_service.go` — Eval service with redundant I/O
- `internal/infrastructure/evalstore/evalstore.go` — Store implementation (no cache)
- `internal/interfaces/http/router/router.go` — Static handler with max-age=300
- `internal/interfaces/http/evalhtml/templates/base.html` — Base template with global Chart.js
- `internal/interfaces/http/evalhtml/templates/pipeline.html` — 11 phase cards (~670 nodes)
- `internal/interfaces/http/evalhtml/templates/index.html` — Dashboard page (~320 nodes)
