package upstreamhttp

import (
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
)

// HTTPClient 上游 HTTP 客户端
type HTTPClient struct {
	directClient *http.Client
	proxyClient  *http.Client
	logger       *slog.Logger
	config       *HTTPClientConfig
	configMu     sync.RWMutex // 保护配置与底层客户端的读写
}

// HTTPClientConfig HTTP 客户端配置
type HTTPClientConfig struct {
	RequestTimeout        time.Duration // 请求超时时间
	UpstreamProxyURL      string        // 上游网络代理地址（http/https/socks5）
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
		UpstreamProxyURL:      "",
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
	// 复制配置以避免外部修改
	configCopy := *config

	return &HTTPClient{
		directClient: newStdHTTPClient(configCopy, logger, ""),
		proxyClient:  newStdHTTPClient(configCopy, logger, configCopy.UpstreamProxyURL),
		logger:       logger,
		config:       &configCopy,
		configMu:     sync.RWMutex{},
	}
}

func newStdHTTPClient(config HTTPClientConfig, logger *slog.Logger, proxyURL string) *http.Client {
	config.UpstreamProxyURL = strings.TrimSpace(proxyURL)
	return &http.Client{
		Transport: buildTransport(config, logger),
		Timeout:   config.RequestTimeout,
	}
}

func buildTransport(config HTTPClientConfig, logger *slog.Logger) *http.Transport {
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

	if proxyFunc := buildProxyFunc(config.UpstreamProxyURL, logger); proxyFunc != nil {
		transport.Proxy = proxyFunc
	}
	return transport
}

func buildProxyFunc(proxyURL string, logger *slog.Logger) func(*http.Request) (*url.URL, error) {
	trimmed := strings.TrimSpace(proxyURL)
	if trimmed == "" {
		return nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		logger.Warn("网络代理地址无效，已忽略",
			slog.String("proxy_url", trimmed),
			slog.String("error", err.Error()),
		)
		return nil
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		logger.Warn("网络代理协议不支持，已忽略",
			slog.String("proxy_url", trimmed),
			slog.String("scheme", parsed.Scheme),
		)
		return nil
	}

	if parsed.Host == "" {
		logger.Warn("网络代理地址缺少主机信息，已忽略",
			slog.String("proxy_url", trimmed),
		)
		return nil
	}

	return http.ProxyURL(parsed)
}

// Do 发送 HTTP 请求
func (hc *HTTPClient) Do(req *http.Request, traceID string) (*http.Response, error) {
	return hc.DoWithProxy(req, traceID, true)
}

// DoWithProxy 根据渠道配置决定本次请求是否使用系统代理。
func (hc *HTTPClient) DoWithProxy(req *http.Request, traceID string, useSystemProxy bool) (*http.Response, error) {
	startTime := time.Now()

	// 记录当前配置快照
	hc.configMu.RLock()
	client := hc.directClient
	currentTimeout := hc.config.RequestTimeout
	currentProxyURL := hc.config.UpstreamProxyURL
	if useSystemProxy {
		client = hc.proxyClient
	}
	hc.configMu.RUnlock()
	if client == nil {
		client = http.DefaultClient
	}

	effectiveProxyURL := ""
	if useSystemProxy {
		effectiveProxyURL = strings.TrimSpace(currentProxyURL)
	}

	hc.logger.Debug("渠道调用开始",
		slog.String("trace_id", traceID),
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Bool("use_system_proxy", useSystemProxy),
		slog.Duration("request_timeout", currentTimeout),
		slog.String("network_proxy_url", effectiveProxyURL),
	)

	resp, err := client.Do(req)

	duration := time.Since(startTime)

	if err != nil {
		hc.logger.Debug("渠道调用异常",
			slog.String("trace_id", traceID),
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.Duration("duration", duration),
			slog.Bool("use_system_proxy", useSystemProxy),
			slog.Duration("request_timeout", currentTimeout),
			slog.String("network_proxy_url", effectiveProxyURL),
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
		slog.Bool("use_system_proxy", useSystemProxy),
		slog.String("network_proxy_url", effectiveProxyURL),
	)

	return resp, nil
}

// Close 关闭客户端(清理连接池)
func (hc *HTTPClient) Close() {
	hc.configMu.RLock()
	directClient := hc.directClient
	proxyClient := hc.proxyClient
	hc.configMu.RUnlock()
	if directClient != nil {
		directClient.CloseIdleConnections()
	}
	if proxyClient != nil && proxyClient != directClient {
		proxyClient.CloseIdleConnections()
	}
}

// UpdateRequestTimeout 动态更新请求超时时间
func (hc *HTTPClient) UpdateRequestTimeout(timeout time.Duration) {
	hc.configMu.Lock()
	oldTimeout := hc.config.RequestTimeout
	oldDirectClient := hc.directClient
	oldProxyClient := hc.proxyClient
	hc.config.RequestTimeout = timeout
	hc.directClient = newStdHTTPClient(*hc.config, hc.logger, "")
	hc.proxyClient = newStdHTTPClient(*hc.config, hc.logger, hc.config.UpstreamProxyURL)
	hc.configMu.Unlock()

	if oldDirectClient != nil {
		oldDirectClient.CloseIdleConnections()
	}
	if oldProxyClient != nil && oldProxyClient != oldDirectClient {
		oldProxyClient.CloseIdleConnections()
	}

	hc.logger.Info("HTTP客户端超时时间已更新",
		slog.Duration("old_timeout", oldTimeout),
		slog.Duration("new_timeout", timeout),
	)
}

// UpdateUpstreamProxyURL 动态更新上游网络代理地址
func (hc *HTTPClient) UpdateUpstreamProxyURL(proxyURL string) {
	hc.configMu.Lock()
	oldProxyURL := hc.config.UpstreamProxyURL
	oldDirectClient := hc.directClient
	oldProxyClient := hc.proxyClient
	hc.config.UpstreamProxyURL = strings.TrimSpace(proxyURL)
	hc.directClient = newStdHTTPClient(*hc.config, hc.logger, "")
	hc.proxyClient = newStdHTTPClient(*hc.config, hc.logger, hc.config.UpstreamProxyURL)
	hc.configMu.Unlock()

	if oldDirectClient != nil {
		oldDirectClient.CloseIdleConnections()
	}
	if oldProxyClient != nil && oldProxyClient != oldDirectClient {
		oldProxyClient.CloseIdleConnections()
	}

	hc.logger.Info("HTTP客户端网络代理已更新",
		slog.String("old_proxy_url", oldProxyURL),
		slog.String("new_proxy_url", strings.TrimSpace(proxyURL)),
	)
}

// ApplyUserAgent 统一设置请求 User-Agent。
func ApplyUserAgent(req *http.Request, userAgent string) {
	normalized := strings.TrimSpace(userAgent)
	if normalized == "" {
		normalized = models.DefaultModelTestUserAgent
	}
	req.Header.Set("User-Agent", normalized)
}

// ApplyJSONHeaders 为 JSON 接口设置通用头。
func ApplyJSONHeaders(req *http.Request, userAgent string) {
	req.Header.Set("Accept", "application/json")
	ApplyUserAgent(req, userAgent)
}
