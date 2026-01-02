package testutil

import (
	"context"
	"io"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
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

func (m *MockUserRepository) FindByUsername(username string) (*models.User, error) {
	args := m.Called(username)
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

// ==================== MOCK RATE LIMITER ====================

// MockRateLimiter implements ratelimit.RateLimiter for testing
type MockRateLimiter struct {
	mock.Mock
}

func (m *MockRateLimiter) CheckDailyLimit(ctx context.Context, userID uint, orgID uint, limits config.PlanLimits) (bool, int64, int64, error) {
	args := m.Called(ctx, userID, orgID, limits)
	return args.Bool(0), args.Get(1).(int64), args.Get(2).(int64), args.Error(3)
}

func (m *MockRateLimiter) IncrementDailyCount(ctx context.Context, userID uint) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRateLimiter) GetRemainingMessages(ctx context.Context, userID uint, orgID uint, limits config.PlanLimits) (int64, error) {
	args := m.Called(ctx, userID, orgID, limits)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRateLimiter) Close() error {
	args := m.Called()
	return args.Error(0)
}

// ==================== MOCK ORGANIZATION REPOSITORY ====================

// MockOrganizationRepository implements repository.OrganizationRepository for testing
type MockOrganizationRepository struct {
	mock.Mock
}

func (m *MockOrganizationRepository) Create(org *models.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *MockOrganizationRepository) FindByID(id uint) (*models.Organization, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Organization), args.Error(1)
}

func (m *MockOrganizationRepository) Update(org *models.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *MockOrganizationRepository) Delete(id uint) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockOrganizationRepository) FindByDomain(domain string) (*models.Organization, error) {
	args := m.Called(domain)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Organization), args.Error(1)
}

func (m *MockOrganizationRepository) ListByOwner(ownerID uint) ([]models.Organization, error) {
	args := m.Called(ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Organization), args.Error(1)
}

func (m *MockOrganizationRepository) AddMember(member *models.OrganizationMember) error {
	args := m.Called(member)
	return args.Error(0)
}

func (m *MockOrganizationRepository) RemoveMember(orgID, userID uint) error {
	args := m.Called(orgID, userID)
	return args.Error(0)
}

func (m *MockOrganizationRepository) UpdateMemberRole(orgID, userID, roleID uint) error {
	args := m.Called(orgID, userID, roleID)
	return args.Error(0)
}

func (m *MockOrganizationRepository) ListMembers(orgID uint, offset, limit int) ([]models.OrganizationMember, int64, error) {
	args := m.Called(orgID, offset, limit)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]models.OrganizationMember), args.Get(1).(int64), args.Error(2)
}

func (m *MockOrganizationRepository) GetMember(orgID, userID uint) (*models.OrganizationMember, error) {
	args := m.Called(orgID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OrganizationMember), args.Error(1)
}

func (m *MockOrganizationRepository) CountMembers(orgID uint) (int64, error) {
	args := m.Called(orgID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockOrganizationRepository) IncrementStorage(orgID uint, bytes int64) error {
	args := m.Called(orgID, bytes)
	return args.Error(0)
}

func (m *MockOrganizationRepository) DecrementStorage(orgID uint, bytes int64) error {
	args := m.Called(orgID, bytes)
	return args.Error(0)
}

func (m *MockOrganizationRepository) UpdatePlanTier(orgID uint, tier string) error {
	args := m.Called(orgID, tier)
	return args.Error(0)
}

func (m *MockOrganizationRepository) UpdateBillingStatus(orgID uint, status string) error {
	args := m.Called(orgID, status)
	return args.Error(0)
}

func (m *MockOrganizationRepository) GetUserOrganizations(userID uint) ([]models.OrganizationMember, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.OrganizationMember), args.Error(1)
}

func (m *MockOrganizationRepository) List(offset, limit int) ([]models.Organization, int64, error) {
	args := m.Called(offset, limit)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]models.Organization), args.Get(1).(int64), args.Error(2)
}

// Organization Role methods
func (m *MockOrganizationRepository) CreateRole(role *models.OrganizationRole) error {
	args := m.Called(role)
	return args.Error(0)
}

func (m *MockOrganizationRepository) FindRoleByID(roleID uint) (*models.OrganizationRole, error) {
	args := m.Called(roleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OrganizationRole), args.Error(1)
}

func (m *MockOrganizationRepository) FindRoleByName(orgID uint, name string) (*models.OrganizationRole, error) {
	args := m.Called(orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OrganizationRole), args.Error(1)
}

func (m *MockOrganizationRepository) ListRoles(orgID uint) ([]models.OrganizationRole, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.OrganizationRole), args.Error(1)
}

func (m *MockOrganizationRepository) UpdateRole(role *models.OrganizationRole) error {
	args := m.Called(role)
	return args.Error(0)
}

func (m *MockOrganizationRepository) DeleteRole(roleID uint) error {
	args := m.Called(roleID)
	return args.Error(0)
}

// ==================== MOCK GROUP REPOSITORY ====================

// MockGroupRepository implements repository.GroupRepository for testing
type MockGroupRepository struct {
	mock.Mock
}

func (m *MockGroupRepository) Create(group *models.Group) error {
	args := m.Called(group)
	return args.Error(0)
}

func (m *MockGroupRepository) FindByID(id uint) (*models.Group, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *MockGroupRepository) Update(group *models.Group) error {
	args := m.Called(group)
	return args.Error(0)
}

func (m *MockGroupRepository) Delete(id uint) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockGroupRepository) ListByOrganization(orgID uint, offset, limit int) ([]models.Group, int64, error) {
	args := m.Called(orgID, offset, limit)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]models.Group), args.Get(1).(int64), args.Error(2)
}

func (m *MockGroupRepository) CountByOrganization(orgID uint) (int64, error) {
	args := m.Called(orgID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockGroupRepository) CreateRole(role *models.GroupRole) error {
	args := m.Called(role)
	return args.Error(0)
}

func (m *MockGroupRepository) FindRoleByID(roleID uint) (*models.GroupRole, error) {
	args := m.Called(roleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GroupRole), args.Error(1)
}

func (m *MockGroupRepository) FindRoleByName(groupID uint, name string) (*models.GroupRole, error) {
	args := m.Called(groupID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GroupRole), args.Error(1)
}

func (m *MockGroupRepository) UpdateRole(roleID uint, name string, permissions pq.StringArray) error {
	args := m.Called(roleID, name, permissions)
	return args.Error(0)
}

func (m *MockGroupRepository) DeleteRole(roleID uint) error {
	args := m.Called(roleID)
	return args.Error(0)
}

func (m *MockGroupRepository) ListRoles(groupID uint) ([]models.GroupRole, error) {
	args := m.Called(groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GroupRole), args.Error(1)
}

func (m *MockGroupRepository) GetRoleMemberCount(roleID uint) (int64, error) {
	args := m.Called(roleID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockGroupRepository) AddMember(member *models.GroupMember) error {
	args := m.Called(member)
	return args.Error(0)
}

func (m *MockGroupRepository) GetMember(userID, groupID uint) (*models.GroupMember, error) {
	args := m.Called(userID, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GroupMember), args.Error(1)
}

func (m *MockGroupRepository) UpdateMemberRole(userID, groupID, newRoleID uint) error {
	args := m.Called(userID, groupID, newRoleID)
	return args.Error(0)
}

func (m *MockGroupRepository) RemoveMember(userID, groupID uint) error {
	args := m.Called(userID, groupID)
	return args.Error(0)
}

func (m *MockGroupRepository) ListMembers(groupID uint, offset, limit int) ([]models.GroupMember, int64, error) {
	args := m.Called(groupID, offset, limit)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]models.GroupMember), args.Get(1).(int64), args.Error(2)
}

func (m *MockGroupRepository) GetUserGroups(userID uint) ([]models.GroupMember, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GroupMember), args.Error(1)
}

func (m *MockGroupRepository) GetUserGroupsInOrganization(userID, orgID uint) ([]uint, error) {
	args := m.Called(userID, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uint), args.Error(1)
}

func (m *MockGroupRepository) GetUserPermissionsInGroup(userID, groupID uint) ([]string, error) {
	args := m.Called(userID, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// ==================== MOCK AUTH SERVICE ====================

// MockAuthService implements service.AuthService for testing
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) Register(username, email, fullName, password string) (*models.User, *service.TokenPair, error) {
	args := m.Called(username, email, fullName, password)
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

func (m *MockKnowledgeBaseServiceClient) ProcessDocument(ctx context.Context, in *pb.ProcessDocumentRequest, opts ...grpc.CallOption) (*pb.ProcessDocumentResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.ProcessDocumentResponse), args.Error(1)
}

func (m *MockKnowledgeBaseServiceClient) DeleteDocument(ctx context.Context, in *pb.DeleteDocumentRequest, opts ...grpc.CallOption) (*pb.DeleteDocumentResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.DeleteDocumentResponse), args.Error(1)
}

// ==================== MOCK DOCUMENT SERVICE ====================

// MockDocumentService implements service.DocumentService for testing
type MockDocumentService struct {
	mock.Mock
}

func (m *MockDocumentService) CreateDocument(orgID *uint, groupID *uint, ownerID uint, filename string, contentType string, fileReader io.Reader) (*models.Document, error) {
	args := m.Called(orgID, groupID, ownerID, filename, contentType, fileReader)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Document), args.Error(1)
}

func (m *MockDocumentService) GetDocument(docID uuid.UUID) (*models.Document, error) {
	args := m.Called(docID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Document), args.Error(1)
}

func (m *MockDocumentService) DeleteDocument(docID uuid.UUID, requesterID uint) error {
	args := m.Called(docID, requesterID)
	return args.Error(0)
}

func (m *MockDocumentService) UpdateDocumentStatus(docID uuid.UUID, status models.DocumentStatus, chunksCount int, errorMsg *string) error {
	args := m.Called(docID, status, chunksCount, errorMsg)
	return args.Error(0)
}

func (m *MockDocumentService) ListDocuments(orgID, userID uint, page, pageSize int) ([]models.Document, int64, error) {
	args := m.Called(orgID, userID, page, pageSize)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]models.Document), args.Get(1).(int64), args.Error(2)
}

func (m *MockDocumentService) ListGroupDocuments(groupID uint, page, pageSize int) ([]models.Document, int64, error) {
	args := m.Called(groupID, page, pageSize)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]models.Document), args.Get(1).(int64), args.Error(2)
}

func (m *MockDocumentService) GetFilePath(docID uuid.UUID) (string, error) {
	args := m.Called(docID)
	return args.String(0), args.Error(1)
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
	docService service.DocumentService,
	authService service.AuthService,
	groupService service.GroupService,
	orgService service.OrganizationService,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg := TestConfig()
	logger := TestLogger()

	// Create mock rate limiter and org repo for chat handler
	mockRateLimiter := new(MockRateLimiter)
	mockRateLimiter.On("CheckDailyLimit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, int64(0), int64(100), nil)
	mockRateLimiter.On("IncrementDailyCount", mock.Anything, mock.Anything).Return(nil)

	mockOrgRepo := new(MockOrganizationRepository)
	mockOrgRepo.On("FindByID", mock.Anything).Return(&models.Organization{
		PlanTier:      config.PlanFree,
		BillingStatus: config.BillingActive,
	}, nil)

	// Create mock group repo for tenant-scoped access
	mockGroupRepo := new(MockGroupRepository)
	mockGroupRepo.On("GetUserGroupsInOrganization", mock.Anything, mock.Anything).Return([]uint{1}, nil)

	chatHandler := handler.NewChatHandler(grpcCli, cfg, logger, mockRateLimiter, mockOrgRepo, mockGroupRepo)
	kbHandler := handler.NewKnowledgeBaseHandler(grpcCli, docService, mockGroupRepo, cfg, logger)
	authHandler := handler.NewAuthHandler(authService, logger)
	var groupHandler *handler.GroupHandler
	if groupService != nil {
		groupHandler = handler.NewGroupHandler(groupService, logger)
	}
	var adminHandler *handler.AdminHandler
	if orgService != nil {
		adminHandler = handler.NewAdminHandler(orgService, groupService, logger)
	}
	authMiddleware := middleware.NewAuthMiddleware(authService, logger)

	return api.SetupRouter(chatHandler, kbHandler, authHandler, groupHandler, adminHandler, authMiddleware)
}

// SetupRouterWithDefaultAuth creates a router with a default auth mock that accepts any token
func SetupRouterWithDefaultAuth(grpcCli *grpcClient.Client, docService service.DocumentService) *gin.Engine {
	mockAuthService := new(MockAuthService)
	mockAuthService.On("ValidateAccessToken", mock.Anything).Return(uint(1), nil)
	return SetupRouterWithMocks(grpcCli, docService, mockAuthService, nil, nil)
}

// SetupAuthRouterWithRepos creates a router with auth service using mock repositories
func SetupAuthRouterWithRepos(
	userRepo *MockUserRepository,
	refreshTokenRepo *MockRefreshTokenRepository,
) *gin.Engine {
	cfg := TestConfig()
	logger := TestLogger()

	authService := service.NewAuthService(userRepo, refreshTokenRepo, cfg, logger)
	return SetupRouterWithMocks(nil, nil, authService, nil, nil)
}

// CreateAuthServiceWithMocks creates an auth service with mock repositories for unit testing
func CreateAuthServiceWithMocks(
	userRepo *MockUserRepository,
	refreshTokenRepo *MockRefreshTokenRepository,
) service.AuthService {
	return service.NewAuthService(userRepo, refreshTokenRepo, TestConfig(), TestLogger())
}
