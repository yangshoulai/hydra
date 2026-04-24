# Hydra 快速启动

## 1. 环境要求

- Go 1.25+
- Node.js 20+
- pnpm

## 2. 启动后端

```bash
cd backend
go run ./cmd/hydra --data-dir ../data
```

默认信息：

- 地址: <http://localhost:8080>
- 管理后台: <http://localhost:8080>
- 管理员: `hydra / 123456`

`--data-dir` 指向的目录会在启动时自动创建，并生成：

- `<data-dir>/hydra.db`
- `<data-dir>/logs/hydra.log`

不带参数时，默认使用可执行文件同级的 `data/` 目录。

## 3. 启动前端开发环境（可选）

```bash
cd frontend
pnpm install
pnpm run dev
```

## 4. 构建生产包

先修改版本号文件：

```bash
vim VERSION
```

例如改成：

```text
2.1.0
```

```bash
# 推荐：统一构建（前后端都会使用 VERSION 中的版本号）
make build

# 运行
./bin/hydra --data-dir ./data
```

## 4.1 构建 Linux 镜像（适用于 macOS M2）

默认交叉构建 `linux/amd64` 并加载到本地 Docker：

```bash
make docker-build
```

如果要构建 `linux/arm64`：

```bash
make docker-build DOCKER_PLATFORM=linux/arm64
```

## 5. 验证

```bash
curl http://localhost:8080/health
```

返回：

```json
{"status":"ok"}
```

## 6. 数据目录

所有运行时数据都集中在 `--data-dir`：

- SQLite 数据库: `<data-dir>/hydra.db`
- 日志: `<data-dir>/logs/hydra.log`（lumberjack 轮转）

## 7. 系统设置

- 系统配置保存在数据库 `system_settings` 表，首次启动写入默认值
- 在管理后台「系统设置」页修改并保存
- 日志、代理、嗅探等类别即时生效；涉及监听端口等基础项会触发 AppManager 优雅重启

## 8. 功能提示

- 渠道模型通过管理后台按需手动同步
- 模型测试自动覆盖流式与非流式两种模式
- 请求明细通过结构化日志输出，便于与日志平台集成
