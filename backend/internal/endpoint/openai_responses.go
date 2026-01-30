package endpoint

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ResponsesEndpoint OpenAI Responses 端点
type ResponsesEndpoint struct{}

func (e *ResponsesEndpoint) GetName() string {
	return "OpenAI Responses"
}

func (e *ResponsesEndpoint) GetType() string {
	return "openai-response"
}

func (e *ResponsesEndpoint) GetPath() string {
	return "/v1/responses"
}

func (e *ResponsesEndpoint) GetDescription() string {
	return "OpenAI Responses API"
}

func (e *ResponsesEndpoint) GetColor() string {
	return "#3b82f6"
}

func (e *ResponsesEndpoint) GetTestPayload(modelName string) map[string]interface{} {
	return map[string]interface{}{
		"model": modelName,
		"input": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]string{
					{"type": "input_text", "text": "请告诉我你的知识库的截止日期是哪一天？"},
				},
			},
		},
	}
}

func (e *ResponsesEndpoint) ValidateResponse(statusCode int, body []byte) (bool, string) {
	if !IsSuccessStatusCode(statusCode) {
		return false, "non-success status code"
	}

	result, err := ValidateJSONResponse(body)
	if err != nil {
		return false, "invalid JSON response"
	}

	// OpenAI Response: 检查 output 字段
	if output, ok := result["output"].([]interface{}); !ok || len(output) <= 0 {
		// 有 output 字段
		if errMsg, ok := result["error"]; ok {
			errBytes, _ := json.Marshal(errMsg)
			return false, fmt.Sprintf("上游渠道异常: %s", string(errBytes))
		}
		responseBody, _ := json.Marshal(result)
		return false, fmt.Sprintf("非法的响应报文: no choices or output (response: %s)", string(responseBody))
	}
	return true, ""
}

func (e *ResponsesEndpoint) ParseTokenUsage(_ []byte, responseBody string, isStream bool) (int64, int64) {
	if isStream {
		return parseTokenUsageFromStream(responseBody, "input_tokens", "output_tokens")
	}
	return parseTokenUsageFromJSON(responseBody, "input_tokens", "output_tokens")
}

func (e *ResponsesEndpoint) ConfigureRequest(req *http.Request, apiKey string, modelName string, requestBody []byte) ([]byte, error) {
	// OpenAI Response 端点的标准配置
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return replaceRequestModel(requestBody, req.Header.Get("Content-Type"), modelName)
}
