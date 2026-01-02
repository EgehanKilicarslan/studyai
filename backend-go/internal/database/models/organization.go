package models

import (
	"time"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Organization represents a tenant in the multi-tenant SaaS architecture
type Organization struct {
	ID               uint                 `gorm:"primarykey" json:"id"`
	Name             string               `gorm:"not null" json:"name"`
	Domain           *string              `gorm:"uniqueIndex" json:"domain,omitempty"`
	PlanTier         config.PlanTier      `gorm:"not null;default:FREE" json:"plan_tier"`
	BillingStatus    config.BillingStatus `gorm:"not null;default:active" json:"billing_status"`
	UsedStorageBytes int64                `gorm:"not null;default:0" json:"used_storage_bytes"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
	DeletedAt        gorm.DeletedAt       `gorm:"index" json:"-"`

	// Relationships
	Members []OrganizationMember `gorm:"foreignKey:OrganizationID" json:"members,omitempty"`
	Groups  []Group              `gorm:"foreignKey:OrganizationID" json:"groups,omitempty"`
	Roles   []OrganizationRole   `gorm:"foreignKey:OrganizationID" json:"roles,omitempty"`
}

// TableName overrides the table name
func (Organization) TableName() string {
	return "organizations"
}

// GetPlanLimits returns the plan limits for this organization's tier
func (o *Organization) GetPlanLimits() config.PlanLimits {
	return config.GetPlanLimits(o.PlanTier)
}

// OrganizationRole represents a dynamic role within an organization with configurable permissions
type OrganizationRole struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	OrganizationID uint           `gorm:"not null;index" json:"organization_id"`
	Name           string         `gorm:"not null" json:"name"`
	Permissions    pq.StringArray `gorm:"type:text[];default:'{}'" json:"permissions"`
	IsSystemRole   bool           `gorm:"not null;default:false" json:"is_system_role"` // true for owner/admin/member
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Organization Organization         `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Members      []OrganizationMember `gorm:"foreignKey:RoleID" json:"members,omitempty"`
}

// TableName overrides the table name
func (OrganizationRole) TableName() string {
	return "organization_roles"
}

// OrganizationMember represents the membership relationship between users and organizations
type OrganizationMember struct {
	OrganizationID uint           `gorm:"primaryKey;not null" json:"organization_id"`
	UserID         uint           `gorm:"primaryKey;not null" json:"user_id"`
	RoleID         uint           `gorm:"not null;index" json:"role_id"`
	JoinedAt       time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"joined_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Organization Organization     `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	User         User             `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Role         OrganizationRole `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

// TableName overrides the table name
func (OrganizationMember) TableName() string {
	return "organization_members"
}

// System role name constants for organization-level roles
const (
	OrgRoleOwner  = "owner"
	OrgRoleAdmin  = "admin"
	OrgRoleMember = "member"
)
