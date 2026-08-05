package browser

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// DownloadAndSave navigates to a URL, captures the PDF response body via CDP,
// and saves it to savePath. Returns the filename, byte count, and any error.
// If savePath already exists it is overwritten.
func DownloadAndSave(page *rod.Page, url string, savePath string, timeout time.Duration) (string, int, error) {
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create save dir: %w", err)
	}

	// Enable Network domain so response/data-received events fire.
	enableReq := proto.NetworkEnable{}
	if err := enableReq.Call(page); err != nil {
		return "", 0, fmt.Errorf("enable network: %w", err)
	}

	var (
		pdfRequestID proto.NetworkRequestID
		dataReady    = make(chan bool, 1)
	)

	// Listen for Network.responseReceived to capture the PDF request ID,
	// and Network.dataReceived to know when the body is ready to retrieve.
	go page.EachEvent(
		func(e *proto.NetworkResponseReceived) {
			if e.Response == nil || e.Response.Headers == nil {
				return
			}
			if ct, ok := e.Response.Headers["content-type"]; ok {
				if strings.Contains(ct.Val().(string), "application/pdf") {
					pdfRequestID = e.RequestID
				}
			}
		},
		func(e *proto.NetworkDataReceived) {
			if pdfRequestID != "" && e.RequestID == pdfRequestID {
				select {
				case dataReady <- true:
				default:
				}
			}
		},
	)()

	// Navigate to the PDF URL — returns after HTTP headers arrive.
	if err := page.Navigate(url); err != nil {
		return "", 0, fmt.Errorf("navigate to PDF URL: %w", err)
	}

	// Wait for the response body to be buffered, or time out.
	select {
	case <-dataReady:
	case <-time.After(timeout):
		return "", 0, fmt.Errorf("download timed out after %v", timeout)
	}

	// Retrieve the full response body via CDP.
	bodyReq := proto.NetworkGetResponseBody{RequestID: pdfRequestID}
	res, err := bodyReq.Call(page)
	if err != nil {
		return "", 0, fmt.Errorf("get response body: %w", err)
	}

	var body []byte
	if res.Base64Encoded {
		body, err = base64.StdEncoding.DecodeString(res.Body)
	} else {
		body = []byte(res.Body)
	}
	if err != nil {
		return "", 0, fmt.Errorf("decode response body: %w", err)
	}
	if len(body) == 0 {
		return "", 0, fmt.Errorf("downloaded file is empty")
	}

	if err := os.WriteFile(savePath, body, 0o644); err != nil {
		return "", 0, fmt.Errorf("save PDF: %w", err)
	}

	return filepath.Base(savePath), len(body), nil
}

// BuildKHSPath builds the canonical save path for a KHS PDF.
// Pattern: {downloadDir}/{npm}/khs/{tahunAjaran}_{semester}.pdf
func BuildKHSPath(downloadDir, npm, tahunAjaran, semester string) string {
	year := strings.ReplaceAll(tahunAjaran, "/", "_")
	filename := fmt.Sprintf("%s_%s.pdf", year, semester)
	return filepath.Join(downloadDir, npm, "khs", filename)
}
