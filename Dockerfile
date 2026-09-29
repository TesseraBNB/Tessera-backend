# syntax=docker/dockerfile:1
# Tessera backend — Go API service for Railway.
# The frontend is built and hosted separately on Vercel.

# --- Build ---
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/tessera ./cmd/tessera/

# --- Runtime ---
FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 tessera
WORKDIR /app
COPY --from=build /out/tessera /app/tessera
RUN mkdir -p /app/reports && chown -R tessera:tessera /app
USER tessera

ENV PORT=8080
EXPOSE 8080

# Railway also health-checks /api/health (see railway.toml); this covers plain Docker.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${PORT}/api/health" >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/tessera"]
CMD ["serve"]
