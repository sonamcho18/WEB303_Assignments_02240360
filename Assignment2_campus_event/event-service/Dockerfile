# Multi-stage build for the Event Service (Service B).
# Build context is the repository root (see docker-compose.yml).

# ---- Stage 1: build a static Go binary ----
FROM golang:1.24-alpine AS build
WORKDIR /src

# Download dependencies first so this layer is cached between builds.
COPY go.mod go.sum ./
RUN go mod download

# Copy only the code this service needs.
COPY gen/ ./gen/
COPY internal/ ./internal/
COPY event-service/ ./event-service/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/event-service ./event-service

# ---- Stage 2: small runtime image (no Go toolchain, no source code) ----
FROM alpine:3.20
RUN adduser -D -H appuser
COPY --from=build /out/event-service /usr/local/bin/event-service
USER appuser
EXPOSE 50051
ENTRYPOINT ["/usr/local/bin/event-service"]
