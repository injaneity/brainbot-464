# Creation Service Dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install ffmpeg for video processing
RUN apk add --no-cache ffmpeg

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the creation service
WORKDIR /app/creation_service
RUN CGO_ENABLED=0 GOOS=linux go build -o creation-service main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates ffmpeg fontconfig ttf-dejavu

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/creation_service/creation-service .

# Copy background videos directory
COPY --from=builder /app/creation_service/backgroundvids ./backgroundvids

# Create necessary directories
RUN mkdir -p inputs outputs

EXPOSE 8081

# Health check for API mode
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

# Default to Kafka consumer mode
CMD ["./creation-service", "-kafka"]
