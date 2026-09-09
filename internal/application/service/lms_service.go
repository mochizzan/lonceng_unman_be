package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lonceng_unman_be/internal/apperror"
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"

	"github.com/gofiber/fiber/v3"
)

// LMSLogin defines the contract for LMS login operations.
type LMSLogin interface {
	// Login validates credentials by attempting a session creation.
	// Returns three values:
	//   - result: business outcome (success or failure) — never nil.
	//   - httpStatus: the recommended HTTP status code for this outcome.
	//     200 for success, 401 for credential failure (verified by LMS),
	//     503 for infrastructure failure (browser/CDP/DNS/network).
	//   - err: non-nil only for unexpected programmer errors (panic recovery).
	//     Infrastructure and credential failures are normal outcomes and
	//     must be communicated through (result, httpStatus), NOT through err.
	Login(req entity.LoginRequest) (result *entity.LoginResult, httpStatus int, err error)
}

// lmsService implements LMSLogin.
type lmsService struct {
	cfg      *config.Config
	sessions port.SessionManager
}

// NewLMSService creates an LMSLogin with session management.
func NewLMSService(cfg *config.Config, sessions port.SessionManager) LMSLogin {
	return &lmsService{cfg: cfg, sessions: sessions}
}

// LMSDocumentService defines the contract for LMS document download operations.
type LMSDocumentService interface {
	// DownloadKRS downloads the KRS PDF for the given student.
	DownloadKRS(req entity.KRSDownloadRequest) (*entity.KRSDownloadResult, error)

	// GetKHSSemesters returns the list of available KHS semesters.
	GetKHSSemesters(req entity.KHSSemestersRequest) (*entity.KHSSemestersResult, error)

	// DownloadKHS downloads the KHS PDF for a specific semester.
	DownloadKHS(req entity.KHSDownloadRequest) (*entity.KHSDownloadResult, error)

	// DownloadKHSFile serves an already-downloaded KHS PDF file as binary data.
	DownloadKHSFile(req entity.KHSDownloadRequest) (string, int64, error)
}

// lmsDocumentService implements LMSDocumentService.
type lmsDocumentService struct {
	cfg      *config.Config
	sessions port.SessionManager
	npmLocks sync.Map // map[string]*sync.Mutex, per-NPM phase-limited lock (distinct from Manager.npmLocks)
}

func (s *lmsDocumentService) getNPMLock(npm string) *sync.Mutex {
	v, _ := s.npmLocks.LoadOrStore(npm, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// NewLMSDocumentService creates an LMSDocumentService with session management.
func NewLMSDocumentService(cfg *config.Config, sessions port.SessionManager) LMSDocumentService {
	return &lmsDocumentService{cfg: cfg, sessions: sessions}
}

// Login validates credentials by creating a session. The session is cached
// for subsequent requests. Returns a business outcome and the HTTP status
// the handler should use.
//
// Error classification (drives the returned httpStatus):
//   - nil err     → success, httpStatus = 200
//   - credential failure (LMS rejected the credentials) → httpStatus = 401
//   - infrastructure failure (browser/CDP/DNS/network) → httpStatus = 503
//   - non-nil err → unexpected programmer error, httpStatus = 500
func (s *lmsService) Login(req entity.LoginRequest) (*entity.LoginResult, int, error) {
	session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
	if err != nil {
		slog.Warn("login failed", "npm", req.NPM, "error", err)
		return classifyLoginResult(req.NPM, err)
	}
	defer func() { _ = session.Close() }()

	slog.Info("login successful", "npm", req.NPM)
	return &entity.LoginResult{
		Success:   true,
		Message:   "Login successful",
		NPM:       req.NPM,
		Timestamp: time.Now(),
	}, fiber.StatusOK, nil
}

// classifyLoginResult converts a session-creation error into a user-facing
// outcome + HTTP status. It distinguishes infrastructure failures (which
// the user should retry) from credential failures (which they should fix).
//
// The previous implementation conflated both cases into "Username atau
// password salah" (HTTP 401), which made CDN/browser cold-start issues
// look identical to bad credentials — debugging nightmare.
//
// Detection rules (in priority order):
//  1. err is nil → unexpected; caller should not invoke this helper.
//  2. err originated from the LMS form submission (login page rendered the
//     error indicator, or login did not redirect to /admin/) → credential
//     failure (401). These errors come from Manager.login() and carry
//     plain-text messages from the LMS, not wrapped Go errors.
//  3. err originated from browser launch / page open / DNS / network /
//     CDP → infrastructure failure (503).
//  4. anything else → unknown infrastructure failure (503, conservative).
func classifyLoginResult(npm string, err error) (*entity.LoginResult, int, error) {
	if err == nil {
		// Defensive: should not happen. Surface as 500 so we notice.
		return &entity.LoginResult{
			Success:   false,
			Message:   "internal error: classifyLoginResult called with nil error",
			NPM:       npm,
			Timestamp: time.Now(),
		}, fiber.StatusInternalServerError, fmt.Errorf("classifyLoginResult: nil error")
	}

	msg := err.Error()
	kind := classifyLoginErrorKind(msg)

	switch kind {
	case loginErrorKindCredential:
		// SECURITY: do not reveal which credential field was wrong.
		// "Username atau password salah" means "either username or password
		// (or both) is incorrect" — the user must verify both. This matches
		// the security best practice of no user enumeration / no credential
		// field disclosure.
		return &entity.LoginResult{
			Success:   false,
			Message:   "Username atau password salah",
			NPM:       npm,
			Timestamp: time.Now(),
		}, fiber.StatusUnauthorized, nil
	case loginErrorKindInfrastructure:
		slog.Warn("login infrastructure failure", "npm", npm, "category", kind, "cause", err)
		return &entity.LoginResult{
			Success:   false,
			Message:   "Layanan LMS tidak dapat diakses saat ini. Silakan coba lagi dalam beberapa saat.",
			NPM:       npm,
			Timestamp: time.Now(),
		}, fiber.StatusServiceUnavailable, nil
	default:
		// Unknown — treat as infrastructure failure to surface the real cause
		// in logs without misleading the user with "wrong password".
		slog.Warn("login unknown failure", "npm", npm, "category", kind, "cause", err)
		return &entity.LoginResult{
			Success:   false,
			Message:   "Login gagal karena kesalahan sistem. Silakan coba lagi.",
			NPM:       npm,
			Timestamp: time.Now(),
		}, fiber.StatusServiceUnavailable, nil
	}
}

// loginErrorKind classifies the source of a login failure.
type loginErrorKind int

const (
	loginErrorKindUnknown        loginErrorKind = iota
	loginErrorKindCredential                    // LMS rejected the credentials
	loginErrorKindInfrastructure                // browser / network / CDP / DNS
)

// loginErrorKeyword maps lowercase substrings to error categories.
// Order: credential first (specific LMS messages), then infrastructure
// patterns. "Unknown" is the fallback when no keyword matches.
var loginErrorKeyword = []struct {
	Substring string
	Kind      loginErrorKind
}{
	// Credential failure indicators — these come from Manager.login()
	// after the LMS rendered the error indicator or did not redirect
	// to /admin/. The messages are user-facing text from the LMS itself
	// (e.g. "LOGIN GAGAL! USERNAME DAN PASSWORD SALAH!") OR the
	// redirect-check sentinel from session/manager.go:587.
	//
	// SECURITY: match ONLY combined phrases ("username dan password",
	// "login gagal"), never "password salah" alone — the system must not
	// expose which credential field is wrong (user enumeration risk).
	// The public message stays generic: "Username atau password salah",
	// meaning "keduanya salah" (either or both), even when only one is
	// actually wrong. Server logs use the same generic phrasing.
	{"username dan password", loginErrorKindCredential},
	{"login gagal", loginErrorKindCredential},
	{"page did not redirect to dashboard", loginErrorKindCredential},
	{"login timed out: no response detected", loginErrorKindCredential},
	{"session expired", loginErrorKindCredential},

	// Infrastructure failure indicators — wrapped errors from
	// session/manager.go (browser connect, page open, DNS) and
	// browser/session.go (page ops).
	{"DNS check", loginErrorKindInfrastructure},
	{"cannot resolve LMS host", loginErrorKindInfrastructure},
	{"browser connect", loginErrorKindInfrastructure},
	{"launch browser", loginErrorKindInfrastructure},
	{"connect browser", loginErrorKindInfrastructure},
	{"open login page", loginErrorKindInfrastructure},
	{"open page", loginErrorKindInfrastructure},
	{"wait login page load", loginErrorKindInfrastructure},
	{"create new page", loginErrorKindInfrastructure},
	{"create profile dir", loginErrorKindInfrastructure},
	{"navigate to dashboard", loginErrorKindInfrastructure},
	{"wait dashboard load", loginErrorKindInfrastructure},
	{"too many open pages", loginErrorKindInfrastructure},
	{"context deadline exceeded", loginErrorKindInfrastructure},
	{"timeout", loginErrorKindInfrastructure},
	{"EOF", loginErrorKindInfrastructure},
	{"connection refused", loginErrorKindInfrastructure},
	{"no such host", loginErrorKindInfrastructure},
	{"network is unreachable", loginErrorKindInfrastructure},
}

// classifyLoginErrorKind returns the most specific category for err's
// message. Substrings are matched case-insensitively; first match wins.
func classifyLoginErrorKind(msg string) loginErrorKind {
	low := strings.ToLower(msg)
	for _, kw := range loginErrorKeyword {
		if strings.Contains(low, strings.ToLower(kw.Substring)) {
			return kw.Kind
		}
	}
	return loginErrorKindUnknown
}

// navigateWithLMSRetry wraps session.Navigate with always-fresh-on-error:
// - ErrLMSExpired (backend TTL vs LMS PHP GC) → MarkStale + evict + re-login once, then Navigate again.
// - IsTransientBrowserError (EOF/deadline/timeout/closed network/target closed/detached) → MarkStale + evict so next GetOrCreate is fresh (session 503 bug fix).
func (s *lmsDocumentService) navigateWithLMSRetry(sess port.BrowserSession, url, npm, password string) (port.BrowserSession, error) {
	if err := sess.Navigate(url); err != nil {
		if errors.Is(err, port.ErrLMSExpired) {
			slog.Warn("lms session expired during navigate — mark stale and re-login once", "npm", npm, "url", url, "error", err)
			s.sessions.MarkStale(npm)
			_ = sess.Close()
			fresh, gerr := s.sessions.GetOrCreate(npm, password)
			if gerr != nil {
				return fresh, fmt.Errorf("re-login after lms expiry: %w", gerr)
			}
			if err2 := fresh.Navigate(url); err2 != nil {
				return fresh, fmt.Errorf("navigate after re-login: %w", err2)
			}
			return fresh, nil
		}
		// Transient CDP/browser error (closed network/deadline/timeout/target closed/detached) → stale session per spec A; 404/401 stays cached.
		if isTransientBrowserError(err) {
			s.sessions.MarkStale(npm)
		}
		return sess, err
	}
	return sess, nil
}

// DownloadKRS downloads the KRS PDF for the given student.
// Flow: get session → navigate to KRS page → extract semester → download PDF.
// The browser phase is serialized per-NPM so concurrent downloads for the same
// NPM do not race on the shared Chrome --user-data-dir.
func (s *lmsDocumentService) DownloadKRS(req entity.KRSDownloadRequest) (*entity.KRSDownloadResult, error) {
	mu := s.getNPMLock(req.NPM)
	mu.Lock()
	defer mu.Unlock()

	session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	defer func() { _ = session.Close() }()

	// Navigate to KRS page — retry once if LMS PHP session expired (backend 24h vs LMS menit).
	krsPageURL := s.cfg.App.LMSBaseURL + port.KRSPagePath
	slog.Info("navigating to KRS page", "url", krsPageURL)

	if fresh, navErr := s.navigateWithLMSRetry(session, krsPageURL, req.NPM, req.Password); navErr != nil {
		return nil, fmt.Errorf("navigate to KRS page: %w", navErr)
	} else if fresh != session {
		session = fresh
	}

	// Extract semester number from the page.
	semesterNum, err := session.ElementAttribute(port.SelKRSSemesterInput, "value")
	if err != nil {
		if errors.Is(err, port.ErrLMSExpired) || isTransientBrowserError(err) {
			s.sessions.MarkStale(req.NPM)
		}
		return nil, fmt.Errorf("extract semester: %w", err)
	}
	slog.Info("KRS semester extracted", "npm", req.NPM, "semester", semesterNum)

	// Download KRS PDF — encode NPM as query param.
	q := url.Values{}
	q.Set("nis", strings.TrimSpace(req.NPM))
	krsURL := s.cfg.App.LMSBaseURL + port.KRSDownloadPath + "?" + q.Encode()
	slog.Info("downloading KRS", "url", krsURL)

	savePath := filepath.Join(s.cfg.App.DownloadDir, req.NPM, "krs", fmt.Sprintf("semester_%s.pdf", semesterNum))

	filename, size, err := session.DownloadPDF(krsURL, savePath)
	if err != nil {
		if isTransientBrowserError(err) {
			s.sessions.MarkStale(req.NPM)
		}
		return nil, fmt.Errorf("download KRS PDF: %w", err)
	}

	slog.Info("KRS download complete", "npm", req.NPM, "filename", filename, "size", size)

	return &entity.KRSDownloadResult{
		Success:   true,
		Message:   "KRS downloaded successfully",
		NPM:       req.NPM,
		FilePath:  savePath,
		Size:      size,
		Timestamp: time.Now(),
	}, nil
}

// GetKHSSemesters returns the list of available KHS semesters.
// The browser phase is serialized per-NPM; KHSListPath is static so no QueryEscape needed.
func (s *lmsDocumentService) GetKHSSemesters(req entity.KHSSemestersRequest) (*entity.KHSSemestersResult, error) {
	mu := s.getNPMLock(req.NPM)
	mu.Lock()
	defer mu.Unlock()

	session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	defer func() { _ = session.Close() }()

	// Navigate to KHS list page — retry once if LMS expired.
	khsListURL := s.cfg.App.LMSBaseURL + port.KHSListPath
	slog.Info("fetching KHS semesters", "url", khsListURL)

	if fresh, navErr := s.navigateWithLMSRetry(session, khsListURL, req.NPM, req.Password); navErr != nil {
		return nil, fmt.Errorf("navigate to KHS list: %w", navErr)
	} else if fresh != session {
		session = fresh
	}

	// Parse HTML to extract semesters via JavaScript.
	jsCode := `async function() {
		const rows = document.querySelectorAll('.table-bordered tbody tr');
		const semesters = [];
		rows.forEach(row => {
			const cells = row.querySelectorAll('td');
			if (cells.length >= 6) {
				const sksCell = cells[5].innerText;
				const match = sksCell.match(/(\d{4}\/\d{4})\s*-\s*(\w+)\s*-\s*(\d+)\s*SKS/g);
				if (match) {
					match.forEach(m => {
						const parts = m.split('-').map(s => s.trim());
						if (parts.length >= 3) {
							semesters.push({
								tahun_ajaran: parts[0],
								semester: parts[1],
								sks: parseInt(parts[2]) || 0
							});
						}
					});
				}
			}
		});
		return JSON.stringify(semesters);
	}`

	result, err := session.Eval(jsCode)
	if err != nil {
		if isTransientBrowserError(err) {
			s.sessions.MarkStale(req.NPM)
		}
		return nil, fmt.Errorf("parse KHS semesters: %w", err)
	}

	var semesters []entity.KHSSemester
	if err := json.Unmarshal([]byte(result), &semesters); err != nil {
		return nil, fmt.Errorf("unmarshal semesters: %w", err)
	}

	slog.Info("KHS semesters found", "npm", req.NPM, "count", len(semesters))

	return &entity.KHSSemestersResult{
		Success:   true,
		Message:   "KHS semesters retrieved",
		NPM:       req.NPM,
		Semesters: semesters,
		Timestamp: time.Now(),
	}, nil
}

// DownloadKHS downloads the KHS PDF for a specific semester.
// Validation is done before acquiring the per-NPM lock to avoid holding the lock for bad input.
func (s *lmsDocumentService) DownloadKHS(req entity.KHSDownloadRequest) (*entity.KHSDownloadResult, error) {
	// Normalize and validate before locking.
	req.Semester = strings.ToUpper(strings.TrimSpace(req.Semester))
	req.TahunAjaran = strings.TrimSpace(req.TahunAjaran)
	if !entity.ValidSemester(req.Semester) {
		return nil, fmt.Errorf("semester must be GANJIL or GENAP")
	}
	if req.TahunAjaran == "" {
		return nil, apperror.BadRequest("tahun_ajaran is required")
	}

	mu := s.getNPMLock(req.NPM)
	mu.Lock()
	defer mu.Unlock()

	session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	defer func() { _ = session.Close() }()

	// Build detail URL — retry once if LMS expired (backend 24h vs LMS menit).
	q := url.Values{}
	q.Set("tahun_ajaran", req.TahunAjaran)
	q.Set("semester", req.Semester)
	detailURL := s.cfg.App.LMSBaseURL + port.KHSDetailPath + "&" + q.Encode()
	slog.Info("navigating to KHS detail", "url", detailURL)

	if fresh, navErr := s.navigateWithLMSRetry(session, detailURL, req.NPM, req.Password); navErr != nil {
		return nil, fmt.Errorf("navigate to KHS detail: %w", navErr)
	} else if fresh != session {
		session = fresh
	}

	// Find the CETAK KHS button to get the PDF URL.
	href, err := session.ElementHref(port.SelKHSCetakBtn)
	if err != nil {
		if errors.Is(err, port.ErrLMSExpired) || isTransientBrowserError(err) {
			s.sessions.MarkStale(req.NPM)
		}
		return nil, fmt.Errorf("find CETAK KHS button: %w", err)
	}

	pdfURL := s.cfg.App.LMSBaseURL + "/admin/" + href
	slog.Info("downloading KHS PDF", "url", pdfURL)

	// Build canonical save path and download.
	savePath := filepath.Join(s.cfg.App.DownloadDir, req.NPM, "khs", entity.KHSFilename(req.TahunAjaran, req.Semester))

	filename, size, err := session.DownloadPDF(pdfURL, savePath)
	if err != nil {
		if isTransientBrowserError(err) {
			s.sessions.MarkStale(req.NPM)
		}
		return nil, fmt.Errorf("download KHS PDF: %w", err)
	}

	slog.Info("KHS download complete", "npm", req.NPM, "tahun_ajaran", req.TahunAjaran, "semester", req.Semester, "filename", filename, "size", size)

	return &entity.KHSDownloadResult{
		Success:     true,
		Message:     "KHS downloaded successfully",
		NPM:         req.NPM,
		TahunAjaran: req.TahunAjaran,
		Semester:    req.Semester,
		FilePath:    savePath,
		Size:        size,
		Timestamp:   time.Now(),
	}, nil
}

// DownloadKHSFile serves a KHS PDF file as binary data with local-first + inline fallback.
//
// Fast path: if the PDF already exists on disk, it is served without touching
// the LMS (no GetOrCreate, no browser, no lock).
//
// Fallback (cache miss): acquires the per-NPM lock, re-checks the filesystem
// (another goroutine may have downloaded while we waited), then performs the
// same download flow as DownloadKHS (GetOrCreate → Navigate detail →
// ElementHref(SelKHSCetakBtn) → DownloadPDF) and serves the freshly
// downloaded file.
func (s *lmsDocumentService) DownloadKHSFile(req entity.KHSDownloadRequest) (string, int64, error) {
	req.Semester = strings.ToUpper(strings.TrimSpace(req.Semester))
	req.TahunAjaran = strings.TrimSpace(req.TahunAjaran)
	if !entity.ValidSemester(req.Semester) {
		return "", 0, apperror.BadRequest("semester must be GANJIL or GENAP")
	}
	if req.TahunAjaran == "" {
		return "", 0, apperror.BadRequest("tahun_ajaran is required")
	}

	filePath := filepath.Join(s.cfg.App.DownloadDir, req.NPM, "khs",
		entity.KHSFilename(req.TahunAjaran, req.Semester))

	// Fast path: serve if already on disk — no LMS, no lock.
	if info, err := os.Stat(filePath); err == nil {
		slog.Info("serving KHS file (cache hit)", "npm", req.NPM, "path", filePath, "size", info.Size())
		return filePath, info.Size(), nil
	} else if !os.IsNotExist(err) {
		return "", 0, apperror.Internal("failed to stat PDF file", err)
	}

	// Cache miss: fallback download — same steps as DownloadKHS, under per-NPM lock.
	mu := s.getNPMLock(req.NPM)
	mu.Lock()
	defer mu.Unlock()

	// Re-check after acquiring lock (another goroutine may have downloaded while we waited).
	if info, err := os.Stat(filePath); err == nil {
		slog.Info("serving KHS file (cache hit after lock)", "npm", req.NPM, "path", filePath, "size", info.Size())
		return filePath, info.Size(), nil
	} else if !os.IsNotExist(err) {
		return "", 0, apperror.Internal("failed to stat PDF file", err)
	}

	session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
	if err != nil {
		return "", 0, fmt.Errorf("get session: %w", err)
	}
	defer func() { _ = session.Close() }()

	q := url.Values{}
	q.Set("tahun_ajaran", req.TahunAjaran)
	q.Set("semester", req.Semester)
	detailURL := s.cfg.App.LMSBaseURL + port.KHSDetailPath + "&" + q.Encode()
	slog.Info("navigating to KHS detail (fallback)", "url", detailURL)

	if fresh, navErr := s.navigateWithLMSRetry(session, detailURL, req.NPM, req.Password); navErr != nil {
		return "", 0, fmt.Errorf("navigate to KHS detail: %w", navErr)
	} else if fresh != session {
		session = fresh
	}

	href, err := session.ElementHref(port.SelKHSCetakBtn)
	if err != nil {
		if errors.Is(err, port.ErrLMSExpired) || isTransientBrowserError(err) {
			s.sessions.MarkStale(req.NPM)
		}
		return "", 0, fmt.Errorf("find CETAK KHS button: %w", err)
	}

	pdfURL := s.cfg.App.LMSBaseURL + "/admin/" + href
	slog.Info("downloading KHS PDF (fallback)", "url", pdfURL)

	filename, size, err := session.DownloadPDF(pdfURL, filePath)
	_ = filename
	if err != nil {
		if isTransientBrowserError(err) {
			s.sessions.MarkStale(req.NPM)
		}
		return "", 0, fmt.Errorf("download KHS PDF: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, apperror.NotFound("KHS PDF not found for the specified year and semester", err)
		}
		return "", 0, apperror.Internal("failed to stat PDF file", err)
	}
	_ = size
	slog.Info("serving KHS file (fallback complete)", "npm", req.NPM, "path", filePath, "size", info.Size())
	return filePath, info.Size(), nil
}

// isTransientBrowserError mirrors browser.IsTransientBrowserError without import cycle.
// Spec A: only these bubble MarkStale; 401/404 credential stays cached.
func isTransientBrowserError(err error) bool {
	if err == nil {
		return false
	}
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "eof") ||
		strings.Contains(m, "deadline") ||
		strings.Contains(m, "timeout") ||
		strings.Contains(m, "closed network") ||
		strings.Contains(m, "closed")
}
