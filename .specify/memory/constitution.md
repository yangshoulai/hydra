<!--
Sync Impact Report:
- Version change: 1.0.0 → 1.0.1
- Modified principles: None
- Modified sections:
  - 技术栈约束 > 后端 > 数据库: MySQL → PostgreSQL
  - 技术栈约束 > 禁止项: 移除对 MySQL 的引用,更新为 PostgreSQL
- Added sections: None
- Removed sections: None
- Templates requiring updates:
  - ✅ .specify/templates/plan-template.md - No changes needed (generic constitution check)
  - ✅ .specify/templates/spec-template.md - No changes needed (technology-agnostic)
  - ✅ .specify/templates/tasks-template.md - No changes needed (no database-specific constraints)
- Follow-up TODOs: None
- Rationale: PATCH version bump - database technology clarification (PostgreSQL instead of MySQL), no principle changes
-->

# Hydra (九头蛇) 宪法

## 核心原则

### 一、细粒度韧性 (Fine-Grained Resilience)

**不可协商 (NON-NEGOTIABLE)**

Hydra 必须实现"砍掉一个头,长出两个头"的高韧性架构。系统必须在最小粒度上进行故障隔离和恢复,绝不因局部故障而误杀整体可用资源。

**强制要求**:
- Key 级别与 Channel 级别必须分别维护独立的健康状态
- Key 故障只能禁用该 Key,不得影响同渠道其他 Key
- 模型故障只能禁用该模型,不得影响同渠道其他模型
- 必须区分硬故障(永久禁用)与软故障(熔断+恢复)
- 熔断必须支持冷却期和半开(Half-Open)探测策略
- 任何熔断决策必须基于可配置的失败阈值和时间窗口

**理由**: 现有网关工具(如 OneAPI/NewAPI)采用粗颗粒度管理,一个 Key 或模型失败导致整个渠道被禁用,造成严重资源浪费。Hydra 的核心价值在于通过细粒度管理最大化可用资源利用率。

---

### 二、智能清洗与自愈 (Intelligent Sanitization & Self-Healing)

**不可协商 (NON-NEGOTIABLE)**

系统必须识别并拦截上游的"假成功"响应,对用户屏蔽上游故障,实现透明的自动重试。

**强制要求**:
- 必须实现响应嗅探器(Response Sniffer)识别 HTTP 200 中的错误 Body
- 检测规则必须覆盖常见错误模式:
  - JSON 响应中包含 `"error"` 字段
  - HTML 响应(如 `<!DOCTYPE html>`)
  - 明文错误消息(如 "无可用后端", "额度不足", "maintenance")
- 识别为"假 200"的响应必须视为软故障,触发自动重试
- 重试必须自动切换到其他可用渠道/Key
- 不得将"假 200"响应直接透传给客户端

**理由**: 第三方渠道经常返回 HTTP 200 但 Body 是错误信息,传统网关无法识别,导致客户端解析失败或用户看到乱码。智能清洗是 Hydra 的核心差异化功能。

---

### 三、自动化运维 (Automated Operations)

系统必须最小化人工维护成本,通过自动化手段发现配置漂移并提供辅助决策。

**强制要求**:
- 必须支持从上游 `/v1/models` 接口自动发现模型列表
- 必须提供模型差异比对(Diff)功能,计算:
  - 🟢 新增模型(上游新上架,本地未配置)
  - 🔴 缺失模型(本地已配置,上游已下架)
  - 🔵 存量模型(两边都有,保留本地重命名配置)
- Diff 结果必须在 UI 中可视化展示,支持用户一键应用变更
- 日志必须支持自动清理,基于可配置的保留天数
- 必须支持"测活"功能,对渠道所有 Key 并发进行健康检测

**理由**: 上游渠道模型命名混乱且经常变动,手动维护映射关系成本极高且易出错。自动发现与差异比对大幅降低运维负担。

---

### 四、用户体验优先 (User Experience First)

所有面向用户的接口必须提供一致、稳定、透明的体验,隐藏上游故障的复杂性。

**强制要求**:
- 对外暴露标准化模型名(如 `gpt-4`),内部自动重写为上游真实名
- `/v1/models` 接口必须返回虚拟列表,包含所有系统配置的统一模型名
- 客户端请求失败时,系统必须自动重试其他可用渠道,对用户透明
- 错误响应必须规范化,提供明确的错误码和人类可读的错误信息
- 流式响应(SSE)必须保持稳定,避免中途断流

**理由**: 用户不应感知到上游渠道的不稳定性和命名差异。一致的接口体验是生产环境可用性的前提。

---

### 五、可观测性 (Observability)

系统必须提供全链路可追踪的日志系统,支持快速问题定位和审计。

**强制要求**:
- 必须为每个请求生成唯一 TraceID,贯穿整个请求生命周期
- 必须实现双层日志架构:
  - **审计日志(数据库)**: 记录 TraceID、用户、模型、耗时、状态码、简略错误信息
  - **调试日志(文件)**: 记录 TraceID、完整 Request/Response Body(仅出错或 Debug 模式)
- 审计日志必须支持 UI 检索,按 TraceID、时间范围、状态、渠道筛选
- 调试日志必须支持日志轮转(Log Rotation),避免磁盘空间耗尽
- 关键指标(QPS、成功率、渠道健康度)必须在仪表盘实时可视化

**理由**: 在复杂的多渠道路由场景下,没有完善的可观测性,问题排查将极其困难。TraceID 是串联审计日志与调试日志的关键。

---

### 六、简洁性与渐进式增强 (Simplicity & Progressive Enhancement)

系统架构必须从简单开始,避免过度设计,按阶段渐进式交付价值。

**强制要求**:
- MVP 阶段必须优先实现核心流程: `Client -> Hydra -> Upstream` 的基础代理与日志
- 高级功能(如权重分配、优先级路由)可以在后续阶段增强
- 避免引入非必要的抽象和第三方依赖
- 每个阶段交付必须是独立可用的,可以部署和验证
- 不得为"假设的未来需求"设计通用框架

**理由**: 按照需求文档的 Roadmap(阶段一 MVP、阶段二高可用、阶段三完善),渐进式交付可以更快验证核心价值,避免前期过度投入。

---

## 技术栈约束 (Technology Stack Constraints)

### 后端

- **语言**: Go 1.21+
- **Web 框架**: Gin
- **数据库**: SQLite(默认),支持 PostgreSQL
- **ORM**: GORM
- **Migration**: Gormigrate
- **日志**: Slog + Lumberjack

**理由**: Go 在高并发 HTTP/SSE 流式转发场景下性能卓越,部署简单。SQLite 实现零配置,适合私有化部署。PostgreSQL 支持为需要更强事务能力和高并发写入的场景提供选择。

### 前端

- **框架**: Vue 3
- **UI 组件库**: Naive UI
- **构建**: Go Embed(前端构建产物打包进 Go 二进制,实现单文件分发)

**理由**: Naive UI 组件丰富(特别是数据表格),TypeScript 支持良好。Go Embed 实现"单文件分发",降低部署复杂度。

### 禁止项

- 不得引入重量级微服务框架(如 gRPC、Kubernetes Operator)
- 不得使用非结构化日志(必须使用 Slog)
- 不得同时支持超过两种数据库类型(当前: SQLite + PostgreSQL)

---

## 开发流程 (Development Workflow)

### 阶段划分

系统开发必须严格按照以下阶段执行:

1. **阶段一: MVP 核心**
   - 目标: 跑通基础代理流程,不含复杂 UI
   - 交付物: 可工作的代理、基础路由、审计日志

2. **阶段二: 高可用与管理**
   - 目标: 实现细粒度熔断、智能清洗、UI 管理
   - 交付物: Key Pool 状态机、响应嗅探器、渠道管理 UI、模型同步 Diff

3. **阶段三: 完善与交付**
   - 目标: 生产环境就绪
   - 交付物: 日志自动清理、仪表盘、访问控制、Docker 部署、压力测试

### 质量门禁

- 每个阶段结束前必须通过功能验证
- 阶段二必须完成核心熔断逻辑的集成测试
- 阶段三必须通过压力测试(目标 QPS 需根据实际场景确定)
- 所有 API 接口必须符合 OpenAI 兼容标准

### 代码规范

- 所有错误处理必须记录到日志系统
- 所有数据库操作必须通过 GORM,不得使用原生 SQL(除非性能关键路径)
- 所有配置项必须可通过环境变量或配置文件覆盖
- 所有 UI 交互必须提供加载状态和错误提示

---

## 治理 (Governance)

### 宪法地位

本宪法是 Hydra 项目的最高开发准则,所有设计决策、代码审查、功能实现必须符合宪法原则。

### 修订流程

宪法修订必须满足以下条件:
1. 有明确的修订理由和影响分析
2. 更新 `CONSTITUTION_VERSION` 并遵循语义化版本规则:
   - **MAJOR**: 删除或重新定义核心原则,不向后兼容
   - **MINOR**: 新增原则或实质性扩展现有原则
   - **PATCH**: 措辞优化、错误修正、非语义性改进
3. 同步更新所有依赖模板(plan-template.md、spec-template.md、tasks-template.md)
4. 在修订提交中包含 Sync Impact Report

### 合规审查

- 所有功能设计必须在 `plan.md` 中包含 "Constitution Check" 章节
- 违反宪法原则的设计必须在 "Complexity Tracking" 中明确说明理由
- Code Review 必须验证实现是否符合宪法约束
- 如遇宪法原则冲突,优先级顺序为: 细粒度韧性 > 智能清洗 > 可观测性 > 其他

---

**版本**: 1.0.1 | **批准日期**: 2026-01-12 | **最后修订**: 2026-01-12
