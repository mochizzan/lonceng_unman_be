package photocache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
	mu      sync.RWMutex // guards Get/Set/Invalidate + orphan delete
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

	c.mu.RLock()
	data, err := os.ReadFile(metaPath)
	if os.IsNotExist(err) {
		c.mu.RUnlock()
		return "", nil // cache miss
	}
	if err != nil {
		c.mu.RUnlock()
		return "", fmt.Errorf("read cache meta: %w", err)
	}

	var meta PhotoCacheMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		c.mu.RUnlock()
		return "", fmt.Errorf("unmarshal cache meta: %w", err)
	}

	if time.Now().After(meta.ExpiresAt) {
		c.mu.RUnlock()
		return "", nil // expired
	}

	if _, err := os.Stat(photoPath); err != nil {
		if os.IsNotExist(err) {
			// Orphan metadata — need write lock to delete.
			c.mu.RUnlock()
			c.mu.Lock()
			// Re-check under write lock to avoid deleting if raced Set succeeded.
			if _, err := os.Stat(photoPath); os.IsNotExist(err) {
				_ = os.Remove(metaPath)
			}
			c.mu.Unlock()
			return "", nil
		}
		c.mu.RUnlock()
		return "", fmt.Errorf("stat photo file: %w", err)
	}

	c.mu.RUnlock()
	return photoPath, nil
}

// Set saves the photo and metadata to cache atomically.
// Both files are written via tmp+rename so a crash mid-write cannot leave
// a partial file. Photo is written first, then metadata.
func (c *PhotoCache) Set(npm string, photoData []byte, originalFilename string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(c.photoDir(npm), 0o755); err != nil {
		return fmt.Errorf("create photo dir: %w", err)
	}

	// Save photo file atomically via tmp+rename
	photoPath := c.photoPath(npm)
	if err := writeAtomic(photoPath, photoData, 0o644); err != nil {
		return fmt.Errorf("save photo: %w", err)
	}

	// Save metadata atomically via tmp+rename
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
	if err := writeAtomic(c.metaPath(npm), metaData, 0o644); err != nil {
		return fmt.Errorf("save cache meta: %w", err)
	}

	return nil
}

// writeAtomic writes data to path via a temporary file + rename so the
// target file is never observed in a partially-written state.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	_, err = tmp.Write(data)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

// Invalidate removes both the metadata and photo files for a given NPM.
// It is idempotent: missing files are ignored.
func (c *PhotoCache) Invalidate(npm string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var firstErr error
	for _, path := range []string{c.metaPath(npm), c.photoPath(npm)} {
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if firstErr != nil {
		return fmt.Errorf("invalidate cache: %w", firstErr)
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
