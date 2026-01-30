package proxy

import (
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// HTTPClient 上游 HTTP 客户端
type HTTPClient struct {
	client   *http.Client
	logger   *slog.Logger
	config   *HTTPClientConfig
	configMu sync.RWMutex // 保护配置的读写
}

// HTTPClientConfig HTTP 客户端配置
type HTTPClientConfig struct {
	RequestTimeout        time.Duration // 请求超时时间
	DialTimeout           time.Duration // 连接超时时间
	KeepAlive             time.Duration // Keep-Alive 时长
	MaxIdleConns          int           // 最大空闲连接数
	MaxIdleConnsPerHost   int           // 每个 Host 最大空闲连接数
	MaxConnsPerHost       int           // 每个 Host 最大连接数
	IdleConnTimeout       time.Duration // 空闲连接超时时间
	TLSHandshakeTimeout   time.Duration // TLS 握手超时时间
	ExpectContinueTimeout time.Duration // Expect 100-continue 超时时间
	InsecureSkipVerify    bool          // 是否跳过 TLS 证书验证
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

	// 复制配置以避免外部修改
	configCopy := *config

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   configCopy.DialTimeout,
			KeepAlive: configCopy.KeepAlive,
		}).DialContext,
		MaxIdleConns:          configCopy.MaxIdleConns,
		MaxIdleConnsPerHost:   configCopy.MaxIdleConnsPerHost,
		MaxConnsPerHost:       configCopy.MaxConnsPerHost,
		IdleConnTimeout:       configCopy.IdleConnTimeout,
		TLSHandshakeTimeout:   configCopy.TLSHandshakeTimeout,
		ExpectContinueTimeout: configCopy.ExpectContinueTimeout,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: configCopy.InsecureSkipVerify,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   configCopy.RequestTimeout,
	}

	return &HTTPClient{
		client:   client,
		logger:   logger,
		config:   &configCopy,
		configMu: sync.RWMutex{},
	}
}

// Do 发送 HTTP 请求
func (hc *HTTPClient) Do(req *http.Request, traceID string) (*http.Response, error) {
	startTime := time.Now()

	// 记录当前超时配置
	hc.configMu.RLock()
	currentTimeout := hc.config.RequestTimeout
	hc.configMu.RUnlock()

	hc.logger.Debug("渠道调用开始",
		slog.String("trace_id", traceID),
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Duration("request_timeout", currentTimeout),
	)

	resp, err := hc.client.Do(req)

	duration := time.Since(startTime)

	if err != nil {
		hc.logger.Debug("渠道调用异常",
			slog.String("trace_id", traceID),
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.Duration("duration", duration),
			slog.Duration("request_timeout", currentTimeout),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	hc.logger.Debug("渠道调用完成",
		slog.String("trace_id", traceID),
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

// UpdateRequestTimeout 动态更新请求超时时间
func (hc *HTTPClient) UpdateRequestTimeout(timeout time.Duration) {
	hc.configMu.Lock()
	defer hc.configMu.Unlock()

	oldTimeout := hc.client.Timeout
	hc.client.Timeout = timeout
	hc.config.RequestTimeout = timeout

	hc.logger.Info("HTTP客户端超时时间已更新",
		slog.Duration("old_timeout", oldTimeout),
		slog.Duration("new_timeout", timeout),
	)
}

// GetRequestTimeout 获取当前请求超时时间
func (hc *HTTPClient) GetRequestTimeout() time.Duration {
	hc.configMu.RLock()
	defer hc.configMu.RUnlock()
	return hc.config.RequestTimeout
}
