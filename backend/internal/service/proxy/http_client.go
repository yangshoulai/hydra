package proxy

import (
	"log/slog"

	"github.com/yangshoulai/hydra/internal/service/upstreamhttp"
)

type HTTPClient = upstreamhttp.HTTPClient
type HTTPClientConfig = upstreamhttp.HTTPClientConfig

func DefaultHTTPClientConfig() *HTTPClientConfig {
	return upstreamhttp.DefaultHTTPClientConfig()
}

func NewHTTPClient(config *HTTPClientConfig, logger *slog.Logger) *HTTPClient {
	return upstreamhttp.NewHTTPClient(config, logger)
}
