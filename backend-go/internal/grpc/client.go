package grpc

import (
	"crypto/tls"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	pb "github.com/EgehanKilicarslan/studyai/backend-go/pb"
)

// Store for gRPC client and connection
type Client struct {
	ChatService          pb.ChatServiceClient
	KnowledgeBaseService pb.KnowledgeBaseServiceClient
	conn                 *grpc.ClientConn
}

// Creates a new gRPC client
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
				Time:                30 * time.Second,
				Timeout:             10 * time.Second,
				PermitWithoutStream: false,
			}),
		)...,
	)
	if err != nil {
		return nil, err
	}

	chatClient := pb.NewChatServiceClient(conn)
	knowledgeBaseClient := pb.NewKnowledgeBaseServiceClient(conn)
	return &Client{
		ChatService:          chatClient,
		KnowledgeBaseService: knowledgeBaseClient,
		conn:                 conn,
	}, nil
}

// Closes the gRPC connection
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
