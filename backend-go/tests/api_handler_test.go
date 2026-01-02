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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
	pb "github.com/EgehanKilicarslan/studyai/backend-go/pb"
	"github.com/EgehanKilicarslan/studyai/backend-go/tests/testutil"
)

// ==================== HEALTH CHECK TEST ====================

func TestHealthCheck(t *testing.T) {
	router := testutil.SetupRouterWithDefaultAuth(nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", testutil.HealthCheckEndpoint, nil)
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
		setupMocks     func(*testutil.MockChatServiceClient, *testutil.MockChatStream)
		expectedStatus int
		checkResponse  func(*testing.T, *testutil.StreamRecorder)
		useStreamRec   bool
	}{
		{
			name:        "success",
			requestBody: map[string]string{"query": "Hi", "session_id": "1"},
			authHeader:  "Bearer test-token",
			setupMocks: func(client *testutil.MockChatServiceClient, stream *testutil.MockChatStream) {
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
			setupMocks:     func(client *testutil.MockChatServiceClient, stream *testutil.MockChatStream) {},
			expectedStatus: 400,
			checkResponse: func(t *testing.T, w *testutil.StreamRecorder) {
				assert.Contains(t, w.Body.String(), "error")
			},
		},
		{
			name:           "missing query",
			requestBody:    map[string]string{"session_id": "123"},
			authHeader:     "Bearer test-token",
			setupMocks:     func(client *testutil.MockChatServiceClient, stream *testutil.MockChatStream) {},
			expectedStatus: 400,
			checkResponse: func(t *testing.T, w *testutil.StreamRecorder) {
				assert.Contains(t, w.Body.String(), "Query")
			},
		},
		{
			name:           "missing auth header",
			requestBody:    map[string]string{"query": "Hi", "session_id": "1"},
			authHeader:     "",
			setupMocks:     func(client *testutil.MockChatServiceClient, stream *testutil.MockChatStream) {},
			expectedStatus: 401,
		},
		{
			name:        "grpc error",
			requestBody: map[string]string{"query": "test", "session_id": "123"},
			authHeader:  "Bearer test-token",
			setupMocks: func(client *testutil.MockChatServiceClient, stream *testutil.MockChatStream) {
				client.On("Chat", mock.Anything, mock.Anything).Return((*testutil.MockChatStream)(nil), errors.New("connection failed"))
			},
			expectedStatus: 500,
		},
		{
			name:        "stream error",
			requestBody: map[string]string{"query": "test", "session_id": "123"},
			authHeader:  "Bearer test-token",
			setupMocks: func(client *testutil.MockChatServiceClient, stream *testutil.MockChatStream) {
				stream.On("Recv").Return((*pb.ChatResponse)(nil), io.ErrUnexpectedEOF).Once()
				client.On("Chat", mock.Anything, mock.Anything).Return(stream, nil)
			},
			expectedStatus: 200, // Handler starts streaming then error occurs
			useStreamRec:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockChatClient := new(testutil.MockChatServiceClient)
			mockKBClient := new(testutil.MockKnowledgeBaseServiceClient)
			mockDocService := new(testutil.MockDocumentService)
			mockStream := new(testutil.MockChatStream)
			tt.setupMocks(mockChatClient, mockStream)

			grpcCli := testutil.CreateMockGrpcClient(mockChatClient, mockKBClient)
			router := testutil.SetupRouterWithDefaultAuth(grpcCli, mockDocService)

			var body *bytes.Buffer
			switch v := tt.requestBody.(type) {
			case string:
				body = bytes.NewBuffer([]byte(v))
			default:
				jsonBody, _ := json.Marshal(v)
				body = bytes.NewBuffer(jsonBody)
			}

			req, _ := http.NewRequest("POST", testutil.ChatEndpoint, body)
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

			mockChatClient.AssertExpectations(t)
			mockStream.AssertExpectations(t)
		})
	}
}

func TestChatHandler_Unauthorized(t *testing.T) {
	mockAuthService := new(testutil.MockAuthService)
	mockAuthService.On("ValidateAccessToken", mock.Anything).Return(uint(0), service.ErrInvalidToken)

	router := testutil.SetupRouterWithMocks(nil, nil, mockAuthService, nil, nil)

	reqBody := map[string]string{"query": "Hi", "session_id": "1"}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", testutil.ChatEndpoint, bytes.NewBuffer(jsonBody))
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
		setupMocks     func(*testutil.MockDocumentService, *testutil.MockKnowledgeBaseServiceClient)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "success",
			setupRequest: func() (*http.Request, error) {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				_ = writer.WriteField("organization_id", "1")
				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write([]byte("dummy content"))
				writer.Close()

				req, _ := http.NewRequest("POST", testutil.UploadEndpoint, body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("Authorization", "Bearer test-token")
				return req, nil
			},
			setupMocks: func(docService *testutil.MockDocumentService, kbClient *testutil.MockKnowledgeBaseServiceClient) {
				docID := uuid.New()
				orgID := uint(1)
				doc := &models.Document{
					ID:             docID,
					Name:           "test.txt",
					FilePath:       "/tmp/test.txt",
					OrganizationID: &orgID,
					OwnerID:        1,
					Status:         models.DocumentStatusPending,
				}
				updatedDoc := &models.Document{
					ID:             docID,
					Name:           "test.txt",
					FilePath:       "/tmp/test.txt",
					OrganizationID: &orgID,
					OwnerID:        1,
					Status:         models.DocumentStatusCompleted,
					ChunksCount:    10,
				}
				docService.On("CreateDocument", &orgID, (*uint)(nil), uint(1), "test.txt", mock.Anything, mock.Anything).Return(doc, nil)
				kbClient.On("ProcessDocument", mock.Anything, mock.Anything).Return(&pb.ProcessDocumentResponse{
					Status:      "success",
					Message:     "Document processed",
					ChunksCount: 10,
				}, nil)
				docService.On("UpdateDocumentStatus", docID, models.DocumentStatusCompleted, int(10), (*string)(nil)).Return(nil)
				docService.On("GetDocument", docID).Return(updatedDoc, nil)
			},
			expectedStatus: 201, // Created
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "success")
			},
		},
		{
			name: "missing organization_id",
			setupRequest: func() (*http.Request, error) {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write([]byte("dummy content"))
				writer.Close()

				req, _ := http.NewRequest("POST", testutil.UploadEndpoint, body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("Authorization", "Bearer test-token")
				return req, nil
			},
			setupMocks: func(docService *testutil.MockDocumentService, kbClient *testutil.MockKnowledgeBaseServiceClient) {
				// When organization_id is missing, it's a user-scoped document (organizationID = nil)
				docID := uuid.New()
				doc := &models.Document{
					ID:             docID,
					Name:           "test.txt",
					FilePath:       "/tmp/test.txt",
					OrganizationID: nil, // User-scoped
					OwnerID:        1,
					Status:         models.DocumentStatusPending,
				}
				updatedDoc := &models.Document{
					ID:             docID,
					Name:           "test.txt",
					FilePath:       "/tmp/test.txt",
					OrganizationID: nil,
					OwnerID:        1,
					Status:         models.DocumentStatusCompleted,
					ChunksCount:    5,
				}
				docService.On("CreateDocument", (*uint)(nil), (*uint)(nil), uint(1), "test.txt", mock.Anything, mock.Anything).Return(doc, nil)
				kbClient.On("ProcessDocument", mock.Anything, mock.Anything).Return(&pb.ProcessDocumentResponse{
					Status:      "success",
					Message:     "Document processed",
					ChunksCount: 5,
				}, nil)
				docService.On("UpdateDocumentStatus", docID, models.DocumentStatusCompleted, int(5), (*string)(nil)).Return(nil)
				docService.On("GetDocument", docID).Return(updatedDoc, nil)
			},
			expectedStatus: 201, // User-scoped documents are valid
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "success")
			},
		},
		{
			name: "no file",
			setupRequest: func() (*http.Request, error) {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				_ = writer.WriteField("organization_id", "1")
				writer.Close()

				req, _ := http.NewRequest("POST", testutil.UploadEndpoint, body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("Authorization", "Bearer test-token")
				return req, nil
			},
			setupMocks:     func(docService *testutil.MockDocumentService, kbClient *testutil.MockKnowledgeBaseServiceClient) {},
			expectedStatus: 400,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "error")
			},
		},
		{
			name: "document service error",
			setupRequest: func() (*http.Request, error) {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				_ = writer.WriteField("organization_id", "1")
				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write([]byte("content"))
				writer.Close()

				req, _ := http.NewRequest("POST", testutil.UploadEndpoint, body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("Authorization", "Bearer test-token")
				return req, nil
			},
			setupMocks: func(docService *testutil.MockDocumentService, kbClient *testutil.MockKnowledgeBaseServiceClient) {
				docService.On("CreateDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("storage error"))
			},
			expectedStatus: 500,
		},
		{
			name: "grpc process error",
			setupRequest: func() (*http.Request, error) {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				_ = writer.WriteField("organization_id", "1")
				part, _ := writer.CreateFormFile("file", "test.txt")
				part.Write([]byte("content"))
				writer.Close()

				req, _ := http.NewRequest("POST", testutil.UploadEndpoint, body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("Authorization", "Bearer test-token")
				return req, nil
			},
			setupMocks: func(docService *testutil.MockDocumentService, kbClient *testutil.MockKnowledgeBaseServiceClient) {
				docID := uuid.New()
				orgID := uint(1)
				doc := &models.Document{
					ID:             docID,
					Name:           "test.txt",
					FilePath:       "/tmp/test.txt",
					OrganizationID: &orgID,
					OwnerID:        1,
					Status:         models.DocumentStatusPending,
				}
				docService.On("CreateDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(doc, nil)
				kbClient.On("ProcessDocument", mock.Anything, mock.Anything).Return(nil, errors.New("grpc error"))
				docService.On("UpdateDocumentStatus", docID, models.DocumentStatusError, 0, mock.AnythingOfType("*string")).Return(nil)
			},
			expectedStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockChatClient := new(testutil.MockChatServiceClient)
			mockKBClient := new(testutil.MockKnowledgeBaseServiceClient)
			mockDocService := new(testutil.MockDocumentService)
			tt.setupMocks(mockDocService, mockKBClient)

			grpcCli := testutil.CreateMockGrpcClient(mockChatClient, mockKBClient)
			router := testutil.SetupRouterWithDefaultAuth(grpcCli, mockDocService)

			req, _ := tt.setupRequest()
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			mockKBClient.AssertExpectations(t)
			mockDocService.AssertExpectations(t)
		})
	}
}

func TestUploadHandler_Unauthorized(t *testing.T) {
	mockAuthService := new(testutil.MockAuthService)
	mockAuthService.On("ValidateAccessToken", mock.Anything).Return(uint(0), service.ErrInvalidToken)

	router := testutil.SetupRouterWithMocks(nil, nil, mockAuthService, nil, nil)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("organization_id", "1")
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("dummy content"))
	writer.Close()

	req, _ := http.NewRequest("POST", testutil.UploadEndpoint, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer invalid-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}

// ==================== DELETE DOCUMENT HANDLER TESTS ====================

func TestDeleteDocumentHandler(t *testing.T) {
	testDocID := uuid.New()

	tests := []struct {
		name           string
		documentID     string
		setupMocks     func(*testutil.MockDocumentService, *testutil.MockKnowledgeBaseServiceClient)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:       "success",
			documentID: testDocID.String(),
			setupMocks: func(docService *testutil.MockDocumentService, kbClient *testutil.MockKnowledgeBaseServiceClient) {
				orgID := uint(1)
				doc := &models.Document{
					ID:             testDocID,
					Name:           "test.txt",
					OrganizationID: &orgID,
					OwnerID:        1,
				}
				docService.On("GetDocument", testDocID).Return(doc, nil)
				docService.On("DeleteDocument", testDocID, uint(1)).Return(nil)
				kbClient.On("DeleteDocument", mock.Anything, mock.Anything).Return(&pb.DeleteDocumentResponse{
					Status:  "success",
					Message: "Document deleted from vector store",
				}, nil)
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "success")
			},
		},
		{
			name:       "invalid uuid",
			documentID: "invalid-uuid",
			setupMocks: func(docService *testutil.MockDocumentService, kbClient *testutil.MockKnowledgeBaseServiceClient) {
			},
			expectedStatus: 400,
		},
		{
			name:       "document not found",
			documentID: testDocID.String(),
			setupMocks: func(docService *testutil.MockDocumentService, kbClient *testutil.MockKnowledgeBaseServiceClient) {
				docService.On("GetDocument", testDocID).Return(nil, errors.New("not found"))
			},
			expectedStatus: 500,
		},
		{
			name:       "grpc error - but still succeeds if db deletion worked",
			documentID: testDocID.String(),
			setupMocks: func(docService *testutil.MockDocumentService, kbClient *testutil.MockKnowledgeBaseServiceClient) {
				orgID := uint(1)
				doc := &models.Document{
					ID:             testDocID,
					Name:           "test.txt",
					OrganizationID: &orgID,
					OwnerID:        1,
				}
				docService.On("GetDocument", testDocID).Return(doc, nil)
				docService.On("DeleteDocument", testDocID, uint(1)).Return(nil)
				kbClient.On("DeleteDocument", mock.Anything, mock.Anything).Return(nil, errors.New("service unavailable"))
			},
			expectedStatus: 200, // Still succeeds because DB deletion worked
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockChatClient := new(testutil.MockChatServiceClient)
			mockKBClient := new(testutil.MockKnowledgeBaseServiceClient)
			mockDocService := new(testutil.MockDocumentService)
			tt.setupMocks(mockDocService, mockKBClient)

			grpcCli := testutil.CreateMockGrpcClient(mockChatClient, mockKBClient)
			router := testutil.SetupRouterWithDefaultAuth(grpcCli, mockDocService)

			req, _ := http.NewRequest("DELETE", testutil.KnowledgeBaseDeleteURL+tt.documentID, nil)
			req.Header.Set("Authorization", "Bearer test-token")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			mockKBClient.AssertExpectations(t)
			mockDocService.AssertExpectations(t)
		})
	}
}

// ==================== LIST DOCUMENTS HANDLER TESTS ====================

func TestListDocumentsHandler(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		setupMocks     func(*testutil.MockDocumentService)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "success with documents",
			queryParams: "?organization_id=1",
			setupMocks: func(docService *testutil.MockDocumentService) {
				orgID := uint(1)
				docs := []models.Document{
					{
						ID:             uuid.New(),
						Name:           "test.pdf",
						FilePath:       "/tmp/test.pdf",
						OrganizationID: &orgID,
						OwnerID:        1,
						Status:         models.DocumentStatusCompleted,
						ChunksCount:    5,
					},
					{
						ID:             uuid.New(),
						Name:           "report.docx",
						FilePath:       "/tmp/report.docx",
						OrganizationID: &orgID,
						OwnerID:        1,
						Status:         models.DocumentStatusCompleted,
						ChunksCount:    10,
					},
				}
				docService.On("ListDocuments", uint(1), uint(1), 1, 20).Return(docs, int64(2), nil)
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "test.pdf")
				assert.Contains(t, w.Body.String(), "report.docx")
			},
		},
		{
			name:        "success with empty list",
			queryParams: "?organization_id=1",
			setupMocks: func(docService *testutil.MockDocumentService) {
				docService.On("ListDocuments", uint(1), uint(1), 1, 20).Return([]models.Document{}, int64(0), nil)
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "documents")
			},
		},
		{
			name:           "missing organization_id",
			queryParams:    "",
			setupMocks:     func(docService *testutil.MockDocumentService) {},
			expectedStatus: 400,
		},
		{
			name:        "service error",
			queryParams: "?organization_id=1",
			setupMocks: func(docService *testutil.MockDocumentService) {
				docService.On("ListDocuments", uint(1), uint(1), 1, 20).Return(nil, int64(0), errors.New("database error"))
			},
			expectedStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockChatClient := new(testutil.MockChatServiceClient)
			mockKBClient := new(testutil.MockKnowledgeBaseServiceClient)
			mockDocService := new(testutil.MockDocumentService)
			tt.setupMocks(mockDocService)

			grpcCli := testutil.CreateMockGrpcClient(mockChatClient, mockKBClient)
			router := testutil.SetupRouterWithDefaultAuth(grpcCli, mockDocService)

			req, _ := http.NewRequest("GET", testutil.KnowledgeBaseListURL+tt.queryParams, nil)
			req.Header.Set("Authorization", "Bearer test-token")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			mockDocService.AssertExpectations(t)
		})
	}
}

func TestListDocumentsHandler_Unauthorized(t *testing.T) {
	mockAuthService := new(testutil.MockAuthService)
	mockAuthService.On("ValidateAccessToken", mock.Anything).Return(uint(0), service.ErrInvalidToken)

	router := testutil.SetupRouterWithMocks(nil, nil, mockAuthService, nil, nil)

	req, _ := http.NewRequest("GET", testutil.KnowledgeBaseListURL+"?organization_id=1", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
}
