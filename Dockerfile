# Build stage — CGO required by onepassword-sdk-go (wraps native Rust library)
FROM golang:1.24 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go test ./internal/... && \
    CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o herald ./cmd/herald/ && \
    CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o herald-agent ./cmd/herald-agent/

# Runtime stage — distroless/base includes glibc needed for CGO binaries
FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=builder /build/herald /herald
COPY --from=builder /build/herald-agent /herald-agent

EXPOSE 8765

ENTRYPOINT ["/herald"]
