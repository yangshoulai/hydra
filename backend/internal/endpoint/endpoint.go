package endpoint

import (
	"encoding/json"
	"net/http"
)

// Endpoint 端点接口定义
type Endpoint interface {
	// GetName 获取端点名称（用于显示）
	GetName() string

	// GetType 获取端点类型标识（用于数据库存储和匹配）
	GetType() string

	// GetPath 获取端点请求路径
	GetPath() string

	// GetTestPayload 获取测试报文
	// modelName: 实际的上游模型名称
	GetTestPayload(modelName string) map[string]interface{}

	// ValidateResponse 验证响应是否符合当前端点格式
	// 返回 (是否有效, 错误信息)
	ValidateResponse(statusCode int, body []byte) (bool, string)

	// ParseTokenUsage 解析响应中的 token 使用量
	// requestBody: 请求体（原始字节）
	// responseBody: 响应体字符串（已完整读取）
	// isStream: 是否为流式响应
	// 返回 (输入 tokens, 输出 tokens)，解析失败返回 0
	ParseTokenUsage(requestBody []byte, responseBody string, isStream bool) (int64, int64)

	// GetDescription 获取端点描述
	GetDescription() string

	// ConfigureRequest 配置请求（设置特定的请求头、请求体或 URL）
	// 在发送请求前调用，允许端点定制化配置
	// modelName: 上游模型名称
	// requestBody: 原始请求体
	// 返回新的请求体（如无需修改可返回原请求体）
	ConfigureRequest(req *http.Request, apiKey string, modelName string, requestBody []byte) ([]byte, error)

	// GetColor 获取端点颜色（用于前端展示）
	GetColor() string
}

// EndpointInfo 端点信息（用于API返回）
type EndpointInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// ToInfo 将 Endpoint 转换为 EndpointInfo
func ToInfo(ep Endpoint) EndpointInfo {
	return EndpointInfo{
		Name:        ep.GetName(),
		Type:        ep.GetType(),
		Path:        ep.GetPath(),
		Description: ep.GetDescription(),
		Color:       ep.GetColor(),
	}
}

// ValidateJSONResponse 通用的 JSON 响应验证辅助函数
func ValidateJSONResponse(body []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// IsSuccessStatusCode 判断状态码是否为成功
func IsSuccessStatusCode(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}
