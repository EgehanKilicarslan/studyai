package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
)

// setupTestDB creates a new in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.User{}, &models.RefreshToken{})
	require.NoError(t, err)

	return db
}

// ==================== USER REPOSITORY TESTS ====================

func TestUserRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewUserRepository(db)

	tests := []struct {
		name    string
		user    *models.User
		wantErr bool
	}{
		{
			name: "success",
			user: &models.User{
				Email:    "test@example.com",
				Password: "hashedpassword",
			},
			wantErr: false,
		},
		{
			name: "duplicate email",
			user: &models.User{
				Email:    "test@example.com",
				Password: "hashedpassword",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(tt.user)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotZero(t, tt.user.ID)
			}
		})
	}
}

func TestUserRepository_FindByEmail(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewUserRepository(db)

	// Create test user
	testUser := &models.User{
		Email:    "find@example.com",
		Password: "hashedpassword",
	}
	require.NoError(t, repo.Create(testUser))

	tests := []struct {
		name      string
		email     string
		wantErr   error
		wantEmail string
	}{
		{
			name:      "found",
			email:     "find@example.com",
			wantEmail: "find@example.com",
		},
		{
			name:    "not found",
			email:   "nonexistent@example.com",
			wantErr: repository.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.FindByEmail(tt.email)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantEmail, user.Email)
			}
		})
	}
}

func TestUserRepository_FindByID(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewUserRepository(db)

	// Create test user
	testUser := &models.User{
		Email:    "findbyid@example.com",
		Password: "hashedpassword",
	}
	require.NoError(t, repo.Create(testUser))

	tests := []struct {
		name    string
		id      uint
		wantErr error
	}{
		{
			name: "found",
			id:   testUser.ID,
		},
		{
			name:    "not found",
			id:      99999,
			wantErr: repository.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.FindByID(tt.id)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.id, user.ID)
			}
		})
	}
}

func TestUserRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewUserRepository(db)

	// Create test user
	testUser := &models.User{
		Email:    "update@example.com",
		Password: "hashedpassword",
	}
	require.NoError(t, repo.Create(testUser))

	t.Run("success", func(t *testing.T) {
		testUser.Email = "updated@example.com"
		err := repo.Update(testUser)
		assert.NoError(t, err)

		// Verify update
		updated, err := repo.FindByID(testUser.ID)
		require.NoError(t, err)
		assert.Equal(t, "updated@example.com", updated.Email)
	})
}

func TestUserRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewUserRepository(db)

	// Create test user
	testUser := &models.User{
		Email:    "delete@example.com",
		Password: "hashedpassword",
	}
	require.NoError(t, repo.Create(testUser))

	t.Run("success", func(t *testing.T) {
		err := repo.Delete(testUser.ID)
		assert.NoError(t, err)

		// Verify soft delete (GORM uses soft delete by default with DeletedAt)
		_, err = repo.FindByID(testUser.ID)
		assert.ErrorIs(t, err, repository.ErrUserNotFound)
	})

	t.Run("delete non-existent", func(t *testing.T) {
		err := repo.Delete(99999)
		// GORM doesn't return error for deleting non-existent soft-delete records
		assert.NoError(t, err)
	})
}

// ==================== REFRESH TOKEN REPOSITORY TESTS ====================

func TestRefreshTokenRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)

	// Create test user first
	testUser := &models.User{
		Email:    "tokenuser@example.com",
		Password: "hashedpassword",
	}
	require.NoError(t, userRepo.Create(testUser))

	t.Run("success", func(t *testing.T) {
		token := &models.RefreshToken{
			UserID:    testUser.ID,
			Token:     "test-refresh-token",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			IsRevoked: false,
		}
		err := tokenRepo.Create(token)
		assert.NoError(t, err)
		assert.NotZero(t, token.ID)
	})

	t.Run("duplicate token", func(t *testing.T) {
		token := &models.RefreshToken{
			UserID:    testUser.ID,
			Token:     "test-refresh-token", // Same token as above
			ExpiresAt: time.Now().Add(24 * time.Hour),
			IsRevoked: false,
		}
		err := tokenRepo.Create(token)
		assert.Error(t, err) // Should fail due to unique constraint
	})
}

func TestRefreshTokenRepository_FindByToken(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)

	// Create test user
	testUser := &models.User{
		Email:    "findtoken@example.com",
		Password: "hashedpassword",
	}
	require.NoError(t, userRepo.Create(testUser))

	// Create valid token
	validToken := &models.RefreshToken{
		UserID:    testUser.ID,
		Token:     "valid-token",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		IsRevoked: false,
	}
	require.NoError(t, tokenRepo.Create(validToken))

	// Create expired token
	expiredToken := &models.RefreshToken{
		UserID:    testUser.ID,
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-24 * time.Hour), // Expired
		IsRevoked: false,
	}
	require.NoError(t, tokenRepo.Create(expiredToken))

	// Create revoked token
	revokedToken := &models.RefreshToken{
		UserID:    testUser.ID,
		Token:     "revoked-token",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		IsRevoked: true,
	}
	require.NoError(t, tokenRepo.Create(revokedToken))

	tests := []struct {
		name    string
		token   string
		wantErr error
	}{
		{
			name:  "valid token",
			token: "valid-token",
		},
		{
			name:    "not found",
			token:   "nonexistent-token",
			wantErr: repository.ErrTokenNotFound,
		},
		{
			name:    "expired token",
			token:   "expired-token",
			wantErr: repository.ErrTokenExpired,
		},
		{
			name:    "revoked token",
			token:   "revoked-token",
			wantErr: repository.ErrTokenNotFound, // Revoked tokens are filtered by query
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := tokenRepo.FindByToken(tt.token)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, found)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.token, found.Token)
			}
		})
	}
}

func TestRefreshTokenRepository_RevokeToken(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)

	// Create test user
	testUser := &models.User{
		Email:    "revoketoken@example.com",
		Password: "hashedpassword",
	}
	require.NoError(t, userRepo.Create(testUser))

	// Create token to revoke
	token := &models.RefreshToken{
		UserID:    testUser.ID,
		Token:     "token-to-revoke",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		IsRevoked: false,
	}
	require.NoError(t, tokenRepo.Create(token))

	tests := []struct {
		name    string
		token   string
		wantErr error
	}{
		{
			name:  "success",
			token: "token-to-revoke",
		},
		{
			name:    "not found",
			token:   "nonexistent-token",
			wantErr: repository.ErrTokenNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tokenRepo.RevokeToken(tt.token)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)

				// Verify token is revoked (should not be found)
				_, err = tokenRepo.FindByToken(tt.token)
				assert.Error(t, err)
			}
		})
	}
}

func TestRefreshTokenRepository_RevokeAllUserTokens(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)

	// Create test user
	testUser := &models.User{
		Email:    "revokeall@example.com",
		Password: "hashedpassword",
	}
	require.NoError(t, userRepo.Create(testUser))

	// Create multiple tokens for user
	for i := 0; i < 3; i++ {
		token := &models.RefreshToken{
			UserID:    testUser.ID,
			Token:     "user-token-" + string(rune('a'+i)),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			IsRevoked: false,
		}
		require.NoError(t, tokenRepo.Create(token))
	}

	t.Run("revoke all", func(t *testing.T) {
		err := tokenRepo.RevokeAllUserTokens(testUser.ID)
		assert.NoError(t, err)

		// Verify all tokens are revoked
		for i := 0; i < 3; i++ {
			_, err := tokenRepo.FindByToken("user-token-" + string(rune('a'+i)))
			assert.Error(t, err)
		}
	})
}

func TestRefreshTokenRepository_DeleteExpiredTokens(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)

	// Create test user
	testUser := &models.User{
		Email:    "deleteexpired@example.com",
		Password: "hashedpassword",
	}
	require.NoError(t, userRepo.Create(testUser))

	// Create expired token
	expiredToken := &models.RefreshToken{
		UserID:    testUser.ID,
		Token:     "expired-to-delete",
		ExpiresAt: time.Now().Add(-24 * time.Hour),
		IsRevoked: false,
	}
	require.NoError(t, tokenRepo.Create(expiredToken))

	// Create valid token
	validToken := &models.RefreshToken{
		UserID:    testUser.ID,
		Token:     "valid-to-keep",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		IsRevoked: false,
	}
	require.NoError(t, tokenRepo.Create(validToken))

	t.Run("delete expired", func(t *testing.T) {
		err := tokenRepo.DeleteExpiredTokens()
		assert.NoError(t, err)

		// Valid token should still exist
		found, err := tokenRepo.FindByToken("valid-to-keep")
		assert.NoError(t, err)
		assert.NotNil(t, found)
	})
}

// ==================== DATABASE PACKAGE TESTS ====================

func TestGetDatabase(t *testing.T) {
	t.Run("returns nil when not initialized", func(t *testing.T) {
		// Save original value
		originalDB := database.DATABASE
		database.DATABASE = nil

		db := database.GetDatabase()
		assert.Nil(t, db)

		// Restore original value
		database.DATABASE = originalDB
	})

	t.Run("returns database when initialized", func(t *testing.T) {
		// Save original value
		originalDB := database.DATABASE

		// Create a mock database for testing
		mockDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		require.NoError(t, err)

		database.DATABASE = mockDB
		db := database.GetDatabase()
		assert.NotNil(t, db)
		assert.Equal(t, mockDB, db)

		// Restore original value
		database.DATABASE = originalDB
	})
}
