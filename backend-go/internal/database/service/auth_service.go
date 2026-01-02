package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
)

// AuthService defines the interface for authentication business logic
type AuthService interface {
	Register(username, email, fullName, password string) (*models.User, *TokenPair, error)
	Login(email, password string) (*models.User, *TokenPair, error)
	RefreshToken(refreshToken string) (*TokenPair, error)
	Logout(refreshToken string) error
	ValidateAccessToken(tokenString string) (uint, error)
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

type authService struct {
	userRepo         repository.UserRepository
	refreshTokenRepo repository.RefreshTokenRepository
	jwtSecret        string
	cfg              *config.Config
	logger           *slog.Logger
}

// NewAuthService creates a new authentication service instance
func NewAuthService(
	userRepo repository.UserRepository,
	refreshTokenRepo repository.RefreshTokenRepository,
	cfg *config.Config,
	logger *slog.Logger,
) AuthService {
	return &authService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		jwtSecret:        cfg.JWTSecret,
		cfg:              cfg,
		logger:           logger,
	}
}

func (s *authService) Register(username, email, fullName, password string) (*models.User, *TokenPair, error) {
	s.logger.Info("📝 [AuthService] Registration attempt", "email", email, "username", username)

	// Check if email already exists
	existingUser, err := s.userRepo.FindByEmail(email)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		s.logger.Error("❌ [AuthService] Database error", "error", err)
		return nil, nil, err
	}

	if existingUser != nil {
		s.logger.Warn("⚠️ [AuthService] Email already registered", "email", email)
		return nil, nil, ErrEmailAlreadyExists
	}

	// Check if username already exists
	existingUser, err = s.userRepo.FindByUsername(username)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		s.logger.Error("❌ [AuthService] Database error checking username", "error", err)
		return nil, nil, err
	}

	if existingUser != nil {
		s.logger.Warn("⚠️ [AuthService] Username already taken", "username", username)
		return nil, nil, errors.New("username already taken")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("❌ [AuthService] Failed to hash password", "error", err)
		return nil, nil, err
	}

	// Create user
	user := &models.User{
		Username: username,
		Email:    email,
		FullName: fullName,
		Password: string(hashedPassword),
	}

	if err := s.userRepo.Create(user); err != nil {
		s.logger.Error("❌ [AuthService] Failed to create user", "error", err)
		return nil, nil, err
	}

	// Generate tokens
	tokens, err := s.generateTokenPair(user.ID)
	if err != nil {
		s.logger.Error("❌ [AuthService] Failed to generate tokens", "error", err)
		return nil, nil, err
	}

	s.logger.Info("✅ [AuthService] User registered successfully", "user_id", user.ID)
	return user, tokens, nil
}

func (s *authService) Login(email, password string) (*models.User, *TokenPair, error) {
	s.logger.Info("🔐 [AuthService] Login attempt", "email", email)

	// Find user
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			s.logger.Warn("⚠️ [AuthService] User not found", "email", email)
			return nil, nil, ErrInvalidCredentials
		}
		s.logger.Error("❌ [AuthService] Database error", "error", err)
		return nil, nil, err
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		s.logger.Warn("⚠️ [AuthService] Invalid password", "email", email)
		return nil, nil, ErrInvalidCredentials
	}

	// Generate tokens
	tokens, err := s.generateTokenPair(user.ID)
	if err != nil {
		s.logger.Error("❌ [AuthService] Failed to generate tokens", "error", err)
		return nil, nil, err
	}

	s.logger.Info("✅ [AuthService] User logged in successfully", "user_id", user.ID)
	return user, tokens, nil
}

func (s *authService) RefreshToken(refreshToken string) (*TokenPair, error) {
	s.logger.Info("🔄 [AuthService] Token refresh attempt")

	// Find refresh token
	storedToken, err := s.refreshTokenRepo.FindByToken(refreshToken)
	if err != nil {
		s.logger.Warn("⚠️ [AuthService] Invalid refresh token", "error", err)
		return nil, ErrInvalidToken
	}

	// Generate new token pair
	tokens, err := s.generateTokenPair(storedToken.UserID)
	if err != nil {
		s.logger.Error("❌ [AuthService] Failed to generate new tokens", "error", err)
		return nil, err
	}

	// Revoke old refresh token (token rotation)
	if err := s.refreshTokenRepo.RevokeToken(refreshToken); err != nil {
		s.logger.Error("❌ [AuthService] Failed to revoke old token", "error", err)
	}

	s.logger.Info("✅ [AuthService] Token refreshed successfully", "user_id", storedToken.UserID)
	return tokens, nil
}

func (s *authService) Logout(refreshToken string) error {
	s.logger.Info("👋 [AuthService] Logout attempt")

	if err := s.refreshTokenRepo.RevokeToken(refreshToken); err != nil {
		if errors.Is(err, repository.ErrTokenNotFound) {
			s.logger.Warn("⚠️ [AuthService] Token not found for logout")
			return repository.ErrTokenNotFound
		}
		return err
	}

	s.logger.Info("✅ [AuthService] User logged out successfully")
	return nil
}

func (s *authService) ValidateAccessToken(tokenString string) (uint, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return 0, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, ErrInvalidToken
	}

	userID, ok := claims["user_id"].(float64)
	if !ok {
		return 0, ErrInvalidToken
	}

	return uint(userID), nil
}

// generateTokenPair creates both access and refresh tokens
func (s *authService) generateTokenPair(userID uint) (*TokenPair, error) {
	// Generate access token
	accessToken, err := s.generateAccessToken(userID)
	if err != nil {
		return nil, err
	}

	// Generate refresh token
	refreshToken, err := s.generateAndStoreRefreshToken(userID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.cfg.AccessTokenExpiration,
	}, nil
}

func (s *authService) generateAccessToken(userID uint) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"type":    "access",
		"exp":     time.Now().Add(time.Duration(time.Duration(s.cfg.AccessTokenExpiration) * time.Second)).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *authService) generateAndStoreRefreshToken(userID uint) (string, error) {
	// Generate cryptographically secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	tokenString := base64.URLEncoding.EncodeToString(tokenBytes)

	// Store in database
	refreshToken := &models.RefreshToken{
		UserID:    userID,
		Token:     tokenString,
		ExpiresAt: time.Now().Add(time.Duration(time.Duration(s.cfg.RefreshTokenExpiration) * time.Second)),
		IsRevoked: false,
	}

	if err := s.refreshTokenRepo.Create(refreshToken); err != nil {
		return "", err
	}

	return tokenString, nil
}

// Service errors
var (
	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
)
