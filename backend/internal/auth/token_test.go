package auth

import (
	"strings"
	"testing"
	"time"
)

const testJWTSecret = "0123456789abcdef0123456789abcdef"

func TestAccessTokenVerification(t *testing.T) {
	tokens, err := NewTokenManager(testJWTSecret)
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	tokens.now = func() time.Time { return now }

	token, expiresAt, err := tokens.GenerateAccessToken("user-123")
	if err != nil {
		t.Fatalf("GenerateAccessToken returned error: %v", err)
	}
	if !expiresAt.Equal(now.Add(accessTokenTTL)) {
		t.Fatalf("unexpected expiry: %s", expiresAt)
	}

	claims, err := tokens.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("VerifyAccessToken returned error: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Fatalf("unexpected subject: %s", claims.Subject)
	}
}

func TestAccessTokenRejectsTamperingAndExpiry(t *testing.T) {
	tokens, err := NewTokenManager(testJWTSecret)
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	tokens.now = func() time.Time { return now }

	token, _, err := tokens.GenerateAccessToken("user-123")
	if err != nil {
		t.Fatalf("GenerateAccessToken returned error: %v", err)
	}

	tampered := token[:len(token)-1] + "x"
	if _, err := tokens.VerifyAccessToken(tampered); err == nil {
		t.Fatal("expected tampered token to be rejected")
	}

	tokens.now = func() time.Time { return now.Add(accessTokenTTL) }
	if _, err := tokens.VerifyAccessToken(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestGenerateRefreshTokenAndHash(t *testing.T) {
	token, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken returned error: %v", err)
	}
	if strings.TrimSpace(token) == "" {
		t.Fatal("expected refresh token")
	}

	hash := HashRefreshToken(token)
	if len(hash) != 64 {
		t.Fatalf("expected sha256 hex hash length 64, got %d", len(hash))
	}
	if hash == token {
		t.Fatal("expected hashed refresh token to differ from raw token")
	}
}
