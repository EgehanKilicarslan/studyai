package rag

import (
	"crypto/tls"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	pb "github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/pb"
)

// Store RAG service client
type Client struct {
	Service pb.RagServiceClient
	conn    *grpc.ClientConn
}

// Creates a new RAG service client
func NewClient(addr string, useTLS bool) (*Client, error) {
	var opts []grpc.DialOption
	if useTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(
		addr,
		append(opts,
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(50*1024*1024), // 50MB for large responses
				grpc.MaxCallSendMsgSize(50*1024*1024),
			),
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                10 * time.Second,
				Timeout:             3 * time.Second,
				PermitWithoutStream: true,
			}),
		)...,
	)
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
