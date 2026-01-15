# 技术研究: Hydra 高可用大模型聚合网关

**日期**: 2026-01-12
**阶段**: Phase 0 - 技术选型与最佳实践研究
**目的**: 为实现计划提供技术决策依据,解决关键技术挑战

## 研究概览

本文档记录了 Hydra 项目关键技术选型的研究过程、决策依据和替代方案评估。

## 1. SSE 流式响应转发

### 决策
使用 Go 标准库 `net/http` 的 `http.ResponseWriter` 配合 `http.Flusher` 接口实现 SSE (Server-Sent Events) 流式转发。

### 理由
- **零依赖**: 标准库原生支持,无需引入第三方库
- **性能优秀**: Go 的 goroutine 和channel 机制天然适合流式处理
- **简单可靠**: `Flusher.Flush()` 确保数据及时写入客户端
- **错误透明**: 上游断开连接时,标准库会自动返回错误,便于检测和记录

### 实现要点
```go
// 伪代码示例
func proxySSE(w http.ResponseWriter, upstream *http.Response) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        // 降级处理: 客户端不支持流式
    }

    scanner := bufio.NewScanner(upstream.Body)
    for scanner.Scan() {
        w.Write(scanner.Bytes())
        flusher.Flush() // 立即推送到客户端
    }

    if err := scanner.Err(); err != nil {
        // 记录上游断流错误
    }
}
```

### 替代方案
- **第三方库 (如 `r3labs/sse`)**: 提供更高级抽象,但增加依赖且功能过剩
- **WebSocket**: 双向通信,但 OpenAI API 使用 SSE,保持兼容性更重要

---

## 2. 数据库适配层设计 (SQLite + PostgreSQL)

### 决策
使用 **适配器模式** + **GORM Dialector** 实现多数据库支持,运行时通过环境变量选择数据库类型。

### 理由
- **GORM 原生支持**: GORM v2 提供 `gorm.Dialector` 接口,轻松切换数据库驱动
- **零运行时开销**: 编译时确定数据库类型,无动态分支
- **配置简单**: 通过 `DATABASE_TYPE=sqlite|postgres` 环境变量控制
- **迁移脚本通用**: Gormigrate 自动适配不同数据库的 SQL 方言

### 实现要点
```go
// 伪代码示例
func NewDB(cfg *Config) (*gorm.DB, error) {
    var dialector gorm.Dialector

    switch cfg.DatabaseType {
    case "sqlite":
        dialector = sqlite.Open(cfg.SQLitePath)
    case "postgres":
        dialector = postgres.Open(cfg.PostgresDSN)
    default:
        return nil, fmt.Errorf("unsupported database: %s", cfg.DatabaseType)
    }

    return gorm.Open(dialector, &gorm.Config{})
}
```

### 数据库选择建议
- **SQLite**: 默认选项,适合单机部署、QPS < 5000、无高并发写入场景
- **PostgreSQL**: 适合分布式部署、QPS > 5000、需要强事务保证的场景

### 替代方案
- **硬编码单一数据库**: 简单但不灵活,违反宪法"支持两种数据库"的要求
- **运行时抽象层 (如 sqlx)**: 增加复杂度,GORM 已提供足够抽象

---

## 3. Tailwind CSS 与 Naive UI 集成

### 决策
使用 **Tailwind CSS** 作为实用工具类样式库,与 **Naive UI** 组件库互补使用。

### 理由
- **互补而非冲突**: Naive UI 提供功能组件(表格、对话框),Tailwind 提供样式工具类(布局、间距、颜色)
- **UI/UX 需求契合**: 规格要求"清新简约",Tailwind 的原子化 CSS 天然支持最小化样式
- **高度可定制**: Tailwind 配置文件可定义主色调(≤ 3 种),符合 NFR-003
- **性能优化**: Tailwind 的 PurgeCSS 自动移除未使用的样式,减小 CSS 体积

### 集成策略
1. **组件结构**: Naive UI 组件包裹在 Tailwind 样式的容器中
2. **主题定制**: 在 `tailwind.config.js` 中定义与 Naive UI 一致的主题色
3. **样式隔离**: Naive UI 的内部样式不受 Tailwind 重置样式影响

### Tailwind 配置示例
```javascript
// tailwind.config.js
module.exports = {
  content: ['./src/**/*.{vue,js,ts}'],
  theme: {
    extend: {
      colors: {
        primary: '#18a058',   // Naive UI 默认绿色
        warning: '#f0a020',   // 警告黄色
        error: '#d03050',     // 错误红色
      },
    },
  },
}
```

### 替代方案
- **仅使用 Naive UI 内置样式**: 定制能力弱,难以实现清新简约的 UI/UX 需求
- **自定义 CSS**: 维护成本高,且不符合现代原子化 CSS 最佳实践
- **其他 CSS 框架 (如 UnoCSS)**: 学习成本高,Tailwind 生态成熟

---

## 4. Go Embed 前端构建流程

### 决策
使用 **Vite 构建前端 → Go Embed 嵌入 → 单二进制文件分发** 的工作流。

### 理由
- **宪法要求**: "通过 Go Embed 打包,实现单文件分发"
- **开发体验**: Vite 提供热重载和快速构建,开发效率高
- **生产优化**: Vite 自动代码分割、Tree Shaking、资源压缩
- **部署简单**: 最终只需一个二进制文件 + 配置文件,无需 Web 服务器

### 构建流程
```bash
# 1. 构建前端 (生成到 web/dist/)
cd web && npm run build

# 2. Go Embed 嵌入
//go:embed web/dist/*
var staticFiles embed.FS

# 3. 在 Gin 中提供静态文件服务
r.StaticFS("/admin", http.FS(staticFiles))

# 4. 编译 Go 二进制
go build -o hydra cmd/hydra/main.go
```

### 注意事项
- **路由冲突**: 管理界面路由 (`/admin/*`) 与代理接口路由 (`/v1/*`) 需明确分离
- **SPA 回退**: Vue Router 使用 History 模式时,需配置 Gin 的 `NoRoute` 处理器返回 `index.html`
- **资源路径**: Vite 构建的资源路径需与 Go Embed 的路径一致 (通过 `base` 配置)

### 替代方案
- **独立前端部署**: 需要 Nginx 等 Web 服务器,增加运维复杂度,违反单文件分发原则
- **服务端渲染 (SSR)**: Go 不适合 Vue SSR,且无必要(管理界面无 SEO 需求)

---

## 5. 熔断器状态机设计

### 决策
实现 **Key 级** 和 **Channel 级** 两层熔断器,状态机包含 **Active → Cooling → HalfOpen → Active/Dead** 四种状态。

### 理由
- **宪法核心原则**: "细粒度韧性",必须分别管理 Key 和 Channel 健康状态
- **故障隔离**: Key 故障不影响同渠道其他 Key,Channel 熔断不影响其他 Channel
- **自愈机制**: 半开状态允许探测恢复,避免永久阻塞可用资源

### 状态转换规则
```
Active (正常)
  ├─ 硬故障 (401/402/403/429 quota) → Dead (永久禁用)
  └─ 软故障 (5xx/timeout) 连续 N 次 → Cooling (冷却中)

Cooling (冷却中)
  └─ 冷却时间结束 → HalfOpen (半开)

HalfOpen (半开)
  ├─ 探测请求成功 → Active (恢复)
  └─ 探测请求失败 → Cooling (重新冷却)

Dead (永久禁用)
  └─ 人工重置 → Active (恢复)
```

### 并发安全
使用 `sync.RWMutex` 保护状态读写,失败计数使用 `atomic` 包原子操作。

### 替代方案
- **单层熔断器**: 粗颗粒度,违反宪法原则
- **第三方库 (如 sony/gobreaker)**: 不支持 Key 级细粒度控制,需大量定制

---

## 6. 假 200 响应嗅探策略

### 决策
实现 **基于规则的响应嗅探器**,支持 JSON/HTML/明文错误三种模式识别。

### 理由
- **宪法核心原则**: "智能清洗与自愈",必须识别并拦截假成功响应
- **规则可扩展**: 新增错误模式只需添加规则,无需修改核心逻辑
- **性能优化**: 仅读取响应前 4KB 进行检测,避免全量读取大响应

### 检测规则实现
```go
type SnifferRule interface {
    Match(body []byte, contentType string) bool
}

// JSON 错误规则
type JSONErrorRule struct{}
func (r *JSONErrorRule) Match(body []byte, ct string) bool {
    if !strings.Contains(ct, "application/json") {
        return false
    }
    return bytes.Contains(body, []byte(`"error"`))
}

// HTML 响应规则
type HTMLResponseRule struct{}
func (r *HTMLResponseRule) Match(body []byte, ct string) bool {
    return bytes.HasPrefix(bytes.TrimSpace(body), []byte("<!DOCTYPE"))
}

// 明文错误规则
type PlainTextErrorRule struct {
    Keywords []string // ["无可用后端", "额度不足", "maintenance"]
}
func (r *PlainTextErrorRule) Match(body []byte, ct string) bool {
    bodyStr := string(body)
    for _, kw := range r.Keywords {
        if strings.Contains(bodyStr, kw) {
            return true
        }
    }
    return false
}
```

### 性能考虑
- **流式读取**: 使用 `io.TeeReader` 复制响应前 4KB 到缓冲区进行检测
- **早期退出**: 一旦匹配任意规则,立即停止检测并触发重试

### 替代方案
- **机器学习模型**: 过度设计,简单规则已能覆盖 95% 场景
- **完整响应解析**: 性能开销大,且大部分错误在响应开头即可识别

---

## 7. 日志轮转与自动清理

### 决策
- **文件日志轮转**: 使用 **Lumberjack** 库,单文件 100MB,保留 10 个
- **数据库日志清理**: 使用 **定时任务 (Cron)** 每日凌晨 3 点执行,删除 30 天前记录并 VACUUM

### 理由
- **Lumberjack**: Slog 官方推荐,成熟稳定,零配置即可使用
- **Cron 调度**: Go 标准库不提供,使用轻量级 `robfig/cron` 库 (单一依赖)
- **VACUUM 必要性**: SQLite 删除记录后不自动释放空间,需手动压缩

### 实现要点
```go
// Lumberjack 配置
logger := &lumberjack.Logger{
    Filename:   "/var/log/hydra/debug.log",
    MaxSize:    100, // MB
    MaxBackups: 10,
    MaxAge:     0,   // 不按天数删除,仅按文件数
    Compress:   true,
}

// Cron 定时清理
c := cron.New()
c.AddFunc("0 3 * * *", func() {
    db.Exec("DELETE FROM request_logs WHERE created_at < ?", time.Now().AddDate(0, 0, -30))
    db.Exec("VACUUM") // SQLite only
})
c.Start()
```

### 替代方案
- **标准库 log/slog 内置轮转**: 不支持,需自行实现
- **系统 logrotate**: 依赖外部工具,不符合单文件分发原则

---

## 8. 会话管理与 CSRF 防护

### 决策
使用 **Gin Session 中间件** (`gin-contrib/sessions`) 管理管理后台登录会话,结合 **SameSite Cookie** 防止 CSRF。

### 理由
- **规格要求**: "基于会话的用户名/密码登录"
- **Gin 生态**: 官方维护,与 Gin 深度集成
- **安全性**: 支持内存/Redis/Cookie 存储,SameSite=Lax 防御 CSRF

### 会话配置
```go
store := cookie.NewStore([]byte("secret-key"))
store.Options(sessions.Options{
    Path:     "/admin",
    MaxAge:   3600 * 24, // 1 天
    HttpOnly: true,
    Secure:   true,      // 生产环境强制 HTTPS
    SameSite: http.SameSiteLaxMode,
})
r.Use(sessions.Sessions("hydra_session", store))
```

### CSRF 防护策略
- **SameSite Cookie**: 浏览器级别防护,阻止跨站请求携带 Cookie
- **Origin/Referer 检查**: 服务端验证请求来源
- **Token 机制 (可选)**: 如需更强防护,可在 POST 请求中嵌入 CSRF Token

### 替代方案
- **JWT**: 适合无状态 API,但管理后台需要会话(登出、过期控制)
- **自实现会话**: 复杂且易出安全漏洞

---

## 9. 配置管理策略

### 决策
使用 **YAML 配置文件** + **环境变量覆盖** 的混合策略。

### 理由
- **宪法要求**: "所有配置项必须可通过环境变量或配置文件覆盖"
- **开发友好**: YAML 易读易写,适合本地开发
- **部署灵活**: 环境变量适合容器化部署(Docker/Kubernetes)

### 配置优先级
```
环境变量 > 配置文件 > 默认值
```

### 实现示例 (使用 viper)
```go
viper.SetConfigName("config")
viper.AddConfigPath("./configs")
viper.AutomaticEnv() // 自动读取环境变量

// 环境变量映射: DATABASE_TYPE → database.type
viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
```

### 配置文件结构
```yaml
server:
  port: 8080
  admin_port: 8081

database:
  type: sqlite
  sqlite_path: ./hydra.db
  postgres_dsn: ""

circuit_breaker:
  failure_threshold: 3
  cooling_duration: 60s
  max_retry: 3

log:
  level: info
  retention_days: 30
```

### 替代方案
- **仅环境变量**: 配置项过多时难以管理
- **仅配置文件**: 容器化部署不便

---

## 10. 前端状态管理

### 决策
使用 **Pinia** 作为 Vue 3 状态管理库。

### 理由
- **Vue 3 官方推荐**: Pinia 是 Vuex 的继任者,更轻量级
- **TypeScript 友好**: 完全类型安全,无需额外类型定义
- **模块化**: 每个 store 独立,易于维护

### Store 设计
```typescript
// stores/auth.ts
export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as User | null,
    isAuthenticated: false,
  }),
  actions: {
    async login(username: string, password: string) {
      const res = await api.post('/admin/api/auth/login', { username, password })
      this.user = res.data.user
      this.isAuthenticated = true
    },
    logout() {
      this.user = null
      this.isAuthenticated = false
    },
  },
})
```

### 替代方案
- **Vuex**: 冗长,Vue 3 不再推荐
- **组合式 API + Provide/Inject**: 适合小型项目,大型项目需 Pinia 的持久化等功能

---

## 11. 实时数据更新策略 (仪表盘)

### 决策
使用 **轮询 (Polling)** 方式定期拉取仪表盘数据,间隔 5 秒。

### 理由
- **简单可靠**: 无需 WebSocket 或 SSE,降低复杂度
- **性能可接受**: 5 秒轮询,QPS 影响 < 0.2/s
- **符合规格**: SC-009 要求延迟 < 5 秒,轮询满足

### 实现方式
```typescript
// Dashboard.vue
onMounted(() => {
  const interval = setInterval(async () => {
    const data = await dashboardAPI.getMetrics()
    qpsData.value = data.qps
    successRate.value = data.success_rate
  }, 5000)

  onUnmounted(() => clearInterval(interval))
})
```

### 替代方案
- **WebSocket**: 实时性更好,但增加复杂度(连接管理、心跳)
- **SSE**: 单向推送,适合通知,但仪表盘需拉取历史数据

---

## 12. 测试策略

### 决策
- **单元测试**: 使用 Go 标准库 `testing` + `testify/assert`
- **集成测试**: 使用 `httptest` 模拟 HTTP 请求,真实数据库 (SQLite in-memory)
- **前端测试**: 使用 Vitest (Vite 原生测试框架)

### 理由
- **最小化依赖**: 标准库 + testify 已足够
- **真实环境**: 集成测试使用真实数据库,确保 SQL 正确性
- **快速反馈**: 内存 SQLite 测试速度快

### 测试覆盖目标
- **核心逻辑**: 嗅探器、熔断器、Key 池 → 单元测试覆盖率 > 80%
- **关键流程**: 代理转发、模型同步、日志查询 → 集成测试覆盖所有用户故事

### 替代方案
- **Mock 数据库**: 无法测试真实 SQL,容易漏测
- **E2E 测试**: 成本高,MVP 阶段不必要

---

## 总结

所有关键技术决策已完成,无 `NEEDS CLARIFICATION` 遗留项。技术栈选择严格遵循宪法约束,实现路径清晰可行。

**下一步**: 进入 Phase 1 设计阶段,生成数据模型、API 契约和快速开始指南。
