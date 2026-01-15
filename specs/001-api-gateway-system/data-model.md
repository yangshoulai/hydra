# Data Model: Hydra 高可用大模型聚合网关

**日期**: 2026-01-12
**功能**: 001-api-gateway-system
**目的**: 定义系统核心数据实体、字段、关系和验证规则

---

## 实体概览

系统包含以下核心实体:

1. **Channel (渠道)**: 上游大模型服务提供商
2. **Key (密钥)**: 归属于某个渠道的 API Key
3. **ChannelModelConfig (渠道模型配置)**: 渠道支持的模型及映射规则
4. **RequestLog (请求日志)**: 审计日志记录
5. **SystemSetting (系统配置)**: KV 格式的系统配置项
6. **AccessToken (访问令牌)**: 客户端访问代理接口的凭证
7. **AdminUser (管理员用户)**: 管理后台登录用户

---

## 1. Channel (渠道)

### 描述
代表一个上游大模型服务提供商,包含该渠道的基础信息和健康状态。

### 字段

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| `id` | bigint | PK, AUTO_INCREMENT | 渠道唯一标识 |
| `name` | varchar(100) | NOT NULL, UNIQUE | 渠道名称(如 "OpenAI Official") |
| `base_url` | varchar(500) | NOT NULL | 上游 API 基础 URL(如 "https://api.openai.com") |
| `priority` | int | NOT NULL, DEFAULT 100 | 优先级(数值越小优先级越高) |
| `weight` | int | NOT NULL, DEFAULT 100 | 负载均衡权重(用于同优先级渠道的流量分配) |
| `status` | enum | NOT NULL, DEFAULT 'Active' | 健康状态: Active(正常), Cooling(冷却中), Disabled(禁用) |
| `failure_count` | int | NOT NULL, DEFAULT 0 | 连续失败次数(用于熔断判断) |
| `last_failure_at` | timestamp | NULL | 最后一次失败时间 |
| `created_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| `updated_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

### 验证规则
- `name`: 长度 1-100,不包含特殊字符(仅字母、数字、空格、连字符)
- `base_url`: 必须是合法 HTTP/HTTPS URL
- `priority`: 范围 1-1000
- `weight`: 范围 1-1000
- `status`: 枚举值 `Active`, `Cooling`, `Disabled`

### 索引
- PRIMARY KEY: `id`
- UNIQUE KEY: `name`
- INDEX: `status` (用于查询可用渠道)

### 状态转换规则
- `Active` → `Cooling`: 连续失败次数达到阈值(默认 3 次)
- `Cooling` → `Active`: 冷却期结束且探测请求成功
- 任何状态 → `Disabled`: 管理员手动禁用

---

## 2. Key (密钥)

### 描述
归属于某个渠道的 API 密钥,支持 Key 池化管理。

### 字段

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| `id` | bigint | PK, AUTO_INCREMENT | 密钥唯一标识 |
| `channel_id` | bigint | NOT NULL, FK → Channel.id | 所属渠道 ID |
| `key_value` | varchar(500) | NOT NULL | API Key 值(如 "sk-xxx") |
| `status` | enum | NOT NULL, DEFAULT 'Active' | 健康状态: Active(正常), Dead(永久禁用), Cooling(冷却中), HalfOpen(半开) |
| `failure_count` | int | NOT NULL, DEFAULT 0 | 连续失败次数 |
| `last_failure_at` | timestamp | NULL | 最后一次失败时间 |
| `last_success_at` | timestamp | NULL | 最后一次成功时间 |
| `created_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| `updated_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

### 验证规则
- `key_value`: 长度 1-500,不能为空
- `status`: 枚举值 `Active`, `Dead`, `Cooling`, `HalfOpen`

### 索引
- PRIMARY KEY: `id`
- FOREIGN KEY: `channel_id` REFERENCES `Channel(id)` ON DELETE CASCADE
- INDEX: `channel_id, status` (用于查询渠道的可用 Key)

### 状态转换规则
- `Active` → `Dead`: 硬故障(401/402/403/429 quota exceeded)
- `Active` → `Cooling`: 软故障(5xx/timeout)连续 N 次
- `Cooling` → `HalfOpen`: 冷却期结束
- `HalfOpen` → `Active`: 探测请求成功
- `HalfOpen` → `Cooling`: 探测请求失败
- `Dead` → `Active`: 管理员手动重置

### 关系
- 多对一: 多个 Key 归属于一个 Channel

---

## 3. ChannelModelConfig (渠道模型配置)

### 描述
定义某渠道支持的模型及其重命名规则,支持将上游模型名映射为统一名称。

### 字段

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| `id` | bigint | PK, AUTO_INCREMENT | 配置唯一标识 |
| `channel_id` | bigint | NOT NULL, FK → Channel.id | 所属渠道 ID |
| `upstream_model` | varchar(200) | NOT NULL | 上游真实模型名(如 "vip-gpt4-0613") |
| `unified_model` | varchar(200) | NOT NULL | 统一映射名称(如 "gpt-4") |
| `enabled` | boolean | NOT NULL, DEFAULT true | 是否启用(禁用后不参与路由) |
| `weight` | int | NOT NULL, DEFAULT 100 | 权重(用于多渠道支持同一模型时的负载均衡) |
| `created_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| `updated_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

### 验证规则
- `upstream_model`: 长度 1-200
- `unified_model`: 长度 1-200
- `weight`: 范围 1-1000

### 索引
- PRIMARY KEY: `id`
- FOREIGN KEY: `channel_id` REFERENCES `Channel(id)` ON DELETE CASCADE
- UNIQUE KEY: `channel_id, upstream_model` (同一渠道的同一上游模型只能配置一次)
- INDEX: `unified_model, enabled` (用于查询支持某统一模型的所有渠道)

### 关系
- 多对一: 多个模型配置归属于一个 Channel

---

## 4. RequestLog (请求日志)

### 描述
审计日志记录,存储每个请求的元数据和结果。

### 字段

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| `id` | bigint | PK, AUTO_INCREMENT | 日志唯一标识 |
| `trace_id` | varchar(36) | NOT NULL, UNIQUE | 请求追踪 ID(UUID v4) |
| `request_time` | timestamp | NOT NULL | 请求到达时间 |
| `client_ip` | varchar(45) | NULL | 客户端 IP 地址(支持 IPv6) |
| `access_token_id` | bigint | NULL, FK → AccessToken.id | 使用的访问令牌 ID(如有) |
| `requested_model` | varchar(200) | NOT NULL | 用户请求的模型名 |
| `actual_model` | varchar(200) | NULL | 实际使用的上游模型名 |
| `channel_id` | bigint | NULL, FK → Channel.id | 使用的渠道 ID |
| `key_id` | bigint | NULL, FK → Key.id | 使用的密钥 ID |
| `status_code` | int | NOT NULL | HTTP 状态码 |
| `duration_ms` | int | NOT NULL | 请求耗时(毫秒) |
| `error_message` | text | NULL | 错误信息(如有) |
| `is_fake_success` | boolean | NOT NULL, DEFAULT false | 是否为假 200 响应 |
| `created_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP | 创建时间 |

### 验证规则
- `trace_id`: UUID v4 格式
- `client_ip`: 合法 IP 地址
- `status_code`: 范围 100-599
- `duration_ms`: >= 0

### 索引
- PRIMARY KEY: `id`
- UNIQUE KEY: `trace_id`
- INDEX: `request_time` (用于时间范围查询)
- INDEX: `status_code` (用于筛选成功/失败请求)
- INDEX: `channel_id` (用于按渠道筛选)
- FOREIGN KEY: `access_token_id` REFERENCES `AccessToken(id)` ON DELETE SET NULL
- FOREIGN KEY: `channel_id` REFERENCES `Channel(id)` ON DELETE SET NULL
- FOREIGN KEY: `key_id` REFERENCES `Key(id)` ON DELETE SET NULL

### 关系
- 多对一: 多个日志可能使用同一 AccessToken / Channel / Key

### 数据保留策略
- 自动清理超过配置天数(默认 30 天)的记录
- 清理后执行 VACUUM 压缩数据库

---

## 5. SystemSetting (系统配置)

### 描述
KV 格式的系统配置项,支持动态修改。

### 字段

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| `id` | bigint | PK, AUTO_INCREMENT | 配置唯一标识 |
| `key` | varchar(100) | NOT NULL, UNIQUE | 配置键(如 "log_retention_days") |
| `value` | text | NOT NULL | 配置值(JSON 格式) |
| `description` | text | NULL | 配置说明 |
| `created_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| `updated_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

### 验证规则
- `key`: 长度 1-100,仅包含小写字母、数字、下划线
- `value`: 合法 JSON 字符串

### 索引
- PRIMARY KEY: `id`
- UNIQUE KEY: `key`

### 预定义配置项

| Key | Value Type | Default | 说明 |
|-----|-----------|---------|------|
| `circuit_breaker.failure_threshold` | int | 3 | 熔断失败阈值 |
| `circuit_breaker.cooling_duration_sec` | int | 60 | 冷却时长(秒) |
| `circuit_breaker.max_retry` | int | 3 | 单个请求最大重试次数 |
| `log.retention_days` | int | 30 | 审计日志保留天数 |
| `log.debug_enabled` | bool | false | 是否启用调试日志(记录完整 Body) |
| `sniffer.error_keywords` | []string | ["无可用后端", ...] | 明文错误关键词列表 |

---

## 6. AccessToken (访问令牌)

### 描述
客户端访问代理接口的凭证。

### 字段

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| `id` | bigint | PK, AUTO_INCREMENT | 令牌唯一标识 |
| `name` | varchar(100) | NOT NULL | 令牌名称(用于标识用途) |
| `token_value` | varchar(500) | NOT NULL, UNIQUE | 令牌值(SHA256 哈希存储) |
| `enabled` | boolean | NOT NULL, DEFAULT true | 是否启用 |
| `last_used_at` | timestamp | NULL | 最后使用时间 |
| `created_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| `updated_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

### 验证规则
- `name`: 长度 1-100
- `token_value`: 长度 64(SHA256 哈希)
- **安全要求**: 创建时生成随机 Token(如 UUID),存储其 SHA256 哈希值,仅在创建时向用户显示明文 Token

### 索引
- PRIMARY KEY: `id`
- UNIQUE KEY: `token_value`
- INDEX: `enabled` (用于查询可用令牌)

---

## 7. AdminUser (管理员用户)

### 描述
管理后台登录用户。

### 字段

| 字段名 | 类型 | 约束 | 说明 |
|--------|------|------|------|
| `id` | bigint | PK, AUTO_INCREMENT | 用户唯一标识 |
| `username` | varchar(50) | NOT NULL, UNIQUE | 用户名 |
| `password_hash` | varchar(200) | NOT NULL | 密码哈希(bcrypt) |
| `created_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP | 创建时间 |
| `last_login_at` | timestamp | NULL | 最后登录时间 |
| `updated_at` | timestamp | NOT NULL, DEFAULT CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

### 验证规则
- `username`: 长度 3-50,仅包含字母、数字、下划线
- `password`: 创建时至少 8 位,包含字母和数字
- **安全要求**: 密码使用 bcrypt 哈希(cost=10),不存储明文

### 索引
- PRIMARY KEY: `id`
- UNIQUE KEY: `username`

### 初始化
- 系统首次启动时,如果 `AdminUser` 表为空,自动创建默认管理员:
  - `username`: `admin`
  - `password`: 随机生成并输出到日志(首次登录后强制修改)

---

## 实体关系图(ER Diagram)

```
┌─────────────────┐
│     Channel     │
│  id, name, ...  │
└────────┬────────┘
         │ 1
         │
         │ n
┌────────▼────────┐         ┌──────────────────────┐
│       Key       │         │ ChannelModelConfig   │
│ id, channel_id  │         │ id, channel_id, ...  │
└────────┬────────┘         └──────────────────────┘
         │ n                         │
         │                           │
         │ 1                         │
┌────────▼────────────────────────────▼───────┐
│              RequestLog                     │
│  id, trace_id, channel_id, key_id, ...     │
└────────────────┬────────────────────────────┘
                 │ n
                 │
                 │ 1
        ┌────────▼────────┐
        │   AccessToken   │
        │  id, name, ...  │
        └─────────────────┘

┌─────────────────┐
│  SystemSetting  │
│  id, key, ...   │
└─────────────────┘

┌─────────────────┐
│   AdminUser     │
│  id, username   │
└─────────────────┘
```

---

## 数据库迁移版本

使用 Gormigrate 管理数据库迁移,每个 Schema 变更对应一个版本:

- `v1.0.0`: 初始 Schema 创建(所有表)
- `v1.0.1`: (预留)添加新字段或索引

### Migration 文件命名规范
```
backend/internal/migration/
├── migration.go        # Gormigrate 配置
├── v1_0_0_init.go      # 初始 Schema
└── v1_0_1_*.go         # 后续变更
```

---

## 数据完整性约束

1. **级联删除**:
   - 删除 `Channel` 时,自动删除关联的 `Key` 和 `ChannelModelConfig`
   - 删除后 `RequestLog` 中的外键设为 NULL(保留历史记录)

2. **并发控制**:
   - `Channel` 和 `Key` 的状态更新使用乐观锁(通过 `updated_at` 字段)
   - 失败计数使用事务保证原子性

3. **数据一致性**:
   - `Key.channel_id` 必须引用存在的 `Channel`
   - `ChannelModelConfig.channel_id` 必须引用存在的 `Channel`
   - `RequestLog` 的外键允许 NULL(上游可能失败前未选择渠道)

---

## GORM 模型示例

```go
// Channel 渠道模型
type Channel struct {
    ID            uint      `gorm:"primaryKey"`
    Name          string    `gorm:"size:100;not null;unique"`
    BaseURL       string    `gorm:"size:500;not null"`
    Priority      int       `gorm:"not null;default:100"`
    Weight        int       `gorm:"not null;default:100"`
    Status        string    `gorm:"type:enum('Active','Cooling','Disabled');not null;default:'Active'"`
    FailureCount  int       `gorm:"not null;default:0"`
    LastFailureAt *time.Time
    CreatedAt     time.Time
    UpdatedAt     time.Time

    Keys                []Key                  `gorm:"foreignKey:ChannelID;constraint:OnDelete:CASCADE"`
    ChannelModelConfigs []ChannelModelConfig   `gorm:"foreignKey:ChannelID;constraint:OnDelete:CASCADE"`
}

// Key 密钥模型
type Key struct {
    ID            uint      `gorm:"primaryKey"`
    ChannelID     uint      `gorm:"not null;index:idx_channel_status"`
    KeyValue      string    `gorm:"size:500;not null"`
    Status        string    `gorm:"type:enum('Active','Dead','Cooling','HalfOpen');not null;default:'Active';index:idx_channel_status"`
    FailureCount  int       `gorm:"not null;default:0"`
    LastFailureAt *time.Time
    LastSuccessAt *time.Time
    CreatedAt     time.Time
    UpdatedAt     time.Time

    Channel Channel `gorm:"foreignKey:ChannelID"`
}

// ChannelModelConfig 渠道模型配置
type ChannelModelConfig struct {
    ID            uint      `gorm:"primaryKey"`
    ChannelID     uint      `gorm:"not null;uniqueIndex:idx_channel_upstream"`
    UpstreamModel string    `gorm:"size:200;not null;uniqueIndex:idx_channel_upstream"`
    UnifiedModel  string    `gorm:"size:200;not null;index:idx_unified_enabled"`
    Enabled       bool      `gorm:"not null;default:true;index:idx_unified_enabled"`
    Weight        int       `gorm:"not null;default:100"`
    CreatedAt     time.Time
    UpdatedAt     time.Time

    Channel Channel `gorm:"foreignKey:ChannelID"`
}

// RequestLog 请求日志
type RequestLog struct {
    ID              uint       `gorm:"primaryKey"`
    TraceID         string     `gorm:"size:36;not null;unique"`
    RequestTime     time.Time  `gorm:"not null;index"`
    ClientIP        string     `gorm:"size:45"`
    AccessTokenID   *uint      `gorm:"index"`
    RequestedModel  string     `gorm:"size:200;not null"`
    ActualModel     string     `gorm:"size:200"`
    ChannelID       *uint      `gorm:"index"`
    KeyID           *uint
    StatusCode      int        `gorm:"not null;index"`
    DurationMs      int        `gorm:"not null"`
    ErrorMessage    string     `gorm:"type:text"`
    IsFakeSuccess   bool       `gorm:"not null;default:false"`
    CreatedAt       time.Time

    AccessToken *AccessToken `gorm:"foreignKey:AccessTokenID;constraint:OnDelete:SET NULL"`
    Channel     *Channel     `gorm:"foreignKey:ChannelID;constraint:OnDelete:SET NULL"`
    Key         *Key         `gorm:"foreignKey:KeyID;constraint:OnDelete:SET NULL"`
}

// SystemSetting 系统配置
type SystemSetting struct {
    ID          uint      `gorm:"primaryKey"`
    Key         string    `gorm:"size:100;not null;unique"`
    Value       string    `gorm:"type:text;not null"`
    Description string    `gorm:"type:text"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// AccessToken 访问令牌
type AccessToken struct {
    ID         uint       `gorm:"primaryKey"`
    Name       string     `gorm:"size:100;not null"`
    TokenValue string     `gorm:"size:500;not null;unique"`
    Enabled    bool       `gorm:"not null;default:true;index"`
    LastUsedAt *time.Time
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

// AdminUser 管理员用户
type AdminUser struct {
    ID           uint       `gorm:"primaryKey"`
    Username     string     `gorm:"size:50;not null;unique"`
    PasswordHash string     `gorm:"size:200;not null"`
    CreatedAt    time.Time
    LastLoginAt  *time.Time
    UpdatedAt    time.Time
}
```

---

## 总结

数据模型设计完成,涵盖所有关键实体和关系。所有设计均符合功能规格和宪法约束,支持细粒度熔断、智能清洗和全链路追踪。
