package proxy

import (
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// HTTPClient 上游 HTTP 客户端
type HTTPClient struct {
	client *http.Client
	logger *slog.Logger
}

// HTTPClientConfig HTTP 客户端配置
type HTTPClientConfig struct {
	RequestTimeout       time.Duration // 请求超时时间
	DialTimeout          time.Duration // 连接超时时间
	KeepAlive            time.Duration // Keep-Alive 时长
	MaxIdleConns         int           // 最大空闲连接数
	MaxIdleConnsPerHost  int           // 每个 Host 最大空闲连接数
	MaxConnsPerHost      int           // 每个 Host 最大连接数
	IdleConnTimeout      time.Duration // 空闲连接超时时间
	TLSHandshakeTimeout  time.Duration // TLS 握手超时时间
	ExpectContinueTimeout time.Duration // Expect 100-continue 超时时间
	InsecureSkipVerify   bool          // 是否跳过 TLS 证书验证
}

// DefaultHTTPClientConfig 默认 HTTP 客户端配置
func DefaultHTTPClientConfig() *HTTPClientConfig {
	return &HTTPClientConfig{
		RequestTimeout:        120 * time.Second,
		DialTimeout:           10 * time.Second,
		KeepAlive:             30 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		InsecureSkipVerify:    false,
	}
}

// NewHTTPClient 创建 HTTP 客户端
func NewHTTPClient(config *HTTPClientConfig, logger *slog.Logger) *HTTPClient {
	if config == nil {
		config = DefaultHTTPClientConfig()
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.RequestTimeout,
	}

	return &HTTPClient{
		client: client,
		logger: logger,
	}
}

// Do 发送 HTTP 请求
func (hc *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	startTime := time.Now()

	hc.logger.Debug("sending upstream request",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
	)

	resp, err := hc.client.Do(req)

	duration := time.Since(startTime)

	if err != nil {
		hc.logger.Error("upstream request failed",
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	hc.logger.Debug("upstream request completed",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Int("status_code", resp.StatusCode),
		slog.Duration("duration", duration),
	)

	return resp, nil
}

// Close 关闭客户端(清理连接池)
func (hc *HTTPClient) Close() {
	hc.client.CloseIdleConnections()
}

// GetClient 获取底层 http.Client
func (hc *HTTPClient) GetClient() *http.Client {
	return hc.client
}
