# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build a fully static binary — pure-Go drivers only, no cgo.
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Also build the ingest CLI so prod can seed / trigger the research corpus on
# demand. The in-process scheduler only fires on RESEARCH_INGEST_SCHEDULE
# (default monthly), so without this there is no manual path to populate it.
RUN CGO_ENABLED=0 GOOS=linux go build -o ingest ./cmd/ingest

# Start a new stage from scratch
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy the Pre-built binary files from the previous stage
COPY --from=builder /app/main .
COPY --from=builder /app/ingest .

# Set ownership and permissions
RUN chown appuser:appuser /app/main /app/ingest && \
    chmod +x /app/main /app/ingest

# Switch to non-root user for security
USER appuser

# Expose port 8080 to the outside world
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Command to run the executable
CMD ["./main"]
