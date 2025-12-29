package api

import (
	"github.com/gin-gonic/gin"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/handler"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/middleware"
)

func SetupRouter(
	apiHandler *handler.ApiHandler,
	authHandler *handler.AuthHandler,
	authMiddleware *middleware.AuthMiddleware,
) *gin.Engine {
	r := gin.Default()
	r.SetTrustedProxies(nil)

	// Public routes
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Auth routes (Public)
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.RefreshToken)
		authGroup.POST("/logout", authHandler.Logout)
	}

	// Protected API routes
	api := r.Group("/api")
	api.Use(authMiddleware.RequireAuth())
	{
		api.POST("/chat", apiHandler.ChatHandler)
		api.POST("/upload", apiHandler.UploadHandler)
	}

	return r
}
