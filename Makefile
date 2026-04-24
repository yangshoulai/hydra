.PHONY: help build run test clean clean-data clean-all install-deps dev docker-build docker-run

# 变量定义
APP_NAME=hydra
BUILD_DIR=./bin
CMD_DIR=./cmd/hydra
VERSION_FILE=./VERSION
VERSION?=$(shell VERSION="$$(tr -d '[:space:]' < $(VERSION_FILE) 2>/dev/null)"; if [ -n "$$VERSION" ]; then echo "$$VERSION"; else git describe --tags --always --dirty 2>/dev/null || echo "dev"; fi)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"
DOCKER_PLATFORM?=linux/amd64
DOCKER_OUTPUT?=--load

# 默认目标
help: ## 显示帮助信息
	@echo "Hydra API Gateway - Makefile 命令:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

install-deps: ## 安装依赖
	@echo "==> Installing dependencies..."
	cd backend && go mod download
	cd backend && go mod tidy
	cd frontend && pnpm install

build-frontend: ## 构建前端
	@echo "==> Building frontend..."
	@echo "==> Frontend app version: $(VERSION)"
	cd frontend && APP_VERSION=$(VERSION) pnpm exec vue-tsc -b && APP_VERSION=$(VERSION) pnpm exec vite build
	@mkdir -p backend/static
	@cp -r frontend/dist/* backend/static/
	@echo "==> Frontend build complete"

build-backend: ## 编译后端
	@echo "==> Building backend..."
	@mkdir -p $(BUILD_DIR)
	cd backend && go build $(LDFLAGS) -o ../$(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)
	@echo "==> Backend build complete: $(BUILD_DIR)/$(APP_NAME)"

build: build-frontend build-backend ## 完整构建（前端+后端）

dev-frontend: ## 开发模式运行前端
	@echo "==> Starting frontend in development mode..."
	cd frontend && pnpm run dev

dev-backend: ## 开发模式运行后端
	@echo "==> Running backend in development mode..."
	cd backend && go run $(LDFLAGS) $(CMD_DIR) --data-dir ../data

dev: ## 同时运行前后端开发模式
	@echo "==> Starting development mode (frontend & backend)..."
	@make -j2 dev-frontend dev-backend

run: build ## 编译并运行
	@echo "==> Running $(APP_NAME)..."
	$(BUILD_DIR)/$(APP_NAME) --data-dir ./data

test: ## 运行测试
	@echo "==> Running tests..."
	cd backend && go test -v -race -cover ./...

test-integration: ## 运行集成测试
	@echo "==> Running integration tests..."
	cd backend && go test -v -tags=integration ./tests/integration/...

clean: ## 清理构建产物
	@echo "==> Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf backend/static
	@echo "==> Clean complete"

clean-data: ## 清理运行时数据（数据库、日志等）
	@echo "==> Cleaning runtime data..."
	rm -rf data/*
	@echo "==> Runtime data cleaned"

clean-all: clean clean-data ## 清理所有（构建产物+运行时数据）

fmt: ## 格式化代码
	@echo "==> Formatting code..."
	cd backend && go fmt ./...

lint: ## 运行 linter
	@echo "==> Running linter..."
	cd backend && golangci-lint run ./...

docker-build: ## 使用 buildx 构建 Linux Docker 镜像（默认 linux/amd64）
	@echo "==> Building Docker image..."
	@echo "==> Version: $(VERSION)"
	@echo "==> Platform: $(DOCKER_PLATFORM)"
	docker buildx build --platform $(DOCKER_PLATFORM) $(DOCKER_OUTPUT) --build-arg VERSION=$(VERSION) -t $(APP_NAME):$(VERSION) -f deployments/Dockerfile .

docker-run: ## 运行 Docker 容器
	@echo "==> Running Docker container..."
	docker run -d --name $(APP_NAME) \
		-p 8080:8080 \
		-v $(PWD)/data:/app/data \
		$(APP_NAME):$(VERSION)

docker-stop: ## 停止 Docker 容器
	@echo "==> Stopping Docker container..."
	-docker stop $(APP_NAME)
	-docker rm $(APP_NAME)

logs: ## 查看日志
	@tail -f data/logs/hydra.log

init-db: ## 初始化数据库
	@echo "==> Initializing database..."
	@rm -f data/hydra.db
	@echo "==> Database will be initialized on first run"

.DEFAULT_GOAL := help
