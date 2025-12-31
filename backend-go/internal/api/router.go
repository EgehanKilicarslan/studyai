package api

import (
	"github.com/gin-gonic/gin"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/handler"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/middleware"
)

func SetupRouter(
	chatHandler *handler.ChatHandler,
	knowledgeBaseHandler *handler.KnowledgeBaseHandler,
	authHandler *handler.AuthHandler,
	authMiddleware *middleware.AuthMiddleware,
) *gin.Engine {
	r := gin.Default()
	r.SetTrustedProxies(nil)

	// Public routes
	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Auth routes (Public)
	authGroup := r.Group("/api/v1/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.RefreshToken)
		authGroup.POST("/logout", authHandler.Logout)
	}

	// Protected API routes
	api := r.Group("/api/v1")
	api.Use(authMiddleware.RequireAuth())
	{
		api.POST("/chat", chatHandler.ChatHandler)
		api.POST("/upload", knowledgeBaseHandler.UploadHandler)
		api.GET("/knowledge-base", knowledgeBaseHandler.ListHandler)
		api.DELETE("/knowledge-base/:document_id", knowledgeBaseHandler.DeleteHandler)
	}

	return r
}
