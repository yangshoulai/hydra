package endpoint

// DefaultEndpoints 显式返回系统内置端点列表。
// 新增端点时在这里登记，避免依赖 init() 隐式副作用。
func DefaultEndpoints() []Endpoint {
	return []Endpoint{
		&ChatCompletionsEndpoint{},
		&ResponsesEndpoint{},
		&ImagesGenerationsEndpoint{},
		&ImagesEditEndpoint{},
		&MessagesEndpoint{},
		&GeminiEndpoint{},
	}
}
