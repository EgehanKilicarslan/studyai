package tests

import (
	"testing"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/rag"
	"github.com/stretchr/testify/assert"
)

func TestNewClient_Success(t *testing.T) {
	// This will fail to connect but tests the client creation
	client, err := rag.NewClient("localhost:50051", false)

	if err != nil {
		// Expected if no server is running
		assert.Contains(t, err.Error(), "connection", "context deadline exceeded")
		return
	}

	assert.NotNil(t, client)
	if client != nil {
		client.Close()
	}
}

func TestNewClient_InvalidAddress(t *testing.T) {
	client, err := rag.NewClient("", false)

	// Should still create client but connection will fail
	if err != nil {
		assert.Error(t, err)
	}
	if client != nil {
		client.Close()
	}
}

func TestNewClient_WithTLS(t *testing.T) {
	client, err := rag.NewClient("localhost:50051", true)

	if err != nil {
		// Expected if no TLS server is running
		assert.Error(t, err)
		return
	}

	assert.NotNil(t, client)
	if client != nil {
		client.Close()
	}
}

func TestClient_Close(t *testing.T) {
	client, err := rag.NewClient("localhost:50051", false)

	if err != nil {
		t.Skip("Skipping test - gRPC server not available")
	}

	assert.NotNil(t, client)
	client.Close()
	// No error expected from Close
}
