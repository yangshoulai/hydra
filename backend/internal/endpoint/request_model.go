package endpoint

import (
	"encoding/json"
	"strings"
)

func replaceRequestModel(requestBody []byte, contentType string, modelName string) ([]byte, error) {
	if len(requestBody) == 0 {
		return requestBody, nil
	}
	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return requestBody, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		return nil, err
	}
	payload["model"] = modelName
	return json.Marshal(payload)
}
