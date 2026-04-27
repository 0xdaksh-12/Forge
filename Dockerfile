FROM golang:1.23-alpine AS go-builder
RUN apk add --no-cache git ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build the frontend first so it gets embedded.
RUN apk add --no-cache nodejs npm && npm install -g pnpm
RUN cd web && pnpm install && pnpm build
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
