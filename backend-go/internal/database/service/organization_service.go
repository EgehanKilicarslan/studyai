package service

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
)

// OrganizationService defines the interface for organization business logic
type OrganizationService interface {
	// Organization CRUD
	CreateOrganization(name string, domain *string, ownerUserID uint) (*models.Organization, error)
	GetOrganization(orgID uint) (*models.Organization, error)
	UpdateOrganization(orgID uint, name string, domain *string) (*models.Organization, error)
	DeleteOrganization(orgID uint) error

	// Member management
	AddMember(orgID, userID uint, role string) (*models.OrganizationMember, error)
	RemoveMember(orgID, userID uint) error
	UpdateMemberRole(orgID, userID uint, role string) error
	ListMembers(orgID uint, page, pageSize int) ([]models.OrganizationMember, int64, error)
	CountMembers(orgID uint) (int64, error)
	GetUserOrganizations(userID uint) ([]models.OrganizationMember, error)

	// Group counting (for quota display)
	CountGroups(orgID uint) (int64, error)

	// Plan management (Admin only)
	UpdatePlanTier(orgID uint, tier config.PlanTier) error
	UpdateBillingStatus(orgID uint, status config.BillingStatus) error

	// Storage management
	GetStorageUsage(orgID uint) (used int64, limit int64, err error)
}

type organizationService struct {
	orgRepo   repository.OrganizationRepository
	groupRepo repository.GroupRepository
	userRepo  repository.UserRepository
	logger    *slog.Logger
}

// NewOrganizationService creates a new organization service instance
func NewOrganizationService(
	orgRepo repository.OrganizationRepository,
	groupRepo repository.GroupRepository,
	userRepo repository.UserRepository,
	logger *slog.Logger,
) OrganizationService {
	return &organizationService{
		orgRepo:   orgRepo,
		groupRepo: groupRepo,
		userRepo:  userRepo,
		logger:    logger,
	}
}

// ==================== Organization CRUD ====================

func (s *organizationService) CreateOrganization(name string, domain *string, ownerUserID uint) (*models.Organization, error) {
	s.logger.Info("🏢 [OrganizationService] Creating organization", "name", name, "owner", ownerUserID)

	// Create organization with FREE tier by default
	org := &models.Organization{
		Name:          name,
		Domain:        domain,
		PlanTier:      config.PlanFree,
		BillingStatus: config.BillingActive,
	}

	if err := s.orgRepo.Create(org); err != nil {
		s.logger.Error("❌ [OrganizationService] Failed to create organization", "error", err)
		return nil, err
	}

	// Create default system roles for the organization
	if err := s.createDefaultRoles(org.ID); err != nil {
		s.logger.Error("❌ [OrganizationService] Failed to create default roles", "error", err)
		return nil, err
	}

	// Get the owner role
	ownerRole, err := s.orgRepo.FindRoleByName(org.ID, models.OrgRoleOwner)
	if err != nil {
		s.logger.Error("❌ [OrganizationService] Failed to find owner role", "error", err)
		return nil, err
	}

	// Add owner as the first member
	member := &models.OrganizationMember{
		OrganizationID: org.ID,
		UserID:         ownerUserID,
		RoleID:         ownerRole.ID,
	}

	if err := s.orgRepo.AddMember(member); err != nil {
		s.logger.Error("❌ [OrganizationService] Failed to add owner", "error", err)
		return nil, err
	}

	s.logger.Info("✅ [OrganizationService] Organization created", "org_id", org.ID)
	return org, nil
}

// createDefaultRoles creates the standard owner, admin, and member roles for an organization
func (s *organizationService) createDefaultRoles(orgID uint) error {
	// Owner role with full permissions
	ownerRole := &models.OrganizationRole{
		OrganizationID: orgID,
		Name:           models.OrgRoleOwner,
		Permissions:    []string{"org:*", "group:*", "member:*", "role:*", "document:*"},
		IsSystemRole:   true,
	}
	if err := s.orgRepo.CreateRole(ownerRole); err != nil {
		return err
	}

	// Admin role with most permissions except org management
	adminRole := &models.OrganizationRole{
		OrganizationID: orgID,
		Name:           models.OrgRoleAdmin,
		Permissions:    []string{"group:create", "group:read", "group:update", "group:delete", "member:invite", "member:read", "document:*"},
		IsSystemRole:   true,
	}
	if err := s.orgRepo.CreateRole(adminRole); err != nil {
		return err
	}

	// Member role with basic permissions
	memberRole := &models.OrganizationRole{
		OrganizationID: orgID,
		Name:           models.OrgRoleMember,
		Permissions:    []string{"group:read", "member:read", "document:read", "document:create"},
		IsSystemRole:   true,
	}
	return s.orgRepo.CreateRole(memberRole)
}

func (s *organizationService) GetOrganization(orgID uint) (*models.Organization, error) {
	return s.orgRepo.FindByID(orgID)
}

func (s *organizationService) UpdateOrganization(orgID uint, name string, domain *string) (*models.Organization, error) {
	org, err := s.orgRepo.FindByID(orgID)
	if err != nil {
		return nil, err
	}

	org.Name = name
	org.Domain = domain

	if err := s.orgRepo.Update(org); err != nil {
		return nil, err
	}

	return org, nil
}

func (s *organizationService) DeleteOrganization(orgID uint) error {
	return s.orgRepo.Delete(orgID)
}

// ==================== Member Management ====================

func (s *organizationService) AddMember(orgID, userID uint, role string) (*models.OrganizationMember, error) {
	s.logger.Info("👤 [OrganizationService] Adding member", "org_id", orgID, "user_id", userID, "role", role)

	// Get organization to check quota
	org, err := s.orgRepo.FindByID(orgID)
	if err != nil {
		s.logger.Error("❌ [OrganizationService] Failed to fetch organization", "error", err)
		return nil, err
	}

	// Check member quota
	limits := org.GetPlanLimits()
	if limits.MaxMembers >= 0 { // -1 means unlimited
		currentCount, err := s.orgRepo.CountMembers(orgID)
		if err != nil {
			s.logger.Error("❌ [OrganizationService] Failed to count members", "error", err)
			return nil, err
		}

		if int(currentCount) >= limits.MaxMembers {
			s.logger.Warn("⚠️ [OrganizationService] Member quota exceeded",
				"org_id", orgID,
				"current", currentCount,
				"limit", limits.MaxMembers,
			)
			return nil, config.NewQuotaError(
				"members",
				int64(limits.MaxMembers),
				currentCount,
				fmt.Sprintf("Member limit reached. Your %s plan allows %d members.", org.PlanTier, limits.MaxMembers),
			)
		}
	}

	// Verify user exists
	if _, err := s.userRepo.FindByID(userID); err != nil {
		return nil, errors.New("user not found")
	}

	// Check if already a member
	existing, _ := s.orgRepo.GetMember(orgID, userID)
	if existing != nil {
		return nil, errors.New("user is already a member of this organization")
	}

	// Find the role by name
	roleObj, err := s.orgRepo.FindRoleByName(orgID, role)
	if err != nil {
		s.logger.Error("❌ [OrganizationService] Role not found", "role", role, "error", err)
		return nil, errors.New("role not found")
	}

	member := &models.OrganizationMember{
		OrganizationID: orgID,
		UserID:         userID,
		RoleID:         roleObj.ID,
	}

	if err := s.orgRepo.AddMember(member); err != nil {
		s.logger.Error("❌ [OrganizationService] Failed to add member", "error", err)
		return nil, err
	}

	s.logger.Info("✅ [OrganizationService] Member added", "org_id", orgID, "user_id", userID)
	return member, nil
}

func (s *organizationService) RemoveMember(orgID, userID uint) error {
	return s.orgRepo.RemoveMember(orgID, userID)
}

func (s *organizationService) UpdateMemberRole(orgID, userID uint, role string) error {
	// Find the role by name
	roleObj, err := s.orgRepo.FindRoleByName(orgID, role)
	if err != nil {
		s.logger.Error("❌ [OrganizationService] Role not found", "role", role, "error", err)
		return errors.New("role not found")
	}
	return s.orgRepo.UpdateMemberRole(orgID, userID, roleObj.ID)
}

func (s *organizationService) ListMembers(orgID uint, page, pageSize int) ([]models.OrganizationMember, int64, error) {
	offset := (page - 1) * pageSize
	return s.orgRepo.ListMembers(orgID, offset, pageSize)
}

// ==================== Plan Management (Admin) ====================

func (s *organizationService) UpdatePlanTier(orgID uint, tier config.PlanTier) error {
	s.logger.Info("📊 [OrganizationService] Updating plan tier", "org_id", orgID, "tier", tier)

	if !config.IsValidTier(tier) {
		return errors.New("invalid plan tier")
	}

	if err := s.orgRepo.UpdatePlanTier(orgID, string(tier)); err != nil {
		s.logger.Error("❌ [OrganizationService] Failed to update plan tier", "error", err)
		return err
	}

	s.logger.Info("✅ [OrganizationService] Plan tier updated", "org_id", orgID, "tier", tier)
	return nil
}

func (s *organizationService) UpdateBillingStatus(orgID uint, status config.BillingStatus) error {
	s.logger.Info("💳 [OrganizationService] Updating billing status", "org_id", orgID, "status", status)

	if err := s.orgRepo.UpdateBillingStatus(orgID, string(status)); err != nil {
		s.logger.Error("❌ [OrganizationService] Failed to update billing status", "error", err)
		return err
	}

	s.logger.Info("✅ [OrganizationService] Billing status updated", "org_id", orgID, "status", status)
	return nil
}

// ==================== Storage Management ====================

func (s *organizationService) GetStorageUsage(orgID uint) (used int64, limit int64, err error) {
	org, err := s.orgRepo.FindByID(orgID)
	if err != nil {
		return 0, 0, err
	}

	limits := org.GetPlanLimits()
	return org.UsedStorageBytes, limits.MaxStorageBytes, nil
}

// ==================== Counting Methods ====================

func (s *organizationService) CountMembers(orgID uint) (int64, error) {
	return s.orgRepo.CountMembers(orgID)
}

func (s *organizationService) CountGroups(orgID uint) (int64, error) {
	return s.groupRepo.CountByOrganization(orgID)
}

func (s *organizationService) GetUserOrganizations(userID uint) ([]models.OrganizationMember, error) {
	s.logger.Info("🏢 [OrganizationService] Getting user organizations", "user_id", userID)

	memberships, err := s.orgRepo.GetUserOrganizations(userID)
	if err != nil {
		s.logger.Error("❌ [OrganizationService] Failed to get user organizations", "error", err)
		return nil, err
	}

	s.logger.Info("✅ [OrganizationService] Found user organizations", "user_id", userID, "count", len(memberships))
	return memberships, nil
}
