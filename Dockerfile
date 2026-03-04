# ── Stage 1: Build ───────────────────────────────────────────────
# golang:alpine tracks the latest stable Go on Alpine Linux (apk available)
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

# Expose port
EXPOSE 7777

CMD ["./server"]
