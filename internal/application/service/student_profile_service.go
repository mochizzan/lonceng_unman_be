package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
	"lonceng_unman_be/internal/infrastructure/photocache"
)

// StudentProfileService defines the contract for student profile operations.
type StudentProfileService interface {
	// Scrape scrapes student profile from LMS and caches the result.
	Scrape(req entity.StudentProfileRequest) (*entity.StudentProfileResult, error)

	// Get retrieves cached student profile data.
	Get(req entity.StudentProfileRequest) (*entity.StudentProfile, error)

	// GetPhoto retrieves the student profile photo. Downloads from LMS if not cached.
	// Returns (photoBytes, contentType, error). Returns (nil, "", nil) if no photo found.
	GetPhoto(req entity.StudentProfileRequest) ([]byte, string, error)
}

// studentProfileService implements StudentProfileService.
type studentProfileService struct {
	cfg        *config.Config
	sessions   port.SessionManager
	scraper    port.StudentProfileScraper
	cache      port.ExtractionCache
	photoCache *photocache.PhotoCache
}

// NewStudentProfileService creates a StudentProfileService.
func NewStudentProfileService(
	cfg *config.Config,
	sessions port.SessionManager,
	scraper port.StudentProfileScraper,
	cache port.ExtractionCache,
	photoCache *photocache.PhotoCache,
) StudentProfileService {
	return &studentProfileService{
		cfg:        cfg,
		sessions:   sessions,
		scraper:    scraper,
		cache:      cache,
		photoCache: photoCache,
	}
}

// doScrape is the inner scrape after a valid session.
func (s *studentProfileService) doScrape(sess port.BrowserSession, req entity.StudentProfileRequest) (*entity.StudentProfile, error) {
	return s.scraper.Scrape(sess, s.cfg.App.LMSBaseURL, s.cfg)
}

// Scrape always scrapes fresh from LMS, overwriting existing cache.
// If LMS PHP session has expired (backend 24h still "valid" but LMS menit has GC'd),
// detect ErrLMSExpired from Navigate, evict once, re-login fresh and retry exactly once.
func (s *studentProfileService) Scrape(req entity.StudentProfileRequest) (*entity.StudentProfileResult, error) {
	// 1. Get or create session (auto-login)
	sess, err := s.sessions.GetOrCreate(req.NPM, req.Password)
	if err != nil {
		slog.Warn("student profile session failed", "npm", req.NPM, "error", err)
		return nil, fmt.Errorf("get session: %w", err)
	}
	defer func() { _ = sess.Close() }()

	// 2. Scrape profile. On LMS expiry, MarkStale + retry fresh once; on transient CDP (deadline/closed) expiry, MarkStale so next GetOrCreate is fresh.
	profile, err := s.doScrape(sess, req)
	if err != nil {
		if errors.Is(err, port.ErrLMSExpired) {
			slog.Warn("lms session expired during scrape — mark stale and re-login once", "npm", req.NPM, "error", err)
			s.sessions.MarkStale(req.NPM)
			_ = sess.Close()
			fresh, gerr := s.sessions.GetOrCreate(req.NPM, req.Password)
			if gerr != nil {
				return nil, fmt.Errorf("re-login after lms expiry: %w", gerr)
			}
			sess = fresh
			// defer above will close fresh; avoid double-close leak on early return.
			profile, err = s.doScrape(sess, req)
			if err != nil {
				slog.Warn("student profile scrape failed after re-login", "npm", req.NPM, "error", err)
				return nil, fmt.Errorf("scrape profile after re-login: %w", err)
			}
		} else {
			if isTransientBrowserError(err) {
				s.sessions.MarkStale(req.NPM)
			}
			slog.Warn("student profile scrape failed", "npm", req.NPM, "error", err)
			return nil, fmt.Errorf("scrape profile: %w", err)
		}
	}

	// 3. Marshal to JSON
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal profile: %w", err)
	}

	// 4. Cache (always overwrite)
	if err := s.cache.Set(req.NPM, "profile", "student_profile.json", data); err != nil {
		slog.Warn("failed to cache student profile", "npm", req.NPM, "error", err)
		// Non-fatal — still return success
	}

	slog.Info("student profile scraped", "npm", req.NPM)

	return &entity.StudentProfileResult{
		NPM:      req.NPM,
		Message:  "Profile scraped successfully",
		CachedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// Get retrieves cached student profile. Returns error if not cached.
func (s *studentProfileService) Get(req entity.StudentProfileRequest) (*entity.StudentProfile, error) {
	// 1. Check cache exists
	if !s.cache.Exists(req.NPM, "profile", "student_profile.json") {
		return nil, fmt.Errorf("student profile not found. Use POST /api/v1/lms/student-profile to fetch")
	}

	// 2. Read from cache
	data, err := s.cache.Get(req.NPM, "profile", "student_profile.json")
	if err != nil {
		return nil, fmt.Errorf("read cache: %w", err)
	}

	// 3. Unmarshal
	var profile entity.StudentProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("unmarshal profile: %w", err)
	}

	return &profile, nil
}

// GetPhoto retrieves student profile photo. Returns cached if valid, otherwise scrapes from LMS.
func (s *studentProfileService) GetPhoto(req entity.StudentProfileRequest) ([]byte, string, error) {
	// 1. Check cache first
	photoPath, err := s.photoCache.Get(req.NPM)
	if err != nil {
		slog.Warn("failed to read photo cache", "npm", req.NPM, "error", err)
		// Non-fatal — continue to scrape
	}
	if photoPath != "" {
		data, err := os.ReadFile(photoPath)
		if err == nil {
			slog.Info("photo served from cache", "npm", req.NPM)
			return data, "image/jpeg", nil
		}
	}

	// 2. Get or create session (auto-login)
	sess, err := s.sessions.GetOrCreate(req.NPM, req.Password)
	if err != nil {
		return nil, "", fmt.Errorf("get session: %w", err)
	}
	defer func() { _ = sess.Close() }()

	// 3. Navigate to dashboard — retry once if LMS expired (backend 24h vs LMS menit)
	dashboardURL := s.cfg.App.LMSBaseURL + "/admin/"
	if err := sess.Navigate(dashboardURL); err != nil {
		if errors.Is(err, port.ErrLMSExpired) {
			slog.Warn("lms session expired during photo navigate — mark stale and re-login once", "npm", req.NPM, "error", err)
			s.sessions.MarkStale(req.NPM)
			_ = sess.Close()
			fresh, gerr := s.sessions.GetOrCreate(req.NPM, req.Password)
			if gerr != nil {
				return nil, "", fmt.Errorf("re-login after lms expiry: %w", gerr)
			}
			sess = fresh
			// fresh will be closed by outer defer (captures var sess by ref)
			if err2 := sess.Navigate(dashboardURL); err2 != nil {
				return nil, "", fmt.Errorf("navigate to dashboard after re-login: %w", err2)
			}
		} else {
			if isTransientBrowserError(err) {
				s.sessions.MarkStale(req.NPM)
			}
			return nil, "", fmt.Errorf("navigate to dashboard: %w", err)
		}
	}

	// 4. Wait for page to fully render (photo may load dynamically).
	// Default 3s (was hardcoded 8s); override with PHOTO_RENDER_WAIT env.
	renderWait := s.cfg.App.PhotoRenderWait
	if renderWait <= 0 {
		renderWait = 3 * time.Second
	}
	time.Sleep(renderWait)

	// 5. Extract photo src via JS eval with retry
	// The <img> is inside <div class="small-box bg-yellow"> wrapped by <a href="ktm_take_foto.php">
	// We use a specific selector to avoid matching default/placeholder images.
	var src string
	for range 3 {
		// go-rod wraps code in: function() { return (CODE).apply(this, arguments) }
		// So we use an async function expression, NOT an IIFE.
		jsCode := `async () => {
			// Find the <a> that links to ktm_take_foto.php, then get its child <img>
			const link = document.querySelector('a[href*="ktm_take_foto"]');
			if (!link) return '';
			const img = link.querySelector('img[src*="uploads_foto"]');
			if (!img) return '';
			return img.getAttribute('src') || '';
		}`

		src, err = sess.Eval(jsCode)
		if err != nil {
			if isTransientBrowserError(err) {
				s.sessions.MarkStale(req.NPM)
			}
			return nil, "", fmt.Errorf("extract photo src: %w", err)
		}

		if src != "" {
			break
		}

		// Wait before retry (was hardcoded 3s; override with PHOTO_RETRY_WAIT).
		retryWait := s.cfg.App.PhotoRetryWait
		if retryWait <= 0 {
			retryWait = 1 * time.Second
		}
		time.Sleep(retryWait)
	}
	if err != nil {
		return nil, "", fmt.Errorf("extract photo src: %w", err)
	}

	if src == "" {
		// No photo found — return nil (handler will return 204)
		slog.Info("no photo found on dashboard", "npm", req.NPM)
		return nil, "", nil
	}

	// 6. Resolve full URL and download to per-NPM dir
	photoURL := s.cfg.App.LMSBaseURL + "/admin/" + src
	savePath := filepath.Join(s.cfg.App.DownloadDir, req.NPM, "photo", req.NPM+".jpg")
	_, _, err = sess.DownloadImage(photoURL, savePath)
	if err != nil {
		if isTransientBrowserError(err) {
			s.sessions.MarkStale(req.NPM)
		}
		return nil, "", fmt.Errorf("download photo: %w", err)
	}

	// 7. Read downloaded photo
	photoData, err := os.ReadFile(savePath)
	if err != nil {
		return nil, "", fmt.Errorf("read downloaded photo: %w", err)
	}

	// 8. Compress photo (resize + re-encode) before caching
	originalSize := len(photoData)
	compressed, err := photocache.CompressPhoto(
		photoData,
		uint(s.cfg.App.MaxPhotoDimension),
		s.cfg.App.PhotoQuality,
	)
	if err != nil {
		slog.Warn("photo compression failed, serving original",
			"npm", req.NPM, "error", err)
		// Non-fatal — use original bytes
	} else {
		photoData = compressed
		// Overwrite disk file with compressed version
		if writeErr := os.WriteFile(savePath, photoData, 0o644); writeErr != nil {
			slog.Warn("failed to write compressed photo to disk",
				"npm", req.NPM, "error", writeErr)
		}
		slog.Info("photo compressed",
			"npm", req.NPM,
			"original_bytes", originalSize,
			"compressed_bytes", len(photoData),
			"ratio", fmt.Sprintf("%.1f%%", float64(len(photoData))/float64(originalSize)*100))
	}

	// 9. Save metadata to cache
	if err := s.photoCache.Set(req.NPM, photoData, filepath.Base(src)); err != nil {
		slog.Warn("failed to cache photo metadata", "npm", req.NPM, "error", err)
		// Non-fatal — still return photo
	}

	slog.Info("photo downloaded and cached", "npm", req.NPM, "src", src)

	return photoData, "image/jpeg", nil
}

// Compile-time interface check
var _ StudentProfileService = (*studentProfileService)(nil)
