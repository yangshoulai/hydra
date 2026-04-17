<template>
  <n-config-provider
    :theme-overrides="themeOverrides"
    :inline-theme-disabled="false"
    :locale="zhCN"
    :date-locale="dateZhCN"
  >
    <n-message-provider>
      <n-dialog-provider>
        <n-notification-provider>
          <slot />
          <GlobalHooks />
        </n-notification-provider>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { defineComponent } from 'vue'
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

const GlobalHooks = defineComponent({
  name: 'GlobalHooks',
  setup() {
    const message = useMessage()
    const dialog = useDialog()
    const notification = useNotification()

    window.$message = message
    window.$dialog = dialog
    window.$notification = notification

    return () => null
  },
})

const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#111111',
    primaryColorHover: '#000000',
    primaryColorPressed: '#000000',
    primaryColorSuppl: '#111111',
    successColor: '#111111',
    warningColor: '#404040',
    warningColorHover: '#262626',
    warningColorPressed: '#111111',
    warningColorSuppl: '#404040',
    errorColor: '#1f1f1f',
    infoColor: '#262626',
    infoColorHover: '#111111',
    infoColorPressed: '#000000',
    infoColorSuppl: '#111111',
    borderRadius: '12px',
    fontSize: '14px',
    textColorBase: '#111111',
    bodyColor: '#f5f5f5',
    cardColor: '#ffffff',
    modalColor: '#ffffff',
    popoverColor: '#ffffff',
    borderColor: '#d4d4d4',
  },
  Layout: {
    color: '#f5f5f5',
    siderColor: '#ffffff',
    siderBorderColor: '#e5e5e5',
    headerColor: '#ffffff',
    headerBorderColor: '#e5e5e5',
  },
  Menu: {
    itemTextColor: '#404040',
    itemTextColorHover: '#000000',
    itemTextColorActive: '#000000',
    itemIconColor: '#525252',
    itemIconColorHover: '#111111',
    itemColorHover: 'transparent',
    itemColorActive: 'transparent',
    itemColorActiveCollapsed: 'transparent',
    itemIconColorActive: '#111111',
    arrowColor: '#737373',
    arrowColorHover: '#262626',
    arrowColorActive: '#111111',
  },
  Card: {
    borderRadius: '12px',
    color: '#ffffff',
    borderColor: '#e2e2e2',
  },
  DataTable: {
    tdColor: '#ffffff',
    thColor: '#f5f5f5',
    borderColor: '#e2e2e2',
    thTextColor: '#525252',
    tdTextColor: '#171717',
  },
  Input: {
    borderRadius: '10px',
  },
  Button: {
    borderRadiusSmall: '10px',
    borderRadiusMedium: '10px',
    borderRadiusLarge: '12px',
  },
  Select: {
    menuBoxShadow: '0 10px 28px rgba(0, 0, 0, 0.12)',
  },
}
</script>
