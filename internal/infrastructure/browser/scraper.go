package browser

import (
	"encoding/json"
	"fmt"
	"time"

	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
)

// studentProfileScraper implements port.StudentProfileScraper.
type studentProfileScraper struct{}

// NewStudentProfileScraper creates a StudentProfileScraper.
func NewStudentProfileScraper() port.StudentProfileScraper {
	return &studentProfileScraper{}
}

// scrapeJSCode is the bulk JavaScript that reads all form fields in one eval call.
// go-rod's Eval wraps the code in: function() { return (CODE).apply(this, arguments) }
// So we pass an async function expression directly — NOT an IIFE.
const scrapeJSCode = `async () => {
    const isPlaceholder = (el, selected) => {
        // Empty value is a placeholder
        if (selected.value === '') return true;
        // Common numeric placeholder values
        if (selected.value === '0' || selected.value === '-1') return true;
        // Text-based placeholders (Indonesian and English)
        const text = (selected.text || '').trim().toLowerCase();
        if (text.startsWith('pilih ') || text === 'pilih') return true;
        if (text === '--' || text === '-' || text === '---') return true;
        if (text === 'select' || text === 'choose') return true;
        // Disabled options are placeholders
        if (selected.disabled) return true;
        // data-placeholder attribute
        if (el.getAttribute('data-placeholder') === 'true') return true;
        return false;
    };

    const getVal = (id) => {
        const el = document.getElementById(id);
        if (!el) return '';
        if (el.tagName === 'SELECT') {
            const selected = el.options[el.selectedIndex];
            if (!selected) return el.value || '';
            // If the selected option is a placeholder, return empty
            if (isPlaceholder(el, selected)) return '';
            // Return display text (not value) for real selections
            return selected.text || selected.value || '';
        }
        return el.value || '';
    };

    return JSON.stringify({
        nim: getVal('nim'),
        nisn: getVal('nisn'),
        nik: getVal('nik'),
        nama_mahasiswa: getVal('nama_mahasiswa'),
        program_studi: getVal('program_studi'),
        semester: getVal('semester'),
        kelas: getVal('kelas'),
        status_konversi: getVal('status_konversi'),
        no_wa: getVal('no_wa'),
        email: getVal('email'),
        tempat_lahir: getVal('tempat_lahir'),
        tanggal_lahir: getVal('tanggal_lahir'),
        agama: getVal('agama'),
        kelamin: getVal('kelamin'),
        suku: getVal('suku'),
        status_menikah: getVal('status_menikah'),
        id_kebutuhan_khusus_mahasiswa: getVal('id_kebutuhan_khusus_mahasiswa'),
        status_tinggal: getVal('status_tinggal'),
        transportasi: getVal('transportasi'),
        nama_asal_sekolah: getVal('nama_asal_sekolah'),
        tahun_lulus: getVal('tahun_lulus'),
        propinsi: getVal('propinsi'),
        kabupaten: getVal('kabupaten'),
        kecamatan: getVal('kecamatan'),
        id_wilayah: getVal('id_wilayah'),
        desa: getVal('desa'),
        alamat_dusun: getVal('alamat_dusun'),
        alamat_rw: getVal('alamat_rw'),
        alamat_rt: getVal('alamat_rt'),
        alamat_jalan: getVal('alamat_jalan'),
        kode_pos: getVal('kode_pos'),
        status_bekerja: getVal('status_bekerja'),
        nama_kantor: getVal('nama_kantor'),
        alamat_kantor: getVal('alamat_kantor'),
        nama_ayah: getVal('nama_ayah'),
        // tanggal_lahir_ayah: removed — element does not exist in HTML form
        nik_ayah: getVal('nik_ayah'),
        no_hp_ayah: getVal('no_hp_ayah'),
        id_jenjang_pendidikan_ayah: getVal('id_jenjang_pendidikan_ayah'),
        id_pekerjaan_ayah: getVal('id_pekerjaan_ayah'),
        id_penghasilan_ayah: getVal('id_penghasilan_ayah'),
        id_kebutuhan_khusus_ayah: getVal('id_kebutuhan_khusus_ayah'),
        nama_ibu: getVal('nama_ibu_kandung'),
        tanggal_lahir_ibu: getVal('tanggal_lahir_ibu'),
        nik_ibu: getVal('nik_ibu'),
        no_hp_ibu: getVal('no_hp_ibu'),
        id_jenjang_pendidikan_ibu: getVal('id_jenjang_pendidikan_ibu'),
        id_pekerjaan_ibu: getVal('id_pekerjaan_ibu'),
        id_penghasilan_ibu: getVal('id_penghasilan_ibu'),
        id_kebutuhan_khusus_ibu: getVal('id_kebutuhan_khusus_ibu'),
        nama_wali: getVal('nama_wali'),
        tanggal_lahir_wali: getVal('tanggal_lahir_wali'),
        id_jenjang_pendidikan_wali: getVal('id_jenjang_pendidikan_wali'),
        id_pekerjaan_wali: getVal('id_pekerjaan_wali'),
        id_penghasilan_wali: getVal('id_penghasilan_wali'),
        penerima_kps: getVal('penerima_kps'),
        no_kps: getVal('no_kps'),
        npwp: getVal('npwp'),
        remark: getVal('remark')
    });
}`

// Scrape navigates to the profile page, validates the form exists,
// and reads all fields via a single bulk JS eval.
//
// The caller must hold the session's Lock() for the entire scrape to
// prevent concurrent requests from interleaving page operations on the
// shared rod.Page. This fixes the race condition where concurrent scrapes
// would fail immediately (3-10ms) because the page was left in a bad state
// by another goroutine.
func (s *studentProfileScraper) Scrape(session port.BrowserSession, lmsBaseURL string, cfg *config.Config) (*entity.StudentProfile, error) {
	// 1. Navigate to student profile page (construct full URL).
	profileURL := lmsBaseURL + port.StudentProfilePath
	if err := session.Navigate(profileURL); err != nil {
		return nil, fmt.Errorf("navigate to student profile: %w", err)
	}

	// 1.5. Verify page is healthy by checking the URL.
	// If the page is in a bad state (e.g., after a previous timeout),
	// the URL may be empty or about:blank. Navigate again if needed.
	pageCheck, err := session.Eval(`async () => window.location.href`)
	if err != nil || pageCheck == "" || pageCheck == `"about:blank"` {
		// Page is in a bad state, try navigating again.
		if err := session.Navigate(profileURL); err != nil {
			return nil, fmt.Errorf("re-navigate to student profile: %w", err)
		}
	}

	// 2. Validate form exists on the page.
	exists, err := session.ElementExists("form")
	if err != nil {
		return nil, fmt.Errorf("check form existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("student profile form not found on page")
	}

	// 2.5. Wait for JavaScript to populate form values.
	// Probe key input fields to verify JS has populated the form.
	// This is more reliable than a fixed sleep — it adapts to actual page
	// readiness instead of hoping a fixed duration is enough.
	//
	// Probe window is tunable via SCRAPE_FORM_WAIT; enforced to [2s,30s]
	// so operators can lower it for fast networks without racing the LMS
	// AJAX population (floor 2s matches config SCRAPE_FORM_WAIT default).
	var probeTimeout time.Duration
	if cfg != nil {
		probeTimeout = cfg.App.ScrapeFormWait
	}
	if probeTimeout <= 0 {
		probeTimeout = 2 * time.Second
	}
	if probeTimeout < 2*time.Second {
		probeTimeout = 2 * time.Second
	}
	if probeTimeout > 30*time.Second {
		probeTimeout = 30 * time.Second
	}
	ready := false
	deadline := time.Now().Add(probeTimeout)
	for time.Now().Before(deadline) {
		// Check multiple key fields — some pages populate inputs in stages
		check, err := session.Eval(`async () => {
			const fields = ['nim', 'nama_mahasiswa', 'nik', 'email', 'no_wa', 'tempat_lahir'];
			for (const id of fields) {
				const el = document.getElementById(id);
				if (el && el.value && el.value.trim() !== '') return el.value.trim();
			}
			return '';
		}`)
		if err == nil && check != "" && check != "''" {
			ready = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !ready {
		// Diagnostic: dump all input/select element IDs and values for debugging.
		dump, _ := session.Eval(`async () => {
			const result = {};
			document.querySelectorAll('input, select, textarea').forEach(el => {
				const id = el.id || el.name || 'unknown';
				let val = '';
				if (el.tagName === 'SELECT') {
					const sel = el.options[el.selectedIndex];
					val = sel ? (sel.text || sel.value || '') : '';
				} else {
					val = el.value || '';
				}
				result[id] = val.substring(0, 100);
			});
			return JSON.stringify(result);
		}`)
		return nil, fmt.Errorf("form fields not populated within %v — page may have failed to load student data. DOM snapshot: %s", probeTimeout, dump)
	}

	// 3. Bulk JS eval — read all 55+ fields in one call.
	result, err := session.Eval(scrapeJSCode)
	if err != nil {
		return nil, fmt.Errorf("scrape student profile: %w", err)
	}

	// 4. Parse JSON result into a flat map.
	var raw map[string]string
	if err := json.Unmarshal([]byte(result), &raw); err != nil {
		return nil, fmt.Errorf("parse scraped data: %w", err)
	}

	// 5. Map flat map to entity struct.
	profile := &entity.StudentProfile{
		PersonalData: entity.PersonalData{
			NIM:            raw["nim"],
			NISN:           raw["nisn"],
			NIK:            raw["nik"],
			NamaMahasiswa:  raw["nama_mahasiswa"],
			ProgramStudi:   raw["program_studi"],
			Semester:       raw["semester"],
			Kelas:          raw["kelas"],
			StatusKonversi: raw["status_konversi"],
		},
		ContactData: entity.ContactData{
			NoWA:            raw["no_wa"],
			Email:           raw["email"],
			TempatLahir:     raw["tempat_lahir"],
			TanggalLahir:    raw["tanggal_lahir"],
			Agama:           raw["agama"],
			Kelamin:         raw["kelamin"],
			Suku:            raw["suku"],
			StatusMenikah:   raw["status_menikah"],
			KebutuhanKhusus: raw["id_kebutuhan_khusus_mahasiswa"],
			StatusTinggal:   raw["status_tinggal"],
			Transportasi:    raw["transportasi"],
		},
		EducationData: entity.EducationData{
			NamaAsalSekolah: raw["nama_asal_sekolah"],
			TahunLulus:      raw["tahun_lulus"],
		},
		AddressData: entity.AddressData{
			Provinsi:    raw["propinsi"],
			Kabupaten:   raw["kabupaten"],
			Kecamatan:   raw["kecamatan"],
			IdWilayah:   raw["id_wilayah"],
			Desa:        raw["desa"],
			AlamatDusun: raw["alamat_dusun"],
			AlamatRW:    raw["alamat_rw"],
			AlamatRT:    raw["alamat_rt"],
			AlamatJalan: raw["alamat_jalan"],
			KodePos:     raw["kode_pos"],
		},
		EmploymentData: entity.EmploymentData{
			StatusBekerja: raw["status_bekerja"],
			NamaKantor:    raw["nama_kantor"],
			AlamatKantor:  raw["alamat_kantor"],
		},
		FatherData: entity.FatherData{
			NamaAyah:              raw["nama_ayah"],
			NIKAyah:               raw["nik_ayah"],
			NoHpAyah:              raw["no_hp_ayah"],
			JenjangPendidikanAyah: raw["id_jenjang_pendidikan_ayah"],
			PekerjaanAyah:         raw["id_pekerjaan_ayah"],
			PenghasilanAyah:       raw["id_penghasilan_ayah"],
			KebutuhanKhususAyah:   raw["id_kebutuhan_khusus_ayah"],
		},
		MotherData: entity.MotherData{
			NamaIbu:              raw["nama_ibu"],
			TanggalLahirIbu:      raw["tanggal_lahir_ibu"],
			NIKIbu:               raw["nik_ibu"],
			NoHpIbu:              raw["no_hp_ibu"],
			JenjangPendidikanIbu: raw["id_jenjang_pendidikan_ibu"],
			PekerjaanIbu:         raw["id_pekerjaan_ibu"],
			PenghasilanIbu:       raw["id_penghasilan_ibu"],
			KebutuhanKhususIbu:   raw["id_kebutuhan_khusus_ibu"],
		},
		GuardianData: entity.GuardianData{
			NamaWali:              raw["nama_wali"],
			TanggalLahirWali:      raw["tanggal_lahir_wali"],
			JenjangPendidikanWali: raw["id_jenjang_pendidikan_wali"],
			PekerjaanWali:         raw["id_pekerjaan_wali"],
			PenghasilanWali:       raw["id_penghasilan_wali"],
		},
		OtherData: entity.OtherData{
			PenerimaKPS: raw["penerima_kps"],
			NoKPS:       raw["no_kps"],
			NPWP:        raw["npwp"],
			Remark:      raw["remark"],
		},
	}

	return profile, nil
}

// Compile-time interface check.
var _ port.StudentProfileScraper = (*studentProfileScraper)(nil)
