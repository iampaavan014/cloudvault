# ─────────────────────────────────────────────────────────────────────────────
# Stage 1: Web UI builder (Node Alpine — smallest possible node image)
# ─────────────────────────────────────────────────────────────────────────────
FROM node:20-alpine AS web-builder
WORKDIR /app

# Layer-cache npm deps independently from source changes
COPY web/package*.json ./
RUN npm ci --prefer-offline

COPY web/ .
RUN npm run build

# ─────────────────────────────────────────────────────────────────────────────
# Stage 2: Go builder (Alpine-based — ~250MB vs ~1GB bookworm)
# cilium/ebpf is pure Go+CGO=0 friendly; bpf2go pre-generates .o files
# at source time so we do NOT need clang/llvm at image-build time.
# ─────────────────────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Only git is needed (some go modules use VCS stamping)
RUN apk add --no-cache git

# Cache go module downloads as a separate layer before copying source
COPY go.mod go.sum ./
RUN go mod download

# Copy everything (source, pre-built eBPF .o objects in pkg/ebpf/internal/)
COPY . .

# Inject pre-built UI assets from web-builder stage
# The Go embed directive (//go:embed dist/*) in server.go picks these up
COPY --from=web-builder /app/dist pkg/dashboard/dist

# Static, fully-portable binaries — no CGO, no libc dependency
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags="-w -s" -o bin/cloudvault     cmd/cli/main.go && \
    go build -trimpath -ldflags="-w -s" -o bin/cloudvault-agent cmd/agent/main.go

# ─────────────────────────────────────────────────────────────────────────────
# Stage 3: Runtime — distroless/static (no shell, no libc, ~2MB base)
# Perfect for CGO_ENABLED=0 static binaries
# ─────────────────────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

# Metadata
LABEL org.opencontainers.image.title="CloudVault Agent" \
    org.opencontainers.image.description="Multi-Cloud Kubernetes Storage Cost Intelligence" \
    org.opencontainers.image.source="https://github.com/cloudvault-io/cloudvault" \
    org.opencontainers.image.licenses="Apache-2.0"

COPY --from=builder /app/bin/cloudvault        /usr/local/bin/cloudvault
COPY --from=builder /app/bin/cloudvault-agent  /usr/local/bin/cloudvault-agent

# Dashboard port
EXPOSE 8080

# Default: run the agent (Helm chart overrides CMD per pod type)
ENTRYPOINT ["/usr/local/bin/cloudvault-agent"]
