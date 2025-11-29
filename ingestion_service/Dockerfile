# API Service Dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files from project root
COPY ../go.mod ../go.sum ./
RUN go mod download

# Copy entire project (needed for imports)
COPY .. .

# Build the ingestion server from ingestion_service directory
WORKDIR /app/ingestion_service
RUN CGO_ENABLED=0 GOOS=linux go build -o ingestion-server main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/ingestion_service/ingestion-server .

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

CMD ["./ingestion-server"]
