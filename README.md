# Hydra - 高可用大模型聚合网关

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue)](https://go.dev/)
[![Vue Version](https://img.shields.io/badge/vue-3.x-green)](https://vuejs.org/)

Hydra 是一个企业级大模型 API 聚合网关,提供多渠道自动切换、熔断保护、假 200 识别等高可用特性,支持 OpenAI 兼容接口。

## 核心特性

- 🔄 **多渠道自动切换**: 按优先级和权重自动路由请求到可用渠道
- ⚡ **双层熔断保护**: Key 级别和 Channel 级别熔断,自动探测恢复
- 🔍 **假 200 响应识别**: 智能识别错误响应并自动重试其他渠道
- 📊 **实时监控仪表盘**: QPS 波形图、成功率统计、渠道健康墙
- 🎯 **模型名统一映射**: 一个模型名对应多个上游模型,自动负载均衡
- 📝 **完整审计日志**: TraceID 追踪,完整请求链路记录
- 🛠️ **低成本运维**: Web 管理界面,一键模型同步和测活
- 🐳 **开箱即用**: 单二进制部署,支持 SQLite 和 PostgreSQL

## 快速开始

### 方式 1: Docker Compose(推荐)

```bash
# 克隆仓库
git clone https://github.com/yangshoulai/hydra.git
cd hydra

# 复制配置文件
cp configs/config.example.yaml configs/config.yaml

# 编辑配置(可选)
vim configs/config.yaml

# 启动服务(SQLite 模式)
cd deployments
docker-compose up -d

# 或启动 PostgreSQL 模式
docker-compose --profile postgres up -d
```

服务启动后:
- API 端点: http://localhost:8080
- 管理后台: http://localhost:8080/admin
- 默认管理员账号: admin / admin123

### 方式 2: 本地开发

**前置要求:**
- Go 1.21+
- Node.js 20+
- pnpm (前端包管理器)
- SQLite 3 或 PostgreSQL 12+
- Vue DevTools (浏览器扩展，强烈推荐)

**后端:**
```bash
cd backend
go mod download
go run cmd/hydra/main.go -c ../configs/config.yaml
```

**前端:**
```bash
cd frontend
pnpm install
pnpm run dev
```

### 方式 3: 手动构建

```bash
# 构建前端
cd frontend
npm install
npm run build

# 构建后端(嵌入静态文件)
cd ../backend
cp -r ../frontend/dist ./static
go build -o hydra cmd/hydra/main.go

# 运行
./hydra -c ../configs/config.yaml
```

## 配置说明

最小配置示例:

```yaml
server:
  port: 8080

database:
  type: sqlite
  sqlite_path: ./hydra.db

admin:
  session_secret: "your-secret-key-change-me"
```

完整配置请参考 [configs/config.example.yaml](configs/config.example.yaml)。

支持环境变量覆盖,格式为 `HYDRA_` 前缀:

```bash
export HYDRA_SERVER_PORT=9000
export HYDRA_DATABASE_TYPE=postgres
export HYDRA_DATABASE_POSTGRES_DSN="postgres://user:pass@localhost:5432/hydra"
```

## 使用示例

### 1. 添加渠道

登录管理后台 -> 渠道管理 -> 添加渠道:
- 渠道名称: OpenAI 官方
- Base URL: https://api.openai.com/v1
- 优先级: 100
- 权重: 50

### 2. 添加 Key

在渠道详情页 -> 添加 Key:
- 输入 API Key
- 点击"立即测活"验证可用性

### 3. 同步模型

在渠道详情页 -> 同步模型:
- 系统自动调用上游 `/v1/models` 获取模型列表
- 查看 Diff 视图,配置模型映射关系
- 设置统一模型名(如 `gpt-4` 映射到上游 `gpt-4-turbo`)

### 4. 生成访问令牌

设置管理 -> 访问令牌 -> 创建令牌:
- 输入备注信息
- 复制生成的 Token(仅显示一次)

### 5. 调用 API

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "stream": false
  }'
```

### 6. 查看日志

管理后台 -> 日志查询:
- 按 TraceID、模型名、状态码筛选
- 查看完整请求元数据和错误信息
- 支持时间范围查询

## 项目结构

```
.
├── backend/              # Go 后端
│   ├── cmd/hydra/        # 主程序入口
│   ├── internal/         # 内部包
│   │   ├── api/          # HTTP 处理器
│   │   ├── models/       # 数据模型
│   │   ├── repository/   # 数据访问层
│   │   ├── service/      # 业务逻辑
│   │   ├── middleware/   # 中间件
│   │   ├── config/       # 配置加载
│   │   └── migration/    # 数据库迁移
│   └── pkg/              # 公共工具
├── frontend/             # Vue 3 前端
│   ├── src/
│   │   ├── pages/        # 页面组件
│   │   ├── components/   # 通用组件
│   │   ├── services/     # API 客户端
│   │   ├── stores/       # Pinia 状态
│   │   └── router/       # 路由配置
├── configs/              # 配置文件
├── deployments/          # 部署文件
│   ├── Dockerfile
│   └── docker-compose.yml
└── specs/                # 需求文档
```

## 技术栈

- **后端**: Go 1.21+, Gin, GORM, Slog, Viper
- **前端**: Vue 3, Naive UI, TypeScript, Vite, Pinia
- **数据库**: SQLite(默认) / PostgreSQL
- **部署**: Docker, Docker Compose

## 文档

- [功能需求](specs/001-api-gateway-system/spec.md)
- [技术方案](specs/001-api-gateway-system/plan.md)
- [数据模型](specs/001-api-gateway-system/data-model.md)
- [API 契约](specs/001-api-gateway-system/contracts/)
- [部署指南](specs/001-api-gateway-system/quickstart.md)
- [任务清单](specs/001-api-gateway-system/tasks.md)

## 开发指南

### 开发工具推荐

**浏览器扩展 (必需):**
- **Vue DevTools** - Vue 3 调试工具
  - Chrome/Edge: [下载链接](https://chrome.google.com/webstore/detail/vuejs-devtools/)
  - Firefox: [下载链接](https://addons.mozilla.org/en-US/firefox/addon/vue-js-devtools/)
  - 功能: 组件调试、Pinia 状态查看、路由调试、性能分析

**VSCode 扩展 (推荐):**
- Go (Google)
- Vue - Official (Vue)
- TypeScript Vue Plugin (Vue)
- material-icon-theme (图标美化)

### 运行测试

```bash
# 单元测试
cd backend
go test ./...

# 集成测试
go test ./tests/integration/...

# 前端测试
cd frontend
npm run test
```

### 代码检查

```bash
# Go 代码检查
golangci-lint run

# 前端检查
npm run lint
```

### 数据库迁移

系统启动时自动执行迁移。手动迁移:

```bash
# 查看迁移状态
go run cmd/hydra/main.go -migrate-status

# 回滚迁移
go run cmd/hydra/main.go -migrate-rollback
```

## 常见问题

**Q: 如何备份数据?**

SQLite 模式: 直接复制 `hydra.db` 文件
PostgreSQL 模式: 使用 `pg_dump` 工具

**Q: 如何升级系统?**

```bash
# 停止服务
docker-compose down

# 拉取新版本
git pull

# 重新构建
docker-compose up -d --build
```

**Q: 日志占用空间过大?**

系统会自动清理过期日志(默认保留 30 天)。可在配置文件中调整:

```yaml
log:
  retention_days: 7  # 修改为 7 天
```

**Q: 如何重置管理员密码?**

```bash
# 进入容器
docker exec -it hydra sh

# 运行密码重置命令
./hydra -reset-admin-password
```

## 许可证

本项目采用 [MIT License](LICENSE)。

## 贡献

欢迎提交 Issue 和 Pull Request!

## 联系方式

- GitHub: https://github.com/yangshoulai/hydra
- Email: yangshoulai@example.com

---

**⚠️ 注意事项:**
- 生产环境请务必修改 `admin.session_secret`
- 建议启用 HTTPS 并设置 `admin.cookie_secure: true`
- PostgreSQL 模式推荐用于高并发场景
