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
	groupHandler *handler.GroupHandler,
	adminHandler *handler.AdminHandler,
	userHandler *handler.UserHandler,
	planHandler *handler.PlanHandler,
	authMiddleware *middleware.AuthMiddleware,
) *gin.Engine {
	r := gin.Default()
	r.SetTrustedProxies(nil)

	// Public routes
	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Public plan info routes
	plansGroup := r.Group("/api/v1/plans")
	{
		plansGroup.GET("", planHandler.GetAllPlans)                        // All plans combined
		plansGroup.GET("/users", planHandler.GetUserPlans)                 // User plan tiers
		plansGroup.GET("/organizations", planHandler.GetOrganizationPlans) // Organization plan tiers
		plansGroup.GET("/groups", planHandler.GetGroupPlans)               // Standalone group plan tiers
	}

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

		// User-specific routes (/me)
		meRoutes := api.Group("/me")
		{
			// User profile and quota (read-only for users)
			meRoutes.GET("/profile", userHandler.GetProfile)
			meRoutes.GET("/quota", userHandler.GetQuota)

			// Permission checks (read-only - helps frontend know what user can do)
			meRoutes.GET("/can-create-organization", userHandler.CheckCanCreateOrganization)
			meRoutes.GET("/can-join-organization", userHandler.CheckCanJoinOrganization)
			meRoutes.GET("/can-create-group", userHandler.CheckCanCreateStandaloneGroup)

			// Existing routes
			meRoutes.GET("/organizations", adminHandler.ListMyOrganizations)
			meRoutes.GET("/groups", groupHandler.ListMyGroups)
		}

		// Organization routes
		orgRoutes := api.Group("/organizations")
		{
			// Organization CRUD
			orgRoutes.POST("", adminHandler.CreateOrganization)
			orgRoutes.GET("/:id", adminHandler.GetOrganization)
			orgRoutes.PUT("/:id", adminHandler.UpdateOrganization)
			orgRoutes.DELETE("/:id", adminHandler.DeleteOrganization)

			// Organization member management
			orgRoutes.GET("/:id/members", adminHandler.ListMembers)
			orgRoutes.POST("/:id/members", adminHandler.AddMember)
			orgRoutes.PUT("/:id/members/:user_id", adminHandler.UpdateMemberRole)
			orgRoutes.DELETE("/:id/members/:user_id", adminHandler.RemoveMember)

			// Organization groups (for reference)
			orgRoutes.GET("/:id/groups", groupHandler.ListGroupsByOrganization)
		}

		// Group routes
		groupRoutes := api.Group("/groups")
		{
			// Group CRUD
			groupRoutes.POST("", groupHandler.CreateGroup)
			groupRoutes.GET("/:group_id", groupHandler.GetGroup)
			groupRoutes.PUT("/:group_id", groupHandler.UpdateGroup)
			groupRoutes.DELETE("/:group_id", groupHandler.DeleteGroup)

			// Role management
			groupRoutes.GET("/:group_id/roles", groupHandler.ListRoles)
			groupRoutes.POST("/:group_id/roles", groupHandler.CreateRole)
			groupRoutes.PUT("/:group_id/roles/:role_id", groupHandler.UpdateRole)
			groupRoutes.DELETE("/:group_id/roles/:role_id", groupHandler.DeleteRole)

			// Member management
			groupRoutes.GET("/:group_id/members", groupHandler.ListMembers)
			groupRoutes.POST("/:group_id/members", groupHandler.AddMember)
			groupRoutes.PUT("/:group_id/members/:user_id", groupHandler.UpdateMemberRole)
			groupRoutes.DELETE("/:group_id/members/:user_id", groupHandler.RemoveMember)
		}

		// Admin routes (protected, requires admin role check in handler)
		adminRoutes := api.Group("/admin")
		{
			// Organization billing management
			adminRoutes.PUT("/organizations/:id/tier", adminHandler.UpdateTier)
			adminRoutes.PUT("/organizations/:id/billing", adminHandler.UpdateBillingStatus)
			adminRoutes.GET("/organizations/:id/quota", adminHandler.GetOrganizationQuota)

			// Standalone group billing management
			adminRoutes.PUT("/groups/:group_id/tier", groupHandler.UpdateGroupTier)
			adminRoutes.PUT("/groups/:group_id/billing", groupHandler.UpdateGroupBillingStatus)
			adminRoutes.GET("/groups/:group_id/quota", groupHandler.GetGroupQuota)

			// User billing management (admin endpoints for managing other users)
			adminRoutes.PUT("/users/:user_id/tier", adminHandler.UpdateUserTier)
			adminRoutes.PUT("/users/:user_id/billing", adminHandler.UpdateUserBillingStatus)
			adminRoutes.GET("/users/:user_id/quota", adminHandler.GetUserQuota)
		}
	}

	return r
}
