# KHS PDF File Endpoint Design

**Date:** 2026-09-01
**Status:** Approved Design
**Implements:** `docs/frontend/khs-pdf-download-endpoint.md`

## Overview

Add a new `POST /api/v1/lms/khs/file` endpoint that serves an already-downloaded KHS PDF file as binary data. The existing `POST /api/v1/lms/khs` endpoint only downloads and saves the PDF to disk, returning a JSON file path. This new endpoint lets clients retrieve the actual PDF bytes.

The design follows existing patterns: extend `DocumentHandler` and `LMSDocumentService` with a new `DownloadKHSFile` method, mirroring `DownloadKHS` but swapping the final step (save-to-disk → serve-from-disk).

## Architecture

```
Request → DocumentHandler.DownloadKHSFile
            → validate NPM/password/tahun_ajaran/semester
            → LMSDocumentService.DownloadKHSFile
                → GetOrCreate session (credential check)
                → Build file path: {DownloadDir}/{npm}/khs/{tahunAjaran}_{semester}.pdf
                → os.Stat (existence check)
            → Set response headers (Content-Type, Content-Disposition, Content-Length, Accept-Ranges)
            → c.SendFile (stream from disk)
```

## Components

### 1. Handler (`internal/interfaces/http/handler/document_handler.go`)

Add `DownloadKHSFile` method to `DocumentHandler`:

```go
func (h *DocumentHandler) DownloadKHSFile(c fiber.Ctx) error {
    var req entity.KHSDownloadRequest
    if err := c.Bind().Body(&req); err != nil {
        return apperror.BadRequest("invalid request body")
    }
    if err := validateNPM(req.NPM); err != nil {
        return err
    }
    if req.Password == "" {
        return apperror.BadRequest("password is required")
    }
    if req.TahunAjaran == "" {
        return apperror.BadRequest("tahun_ajaran is required")
    }
    if req.Semester == "" {
        return apperror.BadRequest("semester is required")
    }

    filePath, size, err := h.docService.DownloadKHSFile(req)
    if err != nil {
        return err
    }

    c.Set("Content-Type", "application/pdf")
    c.Set("Content-Disposition", fmt.Sprintf(
        `attachment; filename="KHS_%s_%s_%s.pdf"`,
        req.NPM, strings.ReplaceAll(req.TahunAjaran, "/", "_"), req.Semester))
    c.Set("Content-Length", strconv.FormatInt(size, 10))
    c.Set("Accept-Ranges", "bytes")

    return c.SendFile(filePath)
}
```

### 2. Service (`internal/application/service/lms_service.go`)

Add to `LMSDocumentService` interface:

```go
DownloadKHSFile(req entity.KHSDownloadRequest) (string, int64, error)
```

Implementation:

```go
func (s *lmsDocumentService) DownloadKHSFile(req entity.KHSDownloadRequest) (string, int64, error) {
    req.Semester = strings.ToUpper(req.Semester)
    if !entity.ValidSemester(req.Semester) {
        return "", 0, apperror.BadRequest("semester must be GANJIL or GENAP")
    }

    session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
    if err != nil {
        return "", 0, apperror.Unauthorized("invalid credentials")
    }
    defer session.Close()

    filePath := filepath.Join(s.cfg.App.DownloadDir, req.NPM, "khs",
        entity.KHSFilename(req.TahunAjaran, req.Semester))

    info, err := os.Stat(filePath)
    if err != nil {
        if os.IsNotExist(err) {
            return "", 0, apperror.NotFound("KHS PDF not found for the specified year and semester", err)
        }
        return "", 0, apperror.Internal("failed to stat PDF file", err)
    }

    slog.Info("serving KHS file", "npm", req.NPM, "path", filePath, "size", info.Size())
    return filePath, info.Size(), nil
}
```

### 3. Router (`internal/interfaces/http/router/router.go`)

Add one route alongside existing KHS routes:

```go
v1.Post("/lms/khs/file", docHandler.DownloadKHSFile)
```

## Error Handling

| Scenario | HTTP Status | Message | Source |
|----------|-------------|---------|--------|
| Invalid JSON body | 400 | "invalid request body" | Handler |
| Missing/invalid NPM | 400 | per `validateNPM` | Handler |
| Missing password | 400 | "password is required" | Handler |
| Missing tahun_ajaran | 400 | "tahun_ajaran is required" | Handler |
| Missing semester | 400 | "semester is required" | Handler |
| Invalid semester value | 400 | "semester must be GANJIL or GENAP" | Service |
| Invalid credentials | 401 | "invalid credentials" | Service (GetOrCreate failure) |
| PDF not on disk | 404 | "KHS PDF not found for the specified year and semester" | Service (os.IsNotExist) |
| File stat error | 500 | "failed to stat PDF file" | Service |

## Headers

```
Content-Type: application/pdf
Content-Disposition: attachment; filename="KHS_{npm}_{tahunAjaran}_{semester}.pdf"
Content-Length: {file_size_bytes}
Accept-Ranges: bytes
```

`c.SendFile()` streams the file directly from disk without loading into memory, satisfying the spec's performance requirement. The `Accept-Ranges` header enables resumable downloads (range requests), which Fiber's `SendFile` supports natively.

## Testing

### Handler Tests (`tests/` — new `khs_file_handler_test.go`)

Table-driven subtests:

1. Valid request with existing PDF → 200 with `Content-Type: application/pdf` and `Content-Disposition` header
2. Valid request with non-existing PDF → 404
3. Invalid credentials → 401
4. Missing fields → 400
5. Invalid semester value → 400

### Service Tests (`tests/` — new `khs_file_service_test.go`)

1. File exists → returns path and size
2. File missing → `apperror.NotFound` with `ErrPDFNotFound` wrapped
3. Invalid semester → `apperror.BadRequest`
4. Invalid credentials → `apperror.Unauthorized`
5. os.Stat error (non-NotExist) → `apperror.Internal`

## Migration

- No breaking changes — new endpoint is additive
- Existing `POST /api/v1/lms/khs` continues unchanged
- No changes to file storage structure or naming convention
- No changes to existing interfaces beyond adding one method

## Files Modified

| File | Change |
|------|--------|
| `internal/interfaces/http/handler/document_handler.go` | Add `DownloadKHSFile` method |
| `internal/application/service/lms_service.go` | Add `DownloadKHSFile` to interface + implementation |
| `internal/interfaces/http/router/router.go` | Add route registration |
| `internal/application/service/lms_service.go` (imports) | Add `os` import |
| `internal/interfaces/http/handler/document_handler.go` (imports) | Add `fmt`, `strconv` imports |
