package config

// RegisterConfigListeners 注册配置监听器
func RegisterConfigListeners(settingService *SettingService, listeners []ConfigListener) {
	for _, listener := range listeners {
		settingService.RegisterListener(listener)
	}
}
