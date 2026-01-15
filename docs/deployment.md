# Hydra API Gateway 部署文档

## 目录

- [环境要求](#环境要求)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [部署方式](#部署方式)
- [生产环境建议](#生产环境建议)

## 环境要求

### 开发环境

- Go 1.21+
- Node.js 18+
- npm 或 pnpm

### 生产环境

- Go 1.21+ 运行时
- SQLite 或 PostgreSQL 数据库
- 至少 512MB 内存
- 至少 100MB 磁盘空间

## 快速开始

### 1. 克隆项目

```bash
git clone <repository-url>
cd hydra
```

### 2. 安装依赖

```bash
# 安装所有依赖（前端使用 pnpm，后端使用 go mod）
make install-deps

# 或分别安装
# 前端
cd frontend
pnpm install

# 后端
cd backend
go mod download
```

### 3. 配置系统

复制配置模板并修改：

```bash
cp configs/config.example.yaml configs/config.yaml
```

编辑 `configs/config.yaml`：

```yaml
server:
  port: 8080

database:
  type: sqlite  # 或 postgres
  sqlite_path: ./hydra.db
  # postgres_dsn: "host=localhost port=5432 user=hydra password=secret dbname=hydra sslmode=disable"

log:
  level: info
  file:
    enabled: true
    path: ./logs/hydra.log
```

### 4. 构建应用

```bash
# 构建前后端
make build

# 或分别构建
make frontend
make backend
```

### 5. 运行应用

```bash
# 运行生产构建
make run

# 或直接运行
./bin/hydra

# 指定配置文件
./bin/hydra -config /path/to/config.yaml
```

访问：
- API 文档：`http://localhost:8080/health`
- 管理后台：`http://localhost:8080/`
- 默认账号：`admin` / `admin123`

## 配置说明

### 数据库配置

#### SQLite（默认）

```yaml
database:
  type: sqlite
  sqlite_path: ./hydra.db
  max_open_conns: 1
  max_idle_conns: 1
```

#### PostgreSQL

```yaml
database:
  type: postgres
  postgres_dsn: "host=localhost port=5432 user=hydra password=secret dbname=hydra sslmode=disable"
  max_open_conns: 25
  max_idle_conns: 10
  conn_max_lifetime: 300s
```

### 代理配置

```yaml
proxy:
  request_timeout: 60s
  max_response_size: 10485760  # 10MB
  max_concurrent: 1000
```

### 熔断器配置

```yaml
circuit_breaker:
  failure_threshold: 3
  cooling_duration_sec: 60
  max_retry: 3
```

### 日志配置

```yaml
log:
  level: info  # debug, info, warn, error
  retention_days: 30
  debug_enabled: false
  file:
    enabled: true
    path: ./logs/hydra.log
    max_size: 100  # MB
    max_backups: 10
    max_age: 0  # 0 = 不删除旧日志
    compress: true
```

## 部署方式

### 方式一：二进制部署

```bash
# 1. 构建应用
make build

# 2. 部署到服务器
scp bin/hydra user@server:/opt/hydra/
scp configs/config.yaml user@server:/opt/hydra/configs/

# 3. 在服务器上运行
ssh user@server
cd /opt/hydra
./hydra -config configs/config.yaml
```

### 方式二：Docker 部署

#### 构建 Docker 镜像

```bash
make docker-build
```

#### 运行 Docker 容器

```bash
docker run -d \
  --name hydra \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/logs:/app/logs \
  --env-file configs/config.env \
  hydra-api-gateway:latest
```

#### Docker Compose

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  hydra:
    image: hydra-api-gateway:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
      - ./configs:/app/configs
    environment:
      - HYDRA_DATABASE_TYPE=postgres
      - HYDRA_DATABASE_POSTGRES_DSN=host=db:5432 user=hydra password=secret dbname=hydra sslmode=disable
    restart: unless-stopped
    depends_on:
      - db

  db:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: hydra
      POSTGRES_USER: hydra
      POSTGRES_PASSWORD: secret
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  postgres_data:
```

运行：

```bash
docker-compose up -d
```

### 方式三：Systemd 服务

创建 `/etc/systemd/system/hydra.service`：

```ini
[Unit]
Description=Hydra API Gateway
After=network.target

[Service]
Type=simple
User=hydra
WorkingDirectory=/opt/hydra
ExecStart=/opt/hydra/bin/hydra -config /opt/hydra/configs/config.yaml
Restart=always
RestartSec=5

# 资源限制
MemoryLimit=512M
CPUQuota=100%

# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/hydra/data /opt/hydra/logs

[Install]
WantedBy=multi-user.target
```

启用服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable hydra
sudo systemctl start hydra
sudo systemctl status hydra
```

### 方式四：Nginx 反向代理

配置 Nginx：

```nginx
upstream hydra {
    server 127.0.0.1:8080;
}

server {
    listen 80;
    server_name api.example.com;

    # API 代理
    location /v1/ {
        proxy_pass http://hydra;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # 超时设置
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # 管理后台
    location / {
        proxy_pass http://hydra;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    # 静态文件缓存
    location /static/ {
        proxy_pass http://hydra;
        proxy_cache_valid 200 7d;
        add_header Cache-Control "public, immutable";
    }
}
```

## 生产环境建议

### 1. 安全配置

**修改默认密码**：

首次登录后立即修改管理员密码。

**配置 HTTPS**：

```bash
# 使用 Let's Encrypt
sudo certbot --nginx -d api.example.com
```

**配置防火墙**：

```bash
# UFW
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable

# iptables
sudo iptables -A INPUT -p tcp --dport 80 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 443 -j ACCEPT
```

### 2. 性能优化

**数据库连接池**：

```yaml
database:
  max_open_conns: 50  # PostgreSQL 推荐
  max_idle_conns: 25
  conn_max_lifetime: 300s
```

**日志级别**：

```yaml
log:
  level: warn  # 生产环境使用 warn 或 error
```

**启用 Gzip 压缩**（Nginx）：

```nginx
gzip on;
gzip_vary on;
gzip_min_length 1024;
gzip_types text/plain text/css text/xml text/javascript application/json application/javascript application/xml+rss;
```

### 3. 监控和告警

**健康检查**：

```bash
# 添加到监控系统
curl http://localhost:8080/health
```

**日志轮转**：

```yaml
log:
  file:
    max_size: 100
    max_backups: 30
    max_age: 90
    compress: true
```

**Prometheus 监控**（可选）：

添加到 `/etc/prometheus/prometheus.yml`：

```yaml
scrape_configs:
  - job_name: 'hydra'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /metrics
```

### 4. 备份策略

**数据库备份**：

```bash
# SQLite
cp hydra.db backups/hydra-$(date +%Y%m%d-%H%M%S).db

# PostgreSQL
pg_dump -U hydra hydra > backups/hydra-$(date +%Y%m%d-%H%M%S).sql
```

**配置文件备份**：

```bash
tar czf backups/config-$(date +%Y%m%d-%H%M%S).tar.gz configs/
```

**自动化备份**（cron）：

```bash
# 每天凌晨 2 点备份
0 2 * * * /opt/hydra/scripts/backup.sh
```

### 5. 更新策略

**零停机更新**：

```bash
# 1. 构建新版本
make build

# 2. 备份当前版本
cp bin/hydra bin/hydra.backup

# 3. 停止服务
sudo systemctl stop hydra

# 4. 部署新版本
cp bin/hydra.new /opt/hydra/bin/hydra

# 5. 启动服务
sudo systemctl start hydra

# 6. 验证
curl http://localhost:8080/health
```

**数据库迁移**：

```bash
# 迁移前备份数据库
./bin/hydra -config configs/config.yaml 2>&1 | tee migrate.log
```

## 故障排查

如遇问题，请参考 [故障排查指南](./troubleshooting.md)。
