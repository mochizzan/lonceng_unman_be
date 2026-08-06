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

// DownloadAndSave navigates to a URL, intercepts the PDF response via CDP Fetch domain,
// and saves it to savePath. Returns the filename, byte count, and any error.
// If savePath already exists it is overwritten.
// Uses Fetch.enable to intercept PDF responses before Chromium aborts navigation.
func DownloadAndSave(page *rod.Page, url string, savePath string, timeout time.Duration) (string, int, error) {
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create save dir: %w", err)
	}

	// Enable Fetch domain to intercept PDF responses.
	enableParams := proto.FetchEnable{
		Patterns: []*proto.FetchRequestPattern{{
			RequestStage: proto.FetchRequestStageResponse,
		}},
	}
	if err := enableParams.Call(page); err != nil {
		return "", 0, fmt.Errorf("enable fetch domain: %w", err)
	}

	var (
		pdfBody    []byte
		pdfReady   = make(chan bool, 1)
		fetchError error
	)

	// Listen for Fetch.requestPaused events.
	// When a PDF response is intercepted, retrieve its body and fulfill the request.
	go page.EachEvent(
		func(e *proto.FetchRequestPaused) {
			// Check if this is a successful response.
			if e.ResponseStatusCode == nil || *e.ResponseStatusCode != 200 {
				contReq := proto.FetchContinueRequest{RequestID: e.RequestID}
				if err := contReq.Call(page); err != nil {
					fetchError = fmt.Errorf("continue request: %w", err)
				}
				return
			}

			// Check content-type header for PDF.
			isPDF := false
			if e.ResponseHeaders != nil {
				for _, h := range e.ResponseHeaders {
					if h.Name == "content-type" && strings.Contains(h.Value, "application/pdf") {
						isPDF = true
						break
					}
				}
			}

			if !isPDF {
				// Not a PDF, continue normally.
				contReq := proto.FetchContinueRequest{RequestID: e.RequestID}
				if err := contReq.Call(page); err != nil {
					fetchError = fmt.Errorf("continue request: %w", err)
				}
				return
			}

			// It's a PDF — get the response body before fulfilling.
			bodyReq := proto.FetchGetResponseBody{RequestID: e.RequestID}
			res, err := bodyReq.Call(page)
			if err != nil {
				fetchError = fmt.Errorf("get fetch response body: %w", err)
				// Fail the request so browser doesn't hang.
				failReq := proto.FetchFailRequest{RequestID: e.RequestID}
				_ = failReq.Call(page)
				pdfReady <- true
				return
			}

			// Decode the body.
			if res.Base64Encoded {
				pdfBody, err = base64.StdEncoding.DecodeString(res.Body)
			} else {
				pdfBody = []byte(res.Body)
			}
			if err != nil {
				fetchError = fmt.Errorf("decode PDF body: %w", err)
			}

			// Fulfill the request with the original response.
			fulfillReq := proto.FetchFulfillRequest{
				RequestID:       e.RequestID,
				ResponseCode:    200,
				ResponseHeaders: e.ResponseHeaders,
				Body:            pdfBody,
			}
			if err := fulfillReq.Call(page); err != nil {
				fetchError = fmt.Errorf("fulfill request: %w", err)
			}

			pdfReady <- true
		},
	)()

	// Navigate to the PDF URL.
	if err := page.Navigate(url); err != nil {
		return "", 0, fmt.Errorf("navigate to PDF URL: %w", err)
	}

	// Wait for the PDF body to be captured, or time out.
	select {
	case <-pdfReady:
	case <-time.After(timeout):
		return "", 0, fmt.Errorf("download timed out after %v", timeout)
	}

	// Disable Fetch domain.
	_ = proto.FetchDisable{}.Call(page)

	if fetchError != nil {
		return "", 0, fetchError
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
