package evalstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lonceng_unman_be/internal/domain/entity"
)

// Store implements port.EvalStore.
type Store struct {
	evalDir     string
	extractDir  string
	downloadDir string
}

// New creates a new EvalStore.
func New(evalDir, extractDir, downloadDir string) *Store {
	return &Store{evalDir: evalDir, extractDir: extractDir, downloadDir: downloadDir}
}

// ListNPMs returns all NPMs that have either extract, GT, or downloaded raw PDFs.
func (s *Store) ListNPMs() ([]string, error) {
	npmSet := make(map[string]bool)

	if entries, err := os.ReadDir(s.evalDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				npmSet[e.Name()] = true
			}
		}
	}

	if entries, err := os.ReadDir(s.extractDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				npmSet[e.Name()] = true
			}
		}
	}

	// Also scan downloadDir for NPM directories containing raw PDFs.
	if s.downloadDir != "" {
		if entries, err := os.ReadDir(s.downloadDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					npmSet[e.Name()] = true
				}
			}
		}
	}

	npms := make([]string, 0, len(npmSet))
	for npm := range npmSet {
		npms = append(npms, npm)
	}
	sort.Strings(npms)
	return npms, nil
}

// ListDocs returns all document files for a NPM.
func (s *Store) ListDocs(npm string) ([]entity.NPMDoc, error) {
	var docs []entity.NPMDoc

	for _, docType := range []string{"krs", "khs"} {
		gtDir := filepath.Join(s.evalDir, npm, docType)
		extractDir := filepath.Join(s.extractDir, npm, docType)
		downloadPDFDir := ""
		if s.downloadDir != "" {
			downloadPDFDir = filepath.Join(s.downloadDir, npm, docType)
		}

		gtFiles, _ := scanJSONDir(gtDir)
		extractFiles, _ := scanJSONDir(extractDir)
		rawFiles, _ := scanPDFDir(downloadPDFDir)

		gtSet := make(map[string]bool)
		for _, f := range gtFiles {
			gtSet[f] = true
		}
		extractSet := make(map[string]bool)
		for _, f := range extractFiles {
			extractSet[f] = true
		}
		// Normalize PDF filenames to their JSON-equivalent names so a raw
		// "semester_8.pdf" matches the JSON entry "semester_8.json".
		rawSet := make(map[string]bool)
		for _, f := range rawFiles {
			base := strings.TrimSuffix(f, ".pdf")
			rawSet[base+".json"] = true
		}

		fileSet := make(map[string]bool)
		for f := range gtSet {
			fileSet[f] = true
		}
		for f := range extractSet {
			fileSet[f] = true
		}
		for f := range rawSet {
			fileSet[f] = true
		}

		for f := range fileSet {
			docs = append(docs, entity.NPMDoc{
				DocType:    docType,
				File:       f,
				Paired:     gtSet[f] && extractSet[f],
				HasGT:      gtSet[f],
				HasExtract: extractSet[f],
				HasRaw:     rawSet[f],
			})
		}
	}

	sort.Slice(docs, func(i, j int) bool {
		if docs[i].DocType != docs[j].DocType {
			return docs[i].DocType < docs[j].DocType
		}
		return docs[i].File < docs[j].File
	})

	return docs, nil
}

// LoadGT loads GT JSON.
func (s *Store) LoadGT(npm, docType, filename string) ([]byte, error) {
	path := filepath.Join(s.evalDir, npm, docType, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read gt: %w", err)
	}
	return data, nil
}

// LoadExtract loads extract JSON.
func (s *Store) LoadExtract(npm, docType, filename string) ([]byte, error) {
	path := filepath.Join(s.extractDir, npm, docType, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read extract: %w", err)
	}
	return data, nil
}

// WriteGT writes GT JSON atomically (tmp + rename).
func (s *Store) WriteGT(npm, docType, filename string, data []byte) error {
	dir := filepath.Join(s.evalDir, npm, docType)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	path := filepath.Join(dir, filename)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("write gt tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename gt: %w", err)
	}
	return nil
}

// Exists checks if GT or extract exists.
func (s *Store) Exists(npm, docType, filename string) (bool, bool, error) {
	gtPath := filepath.Join(s.evalDir, npm, docType, filename)
	extractPath := filepath.Join(s.extractDir, npm, docType, filename)
	gtExists := false
	extractExists := false
	if _, err := os.Stat(gtPath); err == nil {
		gtExists = true
	}
	if _, err := os.Stat(extractPath); err == nil {
		extractExists = true
	}
	return gtExists, extractExists, nil
}

// Helper to scan directory for JSON files
func scanJSONDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

// Helper to scan directory for PDF files
func scanPDFDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".pdf") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}
