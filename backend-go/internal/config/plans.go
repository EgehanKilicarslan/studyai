package config

// PlanTier represents subscription tier levels
type PlanTier string

const (
	PlanFree       PlanTier = "FREE"
	PlanPro        PlanTier = "PRO"
	PlanEnterprise PlanTier = "ENTERPRISE"
)

// BillingStatus represents the billing state of an organization, group, or user
type BillingStatus string

const (
	BillingActive    BillingStatus = "active"
	BillingPastDue   BillingStatus = "past_due"
	BillingCanceled  BillingStatus = "canceled"
	BillingTrialing  BillingStatus = "trialing"
	BillingSuspended BillingStatus = "suspended"
)

// ==================== Organization Plan Limits ====================

// OrganizationPlanLimits defines the resource limits for organization plans
type OrganizationPlanLimits struct {
	MaxMembers           int   // Maximum members in an organization
	MaxGroups            int   // Maximum groups (sub-groups) in an organization
	MaxDocuments         int   // Maximum documents in an organization
	MaxStorageBytes      int64 // Global organization storage limit
	MaxFileSize          int64 // Per-upload file size limit
	DailyMessagesPerUser int   // Rate limit per user per day
}

// DefaultOrganizationPlanLimits returns the limits for organization plan tiers
var DefaultOrganizationPlanLimits = map[PlanTier]OrganizationPlanLimits{
	PlanFree: {
		MaxMembers:           5,
		MaxGroups:            1,
		MaxDocuments:         50,
		MaxStorageBytes:      500 * 1024 * 1024, // 500 MB
		MaxFileSize:          10 * 1024 * 1024,  // 10 MB
		DailyMessagesPerUser: 50,
	},
	PlanPro: {
		MaxMembers:           50,
		MaxGroups:            20,
		MaxDocuments:         500,
		MaxStorageBytes:      10 * 1024 * 1024 * 1024, // 10 GB
		MaxFileSize:          50 * 1024 * 1024,        // 50 MB
		DailyMessagesPerUser: 500,
	},
	PlanEnterprise: {
		MaxMembers:           -1, // Unlimited (-1 means no limit)
		MaxGroups:            -1,
		MaxDocuments:         -1,
		MaxStorageBytes:      -1,
		MaxFileSize:          100 * 1024 * 1024, // 100 MB
		DailyMessagesPerUser: -1,
	},
}

// GetOrganizationPlanLimits returns the organization limits for a given tier, defaulting to FREE if unknown
func GetOrganizationPlanLimits(tier PlanTier) OrganizationPlanLimits {
	if limits, ok := DefaultOrganizationPlanLimits[tier]; ok {
		return limits
	}
	return DefaultOrganizationPlanLimits[PlanFree]
}

// ==================== Group Plan Limits ====================

// GroupPlanLimits defines the resource limits for standalone group plans
// Note: Groups that belong to an organization inherit the organization's limits
type GroupPlanLimits struct {
	MaxMembers           int   // Maximum members in a standalone group
	MaxDocuments         int   // Maximum documents in a standalone group
	MaxStorageBytes      int64 // Global group storage limit
	MaxFileSize          int64 // Per-upload file size limit
	DailyMessagesPerUser int   // Rate limit per user per day
}

// DefaultGroupPlanLimits returns the limits for standalone group plan tiers
var DefaultGroupPlanLimits = map[PlanTier]GroupPlanLimits{
	PlanFree: {
		MaxMembers:           3,
		MaxDocuments:         20,
		MaxStorageBytes:      200 * 1024 * 1024, // 200 MB
		MaxFileSize:          5 * 1024 * 1024,   // 5 MB
		DailyMessagesPerUser: 30,
	},
	PlanPro: {
		MaxMembers:           25,
		MaxDocuments:         200,
		MaxStorageBytes:      5 * 1024 * 1024 * 1024, // 5 GB
		MaxFileSize:          25 * 1024 * 1024,       // 25 MB
		DailyMessagesPerUser: 300,
	},
	PlanEnterprise: {
		MaxMembers:           100,
		MaxDocuments:         1000,
		MaxStorageBytes:      50 * 1024 * 1024 * 1024, // 50 GB
		MaxFileSize:          100 * 1024 * 1024,       // 100 MB
		DailyMessagesPerUser: -1,                      // Unlimited
	},
}

// GetGroupPlanLimits returns the group limits for a given tier, defaulting to FREE if unknown
func GetGroupPlanLimits(tier PlanTier) GroupPlanLimits {
	if limits, ok := DefaultGroupPlanLimits[tier]; ok {
		return limits
	}
	return DefaultGroupPlanLimits[PlanFree]
}

// ==================== User Plan Limits ====================

// UserPlanLimits defines the resource limits for individual user plans
type UserPlanLimits struct {
	MaxStandaloneGroups   int   // Maximum standalone groups a user can create/own
	MaxDocuments          int   // Maximum personal documents
	MaxStorageBytes       int64 // Personal storage limit
	MaxFileSize           int64 // Per-upload file size limit
	DailyMessages         int   // Daily message limit for personal usage
	MaxOrganizations      int   // Maximum organizations a user can join
	CanCreateOrganization bool  // Whether user can create organizations
}

// DefaultUserPlanLimits returns the limits for individual user plan tiers
var DefaultUserPlanLimits = map[PlanTier]UserPlanLimits{
	PlanFree: {
		MaxStandaloneGroups:   1,
		MaxDocuments:          10,
		MaxStorageBytes:       100 * 1024 * 1024, // 100 MB
		MaxFileSize:           5 * 1024 * 1024,   // 5 MB
		DailyMessages:         20,
		MaxOrganizations:      3,
		CanCreateOrganization: false,
	},
	PlanPro: {
		MaxStandaloneGroups:   5,
		MaxDocuments:          100,
		MaxStorageBytes:       2 * 1024 * 1024 * 1024, // 2 GB
		MaxFileSize:           25 * 1024 * 1024,       // 25 MB
		DailyMessages:         200,
		MaxOrganizations:      10,
		CanCreateOrganization: true,
	},
	PlanEnterprise: {
		MaxStandaloneGroups:   -1, // Unlimited
		MaxDocuments:          -1,
		MaxStorageBytes:       -1,
		MaxFileSize:           100 * 1024 * 1024, // 100 MB
		DailyMessages:         -1,
		MaxOrganizations:      -1,
		CanCreateOrganization: true,
	},
}

// GetUserPlanLimits returns the user limits for a given tier, defaulting to FREE if unknown
func GetUserPlanLimits(tier PlanTier) UserPlanLimits {
	if limits, ok := DefaultUserPlanLimits[tier]; ok {
		return limits
	}
	return DefaultUserPlanLimits[PlanFree]
}

// IsValidTier checks if a tier is valid
func IsValidTier(tier PlanTier) bool {
	_, ok := DefaultOrganizationPlanLimits[tier]
	return ok
}

// QuotaError represents a quota limit exceeded error
type QuotaError struct {
	Resource string
	Limit    int64
	Current  int64
	Message  string
}

func (e *QuotaError) Error() string {
	return e.Message
}

// NewQuotaError creates a new quota error
func NewQuotaError(resource string, limit, current int64, message string) *QuotaError {
	return &QuotaError{
		Resource: resource,
		Limit:    limit,
		Current:  current,
		Message:  message,
	}
}
