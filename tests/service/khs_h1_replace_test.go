package service_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
)

type h1Session struct{ calls int }

func (m *h1Session) Navigate(url string) error {
	m.calls++
	if m.calls == 1 {
		return fmt.Errorf("navigate to %s failed after 3 attempts: navigate to %s (attempt 3): context deadline exceeded", url, url)
	}
	return nil
}
func (m *h1Session) Eval(js string) (string, error)                    { return "{}", nil }
func (m *h1Session) ElementAttribute(sel, attr string) (string, error) { return "v", nil }
func (m *h1Session) ElementExists(sel string) (bool, error)            { return true, nil }
func (m *h1Session) ElementHref(sel string) (string, error)            { return "cetak/khs_pdf.php?a=1", nil }
func (m *h1Session) DownloadPDF(url, save string) (string, int, error) { return "f.pdf", 10, nil }
func (m *h1Session) DownloadImage(url, save string) (string, int, error) {
	return "", 0, nil
}
func (m *h1Session) Close() error { return nil }

type h1Manager struct{ sess port.BrowserSession }

func (m *h1Manager) GetOrCreate(npm, password string) (port.BrowserSession, error) {
	return m.sess, nil
}
func (m *h1Manager) Close(npm string) error { return nil }
func (m *h1Manager) CloseAll()              {}

func TestH1_NavigateTimeoutMapsTo503Not401(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{DownloadDir: t.TempDir(), LMSBaseURL: "https://elearning.universitasmandiri.ac.id"}}
	sess := &h1Session{}
	mgr := &h1Manager{sess: sess}
	svc := service.NewLMSDocumentService(cfg, mgr)
	_, err := svc.DownloadKHS(entity.KHSDownloadRequest{NPM: "2211700006", Password: "Izzan027", TahunAjaran: "2023/2024", Semester: "GENAP"})
	if err == nil {
		t.Fatal("expected error from H1 timeout")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") {
		t.Fatalf("error must preserve deadline cause, got %q", err.Error())
	}
	if !apperror.IsInfrastructureError(err) {
		t.Fatalf("must be infrastructure (503), got %q", err.Error())
	}
	if apperror.IsCredentialError(err) {
		t.Fatal("must not be credential (401)")
	}
	appErr := apperror.ClassifyDocumentError(err, "KHS download failed")
	if appErr.StatusCode != 503 {
		t.Fatalf("ClassifyDocumentError must be 503, got %d", appErr.StatusCode)
	}
}

func TestH1_SessionHasReplacePageAndDiagTags(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	root := dir
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Skip("cannot locate go.mod")
			return
		}
		root = parent
	}
	data, err := os.ReadFile(filepath.Join(root, "internal/infrastructure/session/session.go"))
	if err != nil {
		t.Fatalf("cannot read session.go: %v", err)
	}
	s := string(data)
	for _, needle := range []string{"replacePage", "[DIAG-NAV]", "[DIAG-PAGE]", "isTimeout"} {
		if !strings.Contains(s, needle) {
			t.Errorf("session.go missing %q — H1 fix or DIAG instrumentation absent", needle)
		}
	}
	if !strings.Contains(s, "timeout on prior attempt, replacing page") {
		t.Error("session.go must log page replacement on timeout retry (H1)")
	}
}
