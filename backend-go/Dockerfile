# --- Stage 1: Builder ---
FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pb/ ./pb/

# Critical Step: Build the Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o orchestrator ./cmd/server/main.go

# --- Stage 2: Runner ---
FROM alpine:latest

WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

RUN mkdir -p /tmp/studyai/uploads && \
    chown -R appuser:appgroup /tmp/studyai/uploads

COPY --from=builder /app/orchestrator .

RUN chown appuser:appgroup ./orchestrator

EXPOSE 8080

USER appuser

CMD ["./orchestrator"]