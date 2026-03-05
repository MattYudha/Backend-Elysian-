# ── Stage 1: Build ───────────────────────────────────────────────
# golang:alpine is Alpine-based (apk available) and tracks latest stable Go
FROM golang:alpine AS builder

# Git is needed by some go modules
RUN apk add --no-cache git ca-certificates tzdata

# Allow Go to auto-download the required toolchain version from go.mod
ENV GOTOOLCHAIN=auto

WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server/main.go

# ── Stage 2: Run ─────────────────────────────────────────────────
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy compiled binary from builder stage
COPY --from=builder /app/server .

# Copy config directory so Viper can find config.yml at runtime
COPY --from=builder /app/config ./config

# Expose port (must match server.port in config.yml / PORT env var)
EXPOSE 8080

CMD ["./server"]
