package endpoint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// MessagesEndpoint Anthropic Messages 端点
type MessagesEndpoint struct{}

func (e *MessagesEndpoint) GetName() string {
	return "Anthropic Messages"
}

func (e *MessagesEndpoint) GetType() string {
	return TypeAnthropicMessages
}

func (e *MessagesEndpoint) GetPath() string {
	return "/v1/messages"
}

func (e *MessagesEndpoint) GetDescription() string {
	return "Anthropic Messages API"
}

func (e *MessagesEndpoint) GetColor() string {
	return "#f59e0b"
}

func (e *MessagesEndpoint) ConfigureTestRequest(req *http.Request, apiKey string, modelName string) error {
	payload := map[string]any{
		"model": modelName,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": "请告诉我你的知识库的截止日期是哪一天？",
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Anthropic-Version", "2023-06-01")
	req.Header.Set("X-Api-Key", apiKey)
	req.Body = io.NopCloser(bytes.NewBuffer(data))
	req.ContentLength = int64(len(data))
	return nil
}

func (e *MessagesEndpoint) ValidateResponse(statusCode int, body []byte) (bool, string) {
	if statusCode != http.StatusOK {
		return false, "non-200 status code"
	}

	result, err := ValidateJSONResponse(body)
	if err != nil {
		return false, "invalid JSON response"
	}

	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		if errMsg, ok := result["error"]; ok {
			errBytes, _ := json.Marshal(errMsg)
			return false, fmt.Sprintf("上游渠道异常: %s", string(errBytes))
		}
		responseBody, _ := json.Marshal(result)
		return false, fmt.Sprintf("非法的响应报文: no content (response: %s)", string(responseBody))
	}

	return true, ""
}

func (e *MessagesEndpoint) ParseTokenUsage(_ []byte, responseBody string, isStream bool) (int64, int64) {
	if isStream {
		return parseTokenUsageFromStream(responseBody, "input_tokens", "output_tokens")
	}
	return parseTokenUsageFromJSON(responseBody, "input_tokens", "output_tokens")
}

func (e *MessagesEndpoint) ConfigureRequest(req *http.Request, apiKey string, modelName string, requestBody []byte) ([]byte, error) {
	// Anthropic 端点的特定配置
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Anthropic-Version", "2023-06-01")
	req.Header.Set("X-Api-Key", apiKey)
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return replaceRequestModel(requestBody, req.Header.Get("Content-Type"), modelName)
}

func (e *MessagesEndpoint) GetModelFromRequest(_ *http.Request, body []byte) (string, error) {
	return GetModelFromJSONBody(body)
}
