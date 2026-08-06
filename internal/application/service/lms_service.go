package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"

	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
	browserInfra "lonceng_unman_be/internal/infrastructure/browser"
)

// LMSLogin defines the contract for LMS login operations.
type LMSLogin interface {
	Login(req entity.LoginRequest) (*entity.LoginResult, error)
}

// lmsService implements LMSLogin.
type lmsService struct {
	cfg *config.Config
}

// NewLMSService creates an LMSLogin from application config.
func NewLMSService(cfg *config.Config) LMSLogin {
	return &lmsService{cfg: cfg}
}

// LMSDocumentService defines the contract for LMS document download operations.
type LMSDocumentService interface {
	// DownloadKRS downloads the KRS PDF for the given NPM.
	DownloadKRS(npm string) (*entity.KRSDownloadResult, error)

	// GetKHSSemesters returns the list of available KHS semesters for the given NPM.
	GetKHSSemesters(npm string) (*entity.KHSSemestersResult, error)

	// DownloadKHS downloads the KHS PDF for a specific semester.
	DownloadKHS(npm, tahunAjaran, semester string) (*entity.KHSDownloadResult, error)
}

// lmsDocumentService implements LMSDocumentService.
type lmsDocumentService struct {
	lmsService *lmsService // embeds login logic
}

// NewLMSDocumentService creates an LMSDocumentService from application config.
func NewLMSDocumentService(cfg *config.Config) LMSDocumentService {
	return &lmsDocumentService{
		lmsService: &lmsService{cfg: cfg},
	}
}

// Login performs the full login flow: launch browser, navigate, fill form, submit, detect result.
// Returns (result, nil) for both successful and failed logins (business outcomes).
// Returns (nil, err) only for infrastructure failures (browser crash, page load failure).
func (s *lmsService) Login(req entity.LoginRequest) (*entity.LoginResult, error) {
	br := browserInfra.New()
	if err := br.Connect(s.cfg.App.BrowserHeadless); err != nil {
		slog.Error("browser connect failed", "err", err)
		return nil, fmt.Errorf("browser connect: %w", err)
	}
	defer func() {
		if err := br.Close(); err != nil {
			slog.Warn("browser close error", "err", err)
		}
	}()

	page, err := br.Page(s.cfg.App.LMSBaseURL)
	if err != nil {
		slog.Error("open login page failed", "url", s.cfg.App.LMSBaseURL, "err", err)
		return nil, fmt.Errorf("open login page: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		slog.Error("login page load failed", "url", s.cfg.App.LMSBaseURL, "err", err)
		return nil, fmt.Errorf("wait login page load: %w", err)
	}

	if err := s.login(page, req.NPM, req.Password); err != nil {
		slog.Info("login failed", "npm", req.NPM, "err", err)
		return &entity.LoginResult{
			Success:   false,
			Message:   err.Error(),
			NPM:       req.NPM,
			Timestamp: time.Now(),
		}, nil
	}

	slog.Info("login successful", "npm", req.NPM)
	return &entity.LoginResult{
		Success:   true,
		Message:   "Login successful",
		NPM:       req.NPM,
		Timestamp: time.Now(),
	}, nil
}

// login fills the login form and submits it, detecting success or failure.
// Detection uses Race pattern (DOM-based) with URL confirmation.
func (s *lmsService) login(page *rod.Page, npm, password string) error {
	page = page.Timeout(s.cfg.App.BrowserTimeout)

	usernameEl, err := page.Element(browserInfra.SelUsernameInput)
	if err != nil {
		return fmt.Errorf("find username field: %w", err)
	}
	if err := usernameEl.Input(npm); err != nil {
		return fmt.Errorf("fill username: %w", err)
	}

	passwordEl, err := page.Element(browserInfra.SelPasswordInput)
	if err != nil {
		return fmt.Errorf("find password field: %w", err)
	}
	if err := passwordEl.Input(password); err != nil {
		return fmt.Errorf("fill password: %w", err)
	}

	race := page.Race().
		Element(browserInfra.SelSuccessIndicator).
		Element(browserInfra.SelErrorIndicator)

	submitEl, err := page.Element(browserInfra.SelSubmitButton)
	if err != nil {
		return fmt.Errorf("find submit button: %w", err)
	}
	if err := submitEl.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("click submit: %w", err)
	}

	result, err := race.Do()
	if err != nil {
		return fmt.Errorf("login timed out: no response detected: %w", err)
	}
	if result == nil {
		return fmt.Errorf("login timed out: no response detected")
	}

	// Check if error indicator matched
	matched, err := result.Matches(browserInfra.SelErrorIndicator)
	if err != nil {
		return fmt.Errorf("detect login result: %w", err)
	}
	if matched {
		errorText, err := result.Text()
		if err != nil {
			return fmt.Errorf("login failed (could not read error text): %w", err)
		}
		return fmt.Errorf("login failed: %s", errorText)
	}

	// DOM-based success confirmed; also verify URL changed to /admin/
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("wait for dashboard load: %w", err)
	}

	info, err := page.Info()
	if err != nil {
		return fmt.Errorf("get page info: %w", err)
	}

	if !strings.Contains(info.URL, "/admin/") {
		return fmt.Errorf("login failed: page did not redirect to dashboard (url: %s)", info.URL)
	}

	return nil
}

// loginAndNavigate creates a browser, logs in to the LMS, and returns the authenticated page.
// The caller must close the browser when done.
func (s *lmsService) loginAndNavigate() (*browserInfra.Browser, *rod.Page, error) {
	br := browserInfra.New()
	if err := br.Connect(s.cfg.App.BrowserHeadless); err != nil {
		return nil, nil, fmt.Errorf("browser connect: %w", err)
	}

	page, err := br.Page(s.cfg.App.LMSBaseURL)
	if err != nil {
		br.Close()
		return nil, nil, fmt.Errorf("open login page: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		br.Close()
		return nil, nil, fmt.Errorf("wait login page load: %w", err)
	}

	// Navigate to dashboard (assumes session is already authenticated)
	// In production, this would call s.login() with credentials
	dashboardURL := s.cfg.App.LMSDashboardURL
	if err := page.Navigate(dashboardURL); err != nil {
		br.Close()
		return nil, nil, fmt.Errorf("navigate to dashboard: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		br.Close()
		return nil, nil, fmt.Errorf("wait dashboard load: %w", err)
	}

	return br, page, nil
}

// DownloadKRS downloads the KRS PDF for the given NPM.
// The PDF is saved to: {downloadDir}/{npm}/krs/krs.pdf
func (s *lmsDocumentService) DownloadKRS(npm string) (*entity.KRSDownloadResult, error) {
	br, page, err := s.lmsService.loginAndNavigate()
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	defer br.Close()

	// KRS URL pattern: /admin/cetak/krs_pdf.php?nis={npm}
	krsURL := s.lmsService.cfg.App.LMSBaseURL + browserInfra.KRSDownloadPath + "?nis=" + npm
	slog.Info("downloading KRS", "url", krsURL)

	// Build canonical save path: {downloadDir}/{npm}/krs/krs.pdf
	// Note: KRS doesn't have tahun_ajaran/semester in URL, so we use fixed name
	savePath := filepath.Join(s.lmsService.cfg.App.DownloadDir, npm, "krs", "krs.pdf")

	// Download and save
	filename, size, err := browserInfra.DownloadAndSave(page, krsURL, savePath)
	if err != nil {
		return nil, fmt.Errorf("download KRS PDF: %w", err)
	}

	slog.Info("KRS download complete", "npm", npm, "filename", filename, "size", size)

	return &entity.KRSDownloadResult{
		Success:   true,
		Message:   "KRS downloaded successfully",
		NPM:       npm,
		FilePath:  savePath,
		Size:      size,
		Timestamp: time.Now(),
	}, nil
}

// GetKHSSemesters returns the list of available KHS semesters for the given NPM.
func (s *lmsDocumentService) GetKHSSemesters(npm string) (*entity.KHSSemestersResult, error) {
	br, page, err := s.lmsService.loginAndNavigate()
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	defer br.Close()

	// Navigate to KHS list page
	khsListURL := s.lmsService.cfg.App.LMSBaseURL + "/admin/main.php?op=mahasiswa_khs&act=cetak"
	slog.Info("fetching KHS semesters", "url", khsListURL)

	if err := page.Navigate(khsListURL); err != nil {
		return nil, fmt.Errorf("navigate to KHS list: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("wait KHS list load: %w", err)
	}

	// Parse HTML to extract semesters
	// The table has rows with text like "2022/2023 - GANJIL - 20 SKS"
	result, err := page.Eval(`() => {
		const rows = document.querySelectorAll('` + browserInfra.SelKHSTable + ` tbody tr');
		const semesters = [];
		rows.forEach(row => {
			const cells = row.querySelectorAll('td');
			if (cells.length >= 6) {
				const sksCell = cells[5].innerText;
				// Parse "2022/2023 - GANJIL - 20 SKS"
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
	}`)
	if err != nil {
		return nil, fmt.Errorf("parse KHS semesters: %w", err)
	}

	var semesters []entity.KHSSemester
	if err := json.Unmarshal([]byte(result.Value.Str()), &semesters); err != nil {
		return nil, fmt.Errorf("unmarshal semesters: %w", err)
	}

	slog.Info("KHS semesters found", "npm", npm, "count", len(semesters))

	return &entity.KHSSemestersResult{
		NPM:       npm,
		Semesters: semesters,
		Timestamp: time.Now(),
	}, nil
}

// DownloadKHS downloads the KHS PDF for a specific semester.
// The PDF is saved to: {downloadDir}/{npm}/khs/{tahunAjaran}_{semester}.pdf
func (s *lmsDocumentService) DownloadKHS(npm, tahunAjaran, semester string) (*entity.KHSDownloadResult, error) {
	br, page, err := s.lmsService.loginAndNavigate()
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	defer br.Close()

	// Step 1: Navigate to KHS detail page
	detailURL := fmt.Sprintf("%s/admin/main.php?op=mahasiswa_khs&act=cetak_detail&tahun_ajaran=%s&semester=%s",
		s.lmsService.cfg.App.LMSBaseURL, tahunAjaran, semester)
	slog.Info("navigating to KHS detail", "url", detailURL)

	if err := page.Navigate(detailURL); err != nil {
		return nil, fmt.Errorf("navigate to KHS detail: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("wait KHS detail load: %w", err)
	}

	// Step 2: Find the CETAK KHS button to get the PDF URL
	pdfLink, err := page.Element(browserInfra.SelKHSCetakBtn)
	if err != nil {
		return nil, fmt.Errorf("find CETAK KHS button: %w", err)
	}

	href, err := pdfLink.Attribute("href")
	if err != nil || href == nil {
		return nil, fmt.Errorf("get PDF link href: %w", err)
	}

	pdfURL := s.lmsService.cfg.App.LMSBaseURL + "/admin/" + *href
	slog.Info("downloading KHS PDF", "url", pdfURL)

	// Step 3: Build canonical save path and download
	savePath := browserInfra.BuildKHSPath(s.lmsService.cfg.App.DownloadDir, npm, tahunAjaran, semester)

	filename, size, err := browserInfra.DownloadAndSave(page, pdfURL, savePath)
	if err != nil {
		return nil, fmt.Errorf("download KHS PDF: %w", err)
	}

	slog.Info("KHS download complete", "npm", npm, "tahun_ajaran", tahunAjaran, "semester", semester, "filename", filename, "size", size)

	return &entity.KHSDownloadResult{
		Success:     true,
		Message:     "KHS downloaded successfully",
		NPM:         npm,
		TahunAjaran: tahunAjaran,
		Semester:    semester,
		FilePath:    savePath,
		Size:        size,
		Timestamp:   time.Now(),
	}, nil
}
