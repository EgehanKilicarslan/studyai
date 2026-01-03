package repository

import (
	"errors"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
)

// GroupRepository defines the interface for group data operations
type GroupRepository interface {
	// Group CRUD
	Create(group *models.Group) error
	FindByID(id uint) (*models.Group, error)
	Update(group *models.Group) error
	Delete(id uint) error
	ListByOrganization(orgID uint, offset, limit int) ([]models.Group, int64, error)
	CountByOrganization(orgID uint) (int64, error)
	CountStandaloneGroupsByOwner(ownerID uint) (int64, error)

	// Role operations
	CreateRole(role *models.GroupRole) error
	FindRoleByID(roleID uint) (*models.GroupRole, error)
	FindRoleByName(groupID uint, name string) (*models.GroupRole, error)
	UpdateRole(roleID uint, name string, permissions pq.StringArray) error
	DeleteRole(roleID uint) error
	ListRoles(groupID uint) ([]models.GroupRole, error)
	GetRoleMemberCount(roleID uint) (int64, error)

	// Member operations
	AddMember(member *models.GroupMember) error
	GetMember(userID, groupID uint) (*models.GroupMember, error)
	UpdateMemberRole(userID, groupID, newRoleID uint) error
	RemoveMember(userID, groupID uint) error
	ListMembers(groupID uint, offset, limit int) ([]models.GroupMember, int64, error)
	CountMembers(groupID uint) (int64, error)
	GetUserGroups(userID uint) ([]models.GroupMember, error)
	GetUserGroupsInOrganization(userID, orgID uint) ([]uint, error) // Returns group IDs

	// Permission helpers
	GetUserPermissionsInGroup(userID, groupID uint) ([]string, error)

	// Storage operations (for standalone groups)
	IncrementStorage(groupID uint, bytes int64) error
	DecrementStorage(groupID uint, bytes int64) error
}

type groupRepository struct {
	db *gorm.DB
}

// NewGroupRepository creates a new group repository instance
func NewGroupRepository(db *gorm.DB) GroupRepository {
	return &groupRepository{db: db}
}

// ==================== Group CRUD ====================

func (r *groupRepository) Create(group *models.Group) error {
	return r.db.Create(group).Error
}

func (r *groupRepository) FindByID(id uint) (*models.Group, error) {
	var group models.Group
	err := r.db.Preload("Organization").
		Preload("Roles").
		Preload("Members").
		First(&group, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	return &group, nil
}

func (r *groupRepository) Update(group *models.Group) error {
	result := r.db.Save(group)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGroupNotFound
	}
	return nil
}

func (r *groupRepository) Delete(id uint) error {
	result := r.db.Delete(&models.Group{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGroupNotFound
	}
	return nil
}

func (r *groupRepository) ListByOrganization(orgID uint, offset, limit int) ([]models.Group, int64, error) {
	var groups []models.Group
	var total int64

	if err := r.db.Model(&models.Group{}).
		Where("organization_id = ?", orgID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Where("organization_id = ?", orgID).
		Preload("Roles").
		Offset(offset).
		Limit(limit).
		Find(&groups).Error
	return groups, total, err
}

func (r *groupRepository) CountByOrganization(orgID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Group{}).
		Where("organization_id = ?", orgID).
		Count(&count).Error
	return count, err
}

// ==================== Role Operations ====================

func (r *groupRepository) CreateRole(role *models.GroupRole) error {
	return r.db.Create(role).Error
}

func (r *groupRepository) FindRoleByID(roleID uint) (*models.GroupRole, error) {
	var role models.GroupRole
	err := r.db.Preload("Group").First(&role, roleID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (r *groupRepository) FindRoleByName(groupID uint, name string) (*models.GroupRole, error) {
	var role models.GroupRole
	err := r.db.Where("group_id = ? AND name = ?", groupID, name).First(&role).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (r *groupRepository) UpdateRole(roleID uint, name string, permissions pq.StringArray) error {
	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}
	if name != "" {
		updates["name"] = name
	}
	if permissions != nil {
		updates["permissions"] = permissions
	}

	result := r.db.Model(&models.GroupRole{}).Where("id = ?", roleID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRoleNotFound
	}
	return nil
}

func (r *groupRepository) DeleteRole(roleID uint) error {
	result := r.db.Delete(&models.GroupRole{}, roleID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRoleNotFound
	}
	return nil
}

func (r *groupRepository) ListRoles(groupID uint) ([]models.GroupRole, error) {
	var roles []models.GroupRole
	err := r.db.Where("group_id = ?", groupID).Find(&roles).Error
	return roles, err
}

func (r *groupRepository) GetRoleMemberCount(roleID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.GroupMember{}).Where("role_id = ?", roleID).Count(&count).Error
	return count, err
}

// ==================== Member Operations ====================

func (r *groupRepository) AddMember(member *models.GroupMember) error {
	member.JoinedAt = time.Now()
	return r.db.Create(member).Error
}

func (r *groupRepository) GetMember(userID, groupID uint) (*models.GroupMember, error) {
	var member models.GroupMember
	err := r.db.Where("user_id = ? AND group_id = ?", userID, groupID).
		Preload("User").
		Preload("Group").
		Preload("Role").
		First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupMemberNotFound
		}
		return nil, err
	}
	return &member, nil
}

func (r *groupRepository) UpdateMemberRole(userID, groupID, newRoleID uint) error {
	result := r.db.Model(&models.GroupMember{}).
		Where("user_id = ? AND group_id = ?", userID, groupID).
		Updates(map[string]interface{}{
			"role_id":    newRoleID,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGroupMemberNotFound
	}
	return nil
}

func (r *groupRepository) RemoveMember(userID, groupID uint) error {
	result := r.db.Where("user_id = ? AND group_id = ?", userID, groupID).
		Delete(&models.GroupMember{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGroupMemberNotFound
	}
	return nil
}

func (r *groupRepository) ListMembers(groupID uint, offset, limit int) ([]models.GroupMember, int64, error) {
	var members []models.GroupMember
	var total int64

	if err := r.db.Model(&models.GroupMember{}).
		Where("group_id = ?", groupID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Where("group_id = ?", groupID).
		Preload("User").
		Preload("Role").
		Offset(offset).
		Limit(limit).
		Find(&members).Error
	return members, total, err
}

func (r *groupRepository) CountMembers(groupID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.GroupMember{}).
		Where("group_id = ?", groupID).
		Count(&count).Error
	return count, err
}

func (r *groupRepository) GetUserGroups(userID uint) ([]models.GroupMember, error) {
	var memberships []models.GroupMember
	err := r.db.Where("user_id = ?", userID).
		Preload("Group").
		Preload("Role").
		Find(&memberships).Error
	return memberships, err
}

func (r *groupRepository) GetUserGroupsInOrganization(userID, orgID uint) ([]uint, error) {
	var groupIDs []uint
	err := r.db.Model(&models.GroupMember{}).
		Joins("JOIN groups ON groups.id = group_members.group_id").
		Where("group_members.user_id = ? AND groups.organization_id = ?", userID, orgID).
		Pluck("group_members.group_id", &groupIDs).Error
	return groupIDs, err
}

// ==================== Permission Helpers ====================

func (r *groupRepository) GetUserPermissionsInGroup(userID, groupID uint) ([]string, error) {
	member, err := r.GetMember(userID, groupID)
	if err != nil {
		return nil, err
	}

	// Load the role with permissions
	var role models.GroupRole
	if err := r.db.First(&role, member.RoleID).Error; err != nil {
		return nil, err
	}

	return []string(role.Permissions), nil
}

// ==================== Storage Operations (for standalone groups) ====================

func (r *groupRepository) IncrementStorage(groupID uint, bytes int64) error {
	result := r.db.Model(&models.Group{}).
		Where("id = ?", groupID).
		Update("used_storage_bytes", gorm.Expr("used_storage_bytes + ?", bytes))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGroupNotFound
	}
	return nil
}

func (r *groupRepository) DecrementStorage(groupID uint, bytes int64) error {
	// Use GREATEST to prevent going below 0
	result := r.db.Model(&models.Group{}).
		Where("id = ?", groupID).
		Update("used_storage_bytes", gorm.Expr("GREATEST(used_storage_bytes - ?, 0)", bytes))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGroupNotFound
	}
	return nil
}

func (r *groupRepository) CountStandaloneGroupsByOwner(ownerID uint) (int64, error) {
	// Count standalone groups (organization_id IS NULL) where the user is the owner
	var count int64
	err := r.db.Model(&models.Group{}).
		Joins("JOIN group_members ON group_members.group_id = groups.id").
		Joins("JOIN group_roles ON group_roles.id = group_members.role_id").
		Where("groups.organization_id IS NULL").
		Where("group_members.user_id = ?", ownerID).
		Where("group_roles.name = ?", "Owner").
		Count(&count).Error
	return count, err
}

// Repository errors for groups
var (
	ErrGroupNotFound          = errors.New("group not found")
	ErrRoleNotFound           = errors.New("role not found")
	ErrGroupMemberNotFound    = errors.New("group member not found")
	ErrRoleInUse              = errors.New("role is assigned to members and cannot be deleted")
	ErrSystemRoleProtected    = errors.New("system roles (Owner, Admin) cannot be modified or deleted")
	ErrInsufficientPermission = errors.New("insufficient permissions to perform this action")
)
