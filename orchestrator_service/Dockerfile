# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod files from both orchestrator and parent
COPY orchestrator_service/go.mod orchestrator_service/go.sum* ./orchestrator_service/
COPY go.mod go.sum ./

# Copy ingestion_service dependencies (needed for RSS feeds)
COPY ingestion_service ./ingestion_service

# Copy shared dependencies
COPY shared ./shared

# Download dependencies
WORKDIR /app/orchestrator_service
RUN go mod download

# Copy orchestrator source
COPY orchestrator_service ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /orchestrator main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /orchestrator .

# Expose ports
EXPOSE 8081 9999

# Run the orchestrator
CMD ["./orchestrator"]
