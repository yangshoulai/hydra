<template>
  <n-config-provider :theme-overrides="themeOverrides" :inline-theme-disabled="false" :locale="zhCN"
                     :date-locale="dateZhCN">
    <n-message-provider>
      <n-dialog-provider>
        <n-notification-provider>
          <slot/>
          <GlobalHooks/>
        </n-notification-provider>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import {
  dateZhCN,
  type GlobalThemeOverrides,
  NConfigProvider,
  NDialogProvider,
  NMessageProvider,
  NNotificationProvider,
  useDialog,
  useMessage,
  useNotification,
  zhCN,
} from 'naive-ui'

// 全局 hooks 初始化组件
const GlobalHooks = defineComponent({
  name: 'GlobalHooks',
  setup() {
    const message = useMessage()
    const dialog = useDialog()
    const notification = useNotification()

    // 挂载到 window，方便全局访问
    window.$message = message
    window.$dialog = dialog
    window.$notification = notification

    return () => null
  },
})

// Naive UI 主题覆盖配置 - 清新简约、高端大气
const themeOverrides: GlobalThemeOverrides = {
  common: {

  },

  // 布局
  Layout: {
    color: '#ffffff',
    textColor: '#111827',
    siderColor: '#1a1f36',
    siderTextColor: '#ffffff',
    siderBorderColor: 'transparent',
    headerColor: '#ffffff',
    headerBorderColor: '#e5e7eb',
  },

  // 菜单
  Menu: {
    itemTextColor: 'rgba(255, 255, 255, 0.75)',
    itemTextColorHover: '#ffffff',
    itemTextColorActive: '#ffffff',
    itemTextColorChildActive: '#ffffff',
    itemIconColor: 'rgba(255, 255, 255, 0.75)',
    itemIconColorHover: '#ffffff',
    itemIconColorActive: '#667eea',
    itemIconColorChildActive: '#667eea',
    itemColorActive: 'rgba(102, 126, 234, 0.15)',
    itemColorActiveHover: 'rgba(102, 126, 234, 0.2)',
    itemColorActiveCollapsed: 'rgba(102, 126, 234, 0.2)',
    arrowColor: 'rgba(255, 255, 255, 0.6)',
    arrowColorHover: '#ffffff',
    arrowColorActive: '#667eea',
  },
}
</script>

<script lang="ts">
import {defineComponent} from 'vue'

export default {
  name: 'NaiveProvider',
}
</script>
