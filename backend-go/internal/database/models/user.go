package models

import (
	"time"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"gorm.io/gorm"
)

// User represents the user domain entity
type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Username  string         `gorm:"uniqueIndex;not null" json:"username"`
	Email     string         `gorm:"uniqueIndex;not null" json:"email"`
	FullName  string         `gorm:"not null" json:"full_name"`
	Password  string         `gorm:"not null" json:"-"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Billing fields for individual user plans
	PlanTier         config.PlanTier      `gorm:"not null;default:FREE" json:"plan_tier"`
	BillingStatus    config.BillingStatus `gorm:"not null;default:active" json:"billing_status"`
	StripeCustomerID *string              `json:"stripe_customer_id,omitempty"`
	SubscriptionID   *string              `json:"subscription_id,omitempty"`
	CurrentPeriodEnd *time.Time           `json:"current_period_end,omitempty"`
	UsedStorageBytes int64                `gorm:"not null;default:0" json:"used_storage_bytes"`

	// Relationships
	OrganizationMemberships []OrganizationMember `gorm:"foreignKey:UserID" json:"organization_memberships,omitempty"`
	GroupMemberships        []GroupMember        `gorm:"foreignKey:UserID" json:"group_memberships,omitempty"`
}

// TableName overrides the table name
func (User) TableName() string {
	return "users"
}

// GetPlanLimits returns the plan limits for this user's tier
func (u *User) GetPlanLimits() config.UserPlanLimits {
	return config.GetUserPlanLimits(u.PlanTier)
}

// GetOrganizations returns all organizations the user belongs to
func (u *User) GetOrganizations() []Organization {
	orgs := make([]Organization, 0, len(u.OrganizationMemberships))
	for _, membership := range u.OrganizationMemberships {
		orgs = append(orgs, membership.Organization)
	}
	return orgs
}

// GetGroups returns all groups the user belongs to
func (u *User) GetGroups() []Group {
	groups := make([]Group, 0, len(u.GroupMemberships))
	for _, membership := range u.GroupMemberships {
		groups = append(groups, membership.Group)
	}
	return groups
}
