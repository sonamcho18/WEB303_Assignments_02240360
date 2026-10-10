# Multi-stage build for the API Gateway.
# Build context is the repository root (see docker-compose.yml).

# ---- Stage 1: build a static Go binary ----
FROM golang:1.24-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY gen/ ./gen/
COPY internal/ ./internal/
COPY gateway/ ./gateway/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/gateway ./gateway

# ---- Stage 2: small runtime image ----
FROM alpine:3.20
RUN adduser -D -H appuser
COPY --from=build /out/gateway /usr/local/bin/gateway
USER appuser
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gateway"]
