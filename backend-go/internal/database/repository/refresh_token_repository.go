package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
)

// RefreshTokenRepository defines the interface for refresh token operations
type RefreshTokenRepository interface {
	Create(token *models.RefreshToken) error
	FindByToken(token string) (*models.RefreshToken, error)
	RevokeToken(token string) error
	RevokeAllUserTokens(userID uint) error
	DeleteExpiredTokens() error
}

type refreshTokenRepository struct {
	db *gorm.DB
}

// NewRefreshTokenRepository creates a new refresh token repository instance
func NewRefreshTokenRepository(db *gorm.DB) RefreshTokenRepository {
	return &refreshTokenRepository{db: db}
}

func (r *refreshTokenRepository) Create(token *models.RefreshToken) error {
	return r.db.Create(token).Error
}

func (r *refreshTokenRepository) FindByToken(token string) (*models.RefreshToken, error) {
	var refreshToken models.RefreshToken
	err := r.db.Where("token = ? AND is_revoked = false", token).
		Preload("User").
		First(&refreshToken).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	// Check if expired
	if time.Now().After(refreshToken.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	return &refreshToken, nil
}

func (r *refreshTokenRepository) RevokeToken(token string) error {
	result := r.db.Model(&models.RefreshToken{}).
		Where("token = ?", token).
		Update("is_revoked", true)

	if result.RowsAffected == 0 {
		return ErrTokenNotFound
	}

	return result.Error
}

func (r *refreshTokenRepository) RevokeAllUserTokens(userID uint) error {
	return r.db.Model(&models.RefreshToken{}).
		Where("user_id = ?", userID).
		Update("is_revoked", true).Error
}

func (r *refreshTokenRepository) DeleteExpiredTokens() error {
	return r.db.Where("expires_at < ?", time.Now()).
		Delete(&models.RefreshToken{}).Error
}

// Repository errors
var (
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenExpired  = errors.New("token expired")
)
