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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/rag"
	pb "github.com/EgehanKilicarslan/studyai/backend-go/pb"
	"github.com/EgehanKilicarslan/studyai/backend-go/tests/testutil"
)

// ==================== HEALTH CHECK TEST ====================

func TestHealthCheck(t *testing.T) {
	router := testutil.SetupRouterWithDefaultAuth(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

// ==================== CHAT HANDLER TESTS ====================

func TestChatHandler(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		authHeader     string
		setupMocks     func(*testutil.MockRagServiceClient, *testutil.MockChatStream)
		expectedStatus int
		checkResponse  func(*testing.T, *testutil.StreamRecorder)
		useStreamRec   bool
	}{
		{
			name:        "success",
			requestBody: map[string]string{"query": "Hi", "session_id": "1"},
			authHeader:  "Bearer test-token",
			setupMocks: func(client *testutil.MockRagServiceClient, stream *testutil.MockChatStream) {
				stream.On("Recv").Return(&pb.ChatResponse{Answer: "Hello from Go Test"}, nil).Once()
				stream.On("Recv").Return(nil, io.EOF).Once()
				client.On("Chat", mock.Anything, mock.Anything).Return(stream, nil)
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *testutil.StreamRecorder) {
				assert.Contains(t, w.Body.String(), "Hello from Go Test")
			},
			useStreamRec: true,
		},
		{
			name:           "invalid json",
			requestBody:    "invalid json",
			authHeader:     "Bearer test-token",
			setupMocks:     func(client *testutil.MockRagServiceClient, stream *testutil.MockChatStream) {},
			expectedStatus: 400,
			checkResponse: func(t *testing.T, w *testutil.StreamRecorder) {
				assert.Contains(t, w.Body.String(), "error")
			},
		},
		{
			name:           "missing query",
			requestBody:    map[string]string{"session_id": "123"},
			authHeader:     "Bearer test-token",
			setupMocks:     func(client *testutil.MockRagServiceClient, stream *testutil.MockChatStream) {},
			expectedStatus: 400,
			checkResponse: func(t *testing.T, w *testutil.StreamRecorder) {
				assert.Contains(t, w.Body.String(), "Query")
			},
		},
		{
			name:           "missing auth header",
			requestBody:    map[string]string{"query": "Hi", "session_id": "1"},
			authHeader:     "",
			setupMocks:     func(client *testutil.MockRagServiceClient, stream *testutil.MockChatStream) {},
			expectedStatus: 401,
		},
		{
			name:        "grpc error",
			requestBody: map[string]string{"query": "test", "session_id": "123"},
			authHeader:  "Bearer test-token",
			setupMocks: func(client *testutil.MockRagServiceClient, stream *testutil.MockChatStream) {
				client.On("Chat", mock.Anything, mock.Anything).Return((*testutil.MockChatStream)(nil), errors.New("connection failed"))
			},
			expectedStatus: 500,
		},
		{
			name:        "stream error",
			requestBody: map[string]string{"query": "test", "session_id": "123"},
			authHeader:  "Bearer test-token",
			setupMocks: func(client *testutil.MockRagServiceClient, stream *testutil.MockChatStream) {
				stream.On("Recv").Return((*pb.ChatResponse)(nil), io.ErrUnexpectedEOF).Once()
				client.On("Chat", mock.Anything, mock.Anything).Return(stream, nil)
			},
			expectedStatus: 200, // Handler starts streaming then error occurs
			useStreamRec:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(testutil.MockRagServiceClient)
			mockStream := new(testutil.MockChatStream)
			tt.setupMocks(mockClient, mockStream)

			ragClient := &rag.Client{Service: mockClient}
			router := testutil.SetupRouterWithDefaultAuth(ragClient)

			var body *bytes.Buffer
			switch v := tt.requestBody.(type) {
			case string:
				body = bytes.NewBuffer([]byte(v))
			default:
				jsonBody, _ := json.Marshal(v)
				body = bytes.NewBuffer(jsonBody)
			}

			req, _ := http.NewRequest("POST", "/api/chat", body)
			req.Header.Set("Content-Type", "application/json")
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			var w *testutil.StreamRecorder
			if tt.useStreamRec {
				w = testutil.NewStreamRecorder()
			} else {
				w = &testutil.StreamRecorder{ResponseRecorder: httptest.NewRecorder()}
			}

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			mockClient.AssertExpectations(t)
			mockStream.AssertExpectations(t)
		})
	}
}

func TestChatHandler_Unauthorized(t *testing.T) {
	mockAuthService := new(testutil.MockAuthService)
	mockAuthService.On("ValidateAccessToken", mock.Anything).Return(uint(0), service.ErrInvalidToken)

	router := testutil.SetupRouterWithMocks(nil, mockAuthService)

	reqBody := map[string]string{"query": "Hi", "session_id": "1"}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/chat", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

// ==================== UPLOAD HANDLER TESTS ====================

func TestUploadHandler(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() (*http.Request, error)
		setupMocks     func(*testutil.MockRagServiceClient, *testutil.MockUploadStream)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "success",
			setupRequest: func() (*http.Request, error) {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write([]byte("dummy content"))
				writer.Close()

				req, _ := http.NewRequest("POST", "/api/upload", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("Authorization", "Bearer test-token")
				return req, nil
			},
			setupMocks: func(client *testutil.MockRagServiceClient, stream *testutil.MockUploadStream) {
				client.On("UploadDocument", mock.Anything).Return(stream, nil)
				stream.On("Send", mock.AnythingOfType("*pb.UploadRequest")).Return(nil)
				stream.On("CloseAndRecv").Return(&pb.UploadResponse{
					Status:      "success",
					Message:     "File uploaded",
					ChunksCount: 10,
				}, nil)
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "success")
				assert.Contains(t, w.Body.String(), "10")
			},
		},
		{
			name: "no file",
			setupRequest: func() (*http.Request, error) {
				req, _ := http.NewRequest("POST", "/api/upload", nil)
				req.Header.Set("Content-Type", "multipart/form-data")
				req.Header.Set("Authorization", "Bearer test-token")
				return req, nil
			},
			setupMocks:     func(client *testutil.MockRagServiceClient, stream *testutil.MockUploadStream) {},
			expectedStatus: 400,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "error")
			},
		},
		{
			name: "invalid content type",
			setupRequest: func() (*http.Request, error) {
				req, _ := http.NewRequest("POST", "/api/upload", bytes.NewBuffer([]byte("test")))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test-token")
				return req, nil
			},
			setupMocks:     func(client *testutil.MockRagServiceClient, stream *testutil.MockUploadStream) {},
			expectedStatus: 400,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "error")
			},
		},
		{
			name: "grpc error",
			setupRequest: func() (*http.Request, error) {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write([]byte("content"))
				writer.Close()

				req, _ := http.NewRequest("POST", "/api/upload", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("Authorization", "Bearer test-token")
				return req, nil
			},
			setupMocks: func(client *testutil.MockRagServiceClient, stream *testutil.MockUploadStream) {
				client.On("UploadDocument", mock.Anything).Return((*testutil.MockUploadStream)(nil), errors.New("connection failed"))
			},
			expectedStatus: 500,
		},
		{
			name: "stream send error",
			setupRequest: func() (*http.Request, error) {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write([]byte("content"))
				writer.Close()

				req, _ := http.NewRequest("POST", "/api/upload", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("Authorization", "Bearer test-token")
				return req, nil
			},
			setupMocks: func(client *testutil.MockRagServiceClient, stream *testutil.MockUploadStream) {
				client.On("UploadDocument", mock.Anything).Return(stream, nil)
				stream.On("Send", mock.Anything).Return(nil).Once()
				stream.On("Send", mock.Anything).Return(errors.New("send failed")).Once()
			},
			expectedStatus: 500,
		},
		{
			name: "close and recv error",
			setupRequest: func() (*http.Request, error) {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write([]byte("content"))
				writer.Close()

				req, _ := http.NewRequest("POST", "/api/upload", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("Authorization", "Bearer test-token")
				return req, nil
			},
			setupMocks: func(client *testutil.MockRagServiceClient, stream *testutil.MockUploadStream) {
				client.On("UploadDocument", mock.Anything).Return(stream, nil)
				stream.On("Send", mock.Anything).Return(nil)
				stream.On("CloseAndRecv").Return((*pb.UploadResponse)(nil), errors.New("close failed"))
			},
			expectedStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(testutil.MockRagServiceClient)
			mockStream := new(testutil.MockUploadStream)
			tt.setupMocks(mockClient, mockStream)

			ragClient := &rag.Client{Service: mockClient}
			router := testutil.SetupRouterWithDefaultAuth(ragClient)

			req, _ := tt.setupRequest()
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			mockClient.AssertExpectations(t)
			mockStream.AssertExpectations(t)
		})
	}
}

func TestUploadHandler_Unauthorized(t *testing.T) {
	mockAuthService := new(testutil.MockAuthService)
	mockAuthService.On("ValidateAccessToken", mock.Anything).Return(uint(0), service.ErrInvalidToken)

	router := testutil.SetupRouterWithMocks(nil, mockAuthService)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("dummy content"))
	writer.Close()

	req, _ := http.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer invalid-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}
