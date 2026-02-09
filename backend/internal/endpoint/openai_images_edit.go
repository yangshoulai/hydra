package endpoint

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
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
		var dst io.Writer
		if part.FileName() != "" {
			dst, err = writer.CreateFormFile(part.FormName(), part.FileName())
		} else {
			dst, err = writer.CreateFormField(part.FormName())
		}
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
