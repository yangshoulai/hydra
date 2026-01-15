# Implementation Plan: Hydra 高可用大模型聚合网关

**Branch**: `001-api-gateway-system` | **Date**: 2026-01-12 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-api-gateway-system/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

构建一个高可用大模型聚合网关系统,实现细粒度熔断、智能清洗、自动化运维和全链路日志功能。系统支持多渠道大模型 API 代理,在单个渠道或 Key 失效时自动切换,提供比直接调用上游更稳定的服务体验。核心价值包括:Key 级别和 Channel 级别的独立健康管理、HTTP 200 假成功响应的智能识别与拦截、上游模型自动发现与差异比对、基于 TraceID 的全链路请求追踪,以及清新简约的 Web 管理界面。

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**: Gin (Web 框架), GORM (ORM), Gormigrate (Migration), Slog (日志), Lumberjack (日志轮转)
**Storage**: SQLite (默认), PostgreSQL (可选)
**Testing**: Go 标准测试框架 (testing package), 集成测试与压力测试
**Target Platform**: Linux/macOS/Windows 服务器环境
**Project Type**: Web (后端 Go + 前端 Vue 3)
**Performance Goals**: 支持 1000-10000 QPS, 代理层开销 <50ms, 审计日志查询响应时间 <2s (100万条记录)
**Constraints**: 平均响应时间 ≤ 上游响应时间 + 50ms, 单文件部署, 启动时间 <5s, 仪表盘延迟 <5s
**Scale/Scope**: 中小规模使用 (QPS <10000), 单机部署, 审计日志保留30天 (<1000万条), 支持多渠道/多Key管理
**Frontend Stack**: Vue 3, Naive UI, TypeScript, 打包通过 Go Embed 内嵌到二进制文件

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### ✅ 核心原则符合性检查

1. **细粒度韧性 (Fine-Grained Resilience)** - 通过
   - ✅ Key 级别与 Channel 级别分别维护独立健康状态 (FR-011)
   - ✅ Key 故障只禁用该 Key,不影响同渠道其他 Key (FR-008)
   - ✅ 模型故障单独熔断,不影响渠道其他模型 (FR-012)
   - ✅ 区分硬故障(永久禁用)与软故障(熔断+恢复) (FR-007)
   - ✅ 熔断支持冷却期和半开探测策略 (FR-009, FR-010)
   - ✅ 熔断决策基于可配置的失败阈值和时间窗口 (FR-032)

2. **智能清洗与自愈 (Intelligent Sanitization & Self-Healing)** - 通过
   - ✅ 实现响应嗅探器识别 HTTP 200 中的错误 Body (FR-013)
   - ✅ 检测规则覆盖 JSON error 字段、HTML 响应、明文错误消息 (FR-014)
   - ✅ 假 200 响应视为软故障,触发自动重试 (FR-015)
   - ✅ 假 200 不透传给客户端 (FR-015)
   - ✅ 假 200 拦截事件记录到审计日志 (FR-016)

3. **自动化运维 (Automated Operations)** - 通过
   - ✅ 支持从上游 `/v1/models` 接口自动发现模型列表 (FR-017)
   - ✅ 提供模型差异比对功能(新增/缺失/存量) (FR-018)
   - ✅ 差异结果在 UI 中可视化展示 (FR-019, FR-030)
   - ✅ 支持一键应用变更 (FR-020)
   - ✅ 日志支持自动清理,基于可配置保留天数 (FR-026)
   - ✅ 支持测活功能,并发检测所有 Key 健康状态 (FR-030)

4. **用户体验优先 (User Experience First)** - 通过
   - ✅ 对外暴露标准化模型名,内部自动重写为上游真实名 (FR-004)
   - ✅ `/v1/models` 接口返回虚拟列表 (FR-002)
   - ✅ 请求失败时自动重试其他可用渠道,对用户透明 (FR-005, FR-007)
   - ✅ 错误响应规范化,提供明确错误码和可读信息 (边界情况处理)
   - ✅ 流式响应(SSE)保持稳定 (FR-001, 边界情况处理)

5. **可观测性 (Observability)** - 通过
   - ✅ 每个请求生成唯一 TraceID (FR-021)
   - ✅ 双层日志架构:审计日志(数据库) + 调试日志(文件) (FR-022, FR-023, FR-024)
   - ✅ 审计日志支持 UI 检索 (FR-025)
   - ✅ 调试日志支持日志轮转 (FR-027)
   - ✅ 关键指标在仪表盘实时可视化 (FR-029)

6. **简洁性与渐进式增强 (Simplicity & Progressive Enhancement)** - 通过
   - ✅ 按阶段划分:阶段一 MVP(基础代理)、阶段二高可用(熔断/UI)、阶段三完善(生产就绪)
   - ✅ 避免非必要抽象和第三方依赖(技术栈简洁)
   - ✅ 每个阶段独立可用

### ✅ 技术栈约束符合性检查

- ✅ 后端:Go 1.21+ + Gin + GORM + Gormigrate + Slog + Lumberjack
- ✅ 数据库:SQLite(默认) + PostgreSQL(可选支持)
- ✅ 前端:Vue 3 + Naive UI + Go Embed
- ✅ 禁止项:未引入微服务框架,使用 Slog 结构化日志,数据库限制为两种

### 结论

**通过所有宪法检查 ✓**

当前设计完全符合 Hydra 宪法的所有核心原则和技术栈约束,无需记录任何违规项或复杂性跟踪。

## Project Structure

### Documentation (this feature)

```text
specs/001-api-gateway-system/
├── plan.md              # 本文件 (/speckit.plan 命令输出)
├── research.md          # Phase 0 输出 (/speckit.plan 命令)
├── data-model.md        # Phase 1 输出 (/speckit.plan 命令)
├── quickstart.md        # Phase 1 输出 (/speckit.plan 命令)
├── contracts/           # Phase 1 输出 (/speckit.plan 命令)
│   ├── proxy-api.yaml   # OpenAPI 规范:代理接口(/v1/chat/completions, /v1/models)
│   └── admin-api.yaml   # OpenAPI 规范:管理后台 API
└── tasks.md             # Phase 2 输出 (/speckit.tasks 命令 - 不由 /speckit.plan 创建)
```

### Source Code (repository root)

```text
# Web 应用结构 (后端 Go + 前端 Vue 3)
backend/
├── cmd/
│   └── hydra/           # 主程序入口
│       └── main.go
├── internal/
│   ├── models/          # GORM 数据模型 (Channel, Key, RequestLog, SystemSetting, AccessToken, AdminUser)
│   ├── repository/      # 数据访问层
│   ├── service/         # 业务逻辑层
│   │   ├── proxy/       # 代理服务:路由、负载均衡、重试
│   │   ├── circuit/     # 熔断器:Key/Channel 级别状态机
│   │   ├── sniffer/     # 响应嗅探器:假 200 识别
│   │   ├── modelsync/   # 模型同步与差异比对
│   │   └── logger/      # 双层日志系统
│   ├── api/             # HTTP 处理器
│   │   ├── proxy/       # 代理接口 (/v1/*)
│   │   └── admin/       # 管理后台接口 (/admin/*)
│   ├── middleware/      # 中间件:认证、TraceID、日志
│   ├── config/          # 配置加载与验证
│   └── migration/       # Gormigrate 数据库迁移
├── pkg/                 # 公共工具包(可选)
└── tests/
    ├── integration/     # 集成测试:代理流程、熔断逻辑
    ├── contract/        # 契约测试:API 兼容性
    └── load/            # 压力测试:QPS、响应时间

frontend/
├── src/
│   ├── components/      # UI 组件:表格、表单、仪表盘图表
│   ├── pages/           # 页面:登录、仪表盘、渠道管理、日志查询、系统设置
│   ├── services/        # API 客户端:调用 /admin/* 接口
│   ├── stores/          # 状态管理 (Pinia 或 Composition API)
│   ├── router/          # Vue Router 路由配置
│   └── utils/           # 工具函数:时间格式化、错误处理
├── public/              # 静态资源
└── tests/
    └── unit/            # 前端单元测试 (Vitest)

# 根目录配置文件
configs/
├── config.yaml          # 默认配置模板
└── config.example.yaml  # 示例配置

# 部署相关
deployments/
├── Dockerfile           # Docker 镜像构建
└── docker-compose.yml   # 本地开发环境
```

**Structure Decision**:
采用 Web 应用结构(Option 2),因为项目包含 Go 后端 + Vue 3 前端。前端通过 Vite 构建,产物通过 Go Embed 内嵌到 `backend/cmd/hydra` 的二进制文件中,实现单文件分发。后端使用标准 Go 项目布局,`internal/` 包含核心业务逻辑,`cmd/` 包含主程序入口,`pkg/` 留作可复用的公共包(如需要)。前端遵循 Vue 3 最佳实践,清晰分离 components(组件)、pages(页面)、services(API 调用)和 stores(状态)。

## Complexity Tracking

> **当前设计无需记录任何复杂性跟踪项** - 所有设计决策均符合宪法原则。
