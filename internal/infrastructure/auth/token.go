package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// SignToken creates a tamper-proof session token by HMAC-SHA256 signing
// a static payload with the secret key. URL-safe base64 for cookie safety.
func SignToken(secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte("eval-session"))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// ValidateToken checks the provided token against the expected HMAC
// using constant-time comparison to prevent timing attacks.
func ValidateToken(secret []byte, token string) bool {
	expected := SignToken(secret)
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}
