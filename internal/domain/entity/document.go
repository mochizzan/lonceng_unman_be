package entity

import (
	"fmt"
	"strings"
	"time"
)

// KRSDownloadRequest represents the input for a KRS PDF download.
type KRSDownloadRequest struct {
	NPM      string `json:"npm"`
	Password string `json:"password"`
}

// KHSSemestersRequest represents the input for fetching KHS semesters.
type KHSSemestersRequest struct {
	NPM      string `json:"npm"`
	Password string `json:"password"`
}

// KHSDownloadRequest represents the input for a KHS PDF download.
type KHSDownloadRequest struct {
	NPM         string `json:"npm"`
	Password    string `json:"password"`
	TahunAjaran string `json:"tahun_ajaran"`
	Semester    string `json:"semester"`
}

// KHSSemester represents one semester entry from the KHS list page.
type KHSSemester struct {
	TahunAjaran string `json:"tahun_ajaran"` // e.g. "2022/2023"
	Semester    string `json:"semester"`     // e.g. "GANJIL", "GENAP"
	SKS         int    `json:"sks"`          // total SKS for that semester
}

// KRSDownloadResult represents the outcome of a KRS PDF download.
type KRSDownloadResult struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	NPM       string    `json:"npm"`
	FilePath  string    `json:"file_path"` // canonical path: downloads/{NPM}/krs/{TahunAjaran}_{Semester}.pdf
	Size      int       `json:"size"`      // bytes
	Timestamp time.Time `json:"timestamp"`
}

// KHSDownloadResult represents the outcome of a KHS PDF download for one semester.
type KHSDownloadResult struct {
	Success     bool      `json:"success"`
	Message     string    `json:"message"`
	NPM         string    `json:"npm"`
	TahunAjaran string    `json:"tahun_ajaran"`
	Semester    string    `json:"semester"`
	FilePath    string    `json:"file_path"` // canonical path: downloads/{NPM}/khs/{TahunAjaran}_{Semester}.pdf
	Size        int       `json:"size"`      // bytes
	Timestamp   time.Time `json:"timestamp"`
}

// KHSSemestersResult represents the list of available KHS semesters.
type KHSSemestersResult struct {
	Success   bool          `json:"success"`
	Message   string        `json:"message"`
	NPM       string        `json:"npm"`
	Semesters []KHSSemester `json:"semesters"`
	Timestamp time.Time     `json:"timestamp"`
}

// Semester values for academic periods.
const (
	SemesterGanjil = "GANJIL"
	SemesterGenap  = "GENAP"
)

// ValidSemester checks if a string is a recognized semester value.
func ValidSemester(s string) bool {
	return s == SemesterGanjil || s == SemesterGenap
}

// DocumentType identifies the kind of academic document.
type DocumentType string

// Document type identifiers for KRS and KHS.
const (
	DocTypeKRS DocumentType = "krs"
	DocTypeKHS DocumentType = "khs"
)

// IsValid reports whether d is a recognized document type.
func (d DocumentType) IsValid() bool {
	return d == DocTypeKRS || d == DocTypeKHS
}

// DirName returns the canonical directory name for this document type.
func (d DocumentType) DirName() string {
	return string(d)
}

// String returns the string representation of the document type.
func (d DocumentType) String() string {
	return string(d)
}

// RawDocument records metadata about a downloaded PDF file.
type RawDocument struct {
	NPM          string    `json:"npm"`
	Type         string    `json:"type"`
	FilePath     string    `json:"file_path"`
	FileName     string    `json:"file_name"`
	FileSize     int64     `json:"file_size"`
	DownloadedAt time.Time `json:"downloaded_at"`
	SourceURL    string    `json:"source_url"`
	ContentHash  string    `json:"content_hash"`
	IsValidPDF   bool      `json:"is_valid_pdf"`
}

// File extension constants.
const (
	ExtPDF  = ".pdf"
	ExtJSON = ".json"
)

// KRSFilePrefix is the prefix for KRS semester filenames (e.g. "semester_7.pdf").
const KRSFilePrefix = "semester_"

// KHSFilename generates the canonical KHS PDF filename.
// Format: {TahunAjaran}_{Semester}.pdf where "/" in TahunAjaran is replaced with "_".
// Example: KHSFilename("2022/2023", "GANJIL") -> "2022_2023_GANJIL.pdf"
func KHSFilename(tahunAjaran string, semester string) string {
	semester = strings.ToUpper(semester)
	tahun := strings.Replace(tahunAjaran, "/", "_", -1)
	return fmt.Sprintf("%s_%s.pdf", tahun, semester)
}
