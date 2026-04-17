# Hydra - Multi-stage Dockerfile
# Stage 1: Frontend Build
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package*.json frontend/pnpm-lock.yaml ./
COPY frontend/.npmrc ./
RUN npm install -g pnpm && pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm run build

# Stage 2: Backend Build
FROM golang:1.25.6-alpine AS backend-builder

WORKDIR /app

# Install dependencies (gcc and musl-dev for CGO)
RUN apk add --no-cache gcc musl-dev

# Copy Go modules
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy backend source
COPY backend/ ./

# Copy frontend build output
COPY --from=frontend-builder /app/frontend/dist ./static

# Build Go binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o hydra ./cmd/hydra

# Stage 3: Production Image
FROM alpine:3.20

WORKDIR /app

# Install runtime dependencies and timezone data
RUN apk --no-cache add ca-certificates sqlite-libs tzdata wget && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

# Set timezone environment variable
ENV TZ=Asia/Shanghai

# Create non-root user (uid 1000 matches common host users for bind-mount friendliness)
RUN adduser -D -u 1000 hydra

# Copy binary and static files with correct ownership
COPY --from=backend-builder --chown=hydra:hydra /app/hydra ./
COPY --from=backend-builder --chown=hydra:hydra /app/static ./static

# Data directory; ensureDataDir 会在启动时自行创建，这里仅声明挂载点
VOLUME /app/data

USER hydra

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["./hydra", "--data-dir", "/app/data"]
