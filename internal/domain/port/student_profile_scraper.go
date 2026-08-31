package port

import (
	"lonceng_unman_be/internal/config"
	"lonceng_unman_be/internal/domain/entity"
)

// StudentProfileScraper scrapes student profile data from the LMS HTML form.
type StudentProfileScraper interface {
	// Scrape navigates to the profile page, reads all form fields via bulk JS eval,
	// and returns the parsed StudentProfile.
	// lmsBaseURL is the full base URL of the LMS (e.g. "https://elearning.universitasmandiri.ac.id").
	// cfg provides cold-start tuning knobs (ScrapeFormWait) and may be nil — in
	// which case the scraper falls back to its hardcoded default.
	Scrape(session BrowserSession, lmsBaseURL string, cfg *config.Config) (*entity.StudentProfile, error)
}
