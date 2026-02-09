package endpoint

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

//go:embed assets/test.png
var testImage []byte

// ImagesEditEndpoint OpenAI Images Edit 端点
type ImagesEditEndpoint struct{}

func (e *ImagesEditEndpoint) GetName() string {
	return "OpenAI Images Edits"
}

func (e *ImagesEditEndpoint) GetType() string {
	return "openai-image-edit"
}

func (e *ImagesEditEndpoint) GetPath() string {
	return "/v1/images/edits"
}

func (e *ImagesEditEndpoint) GetDescription() string {
	return "OpenAI Images Edit API"
}

func (e *ImagesEditEndpoint) GetColor() string {
	return "#f472b6"
}

func (e *ImagesEditEndpoint) ConfigureTestRequest(req *http.Request, apiKey string, modelName string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	_ = writer.WriteField("model", modelName)
	_ = writer.WriteField("prompt", "请将图片中的背景替换为星空")

	part, err := writer.CreateFormFile("image", "test.png")
	if err != nil {
		return fmt.Errorf("创建测试图片字段异常: %w", err)
	}
	if _, err := part.Write(testImage); err != nil {
		return fmt.Errorf("写入测试图片异常: %w", err)
	}

	_ = writer.Close()
	data := buf.Bytes()
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Body = io.NopCloser(bytes.NewBuffer(data))
	req.ContentLength = int64(len(data))
	return nil
}

func (e *ImagesEditEndpoint) ValidateResponse(statusCode int, body []byte) (bool, string) {
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

func (e *ImagesEditEndpoint) ParseTokenUsage(_ []byte, _ string, _ bool) (int64, int64) {
	return 0, 0
}

func (e *ImagesEditEndpoint) ConfigureRequest(req *http.Request, apiKey string, modelName string, requestBody []byte) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "multipart/form-data")
	}
	return requestBody, nil
}
