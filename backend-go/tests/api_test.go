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
	"github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/internal/rag"
	pb "github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/pb"
)

// --- MOCK DEFINITIONS ---

// MockRagServiceClient, pb.RagServiceClient arayüzünü taklit eder
type MockRagServiceClient struct {
	mock.Mock
}

func (m *MockRagServiceClient) Chat(ctx context.Context, in *pb.ChatRequest, opts ...grpc.CallOption) (pb.RagService_ChatClient, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(pb.RagService_ChatClient), args.Error(1)
}

func (m *MockRagServiceClient) UploadDocument(ctx context.Context, in *pb.UploadRequest, opts ...grpc.CallOption) (*pb.UploadResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.UploadResponse), args.Error(1)
}

// MockChatStream, streaming yanıtları taklit eder
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

// --- CUSTOM RESPONSE RECORDER ---

// StreamRecorder, httptest.ResponseRecorder'ı sarmalar ve http.CloseNotifier arayüzünü uygular.
// Gin'in c.Stream() fonksiyonu bu metoda ihtiyaç duyar.
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

// CloseNotify, http.CloseNotifier arayüzünü tatmin eder
func (w *StreamRecorder) CloseNotify() <-chan bool {
	return w.closeNotifyChan
}

// --- TESTS ---

func init() {
	// Test modunda logları kapat
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard
}

func TestHealthCheck(t *testing.T) {
	handler := api.NewHandler(nil)
	router := api.SetupRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

func TestChatHandler_Success(t *testing.T) {
	// 1. Mock Kurulumu
	mockClient := new(MockRagServiceClient)
	mockStream := new(MockChatStream)

	// Stream yanıtlarını ayarla
	mockStream.On("Recv").Return(&pb.ChatResponse{
		Answer: "Hello from Go Test",
	}, nil).Once()
	mockStream.On("Recv").Return(nil, io.EOF).Once()

	mockClient.On("Chat", mock.Anything, mock.Anything).Return(mockStream, nil)

	ragClient := &rag.Client{
		Service: mockClient,
	}

	handler := api.NewHandler(ragClient)
	router := api.SetupRouter(handler)

	// 2. HTTP İsteği
	reqBody := map[string]string{"query": "Hi", "session_id": "1"}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/chat", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	// DÜZELTME: httptest.NewRecorder() yerine özel StreamRecorder kullanıyoruz
	w := NewStreamRecorder()

	// 3. Çalıştır
	router.ServeHTTP(w, req)

	// 4. Doğrulama
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "Hello from Go Test")
}

func TestUploadHandler_Success(t *testing.T) {
	// 1. Mock Kurulumu
	mockClient := new(MockRagServiceClient)

	expectedResp := &pb.UploadResponse{
		Status:      "success",
		Message:     "File uploaded",
		ChunksCount: 10,
	}
	mockClient.On("UploadDocument", mock.Anything, mock.Anything).Return(expectedResp, nil)

	ragClient := &rag.Client{Service: mockClient}
	handler := api.NewHandler(ragClient)
	router := api.SetupRouter(handler)

	// 2. Multipart Upload İsteği
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("dummy content"))
	writer.Close()

	req, _ := http.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()

	// 3. Çalıştır
	router.ServeHTTP(w, req)

	// 4. Doğrulama
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "success")
	assert.Contains(t, w.Body.String(), "10")
}
