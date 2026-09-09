package service_test

import (
	"fmt"
	"testing"

	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
)

// trackingManager wraps mock behavior and tracks MarkStale calls.
type trackingManager struct {
	session        port.BrowserSession
	markStaleCount int
	markStaleNPM   string
	getOrCreateErr error
}

func (m *trackingManager) GetOrCreate(npm, password string) (port.BrowserSession, error) {
	if m.getOrCreateErr != nil {
		return nil, m.getOrCreateErr
	}
	return m.session, nil
}
func (m *trackingManager) Close(npm string) error { return nil }
func (m *trackingManager) CloseAll()              {}
func (m *trackingManager) MarkStale(npm string) bool {
	m.markStaleCount++
	m.markStaleNPM = npm
	return true
}
func (m *trackingManager) Invalidate(npm string) bool { return m.MarkStale(npm) }

type timeoutSession struct {
	navErr error
}

func (s *timeoutSession) Navigate(url string) error { return s.navErr }
func (s *timeoutSession) Eval(js string) (string, error) {
	return "", fmt.Errorf("context deadline exceeded")
}

func (s *timeoutSession) ElementAttribute(sel, attr string) (string, error) {
	return "", fmt.Errorf("context deadline exceeded")
}
func (s *timeoutSession) ElementExists(sel string) (bool, error) { return false, nil }
func (s *timeoutSession) ElementHref(sel string) (string, error) {
	return "", fmt.Errorf("write tcp 127.0.0.1:1->127.0.0.1:2: use of closed network connection")
}

func (s *timeoutSession) DownloadPDF(url, save string) (string, int, error) {
	return "", 0, fmt.Errorf("closed network")
}

func (s *timeoutSession) DownloadImage(url, save string) (string, int, error) {
	return "", 0, fmt.Errorf("EOF")
}
func (s *timeoutSession) Close() error { return nil }

func TestAlwaysFreshOnError_NavigateTimeout(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{
		LMSBaseURL:  "https://elearning.universitasmandiri.ac.id",
		DownloadDir: t.TempDir(),
	}}
	// DownloadKRS path: GetOrCreate succeeds, Navigate returns timeout → MarkStale should be called.
	mgr := &trackingManager{
		session: &timeoutSession{navErr: fmt.Errorf("navigate to https://example.com failed after 3 attempts: context deadline exceeded")},
	}
	svc := service.NewLMSDocumentService(cfg, mgr)
	_, err := svc.DownloadKRS(entity.KRSDownloadRequest{NPM: "2211700006", Password: "secret"})
	if err == nil {
		t.Fatalf("expected error from timeout session")
	}
	if mgr.markStaleCount == 0 {
		t.Fatalf("expected MarkStale to be called on IsTransient Navigate timeout, got 0")
	}
	if mgr.markStaleNPM != "2211700006" {
		t.Fatalf("MarkStale npm = %q, want %q", mgr.markStaleNPM, "2211700006")
	}

	// GetKHSSemesters Eval timeout → MarkStale
	mgr2 := &trackingManager{
		session: &timeoutSession{navErr: nil}, // Navigate ok, Eval will timeout internally via timeoutSession.Eval
	}
	svc2 := service.NewLMSDocumentService(cfg, mgr2)
	_, err = svc2.GetKHSSemesters(entity.KHSSemestersRequest{NPM: "2211700006", Password: "secret"})
	if err == nil {
		t.Fatalf("expected error from Eval timeout")
	}
	if mgr2.markStaleCount == 0 {
		t.Fatalf("expected MarkStale on Eval IsTransient, got 0")
	}

	// 404-style error must NOT trigger MarkStale (opsi A strict)
	mgr3 := &trackingManager{
		session: &timeoutSession{navErr: fmt.Errorf("element not found: pdf not found 404")},
	}
	svc3 := service.NewLMSDocumentService(cfg, mgr3)
	_, err = svc3.DownloadKRS(entity.KRSDownloadRequest{NPM: "2211700006", Password: "secret"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if mgr3.markStaleCount != 0 {
		t.Fatalf("404 must NOT MarkStale (opsi A strict), got %d", mgr3.markStaleCount)
	}
}

func TestMarkStale_ServiceIntegration(t *testing.T) {
	// Additional guard: ErrLMSExpired also triggers MarkStale (ported from manager via session)
	cfg := &config.Config{App: config.AppConfig{
		LMSBaseURL:  "https://elearning.universitasmandiri.ac.id",
		DownloadDir: t.TempDir(),
	}}
	mgr := &trackingManager{
		session: &timeoutSession{navErr: fmt.Errorf("%w: lms expired", port.ErrLMSExpired)},
	}
	svc := service.NewLMSDocumentService(cfg, mgr)
	// DownloadKHS will hit navigateWithLMSRetry which does MarkStale+retry once; we simulate re-login failure to still verify MarkStale was hit.
	// Instead directly test DownloadKRS ElementAttribute ErrLMSExpired path
	mgr.session = &struct {
		timeoutSession
	}{
		timeoutSession: timeoutSession{navErr: nil},
	}
	// Override ElementAttribute to return ErrLMSExpired
	type expiredAttrSession struct{ timeoutSession }
	// Use inline session that returns ErrLMSExpired on ElementAttribute
	sess := &expiredSession{}
	mgr.session = sess
	mgr.markStaleCount = 0
	_, _ = svc.DownloadKRS(entity.KRSDownloadRequest{NPM: "2211700006", Password: "secret"})
	if mgr.markStaleCount == 0 {
		t.Fatalf("expected MarkStale on ErrLMSExpired ElementAttribute")
	}
}

type expiredSession struct{}

func (s *expiredSession) Navigate(url string) error      { return nil }
func (s *expiredSession) Eval(js string) (string, error) { return "[]", nil }
func (s *expiredSession) ElementAttribute(sel, attr string) (string, error) {
	return "", port.ErrLMSExpired
}
func (s *expiredSession) ElementExists(sel string) (bool, error)            { return false, nil }
func (s *expiredSession) ElementHref(sel string) (string, error)            { return "", nil }
func (s *expiredSession) DownloadPDF(url, save string) (string, int, error) { return "f.pdf", 1, nil }

func (s *expiredSession) DownloadImage(url, save string) (string, int, error) {
	return "", 0, nil
}
func (s *expiredSession) Close() error { return nil }
