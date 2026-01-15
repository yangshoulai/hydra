# Hydra API Gateway API 文档

## 目录

- [概述](#概述)
- [认证方式](#认证方式)
- [代理 API](#代理-api)
- [管理后台 API](#管理后台-api)
- [错误码说明](#错误码说明)

## 概述

Hydra API Gateway 提供 OpenAI 兼容的 API 代理服务，支持多渠道自动切换、熔断保护和负载均衡。

**Base URL**: `http://your-domain:8080`

## 认证方式

### API Token 认证

在请求头中添加 Authorization：

```http
Authorization: Bearer YOUR_ACCESS_TOKEN
```

或在 URL 参数中传递：

```
?access_token=YOUR_ACCESS_TOKEN
```

### 管理后台认证

管理后台使用 Session 认证，需要先登录获取 Session。

```http
POST /admin/api/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "admin123"
}
```

登录成功后，Cookie 会自动设置。

## 代理 API

### POST /v1/chat/completions

创建聊天完成请求。

**请求头**：

```http
Content-Type: application/json
Authorization: Bearer YOUR_ACCESS_TOKEN
```

**请求体**：

```json
{
  "model": "gpt-4",
  "messages": [
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ],
  "stream": false,
  "temperature": 0.7,
  "max_tokens": 2000
}
```

**参数说明**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称 |
| messages | array | 是 | 消息列表 |
| stream | boolean | 否 | 是否流式输出，默认 false |
| temperature | number | 否 | 温度参数，0-2 |
| max_tokens | integer | 否 | 最大生成 token 数 |
| top_p | number | 否 | nucleus sampling 参数 |

**响应**：

```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "gpt-4",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "I'm doing well, thank you!"
    },
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30
  }
}
```

### GET /v1/models

获取可用的模型列表。

**请求头**：

```http
Authorization: Bearer YOUR_ACCESS_TOKEN
```

**响应**：

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4",
      "object": "model",
      "owned_by": "hydra-gateway"
    },
    {
      "id": "gpt-3.5-turbo",
      "object": "model",
      "owned_by": "hydra-gateway"
    }
  ]
}
```

## 管理后台 API

### 认证相关

#### POST /admin/api/auth/login

管理员登录。

**请求体**：

```json
{
  "username": "admin",
  "password": "admin123"
}
```

**响应**：

```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": {
      "id": 1,
      "username": "admin",
      "status": "active"
    }
  }
}
```

#### POST /admin/api/auth/logout

管理员登出。

**响应**：

```json
{
  "message": "Logged out successfully"
}
```

#### GET /admin/api/auth/me

获取当前登录用户信息。

**响应**：

```json
{
  "data": {
    "id": 1,
    "username": "admin",
    "status": "active"
  }
}
```

### 渠道管理

#### GET /admin/api/channels

获取渠道列表。

**查询参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | integer | 否 | 页码，默认 1 |
| page_size | integer | 否 | 每页数量，默认 20 |

**响应**：

```json
{
  "data": [
    {
      "id": 1,
      "name": "OpenAI",
      "base_url": "https://api.openai.com/v1",
      "priority": 100,
      "weight": 100,
      "status": "active",
      "created_at": "2024-01-12T00:00:00Z",
      "keys": []
    }
  ],
  "total": 1
}
```

#### POST /admin/api/channels

创建新渠道。

**请求体**：

```json
{
  "name": "OpenAI",
  "base_url": "https://api.openai.com/v1",
  "priority": 100,
  "weight": 100,
  "description": "OpenAI API"
}
```

**响应**：

```json
{
  "data": {
    "id": 1,
    "name": "OpenAI",
    ...
  }
}
```

#### GET /admin/api/channels/:id

获取渠道详情。

**响应**：同创建渠道。

#### PUT /admin/api/channels/:id

更新渠道。

**请求体**：同创建渠道。

**响应**：

```json
{
  "message": "Channel updated successfully"
}
```

#### DELETE /admin/api/channels/:id

删除渠道。

**响应**：

```json
{
  "message": "Channel deleted successfully"
}
```

### Key 管理

#### POST /admin/api/keys

添加 Key 到渠道。

**请求体**：

```json
{
  "channel_id": 1,
  "key_value": "sk-xxxxxxxxxxxxxxxx",
  "remark": "Primary key"
}
```

**响应**：

```json
{
  "data": {
    "id": 1,
    "channel_id": 1,
    "key_value": "sk-...xxx",
    "status": "active",
    ...
  }
}
```

#### DELETE /admin/api/keys/:id

删除 Key。

**响应**：

```json
{
  "message": "Key deleted successfully"
}
```

#### PATCH /admin/api/keys/:id

重置 Key 状态。

**响应**：

```json
{
  "message": "Key status reset successfully"
}
```

### 模型同步

#### POST /admin/api/channels/:id/sync-models

同步渠道的可用模型。

**响应**：

```json
{
  "data": {
    "added": ["gpt-4-turbo"],
    "removed": ["gpt-3.5-turbo-old"],
    "existing": ["gpt-4"],
    "total": 3
  }
}
```

### 渠道测活

#### POST /admin/api/channels/:id/test-keys

测试渠道的所有 Key。

**响应**：

```json
{
  "data": {
    "total_keys": 3,
    "healthy_keys": 2,
    "unhealthy_keys": 1,
    "results": [
      {
        "key_id": 1,
        "status": "healthy",
        "response_time_ms": 250
      },
      {
        "key_id": 2,
        "status": "unhealthy",
        "error": "Invalid API key"
      }
    ]
  }
}
```

### 日志查询

#### GET /admin/api/logs

查询请求日志。

**查询参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| trace_id | string | 否 | Trace ID |
| access_token | string | 否 | 访问令牌 |
| requested_model | string | 否 | 请求的模型 |
| channel_id | integer | 否 | 渠道 ID |
| status_code | integer | 否 | HTTP 状态码 |
| start_time | string | 否 | 开始时间（ISO 8601） |
| end_time | string | 否 | 结束时间（ISO 8601） |
| page | integer | 否 | 页码，默认 1 |
| page_size | integer | 否 | 每页数量，默认 20 |

**响应**：

```json
{
  "data": [
    {
      "id": 1,
      "trace_id": "550e8400-e29b-41d4-a716-446655440000",
      "requested_model": "gpt-4",
      "channel_name": "OpenAI",
      "status_code": 200,
      "is_success": true,
      "response_time": 1250,
      "created_at": "2024-01-12T00:00:00Z"
    }
  ],
  "total": 100
}
```

#### GET /admin/api/logs/:traceId

根据 Trace ID 查询日志详情。

**响应**：

```json
{
  "data": {
    "id": 1,
    "trace_id": "550e8400-e29b-41d4-a716-446655440000",
    "request_path": "/v1/chat/completions",
    "request_method": "POST",
    "requested_model": "gpt-4",
    "channel_name": "OpenAI",
    "status_code": 200,
    "is_success": true,
    "response_time": 1250,
    "retry_count": 0,
    "is_stream": false,
    "error_message": "",
    "created_at": "2024-01-12T00:00:00Z"
  }
}
```

### 仪表盘

#### GET /admin/api/dashboard/metrics

获取仪表盘指标。

**响应**：

```json
{
  "data": {
    "current_qps": 15.5,
    "qps_time_series": [
      {
        "timestamp": "15:00",
        "qps": 12.3
      }
    ],
    "today_success_rate": {
      "total_requests": 1000,
      "success_requests": 950,
      "failed_requests": 50,
      "success_rate": 95.0
    },
    "overall_health": {
      "total_channels": 5,
      "healthy_channels": 4,
      "degraded_channels": 1,
      "unhealthy_channels": 0,
      "overall_health": 80.0
    },
    "channel_health_list": [
      {
        "channel_id": 1,
        "channel_name": "OpenAI",
        "status": "active",
        "health_percentage": 100.0,
        "total_keys": 5,
        "healthy_keys": 5,
        "success_rate": 98.5
      }
    ],
    "total_requests_today": 1000,
    "total_channels": 5,
    "total_keys": 25,
    "active_channels": 4,
    "generated_at": "2024-01-12T15:30:00Z"
  }
}
```

### 系统设置

#### GET /admin/api/settings

获取所有系统设置。

**响应**：

```json
{
  "data": {
    "circuit_breaker_failure_threshold": "3",
    "circuit_breaker_cooling_duration": "60",
    "proxy_max_retry": "3",
    "proxy_request_timeout": "60",
    "debug_mode": "false"
  }
}
```

#### PUT /admin/api/settings

批量更新系统设置。

**请求体**：

```json
{
  "settings": {
    "circuit_breaker_failure_threshold": "5",
    "debug_mode": "true"
  }
}
```

**响应**：

```json
{
  "message": "Settings updated successfully"
}
```

### 访问令牌管理

#### GET /admin/api/tokens

获取访问令牌列表。

**响应**：

```json
{
  "data": [
    {
      "id": 1,
      "remark": "Production token",
      "status": "active",
      "created_at": "2024-01-12T00:00:00Z",
      "last_used_at": "2024-01-12T15:30:00Z"
    }
  ]
}
```

#### POST /admin/api/tokens

创建新访问令牌。

**请求体**：

```json
{
  "remark": "New token"
}
```

**响应**：

```json
{
  "data": {
    "id": 2,
    "remark": "New token",
    "access_token": "hydra-base64encodedtoken",
    "created_at": "2024-01-12T15:30:00Z",
    "message": "Please save the access token securely. It will not be shown again."
  }
}
```

#### DELETE /admin/api/tokens/:id

删除访问令牌。

**响应**：

```json
{
  "message": "Token deleted successfully"
}
```

#### PATCH /admin/api/tokens/:id/toggle

切换令牌状态。

**响应**：

```json
{
  "message": "Token status updated successfully",
  "data": {
    "id": 1,
    "status": "disabled"
  }
}
```

## 错误码说明

### HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 429 | 请求过于频繁 |
| 500 | 服务器内部错误 |
| 503 | 服务不可用（所有渠道熔断） |

### 错误响应格式

```json
{
  "error": "Error type",
  "message": "Detailed error message",
  "details": {}
}
```

### 常见错误

#### 400 Bad Request

```json
{
  "error": "Invalid request",
  "message": "model field is required"
}
```

#### 401 Unauthorized

```json
{
  "error": "Unauthorized",
  "message": "Invalid or missing access token"
}
```

#### 503 Service Unavailable

```json
{
  "error": "Service unavailable",
  "message": "All channels are currently in circuit breaker state"
}
```

## 速率限制

代理 API 默认限制：
- 最大并发请求数：1000（可配置）
- 单个请求超时：60 秒（可配置）

## Webhook

暂不支持 Webhook 功能。

## SDK

暂无官方 SDK，但可以使用标准 OpenAI SDK，只需修改 Base URL：

```python
import openai

openai.api_base = "http://your-domain:8080"
openai.api_key = "your-access-token"

response = openai.ChatCompletion.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello!"}]
)
```
