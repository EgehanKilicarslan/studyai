package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
	"github.com/EgehanKilicarslan/studyai/backend-go/tests/testutil"
)

// ==================== AUTH SERVICE UNIT TESTS ====================

func TestAuthService_Register(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		password    string
		setupMocks  func(*testutil.MockUserRepository, *testutil.MockRefreshTokenRepository)
		wantErr     error
		wantUserID  uint
		checkTokens bool
	}{
		{
			name:     "success",
			email:    "test@example.com",
			password: "password123",
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				userRepo.On("FindByEmail", "test@example.com").Return(nil, repository.ErrUserNotFound)
				userRepo.On("FindByUsername", "testuser").Return(nil, repository.ErrUserNotFound)
				userRepo.On("Create", mock.AnythingOfType("*models.User")).Run(func(args mock.Arguments) {
					user := args.Get(0).(*models.User)
					user.ID = 1
				}).Return(uint(1), nil)
				tokenRepo.On("Create", mock.AnythingOfType("*models.RefreshToken")).Return(nil)
			},
			wantUserID:  1,
			checkTokens: true,
		},
		{
			name:     "email already exists",
			email:    "existing@example.com",
			password: "password123",
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				userRepo.On("FindByEmail", "existing@example.com").Return(&models.User{ID: 1, Email: "existing@example.com"}, nil)
			},
			wantErr: service.ErrEmailAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(testutil.MockUserRepository)
			tokenRepo := new(testutil.MockRefreshTokenRepository)
			tt.setupMocks(userRepo, tokenRepo)

			authService := testutil.CreateAuthServiceWithMocks(userRepo, tokenRepo)
			user, tokens, err := authService.Register("testuser", tt.email, "Test User", tt.password)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, user)
				assert.Nil(t, tokens)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantUserID, user.ID)
				if tt.checkTokens {
					assert.NotEmpty(t, tokens.AccessToken)
					assert.NotEmpty(t, tokens.RefreshToken)
				}
			}

			userRepo.AssertExpectations(t)
			tokenRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_Login(t *testing.T) {
	// Password hash for "password" (bcrypt)
	validPasswordHash := "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi"

	tests := []struct {
		name        string
		email       string
		password    string
		setupMocks  func(*testutil.MockUserRepository, *testutil.MockRefreshTokenRepository)
		wantErr     error
		checkTokens bool
	}{
		{
			name:     "success",
			email:    "test@example.com",
			password: "password",
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				userRepo.On("FindByEmail", "test@example.com").Return(&models.User{
					ID:       1,
					Email:    "test@example.com",
					Password: validPasswordHash,
				}, nil)
				tokenRepo.On("Create", mock.AnythingOfType("*models.RefreshToken")).Return(nil)
			},
			checkTokens: true,
		},
		{
			name:     "user not found",
			email:    "nonexistent@example.com",
			password: "password123",
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				userRepo.On("FindByEmail", "nonexistent@example.com").Return(nil, repository.ErrUserNotFound)
			},
			wantErr: service.ErrInvalidCredentials,
		},
		{
			name:     "wrong password",
			email:    "test@example.com",
			password: "wrongpassword",
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				userRepo.On("FindByEmail", "test@example.com").Return(&models.User{
					ID:       1,
					Email:    "test@example.com",
					Password: validPasswordHash,
				}, nil)
			},
			wantErr: service.ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(testutil.MockUserRepository)
			tokenRepo := new(testutil.MockRefreshTokenRepository)
			tt.setupMocks(userRepo, tokenRepo)

			authService := testutil.CreateAuthServiceWithMocks(userRepo, tokenRepo)
			user, tokens, err := authService.Login(tt.email, tt.password)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, user)
				if tt.checkTokens {
					assert.NotEmpty(t, tokens.AccessToken)
					assert.NotEmpty(t, tokens.RefreshToken)
				}
			}

			userRepo.AssertExpectations(t)
			tokenRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_RefreshToken(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		setupMocks func(*testutil.MockUserRepository, *testutil.MockRefreshTokenRepository)
		wantErr    bool
	}{
		{
			name:  "success",
			token: "valid-refresh-token",
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				tokenRepo.On("FindByToken", "valid-refresh-token").Return(&models.RefreshToken{
					ID:        1,
					UserID:    1,
					Token:     "valid-refresh-token",
					ExpiresAt: time.Now().Add(24 * time.Hour),
					IsRevoked: false,
				}, nil)
				tokenRepo.On("Create", mock.AnythingOfType("*models.RefreshToken")).Return(nil)
				tokenRepo.On("RevokeToken", "valid-refresh-token").Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "token not found",
			token: "invalid-token",
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				tokenRepo.On("FindByToken", "invalid-token").Return(nil, repository.ErrTokenNotFound)
			},
			wantErr: true,
		},
		{
			name:  "expired token",
			token: "expired-token",
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				tokenRepo.On("FindByToken", "expired-token").Return(nil, repository.ErrTokenExpired)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(testutil.MockUserRepository)
			tokenRepo := new(testutil.MockRefreshTokenRepository)
			tt.setupMocks(userRepo, tokenRepo)

			authService := testutil.CreateAuthServiceWithMocks(userRepo, tokenRepo)
			tokens, err := authService.RefreshToken(tt.token)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, tokens)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, tokens.AccessToken)
				assert.NotEmpty(t, tokens.RefreshToken)
			}

			tokenRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_Logout(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		setupMocks func(*testutil.MockRefreshTokenRepository)
		wantErr    error
	}{
		{
			name:  "success",
			token: "valid-refresh-token",
			setupMocks: func(tokenRepo *testutil.MockRefreshTokenRepository) {
				tokenRepo.On("RevokeToken", "valid-refresh-token").Return(nil)
			},
			wantErr: nil,
		},
		{
			name:  "token not found",
			token: "invalid-token",
			setupMocks: func(tokenRepo *testutil.MockRefreshTokenRepository) {
				tokenRepo.On("RevokeToken", "invalid-token").Return(repository.ErrTokenNotFound)
			},
			wantErr: repository.ErrTokenNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(testutil.MockUserRepository)
			tokenRepo := new(testutil.MockRefreshTokenRepository)
			tt.setupMocks(tokenRepo)

			authService := testutil.CreateAuthServiceWithMocks(userRepo, tokenRepo)
			err := authService.Logout(tt.token)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			tokenRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_ValidateAccessToken(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		userRepo := new(testutil.MockUserRepository)
		tokenRepo := new(testutil.MockRefreshTokenRepository)

		// Register to get a valid token
		userRepo.On("FindByEmail", "test@example.com").Return(nil, repository.ErrUserNotFound)
		userRepo.On("FindByUsername", "testuser").Return(nil, repository.ErrUserNotFound)
		userRepo.On("Create", mock.AnythingOfType("*models.User")).Run(func(args mock.Arguments) {
			user := args.Get(0).(*models.User)
			user.ID = 1
		}).Return(uint(1), nil)
		tokenRepo.On("Create", mock.AnythingOfType("*models.RefreshToken")).Return(nil)

		authService := testutil.CreateAuthServiceWithMocks(userRepo, tokenRepo)
		_, tokens, err := authService.Register("testuser", "test@example.com", "Test User", "password123")
		require.NoError(t, err)

		userID, err := authService.ValidateAccessToken(tokens.AccessToken)
		assert.NoError(t, err)
		assert.Equal(t, uint(1), userID)
	})

	t.Run("invalid token", func(t *testing.T) {
		userRepo := new(testutil.MockUserRepository)
		tokenRepo := new(testutil.MockRefreshTokenRepository)

		authService := testutil.CreateAuthServiceWithMocks(userRepo, tokenRepo)
		_, err := authService.ValidateAccessToken("invalid-token")
		assert.ErrorIs(t, err, service.ErrInvalidToken)
	})

	t.Run("malformed token", func(t *testing.T) {
		userRepo := new(testutil.MockUserRepository)
		tokenRepo := new(testutil.MockRefreshTokenRepository)

		authService := testutil.CreateAuthServiceWithMocks(userRepo, tokenRepo)
		_, err := authService.ValidateAccessToken("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.malformed.signature")
		assert.Error(t, err)
	})
}

// ==================== AUTH HANDLER INTEGRATION TESTS ====================

func TestRegisterHandler(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    map[string]string
		setupMocks     func(*testutil.MockUserRepository, *testutil.MockRefreshTokenRepository)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "success",
			requestBody: map[string]string{
				"username":  "testuser",
				"email":     "test@example.com",
				"full_name": "Test User",
				"password":  "password123",
			},
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				userRepo.On("FindByEmail", "test@example.com").Return(nil, repository.ErrUserNotFound)
				userRepo.On("FindByUsername", "testuser").Return(nil, repository.ErrUserNotFound)
				userRepo.On("Create", mock.AnythingOfType("*models.User")).Run(func(args mock.Arguments) {
					user := args.Get(0).(*models.User)
					user.ID = 1
				}).Return(uint(1), nil)
				tokenRepo.On("Create", mock.AnythingOfType("*models.RefreshToken")).Return(nil)
			},
			expectedStatus: 201,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				assert.NotEmpty(t, response["access_token"])
				assert.NotEmpty(t, response["refresh_token"])
				assert.Equal(t, "Bearer", response["token_type"])
			},
		},
		{
			name: "email already exists",
			requestBody: map[string]string{
				"username":  "testuser",
				"email":     "existing@example.com",
				"full_name": "Test User",
				"password":  "password123",
			},
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				userRepo.On("FindByEmail", "existing@example.com").Return(&models.User{ID: 1}, nil)
			},
			expectedStatus: 409,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Email already registered")
			},
		},
		{
			name: "invalid email",
			requestBody: map[string]string{
				"username":  "testuser",
				"email":     "invalid-email",
				"full_name": "Test User",
				"password":  "password123",
			},
			setupMocks:     func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {},
			expectedStatus: 400,
		},
		{
			name: "short password",
			requestBody: map[string]string{
				"username":  "testuser",
				"email":     "test@example.com",
				"full_name": "Test User",
				"password":  "short",
			},
			setupMocks:     func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {},
			expectedStatus: 400,
		},
		{
			name: "missing email",
			requestBody: map[string]string{
				"username":  "testuser",
				"full_name": "Test User",
				"password":  "password123",
			},
			setupMocks:     func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {},
			expectedStatus: 400,
		},
		{
			name: "missing password",
			requestBody: map[string]string{
				"username":  "testuser",
				"email":     "test@example.com",
				"full_name": "Test User",
			},
			setupMocks:     func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {},
			expectedStatus: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(testutil.MockUserRepository)
			tokenRepo := new(testutil.MockRefreshTokenRepository)
			tt.setupMocks(userRepo, tokenRepo)

			router := testutil.SetupAuthRouterWithRepos(userRepo, tokenRepo)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", testutil.RegisterEndpoint, bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			userRepo.AssertExpectations(t)
			tokenRepo.AssertExpectations(t)
		})
	}
}

func TestLoginHandler(t *testing.T) {
	validPasswordHash := "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi" // hash of "password"

	tests := []struct {
		name           string
		requestBody    map[string]string
		setupMocks     func(*testutil.MockUserRepository, *testutil.MockRefreshTokenRepository)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "success",
			requestBody: map[string]string{
				"email":    "test@example.com",
				"password": "password",
			},
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				userRepo.On("FindByEmail", "test@example.com").Return(&models.User{
					ID:       1,
					Email:    "test@example.com",
					Password: validPasswordHash,
				}, nil)
				tokenRepo.On("Create", mock.AnythingOfType("*models.RefreshToken")).Return(nil)
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				assert.NotEmpty(t, response["access_token"])
				assert.NotEmpty(t, response["refresh_token"])
				assert.Equal(t, "Bearer", response["token_type"])
			},
		},
		{
			name: "user not found",
			requestBody: map[string]string{
				"email":    "nonexistent@example.com",
				"password": "password123",
			},
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				userRepo.On("FindByEmail", "nonexistent@example.com").Return(nil, repository.ErrUserNotFound)
			},
			expectedStatus: 401,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Invalid")
			},
		},
		{
			name: "wrong password",
			requestBody: map[string]string{
				"email":    "test@example.com",
				"password": "wrongpassword",
			},
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				userRepo.On("FindByEmail", "test@example.com").Return(&models.User{
					ID:       1,
					Email:    "test@example.com",
					Password: validPasswordHash,
				}, nil)
			},
			expectedStatus: 401,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Invalid")
			},
		},
		{
			name: "missing password",
			requestBody: map[string]string{
				"email": "test@example.com",
			},
			setupMocks:     func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {},
			expectedStatus: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(testutil.MockUserRepository)
			tokenRepo := new(testutil.MockRefreshTokenRepository)
			tt.setupMocks(userRepo, tokenRepo)

			router := testutil.SetupAuthRouterWithRepos(userRepo, tokenRepo)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", testutil.LoginEndpoint, bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			userRepo.AssertExpectations(t)
			tokenRepo.AssertExpectations(t)
		})
	}
}

func TestRefreshHandler(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    map[string]string
		setupMocks     func(*testutil.MockUserRepository, *testutil.MockRefreshTokenRepository)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "success",
			requestBody: map[string]string{
				"refresh_token": "valid-refresh-token",
			},
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				tokenRepo.On("FindByToken", "valid-refresh-token").Return(&models.RefreshToken{
					ID:        1,
					UserID:    1,
					Token:     "valid-refresh-token",
					ExpiresAt: time.Now().Add(24 * time.Hour),
					IsRevoked: false,
				}, nil)
				tokenRepo.On("Create", mock.AnythingOfType("*models.RefreshToken")).Return(nil)
				tokenRepo.On("RevokeToken", "valid-refresh-token").Return(nil)
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				assert.NotEmpty(t, response["access_token"])
				assert.NotEmpty(t, response["refresh_token"])
				assert.Equal(t, "Bearer", response["token_type"])
			},
		},
		{
			name: "invalid token",
			requestBody: map[string]string{
				"refresh_token": "invalid-token",
			},
			setupMocks: func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {
				tokenRepo.On("FindByToken", "invalid-token").Return(nil, repository.ErrTokenNotFound)
			},
			expectedStatus: 401,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Invalid")
			},
		},
		{
			name:           "missing token",
			requestBody:    map[string]string{},
			setupMocks:     func(userRepo *testutil.MockUserRepository, tokenRepo *testutil.MockRefreshTokenRepository) {},
			expectedStatus: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(testutil.MockUserRepository)
			tokenRepo := new(testutil.MockRefreshTokenRepository)
			tt.setupMocks(userRepo, tokenRepo)

			router := testutil.SetupAuthRouterWithRepos(userRepo, tokenRepo)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", testutil.RefreshTokenEndpoint, bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			tokenRepo.AssertExpectations(t)
		})
	}
}

func TestLogoutHandler(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    map[string]string
		setupMocks     func(*testutil.MockRefreshTokenRepository)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "success",
			requestBody: map[string]string{
				"refresh_token": "valid-refresh-token",
			},
			setupMocks: func(tokenRepo *testutil.MockRefreshTokenRepository) {
				tokenRepo.On("RevokeToken", "valid-refresh-token").Return(nil)
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Logged out")
			},
		},
		{
			name: "invalid token",
			requestBody: map[string]string{
				"refresh_token": "invalid-token",
			},
			setupMocks: func(tokenRepo *testutil.MockRefreshTokenRepository) {
				tokenRepo.On("RevokeToken", "invalid-token").Return(repository.ErrTokenNotFound)
			},
			expectedStatus: 401,
		},
		{
			name:           "missing token",
			requestBody:    map[string]string{},
			setupMocks:     func(tokenRepo *testutil.MockRefreshTokenRepository) {},
			expectedStatus: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(testutil.MockUserRepository)
			tokenRepo := new(testutil.MockRefreshTokenRepository)
			tt.setupMocks(tokenRepo)

			router := testutil.SetupAuthRouterWithRepos(userRepo, tokenRepo)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", testutil.LogoutEndpoint, bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			tokenRepo.AssertExpectations(t)
		})
	}
}
