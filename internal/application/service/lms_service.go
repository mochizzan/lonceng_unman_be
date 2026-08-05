package service

import (
	"fmt"
	"log/slog"
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
