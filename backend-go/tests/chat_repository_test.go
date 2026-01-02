package tests

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
)

func setupChatTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate all required tables
	err = db.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.ChatSession{},
		&models.ChatMessage{},
	)
	require.NoError(t, err)

	return db
}

// ==================== CHAT REPOSITORY TESTS ====================

func TestChatRepository_CreateSession(t *testing.T) {
	db := setupChatTestDB(t)
	repo := repository.NewChatRepository(db)

	// Create test user and organization first
	user := &models.User{
		Username: "testuser",
		Email:    "test@example.com",
		FullName: "Test User",
		Password: "hashedpassword",
	}
	require.NoError(t, db.Create(user).Error)

	org := &models.Organization{
		Name: "Test Org",
	}
	require.NoError(t, db.Create(org).Error)

	tests := []struct {
		name    string
		session *models.ChatSession
		wantErr bool
	}{
		{
			name: "success",
			session: &models.ChatSession{
				ID:             uuid.New(),
				UserID:         user.ID,
				OrganizationID: org.ID,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			},
			wantErr: false,
		},
		{
			name: "duplicate_id",
			session: &models.ChatSession{
				ID:             uuid.New(),
				UserID:         user.ID,
				OrganizationID: org.ID,
			},
			wantErr: true, // Should fail on duplicate ID
		},
	}

	var firstSessionID uuid.UUID
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "duplicate_id" && firstSessionID != uuid.Nil {
				tt.session.ID = firstSessionID
			}

			err := repo.CreateSession(tt.session)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if firstSessionID == uuid.Nil {
					firstSessionID = tt.session.ID
				}
			}
		})
	}
}

func TestChatRepository_GetSession(t *testing.T) {
	db := setupChatTestDB(t)
	repo := repository.NewChatRepository(db)

	// Create test data
	user := &models.User{
		Username: "testuser",
		Email:    "test@example.com",
		FullName: "Test User",
		Password: "hashedpassword",
	}
	require.NoError(t, db.Create(user).Error)

	org := &models.Organization{
		Name: "Test Org",
	}
	require.NoError(t, db.Create(org).Error)

	sessionID := uuid.New()
	session := &models.ChatSession{
		ID:             sessionID,
		UserID:         user.ID,
		OrganizationID: org.ID,
	}
	require.NoError(t, repo.CreateSession(session))

	tests := []struct {
		name      string
		sessionID uuid.UUID
		wantErr   bool
		checkFunc func(*testing.T, *models.ChatSession)
	}{
		{
			name:      "found",
			sessionID: sessionID,
			wantErr:   false,
			checkFunc: func(t *testing.T, s *models.ChatSession) {
				assert.Equal(t, sessionID, s.ID)
				assert.Equal(t, user.ID, s.UserID)
				assert.Equal(t, org.ID, s.OrganizationID)
			},
		},
		{
			name:      "not_found",
			sessionID: uuid.New(),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetSession(tt.sessionID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.checkFunc != nil {
					tt.checkFunc(t, result)
				}
			}
		})
	}
}

func TestChatRepository_GetOrCreateSession(t *testing.T) {
	db := setupChatTestDB(t)
	repo := repository.NewChatRepository(db)

	// Create test data
	user := &models.User{
		Username: "testuser",
		Email:    "test@example.com",
		FullName: "Test User",
		Password: "hashedpassword",
	}
	require.NoError(t, db.Create(user).Error)

	org := &models.Organization{
		Name: "Test Org",
	}
	require.NoError(t, db.Create(org).Error)

	tests := []struct {
		name           string
		sessionID      uuid.UUID
		userID         uint
		organizationID uint
		setupFunc      func() uuid.UUID
		wantCreated    bool
		wantErr        bool
	}{
		{
			name:           "create_new_session",
			sessionID:      uuid.New(),
			userID:         user.ID,
			organizationID: org.ID,
			wantCreated:    true,
			wantErr:        false,
		},
		{
			name:      "get_existing_session",
			sessionID: uuid.New(),
			setupFunc: func() uuid.UUID {
				sid := uuid.New()
				session := &models.ChatSession{
					ID:             sid,
					UserID:         user.ID,
					OrganizationID: org.ID,
				}
				require.NoError(t, repo.CreateSession(session))
				return sid
			},
			userID:         user.ID,
			organizationID: org.ID,
			wantCreated:    false,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionID := tt.sessionID
			if tt.setupFunc != nil {
				sessionID = tt.setupFunc()
			}

			result, created, err := repo.GetOrCreateSession(sessionID, tt.userID, tt.organizationID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantCreated, created)
				assert.Equal(t, sessionID, result.ID)
				assert.Equal(t, tt.userID, result.UserID)
				assert.Equal(t, tt.organizationID, result.OrganizationID)
			}
		})
	}
}

func TestChatRepository_CreateMessage(t *testing.T) {
	db := setupChatTestDB(t)
	repo := repository.NewChatRepository(db)

	// Create test data
	user := &models.User{
		Username: "testuser",
		Email:    "test@example.com",
		FullName: "Test User",
		Password: "hashedpassword",
	}
	require.NoError(t, db.Create(user).Error)

	org := &models.Organization{
		Name: "Test Org",
	}
	require.NoError(t, db.Create(org).Error)

	sessionID := uuid.New()
	session := &models.ChatSession{
		ID:             sessionID,
		UserID:         user.ID,
		OrganizationID: org.ID,
	}
	require.NoError(t, repo.CreateSession(session))

	tests := []struct {
		name    string
		message *models.ChatMessage
		wantErr bool
	}{
		{
			name: "success_user_message",
			message: &models.ChatMessage{
				ID:        uuid.New(),
				SessionID: sessionID,
				Role:      "user",
				Content:   "Hello, how are you?",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "success_assistant_message",
			message: &models.ChatMessage{
				ID:        uuid.New(),
				SessionID: sessionID,
				Role:      "assistant",
				Content:   "I'm doing well, thank you!",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.CreateMessage(tt.message)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify message was saved
				var savedMsg models.ChatMessage
				err = db.Where("id = ?", tt.message.ID).First(&savedMsg).Error
				assert.NoError(t, err)
				assert.Equal(t, tt.message.Content, savedMsg.Content)
				assert.Equal(t, tt.message.Role, savedMsg.Role)
			}
		})
	}
}

func TestChatRepository_GetRecentMessages(t *testing.T) {
	db := setupChatTestDB(t)
	repo := repository.NewChatRepository(db)

	// Create test data
	user := &models.User{
		Username: "testuser",
		Email:    "test@example.com",
		FullName: "Test User",
		Password: "hashedpassword",
	}
	require.NoError(t, db.Create(user).Error)

	org := &models.Organization{
		Name: "Test Org",
	}
	require.NoError(t, db.Create(org).Error)

	sessionID := uuid.New()
	session := &models.ChatSession{
		ID:             sessionID,
		UserID:         user.ID,
		OrganizationID: org.ID,
	}
	require.NoError(t, repo.CreateSession(session))

	// Create 10 messages
	baseTime := time.Now().Add(-10 * time.Hour)
	for i := 0; i < 10; i++ {
		msg := &models.ChatMessage{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "user",
			Content:   "Message " + string(rune('0'+i)),
			CreatedAt: baseTime.Add(time.Duration(i) * time.Hour),
			UpdatedAt: baseTime.Add(time.Duration(i) * time.Hour),
		}
		require.NoError(t, repo.CreateMessage(msg))
	}

	tests := []struct {
		name          string
		limit         int
		expectedCount int
		checkOrder    bool
	}{
		{
			name:          "get_last_5",
			limit:         5,
			expectedCount: 5,
			checkOrder:    true,
		},
		{
			name:          "get_last_20_returns_all_10",
			limit:         20,
			expectedCount: 10,
			checkOrder:    false,
		},
		{
			name:          "get_last_1",
			limit:         1,
			expectedCount: 1,
			checkOrder:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := repo.GetRecentMessages(sessionID, tt.limit)

			assert.NoError(t, err)
			assert.Len(t, messages, tt.expectedCount)

			// Verify chronological order (oldest first)
			if tt.checkOrder && len(messages) > 1 {
				for i := 1; i < len(messages); i++ {
					assert.True(t, messages[i-1].CreatedAt.Before(messages[i].CreatedAt) ||
						messages[i-1].CreatedAt.Equal(messages[i].CreatedAt))
				}
			}
		})
	}
}

func TestChatRepository_DeleteSessionMessages(t *testing.T) {
	db := setupChatTestDB(t)
	repo := repository.NewChatRepository(db)

	// Create test data
	user := &models.User{
		Username: "testuser",
		Email:    "test@example.com",
		FullName: "Test User",
		Password: "hashedpassword",
	}
	require.NoError(t, db.Create(user).Error)

	org := &models.Organization{
		Name: "Test Org",
	}
	require.NoError(t, db.Create(org).Error)

	sessionID := uuid.New()
	session := &models.ChatSession{
		ID:             sessionID,
		UserID:         user.ID,
		OrganizationID: org.ID,
	}
	require.NoError(t, repo.CreateSession(session))

	// Create messages
	for i := 0; i < 5; i++ {
		msg := &models.ChatMessage{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "user",
			Content:   "Test message",
		}
		require.NoError(t, repo.CreateMessage(msg))
	}

	// Delete all messages
	err := repo.DeleteSessionMessages(sessionID)
	assert.NoError(t, err)

	// Verify deletion
	messages, err := repo.GetSessionMessages(sessionID, 100)
	assert.NoError(t, err)
	assert.Empty(t, messages)
}

func TestChatRepository_GetUserSessions(t *testing.T) {
	db := setupChatTestDB(t)
	repo := repository.NewChatRepository(db)

	// Create test data
	user := &models.User{
		Username: "testuser",
		Email:    "test@example.com",
		FullName: "Test User",
		Password: "hashedpassword",
	}
	require.NoError(t, db.Create(user).Error)

	org1 := &models.Organization{Name: "Org 1"}
	org2 := &models.Organization{Name: "Org 2"}
	require.NoError(t, db.Create(org1).Error)
	require.NoError(t, db.Create(org2).Error)

	// Create sessions for org1
	for i := 0; i < 3; i++ {
		session := &models.ChatSession{
			ID:             uuid.New(),
			UserID:         user.ID,
			OrganizationID: org1.ID,
		}
		require.NoError(t, repo.CreateSession(session))
	}

	// Create session for org2
	session := &models.ChatSession{
		ID:             uuid.New(),
		UserID:         user.ID,
		OrganizationID: org2.ID,
	}
	require.NoError(t, repo.CreateSession(session))

	tests := []struct {
		name           string
		userID         uint
		organizationID uint
		limit          int
		expectedCount  int
	}{
		{
			name:           "get_all_sessions_for_org1",
			userID:         user.ID,
			organizationID: org1.ID,
			limit:          10,
			expectedCount:  3,
		},
		{
			name:           "get_limited_sessions",
			userID:         user.ID,
			organizationID: org1.ID,
			limit:          2,
			expectedCount:  2,
		},
		{
			name:           "get_sessions_for_org2",
			userID:         user.ID,
			organizationID: org2.ID,
			limit:          10,
			expectedCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions, err := repo.GetUserSessions(tt.userID, tt.organizationID, tt.limit)

			assert.NoError(t, err)
			assert.Len(t, sessions, tt.expectedCount)

			// Verify all sessions belong to the correct user and org
			for _, session := range sessions {
				assert.Equal(t, tt.userID, session.UserID)
				assert.Equal(t, tt.organizationID, session.OrganizationID)
			}
		})
	}
}
