package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
)

// AdminHandler handles admin API requests for organization management
type AdminHandler struct {
	orgService   service.OrganizationService
	groupService service.GroupService
	userService  service.UserService
	logger       *slog.Logger
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(orgService service.OrganizationService, groupService service.GroupService, userService service.UserService, logger *slog.Logger) *AdminHandler {
	return &AdminHandler{
		orgService:   orgService,
		groupService: groupService,
		userService:  userService,
		logger:       logger,
	}
}

// UpdateTierRequest represents the request body for updating organization tier
type UpdateTierRequest struct {
	Tier string `json:"tier" binding:"required"`
}

// UpdateTier handles PUT /admin/organizations/:id/tier
func (h *AdminHandler) UpdateTier(c *gin.Context) {
	// Parse organization ID
	orgIDStr := c.Param("id")
	orgID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid organization ID", "org_id", orgIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	// Parse request body
	var req UpdateTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate tier value
	tier := config.PlanTier(req.Tier)
	if !isValidTier(tier) {
		h.logger.Error("❌ [AdminHandler] Invalid tier value", "tier", req.Tier)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "Invalid tier value",
			"valid_tiers": []string{string(config.PlanFree), string(config.PlanPro), string(config.PlanEnterprise)},
		})
		return
	}

	h.logger.Info("📊 [AdminHandler] Updating organization tier",
		"org_id", orgID,
		"new_tier", tier,
	)

	// Update the tier
	if err := h.orgService.UpdatePlanTier(uint(orgID), tier); err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to update tier", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update tier"})
		return
	}

	// Get updated organization info
	org, err := h.orgService.GetOrganization(uint(orgID))
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to get organization", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tier updated but failed to retrieve organization"})
		return
	}

	limits := org.GetOrganizationPlanLimits()

	h.logger.Info("✅ [AdminHandler] Organization tier updated",
		"org_id", orgID,
		"tier", tier,
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "Organization tier updated successfully",
		"organization": gin.H{
			"id":                 org.ID,
			"name":               org.Name,
			"plan_tier":          org.PlanTier,
			"billing_status":     org.BillingStatus,
			"used_storage_bytes": org.UsedStorageBytes,
		},
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

// UpdateBillingStatusRequest represents the request body for updating billing status
type UpdateBillingStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// UpdateBillingStatus handles PUT /admin/organizations/:id/billing
func (h *AdminHandler) UpdateBillingStatus(c *gin.Context) {
	// Parse organization ID
	orgIDStr := c.Param("id")
	orgID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid organization ID", "org_id", orgIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	// Parse request body
	var req UpdateBillingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate billing status value
	status := config.BillingStatus(req.Status)
	if !isValidBillingStatus(status) {
		h.logger.Error("❌ [AdminHandler] Invalid billing status value", "status", req.Status)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          "Invalid billing status value",
			"valid_statuses": []string{string(config.BillingActive), string(config.BillingPastDue), string(config.BillingSuspended), string(config.BillingCanceled)},
		})
		return
	}

	h.logger.Info("📊 [AdminHandler] Updating organization billing status",
		"org_id", orgID,
		"new_status", status,
	)

	// Update the billing status
	if err := h.orgService.UpdateBillingStatus(uint(orgID), status); err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to update billing status", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update billing status"})
		return
	}

	h.logger.Info("✅ [AdminHandler] Organization billing status updated",
		"org_id", orgID,
		"status", status,
	)

	c.JSON(http.StatusOK, gin.H{
		"message":        "Organization billing status updated successfully",
		"org_id":         orgID,
		"billing_status": status,
	})
}

// GetOrganizationQuota handles GET /admin/organizations/:id/quota
func (h *AdminHandler) GetOrganizationQuota(c *gin.Context) {
	// Parse organization ID
	orgIDStr := c.Param("id")
	orgID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid organization ID", "org_id", orgIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	// Get organization
	org, err := h.orgService.GetOrganization(uint(orgID))
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to get organization", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get organization"})
		return
	}

	limits := org.GetOrganizationPlanLimits()

	// Get member count
	memberCount, err := h.orgService.CountMembers(uint(orgID))
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to count members", "error", err)
		memberCount = 0
	}

	// Get group count
	groupCount, err := h.orgService.CountGroups(uint(orgID))
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to count groups", "error", err)
		groupCount = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"organization": gin.H{
			"id":             org.ID,
			"name":           org.Name,
			"plan_tier":      org.PlanTier,
			"billing_status": org.BillingStatus,
		},
		"usage": gin.H{
			"members":       memberCount,
			"groups":        groupCount,
			"storage_bytes": org.UsedStorageBytes,
		},
		"limits": gin.H{
			"max_members":             limits.MaxMembers,
			"max_groups":              limits.MaxGroups,
			"max_storage_bytes":       limits.MaxStorageBytes,
			"max_file_size":           limits.MaxFileSize,
			"daily_messages_per_user": limits.DailyMessagesPerUser,
		},
		"utilization": gin.H{
			"members_percent": calculatePercent(memberCount, int64(limits.MaxMembers)),
			"groups_percent":  calculatePercent(groupCount, int64(limits.MaxGroups)),
			"storage_percent": calculatePercent(org.UsedStorageBytes, limits.MaxStorageBytes),
		},
	})
}

func isValidTier(tier config.PlanTier) bool {
	return tier == config.PlanFree || tier == config.PlanPro || tier == config.PlanEnterprise
}

func isValidBillingStatus(status config.BillingStatus) bool {
	return status == config.BillingActive || status == config.BillingPastDue ||
		status == config.BillingSuspended || status == config.BillingCanceled
}

func calculatePercent(used, limit int64) float64 {
	if limit <= 0 {
		return 0
	}
	return float64(used) / float64(limit) * 100
}

// ==================== Organization CRUD ====================

// CreateOrganizationRequest represents the request for creating an organization
type CreateOrganizationRequest struct {
	Name   string  `json:"name" binding:"required"`
	Domain *string `json:"domain,omitempty"`
}

// CreateOrganization handles POST /organizations
func (h *AdminHandler) CreateOrganization(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [AdminHandler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userIDUint := userID.(uint)

	var req CreateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	h.logger.Info("🏢 [AdminHandler] Creating organization",
		"name", req.Name,
		"owner_id", userIDUint,
	)

	org, err := h.orgService.CreateOrganization(req.Name, req.Domain, userIDUint)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to create organization", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organization"})
		return
	}

	h.logger.Info("✅ [AdminHandler] Organization created", "org_id", org.ID, "name", org.Name)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Organization created successfully",
		"organization": gin.H{
			"id":             org.ID,
			"name":           org.Name,
			"domain":         org.Domain,
			"plan_tier":      org.PlanTier,
			"billing_status": org.BillingStatus,
			"created_at":     org.CreatedAt,
		},
	})
}

// GetOrganization handles GET /organizations/:id
func (h *AdminHandler) GetOrganization(c *gin.Context) {
	orgIDStr := c.Param("id")
	orgID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid organization ID", "org_id", orgIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	org, err := h.orgService.GetOrganization(uint(orgID))
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to get organization", "org_id", orgID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	limits := org.GetOrganizationPlanLimits()

	c.JSON(http.StatusOK, gin.H{
		"organization": gin.H{
			"id":                 org.ID,
			"name":               org.Name,
			"domain":             org.Domain,
			"plan_tier":          org.PlanTier,
			"billing_status":     org.BillingStatus,
			"used_storage_bytes": org.UsedStorageBytes,
			"created_at":         org.CreatedAt,
			"updated_at":         org.UpdatedAt,
		},
		"limits": limits,
	})
}

// UpdateOrganizationRequest represents the request for updating an organization
type UpdateOrganizationRequest struct {
	Name   string  `json:"name" binding:"required"`
	Domain *string `json:"domain,omitempty"`
}

// UpdateOrganization handles PUT /organizations/:id
func (h *AdminHandler) UpdateOrganization(c *gin.Context) {
	orgIDStr := c.Param("id")
	orgID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid organization ID", "org_id", orgIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	var req UpdateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	h.logger.Info("🏢 [AdminHandler] Updating organization", "org_id", orgID, "name", req.Name)

	org, err := h.orgService.UpdateOrganization(uint(orgID), req.Name, req.Domain)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to update organization", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update organization"})
		return
	}

	h.logger.Info("✅ [AdminHandler] Organization updated", "org_id", org.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Organization updated successfully",
		"organization": gin.H{
			"id":             org.ID,
			"name":           org.Name,
			"domain":         org.Domain,
			"plan_tier":      org.PlanTier,
			"billing_status": org.BillingStatus,
			"updated_at":     org.UpdatedAt,
		},
	})
}

// DeleteOrganization handles DELETE /organizations/:id
func (h *AdminHandler) DeleteOrganization(c *gin.Context) {
	orgIDStr := c.Param("id")
	orgID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid organization ID", "org_id", orgIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	h.logger.Info("🗑️ [AdminHandler] Deleting organization", "org_id", orgID)

	if err := h.orgService.DeleteOrganization(uint(orgID)); err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to delete organization", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete organization"})
		return
	}

	h.logger.Info("✅ [AdminHandler] Organization deleted", "org_id", orgID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Organization deleted successfully",
	})
}

// ==================== Member Management ====================

// OrgAddMemberRequest represents the request for adding a member to an organization
type OrgAddMemberRequest struct {
	UserID uint   `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"`
}

// AddMember handles POST /organizations/:id/members
func (h *AdminHandler) AddMember(c *gin.Context) {
	orgIDStr := c.Param("id")
	orgID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid organization ID", "org_id", orgIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	var req OrgAddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	h.logger.Info("👥 [AdminHandler] Adding member to organization",
		"org_id", orgID,
		"user_id", req.UserID,
		"role", req.Role,
	)

	member, err := h.orgService.AddMember(uint(orgID), req.UserID, req.Role)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to add member", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("✅ [AdminHandler] Member added", "org_id", orgID, "user_id", req.UserID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Member added successfully",
		"member": gin.H{
			"user_id":         member.UserID,
			"organization_id": member.OrganizationID,
			"role":            member.Role,
			"joined_at":       member.JoinedAt,
		},
	})
}

// RemoveMember handles DELETE /organizations/:id/members/:user_id
func (h *AdminHandler) RemoveMember(c *gin.Context) {
	orgIDStr := c.Param("id")
	orgID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid organization ID", "org_id", orgIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid user ID", "user_id", userIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	h.logger.Info("👥 [AdminHandler] Removing member from organization",
		"org_id", orgID,
		"user_id", userID,
	)

	if err := h.orgService.RemoveMember(uint(orgID), uint(userID)); err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to remove member", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("✅ [AdminHandler] Member removed", "org_id", orgID, "user_id", userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Member removed successfully",
	})
}

// OrgUpdateMemberRoleRequest represents the request for updating a member's role
type OrgUpdateMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// UpdateMemberRole handles PUT /organizations/:id/members/:user_id
func (h *AdminHandler) UpdateMemberRole(c *gin.Context) {
	orgIDStr := c.Param("id")
	orgID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid organization ID", "org_id", orgIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid user ID", "user_id", userIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req OrgUpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	h.logger.Info("👥 [AdminHandler] Updating member role",
		"org_id", orgID,
		"user_id", userID,
		"new_role", req.Role,
	)

	if err := h.orgService.UpdateMemberRole(uint(orgID), uint(userID), req.Role); err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to update member role", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("✅ [AdminHandler] Member role updated", "org_id", orgID, "user_id", userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Member role updated successfully",
	})
}

// ListMembers handles GET /organizations/:id/members
func (h *AdminHandler) ListMembers(c *gin.Context) {
	orgIDStr := c.Param("id")
	orgID, err := strconv.ParseUint(orgIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid organization ID", "org_id", orgIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	// Parse pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	members, total, err := h.orgService.ListMembers(uint(orgID), page, pageSize)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to list members", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list members"})
		return
	}

	// Transform members to response format
	membersResponse := make([]gin.H, len(members))
	for i, member := range members {
		membersResponse[i] = gin.H{
			"user_id":         member.UserID,
			"organization_id": member.OrganizationID,
			"role_id":         member.RoleID,
			"role_name":       member.Role.Name,
			"joined_at":       member.JoinedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"members": membersResponse,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// ListMyOrganizations handles GET /organizations/my/list
func (h *AdminHandler) ListMyOrganizations(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [AdminHandler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userIDUint := userID.(uint)

	h.logger.Info("🏢 [AdminHandler] Listing user's organizations", "user_id", userIDUint)

	// Get organizations from service
	memberships, err := h.orgService.GetUserOrganizations(userIDUint)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to get user organizations", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve organizations"})
		return
	}

	// Build full organization details with groups for each membership
	orgs := make([]gin.H, 0, len(memberships))
	for _, membership := range memberships {
		// Get full organization details
		org, err := h.orgService.GetOrganization(membership.OrganizationID)
		if err != nil {
			h.logger.Warn("⚠️ [AdminHandler] Failed to get organization details",
				"org_id", membership.OrganizationID, "error", err)
			continue
		}

		// Get groups in this organization
		groups, _, err := h.groupService.ListGroupsByOrganization(membership.OrganizationID, 1, 100)
		if err != nil {
			h.logger.Warn("⚠️ [AdminHandler] Failed to get organization groups",
				"org_id", membership.OrganizationID, "error", err)
			groups = []models.Group{} // Continue with empty groups
		}

		// Transform groups to response format
		groupsResponse := make([]gin.H, len(groups))
		for j, group := range groups {
			groupsResponse[j] = gin.H{
				"id":          group.ID,
				"name":        group.Name,
				"description": group.Description,
				"created_at":  group.CreatedAt,
			}
		}

		limits := org.GetOrganizationPlanLimits()

		orgResponse := gin.H{
			"id":             org.ID,
			"name":           org.Name,
			"domain":         org.Domain,
			"plan_tier":      org.PlanTier,
			"billing_status": org.BillingStatus,
			"created_at":     org.CreatedAt,
			"membership": gin.H{
				"role_id":   membership.RoleID,
				"role_name": membership.Role.Name,
				"joined_at": membership.JoinedAt,
			},
			"groups": groupsResponse,
			"limits": gin.H{
				"max_members":             limits.MaxMembers,
				"max_groups":              limits.MaxGroups,
				"max_documents":           limits.MaxDocuments,
				"max_storage_bytes":       limits.MaxStorageBytes,
				"max_file_size":           limits.MaxFileSize,
				"daily_messages_per_user": limits.DailyMessagesPerUser,
			},
		}
		orgs = append(orgs, orgResponse)
	}

	c.JSON(http.StatusOK, gin.H{
		"organizations": orgs,
		"total":         len(orgs),
	})
}

// ==================== Admin User Management ====================

// UpdateUserTierRequest represents the request body for updating a user's tier
type UpdateUserTierRequest struct {
	Tier string `json:"tier" binding:"required"`
}

// UpdateUserTier handles PUT /admin/users/:user_id/tier
func (h *AdminHandler) UpdateUserTier(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid user ID", "user_id", userIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req UpdateUserTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tier := config.PlanTier(req.Tier)
	if !config.IsValidTier(tier) {
		h.logger.Error("❌ [AdminHandler] Invalid tier value", "tier", req.Tier)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "Invalid tier value",
			"valid_tiers": []string{string(config.PlanFree), string(config.PlanPro), string(config.PlanEnterprise)},
		})
		return
	}

	h.logger.Info("📊 [AdminHandler] Admin updating user tier",
		"target_user_id", userID,
		"new_tier", tier,
	)

	user, err := h.userService.UpdateUserPlanTier(uint(userID), tier)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to update user tier", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update tier"})
		return
	}

	limits := user.GetPlanLimits()

	h.logger.Info("✅ [AdminHandler] User tier updated",
		"user_id", userID,
		"tier", tier,
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "User tier updated successfully",
		"user": gin.H{
			"id":             user.ID,
			"username":       user.Username,
			"email":          user.Email,
			"plan_tier":      user.PlanTier,
			"billing_status": user.BillingStatus,
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

// UpdateUserBillingStatus handles PUT /admin/users/:user_id/billing
func (h *AdminHandler) UpdateUserBillingStatus(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid user ID", "user_id", userIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req UpdateBillingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	status := config.BillingStatus(req.Status)
	validStatuses := []config.BillingStatus{
		config.BillingActive,
		config.BillingPastDue,
		config.BillingCanceled,
		config.BillingTrialing,
		config.BillingSuspended,
	}

	isValid := false
	for _, s := range validStatuses {
		if status == s {
			isValid = true
			break
		}
	}

	if !isValid {
		h.logger.Error("❌ [AdminHandler] Invalid billing status", "status", req.Status)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          "Invalid billing status",
			"valid_statuses": validStatuses,
		})
		return
	}

	h.logger.Info("💳 [AdminHandler] Admin updating user billing status",
		"target_user_id", userID,
		"new_status", status,
	)

	if err := h.userService.UpdateUserBillingStatus(uint(userID), status); err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to update user billing status", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update billing status"})
		return
	}

	h.logger.Info("✅ [AdminHandler] User billing status updated",
		"user_id", userID,
		"status", status,
	)

	c.JSON(http.StatusOK, gin.H{
		"message":        "User billing status updated successfully",
		"billing_status": status,
	})
}

// GetUserQuota handles GET /admin/users/:user_id/quota
func (h *AdminHandler) GetUserQuota(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Invalid user ID", "user_id", userIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	quota, err := h.userService.GetUserQuota(uint(userID))
	if err != nil {
		h.logger.Error("❌ [AdminHandler] Failed to get user quota", "user_id", userID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":             quota.User.ID,
			"username":       quota.User.Username,
			"email":          quota.User.Email,
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
