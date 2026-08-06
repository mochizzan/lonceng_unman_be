package browser

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

// DownloadAndSave downloads a PDF from the given URL using JavaScript fetch(),
// bypassing Chromium's PDF download behavior. Saves to savePath.
// Returns the filename, byte count, and any error.
func DownloadAndSave(page *rod.Page, url string, savePath string, timeout time.Duration) (string, int, error) {
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create save dir: %w", err)
	}

	// Use JavaScript fetch() to download the PDF content.
	// This avoids Chromium's net::ERR_ABORTED when navigating to PDF URLs.
	jsCode := fmt.Sprintf(`
		(async () => {
			const response = await fetch("%s", {
				credentials: 'include'
			});
			if (!response.ok) {
				throw new Error("HTTP " + response.status + " " + response.statusText);
			}
			const contentType = response.headers.get("content-type");
			if (!contentType || !contentType.includes("application/pdf")) {
				throw new Error("Expected PDF but got: " + contentType);
			}
			const buffer = await response.arrayBuffer();
			const bytes = new Uint8Array(buffer);
			let binary = "";
			for (let i = 0; i < bytes.byteLength; i++) {
				binary += String.fromCharCode(bytes[i]);
			}
			return btoa(binary);
		})()
	`, url)

	result, err := page.Eval(jsCode)
	if err != nil {
		return "", 0, fmt.Errorf("fetch PDF via JS: %w", err)
	}

	// Decode base64 to bytes.
	pdfBody, err := base64.StdEncoding.DecodeString(result.Value.Str())
	if err != nil {
		return "", 0, fmt.Errorf("decode PDF body: %w", err)
	}

	if len(pdfBody) == 0 {
		return "", 0, fmt.Errorf("downloaded file is empty")
	}

	if err := os.WriteFile(savePath, pdfBody, 0o644); err != nil {
		return "", 0, fmt.Errorf("save PDF: %w", err)
	}

	return filepath.Base(savePath), len(pdfBody), nil
}

// BuildKHSPath builds the canonical save path for a KHS PDF.
// Pattern: {downloadDir}/{npm}/khs/{tahunAjaran}_{semester}.pdf
func BuildKHSPath(downloadDir, npm, tahunAjaran, semester string) string {
	year := strings.ReplaceAll(tahunAjaran, "/", "_")
	filename := fmt.Sprintf("%s_%s.pdf", year, semester)
	return filepath.Join(downloadDir, npm, "khs", filename)
}
