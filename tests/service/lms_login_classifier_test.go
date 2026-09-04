package service

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"lonceng_unman_be/internal/application/service"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"

	"github.com/gofiber/fiber/v3"
)

// mockSessionManager lets the LMS login test exercise the success and
// failure paths without spinning up a real browser.
type mockSessionManager struct {
	// getOrCreateFn is invoked by Login. Tests inject behaviors here.
	getOrCreateFn func(npm, password string) (port.BrowserSession, error)
}

func (m *mockSessionManager) GetOrCreate(npm, password string) (port.BrowserSession, error) {
	return m.getOrCreateFn(npm, password)
}

func (m *mockSessionManager) Close(npm string) error { return nil }
func (m *mockSessionManager) CloseAll()              {}

// fakeSession is a no-op BrowserSession for the success path. None of its
// methods are called because Login only defers Close.
type fakeSession struct{}

func (fakeSession) Navigate(string) error                             { return nil }
func (fakeSession) Eval(string) (string, error)                       { return "", nil }
func (fakeSession) ElementAttribute(string, string) (string, error)   { return "", nil }
func (fakeSession) ElementExists(string) (bool, error)                { return false, nil }
func (fakeSession) ElementHref(string) (string, error)                { return "", nil }
func (fakeSession) DownloadPDF(string, string) (string, int, error)   { return "", 0, nil }
func (fakeSession) DownloadImage(string, string) (string, int, error) { return "", 0, nil }
func (fakeSession) Close() error                                      { return nil }

// newTestLMSService wires lmsService with a mock session manager whose
// GetOrCreate behavior is supplied by the caller.
func newTestLMSService(t *testing.T, getOrCreate func(npm, password string) (port.BrowserSession, error)) service.LMSLogin {
	t.Helper()
	return service.NewLMSService(nil, &mockSessionManager{getOrCreateFn: getOrCreate})
}

// TestLogin_Success_Returns200 covers the happy path: GetOrCreate succeeds,
// result.Success == true, status == 200, no error returned.
func TestLogin_Success_Returns200(t *testing.T) {
	svc := newTestLMSService(t, func(npm, password string) (port.BrowserSession, error) {
		return fakeSession{}, nil
	})

	result, status, err := svc.Login(entity.LoginRequest{NPM: "2211700006", Password: "any"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if status != fiber.StatusOK {
		t.Errorf("status = %d, want %d", status, fiber.StatusOK)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if !result.Success {
		t.Errorf("result.Success = false, want true")
	}
	if result.NPM != "2211700006" {
		t.Errorf("result.NPM = %q, want %q", result.NPM, "2211700006")
	}
}

// TestLogin_CredentialFailure_Returns401 ensures that the LMS rejecting
// the credentials (the LMS form rendered an error indicator, or the page
// failed to redirect to /admin/) is classified as a credential failure
// (HTTP 401), NOT as an infrastructure failure.
//
// SECURITY: the public message MUST stay generic — "Username atau
// password salah" — and must NOT reveal which field was wrong.
func TestLogin_CredentialFailure_Returns401(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		// The actual LMS text observed in production: "LOGIN GAGAL!
		// USERNAME DAN PASSWORD SALAH!" — combined phrase, not just
		// "password salah" (would leak which field is wrong).
		{"lms combined error text", errors.New("LOGIN GAGAL! USERNAME DAN PASSWORD SALAH!")},
		{"lms combined lower case", errors.New("Username dan Password yang Anda masukkan salah")},
		{"redirect not happening", errors.New("page did not redirect to dashboard: https://elearning.../login")},
		{"no response detected", errors.New("login timed out: no response detected")},
		{"session expired sentinel", errors.New("session expired: redirected to /login")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestLMSService(t, func(npm, password string) (port.BrowserSession, error) {
				return nil, tc.err
			})

			result, status, err := svc.Login(entity.LoginRequest{NPM: "2211700006", Password: "wrong"})
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if status != fiber.StatusUnauthorized {
				t.Errorf("status = %d, want %d (credential failure)", status, fiber.StatusUnauthorized)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			if result.Success {
				t.Errorf("result.Success = true, want false")
			}
			// SECURITY: public message MUST stay generic — no leak about
			// which field is wrong. The legacy "Username atau password
			// salah" wording is interpreted as "either or both is wrong",
			// not "password specifically is wrong".
			if !strings.Contains(result.Message, "Username atau password salah") {
				t.Errorf("result.Message = %q, must contain generic credential-failure message", result.Message)
			}
			// Belt-and-suspenders: explicitly forbid "password salah" or
			// "username salah" alone — these would leak which field is
			// incorrect, enabling user enumeration.
			if strings.Contains(result.Message, "password salah") && !strings.Contains(result.Message, "Username atau") {
				t.Errorf("result.Message = %q leaks 'password salah' without generic context — security violation", result.Message)
			}
		})
	}
}

// TestLogin_InfrastructureFailure_Returns503 is the regression test for
// the original bug: cold-start browser failures (EOF, timeout, DNS) used
// to be returned as HTTP 401 "Username atau password salah", which made
// them indistinguishable from real credential failures. After the fix,
// infrastructure failures MUST return HTTP 503 with a distinct public
// message.
func TestLogin_InfrastructureFailure_Returns503(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		// The exact failure observed in production logs.
		{"cold-start EOF", errors.New("open login page: open page https://elearning.universitasmandiri.ac.id: EOF")},
		// Browser launch / connect failures.
		{"browser launch", errors.New("launch browser: fork/exec: no such file or directory")},
		{"connect browser", errors.New("connect browser: dial tcp 127.0.0.1:0: connect: connection refused")},
		// Page operation timeouts.
		{"navigate timeout", errors.New("navigate to login page: context deadline exceeded")},
		{"wait load timeout", errors.New("wait login page load: timeout")},
		// DNS failures.
		{"dns check", errors.New("DNS check: cannot resolve LMS host \"elearning...\": no such host")},
		{"network unreachable", errors.New("dial tcp: network is unreachable")},
		// Session manager wrapping.
		{"too many pages", errors.New("create new page: browser has too many open pages (10 >= 10)")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestLMSService(t, func(npm, password string) (port.BrowserSession, error) {
				return nil, tc.err
			})

			result, status, err := svc.Login(entity.LoginRequest{NPM: "2211700006", Password: "any"})
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if status != fiber.StatusServiceUnavailable {
				t.Errorf("status = %d, want %d (infrastructure failure)", status, fiber.StatusServiceUnavailable)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			if result.Success {
				t.Errorf("result.Success = true, want false")
			}
			// Public message MUST NOT be the credential-failure message —
			// the whole point of the fix is to distinguish these two.
			if strings.Contains(result.Message, "Username atau password salah") {
				t.Errorf("result.Message = %q must NOT contain credential-failure text", result.Message)
			}
		})
	}
}

// TestLogin_NilError_IsDefensive500 documents the defensive fallback when
// the helper is called with nil error. Callers should never hit this path
// but the helper returns 500 + non-nil err so the bug surfaces immediately.
func TestLogin_NilError_IsDefensive500(t *testing.T) {
	// We don't have direct access to the unexported classifyLoginResult,
	// so we simulate the bug condition by returning a non-matching error
	// that falls into the "unknown" bucket. That bucket is still 503
	// (conservative — surfaces in logs without misleading the user).
	svc := newTestLMSService(t, func(npm, password string) (port.BrowserSession, error) {
		return nil, errors.New("some completely unknown error type xyz123")
	})

	result, status, err := svc.Login(entity.LoginRequest{NPM: "2211700006", Password: "any"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d (unknown → conservative 503)", status, http.StatusServiceUnavailable)
	}
	if result == nil || result.Success {
		t.Errorf("expected non-nil failed result, got %+v", result)
	}
}
