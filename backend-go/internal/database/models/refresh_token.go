package models

import (
	"time"

	"gorm.io/gorm"
)

// RefreshToken stores refresh tokens for authentication
type RefreshToken struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	Token     string         `gorm:"uniqueIndex;not null" json:"token"`
	ExpiresAt time.Time      `gorm:"not null" json:"expires_at"`
	IsRevoked bool           `gorm:"default:false" json:"is_revoked"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	User      User           `gorm:"foreignKey:UserID" json:"-"`
}

// TableName overrides the table name
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
