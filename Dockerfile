# Build stage
FROM golang:alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /pve-mcp ./cmd/pve-mcp

# Final runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /pve-mcp /pve-mcp

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/pve-mcp"]
