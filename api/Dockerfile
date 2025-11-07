# syntax=docker/dockerfile:1

# --- Build stage ---
FROM golang:1.24-bookworm AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# --- Runtime stage (distroless, non-root) ---
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /out/api ./api

USER 65532:65532
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/api"]
