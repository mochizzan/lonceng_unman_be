package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
	browserInfra "lonceng_unman_be/internal/infrastructure/browser"
)

// LMSLogin defines the contract for LMS login operations.
type LMSLogin interface {
	// Login validates credentials by attempting a session creation.
	// Returns success/failure as a business outcome (nil error for both).
	// Returns error only for infrastructure failures.
	Login(req entity.LoginRequest) (*entity.LoginResult, error)
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
}

// lmsDocumentService implements LMSDocumentService.
type lmsDocumentService struct {
	cfg      *config.Config
	sessions port.SessionManager
}

// NewLMSDocumentService creates an LMSDocumentService with session management.
func NewLMSDocumentService(cfg *config.Config, sessions port.SessionManager) LMSDocumentService {
	return &lmsDocumentService{cfg: cfg, sessions: sessions}
}

// Login validates credentials by creating a session. The session is cached
// for subsequent requests. Returns success/failure as business outcomes.
func (s *lmsService) Login(req entity.LoginRequest) (*entity.LoginResult, error) {
	session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
	if err != nil {
		slog.Info("login failed", "npm", req.NPM, "err", err)
		return &entity.LoginResult{
			Success:   false,
			Message:   err.Error(),
			NPM:       req.NPM,
			Timestamp: time.Now(),
		}, nil
	}
	defer session.Close()

	slog.Info("login successful", "npm", req.NPM)
	return &entity.LoginResult{
		Success:   true,
		Message:   "Login successful",
		NPM:       req.NPM,
		Timestamp: time.Now(),
	}, nil
}

// DownloadKRS downloads the KRS PDF for the given student.
// Flow: get session → navigate to KRS page → extract semester → download PDF.
func (s *lmsDocumentService) DownloadKRS(req entity.KRSDownloadRequest) (*entity.KRSDownloadResult, error) {
	session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	defer session.Close()

	// Navigate to KRS page to extract semester number.
	krsPageURL := s.cfg.App.LMSBaseURL + browserInfra.KRSPagePath
	slog.Info("navigating to KRS page", "url", krsPageURL)

	if err := session.Navigate(krsPageURL); err != nil {
		return nil, fmt.Errorf("navigate to KRS page: %w", err)
	}

	// Extract semester number from the page.
	semesterNum, err := session.ElementAttribute(browserInfra.SelKRSSemesterInput, "value")
	if err != nil {
		return nil, fmt.Errorf("extract semester: %w", err)
	}
	slog.Info("KRS semester extracted", "npm", req.NPM, "semester", semesterNum)

	// Download KRS PDF.
	krsURL := s.cfg.App.LMSBaseURL + browserInfra.KRSDownloadPath + "?nis=" + req.NPM
	slog.Info("downloading KRS", "url", krsURL)

	savePath := filepath.Join(s.cfg.App.DownloadDir, req.NPM, "krs", fmt.Sprintf("semester_%s.pdf", semesterNum))

	filename, size, err := session.DownloadPDF(krsURL, savePath)
	if err != nil {
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
func (s *lmsDocumentService) GetKHSSemesters(req entity.KHSSemestersRequest) (*entity.KHSSemestersResult, error) {
	session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	defer session.Close()

	// Navigate to KHS list page.
	khsListURL := s.cfg.App.LMSBaseURL + browserInfra.KHSListPath
	slog.Info("fetching KHS semesters", "url", khsListURL)

	if err := session.Navigate(khsListURL); err != nil {
		return nil, fmt.Errorf("navigate to KHS list: %w", err)
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
		return nil, fmt.Errorf("parse KHS semesters: %w", err)
	}

	var semesters []entity.KHSSemester
	if err := json.Unmarshal([]byte(result), &semesters); err != nil {
		return nil, fmt.Errorf("unmarshal semesters: %w", err)
	}

	slog.Info("KHS semesters found", "npm", req.NPM, "count", len(semesters))

	return &entity.KHSSemestersResult{
		NPM:       req.NPM,
		Semesters: semesters,
		Timestamp: time.Now(),
	}, nil
}

// DownloadKHS downloads the KHS PDF for a specific semester.
func (s *lmsDocumentService) DownloadKHS(req entity.KHSDownloadRequest) (*entity.KHSDownloadResult, error) {
	// Validate semester format.
	req.Semester = strings.ToUpper(req.Semester)
	if req.Semester != "GANJIL" && req.Semester != "GENAP" {
		return nil, fmt.Errorf("semester must be GANJIL or GENAP")
	}

	session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	defer session.Close()

	// Navigate to KHS detail page.
	detailURL := fmt.Sprintf("%s%s&tahun_ajaran=%s&semester=%s",
		s.cfg.App.LMSBaseURL, browserInfra.KHSDetailPath, req.TahunAjaran, req.Semester)
	slog.Info("navigating to KHS detail", "url", detailURL)

	if err := session.Navigate(detailURL); err != nil {
		return nil, fmt.Errorf("navigate to KHS detail: %w", err)
	}

	// Find the CETAK KHS button to get the PDF URL.
	href, err := session.ElementHref(browserInfra.SelKHSCetakBtn)
	if err != nil {
		return nil, fmt.Errorf("find CETAK KHS button: %w", err)
	}

	pdfURL := s.cfg.App.LMSBaseURL + "/admin/" + href
	slog.Info("downloading KHS PDF", "url", pdfURL)

	// Build canonical save path and download.
	savePath := filepath.Join(s.cfg.App.DownloadDir, req.NPM, "khs", entity.KHSFilename(req.TahunAjaran, req.Semester))

	filename, size, err := session.DownloadPDF(pdfURL, savePath)
	if err != nil {
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
