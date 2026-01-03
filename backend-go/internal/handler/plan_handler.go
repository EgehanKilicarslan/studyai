package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
)

// PlanHandler handles public plan information API requests
type PlanHandler struct{}

// NewPlanHandler creates a new plan handler
func NewPlanHandler() *PlanHandler {
	return &PlanHandler{}
}

// GetAllPlans handles GET /plans - returns all available plan tiers and their limits
func (h *PlanHandler) GetAllPlans(c *gin.Context) {
	plans := []gin.H{}

	for _, tier := range []config.PlanTier{config.PlanFree, config.PlanPro, config.PlanEnterprise} {
		userLimits := config.GetUserPlanLimits(tier)
		orgLimits := config.GetOrganizationPlanLimits(tier)
		groupLimits := config.GetGroupPlanLimits(tier)

		plans = append(plans, gin.H{
			"tier": tier,
			"user_limits": gin.H{
				"max_standalone_groups":   userLimits.MaxStandaloneGroups,
				"max_documents":           userLimits.MaxDocuments,
				"max_storage_bytes":       userLimits.MaxStorageBytes,
				"max_file_size":           userLimits.MaxFileSize,
				"daily_messages":          userLimits.DailyMessages,
				"max_organizations":       userLimits.MaxOrganizations,
				"can_create_organization": userLimits.CanCreateOrganization,
			},
			"organization_limits": gin.H{
				"max_members":             orgLimits.MaxMembers,
				"max_groups":              orgLimits.MaxGroups,
				"max_documents":           orgLimits.MaxDocuments,
				"max_storage_bytes":       orgLimits.MaxStorageBytes,
				"max_file_size":           orgLimits.MaxFileSize,
				"daily_messages_per_user": orgLimits.DailyMessagesPerUser,
			},
			"group_limits": gin.H{
				"max_members":             groupLimits.MaxMembers,
				"max_documents":           groupLimits.MaxDocuments,
				"max_storage_bytes":       groupLimits.MaxStorageBytes,
				"max_file_size":           groupLimits.MaxFileSize,
				"daily_messages_per_user": groupLimits.DailyMessagesPerUser,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"plans": plans,
	})
}

// GetUserPlans handles GET /plans/users - returns user plan tiers and their limits
func (h *PlanHandler) GetUserPlans(c *gin.Context) {
	plans := []gin.H{}

	for _, tier := range []config.PlanTier{config.PlanFree, config.PlanPro, config.PlanEnterprise} {
		limits := config.GetUserPlanLimits(tier)

		plans = append(plans, gin.H{
			"tier": tier,
			"limits": gin.H{
				"max_standalone_groups":   limits.MaxStandaloneGroups,
				"max_documents":           limits.MaxDocuments,
				"max_storage_bytes":       limits.MaxStorageBytes,
				"max_file_size":           limits.MaxFileSize,
				"daily_messages":          limits.DailyMessages,
				"max_organizations":       limits.MaxOrganizations,
				"can_create_organization": limits.CanCreateOrganization,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"plans": plans,
	})
}

// GetOrganizationPlans handles GET /plans/organizations - returns organization plan tiers and their limits
func (h *PlanHandler) GetOrganizationPlans(c *gin.Context) {
	plans := []gin.H{}

	for _, tier := range []config.PlanTier{config.PlanFree, config.PlanPro, config.PlanEnterprise} {
		limits := config.GetOrganizationPlanLimits(tier)

		plans = append(plans, gin.H{
			"tier": tier,
			"limits": gin.H{
				"max_members":             limits.MaxMembers,
				"max_groups":              limits.MaxGroups,
				"max_documents":           limits.MaxDocuments,
				"max_storage_bytes":       limits.MaxStorageBytes,
				"max_file_size":           limits.MaxFileSize,
				"daily_messages_per_user": limits.DailyMessagesPerUser,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"plans": plans,
	})
}

// GetGroupPlans handles GET /plans/groups - returns standalone group plan tiers and their limits
func (h *PlanHandler) GetGroupPlans(c *gin.Context) {
	plans := []gin.H{}

	for _, tier := range []config.PlanTier{config.PlanFree, config.PlanPro, config.PlanEnterprise} {
		limits := config.GetGroupPlanLimits(tier)

		plans = append(plans, gin.H{
			"tier": tier,
			"limits": gin.H{
				"max_members":             limits.MaxMembers,
				"max_documents":           limits.MaxDocuments,
				"max_storage_bytes":       limits.MaxStorageBytes,
				"max_file_size":           limits.MaxFileSize,
				"daily_messages_per_user": limits.DailyMessagesPerUser,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"plans": plans,
	})
}
