package browser

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-rod/rod"
)

// DownloadAndSave downloads a PDF from the given URL using go-rod's native
// download mechanism (CDP Browser.setDownloadBehavior) and saves to savePath.
// Returns the filename, byte count, and any error.
func DownloadAndSave(page *rod.Page, url string, savePath string) (string, int, error) {
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create save dir: %w", err)
	}

	// Set up native download listener before triggering the download.
	// go-rod saves the file as filepath.Join(dir, info.GUID).
	wait := page.Browser().WaitDownload(dir)

	// Trigger the download via a hidden <a download> click.
	// This preserves the page's session cookies for authenticated URLs.
	jsCode := fmt.Sprintf(`async function() {
		const a = document.createElement('a');
		a.href = %q;
		a.download = '';
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
	}`, url)

	if _, err := page.Eval(jsCode); err != nil {
		return "", 0, fmt.Errorf("trigger download: %w", err)
	}

	// Block until the browser finishes writing the file to disk.
	info := wait()

	downloadedPath := filepath.Join(dir, info.GUID)

	// Move to desired path; fall back to copy if rename fails (cross-device).
	if err := os.Rename(downloadedPath, savePath); err != nil {
		if err := copyFile(downloadedPath, savePath); err != nil {
			return "", 0, err
		}
		os.Remove(downloadedPath)
	}

	fileInfo, err := os.Stat(savePath)
	if err != nil {
		return "", 0, fmt.Errorf("stat file: %w", err)
	}

	return filepath.Base(savePath), int(fileInfo.Size()), nil
}

// copyFile copies src to dst. Used as a fallback when os.Rename fails
// (e.g. across filesystem boundaries).
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}
	return nil
}

// BuildKHSPath builds the canonical save path for a KHS PDF.
// Pattern: {downloadDir}/{npm}/khs/{tahunAjaran}_{semester}.pdf
func BuildKHSPath(downloadDir, npm, tahunAjaran, semester string) string {
	year := strings.ReplaceAll(tahunAjaran, "/", "_")
	filename := fmt.Sprintf("%s_%s.pdf", year, semester)
	return filepath.Join(downloadDir, npm, "khs", filename)
}
