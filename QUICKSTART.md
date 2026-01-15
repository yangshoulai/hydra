# Hydra API Gateway - 快速启动指南

## 项目概览

Hydra（九头蛇）是一个高可用的大模型聚合网关，实现细粒度的故障隔离、智能响应清洗和自动重试机制。

## 核心特性

- ✅ **细粒度熔断**: Key 级别和 Channel 级别独立维护健康状态
- ✅ **智能清洗**: 识别并拦截"假 200"响应，自动重试
- ✅ **负载均衡**: 支持优先级和权重的多渠道流量分配
- ✅ **自动重试**: 透明的故障转移，对用户无感知
- ✅ **完整日志**: 审计日志（数据库）+ 调试日志（文件）
- ✅ **实时监控**: TraceID 全链路追踪

## 前置要求

- Go 1.21+
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

编辑 `configs/config.yaml` 根据需要调整配置。

### 3. 启动服务

```bash
# 开发模式（使用 go run）
make dev

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

- **管理员账户**: admin / admin123

## API 端点

### 代理 API（需要 Access Token）

- `POST /v1/chat/completions` - Chat Completions（兼容 OpenAI API）
- `GET /v1/models` - 获取可用模型列表

### 使用示例

```bash
# 设置 Access Token（需要先在管理后台创建）
export ACCESS_TOKEN="your-access-token"

# 调用 Chat Completions
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
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
├── configs/                 # 配置文件
├── logs/                    # 日志目录
└── Makefile                 # 构建脚本
```

## 主要概念

### 熔断器（Circuit Breaker）

- **Key 级别**: 单个 API Key 失败不影响同渠道其他 Key
- **Channel 级别**: 整个渠道的健康状态
- **状态转换**: Active → Cooling → Half-Open → Active
- **硬故障**: 401/403/429 quota exceeded → 永久禁用
- **软故障**: 5xx/timeout → 熔断后可恢复

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

### 日志系统

- **审计日志**: 写入数据库，记录请求元数据（TraceID、模型、耗时、状态）
- **调试日志**: 写入文件，记录完整 Request/Response Body（可配置）
- **日志轮转**: 自动切割和压缩日志文件

## 配置说明

### 熔断器配置

```yaml
circuit_breaker:
  failure_threshold: 3      # 连续失败次数阈值
  cooling_duration_sec: 60  # 冷却时长(秒)
  max_retry: 3              # 单个请求最大重试次数
```

### 日志配置

```yaml
log:
  level: info               # debug, info, warn, error
  retention_days: 30        # 审计日志保留天数
  debug_enabled: false      # 是否记录完整 Body
  file:
    enabled: true
    path: ./logs/hydra.log
    max_size: 100           # MB
    max_backups: 10
    compress: true
```

### 代理配置

```yaml
proxy:
  request_timeout: 60s      # 上游请求超时
  max_response_size: 10485760  # 10MB
  max_concurrent: 1000      # 最大并发请求数
```

## 数据库

### SQLite（默认）

```yaml
database:
  type: sqlite
  sqlite_path: ./hydra.db
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
- `request_logs` - 请求审计日志
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
tail -f logs/hydra.log
```

### 检查数据库

```bash
# SQLite
sqlite3 hydra.db "SELECT * FROM channels;"
sqlite3 hydra.db "SELECT * FROM keys;"
```

### 查看熔断器状态

查看日志中的熔断器状态变化：

```bash
grep "circuit breaker" logs/hydra.log
```

## 生产部署

### 安全建议

1. **修改默认密码**: 首次登录后立即修改 admin 密码
2. **修改 Session Secret**: 在配置文件中设置强随机字符串
3. **启用 HTTPS**: 使用反向代理（Nginx/Caddy）
4. **限制访问**: 配置防火墙规则
5. **备份数据库**: 定期备份 hydra.db

### Docker 部署

```bash
# 构建镜像
make docker-build

# 启动容器
make docker-run

# 停止容器
make docker-stop
```

## License

MIT License

## 支持

如有问题，请提交 Issue 或 Pull Request。
