package extractor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheManager handles caching of extraction results as JSON files.
type CacheManager struct {
	baseDir string
}

// NewCacheManager creates a new cache manager.
func NewCacheManager(baseDir string) *CacheManager {
	return &CacheManager{baseDir: baseDir}
}

// Get retrieves a cached extraction result.
func (c *CacheManager) Get(npm string, docType string, filename string) ([]byte, error) {
	path := c.buildPath(npm, docType, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cache: %w", err)
	}
	return data, nil
}

// Set saves an extraction result to cache with atomic write.
func (c *CacheManager) Set(npm string, docType string, filename string, data []byte) error {
	path := c.buildPath(npm, docType, filename)

	// Create directory if not exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	// Atomic write: write to temp file first, then rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		os.Remove(tmpPath) // cleanup on failure
		return fmt.Errorf("write cache tmp: %w", err)
	}

	// Rename is atomic on same filesystem
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // cleanup on failure
		return fmt.Errorf("rename cache: %w", err)
	}

	return nil
}

// Exists checks if a cached extraction exists.
func (c *CacheManager) Exists(npm string, docType string, filename string) bool {
	path := c.buildPath(npm, docType, filename)
	_, err := os.Stat(path)
	return err == nil
}

// GetModTime returns the modification time of a cached extraction.
func (c *CacheManager) GetModTime(npm string, docType string, filename string) (time.Time, error) {
	path := c.buildPath(npm, docType, filename)
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, fmt.Errorf("stat cache: %w", err)
	}
	return info.ModTime(), nil
}

// Invalidate removes a cached extraction.
func (c *CacheManager) Invalidate(npm string, docType string, filename string) error {
	path := c.buildPath(npm, docType, filename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove cache: %w", err)
	}
	return nil
}

// buildPath constructs the full path for a cached extraction.
func (c *CacheManager) buildPath(npm string, docType string, filename string) string {
	return filepath.Join(c.baseDir, npm, docType, filename)
}

// MarshalJSON marshals data to JSON with indentation.
func MarshalJSON(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
