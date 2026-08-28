package service

import (
	"os"
	"path/filepath"

	"lonceng_unman_be/internal/domain/entity"
)

// DocumentService provides document inventory across download, extract, and eval directories.
type DocumentService struct {
	downloadDir string
	extractDir  string
	evalDir     string
}

// NewDocumentService creates a new DocumentService.
func NewDocumentService(downloadDir, extractDir, evalDir string) *DocumentService {
	return &DocumentService{
		downloadDir: downloadDir,
		extractDir:  extractDir,
		evalDir:     evalDir,
	}
}

// GetDocumentInventory scans all three directories and builds a map of NPM → StudentDocuments.
func (s *DocumentService) GetDocumentInventory() (*entity.DocumentInventory, error) {
	inventory := &entity.DocumentInventory{
		Students: make(map[string]*entity.StudentDocuments),
	}

	// Helper to get or create student entry
	getStudent := func(npm string) *entity.StudentDocuments {
		if stu, ok := inventory.Students[npm]; ok {
			return stu
		}
		stu := &entity.StudentDocuments{NPM: npm}
		inventory.Students[npm] = stu
		return stu
	}

	// Scan download dir for raw PDFs
	s.scanDir(s.downloadDir, func(npm, docType, path, name string, info os.FileInfo) {
		raw := entity.RawDocument{
			NPM:          npm,
			Type:         docType,
			FilePath:     path,
			FileName:     name,
			FileSize:     info.Size(),
			DownloadedAt: info.ModTime(),
			IsValidPDF:   filepath.Ext(name) == entity.ExtPDF,
		}
		getStudent(npm).Raw = append(getStudent(npm).Raw, raw)
		inventory.RawCount++
	})

	// Scan extract dir for JSON files
	s.scanDir(s.extractDir, func(npm, docType, path, name string, info os.FileInfo) {
		extracted := entity.ExtractedDocument{
			NPM:              npm,
			Type:             docType,
			FilePath:         path,
			FileName:         name,
			FileSize:         info.Size(),
			ExtractedAt:      info.ModTime(),
			DocumentCategory: string(entity.CategoryExtracted),
		}
		getStudent(npm).Extracted = append(getStudent(npm).Extracted, extracted)
		inventory.ExtractedCount++
	})

	// Scan eval dir for JSON files
	s.scanDir(s.evalDir, func(npm, docType, path, name string, info os.FileInfo) {
		gt := entity.GTDocument{
			NPM:              npm,
			Type:             docType,
			FilePath:         path,
			FileName:         name,
			FileSize:         info.Size(),
			ExtractedAt:      info.ModTime(),
			DocumentCategory: string(entity.CategoryGT),
		}
		getStudent(npm).GT = append(getStudent(npm).GT, gt)
		inventory.GTCount++
	})

	// Compute paired status per student
	for _, stu := range inventory.Students {
		if len(stu.GT) > 0 && len(stu.Extracted) > 0 {
			stu.Paired = true
			inventory.PairedCount++
		}
	}

	return inventory, nil
}

// scanDir walks the directory structure {base}/{npm}/{docType}/{file} and invokes the callback.
func (s *DocumentService) scanDir(base string, cb func(npm, docType, path, name string, info os.FileInfo)) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return // directory may not exist yet
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		npm := e.Name()
		npmDir := filepath.Join(base, npm)

		docTypes, err := os.ReadDir(npmDir)
		if err != nil {
			continue
		}
		for _, dt := range docTypes {
			if !dt.IsDir() {
				continue
			}
			docType := dt.Name()
			docDir := filepath.Join(npmDir, docType)

			files, err := os.ReadDir(docDir)
			if err != nil {
				continue
			}
			for _, f := range files {
				if f.IsDir() {
					continue
				}
				info, err := f.Info()
				if err != nil {
					continue
				}
				path := filepath.Join(docDir, f.Name())
				cb(npm, docType, path, f.Name(), info)
			}
		}
	}
}
