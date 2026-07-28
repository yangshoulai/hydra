package endpoint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
)

// ImagesEditEndpoint OpenAI Images Edit 端点
type ImagesEditEndpoint struct{}

func (e *ImagesEditEndpoint) GetName() string {
	return "OpenAI Images Edits"
}

func (e *ImagesEditEndpoint) GetType() string {
	return TypeOpenAIImagesEdits
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
	req.Header.Set("Authorization", "Bearer "+apiKey)
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

	data, ok := result["data"].([]any)
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
	contentType := req.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("parse images edit content type: %w", err)
	}

	switch mediaType {
	case "application/json":
		return replaceRequestModel(requestBody, contentType, modelName)
	case "multipart/form-data":
		updatedBody, newContentType, err := replaceModelInMultipart(requestBody, contentType, modelName)
		if err != nil {
			return nil, fmt.Errorf("rewrite images edit model: %w", err)
		}
		req.Header.Set("Content-Type", newContentType)
		return updatedBody, nil
	default:
		return nil, fmt.Errorf("unsupported content type for images/edits endpoint: %s", mediaType)
	}
}

func (e *ImagesEditEndpoint) GetModelFromRequest(req *http.Request, body []byte) (string, error) {
	contentType := req.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", fmt.Errorf("parse images edit content type: %w", err)
	}

	if mediaType == "application/json" {
		return GetModelFromJSONBody(body)
	}
	if mediaType != "multipart/form-data" {
		return "", fmt.Errorf("unsupported content type for images/edits endpoint: %s", mediaType)
	}
	boundary := params["boundary"]
	if boundary == "" {
		return "", errors.New("missing boundary in content type")
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read images edit multipart: %w", err)
		}
		if part.FormName() == "model" {
			val, err := io.ReadAll(part)
			_ = part.Close()
			if err != nil {
				return "", err
			}
			model := string(val)
			if model == "" {
				return "", errors.New("model field is empty")
			}
			return model, nil
		}
		_ = part.Close()
	}
	return "", errors.New("model field is missing")
}

// replaceModelInMultipart 解析 multipart body，替换 model 字段，重建 body
func replaceModelInMultipart(body []byte, contentType string, modelName string) ([]byte, string, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		return nil, "", errors.New("invalid content type")
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, "", errors.New("missing boundary")
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	modelFound := false

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("read multipart part: %w", err)
		}
		if part.FormName() == "model" {
			_ = part.Close()
			if err := writer.WriteField("model", modelName); err != nil {
				return nil, "", err
			}
			modelFound = true
			continue
		}
		// 复制其他字段（包括文件字段）
		dst, err := writer.CreatePart(part.Header)
		if err != nil {
			_ = part.Close()
			return nil, "", err
		}
		_, err = io.Copy(dst, part)
		_ = part.Close()
		if err != nil {
			return nil, "", err
		}
	}

	if !modelFound {
		return nil, "", errors.New("model field is missing")
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}
