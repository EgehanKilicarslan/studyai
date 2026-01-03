package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
)

// OrganizationRepository defines the interface for organization data operations
type OrganizationRepository interface {
	// Organization CRUD
	Create(org *models.Organization) error
	FindByID(id uint) (*models.Organization, error)
	FindByDomain(domain string) (*models.Organization, error)
	Update(org *models.Organization) error
	Delete(id uint) error
	List(offset, limit int) ([]models.Organization, int64, error)

	// Organization Role operations
	CreateRole(role *models.OrganizationRole) error
	FindRoleByID(roleID uint) (*models.OrganizationRole, error)
	FindRoleByName(orgID uint, name string) (*models.OrganizationRole, error)
	ListRoles(orgID uint) ([]models.OrganizationRole, error)
	UpdateRole(role *models.OrganizationRole) error
	DeleteRole(roleID uint) error

	// Organization Member operations
	AddMember(member *models.OrganizationMember) error
	GetMember(orgID, userID uint) (*models.OrganizationMember, error)
	UpdateMemberRole(orgID, userID, roleID uint) error
	RemoveMember(orgID, userID uint) error
	ListMembers(orgID uint, offset, limit int) ([]models.OrganizationMember, int64, error)
	GetUserOrganizations(userID uint) ([]models.OrganizationMember, error)
	CountMembers(orgID uint) (int64, error)
	CountUserOrganizations(userID uint) (int64, error)

	// Storage operations (atomic)
	IncrementStorage(orgID uint, bytes int64) error
	DecrementStorage(orgID uint, bytes int64) error

	// Plan operations
	UpdatePlanTier(orgID uint, tier string) error
	UpdateBillingStatus(orgID uint, status string) error
}

type organizationRepository struct {
	db *gorm.DB
}

// NewOrganizationRepository creates a new organization repository instance
func NewOrganizationRepository(db *gorm.DB) OrganizationRepository {
	return &organizationRepository{db: db}
}

// ==================== Organization CRUD ====================

func (r *organizationRepository) Create(org *models.Organization) error {
	return r.db.Create(org).Error
}

func (r *organizationRepository) FindByID(id uint) (*models.Organization, error) {
	var org models.Organization
	err := r.db.Preload("Members").Preload("Groups").First(&org, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}
	return &org, nil
}

func (r *organizationRepository) FindByDomain(domain string) (*models.Organization, error) {
	var org models.Organization
	err := r.db.Where("domain = ?", domain).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}
	return &org, nil
}

func (r *organizationRepository) Update(org *models.Organization) error {
	result := r.db.Save(org)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

func (r *organizationRepository) Delete(id uint) error {
	result := r.db.Delete(&models.Organization{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

func (r *organizationRepository) List(offset, limit int) ([]models.Organization, int64, error) {
	var orgs []models.Organization
	var total int64

	if err := r.db.Model(&models.Organization{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Offset(offset).Limit(limit).Find(&orgs).Error
	return orgs, total, err
}

// ==================== Organization Member Operations ====================

func (r *organizationRepository) AddMember(member *models.OrganizationMember) error {
	member.JoinedAt = time.Now()
	return r.db.Create(member).Error
}

func (r *organizationRepository) GetMember(orgID, userID uint) (*models.OrganizationMember, error) {
	var member models.OrganizationMember
	err := r.db.Where("organization_id = ? AND user_id = ?", orgID, userID).
		Preload("User").
		Preload("Organization").
		First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}
	return &member, nil
}

func (r *organizationRepository) UpdateMemberRole(orgID, userID, roleID uint) error {
	result := r.db.Model(&models.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		Update("role_id", roleID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMemberNotFound
	}
	return nil
}

func (r *organizationRepository) RemoveMember(orgID, userID uint) error {
	result := r.db.Where("organization_id = ? AND user_id = ?", orgID, userID).
		Delete(&models.OrganizationMember{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMemberNotFound
	}
	return nil
}

func (r *organizationRepository) ListMembers(orgID uint, offset, limit int) ([]models.OrganizationMember, int64, error) {
	var members []models.OrganizationMember
	var total int64

	if err := r.db.Model(&models.OrganizationMember{}).
		Where("organization_id = ?", orgID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Where("organization_id = ?", orgID).
		Preload("User").
		Preload("Role").
		Offset(offset).
		Limit(limit).
		Find(&members).Error
	return members, total, err
}

func (r *organizationRepository) GetUserOrganizations(userID uint) ([]models.OrganizationMember, error) {
	var memberships []models.OrganizationMember
	err := r.db.Where("user_id = ?", userID).
		Preload("Organization").
		Preload("Role").
		Find(&memberships).Error
	return memberships, err
}

func (r *organizationRepository) CountMembers(orgID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.OrganizationMember{}).
		Where("organization_id = ?", orgID).
		Count(&count).Error
	return count, err
}

// ==================== Storage Operations (Atomic) ====================

func (r *organizationRepository) IncrementStorage(orgID uint, bytes int64) error {
	result := r.db.Model(&models.Organization{}).
		Where("id = ?", orgID).
		Update("used_storage_bytes", gorm.Expr("used_storage_bytes + ?", bytes))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

func (r *organizationRepository) DecrementStorage(orgID uint, bytes int64) error {
	// Use GREATEST to prevent going below 0
	result := r.db.Model(&models.Organization{}).
		Where("id = ?", orgID).
		Update("used_storage_bytes", gorm.Expr("GREATEST(used_storage_bytes - ?, 0)", bytes))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

// ==================== Plan Operations ====================

func (r *organizationRepository) UpdatePlanTier(orgID uint, tier string) error {
	result := r.db.Model(&models.Organization{}).
		Where("id = ?", orgID).
		Update("plan_tier", tier)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

func (r *organizationRepository) UpdateBillingStatus(orgID uint, status string) error {
	result := r.db.Model(&models.Organization{}).
		Where("id = ?", orgID).
		Update("billing_status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

// ==================== Organization Role Operations ====================

func (r *organizationRepository) CreateRole(role *models.OrganizationRole) error {
	return r.db.Create(role).Error
}

func (r *organizationRepository) FindRoleByID(roleID uint) (*models.OrganizationRole, error) {
	var role models.OrganizationRole
	if err := r.db.Where("id = ?", roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("role not found")
		}
		return nil, err
	}
	return &role, nil
}

func (r *organizationRepository) FindRoleByName(orgID uint, name string) (*models.OrganizationRole, error) {
	var role models.OrganizationRole
	if err := r.db.Where("organization_id = ? AND name = ?", orgID, name).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("role not found")
		}
		return nil, err
	}
	return &role, nil
}

func (r *organizationRepository) ListRoles(orgID uint) ([]models.OrganizationRole, error) {
	var roles []models.OrganizationRole
	if err := r.db.Where("organization_id = ?", orgID).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *organizationRepository) UpdateRole(role *models.OrganizationRole) error {
	return r.db.Save(role).Error
}

func (r *organizationRepository) DeleteRole(roleID uint) error {
	result := r.db.Delete(&models.OrganizationRole{}, roleID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("role not found")
	}
	return nil
}

func (r *organizationRepository) CountUserOrganizations(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.OrganizationMember{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}

// Repository errors for organizations
var (
	ErrOrganizationNotFound = errors.New("organization not found")
	ErrMemberNotFound       = errors.New("member not found")
	ErrMemberAlreadyExists  = errors.New("member already exists in organization")
)
