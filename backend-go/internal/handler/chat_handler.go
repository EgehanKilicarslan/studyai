package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/grpc"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/middleware"
	pb "github.com/EgehanKilicarslan/studyai/backend-go/pb"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"
)

// ChatHandler handles HTTP requests and forwards them to the RAG service
type ChatHandler struct {
	services    *grpc.Client
	cfg         *config.Config
	logger      *slog.Logger
	rateLimiter middleware.RateLimiter
	orgRepo     repository.OrganizationRepository
	groupRepo   repository.GroupRepository
}

// NewChatHandler injects dependencies (Dependency Injection Go Style)
func NewChatHandler(
	services *grpc.Client,
	cfg *config.Config,
	logger *slog.Logger,
	rateLimiter middleware.RateLimiter,
	orgRepo repository.OrganizationRepository,
	groupRepo repository.GroupRepository,
) *ChatHandler {
	return &ChatHandler{
		services:    services,
		cfg:         cfg,
		logger:      logger,
		rateLimiter: rateLimiter,
		orgRepo:     orgRepo,
		groupRepo:   groupRepo,
	}
}

// ChatHandler: POST /api/v1/chat
func (h *ChatHandler) ChatHandler(c *gin.Context) {
	// Extract authenticated user ID
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [Handler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		h.logger.Error("❌ [Handler] Invalid user ID type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal error"})
		return
	}

	// Extract organization ID and group IDs (optional - check headers, query params, or context)
	var orgIDUint uint
	var groupIDs []uint
	var limits config.PlanLimits

	// Try to get orgID from headers first
	if orgIDStr := c.GetHeader("x-organization-id"); orgIDStr != "" {
		if id, err := strconv.ParseUint(orgIDStr, 10, 64); err == nil {
			orgIDUint = uint(id)
		}
	}

	// If not in headers, try context (set by middleware)
	if orgIDUint == 0 {
		orgID, orgExists := c.Get("orgID")
		if orgExists {
			if id, ok := orgID.(uint); ok {
				orgIDUint = id
			}
		}
	}

	// If not in context, try query parameter
	if orgIDUint == 0 {
		if orgIDStr := c.Query("organization_id"); orgIDStr != "" {
			if id, err := strconv.ParseUint(orgIDStr, 10, 64); err == nil {
				orgIDUint = uint(id)
			}
		}
	}

	// Parse group IDs from headers (comma-separated or multiple headers)
	if groupIDsStr := c.GetHeader("x-group-ids"); groupIDsStr != "" {
		for _, gidStr := range strings.Split(groupIDsStr, ",") {
			gidStr = strings.TrimSpace(gidStr)
			if gid, err := strconv.ParseUint(gidStr, 10, 32); err == nil {
				groupIDs = append(groupIDs, uint(gid))
			}
		}
	}

	// Check rate limit if we have an org
	if orgIDUint > 0 {
		org, err := h.orgRepo.FindByID(orgIDUint)
		if err != nil {
			h.logger.Warn("⚠️ [Handler] Failed to get organization for rate limiting", "error", err)
			// Continue without rate limiting
			limits = config.DefaultPlanLimits[config.PlanFree]
		} else {
			limits = org.GetPlanLimits()

			allowed, used, limit, err := h.rateLimiter.CheckDailyLimit(c.Request.Context(), userIDUint, orgIDUint, limits)
			if err != nil {
				h.logger.Warn("⚠️ [Handler] Rate limit check failed, allowing request", "error", err)
			} else if !allowed {
				h.logger.Warn("⚠️ [Handler] Rate limit exceeded",
					"user_id", userIDUint,
					"org_id", orgIDUint,
					"used", used,
					"limit", limit,
				)
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":          "Daily message limit exceeded",
					"messages_used":  used,
					"messages_limit": limit,
					"reset_at":       getNextMidnightUTC(),
				})
				return
			}
		}
	} else {
		// No org context - use free plan limits but don't enforce rate limiting
		limits = config.DefaultPlanLimits[config.PlanFree]
		h.logger.Debug("📊 [Handler] No organization context, skipping rate limiting")
	}

	// Convert userID to string for gRPC metadata
	userIDStr := fmt.Sprintf("%v", userID)

	h.logger.Info("📨 [Handler] Chat request received",
		"user_id", userID,
	)

	var reqBody struct {
		Query     string `json:"query"`
		SessionID string `json:"session_id"`
	}

	if err := c.BindJSON(&reqBody); err != nil {
		h.logger.Error("❌ [Handler] Invalid JSON in chat request",
			"error", err,
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if reqBody.Query == "" {
		h.logger.Error("❌ [Handler] Missing query in chat request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query is required"})
		return
	}

	h.logger.Info("🔍 [Handler] Processing chat query",
		"query", reqBody.Query,
		"session_id", reqBody.SessionID,
		"user_id", userID,
	)

	// 1. gRPC Request
	grpcReq := &pb.ChatRequest{
		Query:     reqBody.Query,
		SessionId: reqBody.SessionID,
	}

	// Create context with timeout and user ID metadata
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.cfg.ChatTimeout)*time.Second)
	defer cancel()

	// Add user ID to gRPC metadata headers
	ctx = metadata.AppendToOutgoingContext(ctx, "x-user-id", userIDStr)

	// Add tenant-scoping metadata for RAG filtering
	if orgIDUint > 0 {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-organization-id", fmt.Sprintf("%d", orgIDUint))

		// If group IDs weren't provided in headers, get user's groups from database
		if len(groupIDs) == 0 {
			// Get user's groups within this organization for access control
			dbGroupIDs, err := h.groupRepo.GetUserGroupsInOrganization(userIDUint, orgIDUint)
			if err != nil {
				h.logger.Warn("⚠️ [Handler] Failed to get user groups", "error", err)
			} else if len(dbGroupIDs) > 0 {
				groupIDs = dbGroupIDs
			}
		}

		// Add group IDs to metadata if present
		if len(groupIDs) > 0 {
			// Convert group IDs to comma-separated string
			groupIDStrs := make([]string, len(groupIDs))
			for i, gid := range groupIDs {
				groupIDStrs[i] = fmt.Sprintf("%d", gid)
			}
			ctx = metadata.AppendToOutgoingContext(ctx, "x-group-ids", strings.Join(groupIDStrs, ","))
		}
	}

	stream, err := h.services.ChatService.Chat(ctx, grpcReq)
	if err != nil {
		h.logger.Error("❌ [Handler] Failed to call RAG service",
			"error", err,
			"query", reqBody.Query,
		)
		c.JSON(500, gin.H{"error": "Failed to call RAG service"})
		return
	}

	h.logger.Info("✅ [Handler] RAG service stream established")

	// Increment rate limit counter (do this early to prevent abuse)
	if err := h.rateLimiter.IncrementDailyCount(c.Request.Context(), userIDUint); err != nil {
		h.logger.Warn("⚠️ [Handler] Failed to increment rate limit counter", "error", err)
	}

	// 2. SSE Header Setup
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	// 3. Stream Responses
	messageCount := 0
	c.Stream(func(w io.Writer) bool {
		resp, err := stream.Recv()
		if err == io.EOF {
			h.logger.Info("✅ [Handler] Chat stream completed",
				"messages_sent", messageCount,
				"query", reqBody.Query,
			)
			return false // Stop streaming on EOF
		}
		if err != nil {
			h.logger.Error("❌ [Handler] Error receiving stream message",
				"error", err,
				"messages_sent", messageCount,
			)
			return false // Stop streaming on error
		}

		messageCount++
		c.SSEvent("message", gin.H{
			"answer":  resp.Answer,
			"sources": resp.SourceDocuments,
			"time":    resp.ProcessingTimeMs,
		})

		return true // Continue streaming
	})
}

// getNextMidnightUTC returns the time of the next midnight in UTC
func getNextMidnightUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}
