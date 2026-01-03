package service

import (
	"errors"
	"log/slog"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
)

// UserService defines the interface for user business logic
type UserService interface {
	// User retrieval
	GetUser(userID uint) (*models.User, error)
	GetUserByEmail(email string) (*models.User, error)

	// Plan management
	GetUserPlanLimits(userID uint) (*config.UserPlanLimits, error)
	UpdateUserPlanTier(userID uint, tier config.PlanTier) (*models.User, error)
	UpdateUserBillingStatus(userID uint, status config.BillingStatus) error

	// Quota management
	GetUserQuota(userID uint) (*UserQuota, error)
	CheckStorageLimit(userID uint, additionalBytes int64) error
	UpdateStorageUsage(userID uint, deltaBytes int64) error

	// Limit checks
	CanCreateOrganization(userID uint) (bool, error)
	CanJoinOrganization(userID uint) (bool, error)
	CanCreateStandaloneGroup(userID uint) (bool, error)

	// Stripe integration helpers
	UpdateStripeCustomerID(userID uint, customerID string) error
	UpdateSubscriptionID(userID uint, subscriptionID string) error
}

// UserQuota represents quota information for a user
type UserQuota struct {
	User   *models.User
	Usage  UserUsage
	Limits config.UserPlanLimits
}

// UserUsage represents current resource usage for a user
type UserUsage struct {
	StandaloneGroups int64
	Organizations    int64
	StorageBytes     int64
}

type userService struct {
	userRepo  repository.UserRepository
	orgRepo   repository.OrganizationRepository
	groupRepo repository.GroupRepository
	logger    *slog.Logger
}

// NewUserService creates a new user service instance
func NewUserService(
	userRepo repository.UserRepository,
	orgRepo repository.OrganizationRepository,
	groupRepo repository.GroupRepository,
	logger *slog.Logger,
) UserService {
	return &userService{
		userRepo:  userRepo,
		orgRepo:   orgRepo,
		groupRepo: groupRepo,
		logger:    logger,
	}
}

// ==================== User Retrieval ====================

func (s *userService) GetUser(userID uint) (*models.User, error) {
	return s.userRepo.FindByID(userID)
}

func (s *userService) GetUserByEmail(email string) (*models.User, error) {
	return s.userRepo.FindByEmail(email)
}

// ==================== Plan Management ====================

func (s *userService) GetUserPlanLimits(userID uint) (*config.UserPlanLimits, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}

	limits := user.GetPlanLimits()
	return &limits, nil
}

func (s *userService) UpdateUserPlanTier(userID uint, tier config.PlanTier) (*models.User, error) {
	s.logger.Info("💳 [UserService] Updating user plan tier", "user_id", userID, "tier", tier)

	if !config.IsValidTier(tier) {
		return nil, ErrInvalidUserPlanTier
	}

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		s.logger.Error("❌ [UserService] Failed to find user", "user_id", userID, "error", err)
		return nil, err
	}

	user.PlanTier = tier

	if err := s.userRepo.Update(user); err != nil {
		s.logger.Error("❌ [UserService] Failed to update user plan tier", "user_id", userID, "error", err)
		return nil, err
	}

	s.logger.Info("✅ [UserService] User plan tier updated", "user_id", userID, "tier", tier)
	return user, nil
}

func (s *userService) UpdateUserBillingStatus(userID uint, status config.BillingStatus) error {
	s.logger.Info("💳 [UserService] Updating user billing status", "user_id", userID, "status", status)

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		s.logger.Error("❌ [UserService] Failed to find user", "user_id", userID, "error", err)
		return err
	}

	user.BillingStatus = status

	if err := s.userRepo.Update(user); err != nil {
		s.logger.Error("❌ [UserService] Failed to update user billing status", "user_id", userID, "error", err)
		return err
	}

	s.logger.Info("✅ [UserService] User billing status updated", "user_id", userID, "status", status)
	return nil
}

// ==================== Quota Management ====================

func (s *userService) GetUserQuota(userID uint) (*UserQuota, error) {
	s.logger.Info("📊 [UserService] Getting user quota", "user_id", userID)

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		s.logger.Error("❌ [UserService] Failed to find user", "user_id", userID, "error", err)
		return nil, err
	}

	// Count standalone groups owned by user
	standaloneGroupCount, err := s.groupRepo.CountStandaloneGroupsByOwner(userID)
	if err != nil {
		s.logger.Warn("⚠️ [UserService] Failed to count standalone groups", "user_id", userID, "error", err)
		standaloneGroupCount = 0
	}

	// Count organizations user belongs to
	orgCount, err := s.orgRepo.CountUserOrganizations(userID)
	if err != nil {
		s.logger.Warn("⚠️ [UserService] Failed to count organizations", "user_id", userID, "error", err)
		orgCount = 0
	}

	limits := user.GetPlanLimits()

	return &UserQuota{
		User: user,
		Usage: UserUsage{
			StandaloneGroups: standaloneGroupCount,
			Organizations:    orgCount,
			StorageBytes:     user.UsedStorageBytes,
		},
		Limits: limits,
	}, nil
}

func (s *userService) CheckStorageLimit(userID uint, additionalBytes int64) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	limits := user.GetPlanLimits()

	// Check if unlimited storage (-1)
	if limits.MaxStorageBytes < 0 {
		return nil
	}

	newTotal := user.UsedStorageBytes + additionalBytes
	if newTotal > limits.MaxStorageBytes {
		s.logger.Warn("⚠️ [UserService] Storage quota exceeded",
			"user_id", userID,
			"current", user.UsedStorageBytes,
			"additional", additionalBytes,
			"limit", limits.MaxStorageBytes,
		)
		return config.NewQuotaError(
			"storage",
			limits.MaxStorageBytes,
			user.UsedStorageBytes,
			"Storage limit reached for your plan",
		)
	}

	return nil
}

func (s *userService) UpdateStorageUsage(userID uint, deltaBytes int64) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	newTotal := user.UsedStorageBytes + deltaBytes
	if newTotal < 0 {
		newTotal = 0
	}

	user.UsedStorageBytes = newTotal
	return s.userRepo.Update(user)
}

// ==================== Limit Checks ====================

func (s *userService) CanCreateOrganization(userID uint) (bool, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return false, err
	}

	limits := user.GetPlanLimits()
	return limits.CanCreateOrganization, nil
}

func (s *userService) CanJoinOrganization(userID uint) (bool, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return false, err
	}

	limits := user.GetPlanLimits()

	// Check if unlimited (-1)
	if limits.MaxOrganizations < 0 {
		return true, nil
	}

	// Count current organizations
	orgCount, err := s.orgRepo.CountUserOrganizations(userID)
	if err != nil {
		return false, err
	}

	return int(orgCount) < limits.MaxOrganizations, nil
}

func (s *userService) CanCreateStandaloneGroup(userID uint) (bool, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return false, err
	}

	limits := user.GetPlanLimits()

	// Check if unlimited (-1)
	if limits.MaxStandaloneGroups < 0 {
		return true, nil
	}

	// Count current standalone groups
	groupCount, err := s.groupRepo.CountStandaloneGroupsByOwner(userID)
	if err != nil {
		return false, err
	}

	return int(groupCount) < limits.MaxStandaloneGroups, nil
}

// ==================== Stripe Integration ====================

func (s *userService) UpdateStripeCustomerID(userID uint, customerID string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	user.StripeCustomerID = &customerID
	return s.userRepo.Update(user)
}

func (s *userService) UpdateSubscriptionID(userID uint, subscriptionID string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	user.SubscriptionID = &subscriptionID
	return s.userRepo.Update(user)
}

// Service errors
var (
	ErrInvalidUserPlanTier = errors.New("invalid user plan tier")
)
