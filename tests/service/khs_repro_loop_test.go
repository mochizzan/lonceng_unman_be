package service_test

import (
	"fmt"
	"strings"
	"testing"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
)

// reproSession wraps a controllable mock to simulate the paste-1.md bug:
//   - First N Navigate calls succeed.
//   - Next Navigate returns context deadline exceeded (H1/H2 path).
//   - Browser.Page("about:blank") then also hangs, visible as newSession failure
//     classified as infrastructure → 503.
type reproSession struct {
	navigateCalls int
	failAt        int // 1-indexed call number where Navigate starts failing
	navigateErr   error
	downloadErr   error
	closed        int
}

func (m *reproSession) Navigate(url string) error {
	m.navigateCalls++
	if m.failAt > 0 && m.navigateCalls >= m.failAt {
		if m.navigateErr != nil {
			return m.navigateErr
		}
		return fmt.Errorf("navigate to %s failed after 3 attempts: navigate to %s (attempt 3): context deadline exceeded", url, url)
	}
	return nil
}
func (m *reproSession) Eval(js string) (string, error) { return "{}", nil }
func (m *reproSession) ElementAttribute(sel, attr string) (string, error) {
	return "dummy.pdf", nil
}
func (m *reproSession) ElementExists(sel string) (bool, error) { return true, nil }
func (m *reproSession) ElementHref(sel string) (string, error) {
	return "cetak/khs_pdf.php?tahun_ajaran=2023/2024&semester=GANJIL", nil
}

func (m *reproSession) DownloadPDF(url, save string) (string, int, error) {
	if m.downloadErr != nil {
		return "", 0, m.downloadErr
	}
	return "2023_2024_GANJIL.pdf", 1234, nil
}

func (m *reproSession) DownloadImage(url, save string) (string, int, error) {
	return "", 0, nil
}
func (m *reproSession) Close() error { m.closed++; return nil }

type reproManager struct {
	call      int
	session   *reproSession
	failAfter int // after this many GetOrCreate calls, return Page() style 503
}

func (m *reproManager) GetOrCreate(npm, password string) (port.BrowserSession, error) {
	m.call++
	if m.failAfter > 0 && m.call > m.failAfter {
		return nil, fmt.Errorf("create new page: open page about:blank after 3 attempts: context deadline exceeded")
	}
	return m.session, nil
}
func (m *reproManager) Close(npm string) error { return nil }
func (m *reproManager) CloseAll()              {}
func (m *reproManager) MarkStale(string) bool  { return false }
func (m *reproManager) Invalidate(string) bool { return false }

// TestLoop_KHSRepro_TightSignal is the tight red-capable loop for paste-1.md:
// DOWNLOAD_DETAIL → 503 infrastructure → session corrupted → next Page() 503.
// It drives the real bug code path (lmsService.DownloadKHS via port.SessionManager)
// and asserts the exact user symptom, not just "didn't crash".
func TestLoop_KHSRepro_TightSignal(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{DownloadDir: t.TempDir(), LMSBaseURL: "https://elearning.universitasmandiri.ac.id"}}

	t.Run("happy path returns 200 (not 401)", func(t *testing.T) {
		sess := &reproSession{failAt: 0}
		mgr := &reproManager{session: sess}
		svc := service.NewLMSDocumentService(cfg, mgr)
		_, err := svc.DownloadKHS(entity.KHSDownloadRequest{NPM: "2211700006", Password: "any", TahunAjaran: "2022/2023", Semester: "GANJIL"})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})

	t.Run("navigate deadline → 503 not 401 (paste-1 symptom)", func(t *testing.T) {
		sess := &reproSession{failAt: 1, navigateErr: fmt.Errorf("navigate to https://elearning.universitasmandiri.ac.id/admin/main.php?op=mahasiswa_khs&act=cetak_detail&semester=GANJIL&tahun_ajaran=2023%%2F2024 failed after 3 attempts: navigate to https://elearning.universitasmandiri.ac.id/admin/main.php?op=mahasiswa_khs&act=cetak_detail&semester=GANJIL&tahun_ajaran=2023%%2F2024 (attempt 3): context deadline exceeded")}
		mgr := &reproManager{session: sess}
		svc := service.NewLMSDocumentService(cfg, mgr)
		_, err := svc.DownloadKHS(entity.KHSDownloadRequest{NPM: "2211700006", Password: "Izzan027", TahunAjaran: "2023/2024", Semester: "GANJIL"})
		if err == nil {
			t.Fatal("expected error (paste-1: 503 on GENAP 2023/2024)")
		}
		// Must be classified as infrastructure → 503, NOT 401.
		if apperror.IsCredentialError(err) {
			t.Fatalf("must NOT be credential error (would map to 401), got %v", err)
		}
		if !apperror.IsInfrastructureError(err) {
			t.Fatalf("must be infrastructure error (maps to 503), got %q", err.Error())
		}
		appErr := apperror.ClassifyDocumentError(err, "KHS download failed")
		if appErr.StatusCode != 503 {
			t.Fatalf("ClassifyDocumentError must give 503, got %d (%q)", appErr.StatusCode, appErr.PublicMsg)
		}
		if !strings.Contains(strings.ToLower(appErr.Internal.Error()), "context deadline exceeded") {
			t.Errorf("internal should preserve deadline cause, got %q", appErr.Internal.Error())
		}
	})

	t.Run("create new page deadline after navigate failure → 503 (session corruption chain)", func(t *testing.T) {
		// First call: Navigate hangs. Second call: Page("about:blank") hangs.
		sess := &reproSession{failAt: 1}
		mgr := &reproManager{session: sess, failAfter: 1}
		svc := service.NewLMSDocumentService(cfg, mgr)
		_, err := svc.DownloadKHS(entity.KHSDownloadRequest{NPM: "2211700006", Password: "Izzan027", TahunAjaran: "2023/2024", Semester: "GENAP"})
		if err == nil || !apperror.IsInfrastructureError(err) {
			t.Fatalf("first failure must be infrastructure, got %v", err)
		}
		// Next request would call GetOrCreate again and hit Page() failure.
		_, err2 := mgr.GetOrCreate("2211700006", "Izzan027")
		if err2 == nil || !strings.Contains(strings.ToLower(err2.Error()), "context deadline exceeded") {
			t.Fatalf("Page() hang must surface as deadline, got %v", err2)
		}
		if !apperror.IsInfrastructureError(err2) {
			t.Fatalf("Page hang must be infrastructure (503), got %v", err2)
		}
	})

	t.Run("deterministic and fast (<1s)", func(t *testing.T) {
		for range 10 {
			sess := &reproSession{failAt: 1}
			mgr := &reproManager{session: sess}
			svc := service.NewLMSDocumentService(cfg, mgr)
			_, err := svc.DownloadKHS(entity.KHSDownloadRequest{NPM: "2211700006", Password: "x", TahunAjaran: "2023/2024", Semester: "GENAP"})
			if err == nil {
				t.Fatal("must fail deterministically each run")
			}
		}
	})
}
