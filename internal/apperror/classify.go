package apperror

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// infraKeywords are substrings that indicate an LMS infrastructure failure
// (browser/CDP/DNS/network). Matched case-insensitively against err.Error().
// Reuses the same keywords as lms_service.loginErrorKeyword infrastructure set
// so document handlers and extraction verifySession stay consistent with Login.
var infraKeywords = []string{
	"dns check",
	"cannot resolve lms host",
	"browser connect",
	"launch browser",
	"connect browser",
	"open login page",
	"open page",
	"wait login page load",
	"create new page",
	"create profile dir",
	"navigate to dashboard",
	"wait dashboard load",
	"too many open pages",
	"context deadline exceeded",
	"timeout",
	"eof",
	"connection refused",
	"no such host",
	"network is unreachable",
}

// credentialKeywords are substrings that indicate the LMS rejected the
// credentials (login page rendered the error indicator or redirect check
// failed). Matched case-insensitively.
//
// SECURITY: match ONLY combined phrases ("username dan password",
// "login gagal"), never "password salah" alone — the system must not
// expose which credential field is wrong.
var credentialKeywords = []string{
	"username dan password",
	"login gagal",
	"page did not redirect to dashboard",
	"login timed out: no response detected",
	"session expired",
}

// IsInfrastructureError reports whether err looks like a transient
// browser/CDP/DNS/network failure worth retrying.
func IsInfrastructureError(err error) bool {
	if err == nil {
		return false
	}
	low := strings.ToLower(err.Error())
	for _, kw := range infraKeywords {
		if strings.Contains(low, kw) {
			return true
		}
	}
	return false
}

// IsCredentialError reports whether err looks like the LMS rejected the
// credentials (combined phrase only, no field disclosure).
func IsCredentialError(err error) bool {
	if err == nil {
		return false
	}
	low := strings.ToLower(err.Error())
	for _, kw := range credentialKeywords {
		if strings.Contains(low, kw) {
			return true
		}
	}
	return false
}

// ClassifyDocumentError converts a document-service error into an AppError
// with the correct HTTP status. Priority:
//
//  1. err already is *AppError (e.g. BadRequest from validation) → pass through.
//  2. infrastructure failure → 503 with the same public message as Login.
//  3. credential failure → 401 Username atau password salah.
//  4. fallback → 500 with the provided fallbackMsg (or "KHS download failed").
//
// Errors.As check MUST come first so a 400/404 AppError whose message
// happens to contain "timeout" is not reclassified as 503.
func ClassifyDocumentError(err error, fallbackMsg string) *AppError {
	if err == nil {
		return Internal("internal error: ClassifyDocumentError called with nil error", fmt.Errorf("nil error"))
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	if IsInfrastructureError(err) {
		return &AppError{
			StatusCode: fiber.StatusServiceUnavailable,
			PublicMsg:  "Layanan LMS tidak dapat diakses saat ini. Silakan coba lagi dalam beberapa saat.",
			Internal:   err,
		}
	}
	if IsCredentialError(err) {
		return Unauthorized("Username atau password salah")
	}
	if fallbackMsg == "" {
		fallbackMsg = "KHS download failed"
	}
	return Internal(fallbackMsg, err)
}
