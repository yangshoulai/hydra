package config

import "context"

// ConfigListener 配置变更监听器接口
type ConfigListener interface {
	// OnConfigChanged 当配置变更时调用
	// category: 配置分类 (circuit_breaker, logging, proxy 等)
	OnConfigChanged(ctx context.Context, category string)
}

// ConfigNotifier 配置变更通知器
type ConfigNotifier struct {
	listeners []ConfigListener
}

// NewConfigNotifier 创建配置通知器
func NewConfigNotifier() *ConfigNotifier {
	return &ConfigNotifier{
		listeners: make([]ConfigListener, 0),
	}
}

// Register 注册配置监听器
func (n *ConfigNotifier) Register(listener ConfigListener) {
	n.listeners = append(n.listeners, listener)
}

// Notify 通知所有监听器配置已变更
func (n *ConfigNotifier) Notify(ctx context.Context, category string) {
	for _, listener := range n.listeners {
		listener.OnConfigChanged(ctx, category)
	}
}
