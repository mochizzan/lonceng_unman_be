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

// ExtractKRS extracts KRS data from the downloaded PDF.
// Always re-extracts and overwrites existing cache.
func (s *extractionService) ExtractKRS(npm string, password string) (*entity.ExtractionResult, error) {
	// Verify LMS credentials before extraction
	if _, err := s.sessions.GetOrCreate(npm, password); err != nil {
		return nil, apperror.Unauthorized("LMS login failed: " + err.Error())
	}

	// Find the KRS PDF file
	pdfPath, err := s.findKRSFile(npm)
	if err != nil {
		return nil, fmt.Errorf("find krs file: %w", err)
	}

	// Parse PDF
	extraction, err := s.parser.ParseKRS(pdfPath, npm)
	if err != nil {
		return nil, fmt.Errorf("parse krs: %w", err)
	}

	// Update metadata
	fileInfo, _ := os.Stat(pdfPath)
	if fileInfo != nil {
		extraction.Metadata.FileSize = int(fileInfo.Size())
	}

	// Marshal to JSON
	data, err := s.parser.MarshalToJSON(extraction)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	// Save to cache (overwrite if exists)
	cacheFile := entity.KRSFilePrefix + s.getKRSSemester(pdfPath) + entity.ExtJSON
	if err := s.cache.Set(npm, entity.DocTypeKRS, cacheFile, data); err != nil {
		return nil, fmt.Errorf("save cache: %w", err)
	}

	return &entity.ExtractionResult{
		Success:   true,
		Message:   "KRS extracted successfully",
		NPM:       npm,
		FilePath:  filepath.Join(s.extractDir, npm, entity.DocTypeKRS, cacheFile),
		Timestamp: time.Now(),
	}, nil
}

// ExtractKHS extracts KHS data from the downloaded PDF.
// Always re-extracts and overwrites existing cache.
func (s *extractionService) ExtractKHS(npm string, password string, tahunAjaran string, semester string) (*entity.ExtractionResult, error) {
	// Verify LMS credentials before extraction
	if _, err := s.sessions.GetOrCreate(npm, password); err != nil {
		return nil, apperror.Unauthorized("LMS login failed: " + err.Error())
	}

	semester = strings.ToUpper(semester)

	// Find the KHS PDF file
	pdfPath, err := s.findKHSFile(npm, tahunAjaran, semester)
	if err != nil {
		return nil, fmt.Errorf("find khs file: %w", err)
	}

	// Parse PDF
	extraction, err := s.parser.ParseKHS(pdfPath, npm, tahunAjaran, semester)
	if err != nil {
		return nil, fmt.Errorf("parse khs: %w", err)
	}

	// Update metadata
	fileInfo, _ := os.Stat(pdfPath)
	if fileInfo != nil {
		extraction.Metadata.FileSize = int(fileInfo.Size())
	}

	// Marshal to JSON
	data, err := s.parser.MarshalToJSON(extraction)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	// Save to cache (overwrite if exists)
	cacheFile := s.khsCacheFilename(tahunAjaran, semester)
	if err := s.cache.Set(npm, entity.DocTypeKHS, cacheFile, data); err != nil {
		return nil, fmt.Errorf("save cache: %w", err)
	}

	return &entity.ExtractionResult{
		Success:   true,
		Message:   "KHS extracted successfully",
		NPM:       npm,
		FilePath:  filepath.Join(s.extractDir, npm, entity.DocTypeKHS, cacheFile),
		Timestamp: time.Now(),
	}, nil
}

// GetKRSExtraction retrieves cached KRS extraction.
func (s *extractionService) GetKRSExtraction(npm string) ([]byte, error) {
	// Find the latest KRS JSON file
	krsDir := filepath.Join(s.extractDir, npm, entity.DocTypeKRS)
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

	// Get the latest file
	latest := entries[0]
	for _, e := range entries {
		if e.Name() > latest.Name() {
			latest = e
		}
	}

	return os.ReadFile(filepath.Join(krsDir, latest.Name()))
}

// GetKHSExtraction retrieves cached KHS extraction.
func (s *extractionService) GetKHSExtraction(npm string, tahunAjaran string, semester string) ([]byte, error) {
	cacheFile := s.khsCacheFilename(tahunAjaran, semester)
	data, err := s.cache.Get(npm, entity.DocTypeKHS, cacheFile)
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
	krsDir := filepath.Join(s.downloadDir, npm, entity.DocTypeKRS)
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
	khsDir := filepath.Join(s.downloadDir, npm, entity.DocTypeKHS)
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
