package student_profile

import (
	"encoding/json"
	"testing"

	"lonceng_unman_be/internal/domain/entity"
)

func TestStudentProfileJSONRoundTrip(t *testing.T) {
	profile := entity.StudentProfile{
		PersonalData: entity.PersonalData{
			NIM:            "2211700006",
			NISN:           "0012345678",
			NIK:            "3201234567890001",
			NamaMahasiswa:  "Budi Santoso",
			ProgramStudi:   "Teknik Informatika",
			Semester:       "5",
			Kelas:          "A",
			StatusKonversi: "Tidak",
		},
		ContactData: entity.ContactData{
			NoWA:            "081234567890",
			Email:           "budi@example.com",
			TempatLahir:     "Bandung",
			TanggalLahir:    "2000-05-15",
			Agama:           "1",
			Kelamin:         "Laki-laki",
			Suku:            "Sunda",
			StatusMenikah:   "Belum",
			KebutuhanKhusus: "Tidak Ada",
			StatusTinggal:   "Kost",
			Transportasi:    "Kendaraan Pribadi",
		},
		EducationData: entity.EducationData{
			NamaAsalSekolah: "SMA Negeri 1 Bandung",
			TahunLulus:      "2018",
		},
		AddressData: entity.AddressData{
			Provinsi:    "Jawa Barat",
			Kabupaten:   "Bandung",
			Kecamatan:   "Coblong",
			IdWilayah:   "320101",
			Desa:        "Dago",
			AlamatDusun: "Dusun 1",
			AlamatRW:    "001",
			AlamatRT:    "002",
			AlamatJalan: "Jl. Dago No. 10",
			KodePos:     "40135",
		},
		EmploymentData: entity.EmploymentData{
			StatusBekerja: "Tidak Bekerja",
			NamaKantor:    "",
			AlamatKantor:  "",
		},
		FatherData: entity.FatherData{
			NamaAyah:              "Ahmad Santoso",
			TanggalLahirAyah:      "1965-03-20",
			NIKAyah:               "3201234567890010",
			NoHpAyah:              "081234567891",
			JenjangPendidikanAyah: "4",
			PekerjaanAyah:         "2",
			PenghasilanAyah:       "3",
			KebutuhanKhususAyah:   "Tidak Ada",
		},
		MotherData: entity.MotherData{
			NamaIbu:              "Siti Santoso",
			TanggalLahirIbu:      "1967-07-10",
			NIKIbu:               "3201234567890020",
			NoHpIbu:              "081234567892",
			JenjangPendidikanIbu: "4",
			PekerjaanIbu:         "1",
			PenghasilanIbu:       "2",
			KebutuhanKhususIbu:   "Tidak Ada",
		},
		GuardianData: entity.GuardianData{
			NamaWali:              "",
			TanggalLahirWali:      "",
			JenjangPendidikanWali: "",
			PekerjaanWali:         "",
			PenghasilanWali:       "",
		},
		OtherData: entity.OtherData{
			PenerimaKPS: "Tidak",
			NoKPS:       "",
			NPWP:        "",
			Remark:      "",
		},
	}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded entity.StudentProfile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify round-trip for PersonalData
	if decoded.PersonalData.NIM != "2211700006" {
		t.Errorf("PersonalData.NIM = %q, want %q", decoded.PersonalData.NIM, "2211700006")
	}
	if decoded.PersonalData.NamaMahasiswa != "Budi Santoso" {
		t.Errorf("PersonalData.NamaMahasiswa = %q, want %q", decoded.PersonalData.NamaMahasiswa, "Budi Santoso")
	}

	// Verify round-trip for ContactData
	if decoded.ContactData.Email != "budi@example.com" {
		t.Errorf("ContactData.Email = %q, want %q", decoded.ContactData.Email, "budi@example.com")
	}
	if decoded.ContactData.Kelamin != "Laki-laki" {
		t.Errorf("ContactData.Kelamin = %q, want %q", decoded.ContactData.Kelamin, "Laki-laki")
	}

	// Verify round-trip for EducationData
	if decoded.EducationData.NamaAsalSekolah != "SMA Negeri 1 Bandung" {
		t.Errorf("EducationData.NamaAsalSekolah = %q, want %q", decoded.EducationData.NamaAsalSekolah, "SMA Negeri 1 Bandung")
	}

	// Verify round-trip for AddressData
	if decoded.AddressData.Provinsi != "Jawa Barat" {
		t.Errorf("AddressData.Provinsi = %q, want %q", decoded.AddressData.Provinsi, "Jawa Barat")
	}
	if decoded.AddressData.AlamatJalan != "Jl. Dago No. 10" {
		t.Errorf("AddressData.AlamatJalan = %q, want %q", decoded.AddressData.AlamatJalan, "Jl. Dago No. 10")
	}

	// Verify round-trip for EmploymentData
	if decoded.EmploymentData.StatusBekerja != "Tidak Bekerja" {
		t.Errorf("EmploymentData.StatusBekerja = %q, want %q", decoded.EmploymentData.StatusBekerja, "Tidak Bekerja")
	}

	// Verify round-trip for FatherData
	if decoded.FatherData.NamaAyah != "Ahmad Santoso" {
		t.Errorf("FatherData.NamaAyah = %q, want %q", decoded.FatherData.NamaAyah, "Ahmad Santoso")
	}

	// Verify round-trip for MotherData
	if decoded.MotherData.NamaIbu != "Siti Santoso" {
		t.Errorf("MotherData.NamaIbu = %q, want %q", decoded.MotherData.NamaIbu, "Siti Santoso")
	}

	// Verify round-trip for GuardianData (empty)
	if decoded.GuardianData.NamaWali != "" {
		t.Errorf("GuardianData.NamaWali = %q, want empty", decoded.GuardianData.NamaWali)
	}

	// Verify round-trip for OtherData
	if decoded.OtherData.PenerimaKPS != "Tidak" {
		t.Errorf("OtherData.PenerimaKPS = %q, want %q", decoded.OtherData.PenerimaKPS, "Tidak")
	}
}

func TestStudentProfileJSONTags(t *testing.T) {
	profile := entity.StudentProfile{}
	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("Marshal empty profile failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	// Check top-level keys
	topLevelKeys := []string{
		"personal_data", "contact_data", "education_data", "address_data",
		"employment_data", "father_data", "mother_data", "guardian_data", "other_data",
	}
	for _, key := range topLevelKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing top-level key %q", key)
		}
	}

	// Verify PersonalData JSON tags
	personalData := raw["personal_data"].(map[string]interface{})
	personalKeys := []string{
		"nim", "nisn", "nik", "nama_mahasiswa", "program_studi",
		"semester", "kelas", "status_konversi",
	}
	for _, key := range personalKeys {
		if _, ok := personalData[key]; !ok {
			t.Errorf("PersonalData missing key %q", key)
		}
	}

	// Verify ContactData JSON tags (includes id_ prefixed keys)
	contactData := raw["contact_data"].(map[string]interface{})
	contactKeys := []string{
		"no_wa", "email", "tempat_lahir", "tanggal_lahir", "agama",
		"kelamin", "suku", "status_menikah", "id_kebutuhan_khusus_mahasiswa",
		"status_tinggal", "transportasi",
	}
	for _, key := range contactKeys {
		if _, ok := contactData[key]; !ok {
			t.Errorf("ContactData missing key %q", key)
		}
	}

	// Verify EducationData JSON tags
	educationData := raw["education_data"].(map[string]interface{})
	educationKeys := []string{"nama_asal_sekolah", "tahun_lulus"}
	for _, key := range educationKeys {
		if _, ok := educationData[key]; !ok {
			t.Errorf("EducationData missing key %q", key)
		}
	}

	// Verify AddressData JSON tags (note: propinsi not provinsi)
	addressData := raw["address_data"].(map[string]interface{})
	addressKeys := []string{
		"propinsi", "kabupaten", "kecamatan", "id_wilayah", "desa",
		"alamat_dusun", "alamat_rw", "alamat_rt", "alamat_jalan", "kode_pos",
	}
	for _, key := range addressKeys {
		if _, ok := addressData[key]; !ok {
			t.Errorf("AddressData missing key %q", key)
		}
	}
	// Explicitly verify "propinsi" is the tag, not "provinsi"
	if _, ok := addressData["provinsi"]; ok {
		t.Error("AddressData should use 'propinsi' tag, not 'provinsi'")
	}

	// Verify EmploymentData JSON tags
	employmentData := raw["employment_data"].(map[string]interface{})
	employmentKeys := []string{"status_bekerja", "nama_kantor", "alamat_kantor"}
	for _, key := range employmentKeys {
		if _, ok := employmentData[key]; !ok {
			t.Errorf("EmploymentData missing key %q", key)
		}
	}

	// Verify FatherData JSON tags (includes id_ prefixed keys)
	fatherData := raw["father_data"].(map[string]interface{})
	fatherKeys := []string{
		"nama_ayah", "tanggal_lahir_ayah", "nik_ayah", "no_hp_ayah",
		"id_jenjang_pendidikan_ayah", "id_pekerjaan_ayah",
		"id_penghasilan_ayah", "id_kebutuhan_khusus_ayah",
	}
	for _, key := range fatherKeys {
		if _, ok := fatherData[key]; !ok {
			t.Errorf("FatherData missing key %q", key)
		}
	}

	// Verify MotherData JSON tags (includes id_ prefixed keys)
	motherData := raw["mother_data"].(map[string]interface{})
	motherKeys := []string{
		"nama_ibu", "tanggal_lahir_ibu", "nik_ibu", "no_hp_ibu",
		"id_jenjang_pendidikan_ibu", "id_pekerjaan_ibu",
		"id_penghasilan_ibu", "id_kebutuhan_khusus_ibu",
	}
	for _, key := range motherKeys {
		if _, ok := motherData[key]; !ok {
			t.Errorf("MotherData missing key %q", key)
		}
	}

	// Verify GuardianData JSON tags (includes id_ prefixed keys)
	guardianData := raw["guardian_data"].(map[string]interface{})
	guardianKeys := []string{
		"nama_wali", "tanggal_lahir_wali", "id_jenjang_pendidikan_wali",
		"id_pekerjaan_wali", "id_penghasilan_wali",
	}
	for _, key := range guardianKeys {
		if _, ok := guardianData[key]; !ok {
			t.Errorf("GuardianData missing key %q", key)
		}
	}

	// Verify OtherData JSON tags
	otherData := raw["other_data"].(map[string]interface{})
	otherKeys := []string{"penerima_kps", "no_kps", "npwp", "remark"}
	for _, key := range otherKeys {
		if _, ok := otherData[key]; !ok {
			t.Errorf("OtherData missing key %q", key)
		}
	}
}

func TestStudentProfileEmptyRoundTrip(t *testing.T) {
	profile := entity.StudentProfile{}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("Marshal empty profile failed: %v", err)
	}

	var decoded entity.StudentProfile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal empty profile failed: %v", err)
	}

	// All fields should be zero values
	if decoded.PersonalData.NIM != "" {
		t.Errorf("empty PersonalData.NIM = %q, want empty", decoded.PersonalData.NIM)
	}
	if decoded.ContactData.Email != "" {
		t.Errorf("empty ContactData.Email = %q, want empty", decoded.ContactData.Email)
	}
	if decoded.AddressData.AlamatJalan != "" {
		t.Errorf("empty AddressData.AlamatJalan = %q, want empty", decoded.AddressData.AlamatJalan)
	}
}

func TestStudentProfileRequestJSONRoundTrip(t *testing.T) {
	req := entity.StudentProfileRequest{
		NPM:      "2211700006",
		Password: "test123",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded entity.StudentProfileRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.NPM != "2211700006" {
		t.Errorf("NPM = %q, want %q", decoded.NPM, "2211700006")
	}
	if decoded.Password != "test123" {
		t.Errorf("Password = %q, want %q", decoded.Password, "test123")
	}
}

func TestStudentProfileRequestJSONTags(t *testing.T) {
	req := entity.StudentProfileRequest{}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	expectedKeys := []string{"npm", "password"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("StudentProfileRequest missing key %q", key)
		}
	}
}

func TestStudentProfileResultJSONRoundTrip(t *testing.T) {
	result := entity.StudentProfileResult{
		NPM:      "2211700006",
		Message:  "Profile scraped successfully",
		CachedAt: "2024-01-15T10:30:00Z",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded entity.StudentProfileResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.NPM != "2211700006" {
		t.Errorf("NPM = %q, want %q", decoded.NPM, "2211700006")
	}
	if decoded.Message != "Profile scraped successfully" {
		t.Errorf("Message = %q, want %q", decoded.Message, "Profile scraped successfully")
	}
	if decoded.CachedAt != "2024-01-15T10:30:00Z" {
		t.Errorf("CachedAt = %q, want %q", decoded.CachedAt, "2024-01-15T10:30:00Z")
	}
}

func TestStudentProfileResultJSONTags(t *testing.T) {
	result := entity.StudentProfileResult{}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	expectedKeys := []string{"npm", "message", "cached_at"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("StudentProfileResult missing key %q", key)
		}
	}
}

func TestStudentProfileJSONUnmarshalExtraFields(t *testing.T) {
	// JSON with extra unknown fields should not cause an error
	jsonData := `{
		"personal_data": {
			"nim": "2211700006",
			"nama_mahasiswa": "Test Student",
			"unknown_field": "should be ignored"
		},
		"contact_data": {
			"email": "test@example.com"
		},
		"extra_top_level": "ignored"
	}`

	var profile entity.StudentProfile
	if err := json.Unmarshal([]byte(jsonData), &profile); err != nil {
		t.Fatalf("Unmarshal with extra fields failed: %v", err)
	}

	if profile.PersonalData.NIM != "2211700006" {
		t.Errorf("PersonalData.NIM = %q, want %q", profile.PersonalData.NIM, "2211700006")
	}
	if profile.PersonalData.NamaMahasiswa != "Test Student" {
		t.Errorf("PersonalData.NamaMahasiswa = %q, want %q", profile.PersonalData.NamaMahasiswa, "Test Student")
	}
}

func TestStudentProfileJSONUnmarshalPartial(t *testing.T) {
	// JSON with only some fields should unmarshal correctly with zero values for missing fields
	jsonData := `{
		"personal_data": {
			"nim": "2211700006"
		}
	}`

	var profile entity.StudentProfile
	if err := json.Unmarshal([]byte(jsonData), &profile); err != nil {
		t.Fatalf("Unmarshal partial failed: %v", err)
	}

	if profile.PersonalData.NIM != "2211700006" {
		t.Errorf("PersonalData.NIM = %q, want %q", profile.PersonalData.NIM, "2211700006")
	}
	if profile.PersonalData.NISN != "" {
		t.Errorf("PersonalData.NISN = %q, want empty", profile.PersonalData.NISN)
	}
	if profile.ContactData.Email != "" {
		t.Errorf("ContactData.Email = %q, want empty", profile.ContactData.Email)
	}
}

func TestStudentProfileJSONInvalid(t *testing.T) {
	var profile entity.StudentProfile
	if err := json.Unmarshal([]byte("not valid json{{{"), &profile); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestStudentProfileAddressPropinsiTag(t *testing.T) {
	// The AddressData.Propinsi field uses the tag "propinsi" (not "provinsi").
	// This test verifies the tag is exactly "propinsi".
	profile := entity.StudentProfile{
		AddressData: entity.AddressData{
			Provinsi: "Jawa Barat",
		},
	}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	addr := raw["address_data"].(map[string]interface{})

	// Should have "propinsi" key
	if val, ok := addr["propinsi"]; !ok {
		t.Error("AddressData missing 'propinsi' key (tag mismatch)")
	} else if val != "Jawa Barat" {
		t.Errorf("address_data.propinsi = %v, want %q", val, "Jawa Barat")
	}

	// Should NOT have "provinsi" key
	if _, ok := addr["provinsi"]; ok {
		t.Error("AddressData has unexpected 'provinsi' key; tag should be 'propinsi'")
	}
}

func TestStudentProfileIDPrefixTags(t *testing.T) {
	// Several fields use "id_" prefixed JSON tags.
	// This test verifies they serialize correctly.
	profile := entity.StudentProfile{
		ContactData: entity.ContactData{
			KebutuhanKhusus: "Tidak Ada",
		},
		FatherData: entity.FatherData{
			JenjangPendidikanAyah: "4",
			PekerjaanAyah:         "2",
			PenghasilanAyah:       "3",
			KebutuhanKhususAyah:   "Tidak Ada",
		},
		MotherData: entity.MotherData{
			JenjangPendidikanIbu: "4",
			PekerjaanIbu:         "1",
			PenghasilanIbu:       "2",
			KebutuhanKhususIbu:   "Tidak Ada",
		},
		GuardianData: entity.GuardianData{
			JenjangPendidikanWali: "4",
			PekerjaanWali:         "2",
			PenghasilanWali:       "3",
		},
	}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	// ContactData id_ key
	contact := raw["contact_data"].(map[string]interface{})
	if _, ok := contact["id_kebutuhan_khusus_mahasiswa"]; !ok {
		t.Error("ContactData missing 'id_kebutuhan_khusus_mahasiswa' key")
	}

	// FatherData id_ keys
	father := raw["father_data"].(map[string]interface{})
	fatherIDKeys := []string{
		"id_jenjang_pendidikan_ayah", "id_pekerjaan_ayah",
		"id_penghasilan_ayah", "id_kebutuhan_khusus_ayah",
	}
	for _, key := range fatherIDKeys {
		if _, ok := father[key]; !ok {
			t.Errorf("FatherData missing 'id_' key %q", key)
		}
	}

	// MotherData id_ keys
	mother := raw["mother_data"].(map[string]interface{})
	motherIDKeys := []string{
		"id_jenjang_pendidikan_ibu", "id_pekerjaan_ibu",
		"id_penghasilan_ibu", "id_kebutuhan_khusus_ibu",
	}
	for _, key := range motherIDKeys {
		if _, ok := mother[key]; !ok {
			t.Errorf("MotherData missing 'id_' key %q", key)
		}
	}

	// GuardianData id_ keys
	guardian := raw["guardian_data"].(map[string]interface{})
	guardianIDKeys := []string{
		"id_jenjang_pendidikan_wali", "id_pekerjaan_wali",
		"id_penghasilan_wali",
	}
	for _, key := range guardianIDKeys {
		if _, ok := guardian[key]; !ok {
			t.Errorf("GuardianData missing 'id_' key %q", key)
		}
	}
}
