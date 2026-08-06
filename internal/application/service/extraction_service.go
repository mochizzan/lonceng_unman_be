package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/infrastructure/extractor"
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
	cache       *extractor.CacheManager
}

// NewExtractionService creates a new extraction service.
func NewExtractionService(downloadDir string, extractDir string) ExtractionService {
	return &extractionService{
		downloadDir: downloadDir,
		extractDir:  extractDir,
		cache:       extractor.NewCacheManager(extractDir),
	}
}

// ExtractKRS extracts KRS data from the downloaded PDF.
func (s *extractionService) ExtractKRS(npm string, password string) (*entity.ExtractionResult, error) {
	// Find the KRS PDF file
	pdfPath, err := s.findKRSFile(npm)
	if err != nil {
		return nil, fmt.Errorf("find krs file: %w", err)
	}

	// Check cache first
	cacheFile := "semester_" + s.getKRSSemester(pdfPath) + ".json"
	if s.cache.Exists(npm, "krs", cacheFile) {
		return &entity.ExtractionResult{
			Success:   true,
			Message:   "KRS extraction loaded from cache",
			NPM:       npm,
			FilePath:  filepath.Join(s.extractDir, npm, "krs", cacheFile),
			Timestamp: time.Now(),
		}, nil
	}

	// Parse PDF
	extraction, err := extractor.ParseKRS(pdfPath, npm)
	if err != nil {
		return nil, fmt.Errorf("parse krs: %w", err)
	}

	// Update metadata
	fileInfo, _ := os.Stat(pdfPath)
	if fileInfo != nil {
		extraction.Metadata.FileSize = int(fileInfo.Size())
	}

	// Marshal to JSON
	data, err := extractor.MarshalJSON(extraction)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	// Save to cache
	if err := s.cache.Set(npm, "krs", cacheFile, data); err != nil {
		return nil, fmt.Errorf("save cache: %w", err)
	}

	return &entity.ExtractionResult{
		Success:   true,
		Message:   "KRS extracted successfully",
		NPM:       npm,
		FilePath:  filepath.Join(s.extractDir, npm, "krs", cacheFile),
		Timestamp: time.Now(),
	}, nil
}

// ExtractKHS extracts KHS data from the downloaded PDF.
func (s *extractionService) ExtractKHS(npm string, password string, tahunAjaran string, semester string) (*entity.ExtractionResult, error) {
	// Find the KHS PDF file
	pdfPath, err := s.findKHSFile(npm, tahunAjaran, semester)
	if err != nil {
		return nil, fmt.Errorf("find khs file: %w", err)
	}

	// Check cache first
	cacheFile := s.khsCacheFilename(tahunAjaran, semester)
	if s.cache.Exists(npm, "khs", cacheFile) {
		return &entity.ExtractionResult{
			Success:   true,
			Message:   "KHS extraction loaded from cache",
			NPM:       npm,
			FilePath:  filepath.Join(s.extractDir, npm, "khs", cacheFile),
			Timestamp: time.Now(),
		}, nil
	}

	// Parse PDF
	extraction, err := extractor.ParseKHS(pdfPath, npm, tahunAjaran, semester)
	if err != nil {
		return nil, fmt.Errorf("parse khs: %w", err)
	}

	// Update metadata
	fileInfo, _ := os.Stat(pdfPath)
	if fileInfo != nil {
		extraction.Metadata.FileSize = int(fileInfo.Size())
	}

	// Marshal to JSON
	data, err := extractor.MarshalJSON(extraction)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	// Save to cache
	if err := s.cache.Set(npm, "khs", cacheFile, data); err != nil {
		return nil, fmt.Errorf("save cache: %w", err)
	}

	return &entity.ExtractionResult{
		Success:   true,
		Message:   "KHS extracted successfully",
		NPM:       npm,
		FilePath:  filepath.Join(s.extractDir, npm, "khs", cacheFile),
		Timestamp: time.Now(),
	}, nil
}

// GetKRSExtraction retrieves cached KRS extraction.
func (s *extractionService) GetKRSExtraction(npm string) ([]byte, error) {
	// Find the latest KRS JSON file
	krsDir := filepath.Join(s.extractDir, npm, "krs")
	entries, err := os.ReadDir(krsDir)
	if err != nil {
		return nil, fmt.Errorf("read krs dir: %w", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no krs extraction found for npm %s", npm)
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
	return s.cache.Get(npm, "khs", cacheFile)
}

// findKRSFile finds the KRS PDF file for a given NPM.
func (s *extractionService) findKRSFile(npm string) (string, error) {
	krsDir := filepath.Join(s.downloadDir, npm, "krs")
	entries, err := os.ReadDir(krsDir)
	if err != nil {
		return "", fmt.Errorf("read krs dir: %w", err)
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".pdf" {
			return filepath.Join(krsDir, e.Name()), nil
		}
	}

	return "", fmt.Errorf("no krs pdf found for npm %s", npm)
}

// findKHSFile finds the KHS PDF file for a given NPM, tahun ajaran, and semester.
func (s *extractionService) findKHSFile(npm string, tahunAjaran string, semester string) (string, error) {
	khsDir := filepath.Join(s.downloadDir, npm, "khs")
	filename := s.khsFilename(tahunAjaran, semester)
	path := filepath.Join(khsDir, filename)

	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("khs pdf not found: %s", path)
	}

	return path, nil
}

// getKRSSemester extracts semester number from KRS filename.
// Handles: "semester_8.pdf" -> "8", "semester_12.pdf" -> "12"
func (s *extractionService) getKRSSemester(pdfPath string) string {
	filename := filepath.Base(pdfPath)
	// Remove .pdf extension
	name := strings.TrimSuffix(filename, ".pdf")
	// Remove "semester_" prefix
	if strings.HasPrefix(name, "semester_") {
		return strings.TrimPrefix(name, "semester_")
	}
	// Fallback: return cleaned filename
	return strings.ReplaceAll(name, " ", "_")
}

// khsFilename generates the canonical KHS filename.
func (s *extractionService) khsFilename(tahunAjaran string, semester string) string {
	// Replace / with _ in tahun_ajaran
	tahun := strings.Replace(tahunAjaran, "/", "_", -1)
	return fmt.Sprintf("%s_%s.pdf", tahun, semester)
}

// khsCacheFilename generates the KHS cache filename.
func (s *extractionService) khsCacheFilename(tahunAjaran string, semester string) string {
	tahun := strings.Replace(tahunAjaran, "/", "_", -1)
	return fmt.Sprintf("%s_%s.json", tahun, semester)
}
