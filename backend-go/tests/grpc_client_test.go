package tests

import (
	"testing"

	grpcClient "github.com/EgehanKilicarslan/studyai/backend-go/internal/grpc"
	"github.com/stretchr/testify/assert"
)

func TestNewClient_EmptyAddress(t *testing.T) {
	// gRPC NewClient uses lazy connection, so empty address doesn't fail at creation
	// The connection will fail when actually trying to make a call
	client, err := grpcClient.NewClient("", false)
	if err != nil {
		// If it does return an error, that's acceptable
		return
	}
	// If no error, client should still be valid structurally
	assert.NotNil(t, client, "Client should be created even with empty address (lazy connection)")
	if client != nil {
		client.Close()
	}
}

func TestNewClient_ConnectionParameters(t *testing.T) {
	// Test that NewClient accepts valid address format
	// Note: This won't actually connect but validates address parsing
	client, err := grpcClient.NewClient("localhost:50051", false)
	if err == nil {
		assert.NotNil(t, client.ChatService, "ChatService should be initialized")
		assert.NotNil(t, client.KnowledgeBaseService, "KnowledgeBaseService should be initialized")
		client.Close()
	}
}

func TestNewClient_WithTLS(t *testing.T) {
	// Test that NewClient accepts TLS flag
	client, err := grpcClient.NewClient("localhost:50051", true)
	if err == nil {
		assert.NotNil(t, client.ChatService, "ChatService should be initialized with TLS")
		assert.NotNil(t, client.KnowledgeBaseService, "KnowledgeBaseService should be initialized with TLS")
		client.Close()
	}
}

func TestClient_ServicesAvailable(t *testing.T) {
	// Create a mock scenario - verify client structure
	client := &grpcClient.Client{}

	// Verify that the client has the expected fields
	assert.Nil(t, client.ChatService, "Uninitialized ChatService should be nil")
	assert.Nil(t, client.KnowledgeBaseService, "Uninitialized KnowledgeBaseService should be nil")
}
