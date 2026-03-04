# ── Stage 1: Build ───────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

# Git is needed by some go modules
RUN apk add --no-cache git ca-certificates tzdata

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

# Expose port (Railway injects $PORT at runtime)
EXPOSE 7777

# Railway injects the PORT env variable; pass it through
CMD ["./server"]
