package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
	"lonceng_unman_be/internal/domain/port"
)

// StudentProfileService defines the contract for student profile operations.
type StudentProfileService interface {
	// Scrape scrapes student profile from LMS and caches the result.
	Scrape(req entity.StudentProfileRequest) (*entity.StudentProfileResult, error)

	// Get retrieves cached student profile data.
	Get(req entity.StudentProfileRequest) (*entity.StudentProfile, error)
}

// studentProfileService implements StudentProfileService.
type studentProfileService struct {
	cfg      *config.Config
	sessions port.SessionManager
	scraper  port.StudentProfileScraper
	cache    port.ExtractionCache
}

// NewStudentProfileService creates a StudentProfileService.
func NewStudentProfileService(
	cfg *config.Config,
	sessions port.SessionManager,
	scraper port.StudentProfileScraper,
	cache port.ExtractionCache,
) StudentProfileService {
	return &studentProfileService{
		cfg:      cfg,
		sessions: sessions,
		scraper:  scraper,
		cache:    cache,
	}
}

// Scrape always scrapes fresh from LMS, overwriting existing cache.
func (s *studentProfileService) Scrape(req entity.StudentProfileRequest) (*entity.StudentProfileResult, error) {
	// 1. Get or create session (auto-login)
	session, err := s.sessions.GetOrCreate(req.NPM, req.Password)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	// 2. Scrape profile (pass base URL for full navigation)
	profile, err := s.scraper.Scrape(session, s.cfg.App.LMSBaseURL)
	if err != nil {
		return nil, fmt.Errorf("scrape profile: %w", err)
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

// Compile-time interface check
var _ StudentProfileService = (*studentProfileService)(nil)
