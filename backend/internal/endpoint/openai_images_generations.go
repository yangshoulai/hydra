package endpoint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ImagesGenerationsEndpoint OpenAI Images Generations 端点
type ImagesGenerationsEndpoint struct{}

func (e *ImagesGenerationsEndpoint) GetName() string {
	return "OpenAI Images Generations"
}

func (e *ImagesGenerationsEndpoint) GetType() string {
	return "openai-image"
}

func (e *ImagesGenerationsEndpoint) GetPath() string {
	return "/v1/images/generations"
}

func (e *ImagesGenerationsEndpoint) GetDescription() string {
	return "OpenAI Images Generations API"
}

func (e *ImagesGenerationsEndpoint) GetColor() string {
	return "#ec4899"
}

func (e *ImagesGenerationsEndpoint) ConfigureTestRequest(req *http.Request, apiKey string, modelName string) error {
	payload := map[string]interface{}{
		"model":  modelName,
		"prompt": "请生成一只戴着耳机的柯基",
		"n":      1,
		"size":   "512x512",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Body = io.NopCloser(bytes.NewBuffer(data))
	req.ContentLength = int64(len(data))
	return nil
}

func (e *ImagesGenerationsEndpoint) ValidateResponse(statusCode int, body []byte) (bool, string) {
	if !IsSuccessStatusCode(statusCode) {
		return false, "non-success status code"
	}

	result, err := ValidateJSONResponse(body)
	if err != nil {
		return false, "invalid JSON response"
	}

	data, ok := result["data"].([]interface{})
	if !ok || len(data) == 0 {
		if errMsg, ok := result["error"]; ok {
			errBytes, _ := json.Marshal(errMsg)
			return false, fmt.Sprintf("上游渠道异常: %s", string(errBytes))
		}
		responseBody, _ := json.Marshal(result)
		return false, fmt.Sprintf("非法的响应报文: no data (response: %s)", string(responseBody))
	}
	return true, ""
}

func (e *ImagesGenerationsEndpoint) ParseTokenUsage(_ []byte, _ string, _ bool) (int64, int64) {
	return 0, 0
}

func (e *ImagesGenerationsEndpoint) ConfigureRequest(req *http.Request, apiKey string, modelName string, requestBody []byte) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return replaceRequestModel(requestBody, req.Header.Get("Content-Type"), modelName)
}
