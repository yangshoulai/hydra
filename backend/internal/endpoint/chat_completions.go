package endpoint

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ChatCompletionsEndpoint OpenAI Chat Completions 端点
type ChatCompletionsEndpoint struct{}

func (e *ChatCompletionsEndpoint) GetName() string {
	return "OpenAI Chat Completions"
}

func (e *ChatCompletionsEndpoint) GetType() string {
	return "openai"
}

func (e *ChatCompletionsEndpoint) GetPath() string {
	return "/v1/chat/completions"
}

func (e *ChatCompletionsEndpoint) GetDescription() string {
	return "OpenAI Chat Completions API"
}

func (e *ChatCompletionsEndpoint) GetColor() string {
	return "#10b981"
}

func (e *ChatCompletionsEndpoint) GetTestPayload(modelName string) map[string]interface{} {
	return map[string]interface{}{
		"model": modelName,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "请告诉我你的知识库的截止日期是哪一天？",
			},
		},
		"max_tokens": 10,
	}
}

func (e *ChatCompletionsEndpoint) ValidateResponse(statusCode int, body []byte) (bool, string) {
	if !IsSuccessStatusCode(statusCode) {
		return false, "non-success status code"
	}

	result, err := ValidateJSONResponse(body)
	if err != nil {
		return false, "invalid JSON response"
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		if errMsg, ok := result["error"]; ok {
			errBytes, _ := json.Marshal(errMsg)
			return false, fmt.Sprintf("上游渠道异常: %s", string(errBytes))
		}
		responseBody, _ := json.Marshal(result)
		return false, fmt.Sprintf("非法的响应报文: no choices (response: %s)", string(responseBody))
	}
	return true, ""
}

func (e *ChatCompletionsEndpoint) ParseTokenUsage(_ []byte, responseBody string, isStream bool) (int64, int64) {
	if isStream {
		return parseTokenUsageFromStream(responseBody, "prompt_tokens", "completion_tokens")
	}
	return parseTokenUsageFromJSON(responseBody, "prompt_tokens", "completion_tokens")
}

func (e *ChatCompletionsEndpoint) ConfigureRequest(req *http.Request, apiKey string, modelName string, requestBody []byte) ([]byte, error) {
	// OpenAI 端点的标准配置
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	return replaceRequestModel(requestBody, req.Header.Get("Content-Type"), modelName)
}
