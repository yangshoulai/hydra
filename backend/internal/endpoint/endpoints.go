package endpoint

// init 注册所有端点
func init() {
	Register(&ChatCompletionsEndpoint{})
	Register(&ResponsesEndpoint{})
	Register(&ImagesGenerationsEndpoint{})
	Register(&MessagesEndpoint{})
	Register(&GeminiEndpoint{})
}
