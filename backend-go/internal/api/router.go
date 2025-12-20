package api

import "github.com/gin-gonic/gin"

// SetupRouter defines routes and returns the engine
func SetupRouter(h *Handler) *gin.Engine {
	r := gin.Default()
	r.SetTrustedProxies(nil)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Bind handler function
	r.POST("/api/chat", h.ChatHandler)

	return r
}
