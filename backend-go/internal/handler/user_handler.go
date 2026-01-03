package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
)

// UserHandler handles user API requests for profile and billing management
type UserHandler struct {
	userService service.UserService
	logger      *slog.Logger
}

// NewUserHandler creates a new user handler
func NewUserHandler(userService service.UserService, logger *slog.Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		logger:      logger,
	}
}

// GetProfile handles GET /me/profile - returns current user's profile and plan info
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [UserHandler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		h.logger.Error("❌ [UserHandler] Invalid user ID type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal error"})
		return
	}

	user, err := h.userService.GetUser(userIDUint)
	if err != nil {
		h.logger.Error("❌ [UserHandler] Failed to get user", "user_id", userIDUint, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	limits := user.GetPlanLimits()

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":                 user.ID,
			"username":           user.Username,
			"email":              user.Email,
			"full_name":          user.FullName,
			"plan_tier":          user.PlanTier,
			"billing_status":     user.BillingStatus,
			"used_storage_bytes": user.UsedStorageBytes,
			"created_at":         user.CreatedAt,
			"updated_at":         user.UpdatedAt,
		},
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

// GetQuota handles GET /me/quota - returns current user's quota usage
func (h *UserHandler) GetQuota(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [UserHandler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		h.logger.Error("❌ [UserHandler] Invalid user ID type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal error"})
		return
	}

	quota, err := h.userService.GetUserQuota(userIDUint)
	if err != nil {
		h.logger.Error("❌ [UserHandler] Failed to get user quota", "user_id", userIDUint, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get quota"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":             quota.User.ID,
			"plan_tier":      quota.User.PlanTier,
			"billing_status": quota.User.BillingStatus,
		},
		"usage": gin.H{
			"standalone_groups": quota.Usage.StandaloneGroups,
			"organizations":     quota.Usage.Organizations,
			"storage_bytes":     quota.Usage.StorageBytes,
		},
		"limits": gin.H{
			"max_standalone_groups":   quota.Limits.MaxStandaloneGroups,
			"max_documents":           quota.Limits.MaxDocuments,
			"max_storage_bytes":       quota.Limits.MaxStorageBytes,
			"max_file_size":           quota.Limits.MaxFileSize,
			"daily_messages":          quota.Limits.DailyMessages,
			"max_organizations":       quota.Limits.MaxOrganizations,
			"can_create_organization": quota.Limits.CanCreateOrganization,
		},
	})
}

// CheckCanCreateOrganization handles GET /me/can-create-organization
func (h *UserHandler) CheckCanCreateOrganization(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [UserHandler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		h.logger.Error("❌ [UserHandler] Invalid user ID type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal error"})
		return
	}

	canCreate, err := h.userService.CanCreateOrganization(userIDUint)
	if err != nil {
		h.logger.Error("❌ [UserHandler] Failed to check organization creation permission", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permission"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"can_create_organization": canCreate,
	})
}

// CheckCanJoinOrganization handles GET /me/can-join-organization
func (h *UserHandler) CheckCanJoinOrganization(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [UserHandler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		h.logger.Error("❌ [UserHandler] Invalid user ID type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal error"})
		return
	}

	canJoin, err := h.userService.CanJoinOrganization(userIDUint)
	if err != nil {
		h.logger.Error("❌ [UserHandler] Failed to check organization join permission", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permission"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"can_join_organization": canJoin,
	})
}

// CheckCanCreateStandaloneGroup handles GET /me/can-create-group
func (h *UserHandler) CheckCanCreateStandaloneGroup(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [UserHandler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		h.logger.Error("❌ [UserHandler] Invalid user ID type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal error"})
		return
	}

	canCreate, err := h.userService.CanCreateStandaloneGroup(userIDUint)
	if err != nil {
		h.logger.Error("❌ [UserHandler] Failed to check group creation permission", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permission"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"can_create_standalone_group": canCreate,
	})
}
