# Quick Start: Hydra 高可用大模型聚合网关

**版本**: 1.0.0
**日期**: 2026-01-12
**目的**: 快速部署和使用 Hydra 系统

---

## 系统概览

Hydra 是一个高可用大模型聚合网关,提供:
- ✅ 多渠道自动切换与熔断保护
- ✅ 假 200 响应智能识别
- ✅ Key 池化管理与自动恢复
- ✅ 全链路请求追踪
- ✅ 清新简约的 Web 管理界面

### 系统架构

```
┌──────────┐     ┌─────────────────┐     ┌──────────────┐
│  Client  │────▶│  Hydra Proxy    │────▶│ Upstream API │
│          │◀────│  /v1/*          │◀────│ (OpenAI/...)│
└──────────┘     └─────────────────┘     └──────────────┘
                         │
                         ▼
                 ┌─────────────────┐
                 │  Admin UI       │
                 │  /admin/*       │
                 └─────────────────┘
                         │
                         ▼
                 ┌─────────────────┐
                 │  SQLite/PgSQL   │
                 └─────────────────┘
```

---

## 快速部署

### 方式一:单二进制部署(推荐)

#### 前置要求
- Go 1.21+ (仅构建时需要)
- Node.js 18+ 和 npm (仅构建前端时需要)

#### 步骤

1. **克隆仓库**
   ```bash
   git clone https://github.com/your-org/hydra.git
   cd hydra
   ```

2. **构建前端**
   ```bash
   cd frontend
   npm install
   npm run build  # 输出到 dist/
   cd ..
   ```

3. **构建后端(嵌入前端资源)**
   ```bash
   # 复制前端构建产物到后端
   cp -r frontend/dist backend/static

   # 构建 Go 二进制
   cd backend
   go build -o hydra cmd/hydra/main.go
   ```

4. **准备配置文件**
   ```bash
   mkdir -p /opt/hydra
   cp configs/config.example.yaml /opt/hydra/config.yaml
   ```

   编辑 `/opt/hydra/config.yaml`:
   ```yaml
   server:
     port: 8080

   database:
     type: sqlite
     sqlite_path: /opt/hydra/hydra.db

   log:
     level: info
     retention_days: 30
   ```

5. **启动服务**
   ```bash
   ./hydra --config /opt/hydra/config.yaml
   ```

   启动成功后,你将看到:
   ```
   [INFO] Hydra is starting...
   [INFO] Database initialized: /opt/hydra/hydra.db
   [INFO] Default admin user created: username=admin, password=<random>
   [WARN] Please change the default password after first login!
   [INFO] Proxy API listening on :8080/v1
   [INFO] Admin UI listening on :8080/admin
   ```

6. **访问管理界面**
   - 打开浏览器访问: `http://localhost:8080/admin`
   - 使用日志中的默认密码登录
   - 登录后立即修改密码

---

### 方式二:Docker 部署

#### 使用预构建镜像

```bash
docker run -d \
  --name hydra \
  -p 8080:8080 \
  -v /opt/hydra:/data \
  your-org/hydra:latest
```

#### 使用 Docker Compose

```yaml
# docker-compose.yml
version: '3.8'
services:
  hydra:
    image: your-org/hydra:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
      - ./config.yaml:/etc/hydra/config.yaml
    environment:
      - DATABASE_TYPE=sqlite
      - LOG_LEVEL=info
    restart: unless-stopped
```

启动:
```bash
docker-compose up -d
```

---

### 方式三:从源码构建 Docker 镜像

```bash
# 使用多阶段构建
docker build -t hydra:latest .
```

Dockerfile 示例:
```dockerfile
# 阶段一:构建前端
FROM node:18-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# 阶段二:构建后端
FROM golang:1.21-alpine AS backend-builder
WORKDIR /app
COPY backend/ ./backend/
COPY --from=frontend-builder /app/frontend/dist ./backend/static
WORKDIR /app/backend
RUN go build -o hydra cmd/hydra/main.go

# 阶段三:运行时镜像
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=backend-builder /app/backend/hydra .
COPY configs/config.example.yaml /etc/hydra/config.yaml
EXPOSE 8080
CMD ["./hydra", "--config", "/etc/hydra/config.yaml"]
```

---

## 初始配置

### 1. 创建第一个渠道

登录管理后台后,在"渠道管理"页面:

1. 点击"添加渠道"
2. 填写表单:
   - **渠道名称**: `OpenAI Official`
   - **Base URL**: `https://api.openai.com`
   - **优先级**: `100`(数值越小优先级越高)
   - **权重**: `100`(用于同优先级渠道的负载均衡)
3. 点击"保存"

### 2. 添加 API Key

在渠道详情页:

1. 点击"添加 Key"
2. 粘贴你的 OpenAI API Key(如 `sk-xxx`)
3. 点击"保存"

### 3. 同步模型列表

1. 点击渠道卡片上的"同步模型"按钮
2. 系统将调用上游 `/v1/models` 接口,展示差异比对表格:
   - 🟢 **新增模型**: 上游新上架的模型(如 `gpt-4-turbo-preview`)
   - 🔴 **缺失模型**: 本地已配置但上游已下架的模型
   - 🔵 **存量模型**: 两边都有的模型

3. 为新增模型配置统一名称:
   - 上游模型名: `gpt-4-turbo-preview`
   - 统一映射名: `gpt-4-turbo`(客户端请求时使用此名称)
   - 权重: `100`
   - 启用: ✅

4. 点击"应用变更"

### 4. 创建访问令牌

在"系统设置" → "访问令牌"页面:

1. 点击"创建令牌"
2. 输入令牌名称(如 `My App Token`)
3. 点击"创建"
4. **重要**:复制生成的令牌(如 `sk-hydra-abc123xyz`),此令牌仅显示一次!

---

## 使用代理接口

### 非流式请求

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-hydra-abc123xyz" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ],
    "temperature": 0.7,
    "max_tokens": 100
  }'
```

### 流式请求

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-hydra-abc123xyz" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Tell me a story"}
    ],
    "stream": true
  }'
```

### 查询可用模型

```bash
curl -X GET http://localhost:8080/v1/models \
  -H "Authorization: Bearer sk-hydra-abc123xyz"
```

响应示例:
```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4",
      "object": "model",
      "created": 1686935002,
      "owned_by": "system"
    },
    {
      "id": "gpt-3.5-turbo",
      "object": "model",
      "created": 1686935002,
      "owned_by": "system"
    }
  ]
}
```

---

## 问题排查

### 查看请求日志

在管理后台"日志查询"页面:

1. 输入 TraceID(从响应头 `X-Trace-ID` 获取)
2. 查看完整请求链路:
   - 使用的渠道和 Key
   - 请求耗时
   - 状态码和错误信息
   - 是否为假 200 响应

### 检查渠道健康状态

在"仪表盘"查看:

- **渠道健康墙**: 每个渠道的当前状态(正常/冷却/禁用)
- **Key 健康度**: 渠道的可用 Key 数量(如 7/10 表示 10 个 Key 中有 7 个可用)
- **QPS 波形图**: 过去 1 小时的请求量变化
- **今日成功率**: 成功请求占比

### 测活失效的 Key

1. 进入渠道详情页
2. 点击"立即测活"
3. 系统并发测试所有 Key,显示每个 Key 的健康状态和延迟
4. 对于 Dead 状态的 Key:
   - 如果是硬故障(如 401),检查 Key 是否过期,更换后点击"重置"
   - 如果是软故障(如 5xx),等待冷却期结束后自动恢复

---

## 高级配置

### 调整熔断策略

在"系统设置"页面修改:

```yaml
circuit_breaker:
  failure_threshold: 3       # 连续失败 N 次后进入熔断
  cooling_duration_sec: 60   # 冷却时长(秒)
  max_retry: 3               # 单个请求最大重试次数
```

### 配置日志保留策略

```yaml
log:
  retention_days: 30          # 审计日志保留天数
  debug_enabled: false        # 是否记录完整 Request/Response Body
```

### 自定义嗅探器错误关键词

在"系统设置" → "高级"中添加明文错误关键词:

```json
["无可用后端", "额度不足", "maintenance", "系统繁忙"]
```

---

## 性能优化建议

### SQLite 性能调优

对于 QPS <5000 的场景,SQLite 已足够。如需优化:

1. 启用 WAL 模式(配置文件中已默认启用)
2. 定期执行 VACUUM(自动清理任务会执行)
3. 确保数据库文件在 SSD 上

### 迁移到 PostgreSQL

当 QPS >5000 或需要分布式部署时:

1. 修改配置文件:
   ```yaml
   database:
     type: postgres
     postgres_dsn: "host=localhost user=hydra password=secret dbname=hydra port=5432 sslmode=disable"
   ```

2. 重启服务,系统会自动迁移 Schema

### 启用 HTTP/2

在反向代理(如 Nginx)中启用 HTTP/2:

```nginx
server {
    listen 443 ssl http2;
    server_name api.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

---

## 常见问题

### Q: 如何修改管理员密码?

A: 登录后在右上角用户菜单点击"修改密码"。

### Q: 为什么某个 Key 一直显示 Dead 状态?

A: Dead 状态表示硬故障(如 401/403),需要人工检查 Key 是否过期或被禁用。更换 Key 后点击"重置"恢复。

### Q: 如何查看调试日志(完整 Request/Response)?

A: 在"系统设置"中启用 `debug_enabled`,日志将写入 `/var/log/hydra/debug.log`,可通过 TraceID 关联审计日志。

### Q: 系统是否支持多实例部署?

A: MVP 阶段仅支持单机部署。如需水平扩展,可使用 PostgreSQL + 负载均衡器,但需注意熔断器状态同步(后续版本支持)。

### Q: 如何备份数据?

A: **SQLite**: 定期复制 `hydra.db` 文件
   **PostgreSQL**: 使用 `pg_dump` 导出

---

## 下一步

- 阅读 [API 文档](./contracts/) 了解完整接口规范
- 查看 [数据模型文档](./data-model.md) 了解数据库结构
- 参考 [技术研究文档](./research.md) 了解架构设计决策

---

## 支持

遇到问题?

- 📖 查看 [完整文档](https://docs.example.com)
- 🐛 提交 Issue: https://github.com/your-org/hydra/issues
- 💬 加入讨论: https://discord.gg/hydra
