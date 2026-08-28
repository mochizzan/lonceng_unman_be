package entity

// ConfusionCounts represents the four possible outcomes for a single field or course entry.
type ConfusionCounts struct {
	TP int `json:"tp"`
	FN int `json:"fn"`
	FP int `json:"fp"`
	TN int `json:"tn"`
}

// Metrics holds the overall evaluation metrics for a paired document pair.
type Metrics struct {
	Confusion ConfusionCounts `json:"confusion"`
	Precision float64         `json:"precision"`
	Recall    float64         `json:"recall"`
	F1        float64         `json:"f1"`
	Accuracy  float64         `json:"accuracy,omitempty"`
}

// NPMDoc represents a single document file (KRS or KHS) for a student.
type NPMDoc struct {
	DocType    string `json:"doc_type"`
	File       string `json:"file"`
	Paired     bool   `json:"paired"`
	HasGT      bool   `json:"has_gt"`
	HasExtract bool   `json:"has_extract"`
	HasRaw     bool   `json:"has_raw"`
}

// FieldHeatmapRow represents a single field's heatmap aggregation.
type FieldHeatmapRow struct {
	Field   string `json:"field"`
	Correct int    `json:"correct"`
	Wrong   int    `json:"wrong"`
	Missing int    `json:"missing"`
	Extra   int    `json:"extra"`
}

// CompareRow represents a single field comparison between GT and prediction.
type CompareRow struct {
	Field  string `json:"field"`
	GT     string `json:"gt"`
	Pred   string `json:"pred"`
	Status string `json:"status"`
}

// DocEval represents per-document evaluation result.
type DocEval struct {
	File        string            `json:"file"`
	Paired      bool              `json:"paired"`
	HasGT       bool              `json:"has_gt"`
	HasExtract  bool              `json:"has_extract"`
	HasRaw      bool              `json:"has_raw"`
	Semester    string            `json:"semester,omitempty"`
	TahunAjaran string            `json:"tahun_ajaran,omitempty"`
	Metrics     Metrics           `json:"metrics"`
	Heatmap     []FieldHeatmapRow `json:"heatmap,omitempty"`
	CompareRows []CompareRow      `json:"compare_rows,omitempty"`
}

// StudentEval represents the full evaluation view for a single student.
type StudentEval struct {
	NPM          string    `json:"npm"`
	Name         string    `json:"name"`
	ProgramStudi string    `json:"program_studi"`
	KRSDocs      []DocEval `json:"krs_docs"`
	KHSDocs      []DocEval `json:"khs_docs"`
	KRSMetrics   Metrics   `json:"krs_metrics"`
	KHSMetrics   Metrics   `json:"khs_metrics"`
}

// SaveGTRequest represents a request to save ground truth for a document.
type SaveGTRequest struct {
	ConfirmOverwrite bool          `json:"confirm_overwrite"`
	KRS              KRSExtraction `json:"krs,omitempty"`
	KHS              KHSExtraction `json:"khs,omitempty"`
}

// UnifiedTableRow represents a single row in the unified evaluation table.
type UnifiedTableRow struct {
	NPM         string  `json:"npm"`
	Name        string  `json:"name"`
	DocType     string  `json:"doc_type"`
	Category    string  `json:"category"` // "KRS" or "KHS"
	Semester    string  `json:"semester,omitempty"`
	TahunAjaran string  `json:"tahun_ajaran,omitempty"`
	Status      string  `json:"status"`
	StatusClass string  `json:"status_class"`
	TP          int     `json:"tp"`
	FN          int     `json:"fn"`
	FP          int     `json:"fp"`
	Precision   float64 `json:"precision"`
	Recall      float64 `json:"recall"`
	F1          float64 `json:"f1"`
	Action      string  `json:"action"`
	File        string  `json:"file"`
	Paired      bool    `json:"paired"`
	HasGT       bool    `json:"has_gt"`
	HasExtract  bool    `json:"has_extract"`
	HasRaw      bool    `json:"has_raw"`
	Metrics     Metrics `json:"metrics"`
}

// UnifiedEval is the new aggregate view for unified evaluation page.
type UnifiedEval struct {
	TotalNPMs   int               `json:"total_npms"`
	UnifiedRows []UnifiedTableRow `json:"unified_rows"`
	KRSMetrics  Metrics           `json:"krs_metrics"`
	KHSMetrics  Metrics           `json:"khs_metrics"`
	RawPDFURL   string            `json:"raw_pdf_url"`
	PDFPreview  *PDFPreview       `json:"pdf_preview,omitempty"`
}

// PDFPreview represents raw PDF preview data.
type PDFPreview struct {
	NPM         string `json:"npm"`
	File        string `json:"file"`
	URL         string `json:"url"`
	Base64      string `json:"base64,omitempty"`
	IsAvailable bool   `json:"is_available"`
}

// IndexEval is kept for backward compatibility.
type IndexEval struct {
	TotalNPMs      int        `json:"total_npms"`
	PairedKRSCount int        `json:"paired_krs_count"`
	PairedKHSCount int        `json:"paired_khs_count"`
	KRSMetrics     Metrics    `json:"krs_metrics"`
	KHSMetrics     Metrics    `json:"khs_metrics"`
	KRSRows        []IndexRow `json:"krs_rows"`
	KHSRows        []IndexRow `json:"khs_rows"`
}

// IndexRow is kept for compatibility.
type IndexRow struct {
	NPM         string  `json:"npm"`
	Name        string  `json:"nama"`
	Semester    string  `json:"semester,omitempty"`
	TahunAjaran string  `json:"tahun_ajaran,omitempty"`
	DocType     string  `json:"doc_type"`
	File        string  `json:"file"`
	Paired      bool    `json:"paired"`
	HasGT       bool    `json:"has_gt"`
	HasExtract  bool    `json:"has_extract"`
	Metrics     Metrics `json:"metrics"`
}
