package port

import (
	"time"

	"lonceng_unman_be/internal/domain/entity"
)

// PDFParser parses KRS and KHS PDF files into structured entities.
type PDFParser interface {
	ParseKRS(path string, npm string) (*entity.KRSExtraction, error)
	ParseKHS(path string, npm string, tahunAjaran string, semester string) (*entity.KHSExtraction, error)
	MarshalToJSON(v interface{}) ([]byte, error)
}

// ExtractionCache manages cached extraction results as JSON files.
type ExtractionCache interface {
	Get(npm string, docType string, filename string) ([]byte, error)
	Set(npm string, docType string, filename string, data []byte) error
	Exists(npm string, docType string, filename string) bool
	GetModTime(npm string, docType string, filename string) (time.Time, error)
	Invalidate(npm string, docType string, filename string) error
}
