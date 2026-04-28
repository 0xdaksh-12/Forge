FROM golang:1.24-alpine AS go-builder

# Install tools
RUN apk add --no-cache git ca-certificates nodejs npm
RUN npm install -g pnpm@10.32.1

WORKDIR /src

# 1. Build Frontend
COPY web/package.json web/pnpm-lock.yaml ./web/
RUN cd web && pnpm install --frozen-lockfile

COPY web/ ./web/
RUN cd web && pnpm build

# 2. Build Backend
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o /forge ./cmd/forge

# Runtime image 
FROM alpine:3.20
RUN apk add --no-cache ca-certificates git docker-cli
WORKDIR /app
COPY --from=go-builder /forge /app/forge
COPY --from=go-builder /src/web/dist /app/web/dist

ENV FORGE_PORT=8080 \
    FORGE_DB_PATH=/data/forge.db \
    FORGE_DATA_DIR=/data \
    FORGE_DOCKER_SOCKET=unix:///var/run/docker.sock

VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/app/forge"]
