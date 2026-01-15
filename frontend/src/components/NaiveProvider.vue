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
    // 主色系 - 使用优雅的紫蓝色
    primaryColor: '#667eea',
    primaryColorHover: '#5568d3',
    primaryColorPressed: '#4c5db8',
    primaryColorSuppl: '#7c8ef5',

    // 功能色 - 柔和而醒目
    successColor: '#10b981',
    successColorHover: '#059669',
    successColorPressed: '#047857',
    warningColor: '#f59e0b',
    warningColorHover: '#d97706',
    warningColorPressed: '#b45309',
    errorColor: '#ef4444',
    errorColorHover: '#dc2626',
    errorColorPressed: '#b91c1c',
    infoColor: '#3b82f6',
    infoColorHover: '#2563eb',
    infoColorPressed: '#1d4ed8',

    // 文字颜色
    textColorBase: '#111827',
    textColor1: '#111827',
    textColor2: '#4b5563',
    textColor3: '#6b7280',
    textColorDisabled: '#d1d5db',

    // 边框和分割线
    borderColor: '#e5e7eb',
    borderColorHover: '#d1d5db',
    dividerColor: '#e5e7eb',

    // 圆角
    borderRadius: '0.75rem',  // 12px
    borderRadiusSmall: '0.5rem',  // 8px

    // 字体
    fontWeight: '500',
    fontWeightStrong: '600',
    fontSizeSmall: '0.875rem',  // 14px
    fontSizeMedium: '1rem',     // 16px
    fontSizeLarge: '1.125rem',  // 18px

    // 盒子阴影
    boxShadow1: '0 1px 3px 0 rgba(0, 0, 0, 0.1)',
    boxShadow2: '0 4px 6px -1px rgba(0, 0, 0, 0.1)',
    boxShadow3: '0 10px 15px -3px rgba(0, 0, 0, 0.1)',
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
    borderRadius: '0.5rem',
    itemHeight: '40px',
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
