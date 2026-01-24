package endpoint

import (
	"encoding/json"
	"strings"
)

// parseTokenUsageFromJSON 解析非流式响应中的 token 使用量
func parseTokenUsageFromJSON(responseBody string, inputKey, outputKey string) (int64, int64) {
	var payload interface{}
	if err := json.Unmarshal([]byte(responseBody), &payload); err != nil {
		return 0, 0
	}

	usage := findUsageMap(payload)
	if usage == nil {
		return 0, 0
	}

	promptTokens, completionTokens, ok := extractTokensFromUsage(usage, inputKey, outputKey)
	if !ok {
		return 0, 0
	}
	return promptTokens, completionTokens
}

// parseTokenUsageFromStream 解析流式响应中的 token 使用量
func parseTokenUsageFromStream(responseBody string, inputKey, outputKey string) (int64, int64) {
	if responseBody == "" {
		return 0, 0
	}

	var lastPromptTokens int64
	var lastCompletionTokens int64
	lines := strings.Split(responseBody, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var payload interface{}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			continue
		}

		usage := findUsageMap(payload)
		if usage == nil {
			continue
		}

		promptTokens, completionTokens, ok := extractTokensFromUsage(usage, inputKey, outputKey)
		if !ok {
			continue
		}

		lastPromptTokens = promptTokens
		lastCompletionTokens = completionTokens
	}

	return lastPromptTokens, lastCompletionTokens
}

// findUsageMap 在响应中递归查找 usage 字段
func findUsageMap(payload interface{}) map[string]interface{} {
	switch value := payload.(type) {
	case map[string]interface{}:
		if usage, ok := value["usage"].(map[string]interface{}); ok {
			return usage
		}
		for _, nested := range value {
			if found := findUsageMap(nested); found != nil {
				return found
			}
		}
	case []interface{}:
		for _, item := range value {
			if found := findUsageMap(item); found != nil {
				return found
			}
		}
	}
	return nil
}

// extractTokensFromUsage 从 usage 中提取输入输出 tokens
func extractTokensFromUsage(usage map[string]interface{}, inputKey, outputKey string) (int64, int64, bool) {
	inputTokens, inputOk := parseUsageValue(usage, inputKey)
	outputTokens, outputOk := parseUsageValue(usage, outputKey)
	if !inputOk && !outputOk {
		return 0, 0, false
	}

	if !inputOk {
		inputTokens = 0
	}
	if !outputOk {
		outputTokens = 0
	}

	return inputTokens, outputTokens, true
}

// parseNumericToInt64 将数值类型转换为 int64
func parseNumericToInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

func parseUsageValue(usage map[string]interface{}, key string) (int64, bool) {
	value, ok := usage[key]
	if !ok {
		return 0, false
	}
	return parseNumericToInt64(value)
}
