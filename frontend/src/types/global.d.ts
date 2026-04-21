import type { DialogApi, MessageApi, NotificationApi } from 'naive-ui'

declare global {
  interface Window {
    $message: MessageApi
    $dialog: DialogApi
    $notification: NotificationApi
  }

  const __APP_VERSION__: string
  const __BUILD_DATE__: string
}

export {}
