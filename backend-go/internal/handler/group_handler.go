package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
)

// GroupHandler handles HTTP requests for group operations
type GroupHandler struct {
	groupService service.GroupService
	logger       *slog.Logger
}

// NewGroupHandler creates a new group handler
func NewGroupHandler(groupService service.GroupService, logger *slog.Logger) *GroupHandler {
	return &GroupHandler{
		groupService: groupService,
		logger:       logger,
	}
}

// ==================== Request/Response DTOs ====================

type CreateGroupRequest struct {
	OrganizationID *uint  `json:"organization_id,omitempty"` // nullable for standalone groups
	Name           string `json:"name" binding:"required,min=1,max=255"`
	Description    string `json:"description"`
}

type UpdateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CreateRoleRequest struct {
	Name        string   `json:"name" binding:"required,min=1,max=100"`
	Permissions []string `json:"permissions" binding:"required"`
}

type UpdateRoleRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type AddMemberRequest struct {
	UserID uint `json:"user_id" binding:"required"`
	RoleID uint `json:"role_id" binding:"required"`
}

type UpdateMemberRoleRequest struct {
	RoleID uint `json:"role_id" binding:"required"`
}

type UpdateGroupTierRequest struct {
	PlanTier      string `json:"plan_tier" binding:"required"`
	BillingStatus string `json:"billing_status,omitempty"`
}

type UpdateGroupBillingStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type GroupResponse struct {
	ID             uint   `json:"id"`
	OrganizationID *uint  `json:"organization_id,omitempty"` // nullable for standalone groups
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	// Billing fields (only populated for standalone groups)
	PlanTier         string `json:"plan_tier,omitempty"`
	BillingStatus    string `json:"billing_status,omitempty"`
	UsedStorageBytes int64  `json:"used_storage_bytes,omitempty"`
}

type RoleResponse struct {
	ID          uint     `json:"id"`
	GroupID     uint     `json:"group_id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type MemberResponse struct {
	UserID   uint         `json:"user_id"`
	GroupID  uint         `json:"group_id"`
	RoleID   uint         `json:"role_id"`
	Role     RoleResponse `json:"role,omitempty"`
	JoinedAt string       `json:"joined_at"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// ==================== Group Handlers ====================

// CreateGroup handles POST /groups
// If organization_id is provided, creates a group within that organization.
// If organization_id is omitted/null, creates a standalone group with a FREE plan tier.
func (h *GroupHandler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ [GroupHandler] Invalid create group request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var group *models.Group
	var err error

	if req.OrganizationID != nil {
		// Create group within an organization
		group, err = h.groupService.CreateGroup(*req.OrganizationID, req.Name, req.Description, userID)
	} else {
		// Create standalone group with default FREE tier
		group, err = h.groupService.CreateStandaloneGroup(req.Name, req.Description, userID)
	}

	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, h.mapGroupToResponse(group))
}

// GetGroup handles GET /groups/:group_id
func (h *GroupHandler) GetGroup(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	group, err := h.groupService.GetGroup(groupID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, h.mapGroupToResponse(group))
}

// UpdateGroup handles PUT /groups/:group_id
func (h *GroupHandler) UpdateGroup(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	group, err := h.groupService.UpdateGroup(userID, groupID, req.Name, req.Description)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, h.mapGroupToResponse(group))
}

// DeleteGroup handles DELETE /groups/:group_id
func (h *GroupHandler) DeleteGroup(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := h.groupService.DeleteGroup(userID, groupID); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Group deleted successfully"})
}

// UpdateGroupTier handles PUT /groups/:group_id/tier
// Updates the plan tier and billing status for standalone groups only.
// Returns 400 Bad Request if the group belongs to an organization.
func (h *GroupHandler) UpdateGroupTier(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	var req UpdateGroupTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ [GroupHandler] Invalid update group tier request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse plan tier
	planTier := config.PlanTier(req.PlanTier)

	// Default billing status to active if not provided
	billingStatus := config.BillingActive
	if req.BillingStatus != "" {
		billingStatus = config.BillingStatus(req.BillingStatus)
	}

	group, err := h.groupService.UpdateStandaloneGroupTier(userID, groupID, planTier, billingStatus)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, h.mapGroupToResponse(group))
}

// UpdateGroupBillingStatus handles PUT /admin/groups/:group_id/billing
// Updates the billing status for standalone groups only.
// Returns 400 Bad Request if the group belongs to an organization.
func (h *GroupHandler) UpdateGroupBillingStatus(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	var req UpdateGroupBillingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ [GroupHandler] Invalid update group billing status request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Validate billing status value
	status := config.BillingStatus(req.Status)
	if !isValidBillingStatus(status) {
		h.logger.Error("❌ [GroupHandler] Invalid billing status value", "status", req.Status)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          "Invalid billing status value",
			"valid_statuses": []string{string(config.BillingActive), string(config.BillingPastDue), string(config.BillingSuspended), string(config.BillingCanceled), string(config.BillingTrialing)},
		})
		return
	}

	h.logger.Info("📊 [GroupHandler] Updating standalone group billing status",
		"group_id", groupID,
		"new_status", status,
	)

	if err := h.groupService.UpdateStandaloneGroupBillingStatus(groupID, status); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Group billing status updated successfully",
		"group_id":       groupID,
		"billing_status": status,
	})
}

// GetGroupQuota handles GET /admin/groups/:group_id/quota
// Returns quota information for standalone groups only.
// Returns 400 Bad Request if the group belongs to an organization.
func (h *GroupHandler) GetGroupQuota(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	quota, err := h.groupService.GetStandaloneGroupQuota(groupID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"group": gin.H{
			"id":             quota.Group.ID,
			"name":           quota.Group.Name,
			"plan_tier":      quota.Group.PlanTier,
			"billing_status": quota.Group.BillingStatus,
		},
		"usage": gin.H{
			"members":       quota.Usage.Members,
			"storage_bytes": quota.Usage.StorageBytes,
		},
		"limits": gin.H{
			"max_members":             quota.Limits.MaxMembers,
			"max_storage_bytes":       quota.Limits.MaxStorageBytes,
			"max_file_size":           quota.Limits.MaxFileSize,
			"daily_messages_per_user": quota.Limits.DailyMessagesPerUser,
		},
	})
}

// ListGroupsByOrganization handles GET /organizations/:org_id/groups
func (h *GroupHandler) ListGroupsByOrganization(c *gin.Context) {
	orgID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
		return
	}

	page, pageSize := h.getPagination(c)

	groups, total, err := h.groupService.ListGroupsByOrganization(uint(orgID), page, pageSize)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	response := make([]GroupResponse, len(groups))
	for i, g := range groups {
		response[i] = h.mapGroupToResponse(&g)
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       response,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	})
}

// ==================== Role Handlers ====================

// CreateRole handles POST /groups/:group_id/roles
func (h *GroupHandler) CreateRole(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	var req CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	role, err := h.groupService.CreateRole(userID, groupID, req.Name, req.Permissions)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, h.mapRoleToResponse(role))
}

// UpdateRole handles PUT /groups/:group_id/roles/:role_id
func (h *GroupHandler) UpdateRole(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	roleID, err := h.parseRoleID(c)
	if err != nil {
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	role, err := h.groupService.UpdateRole(userID, groupID, roleID, req.Name, req.Permissions)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, h.mapRoleToResponse(role))
}

// DeleteRole handles DELETE /groups/:group_id/roles/:role_id
func (h *GroupHandler) DeleteRole(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	roleID, err := h.parseRoleID(c)
	if err != nil {
		return
	}

	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := h.groupService.DeleteRole(userID, groupID, roleID); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role deleted successfully"})
}

// ListRoles handles GET /groups/:group_id/roles
func (h *GroupHandler) ListRoles(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	roles, err := h.groupService.ListRoles(groupID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	response := make([]RoleResponse, len(roles))
	for i, r := range roles {
		response[i] = h.mapRoleToResponse(&r)
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

// ==================== Member Handlers ====================

// AddMember handles POST /groups/:group_id/members
func (h *GroupHandler) AddMember(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID := h.getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	member, err := h.groupService.AddMember(userID, groupID, req.UserID, req.RoleID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, h.mapMemberToResponse(member))
}

// UpdateMemberRole handles PUT /groups/:group_id/members/:user_id
func (h *GroupHandler) UpdateMemberRole(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	targetUserID, err := h.parseUserID(c)
	if err != nil {
		return
	}

	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	requesterID := h.getUserIDFromContext(c)
	if requesterID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := h.groupService.UpdateMemberRole(requesterID, groupID, targetUserID, req.RoleID); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member role updated successfully"})
}

// RemoveMember handles DELETE /groups/:group_id/members/:user_id
func (h *GroupHandler) RemoveMember(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	targetUserID, err := h.parseUserID(c)
	if err != nil {
		return
	}

	requesterID := h.getUserIDFromContext(c)
	if requesterID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := h.groupService.RemoveMember(requesterID, groupID, targetUserID); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member removed successfully"})
}

// ListMembers handles GET /groups/:group_id/members
func (h *GroupHandler) ListMembers(c *gin.Context) {
	groupID, err := h.parseGroupID(c)
	if err != nil {
		return
	}

	page, pageSize := h.getPagination(c)

	members, total, err := h.groupService.ListMembers(groupID, page, pageSize)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	response := make([]MemberResponse, len(members))
	for i, m := range members {
		response[i] = h.mapMemberToResponse(&m)
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       response,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize)),
	})
}

// ==================== Helper Methods ====================

func (h *GroupHandler) getUserIDFromContext(c *gin.Context) uint {
	userID, exists := c.Get("userID")
	if !exists {
		return 0
	}
	if id, ok := userID.(uint); ok {
		return id
	}
	return 0
}

func (h *GroupHandler) parseGroupID(c *gin.Context) (uint, error) {
	groupID, err := strconv.ParseUint(c.Param("group_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return 0, err
	}
	return uint(groupID), nil
}

func (h *GroupHandler) parseRoleID(c *gin.Context) (uint, error) {
	roleID, err := strconv.ParseUint(c.Param("role_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return 0, err
	}
	return uint(roleID), nil
}

func (h *GroupHandler) parseUserID(c *gin.Context) (uint, error) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return 0, err
	}
	return uint(userID), nil
}

func (h *GroupHandler) getPagination(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20

	if p, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(c.DefaultQuery("page_size", "20")); err == nil && ps > 0 && ps <= 100 {
		pageSize = ps
	}

	return page, pageSize
}

func (h *GroupHandler) mapGroupToResponse(g *models.Group) GroupResponse {
	resp := GroupResponse{
		ID:             g.ID,
		OrganizationID: g.OrganizationID,
		Name:           g.Name,
	}
	if g.Description != nil {
		resp.Description = *g.Description
	}
	// Include billing fields for standalone groups
	if g.IsStandalone() {
		resp.PlanTier = string(g.PlanTier)
		resp.BillingStatus = string(g.BillingStatus)
		resp.UsedStorageBytes = g.UsedStorageBytes
	}
	return resp
}

func (h *GroupHandler) mapRoleToResponse(r *models.GroupRole) RoleResponse {
	return RoleResponse{
		ID:          r.ID,
		GroupID:     r.GroupID,
		Name:        r.Name,
		Permissions: []string(r.Permissions),
	}
}

func (h *GroupHandler) mapMemberToResponse(m *models.GroupMember) MemberResponse {
	resp := MemberResponse{
		UserID:   m.UserID,
		GroupID:  m.GroupID,
		RoleID:   m.RoleID,
		JoinedAt: m.JoinedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if m.Role.ID != 0 {
		resp.Role = RoleResponse{
			ID:          m.Role.ID,
			GroupID:     m.Role.GroupID,
			Name:        m.Role.Name,
			Permissions: []string(m.Role.Permissions),
		}
	}
	return resp
}

// ListMyGroups handles GET /api/v1/me/groups - lists all groups the authenticated user is a member of
func (h *GroupHandler) ListMyGroups(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [GroupHandler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userIDUint := userID.(uint)

	h.logger.Info("📋 [GroupHandler] Listing user's groups", "user_id", userIDUint)

	memberships, err := h.groupService.GetUserGroups(userIDUint)
	if err != nil {
		h.logger.Error("❌ [GroupHandler] Failed to get user groups", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve groups"})
		return
	}

	// Transform memberships to response format with full group details
	groups := make([]gin.H, len(memberships))
	for i, membership := range memberships {
		// Get full group details
		group, err := h.groupService.GetGroup(membership.GroupID)
		if err != nil {
			h.logger.Warn("⚠️ [GroupHandler] Failed to get group details",
				"group_id", membership.GroupID, "error", err)
			continue
		}

		groups[i] = gin.H{
			"id":              group.ID,
			"organization_id": group.OrganizationID,
			"name":            group.Name,
			"description":     group.Description,
			"created_at":      group.CreatedAt,
			"membership": gin.H{
				"role_id":   membership.RoleID,
				"role_name": membership.Role.Name,
				"joined_at": membership.JoinedAt,
			},
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"total":  len(groups),
	})
}

func (h *GroupHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrPermissionDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
	case errors.Is(err, service.ErrSystemRoleProtected):
		c.JSON(http.StatusForbidden, gin.H{"error": "System roles (Owner, Admin) cannot be modified or deleted"})
	case errors.Is(err, service.ErrRoleInUse):
		c.JSON(http.StatusConflict, gin.H{"error": "Role is assigned to members and cannot be deleted"})
	case errors.Is(err, service.ErrRoleNameExists):
		c.JSON(http.StatusConflict, gin.H{"error": "Role with this name already exists"})
	case errors.Is(err, service.ErrMemberAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": "User is already a member of this group"})
	case errors.Is(err, service.ErrCannotEscalatePermissions):
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot assign permissions higher than your own"})
	case errors.Is(err, service.ErrCannotModifyOwner):
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot modify the owner's role"})
	case errors.Is(err, service.ErrCannotRemoveOwner):
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot remove the group owner"})
	case errors.Is(err, service.ErrOrganizationGroupBilling):
		c.JSON(http.StatusBadRequest, gin.H{"error": "This group is managed by an organization. Billing must be handled at the organization level."})
	case errors.Is(err, service.ErrInvalidPlanTier):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plan tier. Valid values are: FREE, PRO, ENTERPRISE"})
	case errors.Is(err, repository.ErrGroupNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
	case errors.Is(err, repository.ErrRoleNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
	case errors.Is(err, repository.ErrGroupMemberNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found in group"})
	case errors.Is(err, repository.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
	default:
		h.logger.Error("❌ [GroupHandler] Internal error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	}
}
