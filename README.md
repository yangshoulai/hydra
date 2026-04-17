# Hydra - 个人 AI 渠道中转站

Hydra 是一个面向个人使用的 AI 渠道中转网关，提供统一模型管理、渠道路由、密钥分组、熔断恢复与可视化管理后台。

## 功能特性

- **统一模型视图**：把多个上游渠道的不同模型名归并为同一个对外模型，客户端只关心统一模型名
- **多渠道负载均衡**：按优先级与健康状态挑选渠道与密钥，失败自动切换
- **密钥级熔断**：对密钥与模型配置分别做软/硬故障归因，冷却后自动恢复
- **响应嗅探**：识别假 200 / 空流式响应，触发重试并归因
- **管理后台**：渠道、密钥、模型、访问令牌、仪表盘全部在 Web UI 管理
- **SQLite 持久化**：单文件部署，零外部依赖
- **系统设置热更新**：日志、代理、嗅探等类别即时生效；涉及监听端口的设置触发 AppManager 优雅重启
- **模型测试**：一键测试模型可用性，自动覆盖流式与非流式两种模式
- **结构化日志**：slog + lumberjack 滚动，便于聚合分析

## 快速开始

### 本地运行

```bash
cd backend
go run ./cmd/hydra --data-dir ../data
```

启动后：

- 管理后台: <http://localhost:8080>
- 代理入口: <http://localhost:8080/v1>
- 默认管理员: `hydra / 123456`

### 前端开发

```bash
cd frontend
pnpm install
pnpm run dev
```

### 一键构建

```bash
make build
make run
```

## Docker 运行

```bash
docker build -t hydra:local -f deployments/Dockerfile .

docker run -d --name hydra \
  -p 8080:8080 \
  -v "$(pwd)/data:/app/data" \
  hydra:local
```

容器内的数据目录固定为 `/app/data`，挂载宿主机目录即可持久化。

## 数据目录

所有运行时数据都落在 `--data-dir` 指向的目录中。未指定时默认使用可执行文件同级的 `data/`。

```
<data-dir>/
├── hydra.db           # SQLite 数据库
└── logs/
    └── hydra.log      # 滚动日志文件
```

目录不存在会在启动时自动创建。可以用 `--data-dir /opt/hydra` 指向任意位置。

## 系统设置

所有可配置项保存在数据库的 `system_settings` 表：

- 首次启动写入默认值
- 在管理后台「系统设置」页修改并保存
- 日志、代理、嗅探等类别的设置即时生效
- 涉及监听端口等基础项的设置会触发 AppManager 优雅重启，期间允许短暂中断

## 日志

- **控制台**：结构化 slog 输出
- **文件**：`<data-dir>/logs/hydra.log`，通过 lumberjack 轮转（大小、保留天数、压缩等策略在系统设置中配置）
- 启用 `log_debug_enabled` 后，日志级别切换为 debug

## 核心端点

| 端点 | 协议 |
|---|---|
| `POST /v1/chat/completions` | OpenAI Chat Completions |
| `POST /v1/responses` | OpenAI Responses |
| `POST /v1/messages` | Anthropic Messages |
| `POST /v1beta/models/{model}:generateContent` | Google Gemini |
| `POST /v1beta/models/{model}:streamGenerateContent` | Google Gemini 流式 |
| `GET /v1/models` / `GET /v1beta/models` | 模型列表 |

## 开发命令

```bash
# 后端测试
cd backend && go test ./...

# 前端构建
cd frontend && pnpm run build
```
