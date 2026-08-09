package student_profile

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
	"lonceng_unman_be/internal/infrastructure/photocache"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockBrowserSession struct{}

func (m *mockBrowserSession) Navigate(url string) error      { return nil }
func (m *mockBrowserSession) Eval(js string) (string, error) { return "{}", nil }
func (m *mockBrowserSession) ElementAttribute(sel, attr string) (string, error) {
	return "", nil
}
func (m *mockBrowserSession) ElementExists(sel string) (bool, error) { return true, nil }
func (m *mockBrowserSession) ElementHref(sel string) (string, error) { return "", nil }
func (m *mockBrowserSession) DownloadPDF(url, save string) (string, int, error) {
	return "", 0, nil
}

func (m *mockBrowserSession) DownloadImage(url, save string) (string, int, error) {
	return "", 0, nil
}
func (m *mockBrowserSession) Close() error { return nil }

type mockSessionManager struct {
	createFunc func(npm, password string) (port.BrowserSession, error)
	closeFunc  func(npm string) error
}

func (m *mockSessionManager) GetOrCreate(npm, password string) (port.BrowserSession, error) {
	if m.createFunc != nil {
		return m.createFunc(npm, password)
	}
	return &mockBrowserSession{}, nil
}

func (m *mockSessionManager) Close(npm string) error {
	if m.closeFunc != nil {
		return m.closeFunc(npm)
	}
	return nil
}
func (m *mockSessionManager) CloseAll() {}

type mockStudentProfileScraper struct {
	scrapeFunc func(session port.BrowserSession, lmsBaseURL string) (*entity.StudentProfile, error)
}

func (m *mockStudentProfileScraper) Scrape(session port.BrowserSession, lmsBaseURL string) (*entity.StudentProfile, error) {
	if m.scrapeFunc != nil {
		return m.scrapeFunc(session, lmsBaseURL)
	}
	return &entity.StudentProfile{
		PersonalData: entity.PersonalData{
			NIM:           "2211700006",
			NamaMahasiswa: "Test Student",
			ProgramStudi:  "TI",
			Semester:      "5",
			Kelas:         "A",
		},
		ContactData: entity.ContactData{
			Email:   "test@example.com",
			Kelamin: "L",
			Agama:   "1",
		},
	}, nil
}

type mockExtractionCache struct {
	existsFunc func(npm, docType, filename string) bool
	getFunc    func(npm, docType, filename string) ([]byte, error)
	setFunc    func(npm, docType, filename string, data []byte) error
	data       map[string][]byte
}

func newMockCache() *mockExtractionCache {
	return &mockExtractionCache{data: make(map[string][]byte)}
}

func (m *mockExtractionCache) cacheKey(npm, docType, filename string) string {
	return npm + "/" + docType + "/" + filename
}

func (m *mockExtractionCache) Get(npm, docType, filename string) ([]byte, error) {
	if m.getFunc != nil {
		return m.getFunc(npm, docType, filename)
	}
	key := m.cacheKey(npm, docType, filename)
	if d, ok := m.data[key]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockExtractionCache) Set(npm, docType, filename string, data []byte) error {
	if m.setFunc != nil {
		return m.setFunc(npm, docType, filename, data)
	}
	key := m.cacheKey(npm, docType, filename)
	m.data[key] = data
	return nil
}

func (m *mockExtractionCache) Exists(npm, docType, filename string) bool {
	if m.existsFunc != nil {
		return m.existsFunc(npm, docType, filename)
	}
	key := m.cacheKey(npm, docType, filename)
	_, ok := m.data[key]
	return ok
}

func (m *mockExtractionCache) GetModTime(npm, docType, filename string) (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockExtractionCache) Invalidate(npm, docType, filename string) error {
	key := m.cacheKey(npm, docType, filename)
	delete(m.data, key)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestConfig() *config.Config {
	return &config.Config{
		App: config.AppConfig{
			Name:           "test-service",
			Env:            "testing",
			Port:           "8080",
			Host:           "localhost",
			LMSBaseURL:     "https://lms.test",
			DownloadDir:    "/tmp/downloads",
			ExtractDir:     "/tmp/extracts",
			SessionTTL:     30 * time.Minute,
			MaxSessions:    5,
			MaxBodySize:    10 << 20,
			MaxPDFSize:     5 << 20,
			ProfileBaseDir: "/tmp/profiles",
		},
	}
}

func newTestService(
	sessions port.SessionManager,
	scraper port.StudentProfileScraper,
	cache port.ExtractionCache,
) service.StudentProfileService {
	photoCache := photocache.New("/tmp/downloads", 15*time.Minute)
	return service.NewStudentProfileService(newTestConfig(), sessions, scraper, cache, photoCache)
}

var testReq = entity.StudentProfileRequest{
	NPM:      "2211700006",
	Password: "test123",
}

// ---------------------------------------------------------------------------
// Tests: Scrape
// ---------------------------------------------------------------------------

func TestService_Scrape_Success(t *testing.T) {
	sessions := &mockSessionManager{}
	scraper := &mockStudentProfileScraper{}
	cache := newMockCache()

	svc := newTestService(sessions, scraper, cache)

	result, err := svc.Scrape(testReq)
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}
	if result.NPM != testReq.NPM {
		t.Errorf("NPM = %q, want %q", result.NPM, testReq.NPM)
	}
	if result.Message != "Profile scraped successfully" {
		t.Errorf("Message = %q, want %q", result.Message, "Profile scraped successfully")
	}
	if result.CachedAt == "" {
		t.Error("CachedAt should not be empty")
	}

	// Verify data was cached
	if !cache.Exists(testReq.NPM, "profile", "student_profile.json") {
		t.Error("profile should be cached after Scrape()")
	}
}

func TestScrape_SessionError(t *testing.T) {
	sessions := &mockSessionManager{
		createFunc: func(npm, password string) (port.BrowserSession, error) {
			return nil, fmt.Errorf("login failed")
		},
	}
	scraper := &mockStudentProfileScraper{}
	cache := newMockCache()

	svc := newTestService(sessions, scraper, cache)

	result, err := svc.Scrape(testReq)
	if err == nil {
		t.Fatal("Scrape() should return error when session creation fails")
	}
	if result != nil {
		t.Error("result should be nil on error")
	}
}

func TestScrape_ScraperError(t *testing.T) {
	sessions := &mockSessionManager{}
	scraper := &mockStudentProfileScraper{
		scrapeFunc: func(session port.BrowserSession, lmsBaseURL string) (*entity.StudentProfile, error) {
			return nil, fmt.Errorf("scrape failed")
		},
	}
	cache := newMockCache()

	svc := newTestService(sessions, scraper, cache)

	result, err := svc.Scrape(testReq)
	if err == nil {
		t.Fatal("Scrape() should return error when scraper fails")
	}
	if result != nil {
		t.Error("result should be nil on error")
	}
}

func TestScrape_CacheSetError_NonFatal(t *testing.T) {
	sessions := &mockSessionManager{}
	scraper := &mockStudentProfileScraper{}
	cache := &mockExtractionCache{
		setFunc: func(npm, docType, filename string, data []byte) error {
			return fmt.Errorf("disk full")
		},
	}

	svc := newTestService(sessions, scraper, cache)

	// Scrape should succeed even when cache write fails (non-fatal)
	result, err := svc.Scrape(testReq)
	if err != nil {
		t.Fatalf("Scrape() should not fail when cache write fails, got: %v", err)
	}
	if result.NPM != testReq.NPM {
		t.Errorf("NPM = %q, want %q", result.NPM, testReq.NPM)
	}
}

func TestScrape_ProfileData(t *testing.T) {
	expectedProfile := &entity.StudentProfile{
		PersonalData: entity.PersonalData{
			NIM:           "2211700006",
			NamaMahasiswa: "Mahasiswa Tes",
			ProgramStudi:  "Teknologi Informasi",
			Semester:      "7",
			Kelas:         "B",
		},
		ContactData: entity.ContactData{
			Email:   "mahasiswa@univ.ac.id",
			Kelamin: "P",
			Agama:   "2",
			NoWA:    "081234567890",
		},
		EducationData: entity.EducationData{
			NamaAsalSekolah: "SMA Negeri 1",
			TahunLulus:      "2020",
		},
		AddressData: entity.AddressData{
			Provinsi:  "Jawa Barat",
			Kabupaten: "Bandung",
			KodePos:   "40111",
		},
		FatherData: entity.FatherData{
			NamaAyah: "Bapak Tes",
		},
		MotherData: entity.MotherData{
			NamaIbu: "Ibu Tes",
		},
	}

	sessions := &mockSessionManager{}
	scraper := &mockStudentProfileScraper{
		scrapeFunc: func(session port.BrowserSession, lmsBaseURL string) (*entity.StudentProfile, error) {
			return expectedProfile, nil
		},
	}
	cache := newMockCache()

	svc := newTestService(sessions, scraper, cache)

	_, err := svc.Scrape(testReq)
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}

	// Verify cached data matches scraped profile
	data, err := cache.Get(testReq.NPM, "profile", "student_profile.json")
	if err != nil {
		t.Fatalf("cache.Get() error = %v", err)
	}

	var cached entity.StudentProfile
	if err := json.Unmarshal(data, &cached); err != nil {
		t.Fatalf("cached data unmarshal error = %v", err)
	}

	if cached.PersonalData.NIM != expectedProfile.PersonalData.NIM {
		t.Errorf("cached NIM = %q, want %q", cached.PersonalData.NIM, expectedProfile.PersonalData.NIM)
	}
	if cached.FatherData.NamaAyah != expectedProfile.FatherData.NamaAyah {
		t.Errorf("cached NamaAyah = %q, want %q", cached.FatherData.NamaAyah, expectedProfile.FatherData.NamaAyah)
	}
	if cached.ContactData.NoWA != expectedProfile.ContactData.NoWA {
		t.Errorf("cached NoWA = %q, want %q", cached.ContactData.NoWA, expectedProfile.ContactData.NoWA)
	}
}

// ---------------------------------------------------------------------------
// Tests: Get
// ---------------------------------------------------------------------------

func TestService_Get_Success(t *testing.T) {
	sessions := &mockSessionManager{}
	scraper := &mockStudentProfileScraper{}
	cache := newMockCache()

	svc := newTestService(sessions, scraper, cache)

	// First Scrape to populate cache
	_, err := svc.Scrape(testReq)
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}

	// Now Get should return the cached profile
	profile, err := svc.Get(testReq)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if profile == nil {
		t.Fatal("Get() returned nil profile")
	}
	if profile.PersonalData.NIM != testReq.NPM {
		t.Errorf("NIM = %q, want %q", profile.PersonalData.NIM, testReq.NPM)
	}
	if profile.PersonalData.NamaMahasiswa != "Test Student" {
		t.Errorf("NamaMahasiswa = %q, want %q", profile.PersonalData.NamaMahasiswa, "Test Student")
	}
}

func TestGet_CacheMiss(t *testing.T) {
	sessions := &mockSessionManager{}
	scraper := &mockStudentProfileScraper{}
	cache := newMockCache()

	svc := newTestService(sessions, scraper, cache)

	// Get without prior Scrape — cache is empty
	profile, err := svc.Get(testReq)
	if err == nil {
		t.Fatal("Get() should return error when cache is empty")
	}
	if profile != nil {
		t.Error("profile should be nil on cache miss")
	}
}

func TestGet_CacheExistsButGetFails(t *testing.T) {
	sessions := &mockSessionManager{}
	scraper := &mockStudentProfileScraper{}
	cache := &mockExtractionCache{
		existsFunc: func(npm, docType, filename string) bool {
			return true // exists
		},
		getFunc: func(npm, docType, filename string) ([]byte, error) {
			return nil, fmt.Errorf("read error")
		},
	}

	svc := newTestService(sessions, scraper, cache)

	profile, err := svc.Get(testReq)
	if err == nil {
		t.Fatal("Get() should return error when cache read fails")
	}
	if profile != nil {
		t.Error("profile should be nil on error")
	}
}

func TestGet_CacheContainsInvalidJSON(t *testing.T) {
	sessions := &mockSessionManager{}
	scraper := &mockStudentProfileScraper{}
	cache := &mockExtractionCache{
		existsFunc: func(npm, docType, filename string) bool {
			return true
		},
		getFunc: func(npm, docType, filename string) ([]byte, error) {
			return []byte("not valid json {{{"), nil
		},
	}

	svc := newTestService(sessions, scraper, cache)

	profile, err := svc.Get(testReq)
	if err == nil {
		t.Fatal("Get() should return error when cached data is invalid JSON")
	}
	if profile != nil {
		t.Error("profile should be nil on unmarshal error")
	}
}

func TestGet_CacheHit_FullProfile(t *testing.T) {
	sessions := &mockSessionManager{}
	scraper := &mockStudentProfileScraper{}
	cache := newMockCache()

	svc := newTestService(sessions, scraper, cache)

	// Scrape to populate
	_, err := svc.Scrape(testReq)
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}

	// Get and verify all sub-structs survive the round-trip
	profile, err := svc.Get(testReq)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if profile.ContactData.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", profile.ContactData.Email, "test@example.com")
	}
	if profile.ContactData.Kelamin != "L" {
		t.Errorf("Kelamin = %q, want %q", profile.ContactData.Kelamin, "L")
	}
	if profile.PersonalData.ProgramStudi != "TI" {
		t.Errorf("ProgramStudi = %q, want %q", profile.PersonalData.ProgramStudi, "TI")
	}
	if profile.PersonalData.Semester != "5" {
		t.Errorf("Semester = %q, want %q", profile.PersonalData.Semester, "5")
	}
	if profile.PersonalData.Kelas != "A" {
		t.Errorf("Kelas = %q, want %q", profile.PersonalData.Kelas, "A")
	}
}

// ---------------------------------------------------------------------------
// Tests: Scrape → Get round-trip
// ---------------------------------------------------------------------------

func TestScrapeGetRoundTrip(t *testing.T) {
	sessions := &mockSessionManager{}
	scraper := &mockStudentProfileScraper{}
	cache := newMockCache()

	svc := newTestService(sessions, scraper, cache)

	// Scrape
	scrapeResult, err := svc.Scrape(testReq)
	if err != nil {
		t.Fatalf("Scrape() error = %v", err)
	}
	if scrapeResult.NPM != testReq.NPM {
		t.Errorf("Scrape result NPM = %q, want %q", scrapeResult.NPM, testReq.NPM)
	}

	// Get — must return identical data
	profile, err := svc.Get(testReq)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Verify NPM round-trips
	if profile.PersonalData.NIM != testReq.NPM {
		t.Errorf("Get NIM = %q, want %q", profile.PersonalData.NIM, testReq.NPM)
	}

	// Re-scrape overwrites — simulate different data
	updatedScraper := &mockStudentProfileScraper{
		scrapeFunc: func(session port.BrowserSession, lmsBaseURL string) (*entity.StudentProfile, error) {
			return &entity.StudentProfile{
				PersonalData: entity.PersonalData{
					NIM:           testReq.NPM,
					NamaMahasiswa: "Updated Student",
				},
			}, nil
		},
	}
	svc2 := newTestService(sessions, updatedScraper, cache)

	_, err = svc2.Scrape(testReq)
	if err != nil {
		t.Fatalf("2nd Scrape() error = %v", err)
	}

	profile2, err := svc2.Get(testReq)
	if err != nil {
		t.Fatalf("2nd Get() error = %v", err)
	}
	if profile2.PersonalData.NamaMahasiswa != "Updated Student" {
		t.Errorf("after re-scrape, NamaMahasiswa = %q, want %q", profile2.PersonalData.NamaMahasiswa, "Updated Student")
	}
}
