# Stage 1: Build Web UI
FROM node:20-alpine AS web-builder
WORKDIR /app
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Stage 2: Build Go Binaries
FROM golang:1.24-bookworm AS builder
WORKDIR /app

# Install libbpf dependencies for eBPF (if needed for CGO)
RUN apt-get update && apt-get install -y clang llvm libbpf-dev

COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy web assets from previous stage
# The Go build expects assets in pkg/dashboard/dist
COPY --from=web-builder /app/dist pkg/dashboard/dist

# Build binaries with CGO enabled for eBPF support if needed, or disable if pure Go
# Using CGO_ENABLED=0 for portability unless eBPF requires CGO (cilium/ebpf is pure Go usually)
# However, if we link against libbpf, we need CGO.
# Given our current eBPF implemention uses cilium/ebpf/link, it might rely on kernel headers.
# Let's try static build first.
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

# Build
RUN go build -ldflags="-w -s" -o bin/cloudvault cmd/cli/main.go
RUN go build -ldflags="-w -s" -o bin/cloudvault-agent cmd/agent/main.go

# Stage 3: Runtime Image
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /

COPY --from=builder /app/bin/cloudvault /usr/local/bin/cloudvault
COPY --from=builder /app/bin/cloudvault-agent /usr/local/bin/cloudvault-agent

# Expose Dashboard port
EXPOSE 8080

# Default entrypoint (can be overridden by k8s command)
ENTRYPOINT ["/usr/local/bin/cloudvault"]
CMD ["dashboard"]
