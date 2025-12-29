package tests

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/rag"
	pb "github.com/EgehanKilicarslan/studyai/backend-go/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestChatHandler_InvalidJSON(t *testing.T) {
	mockClient := new(MockRagServiceClient)
	ragClient := &rag.Client{Service: mockClient}
	router := setupRouter(ragClient)

	req, _ := http.NewRequest("POST", "/api/chat", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestChatHandler_MissingQuery(t *testing.T) {
	mockClient := new(MockRagServiceClient)
	ragClient := &rag.Client{Service: mockClient}
	router := setupRouter(ragClient)

	reqBody := map[string]string{"session_id": "123"}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/chat", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "Query")
}

func TestChatHandler_GrpcError(t *testing.T) {
	mockClient := new(MockRagServiceClient)

	// Mock gRPC connection failure
	mockClient.On("Chat", mock.Anything, mock.Anything).Return((*MockChatStream)(nil), errors.New("connection failed"))

	ragClient := &rag.Client{Service: mockClient}
	router := setupRouter(ragClient)

	reqBody := map[string]string{"query": "test", "session_id": "123"}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/chat", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
	mockClient.AssertExpectations(t)
}

func TestChatHandler_StreamError(t *testing.T) {
	mockClient := new(MockRagServiceClient)
	mockStream := new(MockChatStream)

	// Return error on first Recv
	mockStream.On("Recv").Return((*pb.ChatResponse)(nil), io.ErrUnexpectedEOF).Once()
	mockClient.On("Chat", mock.Anything, mock.Anything).Return(mockStream, nil)

	ragClient := &rag.Client{Service: mockClient}
	router := setupRouter(ragClient)

	reqBody := map[string]string{"query": "test", "session_id": "123"}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/chat", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := NewStreamRecorder()

	router.ServeHTTP(w, req)

	// Handler starts streaming (200) but error occurs during streaming
	assert.Equal(t, 200, w.Code)
	mockClient.AssertExpectations(t)
	mockStream.AssertExpectations(t)
}

func TestUploadHandler_NoFile(t *testing.T) {
	mockClient := new(MockRagServiceClient)
	ragClient := &rag.Client{Service: mockClient}
	router := setupRouter(ragClient)

	req, _ := http.NewRequest("POST", "/api/upload", nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestUploadHandler_InvalidContentType(t *testing.T) {
	mockClient := new(MockRagServiceClient)
	ragClient := &rag.Client{Service: mockClient}
	router := setupRouter(ragClient)

	req, _ := http.NewRequest("POST", "/api/upload", bytes.NewBuffer([]byte("test")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestUploadHandler_StreamSendError(t *testing.T) {
	mockClient := new(MockRagServiceClient)
	mockUploadStream := new(MockUploadStream)

	mockClient.On("UploadDocument", mock.Anything).Return(mockUploadStream, nil)
	// First Send (metadata) succeeds, second Send (chunk) fails
	mockUploadStream.On("Send", mock.Anything).Return(nil).Once()
	mockUploadStream.On("Send", mock.Anything).Return(errors.New("send failed")).Once()

	ragClient := &rag.Client{Service: mockClient}
	router := setupRouter(ragClient)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("content"))
	writer.Close()

	req, _ := http.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
	mockClient.AssertExpectations(t)
	mockUploadStream.AssertExpectations(t)
}

func TestUploadHandler_GrpcError(t *testing.T) {
	mockClient := new(MockRagServiceClient)

	// Mock gRPC connection failure
	mockClient.On("UploadDocument", mock.Anything).Return((*MockUploadStream)(nil), errors.New("connection failed"))

	ragClient := &rag.Client{Service: mockClient}
	router := setupRouter(ragClient)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("content"))
	writer.Close()

	req, _ := http.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
	mockClient.AssertExpectations(t)
}

func TestUploadHandler_CloseAndRecvError(t *testing.T) {
	mockClient := new(MockRagServiceClient)
	mockUploadStream := new(MockUploadStream)

	mockClient.On("UploadDocument", mock.Anything).Return(mockUploadStream, nil)
	mockUploadStream.On("Send", mock.Anything).Return(nil)
	mockUploadStream.On("CloseAndRecv").Return((*pb.UploadResponse)(nil), errors.New("close failed"))

	ragClient := &rag.Client{Service: mockClient}
	router := setupRouter(ragClient)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("content"))
	writer.Close()

	req, _ := http.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
	mockClient.AssertExpectations(t)
	mockUploadStream.AssertExpectations(t)
}
