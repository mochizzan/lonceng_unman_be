package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
)

// ExtractionService defines the interface for PDF extraction operations.
type ExtractionService interface {
	ExtractKRS(npm string, password string) (*entity.ExtractionResult, error)
	ExtractKHS(npm string, password string, tahunAjaran string, semester string) (*entity.ExtractionResult, error)
	GetKRSExtraction(npm string) ([]byte, error)
	GetKHSExtraction(npm string, tahunAjaran string, semester string) ([]byte, error)
}

// extractionService implements ExtractionService.
type extractionService struct {
	downloadDir string
	extractDir  string
	parser      port.PDFParser
	cache       port.ExtractionCache
	sessions    port.SessionManager
}

// NewExtractionService creates a new extraction service.
func NewExtractionService(downloadDir string, extractDir string, parser port.PDFParser, cache port.ExtractionCache, sessions port.SessionManager) ExtractionService {
	return &extractionService{
		downloadDir: downloadDir,
		extractDir:  extractDir,
		parser:      parser,
		cache:       cache,
		sessions:    sessions,
	}
}

// verifySession ensures LMS credentials are valid before extraction.
// Infrastructure failures (browser/CDP/DNS/timeout) are classified as 503 so
// the caller can retry, instead of misleading the user with "wrong password".
func (s *extractionService) verifySession(npm, password string) error {
	session, err := s.sessions.GetOrCreate(npm, password)
	if err != nil {
		if apperror.IsInfrastructureError(err) {
			return apperror.ClassifyDocumentError(err, "LMS extraction failed")
		}
		if apperror.IsCredentialError(err) {
			return apperror.Unauthorized("Username atau password salah")
		}
		// Conservative fallback: treat unknown as infrastructure (503) to avoid
		// masking cold-start issues as credential failures. Only explicit
		// credential keywords above map to 401.
		// Check for generic timeout/deadline that IsInfrastructureError may miss
		// when the error is deeply wrapped without the exact keyword casing.
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "context deadline exceeded") || strings.Contains(low, "dns check") {
			return apperror.ClassifyDocumentError(err, "LMS extraction failed")
		}
		return apperror.Unauthorized("Username atau password salah")
	}
	session.Close() // Release session reference immediately
	return nil
}

// saveParsed updates metadata, marshals to JSON, caches the result, and builds ExtractionResult.
func (s *extractionService) saveParsed(
	npm, docType, cacheFile, message, pdfPath string,
	extraction interface{},
	meta *entity.ExtractionMetadata,
) (*entity.ExtractionResult, error) {
	if fileInfo, err := os.Stat(pdfPath); err == nil {
		meta.FileSize = int(fileInfo.Size())
	}

	meta.DocumentCategory = entity.CategoryExtracted

	data, err := s.parser.MarshalToJSON(extraction)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	if err := s.cache.Set(npm, docType, cacheFile, data); err != nil {
		return nil, fmt.Errorf("save cache: %w", err)
	}

	return &entity.ExtractionResult{
		Success:   true,
		Message:   message,
		NPM:       npm,
		FilePath:  filepath.Join(s.extractDir, npm, docType, cacheFile),
		Timestamp: time.Now(),
	}, nil
}

// ExtractKRS extracts KRS data from the downloaded PDF.
// Always re-extracts and overwrites existing cache.
func (s *extractionService) ExtractKRS(npm string, password string) (*entity.ExtractionResult, error) {
	pdfPath, err := s.findKRSFile(npm)
	if err == nil {
		extraction, parseErr := s.parser.ParseKRS(pdfPath, npm)
		if parseErr != nil {
			return nil, fmt.Errorf("parse krs: %w", parseErr)
		}
		cacheFile := entity.KRSFilePrefix + s.getKRSSemester(pdfPath) + entity.ExtJSON
		return s.saveParsed(npm, entity.DocTypeKRS.String(), cacheFile, "KRS extracted successfully", pdfPath, extraction, &extraction.Metadata)
	}
	// PDF missing — verify session to classify 401 vs 503.
	if vErr := s.verifySession(npm, password); vErr != nil {
		return nil, vErr
	}
	return nil, fmt.Errorf("find krs file: %w", err)
}

// ExtractKHS extracts KHS data from the downloaded PDF.
// Always re-extracts and overwrites existing cache.
func (s *extractionService) ExtractKHS(npm string, password string, tahunAjaran string, semester string) (*entity.ExtractionResult, error) {
	semester = strings.ToUpper(semester)

	pdfPath, err := s.findKHSFile(npm, tahunAjaran, semester)
	if err == nil {
		extraction, parseErr := s.parser.ParseKHS(pdfPath, npm, tahunAjaran, semester)
		if parseErr != nil {
			return nil, fmt.Errorf("parse khs: %w", parseErr)
		}
		cacheFile := s.khsCacheFilename(tahunAjaran, semester)
		return s.saveParsed(npm, entity.DocTypeKHS.String(), cacheFile, "KHS extracted successfully", pdfPath, extraction, &extraction.Metadata)
	}
	// PDF missing — verify session to classify 401 vs 503.
	if vErr := s.verifySession(npm, password); vErr != nil {
		return nil, vErr
	}
	return nil, fmt.Errorf("find khs file: %w", err)
}

// hasAlumniKRS reports whether an ALUMNI KRS PDF exists for the NPM.
// It scans downloads/{npm}/krs/ for any file whose name contains "ALUMNI"
// (case-insensitive), e.g. semester_ALUMNI_2026.pdf. This is a pure
// filesystem check — no browser/Rod interaction — so it matches the spec
// "pengecekan saat sudah terdownload saja, di endpoint GET".
func (s *extractionService) hasAlumniKRS(npm string) bool {
	krsDownloadDir := filepath.Join(s.downloadDir, npm, entity.DocTypeKRS.String())
	entries, err := os.ReadDir(krsDownloadDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(e.Name()), entity.ExtPDF) {
			continue
		}
		upper := strings.ToUpper(e.Name())
		if strings.Contains(upper, "ALUMNI") && !strings.HasSuffix(upper, ".META.JSON") {
			return true
		}
	}
	return false
}

// GetKRSExtraction retrieves cached KRS extraction.
// If an ALUMNI KRS PDF exists in downloads/{npm}/krs/ (e.g.
// semester_ALUMNI_2026.pdf), the student is already graduated and KRS is
// no longer applicable. In that case this returns ErrAlumniKRS so the
// handler can respond with 409 Conflict ("gunakan KHS"). The check is
// filesystem-only — no LMS/browser call.
func (s *extractionService) GetKRSExtraction(npm string) ([]byte, error) {
	// ALUMNI gate — check downloads before serving extracted JSON.
	if s.hasAlumniKRS(npm) {
		return nil, fmt.Errorf("krs unavailable for npm %s: %w", npm, apperror.ErrAlumniKRS)
	}

	// Find the latest KRS JSON file
	krsDir := filepath.Join(s.extractDir, npm, entity.DocTypeKRS.String())
	entries, err := os.ReadDir(krsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no krs extraction for npm %s: %w", npm, apperror.ErrExtractionNotFound)
		}
		return nil, fmt.Errorf("read krs dir: %w", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no krs extraction for npm %s: %w", npm, apperror.ErrExtractionNotFound)
	}

	// Numeric sort, not lexicographic (fixes P28) — symmetric with findKRSFile.
	var bestName string
	bestNum := -1
	for i, e := range entries {
		num := extractSemesterNum(e.Name())
		if i == 0 || num > bestNum {
			bestName = e.Name()
			bestNum = num
		} else if num == bestNum && num == 0 && e.Name() > bestName {
			// Fallback lexicographic for non-semester files.
			bestName = e.Name()
		}
	}

	return os.ReadFile(filepath.Join(krsDir, bestName))
}

// GetKHSExtraction retrieves cached KHS extraction.
func (s *extractionService) GetKHSExtraction(npm string, tahunAjaran string, semester string) ([]byte, error) {
	cacheFile := s.khsCacheFilename(tahunAjaran, semester)
	data, err := s.cache.Get(npm, entity.DocTypeKHS.String(), cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no khs extraction for npm %s: %w", npm, apperror.ErrExtractionNotFound)
		}
		return nil, err
	}
	return data, nil
}

// findKRSFile finds the latest KRS PDF file for a given NPM.
// Files are named semester_<N>.pdf; this sorts by numeric N and returns the highest.
func (s *extractionService) findKRSFile(npm string) (string, error) {
	krsDir := filepath.Join(s.downloadDir, npm, entity.DocTypeKRS.String())
	entries, err := os.ReadDir(krsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no krs directory for npm %s: %w", npm, apperror.ErrPDFNotFound)
		}
		return "", fmt.Errorf("read krs dir: %w", err)
	}

	type pdfEntry struct {
		path string
		num  int
	}
	var pdfs []pdfEntry
	for _, e := range entries {
		if filepath.Ext(e.Name()) == entity.ExtPDF {
			num := extractSemesterNum(e.Name())
			pdfs = append(pdfs, pdfEntry{path: filepath.Join(krsDir, e.Name()), num: num})
		}
	}

	if len(pdfs) == 0 {
		return "", fmt.Errorf("no krs pdf for npm %s: %w", npm, apperror.ErrPDFNotFound)
	}

	sort.Slice(pdfs, func(i, j int) bool { return pdfs[i].num > pdfs[j].num })
	return pdfs[0].path, nil
}

// extractSemesterNum extracts the numeric semester from a filename like "semester_9.pdf".
func extractSemesterNum(filename string) int {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	parts := strings.SplitN(name, "_", 2)
	if len(parts) == 2 {
		if n, err := strconv.Atoi(parts[1]); err == nil {
			return n
		}
	}
	return 0
}

// findKHSFile finds the KHS PDF file for a given NPM, tahun ajaran, and semester.
func (s *extractionService) findKHSFile(npm string, tahunAjaran string, semester string) (string, error) {
	khsDir := filepath.Join(s.downloadDir, npm, entity.DocTypeKHS.String())
	filename := entity.KHSFilename(tahunAjaran, semester)
	path := filepath.Join(khsDir, filename)

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("khs pdf not found: %s: %w", path, apperror.ErrPDFNotFound)
		}
		return "", fmt.Errorf("stat khs pdf: %w", err)
	}

	return path, nil
}

// getKRSSemester extracts the semester number string from a KRS PDF path.
// Handles: "semester_8.pdf" → "8", "semester_12.pdf" → "12"
func (s *extractionService) getKRSSemester(pdfPath string) string {
	return strconv.Itoa(extractSemesterNum(filepath.Base(pdfPath)))
}

// khsCacheFilename generates the KHS cache filename.
func (s *extractionService) khsCacheFilename(tahunAjaran string, semester string) string {
	base := entity.KHSFilename(tahunAjaran, semester)
	return strings.TrimSuffix(base, entity.ExtPDF) + entity.ExtJSON
}
