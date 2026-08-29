# NetoDrive server
FROM golang:1.22-bookworm AS server-build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -o /out/netodrive ./cmd/netodrive

# Web UI
FROM node:22-bookworm AS web-build
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Runtime
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=server-build /out/netodrive /app/netodrive
COPY --from=web-build /web/dist /app/web/dist
ENV NETODRIVE_ADDR=:8080 \
    NETODRIVE_DATA=/data \
    NETODRIVE_DB=/data/netodrive.db \
    NETODRIVE_ADMIN_USER=admin \
    NETODRIVE_ADMIN_PASS=admin123
VOLUME ["/data"]
EXPOSE 8080
CMD ["/app/netodrive"]
