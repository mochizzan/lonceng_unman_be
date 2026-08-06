package entity

import "time"

// ExtractionMetadata contains metadata about the extraction process.
type ExtractionMetadata struct {
	ExtractedAt time.Time `json:"extracted_at"`
	SourceFile  string    `json:"source_file"`
	FileSize    int       `json:"file_size"`
}

// KRSMahasiswa represents student info in KRS.
type KRSMahasiswa struct {
	Nama         string `json:"nama"`
	NPM          string `json:"npm"`
	ProgramStudi string `json:"program_studi"`
}

// KRSPeriode represents the academic period in KRS.
type KRSPeriode struct {
	TahunAjaran struct {
		Awal  string `json:"awal"`
		Akhir string `json:"akhir"`
	} `json:"tahun_ajaran"`
	Semester string `json:"semester"`
}

// KRSJadwal represents a class schedule in KRS.
type KRSJadwal struct {
	Hari         string `json:"hari"`
	WaktuMulai   string `json:"waktu_mulai"`
	WaktuSelesai string `json:"waktu_selesai"`
}

// KRSMataKuliah represents a course entry in KRS.
type KRSMataKuliah struct {
	No     int       `json:"no"`
	Kode   string    `json:"kode"`
	Nama   string    `json:"nama"`
	SKS    int       `json:"sks"`
	Kelas  string    `json:"kelas"`
	Dosen  string    `json:"dosen"`
	Jadwal KRSJadwal `json:"jadwal"`
}

// KRSPersetujuan represents approval section in KRS.
type KRSPersetujuan struct {
	Mahasiswa struct {
		Nama string `json:"nama"`
	} `json:"mahasiswa"`
	KetuaProgramStudi struct {
		Jabatan string  `json:"jabatan"`
		Nama    *string `json:"nama"`
		NIDN    *string `json:"nidn"`
	} `json:"ketua_program_studi"`
}

// KRSPenerbitan represents publication info in KRS.
type KRSPenerbitan struct {
	Tempat  string `json:"tempat"`
	Tanggal string `json:"tanggal"`
}

// KRSExtraction represents the full extracted KRS data.
type KRSExtraction struct {
	KRS struct {
		Mahasiswa   KRSMahasiswa    `json:"mahasiswa"`
		Periode     KRSPeriode      `json:"periode"`
		MataKuliah  []KRSMataKuliah `json:"mata_kuliah"`
		TotalSKS    int             `json:"total_sks"`
		Penerbitan  KRSPenerbitan   `json:"penerbitan"`
		Persetujuan KRSPersetujuan  `json:"persetujuan"`
	} `json:"krs"`
	Metadata ExtractionMetadata `json:"metadata"`
}

// KHSMahasiswa represents student info in KHS.
type KHSMahasiswa struct {
	Nama         string `json:"nama"`
	NPM          string `json:"npm"`
	ProgramStudi string `json:"program_studi"`
}

// KHSPeriode represents the academic period in KHS.
type KHSPeriode struct {
	TahunAjaran struct {
		Awal  string `json:"awal"`
		Akhir string `json:"akhir"`
	} `json:"tahun_ajaran"`
	Semester string `json:"semester"`
}

// KHSMataKuliah represents a course entry in KHS.
type KHSMataKuliah struct {
	No    int    `json:"no"`
	Kode  string `json:"kode"`
	Nama  string `json:"nama"`
	Dosen string `json:"dosen"`
	SKS   int    `json:"sks"`
	Nilai string `json:"nilai"`
	Mutu  int    `json:"mutu"`
}

// KHSRekapitulasi represents summary stats in KHS.
type KHSRekapitulasi struct {
	TotalSKS  int     `json:"total_sks"`
	TotalMutu int     `json:"total_mutu"`
	IPK       float64 `json:"ipk"`
}

// KHSPersetujuan represents approval section in KHS.
type KHSPersetujuan struct {
	Dekan struct {
		Jabatan string `json:"jabatan"`
		Nama    string `json:"nama"`
		NIDN    string `json:"nidn"`
	} `json:"dekan"`
}

// KHSPenerbitan represents publication info in KHS.
type KHSPenerbitan struct {
	Tempat  string `json:"tempat"`
	Tanggal string `json:"tanggal"`
}

// KHSExtraction represents the full extracted KHS data.
type KHSExtraction struct {
	KHS struct {
		Mahasiswa    KHSMahasiswa    `json:"mahasiswa"`
		Periode      KHSPeriode      `json:"periode"`
		MataKuliah   []KHSMataKuliah `json:"mata_kuliah"`
		Rekapitulasi KHSRekapitulasi `json:"rekapitulasi"`
		Penerbitan   KHSPenerbitan   `json:"penerbitan"`
		Persetujuan  KHSPersetujuan  `json:"persetujuan"`
	} `json:"khs"`
	Metadata ExtractionMetadata `json:"metadata"`
}

// ExtractionResult represents the outcome of an extraction operation.
type ExtractionResult struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	NPM       string    `json:"npm"`
	FilePath  string    `json:"file_path"`
	Timestamp time.Time `json:"timestamp"`
}
