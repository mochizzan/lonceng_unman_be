# /eval Page Performance — Implementation Plan

**Date**: 2026-08-30
**Spec**: `docs/superpowers/specs/2026-08-30-eval-page-performance-optimization-design.md`
**Status**: Ready for Execution

---

## Overview

Reduce `/eval` Index page DOM load from **>200ms → <100ms** with 6 optimizations across 3 phases. All optimizations are SSR-compatible, zero stale-data risk, zero CSR.

---

## Phase 1 — Quick Wins (Target: 30-40% latency reduction)

### Task 1.1: Chart.js Conditional Loading

**File**: `internal/interfaces/http/evalhtml/templates/base.html`
**File**: `internal/interfaces/http/evalhtml/templates/index.html`

**Change**:
1. Remove `<script src="chart.umd.min.js">` from `{{define "scripts"}}` partial in base.html
2. Add Chart.js CDN script with `defer` attribute at bottom of `<body>` in index.html
3. Add `defer` to `bootstrap.bundle.min.js` in scripts partial

**Acceptance**:
- Chart.js loads ONLY on `/eval` (index) page — NOT on pipeline, detail, krs, khs, student, edit_krs, edit_khs, login, 404
- Charts still render correctly on index page (Chart.js available when `DOMContentLoaded` fires)
- `index.js` still works (uses `DOMContentLoaded`, not Chart.js global)

**Test**:
- `curl -s http://localhost:3000/eval | grep -c chart.umd.min.js` → 1
- `curl -s http://localhost:3000/eval/pipeline | grep -c chart.umd.min.js` → 0
- `curl -s http://localhost:3000/eval/detail?npm=12345678 | grep -c chart.umd.min.js` → 0

---

### Task 1.2: Cache-Control 1 Year for Embedded Assets

**File**: `internal/interfaces/http/router/router.go:127`

**Change**:
```go
// Before
c.Set("Cache-Control", "public, max-age=300")

// After
c.Set("Cache-Control", "public, max-age=31536000, immutable")
```

**Acceptance**:
- Static assets served with `Cache-Control: public, max-age=31536000, immutable`
- Browser caches assets for 1 year after first load
- Zero stale risk — assets embedded in binary, change only on redeploy

**Test**:
- `curl -sI http://localhost:3000/eval/static/css/main.css | grep cache-control` → `public, max-age=31536000, immutable`

---

## Phase 2 — I/O Reduction (Target: 40-50% latency reduction)

### Task 2.1: Per-Request Scoped Cache in evalstore

**File**: `internal/infrastructure/evalstore/evalstore.go`
**File**: `internal/application/service/eval_service.go`

**Change**:

1. Add `cachedStore` wrapper struct in evalstore.go:

```go
// cachedStore wraps an EvalStore with per-request memoization.
type cachedStore struct {
    inner port.EvalStore
    cache map[string][]byte
}

func newCachedStore(inner port.EvalStore) *cachedStore {
    return &cachedStore{
        inner: inner,
        cache: make(map[string][]byte),
    }
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

func (c *cachedStore) LoadExtract(npm, docType, filename string) ([]byte, error) {
    key := npm + "|" + docType + "|" + filename
    if data, ok := c.cache[key]; ok {
        return data, nil
    }
    data, err := c.inner.LoadExtract(npm, docType, filename)
    if err == nil {
        c.cache[key] = data
    }
    return data, err
}

// Passthrough methods (no caching needed)
func (c *cachedStore) ListNPMs() ([]string, error)   { return c.inner.ListNPMs() }
func (c *cachedStore) ListDocs(npm string) ([]entity.NPMDoc, error) { return c.inner.ListDocs(npm) }
func (c *cachedStore) WriteGT(...) error               { return c.inner.WriteGT(...) }
func (c *cachedStore) Exists(...) (bool, bool, error) { return c.inner.Exists(...) }
```

2. Inject cachedStore per request in eval_service.go — create new cachedStore at start of each `Index()` and `Student()` call:

```go
func (s *EvalService) Index() (entity.UnifiedEval, error) {
    cached := newCachedStore(s.store)
    return s.indexWithStore(cached)
}

func (s *EvalService) indexWithStore(store port.EvalStore) (entity.UnifiedEval, error) {
    npms, err := store.ListNPMs()
    // ... rest uses `store` instead of `s.store`
}
```

**Acceptance**:
- Each unique file is read from disk ONCE per request
- Subsequent reads for same file within same request return cached `[]byte`
- Cache is GC'd after request completes (no stale risk)
- `ListNPMs`, `ListDocs`, `WriteGT`, `Exists` are unaffected (passthrough)

**Test**:
- Create test that calls `LoadGT` twice with same params, verify `os.ReadFile` called once
- `TestCachedStoreLoadGT` — cache hit returns same data
- `TestCachedStoreLoadExtract` — cache hit returns same data
- `TestCachedStoreMiss` — different keys return different data

---

### Task 2.2: Index Name Lookup Partial Decode

**File**: `internal/application/service/eval_service.go:43-62`

**Change**:
Replace full `KHSExtraction`/`KRSExtraction` unmarshal with partial decode using `json.Decoder` + struct with only needed fields:

```go
type nameOnly struct {
    KHS struct {
        Mahasiswa struct {
            Nama string `json:"nama"`
        } `json:"mahasiswa"`
    } `json:"khs"`
    KRS struct {
        Mahasiswa struct {
            Nama string `json:"nama"`
        } `json:"mahasiswa"`
    } `json:"krs"`
}

// In Index() name lookup loop:
for _, doc := range docs {
    if doc.HasGT {
        gtData, err := store.LoadGT(npm, doc.DocType, doc.File)
        if err == nil {
            var n nameOnly
            if err := json.Unmarshal(gtData, &n); err == nil {
                if doc.DocType == "khs" {
                    name = n.KHS.Mahasiswa.Nama
                } else {
                    name = n.KRS.Mahasiswa.Nama
                }
                if name != "" { break }
            }
        }
    }
}
```

**Acceptance**:
- Name lookup uses partial struct (only `Mahasiswa.Nama`) instead of full extraction struct
- Zero stale risk — still reads from disk
- CPU time reduced (smaller struct to unmarshal)

**Test**:
- `TestIndexNameLookupPartialDecode` — verify partial decode returns correct name from sample JSON

---

## Phase 3 — Template Optimization (Target: 10-20% latency reduction)

### Task 3.1: Native `<details>` Collapse for Pipeline.html

**File**: `internal/interfaces/http/evalhtml/templates/pipeline.html`

**Change**:
Wrap each phase card (Fase 2-11) in `<details>` element. Fase 1 remains `<details open>`.

```html
{{/* Fase 1 — open by default */}}
<details open>
  <summary>
    <h4><i class="bi bi-file-earmark-pdf"></i> Fase 1: PDF Reading</h4>
  </summary>
  <div class="phase-content">
    <!-- existing content -->
  </div>
</details>

{{/* Fase 2-11 — collapsed by default */}}
{{range $index, $phase := .Phases}}
<details>
  <summary>
    <h4><i class="bi {{$phase.Icon}}"></i> Fase {{$phase.ID}}: {{$phase.Name}}</h4>
  </summary>
  <div class="phase-content">
    <!-- phase content -->
  </div>
</details>
{{end}}
```

**Acceptance**:
- Fase 1 is visible by default (open)
- Fase 2-11 are collapsed by default
- User can expand/collapse by clicking summary
- No JS required — pure HTML
- Animation tab unaffected (already hidden by default)

**Test**:
- `curl -s http://localhost:3000/eval/pipeline | grep -c '<details'` → 11
- `curl -s http://localhost:3000/eval/pipeline | grep -c '<details open>'` → 1

---

## Execution Order

```
Phase 1 (Quick Wins)
├── Task 1.1: Chart.js conditional loading
└── Task 1.2: Cache-Control 31536000

Phase 2 (I/O Reduction)
├── Task 2.1: Per-request scoped cache
└── Task 2.2: Index name lookup partial decode

Phase 3 (Template Optimization)
└── Task 3.1: <details> collapse pipeline.html
```

**Total estimated effort**: 4-6 hours
**Expected outcome**: `/eval` Index page DOM load <100ms

---

## Verification Checklist

After all tasks complete:

- [ ] `/eval` Index page loads in <100ms (local benchmark)
- [ ] Chart.js loads only on index page
- [ ] Static assets have `max-age=31536000, immutable`
- [ ] No redundant disk reads in Student() (evaluateDoc + buildCompareRows share data)
- [ ] Per-request cache eliminates duplicate reads in Index()
- [ ] Pipeline.html phase cards collapsed by default
- [ ] All existing tests pass (`go test ./...`)
- [ ] No new test failures

---

## Risk Mitigation

| Risk | Mitigation |
|------|-----------|
| Chart.js defer breaks charts | index.js uses DOMContentLoaded, Chart.js loaded with defer — both fire after DOM parse |
| Cache memory overhead | Cache scoped to request, GC'd after response. ~100 bytes per request. |
| `<details>` UX regression | Fase 1 remains open; user can expand others with single click |
| Partial decode breaks name lookup | Fallback to full unmarshal if partial decode returns empty name |
