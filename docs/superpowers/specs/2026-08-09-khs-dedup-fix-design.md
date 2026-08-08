# KHS Deduplication Fix Design

## Problem Statement

The KHS (Kartu Hasil Studi) PDF parser fails to extract courses with duplicate course codes (`Kode`). When two courses share the same code (e.g., `SI40306`), only the first occurrence is extracted; the second is silently dropped.

### Observed Behavior

- **Input:** KHS PDF with 2 courses, both coded `SI40306`
- **Expected Output:** `mata_kuliah` array contains 2 entries
- **Actual Output:** `mata_kuliah` array contains only 1 entry
- **Side Effect:** `total_sks` (12) is correct because it's parsed independently from the "Total" row, but the array length doesn't match

### Root Cause

In `internal/infrastructure/extractor/khs_parser.go`, the `parseKHSMataKuliah` function deduplicates courses by `Kode`:

```go
seenKodes := make(map[string]bool)
// ...
if seenKodes[kode] {
    continue
}
seenKodes[kode] = true
```

This assumes each course code is unique, which is incorrect. KHS documents can have the same course code appearing multiple times (e.g., repeated courses, multiple sections).

## Design Decision

**Approach:** Minimal fix — change deduplication key from `Kode` to `No` (row number).

**Rationale:**
- `No` column is guaranteed unique per row in KHS PDFs
- Minimal code change (2-3 lines)
- Follows existing patterns in the codebase
- User confirmed preference for `No`-based deduplication

**Trade-offs Accepted:**
- No additional validation for `No` column (e.g., positive integer check)
- Trust PDF source for `No` column integrity

## Implementation Details

### File to Modify

`internal/infrastructure/extractor/khs_parser.go`

### Function

`parseKHSMataKuliah`

### Changes

#### 1. Change Map Initialization (line 103)

```go
// BEFORE
seenKodes := make(map[string]bool)

// AFTER
seenNos := make(map[int]bool)
```

#### 2. Move `no` Computation Before Dedup Check (line 158 → before line 153)

```go
// BEFORE (line 158, AFTER dedup check)
no := parseIntSafe(cols["No"])

// AFTER (move BEFORE dedup check, around line 152)
no := parseIntSafe(cols["No"])
```

#### 3. Change Deduplication Logic (lines 153-156)

```go
// BEFORE
if seenKodes[kode] {
    continue
}
seenKodes[kode] = true

// AFTER
if seenNos[no] {
    continue
}
seenNos[no] = true
```

### Variable Dependencies

**Important:** The `no` variable must be computed BEFORE the dedup check. Currently it's computed AFTER (line 158). The fix requires moving `no := parseIntSafe(cols["No"])` to before the dedup block.

No additional parsing or validation is required — `parseIntSafe` already handles the conversion.

## Expected Outcome

### Before Fix

```json
{
  "mata_kuliah": [
    {"no": 1, "kode": "SI40306", "nama": "Tugas Akhir/Skripsi", "sks": 6, "nilai": "", "mutu": 0}
  ],
  "rekapitulasi": {
    "total_sks": 12,
    "total_mutu": 0,
    "ipk": 0
  }
}
```

- Array length: 1
- `total_sks`: 12 (correct, from PDF "Total" row)
- **Mismatch:** Array has 1 item but `total_sks` implies 2 items (6+6)

### After Fix

```json
{
  "mata_kuliah": [
    {"no": 1, "kode": "SI40306", "nama": "Tugas Akhir/Skripsi", "sks": 6, "nilai": "", "mutu": 0},
    {"no": 2, "kode": "SI40306", "nama": "Tugas Akhir/Skripsi", "sks": 6, "nilai": "", "mutu": 0}
  ],
  "rekapitulasi": {
    "total_sks": 12,
    "total_mutu": 0,
    "ipk": 0
  }
}
```

- Array length: 2
- `total_sks`: 12 (unchanged)
- **Match:** Array has 2 items × 6 SKS = 12 SKS ✓

## Testing Strategy

### Unit Test Cases

1. **Happy Path:** KHS with 2 courses, same code, different `No`
   - Input: 2 rows with `No=1, Kode=SI40306` and `No=2, Kode=SI40306`
   - Expected: `mata_kuliah` array length = 2

2. **Edge Case:** KHS with duplicate `No` values
   - Input: 2 rows with `No=1, Kode=SI40306` and `No=1, Kode=SI40307`
   - Expected: Only first row extracted (second skipped)

3. **Regression:** KHS with unique codes (existing behavior preserved)
   - Input: 2 rows with `No=1, Kode=SI40306` and `No=2, Kode=SI40307`
   - Expected: Both rows extracted (no change from current behavior)

### Integration Test

Re-extract the problematic PDF (`extracted/2211700006/khs/2025_2026_GENAP.json`) and verify:
- `mata_kuliah` array contains 2 entries
- Both entries have `kode: "SI40306"`
- `total_sks` remains 12

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Regression in unique-code PDFs | Low | Medium | Existing tests cover this case |
| `No` column missing or malformed | Low | Low | `parseIntSafe` returns 0; dedup by 0 would skip all rows with `No=0` |
| Performance impact | Negligible | None | Map key change from `string` to `int` is faster |

## Out of Scope

- Adding validation for `No` column (e.g., positive integer check)
- Changing the JSON output structure
- Modifying `total_sks` calculation logic
- Handling PDFs with completely malformed row numbers

## References

- Source: `internal/infrastructure/extractor/khs_parser.go`
- Function: `parseKHSMataKuliah` (lines 59-226)
- Key Lines:
  - Line 103: `seenKodes` map initialization
  - Lines 153-156: Deduplication logic
  - Line 158: `no` variable computation (must move before dedup)
- Related: `extracted/2211700006/khs/2025_2026_GENAP.json`
