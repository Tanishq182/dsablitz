package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	repository *Repository
	tokens     *TokenManager
}

type ClientInfo struct {
	UserAgent string
	IPAddress string
}

type AuthResult struct {
	User         UserResponse
	AccessToken  string
	RefreshToken string
}

func NewService(repository *Repository, tokens *TokenManager) *Service {
	return &Service{
		repository: repository,
		tokens:     tokens,
	}
}

func (s *Service) Signup(ctx context.Context, request SignupRequest, client ClientInfo) (AuthResult, error) {
	passwordHash, err := HashPassword(request.Password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repository.CreateUser(ctx, CreateUserParams{
		Email:        normalizeEmail(request.Email),
		PasswordHash: passwordHash,
		Handle:       strings.TrimSpace(request.Handle),
		DisplayName:  strings.TrimSpace(request.DisplayName),
	})
	if err != nil {
		return AuthResult{}, err
	}

	return s.issueTokens(ctx, user, client)
}

func (s *Service) Login(ctx context.Context, request LoginRequest, client ClientInfo) (AuthResult, error) {
	user, err := s.repository.FindUserByEmail(ctx, normalizeEmail(request.Email))
	if err != nil {
		return AuthResult{}, err
	}
	if user.Status != "active" {
		return AuthResult{}, ErrUserDisabled
	}
	if user.PasswordHash == nil {
		return AuthResult{}, ErrInvalidCredentials
	}

	matched, err := VerifyPassword(request.Password, *user.PasswordHash)
	if err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}
	if !matched {
		return AuthResult{}, ErrInvalidCredentials
	}

	if err := s.repository.UpdateLastLogin(ctx, user.ID); err != nil {
		return AuthResult{}, err
	}

	return s.issueTokens(ctx, user, client)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string, client ClientInfo) (AuthResult, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return AuthResult{}, ErrUnauthorized
	}

	oldRefreshTokenHash := HashRefreshToken(refreshToken)
	session, err := s.repository.FindActiveSessionByHash(ctx, oldRefreshTokenHash)
	if err != nil {
		return AuthResult{}, err
	}

	user, err := s.repository.FindUserByID(ctx, session.UserID)
	if err != nil {
		return AuthResult{}, err
	}
	if user.Status != "active" {
		return AuthResult{}, ErrUserDisabled
	}

	newRefreshToken, err := GenerateRefreshToken()
	if err != nil {
		return AuthResult{}, err
	}

	_, err = s.repository.RotateSession(ctx, oldRefreshTokenHash, CreateSessionParams{
		UserID:           user.ID,
		RefreshTokenHash: HashRefreshToken(newRefreshToken),
		UserAgent:        client.UserAgent,
		IPAddress:        client.IPAddress,
		ExpiresAt:        time.Now().UTC().Add(s.tokens.RefreshTokenTTL()),
	})
	if err != nil {
		return AuthResult{}, err
	}

	accessToken, _, err := s.tokens.GenerateAccessToken(user.ID)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		User:         toUserResponse(user),
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return nil
	}

	return s.repository.RevokeSessionByHash(ctx, HashRefreshToken(refreshToken))
}

func (s *Service) Me(ctx context.Context, userID string) (UserResponse, error) {
	user, err := s.repository.FindUserByID(ctx, userID)
	if err != nil {
		return UserResponse{}, err
	}
	if user.Status != "active" {
		return UserResponse{}, ErrUserDisabled
	}

	return toUserResponse(user), nil
}

func (s *Service) issueTokens(ctx context.Context, user User, client ClientInfo) (AuthResult, error) {
	if user.Status != "active" {
		return AuthResult{}, ErrUserDisabled
	}

	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return AuthResult{}, err
	}

	if _, err := s.repository.CreateSession(ctx, CreateSessionParams{
		UserID:           user.ID,
		RefreshTokenHash: HashRefreshToken(refreshToken),
		UserAgent:        client.UserAgent,
		IPAddress:        client.IPAddress,
		ExpiresAt:        time.Now().UTC().Add(s.tokens.RefreshTokenTTL()),
	}); err != nil {
		return AuthResult{}, err
	}

	accessToken, _, err := s.tokens.GenerateAccessToken(user.ID)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		User:         toUserResponse(user),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func toUserResponse(user User) UserResponse {
	return UserResponse{
		ID:          user.ID,
		Email:       user.Email,
		Handle:      user.Handle,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Status:      user.Status,
		LastLoginAt: user.LastLoginAt,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func IsClientAuthError(err error) bool {
	return errors.Is(err, ErrInvalidCredentials) ||
		errors.Is(err, ErrEmailTaken) ||
		errors.Is(err, ErrHandleTaken) ||
		errors.Is(err, ErrUnauthorized) ||
		errors.Is(err, ErrInvalidToken) ||
		errors.Is(err, ErrUserDisabled)
}
