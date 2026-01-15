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

  // 卡片
  Card: {
    color: '#ffffff',
    colorModal: '#ffffff',
    colorTarget: '#ffffff',
    borderColor: '#e5e7eb',
    borderRadius: '1rem',  // 16px
    paddingMedium: '1.5rem',  // 24px
    paddingLarge: '2rem',     // 32px
    titleFontSizeMedium: '1.125rem',  // 18px
    titleFontWeight: '600',
  },

  // 按钮
  Button: {
    borderRadiusSmall: '0.5rem',   // 8px
    borderRadiusMedium: '0.625rem', // 10px
    borderRadiusLarge: '0.75rem',   // 12px
    heightSmall: '32px',
    heightMedium: '40px',
    heightLarge: '48px',
    fontSizeMedium: '0.875rem',  // 14px
    fontSizeLarge: '1rem',       // 16px
    fontWeightStrong: '600',
    paddingMedium: '0 1.25rem',  // 0 20px
    paddingLarge: '0 1.5rem',    // 0 24px
  },

  // 输入框
  Input: {
    borderRadius: '0.625rem',  // 10px
    border: '1px solid #e5e7eb',
    borderFocus: '1px solid #667eea',
    borderHover: '1px solid #d1d5db',
    color: '#ffffff',
    colorFocus: '#ffffff',
    boxShadowFocus: '0 0 0 3px rgba(102, 126, 234, 0.1)',
    heightMedium: '40px',
    heightLarge: '48px',
    paddingMedium: '0 0.875rem',  // 0 14px
    paddingLarge: '0 1rem',       // 0 16px
  },

  // 数据表格
  DataTable: {
    thColor: '#f9fafb',
    thColorHover: '#f3f4f6',
    thTextColor: '#111827',
    thFontWeight: '600',
    tdColor: '#ffffff',
    tdColorHover: '#f9fafb',
    tdColorStriped: '#f9fafb',
    tdTextColor: '#4b5563',
    borderColor: '#e5e7eb',
    borderRadius: '0.75rem',  // 12px
    thPadding: '1rem',        // 16px
    tdPadding: '1rem',        // 16px
  },

  // 标签
  Tag: {
    borderRadius: '0.5rem',  // 8px
    heightMedium: '28px',
    fontSizeMedium: '0.875rem',  // 14px
    fontWeight: '500',
    padding: '0 0.75rem',  // 0 12px
  },

  // 面包屑
  Breadcrumb: {
    fontSize: '0.875rem',  // 14px
    itemTextColor: '#6b7280',
    itemTextColorHover: '#667eea',
    itemTextColorPressed: '#5568d3',
    separatorColor: '#d1d5db',
  },

  // 分页
  Pagination: {
    buttonBorderRadius: '0.5rem',  // 8px
    itemSize: '36px',
    buttonColor: '#ffffff',
    buttonColorHover: '#f9fafb',
    buttonColorPressed: '#f3f4f6',
  },

  // 对话框
  Dialog: {
    borderRadius: '1rem',  // 16px
    iconMargin: '0 0.75rem 0 0',  // 0 12px 0 0
    padding: '2rem',  // 32px
    titleFontSize: '1.25rem',  // 20px
    titleFontWeight: '600',
  },

  // 抽屉
  Drawer: {
    borderRadius: '0px',
    bodyPadding: '1.5rem',  // 24px
    headerPadding: '1.5rem',  // 24px
  },

  // 通知
  Notification: {
    borderRadius: '0.75rem',  // 12px
    padding: '1rem 1.25rem',  // 16px 20px
    titleFontSize: '1rem',    // 16px
    titleFontWeight: '600',
    descriptionFontSize: '0.875rem',  // 14px
  },

  // 消息
  Message: {
    borderRadius: '0.625rem',  // 10px
    padding: '0.75rem 1rem',  // 12px 16px
    fontSize: '0.875rem',  // 14px
  },

  // 下拉菜单
  Dropdown: {
    borderRadius: '0.625rem',  // 10px
    padding: '0.5rem',  // 8px
    optionHeightMedium: '36px',
    optionFontSizeMedium: '0.875rem',  // 14px
  },

  // Select
  Select: {
    menuBorderRadius: '0.625rem',  // 10px
    menuPadding: '0.5rem',  // 8px
    menuBoxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1)',
  },

  // Switch
  Switch: {
    railHeight: '22px',
    railWidth: '44px',
    buttonHeight: '18px',
    buttonWidth: '18px',
  },

  // Tabs
  Tabs: {
    tabBorderRadius: '0.5rem',  // 8px
    tabFontSizeMedium: '0.875rem',  // 14px
    tabFontWeight: '500',
    tabPaddingMedium: '0.625rem 1rem',  // 10px 16px
    tabGapMediumCard: '0.5rem',  // 8px
  },
}
</script>

<script lang="ts">
import {defineComponent} from 'vue'

export default {
  name: 'NaiveProvider',
}
</script>
