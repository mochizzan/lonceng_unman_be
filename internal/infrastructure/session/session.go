package session

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"lonceng_unman_be/internal/infrastructure/browser"

	"github.com/go-rod/rod"
)

// rodSession implements port.BrowserSession using a go-rod page.
type rodSession struct {
	page *rod.Page
}

// newSession wraps a rod.Page into a BrowserSession.
func newSession(page *rod.Page) *rodSession {
	return &rodSession{page: page}
}

// Navigate loads the given URL and waits for the page to be ready.
func (s *rodSession) Navigate(url string) error {
	if err := s.page.Navigate(url); err != nil {
		return fmt.Errorf("navigate to %s: %w", url, err)
	}
	if err := s.page.WaitLoad(); err != nil {
		return fmt.Errorf("wait load %s: %w", url, err)
	}
	return nil
}

// Eval executes JavaScript on the page and returns the result as a string.
func (s *rodSession) Eval(js string) (string, error) {
	result, err := s.page.Eval(js)
	if err != nil {
		return "", fmt.Errorf("eval js: %w", err)
	}
	return result.Value.Str(), nil
}

// ElementAttribute returns the value of an attribute on the first element matching the selector.
func (s *rodSession) ElementAttribute(selector, attr string) (string, error) {
	el, err := s.page.Element(selector)
	if err != nil {
		return "", fmt.Errorf("find element %s: %w", selector, err)
	}
	val, err := el.Attribute(attr)
	if err != nil {
		return "", fmt.Errorf("get attribute %s: %w", attr, err)
	}
	if val == nil {
		return "", fmt.Errorf("attribute %s is nil on %s", attr, selector)
	}
	return *val, nil
}

// ElementExists returns true if an element matching the selector exists on the page.
func (s *rodSession) ElementExists(selector string) (bool, error) {
	_, err := s.page.Element(selector)
	if err != nil {
		// go-rod returns error when element not found
		return false, nil
	}
	return true, nil
}

// ElementHref returns the href attribute of the first element matching the selector.
func (s *rodSession) ElementHref(selector string) (string, error) {
	return s.ElementAttribute(selector, "href")
}

// DownloadPDF downloads a PDF from the given URL using JavaScript fetch()
// and saves it to savePath. Returns the filename and byte count.
func (s *rodSession) DownloadPDF(url, savePath string) (string, int, error) {
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create save dir: %w", err)
	}

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

	result, err := s.page.Eval(jsCode)
	if err != nil {
		return "", 0, fmt.Errorf("fetch PDF via JS: %w", err)
	}

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

// Close releases the page resources. The browser is managed by the session manager.
func (s *rodSession) Close() error {
	// Page closure is handled by the session manager via browser.Close().
	// This is intentionally a no-op to satisfy the interface.
	return nil
}

// Ensure rodSession implements the expected interface at compile time.
var _ interface {
	Navigate(string) error
	Eval(string) (string, error)
	ElementAttribute(string, string) (string, error)
	ElementExists(string) (bool, error)
	ElementHref(string) (string, error)
	DownloadPDF(string, string) (string, int, error)
	Close() error
} = (*rodSession)(nil)

// Ensure selectors are used (prevents import cycle if browser package changes).
var _ = browser.SelUsernameInput
