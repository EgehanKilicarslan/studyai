package service

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/lib/pq"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
)

// System-defined role names that cannot be deleted
const (
	SystemRoleOwner = "Owner"
	SystemRoleAdmin = "Admin"
)

// Permission constants for validation
const (
	PermRoleManage   = "ROLE_MANAGE"
	PermMemberManage = "MEMBER_MANAGE"
	PermGroupAdmin   = "GROUP_ADMIN"
)

// GroupService defines the interface for group business logic
type GroupService interface {
	// Group operations
	CreateGroup(orgID uint, name, description string, creatorUserID uint) (*models.Group, error)
	CreateStandaloneGroup(name, description string, creatorUserID uint) (*models.Group, error)
	GetGroup(groupID uint) (*models.Group, error)
	UpdateGroup(requesterID, groupID uint, name, description string) (*models.Group, error)
	DeleteGroup(requesterID, groupID uint) error
	ListGroupsByOrganization(orgID uint, page, pageSize int) ([]models.Group, int64, error)
	GetUserGroups(userID uint) ([]models.GroupMember, error)

	// Role operations
	CreateRole(requesterID, groupID uint, name string, permissions []string) (*models.GroupRole, error)
	UpdateRole(requesterID, groupID, roleID uint, name string, permissions []string) (*models.GroupRole, error)
	DeleteRole(requesterID, groupID, roleID uint) error
	ListRoles(groupID uint) ([]models.GroupRole, error)

	// Member operations
	AddMember(requesterID, groupID, userID, roleID uint) (*models.GroupMember, error)
	UpdateMemberRole(requesterID, groupID, userID, newRoleID uint) error
	RemoveMember(requesterID, groupID, userID uint) error
	ListMembers(groupID uint, page, pageSize int) ([]models.GroupMember, int64, error)

	// Permission checks
	HasPermission(userID, groupID uint, permission string) (bool, error)
	GetUserPermissions(userID, groupID uint) ([]string, error)

	// Billing/Limits checks for standalone groups
	GetGroupLimits(groupID uint) (*config.PlanLimits, error)
	CheckStorageLimit(groupID uint, additionalBytes int64) error
	UpdateStandaloneGroupTier(requesterID, groupID uint, planTier config.PlanTier, billingStatus config.BillingStatus) (*models.Group, error)
	UpdateStandaloneGroupBillingStatus(groupID uint, billingStatus config.BillingStatus) error
	GetStandaloneGroupQuota(groupID uint) (*StandaloneGroupQuota, error)
}

// StandaloneGroupQuota represents quota information for a standalone group
type StandaloneGroupQuota struct {
	Group  *models.Group
	Usage  GroupUsage
	Limits config.PlanLimits
}

// GroupUsage represents current resource usage for a group
type GroupUsage struct {
	Members      int64
	StorageBytes int64
}

type groupService struct {
	groupRepo repository.GroupRepository
	orgRepo   repository.OrganizationRepository
	userRepo  repository.UserRepository
	logger    *slog.Logger
}

// NewGroupService creates a new group service instance
func NewGroupService(
	groupRepo repository.GroupRepository,
	orgRepo repository.OrganizationRepository,
	userRepo repository.UserRepository,
	logger *slog.Logger,
) GroupService {
	return &groupService{
		groupRepo: groupRepo,
		orgRepo:   orgRepo,
		userRepo:  userRepo,
		logger:    logger,
	}
}

// getGroupLimits is a private helper that returns the applicable plan limits for a group.
// - If the group belongs to an organization, it returns the organization's plan limits.
// - If the group is standalone, it returns limits based on the group's own PlanTier.
func (s *groupService) getGroupLimits(group *models.Group) (config.PlanLimits, error) {
	if group.OrganizationID != nil {
		// Organization group: use organization's plan limits
		org, err := s.orgRepo.FindByID(*group.OrganizationID)
		if err != nil {
			s.logger.Error("❌ [GroupService] Failed to fetch organization for limits", "org_id", *group.OrganizationID, "error", err)
			return config.PlanLimits{}, err
		}
		return org.GetPlanLimits(), nil
	}

	// Standalone group: use the group's own plan tier
	return group.GetPlanLimits(), nil
}

// ==================== Group Operations ====================

func (s *groupService) CreateGroup(orgID uint, name, description string, creatorUserID uint) (*models.Group, error) {
	s.logger.Info("📁 [GroupService] Creating group", "org_id", orgID, "name", name, "creator", creatorUserID)

	// Check organization quota for groups
	org, err := s.orgRepo.FindByID(orgID)
	if err != nil {
		s.logger.Error("❌ [GroupService] Failed to fetch organization", "error", err)
		return nil, err
	}

	limits := org.GetPlanLimits()
	if limits.MaxGroups >= 0 { // -1 means unlimited
		currentCount, err := s.groupRepo.CountByOrganization(orgID)
		if err != nil {
			s.logger.Error("❌ [GroupService] Failed to count groups", "error", err)
			return nil, err
		}

		if int(currentCount) >= limits.MaxGroups {
			s.logger.Warn("⚠️ [GroupService] Group quota exceeded",
				"org_id", orgID,
				"current", currentCount,
				"limit", limits.MaxGroups,
			)
			return nil, config.NewQuotaError(
				"groups",
				int64(limits.MaxGroups),
				currentCount,
				fmt.Sprintf("Group limit reached. Your %s plan allows %d groups.", org.PlanTier, limits.MaxGroups),
			)
		}
	}

	// Create the group
	group := &models.Group{
		OrganizationID: &orgID,
		Name:           name,
		Description:    &description,
	}

	if err := s.groupRepo.Create(group); err != nil {
		s.logger.Error("❌ [GroupService] Failed to create group", "error", err)
		return nil, err
	}

	// Create default system roles for the group
	ownerRole := &models.GroupRole{
		GroupID:     group.ID,
		Name:        SystemRoleOwner,
		Permissions: pq.StringArray{models.PermGroupAdmin, models.PermDocRead, models.PermDocUpload, models.PermDocDelete, models.PermDocEdit, models.PermChatAccess, models.PermMemberAdd, models.PermMemberRemove, models.PermRoleManage},
	}
	if err := s.groupRepo.CreateRole(ownerRole); err != nil {
		s.logger.Error("❌ [GroupService] Failed to create owner role", "error", err)
		return nil, err
	}

	adminRole := &models.GroupRole{
		GroupID:     group.ID,
		Name:        SystemRoleAdmin,
		Permissions: pq.StringArray{models.PermDocRead, models.PermDocUpload, models.PermDocDelete, models.PermDocEdit, models.PermChatAccess, models.PermMemberAdd, models.PermMemberRemove, models.PermRoleManage},
	}
	if err := s.groupRepo.CreateRole(adminRole); err != nil {
		s.logger.Error("❌ [GroupService] Failed to create admin role", "error", err)
		return nil, err
	}

	// Add creator as owner
	member := &models.GroupMember{
		UserID:  creatorUserID,
		GroupID: group.ID,
		RoleID:  ownerRole.ID,
	}
	if err := s.groupRepo.AddMember(member); err != nil {
		s.logger.Error("❌ [GroupService] Failed to add creator as owner", "error", err)
		return nil, err
	}

	s.logger.Info("✅ [GroupService] Group created successfully", "group_id", group.ID)
	return group, nil
}

func (s *groupService) GetGroup(groupID uint) (*models.Group, error) {
	return s.groupRepo.FindByID(groupID)
}

func (s *groupService) UpdateGroup(requesterID, groupID uint, name, description string) (*models.Group, error) {
	s.logger.Info("📝 [GroupService] Updating group", "group_id", groupID, "requester", requesterID)

	// Check permission
	if hasAdmin, _ := s.HasPermission(requesterID, groupID, PermGroupAdmin); !hasAdmin {
		return nil, ErrPermissionDenied
	}

	group, err := s.groupRepo.FindByID(groupID)
	if err != nil {
		return nil, err
	}

	if name != "" {
		group.Name = name
	}
	if description != "" {
		group.Description = &description
	}

	if err := s.groupRepo.Update(group); err != nil {
		return nil, err
	}

	s.logger.Info("✅ [GroupService] Group updated successfully", "group_id", groupID)
	return group, nil
}

func (s *groupService) DeleteGroup(requesterID, groupID uint) error {
	s.logger.Info("🗑️ [GroupService] Deleting group", "group_id", groupID, "requester", requesterID)

	// Only Owner can delete the group
	member, err := s.groupRepo.GetMember(requesterID, groupID)
	if err != nil {
		return ErrPermissionDenied
	}

	role, err := s.groupRepo.FindRoleByID(member.RoleID)
	if err != nil {
		return ErrPermissionDenied
	}

	if role.Name != SystemRoleOwner {
		return ErrPermissionDenied
	}

	if err := s.groupRepo.Delete(groupID); err != nil {
		return err
	}

	s.logger.Info("✅ [GroupService] Group deleted successfully", "group_id", groupID)
	return nil
}

func (s *groupService) ListGroupsByOrganization(orgID uint, page, pageSize int) ([]models.Group, int64, error) {
	offset := (page - 1) * pageSize
	return s.groupRepo.ListByOrganization(orgID, offset, pageSize)
}

func (s *groupService) GetUserGroups(userID uint) ([]models.GroupMember, error) {
	return s.groupRepo.GetUserGroups(userID)
}

// ==================== Role Operations ====================

func (s *groupService) CreateRole(requesterID, groupID uint, name string, permissions []string) (*models.GroupRole, error) {
	s.logger.Info("🎭 [GroupService] Creating role", "group_id", groupID, "name", name, "requester", requesterID)

	// Check permission
	if hasPermission, _ := s.HasPermission(requesterID, groupID, PermRoleManage); !hasPermission {
		return nil, ErrPermissionDenied
	}

	// Validate requester can assign these permissions
	if err := s.validatePermissionAssignment(requesterID, groupID, permissions); err != nil {
		return nil, err
	}

	// Check for duplicate role name
	existingRole, err := s.groupRepo.FindRoleByName(groupID, name)
	if err == nil && existingRole != nil {
		return nil, ErrRoleNameExists
	}

	role := &models.GroupRole{
		GroupID:     groupID,
		Name:        name,
		Permissions: pq.StringArray(permissions),
	}

	if err := s.groupRepo.CreateRole(role); err != nil {
		s.logger.Error("❌ [GroupService] Failed to create role", "error", err)
		return nil, err
	}

	s.logger.Info("✅ [GroupService] Role created successfully", "role_id", role.ID)
	return role, nil
}

func (s *groupService) UpdateRole(requesterID, groupID, roleID uint, name string, permissions []string) (*models.GroupRole, error) {
	s.logger.Info("📝 [GroupService] Updating role", "role_id", roleID, "requester", requesterID)

	// Check permission
	if hasPermission, _ := s.HasPermission(requesterID, groupID, PermRoleManage); !hasPermission {
		return nil, ErrPermissionDenied
	}

	// Find existing role
	role, err := s.groupRepo.FindRoleByID(roleID)
	if err != nil {
		return nil, err
	}

	// Prevent modification of system roles
	if role.Name == SystemRoleOwner || role.Name == SystemRoleAdmin {
		return nil, ErrSystemRoleProtected
	}

	// Validate role belongs to the group
	if role.GroupID != groupID {
		return nil, repository.ErrRoleNotFound
	}

	// Validate permission assignment
	if permissions != nil {
		if err := s.validatePermissionAssignment(requesterID, groupID, permissions); err != nil {
			return nil, err
		}
	}

	// Update the role
	if err := s.groupRepo.UpdateRole(roleID, name, pq.StringArray(permissions)); err != nil {
		return nil, err
	}

	// Fetch updated role
	updatedRole, err := s.groupRepo.FindRoleByID(roleID)
	if err != nil {
		return nil, err
	}

	s.logger.Info("✅ [GroupService] Role updated successfully", "role_id", roleID)
	return updatedRole, nil
}

func (s *groupService) DeleteRole(requesterID, groupID, roleID uint) error {
	s.logger.Info("🗑️ [GroupService] Deleting role", "role_id", roleID, "requester", requesterID)

	// Check permission
	if hasPermission, _ := s.HasPermission(requesterID, groupID, PermRoleManage); !hasPermission {
		return ErrPermissionDenied
	}

	// Find the role
	role, err := s.groupRepo.FindRoleByID(roleID)
	if err != nil {
		return err
	}

	// Prevent deletion of system roles
	if role.Name == SystemRoleOwner || role.Name == SystemRoleAdmin {
		return ErrSystemRoleProtected
	}

	// Validate role belongs to the group
	if role.GroupID != groupID {
		return repository.ErrRoleNotFound
	}

	// Check if any members are using this role
	memberCount, err := s.groupRepo.GetRoleMemberCount(roleID)
	if err != nil {
		return err
	}

	if memberCount > 0 {
		return ErrRoleInUse
	}

	if err := s.groupRepo.DeleteRole(roleID); err != nil {
		return err
	}

	s.logger.Info("✅ [GroupService] Role deleted successfully", "role_id", roleID)
	return nil
}

func (s *groupService) ListRoles(groupID uint) ([]models.GroupRole, error) {
	return s.groupRepo.ListRoles(groupID)
}

// ==================== Member Operations ====================

func (s *groupService) AddMember(requesterID, groupID, userID, roleID uint) (*models.GroupMember, error) {
	s.logger.Info("👤 [GroupService] Adding member", "group_id", groupID, "user_id", userID, "requester", requesterID)

	// Check permission
	if hasPermission, _ := s.HasPermission(requesterID, groupID, PermMemberManage); !hasPermission {
		if hasPermission, _ := s.HasPermission(requesterID, groupID, models.PermMemberAdd); !hasPermission {
			return nil, ErrPermissionDenied
		}
	}

	// Fetch the group to check limits
	group, err := s.groupRepo.FindByID(groupID)
	if err != nil {
		s.logger.Error("❌ [GroupService] Failed to fetch group", "group_id", groupID, "error", err)
		return nil, err
	}

	// Check member quota based on group type (org-based or standalone)
	limits, err := s.getGroupLimits(group)
	if err != nil {
		return nil, err
	}

	if limits.MaxMembers >= 0 { // -1 means unlimited
		currentCount, err := s.groupRepo.CountMembers(groupID)
		if err != nil {
			s.logger.Error("❌ [GroupService] Failed to count members", "group_id", groupID, "error", err)
			return nil, err
		}

		if int(currentCount) >= limits.MaxMembers {
			var planSource string
			var planTier config.PlanTier
			if group.OrganizationID != nil {
				org, _ := s.orgRepo.FindByID(*group.OrganizationID)
				if org != nil {
					planTier = org.PlanTier
					planSource = "organization's"
				}
			} else {
				planTier = group.PlanTier
				planSource = "group's"
			}

			s.logger.Warn("⚠️ [GroupService] Member quota exceeded",
				"group_id", groupID,
				"current", currentCount,
				"limit", limits.MaxMembers,
				"plan_tier", planTier,
			)
			return nil, config.NewQuotaError(
				"members",
				int64(limits.MaxMembers),
				currentCount,
				fmt.Sprintf("Member limit reached. Your %s %s plan allows %d members per group.", planSource, planTier, limits.MaxMembers),
			)
		}
	}

	// Verify user exists
	if _, err := s.userRepo.FindByID(userID); err != nil {
		return nil, repository.ErrUserNotFound
	}

	// Verify role exists and belongs to group
	role, err := s.groupRepo.FindRoleByID(roleID)
	if err != nil {
		return nil, err
	}
	if role.GroupID != groupID {
		return nil, repository.ErrRoleNotFound
	}

	// Validate permission escalation
	rolePerms := []string(role.Permissions)
	if err := s.validatePermissionAssignment(requesterID, groupID, rolePerms); err != nil {
		return nil, err
	}

	// Check if user is already a member
	existingMember, _ := s.groupRepo.GetMember(userID, groupID)
	if existingMember != nil {
		return nil, ErrMemberAlreadyExists
	}

	member := &models.GroupMember{
		UserID:  userID,
		GroupID: groupID,
		RoleID:  roleID,
	}

	if err := s.groupRepo.AddMember(member); err != nil {
		s.logger.Error("❌ [GroupService] Failed to add member", "error", err)
		return nil, err
	}

	// Reload with associations
	member, _ = s.groupRepo.GetMember(userID, groupID)

	s.logger.Info("✅ [GroupService] Member added successfully", "user_id", userID, "group_id", groupID)
	return member, nil
}

func (s *groupService) UpdateMemberRole(requesterID, groupID, userID, newRoleID uint) error {
	s.logger.Info("📝 [GroupService] Updating member role", "user_id", userID, "new_role", newRoleID, "requester", requesterID)

	// Check permission
	if hasPermission, _ := s.HasPermission(requesterID, groupID, PermMemberManage); !hasPermission {
		return ErrPermissionDenied
	}

	// Get the target member
	targetMember, err := s.groupRepo.GetMember(userID, groupID)
	if err != nil {
		return err
	}

	// Get target's current role
	currentRole, err := s.groupRepo.FindRoleByID(targetMember.RoleID)
	if err != nil {
		return err
	}

	// Prevent demoting an Owner
	if currentRole.Name == SystemRoleOwner && requesterID != userID {
		return ErrCannotModifyOwner
	}

	// Verify new role exists and belongs to group
	newRole, err := s.groupRepo.FindRoleByID(newRoleID)
	if err != nil {
		return err
	}
	if newRole.GroupID != groupID {
		return repository.ErrRoleNotFound
	}

	// Validate permission escalation
	newRolePerms := []string(newRole.Permissions)
	if err := s.validatePermissionAssignment(requesterID, groupID, newRolePerms); err != nil {
		return err
	}

	if err := s.groupRepo.UpdateMemberRole(userID, groupID, newRoleID); err != nil {
		return err
	}

	s.logger.Info("✅ [GroupService] Member role updated successfully", "user_id", userID)
	return nil
}

func (s *groupService) RemoveMember(requesterID, groupID, userID uint) error {
	s.logger.Info("🚫 [GroupService] Removing member", "user_id", userID, "group_id", groupID, "requester", requesterID)

	// Check permission (unless removing self)
	if requesterID != userID {
		if hasPermission, _ := s.HasPermission(requesterID, groupID, PermMemberManage); !hasPermission {
			if hasPermission, _ := s.HasPermission(requesterID, groupID, models.PermMemberRemove); !hasPermission {
				return ErrPermissionDenied
			}
		}
	}

	// Get the target member
	targetMember, err := s.groupRepo.GetMember(userID, groupID)
	if err != nil {
		return err
	}

	// Get target's role
	targetRole, err := s.groupRepo.FindRoleByID(targetMember.RoleID)
	if err != nil {
		return err
	}

	// Prevent removing the Owner (unless they're removing themselves and there's another owner)
	if targetRole.Name == SystemRoleOwner {
		return ErrCannotRemoveOwner
	}

	if err := s.groupRepo.RemoveMember(userID, groupID); err != nil {
		return err
	}

	s.logger.Info("✅ [GroupService] Member removed successfully", "user_id", userID)
	return nil
}

func (s *groupService) ListMembers(groupID uint, page, pageSize int) ([]models.GroupMember, int64, error) {
	offset := (page - 1) * pageSize
	return s.groupRepo.ListMembers(groupID, offset, pageSize)
}

// ==================== Permission Helpers ====================

func (s *groupService) HasPermission(userID, groupID uint, permission string) (bool, error) {
	permissions, err := s.groupRepo.GetUserPermissionsInGroup(userID, groupID)
	if err != nil {
		return false, err
	}

	for _, p := range permissions {
		if p == permission || p == PermGroupAdmin {
			return true, nil
		}
	}

	return false, nil
}

func (s *groupService) GetUserPermissions(userID, groupID uint) ([]string, error) {
	return s.groupRepo.GetUserPermissionsInGroup(userID, groupID)
}

// validatePermissionAssignment ensures a user cannot assign permissions higher than their own
func (s *groupService) validatePermissionAssignment(requesterID, groupID uint, permissions []string) error {
	requesterPerms, err := s.groupRepo.GetUserPermissionsInGroup(requesterID, groupID)
	if err != nil {
		return ErrPermissionDenied
	}

	// If requester has GROUP_ADMIN, they can assign any permission
	for _, p := range requesterPerms {
		if p == PermGroupAdmin {
			return nil
		}
	}

	// Create a map of requester's permissions for quick lookup
	requesterPermsMap := make(map[string]bool)
	for _, p := range requesterPerms {
		requesterPermsMap[p] = true
	}

	// Check each permission being assigned
	for _, perm := range permissions {
		if perm == PermGroupAdmin {
			return ErrCannotEscalatePermissions
		}
		if !requesterPermsMap[perm] {
			return ErrCannotEscalatePermissions
		}
	}

	return nil
}

// ==================== Standalone Group Operations ====================

// CreateStandaloneGroup creates a new standalone group (not belonging to any organization)
// with a default FREE plan tier
func (s *groupService) CreateStandaloneGroup(name, description string, creatorUserID uint) (*models.Group, error) {
	s.logger.Info("📁 [GroupService] Creating standalone group", "name", name, "creator", creatorUserID)

	// Create the standalone group with default FREE plan
	group := &models.Group{
		OrganizationID:   nil, // Standalone group
		Name:             name,
		Description:      &description,
		PlanTier:         config.PlanFree,
		BillingStatus:    config.BillingActive,
		UsedStorageBytes: 0,
	}

	if err := s.groupRepo.Create(group); err != nil {
		s.logger.Error("❌ [GroupService] Failed to create standalone group", "error", err)
		return nil, err
	}

	// Create default system roles for the group
	ownerRole := &models.GroupRole{
		GroupID:     group.ID,
		Name:        SystemRoleOwner,
		Permissions: pq.StringArray{models.PermGroupAdmin, models.PermDocRead, models.PermDocUpload, models.PermDocDelete, models.PermDocEdit, models.PermChatAccess, models.PermMemberAdd, models.PermMemberRemove, models.PermRoleManage},
	}
	if err := s.groupRepo.CreateRole(ownerRole); err != nil {
		s.logger.Error("❌ [GroupService] Failed to create owner role", "error", err)
		return nil, err
	}

	adminRole := &models.GroupRole{
		GroupID:     group.ID,
		Name:        SystemRoleAdmin,
		Permissions: pq.StringArray{models.PermDocRead, models.PermDocUpload, models.PermDocDelete, models.PermDocEdit, models.PermChatAccess, models.PermMemberAdd, models.PermMemberRemove, models.PermRoleManage},
	}
	if err := s.groupRepo.CreateRole(adminRole); err != nil {
		s.logger.Error("❌ [GroupService] Failed to create admin role", "error", err)
		return nil, err
	}

	// Add creator as owner
	member := &models.GroupMember{
		UserID:  creatorUserID,
		GroupID: group.ID,
		RoleID:  ownerRole.ID,
	}
	if err := s.groupRepo.AddMember(member); err != nil {
		s.logger.Error("❌ [GroupService] Failed to add creator as owner", "error", err)
		return nil, err
	}

	s.logger.Info("✅ [GroupService] Standalone group created successfully", "group_id", group.ID, "plan_tier", group.PlanTier)
	return group, nil
}

// ==================== Billing/Limits Helpers ====================

// GetGroupLimits returns the plan limits for a group.
// For standalone groups, it returns the group's own plan limits.
// For organization groups, it returns nil (organization limits should be used instead).
func (s *groupService) GetGroupLimits(groupID uint) (*config.PlanLimits, error) {
	group, err := s.groupRepo.FindByID(groupID)
	if err != nil {
		return nil, err
	}

	if !group.IsStandalone() {
		// For organization groups, return nil - organization limits should be checked
		return nil, nil
	}

	limits := group.GetPlanLimits()
	return &limits, nil
}

// CheckStorageLimit verifies if a standalone group has enough storage capacity
// for an additional file of the given size. Returns a QuotaError if the limit
// would be exceeded.
func (s *groupService) CheckStorageLimit(groupID uint, additionalBytes int64) error {
	group, err := s.groupRepo.FindByID(groupID)
	if err != nil {
		return err
	}

	if !group.IsStandalone() {
		// For organization groups, storage is managed at the organization level
		return nil
	}

	limits := group.GetPlanLimits()

	// Check if unlimited storage (-1)
	if limits.MaxStorageBytes < 0 {
		return nil
	}

	newTotal := group.UsedStorageBytes + additionalBytes
	if newTotal > limits.MaxStorageBytes {
		s.logger.Warn("⚠️ [GroupService] Storage quota exceeded for standalone group",
			"group_id", groupID,
			"current", group.UsedStorageBytes,
			"additional", additionalBytes,
			"limit", limits.MaxStorageBytes,
		)
		return config.NewQuotaError(
			"storage",
			limits.MaxStorageBytes,
			group.UsedStorageBytes,
			fmt.Sprintf("Storage limit reached. Your %s plan allows %d bytes.", group.PlanTier, limits.MaxStorageBytes),
		)
	}

	return nil
}

// UpdateStandaloneGroupTier updates the plan tier and billing status for a standalone group.
// Returns an error if the group belongs to an organization (billing must be handled at org level).
func (s *groupService) UpdateStandaloneGroupTier(requesterID, groupID uint, planTier config.PlanTier, billingStatus config.BillingStatus) (*models.Group, error) {
	s.logger.Info("💳 [GroupService] Updating standalone group tier", "group_id", groupID, "plan_tier", planTier, "requester", requesterID)

	// Check permission - only owner or admin can update tier
	if hasAdmin, _ := s.HasPermission(requesterID, groupID, PermGroupAdmin); !hasAdmin {
		return nil, ErrPermissionDenied
	}

	// Fetch the group
	group, err := s.groupRepo.FindByID(groupID)
	if err != nil {
		s.logger.Error("❌ [GroupService] Failed to fetch group", "group_id", groupID, "error", err)
		return nil, err
	}

	// Guard clause: reject if this is an organization group
	if !group.IsStandalone() {
		s.logger.Warn("⚠️ [GroupService] Attempted to update tier of organization group", "group_id", groupID, "org_id", *group.OrganizationID)
		return nil, ErrOrganizationGroupBilling
	}

	// Validate the plan tier
	if !config.IsValidTier(planTier) {
		return nil, ErrInvalidPlanTier
	}

	// Update the group's billing fields
	group.PlanTier = planTier
	group.BillingStatus = billingStatus

	if err := s.groupRepo.Update(group); err != nil {
		s.logger.Error("❌ [GroupService] Failed to update group tier", "group_id", groupID, "error", err)
		return nil, err
	}

	s.logger.Info("✅ [GroupService] Standalone group tier updated successfully", "group_id", groupID, "plan_tier", planTier, "billing_status", billingStatus)
	return group, nil
}

// UpdateStandaloneGroupBillingStatus updates the billing status for a standalone group.
// Returns an error if the group belongs to an organization.
func (s *groupService) UpdateStandaloneGroupBillingStatus(groupID uint, billingStatus config.BillingStatus) error {
	s.logger.Info("💳 [GroupService] Updating standalone group billing status", "group_id", groupID, "billing_status", billingStatus)

	// Fetch the group
	group, err := s.groupRepo.FindByID(groupID)
	if err != nil {
		s.logger.Error("❌ [GroupService] Failed to fetch group", "group_id", groupID, "error", err)
		return err
	}

	// Guard clause: reject if this is an organization group
	if !group.IsStandalone() {
		s.logger.Warn("⚠️ [GroupService] Attempted to update billing status of organization group", "group_id", groupID, "org_id", *group.OrganizationID)
		return ErrOrganizationGroupBilling
	}

	// Update the billing status
	group.BillingStatus = billingStatus

	if err := s.groupRepo.Update(group); err != nil {
		s.logger.Error("❌ [GroupService] Failed to update group billing status", "group_id", groupID, "error", err)
		return err
	}

	s.logger.Info("✅ [GroupService] Standalone group billing status updated successfully", "group_id", groupID, "billing_status", billingStatus)
	return nil
}

// GetStandaloneGroupQuota returns quota information for a standalone group.
// Returns an error if the group belongs to an organization.
func (s *groupService) GetStandaloneGroupQuota(groupID uint) (*StandaloneGroupQuota, error) {
	s.logger.Info("📊 [GroupService] Getting standalone group quota", "group_id", groupID)

	// Fetch the group
	group, err := s.groupRepo.FindByID(groupID)
	if err != nil {
		s.logger.Error("❌ [GroupService] Failed to fetch group", "group_id", groupID, "error", err)
		return nil, err
	}

	// Guard clause: reject if this is an organization group
	if !group.IsStandalone() {
		s.logger.Warn("⚠️ [GroupService] Attempted to get quota of organization group", "group_id", groupID, "org_id", *group.OrganizationID)
		return nil, ErrOrganizationGroupBilling
	}

	// Get member count
	memberCount, err := s.groupRepo.CountMembers(groupID)
	if err != nil {
		s.logger.Error("❌ [GroupService] Failed to count members", "group_id", groupID, "error", err)
		memberCount = 0
	}

	limits := group.GetPlanLimits()

	return &StandaloneGroupQuota{
		Group: group,
		Usage: GroupUsage{
			Members:      memberCount,
			StorageBytes: group.UsedStorageBytes,
		},
		Limits: limits,
	}, nil
}

// Service errors
var (
	ErrPermissionDenied          = errors.New("permission denied")
	ErrSystemRoleProtected       = errors.New("system roles (Owner, Admin) cannot be modified or deleted")
	ErrRoleInUse                 = errors.New("role is assigned to members and cannot be deleted")
	ErrRoleNameExists            = errors.New("role with this name already exists")
	ErrMemberAlreadyExists       = errors.New("user is already a member of this group")
	ErrCannotEscalatePermissions = errors.New("cannot assign permissions higher than your own")
	ErrCannotModifyOwner         = errors.New("cannot modify the owner's role")
	ErrCannotRemoveOwner         = errors.New("cannot remove the group owner")
	ErrOrganizationGroupBilling  = errors.New("this group is managed by an organization. Billing must be handled at the organization level")
	ErrInvalidPlanTier           = errors.New("invalid plan tier")
)
