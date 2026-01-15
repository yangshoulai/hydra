package config

// RegisterConfigListeners 注册所有配置监听器
// 这个函数应该在应用初始化时调用
// 注意：这个函数在 main.go 中调用，直接注册监听器以避免循环导入
func RegisterConfigListeners(settingService *SettingService, listeners []ConfigListener) {
	for _, listener := range listeners {
		settingService.RegisterListener(listener)
	}

	// TODO: 在这里添加其他监听器
	// 例如：
	// settingService.RegisterListener(logService)
	// settingService.RegisterListener(proxyService)
}

// ConfigListenerRegistrar 配置监听器注册器接口
// 各个服务可以实现这个接口来支持配置热更新
type ConfigListenerRegistrar interface {
	// RegisterConfigListeners 注册该服务的配置监听器
	RegisterConfigListeners(settingService *SettingService)
}
