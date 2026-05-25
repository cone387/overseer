# ============================================================
# Stage 1: Build frontend (React/Vite)
# ============================================================
FROM node:20-alpine AS frontend

WORKDIR /build/web

COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY web/ ./
RUN npm run build

# ============================================================
# Stage 2: Build backend (Go)
# ============================================================
FROM golang:1.25-alpine AS backend

WORKDIR /build

# Install git for module downloads (some modules need it)
RUN apk add --no-cache git

# Download dependencies first (layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy frontend build output for go:embed
COPY --from=frontend /build/web/dist ./web/dist

# Build static binary (modernc.org/sqlite is pure Go, no CGO needed)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /out/overseer \
    ./cmd/overseer

# ============================================================
# Stage 3: Final minimal image
# ============================================================
FROM gcr.io/distroless/static-debian12

# Copy binary
COPY --from=backend /out/overseer /usr/local/bin/overseer

# Create data and config directories
# (distroless doesn't have mkdir, but COPY creates parent dirs)
WORKDIR /data

# Default environment
ENV OVERSEER_CONFIG_PATH=/etc/overseer/config.yaml

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/overseer"]
