# Hydra - Multi-stage Dockerfile
# Stage 1: Frontend Build
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm install -g pnpm && pnpm install

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
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite-libs

# Copy binary and static files
COPY --from=backend-builder /app/hydra .
COPY --from=backend-builder /app/static ./static
COPY configs/config.example.yaml ./configs/config.yaml

# Create logs and data directories
RUN mkdir -p /app/data

VOLUME /app/data

# Expose port
EXPOSE 8080

# Run
CMD ["./hydra", "-config", "./configs/config.yaml"]
