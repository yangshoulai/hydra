package endpoint

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
)

func TestImagesEditEndpointSupportsJSONEnvelope(t *testing.T) {
	body := []byte(`{"model":"public-image","prompt":"edit","images":["data:image/png;base64,AAAA"]}`)
	req, err := http.NewRequest(http.MethodPost, "https://example.test/v1/images/edits", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	model, err := (&ImagesEditEndpoint{}).GetModelFromRequest(req, body)
	if err != nil {
		t.Fatalf("extract model: %v", err)
	}
	if model != "public-image" {
		t.Fatalf("unexpected model: %q", model)
	}

	updated, err := (&ImagesEditEndpoint{}).ConfigureRequest(req, "channel-key", "upstream-image", body)
	if err != nil {
		t.Fatalf("configure request: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(updated, &payload); err != nil {
		t.Fatalf("decode updated JSON: %v", err)
	}
	if payload["model"] != "upstream-image" {
		t.Fatalf("model was not replaced: %#v", payload["model"])
	}
	if payload["prompt"] != "edit" {
		t.Fatalf("prompt changed unexpectedly: %#v", payload["prompt"])
	}
}

func TestImagesEditEndpointRewritesMultipartAndPreservesImage(t *testing.T) {
	originalImage := []byte{0x89, 'P', 'N', 'G', 0x00, 0xff, 0x10}
	body, contentType := buildImageEditMultipart(t, "public-image", "keep identity", originalImage)
	req, err := http.NewRequest(http.MethodPost, "https://example.test/v1/images/edits", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", contentType)

	model, err := (&ImagesEditEndpoint{}).GetModelFromRequest(req, body)
	if err != nil {
		t.Fatalf("extract model: %v", err)
	}
	if model != "public-image" {
		t.Fatalf("unexpected model: %q", model)
	}

	updated, err := (&ImagesEditEndpoint{}).ConfigureRequest(req, "channel-key", "upstream-image", body)
	if err != nil {
		t.Fatalf("configure request: %v", err)
	}
	fields, image := readImageEditMultipart(t, updated, req.Header.Get("Content-Type"))
	if fields["model"] != "upstream-image" {
		t.Fatalf("model was not replaced: %q", fields["model"])
	}
	if fields["prompt"] != "keep identity" {
		t.Fatalf("prompt changed unexpectedly: %q", fields["prompt"])
	}
	if !bytes.Equal(image, originalImage) {
		t.Fatalf("image bytes changed: got %x want %x", image, originalImage)
	}
}

func TestImagesEditEndpointRejectsMalformedMultipart(t *testing.T) {
	body := []byte("--broken\r\n" +
		"Content-Disposition: form-data; name=\"model\"\r\n\r\n" +
		"public-image\r\n" +
		"--broken\r\n" +
		"not-a-valid-header\r\n\r\n")
	req, err := http.NewRequest(http.MethodPost, "https://example.test/v1/images/edits", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "multipart/form-data; boundary=broken")

	_, err = (&ImagesEditEndpoint{}).ConfigureRequest(req, "channel-key", "upstream-image", body)
	if err == nil || !strings.Contains(err.Error(), "read multipart part") {
		t.Fatalf("expected malformed multipart error, got %v", err)
	}
}

func buildImageEditMultipart(t *testing.T, model, prompt string, image []byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("model", model); err != nil {
		t.Fatalf("write model: %v", err)
	}
	if err := writer.WriteField("prompt", prompt); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="image"; filename="pet.png"`)
	header.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create image part: %v", err)
	}
	if _, err := part.Write(image); err != nil {
		t.Fatalf("write image: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return buf.Bytes(), writer.FormDataContentType()
}

func readImageEditMultipart(t *testing.T, body []byte, contentType string) (map[string]string, []byte) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("unexpected media type: %s", mediaType)
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	fields := make(map[string]string)
	var image []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part body: %v", err)
		}
		if part.FileName() != "" {
			image = data
		} else {
			fields[part.FormName()] = string(data)
		}
		_ = part.Close()
	}
	return fields, image
}
