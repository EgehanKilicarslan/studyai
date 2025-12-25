package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"

	"github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/internal/api"
	"github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/internal/config"
	"github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/internal/rag"
	pb "github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/pb"
)

type MockRagServiceClient struct {
	mock.Mock
}

func (m *MockRagServiceClient) Chat(ctx context.Context, in *pb.ChatRequest, opts ...grpc.CallOption) (pb.RagService_ChatClient, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(pb.RagService_ChatClient), args.Error(1)
}

func (m *MockRagServiceClient) UploadDocument(ctx context.Context, opts ...grpc.CallOption) (pb.RagService_UploadDocumentClient, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pb.RagService_UploadDocumentClient), args.Error(1)
}

type MockChatStream struct {
	grpc.ClientStream
	mock.Mock
}

func (m *MockChatStream) Recv() (*pb.ChatResponse, error) {
	args := m.Called()
	if resp := args.Get(0); resp != nil {
		return resp.(*pb.ChatResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

type MockUploadStream struct {
	grpc.ClientStream
	mock.Mock
}

func (m *MockUploadStream) Send(req *pb.UploadRequest) error {
	args := m.Called(req)
	return args.Error(0)
}

func (m *MockUploadStream) CloseAndRecv() (*pb.UploadResponse, error) {
	args := m.Called()
	return args.Get(0).(*pb.UploadResponse), args.Error(1)
}

type StreamRecorder struct {
	*httptest.ResponseRecorder
	closeNotifyChan chan bool
}

func NewStreamRecorder() *StreamRecorder {
	return &StreamRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeNotifyChan:  make(chan bool, 1),
	}
}

func (w *StreamRecorder) CloseNotify() <-chan bool {
	return w.closeNotifyChan
}

func init() {
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard
}

func setupRouter(ragClient *rag.Client) *gin.Engine {
	cfg := &config.Config{
		ApiServicePort: "8080",
		AIServiceAddr:  "localhost:50051",
		MaxFileSize:    10 * 1024 * 1024,
		UploadTimeout:  300,
	}
	handler := api.NewHandler(ragClient, cfg)
	return api.SetupRouter(handler)
}

func TestHealthCheck(t *testing.T) {
	router := setupRouter(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

func TestChatHandler_Success(t *testing.T) {
	// 1. ARRANGE
	mockClient := new(MockRagServiceClient)
	mockStream := new(MockChatStream)

	mockStream.On("Recv").Return(&pb.ChatResponse{
		Answer: "Hello from Go Test",
	}, nil).Once()
	mockStream.On("Recv").Return(nil, io.EOF).Once()

	mockClient.On("Chat", mock.Anything, mock.Anything).Return(mockStream, nil)

	ragClient := &rag.Client{Service: mockClient}
	router := setupRouter(ragClient)

	reqBody := map[string]string{"query": "Hi", "session_id": "1"}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/chat", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := NewStreamRecorder()

	// 2. ACT
	router.ServeHTTP(w, req)

	// 3. ASSERT
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "Hello from Go Test")
}

func TestUploadHandler_Success(t *testing.T) {
	// 1. ARRANGE
	mockClient := new(MockRagServiceClient)
	mockUploadStream := new(MockUploadStream)

	// Mock the stream creation
	mockClient.On("UploadDocument", mock.Anything).Return(mockUploadStream, nil)

	// Mock Send calls (metadata + chunks)
	mockUploadStream.On("Send", mock.AnythingOfType("*pb.UploadRequest")).Return(nil)

	// Mock the final response
	expectedResp := &pb.UploadResponse{
		Status:      "success",
		Message:     "File uploaded",
		ChunksCount: 10,
	}
	mockUploadStream.On("CloseAndRecv").Return(expectedResp, nil)

	ragClient := &rag.Client{Service: mockClient}
	router := setupRouter(ragClient)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("dummy content"))
	writer.Close()

	req, _ := http.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()

	// 2. ACT
	router.ServeHTTP(w, req)

	// 3. ASSERT
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "success")
	assert.Contains(t, w.Body.String(), "10")

	mockClient.AssertExpectations(t)
	mockUploadStream.AssertExpectations(t)
}
