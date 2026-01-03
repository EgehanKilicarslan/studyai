package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/grpc"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/middleware"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/worker"
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
	redisClient database.ChatHistoryStore
	chatRepo    repository.ChatRepository
	workerPool  *worker.Pool
}

// NewChatHandler injects dependencies (Dependency Injection Go Style)
func NewChatHandler(
	services *grpc.Client,
	cfg *config.Config,
	logger *slog.Logger,
	rateLimiter middleware.RateLimiter,
	orgRepo repository.OrganizationRepository,
	groupRepo repository.GroupRepository,
	redisClient database.ChatHistoryStore,
	chatRepo repository.ChatRepository,
	workerPool *worker.Pool,
) *ChatHandler {
	return &ChatHandler{
		services:    services,
		cfg:         cfg,
		logger:      logger,
		rateLimiter: rateLimiter,
		orgRepo:     orgRepo,
		groupRepo:   groupRepo,
		redisClient: redisClient,
		chatRepo:    chatRepo,
		workerPool:  workerPool,
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
	var dailyMessageLimit int

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
			// Continue without rate limiting - use default free plan limit
			dailyMessageLimit = config.GetOrganizationPlanLimits(config.PlanFree).DailyMessagesPerUser
		} else {
			limits := org.GetOrganizationPlanLimits()
			dailyMessageLimit = limits.DailyMessagesPerUser

			allowed, used, limit, err := h.rateLimiter.CheckDailyLimit(c.Request.Context(), userIDUint, orgIDUint, dailyMessageLimit)
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
		dailyMessageLimit = config.GetOrganizationPlanLimits(config.PlanFree).DailyMessagesPerUser
		h.logger.Debug("📊 [Handler] No organization context, skipping rate limiting")
	}

	// Keep dailyMessageLimit for potential future use (e.g., returning remaining messages in response)
	_ = dailyMessageLimit

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

	// Parse or generate session ID
	var sessionID uuid.UUID
	var err error
	if reqBody.SessionID != "" {
		sessionID, err = uuid.Parse(reqBody.SessionID)
		if err != nil {
			h.logger.Error("❌ [Handler] Invalid session ID format",
				"session_id", reqBody.SessionID,
				"error", err,
			)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID format"})
			return
		}
	} else {
		// Generate new session ID if not provided
		sessionID = uuid.New()
		h.logger.Info("🆕 [Handler] Generated new session ID", "session_id", sessionID)
	}

	// Convert orgIDUint to pointer (nil if 0, meaning no organization context)
	var orgIDPtr *uint
	if orgIDUint > 0 {
		orgIDPtr = &orgIDUint
	}

	// Get or create session in database (organization is optional)
	_, _, err = h.chatRepo.GetOrCreateSession(sessionID, userIDUint, orgIDPtr)
	if err != nil {
		h.logger.Error("❌ [Handler] Failed to get/create session",
			"session_id", sessionID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize session"})
		return
	}

	// Fetch chat history (hybrid approach: Redis -> Postgres)
	chatHistory, err := h.getChatHistory(c.Request.Context(), sessionID)
	if err != nil {
		h.logger.Warn("⚠️ [Handler] Failed to fetch chat history, continuing without history",
			"session_id", sessionID,
			"error", err,
		)
		chatHistory = []models.ChatMessage{}
	}

	h.logger.Info("📚 [Handler] Loaded chat history",
		"session_id", sessionID,
		"message_count", len(chatHistory),
	)

	// 1. gRPC Request with chat history
	grpcReq := &pb.ChatRequest{
		Query:     reqBody.Query,
		SessionId: sessionID.String(),
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

	// Add chat history to metadata (as JSON array)
	if len(chatHistory) > 0 {
		historyJSON := formatChatHistory(chatHistory)
		ctx = metadata.AppendToOutgoingContext(ctx, "x-chat-history", historyJSON)
		h.logger.Debug("📤 [Handler] Added chat history to gRPC metadata",
			"session_id", sessionID,
			"history_length", len(chatHistory),
		)
	}

	// Save user message before calling gRPC
	userMessage := models.ChatMessage{
		SessionID: sessionID,
		Role:      "user",
		Content:   reqBody.Query,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	h.saveChatMessage(c.Request.Context(), userMessage)

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
	var assistantResponse strings.Builder
	c.Stream(func(w io.Writer) bool {
		resp, err := stream.Recv()
		if err == io.EOF {
			h.logger.Info("✅ [Handler] Chat stream completed",
				"messages_sent", messageCount,
				"query", reqBody.Query,
			)

			// Save assistant's complete response
			if assistantResponse.Len() > 0 {
				assistantMessage := models.ChatMessage{
					SessionID: sessionID,
					Role:      "assistant",
					Content:   assistantResponse.String(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				h.saveChatMessage(c.Request.Context(), assistantMessage)
				h.logger.Debug("💬 [Handler] Saved assistant response",
					"session_id", sessionID,
					"length", assistantResponse.Len(),
				)
			}

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

		// Accumulate assistant's response
		assistantResponse.WriteString(resp.Answer)

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

// getChatHistory fetches chat history using hybrid approach (Redis -> Postgres)
func (h *ChatHandler) getChatHistory(ctx context.Context, sessionID uuid.UUID) ([]models.ChatMessage, error) {
	// Try Redis first
	messages, err := h.redisClient.GetChatHistory(ctx, sessionID)
	if err == nil && len(messages) > 0 {
		h.logger.Debug("✅ [Handler] Chat history loaded from Redis",
			"session_id", sessionID,
			"count", len(messages),
		)
		return messages, nil
	}

	// Redis miss - fetch from Postgres
	h.logger.Debug("🔄 [Handler] Redis miss, fetching from Postgres",
		"session_id", sessionID,
	)

	messages, err = h.chatRepo.GetRecentMessages(sessionID, 20)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages from Postgres: %w", err)
	}

	// Populate Redis cache asynchronously (tracked by worker pool)
	if len(messages) > 0 {
		sessionIDCopy := sessionID
		messagesCopy := messages
		h.workerPool.SubmitWithTimeout(5*time.Second, func(ctx context.Context) {
			if err := h.redisClient.SetChatHistory(ctx, sessionIDCopy, messagesCopy); err != nil {
				h.logger.Warn("⚠️ [Handler] Failed to populate Redis cache",
					"session_id", sessionIDCopy,
					"error", err,
				)
			}
		})
	}

	h.logger.Debug("✅ [Handler] Chat history loaded from Postgres",
		"session_id", sessionID,
		"count", len(messages),
	)

	return messages, nil
}

// saveChatMessage saves a message to both Redis and Postgres
func (h *ChatHandler) saveChatMessage(ctx context.Context, message models.ChatMessage) {
	// Push to Redis immediately (synchronous for speed)
	if err := h.redisClient.AppendChatMessage(ctx, message.SessionID, message); err != nil {
		h.logger.Error("❌ [Handler] Failed to append message to Redis",
			"session_id", message.SessionID,
			"error", err,
		)
	}

	// Save to Postgres asynchronously (tracked by worker pool)
	messageCopy := message
	h.workerPool.SubmitWithTimeout(10*time.Second, func(ctx context.Context) {
		if err := h.chatRepo.CreateMessage(&messageCopy); err != nil {
			h.logger.Error("❌ [Handler] Failed to save message to Postgres",
				"session_id", messageCopy.SessionID,
				"message_id", messageCopy.ID,
				"error", err,
			)
		} else {
			h.logger.Debug("💾 [Handler] Message saved to Postgres",
				"session_id", messageCopy.SessionID,
				"message_id", messageCopy.ID,
			)
		}
	})
}

// formatChatHistory converts messages to JSON format for gRPC metadata
func formatChatHistory(messages []models.ChatMessage) string {
	if len(messages) == 0 {
		return "[]"
	}

	type historyEntry struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	history := make([]historyEntry, len(messages))
	for i, msg := range messages {
		history[i] = historyEntry{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	data, err := json.Marshal(history)
	if err != nil {
		return "[]"
	}

	return string(data)
}
