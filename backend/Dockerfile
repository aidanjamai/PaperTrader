# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
# CGO_ENABLED=1 is required for sqlite
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main .

# Start a new stage from scratch
FROM alpine:latest  

RUN apk --no-cache add ca-certificates sqlite wget && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/main .

# Set ownership and permissions
RUN chown appuser:appuser /app/main && \
    chmod +x /app/main

# Switch to non-root user for security
USER appuser

# Expose port 8080 to the outside world
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Command to run the executable
CMD ["./main"]
