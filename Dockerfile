# Web build stage - build the React app (served as static files by the API)
FROM node:22.23.2-alpine AS webbuilder

WORKDIR /web

# Build arg for private @cloistr registry auth
ARG NPM_AEGIS_TOKEN

# Install dependencies from lockfile first for layer caching
COPY frontend/web/package.json frontend/web/package-lock.json frontend/web/.npmrc ./
# --legacy-peer-deps: nostr-tools (via @cloistr/auth) declares peerOptional
# typescript>=5, but this CRA app pins typescript 4.9.5 (react-scripts). The
# peer is optional, so ignore the conflict rather than break the CRA toolchain.
RUN NPM_AEGIS_TOKEN=${NPM_AEGIS_TOKEN} npm ci --legacy-peer-deps

# Build the production bundle. INLINE_RUNTIME_CHUNK=false keeps all JS in
# external files so the strict default-src 'self' CSP is not violated.
COPY frontend/web/ ./
RUN CI=false INLINE_RUNTIME_CHUNK=false npm run build

# Build stage
FROM golang:1.25-alpine AS builder

# Set working directory
WORKDIR /app

# Install git (needed for some Go modules)
RUN apk add --no-cache git

# Copy go mod and sum files
COPY backend/go.mod backend/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY backend/ ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server

# Final stage
FROM alpine:latest

# Install CA certificates for HTTPS and curl for healthcheck
RUN apk --no-cache add ca-certificates curl

# Create app directory and user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/main .
COPY --from=builder /app/migrations ./migrations

# Copy the built web UI (served as static files by the API at /)
COPY --from=webbuilder /web/build ./web

# Change ownership to app user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 7700

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:7700/api/v1/health || exit 1

# Run the application
CMD ["./main"]