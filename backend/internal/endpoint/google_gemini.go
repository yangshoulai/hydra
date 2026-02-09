package endpoint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// GeminiEndpoint Google Gemini 端点
type GeminiEndpoint struct{}

func (e *GeminiEndpoint) GetName() string {
	return "Google Gemini"
}

func (e *GeminiEndpoint) GetType() string {
	return "gemini"
}

func (e *GeminiEndpoint) GetPath() string {
	return "/v1beta/models/*model"
}

func (e *GeminiEndpoint) GetDescription() string {
	return "Google Gemini API"
}

func (e *GeminiEndpoint) GetColor() string {
	return "#22c55e"
}

func (e *GeminiEndpoint) ConfigureTestRequest(req *http.Request, apiKey string, modelName string) error {
	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": "请告诉我你的知识库的截止日期是哪一天？"},
				},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		query := req.URL.Query()
		if query.Get("key") == "" {
			query.Set("key", apiKey)
			req.URL.RawQuery = query.Encode()
		}
		req.Header.Set("X-Goog-Api-Key", apiKey)
	}
	action := extractGeminiAction(req.URL.Path)
	req.URL.Path = buildGeminiPath(modelName, action)
	req.Body = io.NopCloser(bytes.NewBuffer(data))
	req.ContentLength = int64(len(data))
	return nil
}

func (e *GeminiEndpoint) ValidateResponse(statusCode int, body []byte) (bool, string) {
	if !IsSuccessStatusCode(statusCode) {
		return false, "non-success status code"
	}

	result, err := ValidateJSONResponse(body)
	if err != nil {
		return false, "invalid JSON response"
	}

	candidates, ok := result["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		if errMsg, ok := result["error"]; ok {
			errBytes, _ := json.Marshal(errMsg)
			return false, fmt.Sprintf("上游渠道异常: %s", string(errBytes))
		}
		responseBody, _ := json.Marshal(result)
		return false, fmt.Sprintf("非法的响应报文: no candidates (response: %s)", string(responseBody))
	}
	return true, ""
}

func (e *GeminiEndpoint) ParseTokenUsage(_ []byte, responseBody string, isStream bool) (int64, int64) {
	if responseBody == "" {
		return 0, 0
	}
	if isStream {
		return parseGeminiTokenUsageFromStream(responseBody)
	}
	return parseGeminiTokenUsageFromJSON(responseBody)
}

func (e *GeminiEndpoint) ConfigureRequest(req *http.Request, apiKey string, modelName string, requestBody []byte) ([]byte, error) {
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		query := req.URL.Query()
		if query.Get("key") == "" {
			query.Set("key", apiKey)
			req.URL.RawQuery = query.Encode()
		}
		req.Header.Set("X-Goog-Api-Key", apiKey)
	}

	action := extractGeminiAction(req.URL.Path)
	req.URL.Path = buildGeminiPath(modelName, action)

	return requestBody, nil
}

func (e *GeminiEndpoint) GetModelFromRequest(req *http.Request, body []byte) (string, error) {
	// 先从 URL path 提取模型名
	model := extractGeminiModelPath(req.URL.Path)
	if model != "" {
		// 去掉 :action 部分
		parts := strings.SplitN(model, ":", 2)
		if parts[0] != "" {
			return parts[0], nil
		}
	}
	// 回退到 JSON body
	return GetModelFromJSONBody(body)
}

func parseGeminiTokenUsageFromJSON(responseBody string) (int64, int64) {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(responseBody), &payload); err != nil {
		return 0, 0
	}

	usage, ok := payload["usageMetadata"].(map[string]interface{})
	if !ok {
		return 0, 0
	}

	promptTokens, _ := parseNumericToInt64(usage["promptTokenCount"])
	completionTokens, _ := parseNumericToInt64(usage["candidatesTokenCount"])
	return promptTokens, completionTokens
}

func parseGeminiTokenUsageFromStream(responseBody string) (int64, int64) {
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

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			continue
		}

		usage, ok := payload["usageMetadata"].(map[string]interface{})
		if !ok {
			continue
		}

		promptTokens, okPrompt := parseNumericToInt64(usage["promptTokenCount"])
		completionTokens, okCompletion := parseNumericToInt64(usage["candidatesTokenCount"])
		if !okPrompt && !okCompletion {
			continue
		}
		if okPrompt {
			lastPromptTokens = promptTokens
		}
		if okCompletion {
			lastCompletionTokens = completionTokens
		}
	}

	return lastPromptTokens, lastCompletionTokens
}

func extractGeminiAction(path string) string {
	model := extractGeminiModelPath(path)
	if model == "" {
		return "generateContent"
	}

	parts := strings.SplitN(model, ":", 2)
	if len(parts) == 2 && parts[1] != "" {
		return parts[1]
	}
	return "generateContent"
}

func extractGeminiModelPath(path string) string {
	index := strings.Index(path, "/models/")
	if index < 0 {
		return ""
	}
	return strings.TrimPrefix(path[index:], "/models/")
}

func buildGeminiPath(modelName string, action string) string {
	if action == "" {
		action = "generateContent"
	}
	modelName = strings.TrimPrefix(modelName, "/")
	modelName = strings.TrimPrefix(modelName, "models/")
	escapedModel := url.PathEscape(modelName)
	return "/v1beta/models/" + escapedModel + ":" + action
}
