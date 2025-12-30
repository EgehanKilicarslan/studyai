package testutil

import (
	"context"
	"io"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/api"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
	grpcClient "github.com/EgehanKilicarslan/studyai/backend-go/internal/grpc"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/handler"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/middleware"
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

// ==================== MOCK CHAT SERVICE CLIENT ====================

// MockChatServiceClient implements pb.ChatServiceClient for testing
type MockChatServiceClient struct {
	mock.Mock
}

func (m *MockChatServiceClient) Chat(ctx context.Context, in *pb.ChatRequest, opts ...grpc.CallOption) (pb.ChatService_ChatClient, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pb.ChatService_ChatClient), args.Error(1)
}

// MockChatStream implements pb.ChatService_ChatClient for testing
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

func (m *MockChatStream) Header() (metadata.MD, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(metadata.MD), args.Error(1)
}

func (m *MockChatStream) Trailer() metadata.MD {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(metadata.MD)
}

func (m *MockChatStream) CloseSend() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockChatStream) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

func (m *MockChatStream) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockChatStream) RecvMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

// ==================== MOCK KNOWLEDGE BASE SERVICE CLIENT ====================

// MockKnowledgeBaseServiceClient implements pb.KnowledgeBaseServiceClient for testing
type MockKnowledgeBaseServiceClient struct {
	mock.Mock
}

func (m *MockKnowledgeBaseServiceClient) UploadDocument(ctx context.Context, opts ...grpc.CallOption) (pb.KnowledgeBaseService_UploadDocumentClient, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pb.KnowledgeBaseService_UploadDocumentClient), args.Error(1)
}

func (m *MockKnowledgeBaseServiceClient) DeleteDocument(ctx context.Context, in *pb.DeleteDocumentRequest, opts ...grpc.CallOption) (*pb.DeleteDocumentResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.DeleteDocumentResponse), args.Error(1)
}

func (m *MockKnowledgeBaseServiceClient) ListDocuments(ctx context.Context, in *pb.ListDocumentsRequest, opts ...grpc.CallOption) (*pb.ListDocumentsResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.ListDocumentsResponse), args.Error(1)
}

// MockUploadStream implements pb.KnowledgeBaseService_UploadDocumentClient for testing
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.UploadResponse), args.Error(1)
}

func (m *MockUploadStream) Header() (metadata.MD, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(metadata.MD), args.Error(1)
}

func (m *MockUploadStream) Trailer() metadata.MD {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(metadata.MD)
}

func (m *MockUploadStream) CloseSend() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockUploadStream) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

func (m *MockUploadStream) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockUploadStream) RecvMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
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

// ==================== MOCK GRPC CLIENT WRAPPER ====================

// CreateMockGrpcClient creates a mock grpc.Client with mock services
func CreateMockGrpcClient(chatClient *MockChatServiceClient, kbClient *MockKnowledgeBaseServiceClient) *grpcClient.Client {
	return &grpcClient.Client{
		ChatService:          chatClient,
		KnowledgeBaseService: kbClient,
	}
}

// ==================== ROUTER SETUP HELPERS ====================

// SetupRouterWithMocks creates a router with mock services for testing
func SetupRouterWithMocks(
	grpcCli *grpcClient.Client,
	authService service.AuthService,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg := TestConfig()
	logger := TestLogger()

	chatHandler := handler.NewChatHandler(grpcCli, cfg, logger)
	kbHandler := handler.NewKnowledgeBaseHandler(grpcCli, cfg, logger)
	authHandler := handler.NewAuthHandler(authService, logger)
	authMiddleware := middleware.NewAuthMiddleware(authService, logger)

	return api.SetupRouter(chatHandler, kbHandler, authHandler, authMiddleware)
}

// SetupRouterWithDefaultAuth creates a router with a default auth mock that accepts any token
func SetupRouterWithDefaultAuth(grpcCli *grpcClient.Client) *gin.Engine {
	mockAuthService := new(MockAuthService)
	mockAuthService.On("ValidateAccessToken", mock.Anything).Return(uint(1), nil)
	return SetupRouterWithMocks(grpcCli, mockAuthService)
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
