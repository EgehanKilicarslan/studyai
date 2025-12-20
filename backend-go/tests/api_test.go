package tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/internal/api"
)

// init runs automatically before tests start
func init() {
	// Disable debug mode and access logs for cleaner test output
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard
}

func TestHealthCheck(t *testing.T) {
	// Arrange
	handler := api.NewHandler(nil)
	router := api.SetupRouter(handler)

	// Act
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, `{"status":"ok"}`, w.Body.String())
}
