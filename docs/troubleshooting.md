# Hydra API Gateway 故障排查指南

## 目录

- [常见问题](#常见问题)
- [启动问题](#启动问题)
- [性能问题](#性能问题)
- [数据库问题](#数据库问题)
- [网络问题](#网络问题)
- [日志分析](#日志分析)

## 常见问题

### Q1: 服务无法启动

**症状**：执行 `./hydra` 后立即退出

**排查步骤**：

1. 检查配置文件是否存在：
```bash
ls -la configs/config.yaml
```

2. 验证配置文件格式：
```bash
./hydra -config configs/config.yaml 2>&1 | tee startup.log
```

3. 检查端口占用：
```bash
# Linux/macOS
lsof -i :8080

# 或
netstat -an | grep 8080
```

4. 解决方案：
   - 修改配置文件中的端口号
   - 或终止占用端口的进程：`kill -9 <PID>`

### Q2: 无法连接数据库

**症状**：日志显示 "failed to connect to database"

**排查步骤**：

1. **SQLite**：
```bash
# 检查文件权限
ls -la hydra.db

# 检查文件是否被锁定
fuser hydra.db 2>/dev/null || echo "File not locked"
```

2. **PostgreSQL**：
```bash
# 测试连接
psql -h localhost -U hydra -d hydra

# 检查 PostgreSQL 状态
sudo systemctl status postgresql
```

3. 解决方案：
   - SQLite：确保程序有读写权限
   - PostgreSQL：检查 `postgres_dsn` 配置，确保数据库和用户存在

### Q3: 请求返回 401 Unauthorized

**症状**：API 请求返回 401 错误

**原因**：
- Access Token 无效或过期
- Token 未正确传递

**解决方案**：

1. 检查 Token 格式：
```http
Authorization: Bearer YOUR_TOKEN
```

2. 验证 Token 是否存在：
```bash
# 通过管理后台查看
curl http://localhost:8080/admin/api/tokens \
  -H "Cookie: admin_session=YOUR_SESSION"
```

3. 创建新 Token：
```bash
curl -X POST http://localhost:8080/admin/api/tokens \
  -H "Content-Type: application/json" \
  -H "Cookie: admin_session=YOUR_SESSION" \
  -d '{"remark": "New token"}'
```

### Q4: 所有渠道进入熔断状态

**症状**：日志显示 "All channels are in circuit breaker state"

**原因**：
- 连续失败次数超过阈值
- 冷却时间未结束

**解决方案**：

1. 查看渠道健康状态：
```bash
curl http://localhost:8080/admin/api/dashboard/metrics \
  -H "Cookie: admin_session=YOUR_SESSION" | jq '.data.channel_health_list'
```

2. 检查上游服务是否正常：
```bash
# 测试上游 API
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer YOUR_KEY"
```

3. 调整熔断器配置：
```bash
curl -X PUT http://localhost:8080/admin/api/settings \
  -H "Content-Type: application/json" \
  -H "Cookie: admin_session=YOUR_SESSION" \
  -d '{
    "settings": {
      "circuit_breaker_failure_threshold": "10",
      "circuit_breaker_cooling_duration": "30"
    }
  }'
```

4. 重置 Key 状态：
```bash
curl -X PATCH http://localhost:8080/admin/api/keys/1 \
  -H "Cookie: admin_session=YOUR_SESSION"
```

### Q5: 响应很慢

**症状**：API 响应时间过长

**排查步骤**：

1. 检查日志中的 `response_time`：
```bash
grep "response_time" logs/hydra.log | tail -20
```

2. 查看仪表盘指标：
```bash
curl -s http://localhost:8080/admin/api/dashboard/metrics \
  -H "Cookie: admin_session=YOUR_SESSION" | jq '.data.current_qps'
```

3. 常见原因：
   - 上游 API 响应慢
   - 数据库查询慢（日志表数据过多）
   - 本地网络问题

4. 解决方案：
   - 调整 `request_timeout` 配置
   - 清理旧日志：`rm logs/hydra.log.*`
   - 执行数据库 VACUUM（SQLite）

## 启动问题

### 配置文件错误

**症状**：
```
Error: failed to load config: invalid config
```

**解决方案**：

1. 验证 YAML 语法：
```bash
# 使用 yamllint
yamllint configs/config.yaml

# 或在线验证
# https://www.yamllint.com/
```

2. 检查必填字段：
```yaml
server:
  port: 8080  # 必填

database:
  type: sqlite  # 必填：sqlite 或 postgres
```

3. 查看配置模板：
```bash
cat configs/config.example.yaml
```

### 数据库迁移失败

**症状**：
```
Error: failed to run migrations: migration failed
```

**解决方案**：

1. 查看迁移日志：
```bash
./hydra -config configs/config.yaml 2>&1 | grep -i migration
```

2. 删除数据库重新初始化（⚠️ 会丢失数据）：
```bash
# SQLite
rm hydra.db

# PostgreSQL
psql -U hydra -d hydra -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
```

3. 手动运行迁移：
```bash
# 检查迁移历史
sqlite3 hydra.db "SELECT * FROM schema_migrations;"
```

## 性能问题

### 高 CPU 占用

**症状**：CPU 占用率持续 > 80%

**排查步骤**：

1. 查看进程详情：
```bash
# Linux
top -p $(pgrep hydra)

# 查看线程
ps -eLf | grep hydra
```

2. 检查 QPS：
```bash
curl -s http://localhost:8080/admin/api/dashboard/metrics \
  -H "Cookie: admin_session=YOUR_SESSION" | jq '.data.current_qps'
```

3. 优化方案：
   - 启用调试日志会增加 CPU，生产环境关闭
   - 检查是否有死循环
   - 增加数据库连接池大小

### 内存泄漏

**症状**：内存占用持续增长

**排查步骤**：

1. 监控内存使用：
```bash
# 实时监控
watch -n 5 'ps aux | grep hydra'

# 详细信息
cat /proc/$(pgrep hydra)/status | grep -i mem
```

2. 检查日志文件大小：
```bash
du -sh logs/
```

3. 解决方案：
   - 配置日志轮转
   - 定期重启服务
   - 使用 pprof 分析：
```bash
# 启用 pprof
import _ "net/http/pprof"

# 访问
go tool pprof http://localhost:8080/debug/pprof/heap
```

### 数据库查询慢

**症状**：日志查询超时

**解决方案**：

1. 检查索引：
```bash
sqlite3 hydra.db ".indexes request_logs"
```

2. 运行 ANALYZE：
```bash
sqlite3 hydra.db "ANALYZE;"
```

3. 清理旧日志：
```bash
# 手动清理（保留最近 30 天）
sqlite3 hydra.db "DELETE FROM request_logs WHERE created_at < datetime('now', '-30 days'); VACUUM;"
```

## 网络问题

### 代理请求超时

**症状**：代理请求返回超时错误

**解决方案**：

1. 检查上游 API 状态：
```bash
curl -v https://api.openai.com/v1/models \
  -H "Authorization: Bearer YOUR_KEY"
```

2. 调整超时配置：
```yaml
proxy:
  request_timeout: 120s  # 增加到 120 秒
```

3. 检查本地网络：
```bash
ping api.openai.com
traceroute api.openai.com
```

### TLS/SSL 错误

**症状**：
```
Error: x509: certificate signed by unknown authority
```

**解决方案**：

1. 跳过证书验证（仅测试环境）：
```go
// 代码中添加
http.Transport{
    TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}
```

2. 或更新 CA 证书：
```bash
# Ubuntu/Debian
sudo apt-get install ca-certificates

# CentOS/RHEL
sudo yum update ca-certificates
```

## 日志分析

### 启用调试日志

```bash
# 方式 1：修改配置文件
vim configs/config.yaml
# 设置: log.level: debug

# 方式 2：通过 API
curl -X PUT http://localhost:8080/admin/api/settings \
  -H "Content-Type: application/json" \
  -H "Cookie: admin_session=YOUR_SESSION" \
  -d '{"settings": {"debug_mode": "true"}}'
```

### 查看实时日志

```bash
# 实时 tail
tail -f logs/hydra.log

# 过滤错误
tail -f logs/hydra.log | grep -i error

# 过滤特定 Trace ID
tail -f logs/hydra.log | grep "trace_id"
```

### 日志搜索技巧

```bash
# 查找失败的请求
grep "is_success\":false" logs/hydra.log

# 查找慢请求（> 5 秒）
grep "response_time\":[5-9][0-9][0-9][0-9]" logs/hydra.log

# 查找熔断事件
grep -i "circuit.*breaker.*open" logs/hydra.log

# 统计今日请求数
grep "$(date +%Y-%m-%d)" logs/hydra.log | wc -l
```

### 常见日志信息

| 日志内容 | 说明 | 建议操作 |
|---------|------|---------|
| `failed to get channel` | 渠道不存在 | 检查渠道配置 |
| `all channels in circuit breaker` | 所有渠道熔断 | 检查上游服务 |
| `fake 200 response detected` | 检测到假 200 成功 | 检查嗅探规则 |
| `retry attempt` | 请求重试中 | 正常，无需操作 |
| `request timeout` | 请求超时 | 调整超时配置 |

## 监控和告警

### 健康检查

```bash
# 基础健康检查
curl http://localhost:8080/health

# 预期响应
{
  "status": "ok",
  "version": "1.0.0"
}
```

### 设置监控脚本

创建 `monitor.sh`：

```bash
#!/bin/bash

# 检查服务是否运行
if ! curl -sf http://localhost:8080/health > /dev/null; then
    echo "CRITICAL: Hydra service is down!"
    # 发送告警（邮件、Slack 等）
    exit 2
fi

# 检查 QPS 是否正常
QPS=$(curl -s http://localhost:8080/admin/api/dashboard/metrics \
  -H "Cookie: admin_session=YOUR_SESSION" | jq '.data.current_qps')

if (( $(echo "$QPS > 1000" | bc -l) )); then
    echo "WARNING: High QPS detected: $QPS"
fi

echo "OK: Service is healthy"
```

设置 cron：

```bash
# 每 5 分钟检查一次
*/5 * * * * /opt/hydra/scripts/monitor.sh
```

## 获取支持

如未解决问题，请：

1. 查看 GitHub Issues
2. 提交新的 Issue（包含日志和配置）
3. 查看完整日志：`tail -1000 logs/hydra.log`
4. 导出诊断信息：
```bash
./hydra -version
uname -a
go version
```
