package port

// SessionManager manages authenticated browser sessions keyed by NPM.
// It handles session creation, caching, and lifecycle.
// Implementations must be safe for concurrent use by multiple goroutines.
type SessionManager interface {
	// GetOrCreate returns an existing valid session for the given NPM,
	// or creates a new one by logging in with the provided credentials.
	// The session is cached for subsequent requests with the same NPM.
	// Returns error if login fails or browser cannot be started.
	GetOrCreate(npm, password string) (BrowserSession, error)

	// Close releases the session for the given NPM, closing the browser.
	Close(npm string) error

	// CloseAll releases all cached sessions.
	CloseAll()
}
