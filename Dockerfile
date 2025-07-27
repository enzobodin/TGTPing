# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install git for go modules
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN adduser -D -s /bin/sh appuser

# Create data directory for JSON storage
RUN mkdir -p /data && chown appuser:appuser /data

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/main .

# Copy environment file
COPY .env .

# Change ownership to appuser
RUN chown appuser:appuser main .env

# Switch to non-root user
USER appuser

# Create volume for persistent data
VOLUME ["/data"]

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]
