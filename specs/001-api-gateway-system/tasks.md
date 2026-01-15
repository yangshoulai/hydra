# 任务清单: Hydra 高可用大模型聚合网关

**特性分支**: `001-api-gateway-system`
**创建日期**: 2026-01-12
**规格文档**: [spec.md](./spec.md)
**实施计划**: [plan.md](./plan.md)

---

## 执行策略

### MVP 范围(最小可行产品)

**建议 MVP = 用户故事 1 (P1)**

仅实现 P1 故事即可提供核心价值:稳定的大模型 API 代理,支持自动切换和熔断保护。这是系统存在的根本意义。

P2-P3 故事(管理界面、日志查询、监控仪表盘)可以在 MVP 验证后渐进式交付。

---

## 任务统计

- **总任务数**: 156
- **可并行任务数**: 62
- **用户故事分布**:
  - Setup (初始化): 13 任务
  - Foundational (基础设施): 18 任务
  - User Story 1 (P1): 42 任务
  - User Story 2 (P2): 31 任务
  - User Story 3 (P3): 22 任务
  - User Story 4 (P3): 18 任务
  - Polish (完善): 12 任务

---

## Phase 1: Setup - 项目初始化

**目标**: 创建项目骨架,配置工具链和依赖

### 后端初始化

- [X] T001 在项目根目录创建 backend/ 和 frontend/ 目录
- [X] T002 初始化 Go 模块在 backend/go.mod
- [X] T003 [P] 创建后端目录结构 backend/cmd/hydra/main.go, backend/internal/
- [X] T004 [P] 安装 Go 依赖:gin, gorm, gormigrate, lumberjack 在 backend/go.mod
- [X] T005 [P] 创建配置文件模板 configs/config.example.yaml
- [X] T006 [P] 实现配置加载逻辑在 backend/internal/config/config.go

### 前端初始化

- [X] T007 初始化 Vue 3 项目在 frontend/(使用 Vite)
- [X] T008 [P] 安装前端依赖:Naive UI, Vue Router, Pinia, TypeScript 在 frontend/package.json
- [X] T009 [P] 创建前端目录结构 frontend/src/{components,pages,services,stores,router,utils}
- [X] T010 [P] 配置 Vite 构建输出到 frontend/dist

### 部署配置

- [X] T011 [P] 创建 Dockerfile 在 deployments/Dockerfile(多阶段构建:前端→后端)
- [X] T012 [P] 创建 docker-compose.yml 在 deployments/docker-compose.yml
- [X] T013 [P] 创建 README.md 根目录,包含快速开始指南

---

## Phase 2: Foundational - 基础设施

**目标**: 实现所有用户故事依赖的核心基础设施

**独立测试标准**: 数据库初始化成功,日志系统输出到文件和数据库,中间件正确注入上下文

### 数据库层

- [X] T014 创建 GORM 模型定义在 backend/internal/models/{channel.go, key.go, channel_model_config.go, request_log.go, system_setting.go, access_token.go, admin_user.go}
- [X] T015 实现数据库连接适配器(SQLite/PostgreSQL 切换)在 backend/internal/config/database.go
- [X] T016 创建 Gormigrate 迁移管理器在 backend/internal/migration/migration.go
- [X] T017 创建初始 Schema 迁移 v1.0.0 在 backend/internal/migration/v1_0_0_init.go
- [X] T018 实现数据库健康检查函数在 backend/internal/config/health.go

### 日志系统

- [X] T019 [P] 配置 Slog 结构化日志在 backend/internal/service/logger/slog.go
- [X] T020 [P] 配置 Lumberjack 日志轮转在 backend/internal/service/logger/lumberjack.go
- [X] T021 [P] 实现双层日志写入器(审计日志→数据库,调试日志→文件)在 backend/internal/service/logger/writer.go

### 中间件层

- [X] T022 [P] 实现 TraceID 中间件在 backend/internal/middleware/traceid.go
- [X] T023 [P] 实现请求日志中间件在 backend/internal/middleware/request_logger.go
- [X] T024 [P] 实现错误恢复中间件在 backend/internal/middleware/recovery.go
- [X] T025 [P] 实现访问令牌认证中间件在 backend/internal/middleware/auth.go
- [X] T026 [P] 实现管理后台会话认证中间件在 backend/internal/middleware/admin_auth.go

### Repository 层

- [X] T027 [P] 创建 Channel Repository 在 backend/internal/repository/channel_repo.go
- [X] T028 [P] 创建 Key Repository 在 backend/internal/repository/key_repo.go
- [X] T029 [P] 创建 ChannelModelConfig Repository 在 backend/internal/repository/model_config_repo.go
- [X] T030 [P] 创建 RequestLog Repository 在 backend/internal/repository/request_log_repo.go
- [X] T031 [P] 创建 SystemSetting Repository 在 backend/internal/repository/system_setting_repo.go

---

## Phase 3: User Story 1 (P1) - 稳定的大模型 API 调用

**目标**: 实现核心代理功能,支持多渠道自动切换、熔断保护和假 200 识别

**独立测试标准**:
- 配置 3 个渠道,每个渠道 2 个 Key,模拟其中一个失败,验证请求自动路由到其他可用渠道
- 模拟渠道返回假 200 响应,验证系统识别并重试其他渠道
- 模拟渠道连续失败 3 次,验证进入熔断状态

### 核心服务层

- [X] T032 [US1] 实现熔断器状态机(Key 级别)在 backend/internal/service/circuit/key_breaker.go
- [X] T033 [US1] 实现熔断器状态机(Channel 级别)在 backend/internal/service/circuit/channel_breaker.go
- [X] T034 [P] [US1] 实现熔断器管理器(状态维护和探测调度)在 backend/internal/service/circuit/manager.go
- [X] T035 [P] [US1] 实现响应嗅探器(假 200 识别)在 backend/internal/service/sniffer/response_sniffer.go
- [X] T036 [P] [US1] 配置嗅探规则(JSON error, HTML, 明文错误)在 backend/internal/service/sniffer/rules.go
- [X] T037 [US1] 实现 Key 选择器(从可用 Key 池中轮询)在 backend/internal/service/proxy/key_selector.go
- [X] T038 [US1] 实现 Channel 选择器(按优先级和权重选择)在 backend/internal/service/proxy/channel_selector.go
- [X] T039 [US1] 实现模型路由器(统一模型名→上游模型名映射)在 backend/internal/service/proxy/model_router.go
- [X] T040 [US1] 实现负载均衡器(多渠道流量分配)在 backend/internal/service/proxy/load_balancer.go

### 代理请求处理

- [X] T041 [US1] 实现代理请求构建器在 backend/internal/service/proxy/request_builder.go
- [X] T042 [US1] 实现上游 HTTP 客户端(支持超时和重试)在 backend/internal/service/proxy/http_client.go
- [X] T043 [US1] 实现 SSE 流式响应转发器在 backend/internal/service/proxy/sse_forwarder.go
- [X] T044 [US1] 实现非流式响应转发器在 backend/internal/service/proxy/response_forwarder.go
- [X] T045 [US1] 实现故障分类器(硬故障/软故障判断)在 backend/internal/service/proxy/failure_classifier.go
- [X] T046 [US1] 实现重试协调器(最大重试次数控制)在 backend/internal/service/proxy/retry_coordinator.go
- [X] T047 [US1] 实现代理服务主逻辑在 backend/internal/service/proxy/proxy_service.go

### API 处理器

- [X] T048 [US1] 实现 POST /v1/chat/completions 处理器在 backend/internal/api/proxy/chat_completions.go
- [X] T049 [P] [US1] 实现 GET /v1/models 处理器在 backend/internal/api/proxy/models.go
- [X] T050 [US1] 实现代理路由注册在 backend/internal/api/proxy/router.go

### 审计日志写入

- [X] T051 [P] [US1] 实现请求日志记录器(写入 RequestLog 表)在 backend/internal/service/logger/audit_logger.go
- [X] T052 [P] [US1] 实现调试日志写入(完整 Body 记录)在 backend/internal/service/logger/debug_logger.go

### 系统配置初始化

- [X] T053 [P] [US1] 实现系统设置初始化(熔断阈值、冷却时长、重试次数)在 backend/internal/service/config/initializer.go
- [X] T054 [P] [US1] 实现系统设置读取 Service 在 backend/internal/service/config/setting_service.go

### 主程序集成

- [X] T055 [US1] 在 main.go 中初始化数据库和日志系统在 backend/cmd/hydra/main.go
- [X] T056 [US1] 在 main.go 中注册代理路由和中间件在 backend/cmd/hydra/main.go
- [X] T057 [US1] 在 main.go 中启动熔断器管理器(探测调度)在 backend/cmd/hydra/main.go
- [X] T058 [US1] 在 main.go 中启动 HTTP 服务器在 backend/cmd/hydra/main.go

### 集成测试

- [ ] T059 [US1] 编写代理流程集成测试在 backend/tests/integration/proxy_test.go
- [ ] T060 [US1] 编写熔断逻辑集成测试在 backend/tests/integration/circuit_breaker_test.go
- [ ] T061 [US1] 编写假 200 识别集成测试在 backend/tests/integration/sniffer_test.go
- [ ] T062 [US1] 编写重试逻辑集成测试在 backend/tests/integration/retry_test.go

### 边界情况处理

- [X] T063 [P] [US1] 实现所有渠道不可用时的 503 错误处理在 backend/internal/service/proxy/error_handler.go
- [X] T064 [P] [US1] 实现超大响应 Body 限制(>10MB 返回 413)在 backend/internal/service/proxy/size_limiter.go
- [X] T065 [P] [US1] 实现 SSE 中途断流检测和日志记录在 backend/internal/service/proxy/sse_monitor.go
- [X] T066 [P] [US1] 实现请求队列缓冲(并发超限返回 429)在 backend/internal/service/proxy/rate_limiter.go
- [X] T067 [P] [US1] 实现探测请求失败时的重新冷却逻辑在 backend/internal/service/circuit/probe_handler.go
- [X] T068 [P] [US1] 实现模型名映射冲突处理(按权重选择)在 backend/internal/service/proxy/model_resolver.go
- [X] T069 [P] [US1] 实现模型不存在时的 404 错误返回在 backend/internal/service/proxy/model_validator.go
- [X] T070 [P] [US1] 实现数据库健康检测(启动时检查)在 backend/internal/config/db_health.go
- [X] T071 [P] [US1] 实现日志写入失败降级(磁盘满时降级到 stderr)在 backend/internal/service/logger/fallback_writer.go

### 契约测试

- [ ] T072 [P] [US1] 编写 POST /v1/chat/completions 契约测试在 backend/tests/contract/chat_completions_test.go
- [ ] T073 [P] [US1] 编写 GET /v1/models 契约测试在 backend/tests/contract/models_test.go

---

## Phase 4: User Story 2 (P2) - 低成本的渠道管理与配置

**目标**: 实现 Web 管理界面,支持渠道管理、模型同步和测活功能

**独立测试标准**:
- 在 Web 界面添加新渠道,点击"同步模型"按钮,验证能看到上游模型列表并完成映射配置
- 点击"立即测活",验证所有 Key 的健康状态并发检测完成

### 后端 API - 认证

- [X] T074 [P] [US2] 创建 AdminUser Repository 在 backend/internal/repository/admin_user_repo.go
- [X] T075 [P] [US2] 创建 AccessToken Repository 在 backend/internal/repository/access_token_repo.go
- [X] T076 [US2] 实现管理员登录 Service 在 backend/internal/service/admin/auth_service.go
- [X] T077 [US2] 实现 POST /admin/api/auth/login 处理器在 backend/internal/api/admin/auth.go
- [X] T078 [P] [US2] 实现 POST /admin/api/auth/logout 处理器在 backend/internal/api/admin/auth.go
- [X] T079 [P] [US2] 实现 GET /admin/api/auth/me 处理器在 backend/internal/api/admin/auth.go

### 后端 API - 渠道管理

- [X] T080 [P] [US2] 实现 GET /admin/api/channels 处理器(列表分页)在 backend/internal/api/admin/channels.go
- [X] T081 [P] [US2] 实现 POST /admin/api/channels 处理器(创建渠道)在 backend/internal/api/admin/channels.go
- [X] T082 [P] [US2] 实现 GET /admin/api/channels/:id 处理器(渠道详情)在 backend/internal/api/admin/channels.go
- [X] T083 [P] [US2] 实现 PUT /admin/api/channels/:id 处理器(更新渠道)在 backend/internal/api/admin/channels.go
- [X] T084 [P] [US2] 实现 DELETE /admin/api/channels/:id 处理器(删除渠道)在 backend/internal/api/admin/channels.go

### 后端 API - 模型同步

- [X] T085 [US2] 实现模型同步 Service(调用上游 /v1/models,计算差异)在 backend/internal/service/modelsync/sync_service.go
- [X] T086 [US2] 实现模型差异比对算法(新增/缺失/存量)在 backend/internal/service/modelsync/diff_calculator.go
- [X] T087 [US2] 实现 POST /admin/api/channels/:id/sync-models 处理器在 backend/internal/api/admin/model_sync.go

### 后端 API - Key 管理和测活

- [X] T088 [P] [US2] 实现 POST /admin/api/keys 处理器(添加 Key)在 backend/internal/api/admin/keys.go
- [X] T089 [P] [US2] 实现 DELETE /admin/api/keys/:id 处理器(删除 Key)在 backend/internal/api/admin/keys.go
- [X] T090 [P] [US2] 实现 PATCH /admin/api/keys/:id 处理器(重置 Key 状态)在 backend/internal/api/admin/keys.go
- [X] T091 [US2] 实现测活 Service(并发测试所有 Key)在 backend/internal/service/admin/health_check_service.go
- [X] T092 [US2] 实现 POST /admin/api/channels/:id/test-keys 处理器在 backend/internal/api/admin/keys.go

### 后端 API - 模型配置

- [X] T093 [P] [US2] 实现 POST /admin/api/channel-models 处理器(创建模型配置)在 backend/internal/api/admin/channel_models.go
- [X] T094 [P] [US2] 实现 PUT /admin/api/channel-models/:id 处理器(更新模型配置)在 backend/internal/api/admin/channel_models.go
- [X] T095 [P] [US2] 实现 DELETE /admin/api/channel-models/:id 处理器(删除模型配置)在 backend/internal/api/admin/channel_models.go

### 前端 - 页面和组件

- [X] T096 [P] [US2] 创建登录页面在 frontend/src/pages/Login.vue
- [X] T097 [P] [US2] 创建渠道列表页面在 frontend/src/pages/ChannelList.vue
- [X] T098 [P] [US2] 创建渠道详情页面在 frontend/src/pages/ChannelDetail.vue
- [X] T099 [P] [US2] 创建渠道编辑对话框组件在 frontend/src/components/ChannelDialog.vue
- [X] T100 [P] [US2] 创建模型同步 Diff 视图组件在 frontend/src/components/ModelSyncDiff.vue
- [X] T101 [P] [US2] 创建 Key 健康状态表格组件在 frontend/src/components/KeyHealthTable.vue

### 前端 - 服务和状态

- [X] T102 [P] [US2] 实现认证 API 客户端在 frontend/src/services/authService.ts
- [X] T103 [P] [US2] 实现渠道管理 API 客户端在 frontend/src/services/channelService.ts
- [X] T104 [P] [US2] 实现 Pinia 认证 Store 在 frontend/src/stores/auth.ts

---

## Phase 5: User Story 3 (P3) - 问题排查与审计

**目标**: 实现日志查询和审计功能

**独立测试标准**:
- 模拟一次失败请求,在管理后台日志页面通过 TraceID 搜索,验证能看到完整请求元数据

### 后端 API - 日志查询

- [X] T105 [P] [US3] 实现 GET /admin/api/logs 处理器(支持筛选和分页)在 backend/internal/api/admin/logs.go
- [X] T106 [P] [US3] 实现 GET /admin/api/logs/:traceId 处理器(日志详情)在 backend/internal/api/admin/logs.go
- [X] T107 [US3] 实现日志查询 Service(支持多条件筛选)在 backend/internal/service/admin/log_query_service.go

### 后端 - 日志自动清理

- [X] T108 [US3] 实现日志清理 Service(删除过期日志 + VACUUM)在 backend/internal/service/logger/cleanup_service.go
- [X] T109 [US3] 实现 Cron 定时任务调度器在 backend/internal/service/scheduler/cron.go
- [X] T110 [US3] 在 main.go 中启动日志清理定时任务(每日凌晨 3 点)在 backend/cmd/hydra/main.go

### 前端 - 页面和组件

- [X] T111 [P] [US3] 创建日志查询页面在 frontend/src/pages/LogQuery.vue
- [X] T112 [P] [US3] 创建日志筛选器组件在 frontend/src/components/LogFilter.vue
- [X] T113 [P] [US3] 创建日志详情抽屉组件在 frontend/src/components/LogDetailDrawer.vue
- [X] T114 [P] [US3] 创建 TraceID 复制按钮组件在 frontend/src/components/TraceIdCopy.vue

### 前端 - 服务

- [X] T115 [P] [US3] 实现日志查询 API 客户端在 frontend/src/services/logService.ts

### 调试日志增强

- [X] T116 [P] [US3] 实现调试模式开关逻辑(从 SystemSetting 读取)在 backend/internal/service/logger/debug_mode.go
- [X] T117 [P] [US3] 实现完整 Request/Response Body 写入文件日志在 backend/internal/service/logger/body_logger.go

### 集成测试

- [ ] T118 [US3] 编写日志查询集成测试在 backend/tests/integration/log_query_test.go
- [ ] T119 [US3] 编写日志清理集成测试在 backend/tests/integration/log_cleanup_test.go
- [ ] T120 [US3] 编写调试日志写入集成测试在 backend/tests/integration/debug_log_test.go

### 边界情况处理

- [X] T121 [P] [US3] 实现日志写入失败降级逻辑在 backend/internal/service/logger/fallback.go
- [X] T122 [P] [US3] 实现数据库 VACUUM 失败时的告警日志在 backend/internal/service/logger/cleanup_service.go

### 契约测试

- [ ] T123 [P] [US3] 编写 GET /admin/api/logs 契约测试在 backend/tests/contract/admin_logs_test.go
- [ ] T124 [P] [US3] 编写 GET /admin/api/logs/:traceId 契约测试在 backend/tests/contract/admin_logs_test.go
- [ ] T125 [P] [US3] 编写调试日志文件格式测试在 backend/tests/unit/debug_log_format_test.go
- [ ] T126 [P] [US3] 编写日志轮转测试在 backend/tests/unit/log_rotation_test.go

---

## Phase 6: User Story 4 (P3) - 实时监控与健康可视化

**目标**: 实现仪表盘实时监控功能

**独立测试标准**:
- 在仪表盘查看 QPS 波形图和渠道健康墙,模拟一个渠道进入熔断状态,验证状态实时更新

### 后端 API - 仪表盘指标

- [X] T127 [US4] 实现仪表盘指标计算 Service(QPS、成功率、渠道健康)在 backend/internal/service/admin/dashboard_service.go
- [X] T128 [US4] 实现 GET /admin/api/dashboard/metrics 处理器在 backend/internal/api/admin/dashboard.go
- [X] T129 [P] [US4] 实现 QPS 时间序列数据聚合(过去 1 小时)在 backend/internal/service/admin/qps_aggregator.go
- [X] T130 [P] [US4] 实现成功率计算器(今日成功率)在 backend/internal/service/admin/success_rate_calculator.go
- [X] T131 [P] [US4] 实现渠道健康状态汇总器在 backend/internal/service/admin/channel_health_aggregator.go

### 后端 API - 系统设置

- [X] T132 [P] [US4] 实现 GET /admin/api/settings 处理器(获取所有设置)在 backend/internal/api/admin/settings.go
- [X] T133 [P] [US4] 实现 PUT /admin/api/settings 处理器(批量更新设置)在 backend/internal/api/admin/settings.go

### 后端 API - 访问令牌管理

- [X] T134 [P] [US4] 实现 GET /admin/api/tokens 处理器(令牌列表)在 backend/internal/api/admin/tokens.go
- [X] T135 [P] [US4] 实现 POST /admin/api/tokens 处理器(创建令牌,返回明文)在 backend/internal/api/admin/tokens.go
- [X] T136 [P] [US4] 实现 DELETE /admin/api/tokens/:id 处理器(删除令牌)在 backend/internal/api/admin/tokens.go

### 前端 - 页面和组件

- [X] T137 [P] [US4] 创建仪表盘页面在 frontend/src/pages/Dashboard.vue
- [X] T138 [P] [US4] 创建 QPS 波形图组件(使用 ECharts)在 frontend/src/components/QpsChart.vue
- [X] T139 [P] [US4] 创建成功率指标卡片组件在 frontend/src/components/SuccessRateCard.vue
- [X] T140 [P] [US4] 创建渠道健康墙组件在 frontend/src/components/ChannelHealthWall.vue
- [X] T141 [P] [US4] 创建系统设置页面在 frontend/src/pages/Settings.vue
- [X] T142 [P] [US4] 创建访问令牌管理页面在 frontend/src/pages/TokenManagement.vue

### 前端 - 服务

- [X] T143 [P] [US4] 实现仪表盘 API 客户端在 frontend/src/services/dashboardService.ts
- [X] T144 [P] [US4] 实现轮询逻辑(每 5 秒拉取仪表盘数据)在 frontend/src/composables/usePolling.ts

---

## Phase 7: Polish - 完善与交叉关注点

**目标**: 完善系统细节,优化性能和用户体验

### 性能优化

- [X] T145 [P] 实现 RequestLog 表索引优化在 backend/internal/migration/v1_0_1_optimize_indexes.go
- [X] T146 [P] 实现数据库连接池配置在 backend/internal/config/database.go
- [X] T147 [P] 实现前端资源懒加载在 frontend/src/router/index.ts

### 前端集成与打包

- [X] T148 构建前端资源到 frontend/dist 在 frontend/package.json
- [X] T149 复制前端构建产物到 backend/static 在构建脚本
- [X] T150 实现 Go 静态文件服务在 backend/cmd/hydra/main.go
- [X] T151 实现 SPA 路由回退(NoRoute 返回 index.html)在 backend/cmd/hydra/main.go

### 文档和部署

- [X] T152 [P] 创建部署文档在 docs/deployment.md
- [X] T153 [P] 创建 API 文档在 docs/api.md(基于 OpenAPI 规范)
- [X] T154 [P] 创建故障排查指南在 docs/troubleshooting.md

### 压力测试

- [ ] T155 编写代理接口压力测试(1000 QPS)在 backend/tests/load/proxy_load_test.go
- [ ] T156 编写日志查询性能测试(100 万条记录)在 backend/tests/load/log_query_load_test.go

---

## 依赖关系图

```
┌─────────────────┐
│  Phase 1: Setup │
│  T001 - T013    │
└────────┬────────┘
         │
         ▼
┌──────────────────────┐
│ Phase 2: Foundational│
│ T014 - T031          │ ◀─────┐
└────────┬─────────────┘        │
         │                      │
         ├────────────────────┬─┴─────────────────┬─────────────────┐
         ▼                    ▼                   ▼                 ▼
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│ Phase 3: US1 (P1)│  │ Phase 4: US2 (P2)│  │ Phase 5: US3 (P3)│  │ Phase 6: US4 (P3)│
│ T032 - T073      │  │ T074 - T104      │  │ T105 - T126      │  │ T127 - T144      │
└────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘
         │                     │                     │                     │
         └─────────────────────┴─────────────────────┴─────────────────────┘
                                        │
                                        ▼
                              ┌──────────────────┐
                              │ Phase 7: Polish  │
                              │ T145 - T156      │
                              └──────────────────┘
```

### 用户故事完成顺序

1. **Foundational** (必须首先完成)
2. **User Story 1 (P1)** - MVP 核心功能
3. **User Story 2 (P2)** - 依赖 US1(需要代理功能可测试)
4. **User Story 3 (P3)** - 独立(仅依赖 Foundational)
5. **User Story 4 (P3)** - 依赖 US1(需要审计日志数据)
6. **Polish** - 最后完成

---

## 并行执行示例

### Foundational 阶段并行

```bash
# 组 1: 数据库层(串行,因为迁移依赖模型)
T014 → T015 → T016 → T017 → T018

# 组 2: 日志系统(完全并行,不同文件)
T019 [P] || T020 [P] || T021 [P]

# 组 3: 中间件层(完全并行,不同文件)
T022 [P] || T023 [P] || T024 [P] || T025 [P] || T026 [P]

# 组 4: Repository 层(完全并行,不同文件)
T027 [P] || T028 [P] || T029 [P] || T030 [P] || T031 [P]
```

### User Story 1 阶段并行

```bash
# 组 1: 熔断器(Channel 和 Key 可并行)
T032 → T034 (串行,Manager 依赖 Key Breaker)
T033 → T034 (串行,Manager 依赖 Channel Breaker)

# 组 2: 其他核心服务(完全并行)
T035 [P] || T036 [P] || T037 || T038 || T039 || T040

# 组 3: 代理请求处理(部分并行)
T041 → T042 → T047 (串行,Service 依赖 Client)
T043 [P] || T044 [P] || T045 [P] || T046 [P] (并行)

# 组 4: API 处理器(完全并行)
T048 || T049 [P] || T050

# 组 5: 日志和配置(完全并行)
T051 [P] || T052 [P] || T053 [P] || T054 [P]

# 组 6: 主程序集成(串行,依赖所有服务)
T055 → T056 → T057 → T058

# 组 7: 测试(部分并行)
T059 || T060 || T061 || T062

# 组 8: 边界情况(完全并行)
T063 [P] || T064 [P] || ... || T071 [P]

# 组 9: 契约测试(完全并行)
T072 [P] || T073 [P]
```

### User Story 2 阶段并行

```bash
# 组 1: 认证 API(串行,Handler 依赖 Service)
T074 [P] || T075 [P] → T076 → T077 → T078 [P] || T079 [P]

# 组 2: 渠道管理 API(完全并行)
T080 [P] || T081 [P] || T082 [P] || T083 [P] || T084 [P]

# 组 3: 模型同步(串行,Handler 依赖 Service)
T085 → T086 → T087

# 组 4: Key 管理和测活(部分并行)
T088 [P] || T089 [P] || T090 [P]
T091 → T092 (串行)

# 组 5: 模型配置 API(完全并行)
T093 [P] || T094 [P] || T095 [P]

# 组 6: 前端页面和组件(完全并行)
T096 [P] || T097 [P] || T098 [P] || T099 [P] || T100 [P] || T101 [P]

# 组 7: 前端服务(完全并行)
T102 [P] || T103 [P] || T104 [P]
```

---

## 实施建议

### MVP 阶段(用户故事 1)

```
1. Setup (T001-T013)
2. Foundational (T014-T031)
3. User Story 1 (T032-T073)
```

**总任务数**: 73
**预计时间**: 根据团队规模和并行能力确定
**验收标准**: 能够通过代理接口调用大模型,支持自动切换和熔断保护

### 增量交付(P2-P3)

完成 MVP 后,按优先级渐进式交付:

1. **User Story 2** (T074-T104): 管理界面和渠道配置
2. **User Story 3** (T105-T126): 日志查询和审计
3. **User Story 4** (T127-T144): 监控仪表盘
4. **Polish** (T145-T156): 性能优化和文档

---

## 格式验证

✅ 所有任务遵循严格的 Checklist 格式:
- 每个任务以 `- [ ]` 开头
- 包含任务 ID (T001, T002, ...)
- 可并行任务标记 `[P]`
- 用户故事阶段任务标记 `[US1]`, `[US2]`, `[US3]`, `[US4]`
- 包含清晰的描述和文件路径

✅ 任务组织按用户故事分阶段,支持独立实现和测试

✅ 依赖关系清晰,支持并行执行优化

---

## 下一步

1. 阅读 [spec.md](./spec.md) 了解完整功能需求
2. 阅读 [plan.md](./plan.md) 了解技术栈和架构决策
3. 阅读 [data-model.md](./data-model.md) 了解数据库结构
4. 阅读 [contracts/](./contracts/) 了解 API 接口规范
5. 开始执行任务,建议从 Phase 1 (Setup) 开始!

**祝你编码愉快!** 🚀
