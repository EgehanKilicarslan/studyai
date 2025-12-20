package rag

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/pb"
)

// Store RAG service client
type Client struct {
	Service pb.RagServiceClient
	conn    *grpc.ClientConn
}

// Creates a new RAG service client
func NewClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := pb.NewRagServiceClient(conn)
	return &Client{
		Service: client,
		conn:    conn,
	}, nil
}

// Closes the gRPC connection
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
