package auth_test

import (
	"testing"

	"lonceng_unman_be/internal/infrastructure/auth"
)

func TestSignToken_Deterministic(t *testing.T) {
	secret := []byte("test-secret")
	token1 := auth.SignToken(secret)
	token2 := auth.SignToken(secret)
	if token1 != token2 {
		t.Errorf("SignToken not deterministic: %q != %q", token1, token2)
	}
}

func TestSignToken_URLSafeBase64(t *testing.T) {
	secret := []byte("test-secret")
	token := auth.SignToken(secret)
	for _, c := range token {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			t.Errorf("token contains non-URL-safe character: %c in %q", c, token)
		}
	}
}

func TestValidateToken_Valid(t *testing.T) {
	secret := []byte("test-secret")
	token := auth.SignToken(secret)
	if !auth.ValidateToken(secret, token) {
		t.Error("ValidateToken should accept valid token")
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	secret := []byte("test-secret")
	if auth.ValidateToken(secret, "wrong-token") {
		t.Error("ValidateToken should reject wrong token")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	token := auth.SignToken([]byte("secret-1"))
	if auth.ValidateToken([]byte("secret-2"), token) {
		t.Error("ValidateToken should reject token from different secret")
	}
}

func TestValidateToken_EmptyToken(t *testing.T) {
	secret := []byte("test-secret")
	if auth.ValidateToken(secret, "") {
		t.Error("ValidateToken should reject empty token")
	}
}
