package testutil

const (
	APIBaseURL             = "/api/v1"
	HealthCheckEndpoint    = APIBaseURL + "/health"
	LoginEndpoint          = APIBaseURL + "/auth/login"
	RegisterEndpoint       = APIBaseURL + "/auth/register"
	RefreshTokenEndpoint   = APIBaseURL + "/auth/refresh"
	LogoutEndpoint         = APIBaseURL + "/auth/logout"
	ChatEndpoint           = APIBaseURL + "/chat"
	UploadEndpoint         = APIBaseURL + "/upload"
	KnowledgeBaseListURL   = APIBaseURL + "/knowledge-base"
	KnowledgeBaseDeleteURL = APIBaseURL + "/knowledge-base/" // Append document ID dynamically
)
