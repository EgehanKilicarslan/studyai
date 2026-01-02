package config

// PlanTier represents subscription tier levels
type PlanTier string

const (
	PlanFree       PlanTier = "FREE"
	PlanPro        PlanTier = "PRO"
	PlanEnterprise PlanTier = "ENTERPRISE"
)

// BillingStatus represents the billing state of an organization
type BillingStatus string

const (
	BillingActive    BillingStatus = "active"
	BillingPastDue   BillingStatus = "past_due"
	BillingCanceled  BillingStatus = "canceled"
	BillingTrialing  BillingStatus = "trialing"
	BillingSuspended BillingStatus = "suspended"
)

// PlanLimits defines the resource limits for each plan tier
type PlanLimits struct {
	MaxMembers           int   // Maximum members in an organization
	MaxGroups            int   // Maximum groups in an organization
	MaxStorageBytes      int64 // Global organization storage limit
	MaxFileSize          int64 // Per-upload file size limit
	DailyMessagesPerUser int   // Rate limit per user per day
}

// DefaultPlanLimits returns the limits for a given plan tier
var DefaultPlanLimits = map[PlanTier]PlanLimits{
	PlanFree: {
		MaxMembers:           5,
		MaxGroups:            1,
		MaxStorageBytes:      500 * 1024 * 1024, // 500 MB
		MaxFileSize:          10 * 1024 * 1024,  // 10 MB
		DailyMessagesPerUser: 50,
	},
	PlanPro: {
		MaxMembers:           50,
		MaxGroups:            20,
		MaxStorageBytes:      10 * 1024 * 1024 * 1024, // 10 GB
		MaxFileSize:          50 * 1024 * 1024,        // 50 MB
		DailyMessagesPerUser: 500,
	},
	PlanEnterprise: {
		MaxMembers:           -1, // Unlimited (-1 means no limit)
		MaxGroups:            -1,
		MaxStorageBytes:      -1,
		MaxFileSize:          100 * 1024 * 1024, // 100 MB
		DailyMessagesPerUser: -1,
	},
}

// GetPlanLimits returns the limits for a given tier, defaulting to FREE if unknown
func GetPlanLimits(tier PlanTier) PlanLimits {
	if limits, ok := DefaultPlanLimits[tier]; ok {
		return limits
	}
	return DefaultPlanLimits[PlanFree]
}

// IsValidTier checks if a tier is valid
func IsValidTier(tier PlanTier) bool {
	_, ok := DefaultPlanLimits[tier]
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
