# -----------------------------------------------------------------------------
# VARIABLES
# -----------------------------------------------------------------------------
GO_DIR := backend-go
PY_DIR := backend-python
PROTO_DIR := proto
UI_DIR := frontend-react

# Color Codes (for pretty output)
BOLD := \033[1m
RESET := \033[0m
GREEN := \033[32m
BLUE := \033[34m
YELLOW := \033[33m

# Go Binary Path
GOBIN := $(shell go env GOPATH)/bin

# -----------------------------------------------------------------------------
# HELP (Default Command)
# -----------------------------------------------------------------------------
.PHONY: help
help: ## Shows this help message
	@printf "$(BOLD)StudyAI - Management Console$(RESET)\n"
	@printf "Usage: make [command]\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(GREEN)%-20s$(RESET) %s\n", $$1, $$2}'

# -----------------------------------------------------------------------------
# PROTOCOL BUFFERS (Code Generation)
# -----------------------------------------------------------------------------
.PHONY: gen-proto
gen-proto: ## Generates Go and Python code from .proto files
	@printf "$(BLUE)➡️  Checking folder structure...$(RESET)\n"
	@mkdir -p $(GO_DIR)/pb
	@mkdir -p $(PY_DIR)/pb
	@touch $(PY_DIR)/pb/__init__.py
	
	@printf "$(BLUE)➡️  Generating Go code (Plugin Path: $(GOBIN))...$(RESET)\n"
	@protoc --plugin=protoc-gen-go=$(GOBIN)/protoc-gen-go \
		--plugin=protoc-gen-go-grpc=$(GOBIN)/protoc-gen-go-grpc \
		-I=$(PROTO_DIR) \
		--go_out=$(GO_DIR)/pb --go_opt=paths=source_relative \
		--go-grpc_out=$(GO_DIR)/pb --go-grpc_opt=paths=source_relative \
		rag_service.proto

	@printf "$(BLUE)➡️  Generating Python code (with Type Stub)...$(RESET)\n"
	@python3 -m grpc_tools.protoc \
		-I$(PROTO_DIR) \
		--python_out=$(PY_DIR)/pb \
		--grpc_python_out=$(PY_DIR)/pb \
		--mypy_out=$(PY_DIR)/pb \
		--mypy_grpc_out=$(PY_DIR)/pb \
		rag_service.proto

	@printf "$(YELLOW)🔧 Fixing Python imports (Patching)...$(RESET)\n"
	@sed -i 's/import rag_service_pb2/from . import rag_service_pb2/' $(PY_DIR)/pb/rag_service_pb2_grpc.py
	
	@printf "$(GREEN)✅ Code generation completed!$(RESET)\n"

# -----------------------------------------------------------------------------
# TESTING
# -----------------------------------------------------------------------------
.PHONY: test
test: test-py test-go ## Runs all tests (Python and Go)
	@printf "$(GREEN)✅ All tests completed successfully!$(RESET)\n"

.PHONY: test-py
test-py: ## Runs Python tests (pytest)
	@printf "$(BLUE)🐍 Running Python tests (pytest)...$(RESET)\n"
	@mkdir -p $(PY_DIR)/reports
	@cd $(PY_DIR) && uv run pytest -v --cov=app --cov-report=xml:reports/coverage.xml

.PHONY: test-go
test-go: ## Runs Go tests
	@printf "$(BLUE)🐹 Running Go tests...$(RESET)\n"
	@mkdir -p $(GO_DIR)/reports
	@cd $(GO_DIR) && go test ./... -v -count=1 -coverprofile=reports/coverage.txt -covermode=atomic

# -----------------------------------------------------------------------------
# DOCKER OPERATIONS
# -----------------------------------------------------------------------------
.PHONY: up
up: ## Starts the entire system with Docker Compose (in background)
	@printf "$(BLUE)🐳 Starting containers...$(RESET)\n"
	@docker-compose up -d --build
	@printf "$(GREEN)✅ System is running! For logs: 'make logs'$(RESET)\n"

.PHONY: down
down: ## Stops the entire system and removes containers
	@printf "$(YELLOW)🛑 Stopping system...$(RESET)\n"
	@docker-compose down

.PHONY: logs
logs: ## Watches logs from all services in real-time
	@docker-compose logs -f

.PHONY: restart
restart: down up ## Stops and restarts the system (Reset)

# -----------------------------------------------------------------------------
# DEVELOPMENT UTILS
# -----------------------------------------------------------------------------
.PHONY: tools
tools: ## Installs missing Go plugins to the correct path
	@printf "$(BLUE)🛠️  Installing protobuf tools to $(GOBIN)...$(RESET)\n"
	@mkdir -p $(GOBIN)
	@env GOBIN=$(GOBIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@env GOBIN=$(GOBIN) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@printf "$(GREEN)✅ Tools installed successfully!$(RESET)\n"

.PHONY: deps
deps: ## Updates/installs Go and Python dependencies
	@printf "$(BLUE)📦 Downloading Go modules...$(RESET)\n"
	@cd $(GO_DIR) && go mod tidy && go mod download
	@printf "$(BLUE)📦 Installing Python libraries...$(RESET)\n"
	@cd $(PY_DIR) && uv sync --dev
	@printf "$(GREEN)✅ Dependencies are ready.$(RESET)\n"

.PHONY: clean
clean: ## Cleans generated proto files and temporary files
	@printf "$(YELLOW)🧹 Cleaning up...$(RESET)\n"
	@rm -rf $(GO_DIR)/pb/*
	@rm -rf $(PY_DIR)/pb/*
	@rm -rf $(GO_DIR)/reports
	@rm -rf $(PY_DIR)/reports
	@printf "$(GREEN)✅ Squeaky clean.$(RESET)\n"