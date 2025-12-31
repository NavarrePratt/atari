# Multi-stage build for atari
FROM golang:1.25.4 AS builder
ARG TARGETOS TARGETARCH
ARG VERSION=dev
WORKDIR /app

# Compile with module + build cache mounts
RUN \
  --mount=type=cache,target=/root/.cache \
  --mount=type=cache,target=/go/pkg/mod \
  --mount=type=bind,source=./go.mod,target=/app/go.mod \
  --mount=type=bind,source=./go.sum,target=/app/go.sum \
  --mount=type=bind,source=./internal,target=/app/internal \
  --mount=type=bind,source=./cmd,target=/app/cmd \
  <<'EOF'
  go mod download -x
  go test -v ./...
  CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -X main.version=${VERSION}" \
    -o /go/bin/atari \
    ./cmd/atari
EOF

# Runtime image
FROM gcr.io/distroless/static:nonroot
COPY --link --from=builder /go/bin/atari /usr/local/bin/atari
ENTRYPOINT ["/usr/local/bin/atari"]
