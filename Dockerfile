# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy dependency definition files
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Compile optimized static Go binary for Linux CGO_ENABLED=0
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o inventory_app_online .

# Deployment stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy compiled binary from builder
COPY --from=builder /app/inventory_app_online /app/inventory_app_online

# Create directory for file uploads
RUN mkdir -p /app/uploads

# Expose HTTP service port 8080
EXPOSE 8080

# Environment defaults
ENV PORT=8080
ENV DATABASE_URL=""

# Execution command
ENTRYPOINT ["/app/inventory_app_online"]
