package entity

import "time"

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
	NPM       string        `json:"npm"`
	Semesters []KHSSemester `json:"semesters"`
	Timestamp time.Time     `json:"timestamp"`
}
