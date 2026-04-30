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
	updatedBody, newContentType, err := replaceModelInMultipart(requestBody, req.Header.Get("Content-Type"), modelName)
	if err != nil {
		return requestBody, nil
	}
	req.Header.Set("Content-Type", newContentType)
	return updatedBody, nil
}

func (e *ImagesEditEndpoint) GetModelFromRequest(req *http.Request, body []byte) (string, error) {
	contentType := req.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		return "", errors.New("invalid content type for images/edits endpoint")
	}
	boundary := params["boundary"]
	if boundary == "" {
		return "", errors.New("missing boundary in content type")
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
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

	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}
		if part.FormName() == "model" {
			_ = part.Close()
			_ = writer.WriteField("model", modelName)
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

	_ = writer.Close()
	return buf.Bytes(), writer.FormDataContentType(), nil
}
