# Multi-stage build for the Registration Service (Service A).
# Build context is the repository root (see docker-compose.yml).

# ---- Stage 1: build a static Go binary ----
FROM golang:1.24-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY gen/ ./gen/
COPY internal/ ./internal/
COPY registration-service/ ./registration-service/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/registration-service ./registration-service

# ---- Stage 2: small runtime image ----
FROM alpine:3.20
RUN adduser -D -H appuser
COPY --from=build /out/registration-service /usr/local/bin/registration-service
USER appuser
EXPOSE 50052
ENTRYPOINT ["/usr/local/bin/registration-service"]
