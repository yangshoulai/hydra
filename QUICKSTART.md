# Hydra API Gateway - 快速启动指南

## 项目概览

Hydra（九头蛇）是一个高可用的大模型聚合网关，实现细粒度的故障隔离、智能响应清洗和自动重试机制。

## 核心特性

- ✅ **细粒度熔断**: Key 级别 + 模型配置级别独立维护健康状态
- ✅ **密钥分组**: Key 可按分组管理，模型配置绑定分组路由
- ✅ **智能清洗**: 识别流式/非流式“假 200”响应并自动重试
- ✅ **负载均衡**: 优先级 + 权重 + Key 轮询的多渠道流量分配
- ✅ **自动重试**: 透明的故障转移，对用户无感知
- ✅ **完整日志**: 主/明细审计日志 + Token 使用量统计
- ✅ **实时监控**: QPS、成功率、渠道健康与 Token 统计

## 前置要求

- Go 1.25+
- Node.js 20+
- pnpm
- SQLite 3（或 PostgreSQL）

## 快速启动

### 1. 安装依赖

```bash
make install-deps
```

### 2. 配置文件

复制示例配置：

```bash
cp configs/config.example.yaml configs/config.yaml
```

编辑 `configs/config.yaml` 根据需要调整配置（示例默认 PostgreSQL，可切换为 SQLite）。

### 3. 启动服务

```bash
# 开发模式（默认使用 configs/config.example.yaml）
make dev

# 或使用自定义配置运行
cd backend
go run cmd/hydra/main.go -config ../configs/config.yaml

# 或者编译后运行
make run
```

服务将在 `http://localhost:8080` 启动。

### 4. 验证服务

```bash
# 健康检查
curl http://localhost:8080/health

# 预期响应
{
  "status": "ok",
  "version": "1.0.0"
}
```

## 默认账户

- **管理员账户**: hydra / 123456

## API 端点

### 代理 API（需要 Access Token）

- `POST /v1/chat/completions` - OpenAI Chat Completions
- `POST /v1/responses` - OpenAI Responses
- `POST /v1/messages` - Anthropic Messages
- `GET /v1/models` - 获取可用模型列表
- `POST /v1beta/models/{model}:generateContent` - Google Gemini Generate Content（路径保持 Gemini 官方格式）
- `POST /v1beta/models/{model}:streamGenerateContent` - Google Gemini Stream Generate Content
- `GET /v1beta/models` - 获取 Gemini 可用模型列表（Gemini API 结构）

### 使用示例

```bash
# 设置 Access Token（需要先在管理后台创建）
export ACCESS_TOKEN="your-access-token"

# 调用 Chat Completions
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  # 或使用 X-Api-Key
  # -H "X-Api-Key: $ACCESS_TOKEN" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "stream": false
  }'

# 获取模型列表
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer $ACCESS_TOKEN"

# 获取 Gemini 模型列表（Gemini API 结构）
curl http://localhost:8080/v1beta/models \
  -H "Authorization: Bearer $ACCESS_TOKEN"

# 调用 Gemini Generate Content（注意：model 在 URL 中）
curl -X POST "http://localhost:8080/v1beta/models/gemini-1.5-pro:generateContent" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [
          {"text": "Hello Gemini!"}
        ]
      }
    ]
  }'
```

## 项目结构

```
hydra/
├── backend/
│   ├── cmd/hydra/           # 主程序入口
│   ├── internal/
│   │   ├── api/             # API 处理器
│   │   ├── config/          # 配置管理
│   │   ├── middleware/      # Gin 中间件
│   │   ├── models/          # 数据模型
│   │   ├── repository/      # 数据访问层
│   │   ├── service/         # 业务逻辑
│   │   │   ├── circuit/     # 熔断器
│   │   │   ├── logger/      # 日志系统
│   │   │   ├── proxy/       # 代理服务
│   │   │   └── sniffer/     # 响应嗅探
│   │   └── migration/       # 数据库迁移
│   └── tests/               # 测试
├── frontend/                # Vue 3 前端
├── configs/                 # 配置文件
├── data/                    # 运行时数据（日志、SQLite）
├── deployments/             # 部署文件
│   └── Dockerfile
├── Makefile                 # 构建脚本
└── QUICKSTART.md            # 快速启动文档
```

## 主要概念

### 熔断器（Circuit Breaker）

- **Key 级别**: 认证/额度/限流问题仅影响当前 Key
- **模型配置级别**: 404/5xx/网络异常只熔断对应模型配置，不影响同渠道其他模型
- **自动恢复**: 冷却时间到期后自动重新参与路由

### 响应嗅探器（Response Sniffer）

自动识别"假 200"响应：

- JSON 响应包含 `error` 字段
- HTML 响应（如错误页面）
- 明文错误消息（"无可用后端"、"额度不足"等）

识别后视为软故障，自动重试其他渠道。

### 负载均衡

1. **优先级**: 先选择高优先级的渠道
2. **权重**: 同优先级内按权重分配流量
3. **轮询**: 渠道内的 Key 使用轮询策略

### 密钥分组（Key Group）

- **Key 分组**: 每个 Key 必须属于一个分组（默认 `Default`）
- **模型绑定分组**: 模型配置可选择多个分组，路由时仅使用匹配分组的 Key
- **解决的问题**: 上游可能存在“不同 Key 可访问不同模型”的情况，分组可避免为每个分组重复创建渠道

### 日志系统

- **审计日志**: 主/明细日志写入数据库，记录 TraceID、模型、耗时、Token 使用量
- **调试日志**: 写入文件，记录完整 Request/Response Body（可配置）
- **日志轮转**: 自动切割和压缩日志文件

## 配置说明

Hydra 同时支持「配置文件」和「系统设置」两类配置来源：

- 配置文件：主要包含 server / database / log 等基础启动项
- 系统设置：运行时参数（熔断阈值、冷却时长、探测间隔、重试次数、请求超时、错误关键词、日志保留天数等），可在管理后台配置并热更新

最小配置示例：

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

## 数据库

### SQLite（默认）

```yaml
database:
  type: sqlite
  sqlite_path: ./data/hydra.db
```

### PostgreSQL

```yaml
database:
  type: postgres
  postgres_dsn: "host=localhost user=hydra password=hydra dbname=hydra port=5432 sslmode=disable"
```

### 数据库表

- `channels` - 渠道配置
- `keys` - API Key 配置
- `channel_model_configs` - 模型映射配置
- `request_logs_main` - 请求主日志
- `request_logs_detail` - 请求明细日志
- `providers` - 厂商配置
- `models` - 模型配置
- `system_settings` - 系统设置
- `access_tokens` - 访问令牌
- `admin_users` - 管理员用户

## 开发指南

### 编译

```bash
make build
```

### 运行测试

```bash
make test
```

### 格式化代码

```bash
make fmt
```

### 清理构建产物

```bash
make clean
```

## 故障排查

### 查看日志

```bash
# 查看实时日志
make logs

# 或直接查看文件
tail -f data/logs/hydra.log
```

### 检查数据库

```bash
# SQLite
sqlite3 data/hydra.db "SELECT * FROM channels;"
sqlite3 data/hydra.db "SELECT * FROM keys;"
```

### 查看熔断器状态

查看日志中的熔断器状态变化：

```bash
grep "circuit breaker" data/logs/hydra.log
```

## 生产部署

### 安全建议

1. **修改默认密码**: 首次登录后立即修改 admin 密码
2. **启用 HTTPS**: 使用反向代理（Nginx/Caddy）
3. **限制访问**: 配置防火墙规则
4. **备份数据库**: 定期备份 data/hydra.db

### Docker 部署

```bash
# 构建镜像
docker build -t hydra:local -f deployments/Dockerfile .

# 运行（挂载配置和数据目录）
docker run -d --name hydra \
  -p 8080:8080 \
  -v "$(pwd)/configs/config.yaml:/app/configs/config.yaml" \
  -v "$(pwd)/data:/app/data" \
  hydra:local

# 停止容器
docker rm -f hydra
```

## License

MIT License

## 支持

如有问题，请提交 Issue 或 Pull Request。
