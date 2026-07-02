package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
	tokenIssuer     = "dsablitz"
	tokenAudience   = "dsablitz-api"
)

type TokenManager struct {
	secret []byte
	now    func() time.Time
}

type AccessClaims struct {
	Subject   string `json:"sub"`
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}

func NewTokenManager(secret string) (*TokenManager, error) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 characters")
	}

	return &TokenManager{
		secret: []byte(secret),
		now:    time.Now,
	}, nil
}

func (m *TokenManager) AccessTokenTTL() time.Duration {
	return accessTokenTTL
}

func (m *TokenManager) RefreshTokenTTL() time.Duration {
	return refreshTokenTTL
}

func (m *TokenManager) GenerateAccessToken(userID string) (string, time.Time, error) {
	now := m.now().UTC()
	expiresAt := now.Add(accessTokenTTL)
	claims := AccessClaims{
		Subject:   userID,
		Issuer:    tokenIssuer,
		Audience:  tokenAudience,
		ExpiresAt: expiresAt.Unix(),
		IssuedAt:  now.Unix(),
	}

	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("marshal jwt header: %w", err)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("marshal jwt claims: %w", err)
	}

	unsigned := fmt.Sprintf("%s.%s", base64.RawURLEncoding.EncodeToString(headerJSON), base64.RawURLEncoding.EncodeToString(claimsJSON))
	signature := m.sign(unsigned)

	return fmt.Sprintf("%s.%s", unsigned, signature), expiresAt, nil
}

func (m *TokenManager) VerifyAccessToken(token string) (AccessClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return AccessClaims{}, ErrInvalidToken
	}

	unsigned := fmt.Sprintf("%s.%s", parts[0], parts[1])
	expectedSignature := m.sign(unsigned)
	if subtleCompare(expectedSignature, parts[2]) == false {
		return AccessClaims{}, ErrInvalidToken
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return AccessClaims{}, ErrInvalidToken
	}

	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return AccessClaims{}, ErrInvalidToken
	}
	if header.Algorithm != "HS256" || header.Type != "JWT" {
		return AccessClaims{}, ErrInvalidToken
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AccessClaims{}, ErrInvalidToken
	}

	var claims AccessClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return AccessClaims{}, ErrInvalidToken
	}

	if claims.Subject == "" || claims.Issuer != tokenIssuer || claims.Audience != tokenAudience {
		return AccessClaims{}, ErrInvalidToken
	}
	if m.now().UTC().Unix() >= claims.ExpiresAt {
		return AccessClaims{}, ErrInvalidToken
	}

	return claims, nil
}

func GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (m *TokenManager) sign(value string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func subtleCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	return hmac.Equal([]byte(a), []byte(b))
}
