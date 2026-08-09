package photocache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PhotoCacheMeta stores metadata about a cached photo.
type PhotoCacheMeta struct {
	NPM              string    `json:"npm"`
	CachedAt         time.Time `json:"cached_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	OriginalFilename string    `json:"original_filename"`
}

// PhotoCache manages cached student profile photos.
type PhotoCache struct {
	baseDir string
	ttl     time.Duration
}

// New creates a PhotoCache with the given base directory and TTL.
func New(baseDir string, ttl time.Duration) *PhotoCache {
	return &PhotoCache{baseDir: baseDir, ttl: ttl}
}

// Get returns the photo file path if cached and not expired.
// Returns ("", nil) if cache miss or expired.
func (c *PhotoCache) Get(npm string) (string, error) {
	metaPath := c.metaPath(npm)
	photoPath := c.photoPath(npm)

	data, err := os.ReadFile(metaPath)
	if os.IsNotExist(err) {
		return "", nil // cache miss
	}
	if err != nil {
		return "", fmt.Errorf("read cache meta: %w", err)
	}

	var meta PhotoCacheMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", fmt.Errorf("unmarshal cache meta: %w", err)
	}

	if time.Now().After(meta.ExpiresAt) {
		return "", nil // expired
	}

	// Verify photo file exists
	if _, err := os.Stat(photoPath); os.IsNotExist(err) {
		return "", nil // photo file missing
	}

	return photoPath, nil
}

// Set saves the photo and metadata to cache.
func (c *PhotoCache) Set(npm string, photoData []byte, originalFilename string) error {
	if err := os.MkdirAll(c.photoDir(npm), 0o755); err != nil {
		return fmt.Errorf("create photo dir: %w", err)
	}

	// Save photo file
	if err := os.WriteFile(c.photoPath(npm), photoData, 0o644); err != nil {
		return fmt.Errorf("save photo: %w", err)
	}

	// Save metadata
	meta := PhotoCacheMeta{
		NPM:              npm,
		CachedAt:         time.Now(),
		ExpiresAt:        time.Now().Add(c.ttl),
		OriginalFilename: originalFilename,
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache meta: %w", err)
	}
	if err := os.WriteFile(c.metaPath(npm), metaData, 0o644); err != nil {
		return fmt.Errorf("save cache meta: %w", err)
	}

	return nil
}

func (c *PhotoCache) photoDir(npm string) string {
	return filepath.Join(c.baseDir, npm, "photo")
}

func (c *PhotoCache) photoPath(npm string) string {
	return filepath.Join(c.photoDir(npm), npm+".jpg")
}

func (c *PhotoCache) metaPath(npm string) string {
	return filepath.Join(c.photoDir(npm), npm+".json")
}
