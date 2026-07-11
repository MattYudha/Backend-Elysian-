# ── Stage 1: Build ───────────────────────────────────────────────
# golang:alpine is Alpine-based (apk available) and tracks latest stable Go
FROM golang:alpine AS builder

# Git is needed by some go modules
RUN apk add --no-cache git ca-certificates tzdata

# Allow Go to auto-download the required toolchain version from go.mod
ENV GOTOOLCHAIN=auto

WORKDIR /app

# Copy source and build (sensitive files filtered by .dockerignore)
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o server ./cmd/server/main.go

# ── Stage 2: Run (Production Optimized) ──────────────────────────
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

# Security: Create a non-root system user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy compiled binary from builder stage
COPY --from=builder /app/server .

# Copy config directory so Viper can find config.yml at runtime
COPY --from=builder /app/config ./config

# Security: Ensure ownership is set to the non-root user
RUN chown -R appuser:appgroup /app

# Expose port (must match PORT env var set in Railway Variables)
EXPOSE 7777

# Security: Switch to the non-root user
USER appuser

CMD ["./server"]
