# syntax=docker/dockerfile:1

# --- Build stage ---
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder
WORKDIR /src

# Enable faster module caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Build static binary (no CGO)
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) \
    go build -o /out/brainbot .

# --- Runtime stage ---
FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/brainbot /app/brainbot

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/brainbot"]
