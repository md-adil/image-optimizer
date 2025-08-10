# ---------------------------
# 1. Build Stage
# ---------------------------
FROM golang AS builder

# Install build tools and libvips dependencies
RUN apt update && apt install -y \
    libvips-dev libheif-dev libaom-dev

WORKDIR /app

# Copy go mod and download deps
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the Go binary
RUN go build -o image-loader ./cmd

# ---------------------------
# 2. Runtime Stage
# ---------------------------
FROM golang

RUN apt update && apt -y install libvips

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/image-loader .

# Expose port (change if needed)
EXPOSE 8080

# Run the app
CMD ["./image-loader"]