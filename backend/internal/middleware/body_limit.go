package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxBodyBytes 使用运行时回调为请求体设置大小上限。
// limit <= 0 表示不限制。真正读取超限错误的位置在业务层 io.ReadAll，
// 这样可以保留 Gin 路由与鉴权链路的统一行为。
func MaxBodyBytes(limitFn func() int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limitFn != nil && c.Request.Body != nil {
			if limit := limitFn(); limit > 0 {
				c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
			}
		}
		c.Next()
	}
}
