package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
)

// AuthMiddleware handles JWT validation
type AuthMiddleware struct {
	service service.AuthService
	logger  *slog.Logger
}

// NewAuthMiddleware creates a new auth middleware instance
func NewAuthMiddleware(service service.AuthService, logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		service: service,
		logger:  logger,
	}
}

// RequireAuth validates JWT token and sets userID in context
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			m.logger.Warn("⚠️ [Middleware] Missing Authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			m.logger.Warn("⚠️ [Middleware] Invalid Authorization header format")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		userID, err := m.service.ValidateAccessToken(tokenString)
		if err != nil {
			m.logger.Warn("⚠️ [Middleware] Invalid token", "error", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("userID", userID)
		m.logger.Debug("✅ [Middleware] Token validated", "user_id", userID)

		c.Next()
	}
}
