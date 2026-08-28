package entity

import "time"

// ExtractedDocument records metadata about a JSON file produced by PDF extraction.
type ExtractedDocument struct {
	NPM              string    `json:"npm"`
	Type             string    `json:"type"`
	FilePath         string    `json:"file_path"`
	FileName         string    `json:"file_name"`
	FileSize         int64     `json:"file_size"`
	ExtractedAt      time.Time `json:"extracted_at"`
	DocumentCategory string    `json:"document_category"`
}

// GTDocument records metadata about a ground-truth JSON file.
type GTDocument struct {
	NPM              string    `json:"npm"`
	Type             string    `json:"type"`
	FilePath         string    `json:"file_path"`
	FileName         string    `json:"file_name"`
	FileSize         int64     `json:"file_size"`
	ExtractedAt      time.Time `json:"extracted_at"`
	DocumentCategory string    `json:"document_category"`
}

// StudentDocuments groups all documents for a single student.
type StudentDocuments struct {
	NPM       string              `json:"npm"`
	Raw       []RawDocument       `json:"raw"`
	Extracted []ExtractedDocument `json:"extracted"`
	GT        []GTDocument        `json:"gt"`
	Paired    bool                `json:"paired"`
}

// DocumentInventory aggregates document counts across all students.
type DocumentInventory struct {
	RawCount       int                          `json:"raw_count"`
	ExtractedCount int                          `json:"extracted_count"`
	GTCount        int                          `json:"gt_count"`
	PairedCount    int                          `json:"paired_count"`
	Students       map[string]*StudentDocuments `json:"students"`
}
