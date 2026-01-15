package proxy

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorHandler 统一错误处理器
type ErrorHandler struct{}

// NewErrorHandler 创建错误处理器
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{}
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// HandleServiceUnavailable 处理服务不可用错误 (503)
// 当所有渠道都不可用时调用
func (h *ErrorHandler) HandleServiceUnavailable(c *gin.Context, message string) {
	c.JSON(http.StatusServiceUnavailable, ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    "service_unavailable",
			Code:    "no_available_channels",
		},
	})
}

// HandleBadRequest 处理错误请求 (400)
func (h *ErrorHandler) HandleBadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    "invalid_request_error",
		},
	})
}

// HandleUnauthorized 处理未授权错误 (401)
func (h *ErrorHandler) HandleUnauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    "authentication_error",
		},
	})
}

// HandleNotFound 处理资源不存在错误 (404)
func (h *ErrorHandler) HandleNotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    "not_found_error",
		},
	})
}

// HandlePayloadTooLarge 处理请求体过大错误 (413)
func (h *ErrorHandler) HandlePayloadTooLarge(c *gin.Context, message string) {
	c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    "payload_too_large",
		},
	})
}

// HandleTooManyRequests 处理请求过多错误 (429)
func (h *ErrorHandler) HandleTooManyRequests(c *gin.Context, message string) {
	c.JSON(http.StatusTooManyRequests, ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    "rate_limit_error",
		},
	})
}

// HandleInternalServerError 处理内部服务器错误 (500)
func (h *ErrorHandler) HandleInternalServerError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    "internal_server_error",
		},
	})
}

// HandleBadGateway 处理网关错误 (502)
func (h *ErrorHandler) HandleBadGateway(c *gin.Context, message string) {
	c.JSON(http.StatusBadGateway, ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    "bad_gateway",
		},
	})
}

// HandleGatewayTimeout 处理网关超时错误 (504)
func (h *ErrorHandler) HandleGatewayTimeout(c *gin.Context, message string) {
	c.JSON(http.StatusGatewayTimeout, ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    "gateway_timeout",
		},
	})
}
