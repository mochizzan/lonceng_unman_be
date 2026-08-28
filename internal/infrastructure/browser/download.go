package browser

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
)

// downloadBytes fetches a URL via JS fetch() inside the browser page,
// decodes the response from base64, and returns the raw bytes.
// If expectedContentType is non-empty, the response Content-Type must contain it.
// Uses fetch() with credentials: "include" to send session cookies.
//
// go-rod's Eval wraps code in: function() { return (CODE).apply(this, arguments) }
// So we pass an async function expression — NOT an IIFE.
func downloadBytes(page *rod.Page, url string, expectedContentType string) ([]byte, error) {
	// Build JS: optionally check content-type header.
	contentTypeCheck := ""
	if expectedContentType != "" {
		contentTypeCheck = fmt.Sprintf(`
			const ct = response.headers.get("content-type");
			if (!ct || !ct.includes("%s")) {
				throw new Error("Expected %s but got: " + ct);
			}`, expectedContentType, expectedContentType)
	}

	jsCode := fmt.Sprintf(`async () => {
		const response = await fetch("%s", { credentials: "include" });
		if (!response.ok) {
			throw new Error("HTTP " + response.status + " " + response.statusText);
		}
		%s
		const buffer = await response.arrayBuffer();
		const bytes = new Uint8Array(buffer);
		let binary = "";
		for (let i = 0; i < bytes.byteLength; i++) {
			binary += String.fromCharCode(bytes[i]);
		}
		return btoa(binary);
	}`, url, contentTypeCheck)

	result, err := page.Eval(jsCode)
	if err != nil {
		return nil, fmt.Errorf("fetch via JS: %w", err)
	}

	data, err := base64.StdEncoding.DecodeString(result.Value.Str())
	if err != nil {
		return nil, fmt.Errorf("decode body: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("downloaded file is empty")
	}

	return data, nil
}

// DownloadAndSave downloads a PDF from the given URL using JavaScript fetch()
// and saves to savePath. Validates that the response Content-Type is application/pdf.
//
// Note: We use fetch() instead of <a download> because cross-origin <a download>
// is blocked by browsers for security. fetch() with credentials: "include"
// properly sends session cookies for authenticated URLs.
func DownloadAndSave(page *rod.Page, url string, savePath string) (string, int, error) {
	if err := os.MkdirAll(filepath.Dir(savePath), 0o755); err != nil {
		return "", 0, fmt.Errorf("create save dir: %w", err)
	}

	data, err := downloadBytes(page, url, "application/pdf")
	if err != nil {
		return "", 0, fmt.Errorf("download PDF: %w", err)
	}

	if err := os.WriteFile(savePath, data, 0o644); err != nil {
		return "", 0, fmt.Errorf("save PDF: %w", err)
	}

	if err := writeMetadataSidecar(savePath, data, url); err != nil {
		return "", 0, fmt.Errorf("write metadata sidecar: %w", err)
	}

	return filepath.Base(savePath), len(data), nil
}

// writeMetadataSidecar writes a .meta.json file alongside the saved PDF
// containing provenance and integrity metadata.
func writeMetadataSidecar(savePath string, data []byte, sourceURL string) error {
	npm, docType := parseSavePath(savePath)

	hash := sha256.Sum256(data)

	meta := map[string]any{
		"npm":           npm,
		"type":          docType,
		"file_path":     savePath,
		"file_name":     filepath.Base(savePath),
		"file_size":     len(data),
		"downloaded_at": time.Now().UTC(),
		"source_url":    sourceURL,
		"content_hash":  fmt.Sprintf("%x", hash),
		"is_valid_pdf":  len(data) > 4 && string(data[:4]) == "%PDF",
	}

	metaPath := savePath + ".meta.json"
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, metaJSON, 0o644); err != nil {
		return fmt.Errorf("write metadata file: %w", err)
	}

	return nil
}

// parseSavePath extracts the NPM and document type from a save path.
// Expected format: .../{NPM}/{docType}/{filename}.pdf
func parseSavePath(savePath string) (npm string, docType string) {
	dir := filepath.Dir(savePath)
	docType = filepath.Base(dir)
	parentDir := filepath.Dir(dir)
	npm = filepath.Base(parentDir)
	return npm, docType
}

// DownloadImage downloads an image from the given URL using JavaScript fetch()
// and saves to savePath. No content-type validation — accepts any response.
func DownloadImage(page *rod.Page, url string, savePath string) (string, int, error) {
	if err := os.MkdirAll(filepath.Dir(savePath), 0o755); err != nil {
		return "", 0, fmt.Errorf("create save dir: %w", err)
	}

	data, err := downloadBytes(page, url, "")
	if err != nil {
		return "", 0, fmt.Errorf("download image: %w", err)
	}

	if err := os.WriteFile(savePath, data, 0o644); err != nil {
		return "", 0, fmt.Errorf("save image: %w", err)
	}

	return filepath.Base(savePath), len(data), nil
}
