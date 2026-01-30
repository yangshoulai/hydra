package admin

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
)

const (
	remoteProvidersURL = "https://basellm.github.io/llm-metadata/api/providers.json"
	timeout            = 10 * time.Second
)

// RemoteProviderResponse 远程厂商 API 响应
type RemoteProviderResponse struct {
	Providers []RemoteProvider `json:"providers"`
}

// RemoteProvider 远程厂商
type RemoteProvider struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IconURL    string `json:"iconURL"`
	LobeIcon   string `json:"lobeIcon"`
	ModelCount int    `json:"modelCount"`
}

// SyncProviders 同步远程厂商
func (s *ProviderService) SyncProviders(ctx context.Context) ([]RemoteProvider, error) {
	s.logger.Info("同步远程供应商", slog.String("url", remoteProvidersURL))

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: timeout,
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteProvidersURL, nil)
	if err != nil {
		s.logger.Error("创建同步请求异常", slog.String("error", err.Error()))
		return nil, err
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("获取远程供应商信息异常", slog.String("error", err.Error()))
		return nil, err
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		s.logger.Error("远程供应商接口返回无效状态码",
			slog.Int("status", resp.StatusCode),
		)
		return nil, err
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("读取响应体异常", slog.String("error", err.Error()))
		return nil, err
	}

	// 解析 JSON
	var response RemoteProviderResponse
	if err := json.Unmarshal(body, &response); err != nil {
		s.logger.Error("解析响应体异常", slog.String("error", err.Error()))
		return nil, err
	}

	s.logger.Info("同步远程供应商信息成功",
		slog.Int("count", len(response.Providers)),
	)

	return response.Providers, nil
}

// BatchCreateProviders 批量创建厂商
func (s *ProviderService) BatchCreateProviders(ctx context.Context, providers []CreateProviderRequest) ([]models.Provider, []error) {
	var createdProviders []models.Provider
	var errors []error

	for _, req := range providers {
		provider, err := s.Create(ctx, req)
		if err != nil {
			// 记录错误，继续处理下一个
			s.logger.Warn("failed to create provider during batch",
				slog.String("id", req.ID),
				slog.String("error", err.Error()),
			)
			errors = append(errors, err)
			continue
		}
		createdProviders = append(createdProviders, *provider)
	}

	return createdProviders, errors
}

// NormalizeProviderID 标准化厂商ID
func NormalizeProviderID(id string) string {
	return strings.TrimSpace(strings.ToLower(id))
}
