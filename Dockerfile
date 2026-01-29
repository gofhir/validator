# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gofhir-validator ./cmd/gofhir-validator/

# Final stage
FROM alpine:3.20

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN adduser -D -g '' validator

# Copy binary from builder
COPY --from=builder /gofhir-validator /usr/local/bin/gofhir-validator

# Switch to non-root user
USER validator

# Set working directory
WORKDIR /data

ENTRYPOINT ["/usr/local/bin/gofhir-validator"]
CMD ["--help"]
