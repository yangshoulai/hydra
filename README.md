# Hydra - 高可用大模型聚合网关

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://go.dev/)
[![Vue Version](https://img.shields.io/badge/vue-3.x-green)](https://vuejs.org/)

Hydra 是一个企业级大模型 API 聚合网关，提供多渠道自动切换、细粒度熔断保护、假 200 识别等高可用特性，支持 OpenAI / Anthropic / Google Gemini 接口。

## 核心特性

- 🔄 **多渠道自动切换**: 按优先级和权重自动路由到可用渠道
- ⚡ **双层熔断保护**: Key 级别 + 模型配置级别熔断，自动冷却恢复
- 🔍 **假 200 响应识别**: 支持流式/非流式错误嗅探并自动重试
- 🎯 **模型名统一映射**: 统一模型名映射多个上游模型，自动负载均衡
- 🧩 **密钥分组**: Key 支持分组；模型配置可绑定多个分组，路由时只使用匹配分组的 Key
- 📝 **完整审计日志**: 主/明细日志、TraceID、耗时、Token 使用量
- 📊 **实时监控仪表盘**: QPS、成功率、渠道健康、Token 统计
- 🛠️ **低成本运维**: Web 管理界面，一键模型同步和测活
- 🐳 **开箱即用**: 单二进制 + Docker，支持 SQLite 和 PostgreSQL

## 界面预览

### 仪表盘

![仪表盘](images/仪表盘.png)

### 渠道管理

![渠道管理](images/渠道管理.png)

### 模型管理

![模型管理](images/模型管理.png)

### 厂商管理

![厂商管理](images/厂商管理.png)

### 访问令牌

![访问令牌](images/访问令牌.png)

### 日志查询

![日志查询](images/日志查询.png)

### 系统设置

![系统设置](images/系统设置.png)

## 快速开始

### 方式 1: Docker(推荐)

```bash
# 克隆仓库
git clone https://github.com/yangshoulai/hydra.git
cd hydra

# 复制配置文件
cp configs/config.example.yaml configs/config.yaml

# 编辑配置(按需调整 database.type / postgres_dsn / sqlite_path)
vim configs/config.yaml

# 构建镜像
docker build -t hydra:local -f deployments/Dockerfile .

# 运行（挂载配置和数据目录）
docker run -d --name hydra \
  -p 8080:8080 \
  -v "$(pwd)/configs/config.yaml:/app/configs/config.yaml" \
  -v "$(pwd)/data:/app/data" \
  hydra:local
```

服务启动后:
- API 端点: http://localhost:8080/v1
- 管理后台: http://localhost:8080
- 默认管理员账号: hydra / 123456

### 方式 2: 本地开发

**前置要求:**
- Go 1.25+
- Node.js 20+
- pnpm (前端包管理器)
- SQLite 3 或 PostgreSQL 12+
- Vue DevTools (浏览器扩展，强烈推荐)

**后端:**
```bash
cd backend
go mod download
go run cmd/hydra/main.go -config ../configs/config.yaml
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
pnpm install
pnpm run build

# 构建后端(嵌入静态文件)
cd ../backend
cp -r ../frontend/dist ./static
go build -o hydra cmd/hydra/main.go

# 运行
./hydra -config ../configs/config.yaml
```

## 配置说明

最小配置示例:

```yaml
server:
  port: 8080

database:
  type: sqlite
  sqlite_path: ./data/hydra.db

log:
  level: info
  file:
    enabled: true
    path: ./data/logs/hydra.log
```

完整配置请参考 [configs/config.example.yaml](configs/config.example.yaml)，示例默认使用 PostgreSQL，可按需切换为 SQLite。

系统运行参数（熔断阈值、重试次数、请求超时、日志保留天数、错误关键词等）在管理后台的「系统设置」中配置，并支持热更新。

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
- 选择密钥分组（默认 `Default`；一个 Key 只能属于一个分组）
- 点击"立即测活"验证可用性

### 3. 同步模型

在渠道详情页 -> 同步模型:
- 系统自动调用上游 `/v1/models` 获取模型列表
- 若渠道存在多个密钥分组，系统会分别用每个分组内的一个 Key 查询上游模型列表并合并去重
- 查看 Diff 视图,配置模型映射关系
- 设置统一模型名(如 `gpt-4` 映射到上游 `gpt-4-turbo`)

### 4. 生成访问令牌

设置管理 -> 访问令牌 -> 创建令牌:
- 输入令牌名称
- 复制生成的 Token(仅显示一次)

### 5. 调用 API

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  # 或使用 X-Api-Key
  # -H "X-Api-Key: YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "stream": false
  }'
```

支持的核心端点:
- `POST /v1/chat/completions` (OpenAI Chat Completions)
- `POST /v1/responses` (OpenAI Responses)
- `POST /v1/messages` (Anthropic Messages)
- `GET /v1/models` (可用模型列表)
- `POST /v1beta/models/{model}:generateContent` (Google Gemini Generate Content)
- `POST /v1beta/models/{model}:streamGenerateContent` (Google Gemini Stream Generate Content)
- `GET /v1beta/models` (Gemini 可用模型列表，Gemini API 结构)

### 6. 查看日志

管理后台 -> 日志查询:
- 按 TraceID、模型名、状态码筛选
- 查看完整请求元数据和错误信息
- 支持时间范围查询与 Token 使用量查看

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
├── data/                 # 运行时数据（日志、SQLite）
├── deployments/          # 部署文件
│   └── Dockerfile
└── QUICKSTART.md         # 快速启动文档
```

## 技术栈

- **后端**: Go 1.25+, Gin, GORM, Slog, Viper
- **前端**: Vue 3, Naive UI, TypeScript, Vite, Pinia
- **数据库**: SQLite(默认) / PostgreSQL
- **部署**: Docker

## 运行与构建

本项目推荐使用 Makefile：

```bash
# 安装依赖
make install-deps

# 开发模式（前后端并行）
make dev

# 完整构建（前端静态文件嵌入后端）
make build

# 运行（使用 configs/config.example.yaml 作为默认配置路径）
make run
```

说明：
- 后端启动参数为 `-config`（不是 `-c`）。
- `configs/config.example.yaml` 仅用于示例，生产环境建议复制为 `configs/config.yaml` 并自行管理。

## 文档

- [快速启动](QUICKSTART.md)

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
# 后端测试
cd backend
go test ./...
```

### 代码检查

```bash
# Go 代码检查
golangci-lint run
```

### 数据库迁移

系统启动时自动执行迁移。需要重置 SQLite 数据库时可删除 `data/hydra.db` 后重启服务。

## 常见问题

**Q: 如何备份数据?**

SQLite 模式: 直接复制 `data/hydra.db` 文件
PostgreSQL 模式: 使用 `pg_dump` 工具

**Q: 如何升级系统?**

```bash
# 拉取新版本
git pull

# 重新构建并重启（示例）
docker stop hydra && docker rm hydra
docker build -t hydra:local -f deployments/Dockerfile .
docker run -d --name hydra \
  -p 8080:8080 \
  -v "$(pwd)/configs/config.yaml:/app/configs/config.yaml" \
  -v "$(pwd)/data:/app/data" \
  hydra:local
```

**Q: 日志占用空间过大?**

系统会自动清理过期日志（默认保留 30 天）。可在管理后台「系统设置」中调整日志保留天数和调试日志开关。

**Q: 如何重置管理员密码?**

在管理后台使用「修改密码」功能（需要登录）。如需离线重置，可通过数据库直接更新 `admin_users.password_hash`（bcrypt）或重新初始化数据库（会丢失数据）。

## 许可证

本项目采用 [MIT License](LICENSE)。

## 贡献

欢迎提交 Issue 和 Pull Request!

## 联系方式

- GitHub: https://github.com/yangshoulai/hydra
- Email: shoulai.yang@gmail.com

---

**⚠️ 注意事项:**
- 生产环境请务必修改默认管理员密码
- 建议启用 HTTPS 并设置反向代理
- PostgreSQL 模式推荐用于高并发场景
