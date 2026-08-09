package entity

// StudentProfile represents the complete student profile data from LMS.
// Data is scraped from the HTML form at the student profile page.
type StudentProfile struct {
	PersonalData   PersonalData   `json:"personal_data"`
	ContactData    ContactData    `json:"contact_data"`
	EducationData  EducationData  `json:"education_data"`
	AddressData    AddressData    `json:"address_data"`
	EmploymentData EmploymentData `json:"employment_data"`
	FatherData     FatherData     `json:"father_data"`
	MotherData     MotherData     `json:"mother_data"`
	GuardianData   GuardianData   `json:"guardian_data"`
	OtherData      OtherData      `json:"other_data"`
}

type PersonalData struct {
	NIM            string `json:"nim"`
	NISN           string `json:"nisn"`
	NIK            string `json:"nik"`
	NamaMahasiswa  string `json:"nama_mahasiswa"`
	ProgramStudi   string `json:"program_studi"`
	Semester       string `json:"semester"`
	Kelas          string `json:"kelas"`
	StatusKonversi string `json:"status_konversi"`
}

type ContactData struct {
	NoWA            string `json:"no_wa"`
	Email           string `json:"email"`
	TempatLahir     string `json:"tempat_lahir"`
	TanggalLahir    string `json:"tanggal_lahir"`
	Agama           string `json:"agama"`
	Kelamin         string `json:"kelamin"`
	Suku            string `json:"suku"`
	StatusMenikah   string `json:"status_menikah"`
	KebutuhanKhusus string `json:"id_kebutuhan_khusus_mahasiswa"`
	StatusTinggal   string `json:"status_tinggal"`
	Transportasi    string `json:"transportasi"`
}

type EducationData struct {
	NamaAsalSekolah string `json:"nama_asal_sekolah"`
	TahunLulus      string `json:"tahun_lulus"`
}

type AddressData struct {
	Provinsi    string `json:"propinsi"`
	Kabupaten   string `json:"kabupaten"`
	Kecamatan   string `json:"kecamatan"`
	IdWilayah   string `json:"id_wilayah"`
	Desa        string `json:"desa"`
	AlamatDusun string `json:"alamat_dusun"`
	AlamatRW    string `json:"alamat_rw"`
	AlamatRT    string `json:"alamat_rt"`
	AlamatJalan string `json:"alamat_jalan"`
	KodePos     string `json:"kode_pos"`
}

type EmploymentData struct {
	StatusBekerja string `json:"status_bekerja"`
	NamaKantor    string `json:"nama_kantor"`
	AlamatKantor  string `json:"alamat_kantor"`
}

type FatherData struct {
	NamaAyah              string `json:"nama_ayah"`
	TanggalLahirAyah      string `json:"tanggal_lahir_ayah"`
	NIKAyah               string `json:"nik_ayah"`
	NoHpAyah              string `json:"no_hp_ayah"`
	JenjangPendidikanAyah string `json:"id_jenjang_pendidikan_ayah"`
	PekerjaanAyah         string `json:"id_pekerjaan_ayah"`
	PenghasilanAyah       string `json:"id_penghasilan_ayah"`
	KebutuhanKhususAyah   string `json:"id_kebutuhan_khusus_ayah"`
}

type MotherData struct {
	NamaIbu              string `json:"nama_ibu"`
	TanggalLahirIbu      string `json:"tanggal_lahir_ibu"`
	NIKIbu               string `json:"nik_ibu"`
	NoHpIbu              string `json:"no_hp_ibu"`
	JenjangPendidikanIbu string `json:"id_jenjang_pendidikan_ibu"`
	PekerjaanIbu         string `json:"id_pekerjaan_ibu"`
	PenghasilanIbu       string `json:"id_penghasilan_ibu"`
	KebutuhanKhususIbu   string `json:"id_kebutuhan_khusus_ibu"`
}

type GuardianData struct {
	NamaWali              string `json:"nama_wali"`
	TanggalLahirWali      string `json:"tanggal_lahir_wali"`
	JenjangPendidikanWali string `json:"id_jenjang_pendidikan_wali"`
	PekerjaanWali         string `json:"id_pekerjaan_wali"`
	PenghasilanWali       string `json:"id_penghasilan_wali"`
}

type OtherData struct {
	PenerimaKPS string `json:"penerima_kps"`
	NoKPS       string `json:"no_kps"`
	NPWP        string `json:"npwp"`
	Remark      string `json:"remark"`
}

// StudentProfileRequest represents the input for student profile operations.
type StudentProfileRequest struct {
	NPM      string `json:"npm"`
	Password string `json:"password"`
}

// StudentProfileResult represents the outcome of a student profile scrape.
type StudentProfileResult struct {
	NPM      string `json:"npm"`
	Message  string `json:"message"`
	CachedAt string `json:"cached_at"`
}
