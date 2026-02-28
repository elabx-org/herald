# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o herald ./cmd/herald/ && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o herald-agent ./cmd/herald-agent/

# Runtime stage (distroless)
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /build/herald /herald
COPY --from=builder /build/herald-agent /herald-agent

EXPOSE 8765

ENTRYPOINT ["/herald"]
