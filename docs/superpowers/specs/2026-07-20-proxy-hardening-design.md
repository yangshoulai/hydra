# Hydra 代理可靠性与配置语义硬化设计

日期：2026-07-20  
目标版本基线：2.2.4  
范围：`max_retry` 文案、401/403 故障分类、端点类型结构化、熔断 half-open、协议化健康探测、请求级总预算、Token 与路由缓存。

## 1. 背景与目标

Hydra 2.2.4 的代理主链路已经具备模型映射、重试、流式嗅探、请求日志和基础熔断能力。本次改动聚焦于把现有能力从“可用”提升到“语义更清晰、恢复更稳、路由更准确、热路径更轻”。

目标：

1. 修正 `proxy_max_retry` 的 UI/日志语义，避免“重试次数”和“总尝试次数”混淆。
2. 澄清 401/403 故障分类，区分永久 Key 故障与临时授权/配额类故障。
3. 迁移 `endpoint_types` 为结构化关联表，移除 JSON 文本 `LIKE` 匹配。
4. 熔断器增加 half-open 探测，避免冷却结束后全量流量直接打回故障上游。
5. 健康探测按端点协议构造真实探测请求，而不是只固定 `/v1/models`。
6. 增加一次代理请求的总预算，覆盖所有重试。
7. 增加 AccessToken 缓存与路由缓存，降低高频请求的 SQLite 热路径压力。

非目标：

- 不改访问令牌和渠道 Key 的加密存储；该问题另开安全专项更合适。
- 不引入 Prometheus、OTel、多实例外部数据库或成本账单。
- 不大改设置页信息架构，只做本次新增字段和文案修正。

## 2. 方案选择

采用“结构化迁移 + 小步增强”的方案。

保留 `channel_model_configs.endpoint_types` JSON 字段作为前端展示与兼容字段，同时新增规范化表：

```text
channel_model_config_endpoint_types
- id
- channel_model_config_id
- endpoint_type
- created_at
- updated_at
unique(channel_model_config_id, endpoint_type)
index(endpoint_type)
```

后端创建/更新模型配置时双写 JSON 字段和关联表；路由查询、模型列表查询、模型可用性检查改走关联表。这样既能消除误匹配，又避免一次性重构前端数据结构。

## 3. 详细设计

### 3.1 `max_retry` 语义修正

保持配置键 `proxy_max_retry` 不变，避免破坏已有数据库配置。

现有行为保留：`proxy_max_retry = N` 表示失败后最多再调度 N 次重试；实际总尝试次数最多为 `N + 1`。实现上如果第一次尝试失败，`AttemptCount` 递增后判断是否还能继续。

调整内容：

- UI 文案改为：“单次请求最多尝试 N 个上游路由；0 表示失败后不再重试，仍会进行首次尝试。”
- 后端配置日志继续输出 `max_retry`，同时补充派生字段 `max_route_attempts = max(1, max_retry)`。
- 请求日志字段保持 `retry_count` 与 `route_attempts` 不变。

### 3.2 401/403 故障分类

新增基于响应体的 HTTP 错误分类入口：

```go
ClassifyHTTPErrorWithBody(statusCode int, body []byte, contentType string) (FailureType, FailureScope, string)
```

分类规则：

- `401/403` 且响应体命中永久凭据错误关键词：`FailureTypeHard + FailureScopeKey`
  - 示例：`invalid api key`、`incorrect api key`、`authentication failed`、`permission denied`、`api key not valid`。
- `401/402/403/429` 且响应体命中额度/限流/账单关键词：`FailureTypeSoft + FailureScopeKey`
  - 示例：`quota`、`rate limit`、`billing`、`insufficient quota`、`insufficient funds`。
- `401/402/403` 无法判定：默认 `FailureTypeSoft + FailureScopeKey`
  - 原因：避免因为代理、区域、组织权限等临时策略误停 Key。
- 其它状态码维持现有策略：400 不熔断，404 归因模型配置，5xx 归因模型配置。

非流式和调试可读取响应体的失败分支使用新分类入口；无法读取 body 的场景继续使用旧状态码分类作为兜底。

### 3.3 endpoint type 结构化迁移

新增模型：

```go
type ChannelModelConfigEndpointType struct {
    ID                   uint
    ChannelModelConfigID uint
    EndpointType         string
    CreatedAt            time.Time
    UpdatedAt            time.Time
}
```

迁移步骤：

1. `AutoMigrate` 新表。
2. 扫描所有 `channel_model_configs`。
3. 解析原 `endpoint_types` JSON。
4. 空值或非法 JSON 回退为 `OpenAIChatCompletions`。
5. 去重后写入关联表，冲突忽略。

查询调整：

- `ChannelRepository.FindByModel`：通过关联表匹配 `endpoint_type = ?`。
- `ChannelModelConfigRepository.ExistsActiveModel`：通过关联表精确匹配。
- `ModelRepository.ListWithActiveChannelConfigsByEndpointType`：通过关联表精确匹配。

写入调整：

- 创建模型配置后同步 endpoint type 关联表。
- 更新模型配置时，如果 `EndpointTypes` 有变化，则删除旧关联并重建。
- 删除模型配置时级联或显式删除关联。

### 3.4 half-open 熔断

Key 与模型配置熔断器新增 `half_open` 状态，并增加单探测锁。

行为：

1. `active`：正常可用。
2. `cooling`：冷却未到期不可用。
3. `cooling` 到期：允许一个请求进入 `half_open`。
4. `half_open`：
   - 成功：恢复 `active`，失败计数清零。
   - 失败：重新进入 `cooling`，刷新 `lastFailure`。
   - 其他并发请求不可用，避免冷却结束瞬间打爆坏上游。
5. `inactive`：仅人工恢复或硬故障恢复接口可处理，不自动放行。

记录成功/失败沿用现有调用点：

- 成功路径调用 `RecordKeySuccess` / `RecordModelConfigSuccess`。
- 失败路径调用对应 soft/hard failure。

快照接口显示 `half_open`，前端可复用现有熔断状态表展示；若 UI 没有专门样式，先按 warning 处理。

### 3.5 协议化健康探测

`ProbeHandler` 不再只固定 `GET {base}/v1/models`。

探测选择：

1. 优先从渠道 active 模型配置中选择一个可用于该 Key 分组的配置。
2. 根据配置的第一个 endpoint type 获取 endpoint 实例。
3. 使用 endpoint 的 `ConfigureTestRequest` 构造真实测试请求。
4. Gemini 使用 `/v1beta/models/{model}:generateContent` 并带上 `key` / `X-Goog-Api-Key`。
5. Anthropic 使用 `/v1/messages` 并带上 `X-Api-Key` / `Anthropic-Version`。
6. 如果渠道没有可用模型配置，回退旧逻辑 `GET {base}/v1/models`，兼容 OpenAI 风格渠道的基础 Key 检查。

健康探测结果仍只用于管理端手动测试，不影响代理主链路的自动路由。

### 3.6 请求级总预算

新增系统设置：

```text
proxy_total_timeout
```

语义：

- `0`：不限制整体请求生命周期，保持当前默认。
- `>0`：一次代理请求从进入 `ProxyRequest` 到最终成功/失败的总 deadline，包含所有路由、上游调用、退避等待和重试。

实现：

- `ProxyServiceConfig` 增加 `TotalTimeout`。
- `ProxyContext` 持有 `BudgetDeadline` 或派生 context。
- 进入代理后，如果设置了总预算，为请求创建 `context.WithTimeout`。
- 路由、上游请求、重试等待全部使用该 context。
- 重试前如果预算耗尽，停止重试并返回 502 或 504。推荐使用 504，并在 error body 中说明 `request total timeout exceeded`。

UI：

- 设置页新增“请求总预算”。
- 文案：“限制一次代理请求包含所有重试在内的总耗时，0 表示不限制；流式请求建议保持 0。”

### 3.7 AccessToken 缓存

新增轻量缓存组件，供代理鉴权中间件使用。

缓存键：

```text
sha256(token)
```

缓存值：

- token ID
- token name
- status
- expires_at
- allowed_models

TTL：

- 默认 30 秒。
- 如果 token 自身过期时间早于 TTL，按过期时间截断。

失效：

- 创建 token 后可不处理，因为新 token 首次访问不会命中旧缓存。
- 删除、启停、更新模型权限时主动 invalidate。
- 兜底 TTL 自动过期。

实现边界：

- 不缓存明文 token。
- 不改变 `last_used_at` 合并更新逻辑。

### 3.8 路由缓存

新增路由缓存组件，靠近 `ChannelSelector` 或 `LoadBalancer`。

缓存 key：

```text
model + endpoint_type
```

缓存值：

- 满足统一模型、endpoint type、渠道 active、模型配置 active、Key active 的候选渠道集合。

缓存不包含：

- 当前请求失败排除集合。
- 熔断运行时状态。

原因：排除集合与熔断状态是请求级/内存级动态状态，应在读取缓存后继续过滤。

TTL：

- 默认 10 秒。

主动失效：

- 渠道创建、更新、删除、启停。
- 渠道 Key 创建、删除、启停。
- 渠道模型配置创建、更新、删除、启停。
- endpoint type 迁移完成后自然从空缓存开始。

## 4. 数据流

代理请求主流程：

1. 鉴权中间件从 AccessToken 缓存查 token。
2. 缓存未命中时查库，校验后写缓存。
3. `ProxyRequest` 创建总预算 context。
4. `prepareRequest` 解析模型并用结构化 endpoint type 表确认可用。
5. `LoadBalancer` 从路由缓存取候选。
6. 请求级失败排除和熔断状态过滤在缓存结果上执行。
7. 上游调用和 retry 使用总预算 context。
8. 响应失败时，带 body 的分类器判断故障类型。
9. 熔断器按 soft/hard、key/model 记录状态；half-open 成功或失败由现有成功/失败路径驱动。

## 5. 错误处理

- endpoint type 迁移遇到非法 JSON：记录 warning，回退默认端点，继续迁移。
- 关联表写入失败：返回错误，避免模型配置 JSON 与关联表长期不一致。
- half-open 并发探测：只有第一个请求放行，其他请求视为不可用并尝试其它候选路由。
- 请求总预算耗尽：停止重试，返回明确错误，避免继续等待。
- Token 缓存反序列化或状态异常：丢弃缓存，回源数据库。

## 6. 测试计划

后端测试：

1. endpoint type 迁移：
   - 多 endpoint type JSON 正确拆入关联表。
   - 空值/非法 JSON 回退默认端点。
2. 精确匹配：
   - `Gemini` 不因子串匹配命中其它 endpoint type。
   - `OpenAIResponses` 与 `OpenAIChatCompletions` 不互相误匹配。
3. 401/403 分类：
   - invalid key → hard key。
   - quota/rate limit/billing → soft key。
   - unknown 403 → soft key。
4. half-open：
   - cooling 未到期不可用。
   - cooling 到期只放一个探测。
   - 探测成功恢复 active。
   - 探测失败重新 cooling。
5. 总预算：
   - 预算耗尽时停止 retry。
   - 预算为 0 时保持现有行为。
6. 缓存：
   - AccessToken 二次鉴权命中缓存。
   - token 删除/停用/权限更新后缓存失效。
   - 路由缓存命中后仍受熔断和请求级排除影响。

最终验证：

```bash
cd backend && go test ./...
pnpm --dir frontend exec vue-tsc -b --pretty false
```

## 7. 风险与回滚

主要风险：

- endpoint type 双写不一致导致路由缺失。
- half-open 状态机边界处理不当导致可用候选被错误过滤。
- 缓存失效漏接导致管理端改动短时间不生效。

缓解：

- 关联表写入失败时直接返回错误，不静默降级。
- 路由缓存保留短 TTL，即使漏失效也会自动恢复。
- half-open 增加单元测试覆盖状态迁移。
- `proxy_total_timeout` 默认 0，不改变现有流式默认行为。

回滚：

- 代码回滚后旧 JSON 字段仍存在，旧版本可继续工作。
- 新增关联表不会影响旧代码读取。
