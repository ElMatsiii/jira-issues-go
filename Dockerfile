# ==============================================================================
# Stage 1: Build static Go binary
# ==============================================================================
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /workspace

# Copy contracts and service source code
COPY jira-contracts /workspace/jira-contracts
COPY jira-issues-go /workspace/jira-issues-go

WORKDIR /workspace/jira-issues-go

# Download modules and build static binary
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /bin/jira-issues-go \
    ./cmd/server

# ==============================================================================
# Stage 2: Minimal production runtime
# ==============================================================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /bin/jira-issues-go /usr/local/bin/jira-issues-go
COPY --from=builder /workspace/jira-issues-go/db/migrations /app/db/migrations

RUN chown -R appuser:appgroup /app /usr/local/bin/jira-issues-go

USER appuser:appgroup

EXPOSE 50051

ENV DB_HOST=postgres-issues \
    DB_PORT=5432 \
    DB_USER=postgres \
    DB_PASSWORD=postgres \
    DB_NAME=issues_db \
    DB_SSLMODE=disable \
    KAFKA_BROKERS=kafka:9092 \
    KAFKA_TOPIC_ISSUES=issues.events \
    GRPC_PORT=50051 \
    MIGRATIONS_DIR=/app/db/migrations

ENTRYPOINT ["/usr/local/bin/jira-issues-go"]
