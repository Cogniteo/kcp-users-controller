# Build the manager binary
FROM golang:1.24-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy go.mod and go.sum first for better layer caching
# This layer will be cached as long as go.mod/go.sum don't change
COPY go.mod go.sum ./
RUN go mod download

# Copy source code (this layer changes more often)
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/
COPY pkg/ pkg/

# Build with optimizations
# -ldflags="-w -s" removes debug info and symbol table for smaller binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -ldflags="-w -s" -a -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]