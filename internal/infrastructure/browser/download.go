package browser

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-rod/rod"
)

// DownloadAndSave downloads a PDF from the given URL using JavaScript fetch()
// and saves to savePath. Uses fetch() with credentials to include session cookies.
// Returns the filename, byte count, and any error.
//
// Note: We use fetch() instead of <a download> because cross-origin <a download>
// is blocked by browsers for security. fetch() with credentials: "include"
// properly sends session cookies for authenticated URLs.
func DownloadAndSave(page *rod.Page, url string, savePath string) (string, int, error) {
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create save dir: %w", err)
	}

	// Use async fetch() to download the PDF content.
	// go-rod's Eval wraps the code in: function() { return (CODE).apply(this, arguments) }
	// So we pass an async function expression directly — NOT an IIFE.
	// The wrapper calls it with .apply(), and Eval's AwaitPromise waits for the result.
	jsCode := fmt.Sprintf(`async function() {
		const response = await fetch("%s", { credentials: "include" });
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
	}`, url)

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

// DownloadImage downloads an image from the given URL using JavaScript fetch()
// and saves to savePath. Uses fetch() with credentials to include session cookies.
// Returns the filename, byte count, and any error.
func DownloadImage(page *rod.Page, url string, savePath string) (string, int, error) {
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create save dir: %w", err)
	}

	jsCode := fmt.Sprintf(`async function() {
		const response = await fetch("%s", { credentials: "include" });
		if (!response.ok) {
			throw new Error("HTTP " + response.status + " " + response.statusText);
		}
		const buffer = await response.arrayBuffer();
		const bytes = new Uint8Array(buffer);
		let binary = "";
		for (let i = 0; i < bytes.byteLength; i++) {
			binary += String.fromCharCode(bytes[i]);
		}
		return btoa(binary);
	}`, url)

	result, err := page.Eval(jsCode)
	if err != nil {
		return "", 0, fmt.Errorf("fetch image via JS: %w", err)
	}

	imgBody, err := base64.StdEncoding.DecodeString(result.Value.Str())
	if err != nil {
		return "", 0, fmt.Errorf("decode image body: %w", err)
	}

	if len(imgBody) == 0 {
		return "", 0, fmt.Errorf("downloaded image is empty")
	}

	if err := os.WriteFile(savePath, imgBody, 0o644); err != nil {
		return "", 0, fmt.Errorf("save image: %w", err)
	}

	return filepath.Base(savePath), len(imgBody), nil
}
