# Build stage
# Compile on the build host and cross-compile for the target, so arm64 images
# don't run the Go toolchain under emulation. The Debian-based image ships
# tzdata and CA certificates, which the runtime stage copies from here.
FROM --platform=$BUILDPLATFORM golang:1.26-trixie AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /pve-mcp ./cmd/pve-mcp

# Final runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /pve-mcp /pve-mcp

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/pve-mcp"]
