# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -mod=readonly -ldflags="-s -w -buildid=" -o sentinel ./cmd/sentinel

# Runtime stage
FROM ubuntu:22.04

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    git \
    systemd \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/sitekit

# Copy binary
COPY --from=builder /app/sentinel /opt/sitekit/sentinel

# Copy config example
COPY configs/agent.yaml.example /opt/sitekit/agent.yaml

# Environment variables
ENV SITEKIT_SAAS_URL=http://localhost
ENV SITEKIT_AGENT_TOKEN=test-token
ENV SITEKIT_SERVER_ID=test-server

# Run agent
CMD ["/opt/sitekit/sentinel"]
