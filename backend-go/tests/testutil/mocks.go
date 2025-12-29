package testutil

import (
	"context"
	"io"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/api"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/handler"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/middleware"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/rag"
	pb "github.com/EgehanKilicarslan/studyai/backend-go/pb"
)

// ==================== MOCK USER REPOSITORY ====================

// MockUserRepository implements repository.UserRepository for testing
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(user *models.User) error {
	args := m.Called(user)
	if len(args) > 1 && args.Get(0) != nil {
		user.ID = args.Get(0).(uint)
	}
	return args.Error(len(args) - 1)
}

func (m *MockUserRepository) FindByEmail(email string) (*models.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) FindByID(id uint) (*models.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Update(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(id uint) error {
	args := m.Called(id)
	return args.Error(0)
}

// ==================== MOCK REFRESH TOKEN REPOSITORY ====================

// MockRefreshTokenRepository implements repository.RefreshTokenRepository for testing
type MockRefreshTokenRepository struct {
	mock.Mock
}

func (m *MockRefreshTokenRepository) Create(token *models.RefreshToken) error {
	args := m.Called(token)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) FindByToken(token string) (*models.RefreshToken, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepository) RevokeToken(token string) error {
	args := m.Called(token)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) RevokeAllUserTokens(userID uint) error {
	args := m.Called(userID)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) DeleteExpiredTokens() error {
	args := m.Called()
	return args.Error(0)
}

// ==================== MOCK AUTH SERVICE ====================

// MockAuthService implements service.AuthService for testing
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) Register(email, password string) (*models.User, *service.TokenPair, error) {
	args := m.Called(email, password)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*models.User), args.Get(1).(*service.TokenPair), args.Error(2)
}

func (m *MockAuthService) Login(email, password string) (*models.User, *service.TokenPair, error) {
	args := m.Called(email, password)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*models.User), args.Get(1).(*service.TokenPair), args.Error(2)
}

func (m *MockAuthService) RefreshToken(refreshToken string) (*service.TokenPair, error) {
	args := m.Called(refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.TokenPair), args.Error(1)
}

func (m *MockAuthService) Logout(refreshToken string) error {
	args := m.Called(refreshToken)
	return args.Error(0)
}

func (m *MockAuthService) ValidateAccessToken(tokenString string) (uint, error) {
	args := m.Called(tokenString)
	return args.Get(0).(uint), args.Error(1)
}

// ==================== MOCK RAG SERVICE ====================

// MockRagServiceClient implements pb.RagServiceClient for testing
type MockRagServiceClient struct {
	mock.Mock
}

func (m *MockRagServiceClient) Chat(ctx context.Context, in *pb.ChatRequest, opts ...grpc.CallOption) (pb.RagService_ChatClient, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(pb.RagService_ChatClient), args.Error(1)
}

func (m *MockRagServiceClient) UploadDocument(ctx context.Context, opts ...grpc.CallOption) (pb.RagService_UploadDocumentClient, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pb.RagService_UploadDocumentClient), args.Error(1)
}

// MockChatStream implements pb.RagService_ChatClient for testing
type MockChatStream struct {
	grpc.ClientStream
	mock.Mock
}

func (m *MockChatStream) Recv() (*pb.ChatResponse, error) {
	args := m.Called()
	if resp := args.Get(0); resp != nil {
		return resp.(*pb.ChatResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockUploadStream implements pb.RagService_UploadDocumentClient for testing
type MockUploadStream struct {
	grpc.ClientStream
	mock.Mock
}

func (m *MockUploadStream) Send(req *pb.UploadRequest) error {
	args := m.Called(req)
	return args.Error(0)
}

func (m *MockUploadStream) CloseAndRecv() (*pb.UploadResponse, error) {
	args := m.Called()
	return args.Get(0).(*pb.UploadResponse), args.Error(1)
}

// ==================== TEST CONFIGURATION ====================

// TestConfig returns a config suitable for testing
func TestConfig() *config.Config {
	return &config.Config{
		ApiServicePort:         "8080",
		AIServiceAddr:          "backend-python:50051",
		MaxFileSize:            10 * 1024 * 1024,
		UploadTimeout:          300,
		ChatTimeout:            30,
		AccessTokenExpiration:  900,
		RefreshTokenExpiration: 604800,
		JWTSecret:              "test-secret-key-for-testing-purposes",
	}
}

// TestLogger returns a silent logger for testing
func TestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ==================== ROUTER SETUP HELPERS ====================

// SetupRouterWithMocks creates a router with mock services for testing
func SetupRouterWithMocks(
	ragClient *rag.Client,
	authService service.AuthService,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg := TestConfig()
	logger := TestLogger()

	apiHandler := handler.NewApiHandler(ragClient, cfg, logger)
	authHandler := handler.NewAuthHandler(authService, logger)
	authMiddleware := middleware.NewAuthMiddleware(authService, logger)

	return api.SetupRouter(apiHandler, authHandler, authMiddleware)
}

// SetupRouterWithDefaultAuth creates a router with a default auth mock that accepts any token
func SetupRouterWithDefaultAuth(ragClient *rag.Client) *gin.Engine {
	mockAuthService := new(MockAuthService)
	mockAuthService.On("ValidateAccessToken", mock.Anything).Return(uint(1), nil)
	return SetupRouterWithMocks(ragClient, mockAuthService)
}

// SetupAuthRouterWithRepos creates a router with auth service using mock repositories
func SetupAuthRouterWithRepos(
	userRepo *MockUserRepository,
	refreshTokenRepo *MockRefreshTokenRepository,
) *gin.Engine {
	cfg := TestConfig()
	logger := TestLogger()

	authService := service.NewAuthService(userRepo, refreshTokenRepo, cfg, logger)
	return SetupRouterWithMocks(nil, authService)
}

// CreateAuthServiceWithMocks creates an auth service with mock repositories for unit testing
func CreateAuthServiceWithMocks(
	userRepo *MockUserRepository,
	refreshTokenRepo *MockRefreshTokenRepository,
) service.AuthService {
	return service.NewAuthService(userRepo, refreshTokenRepo, TestConfig(), TestLogger())
}
