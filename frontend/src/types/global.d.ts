/**
 * 全局类型定义
 */

declare global {
  interface Window {
    $message: {
      success: (content: string) => void
      error: (content: string) => void
      warning: (content: string) => void
      info: (content: string) => void
    }
    $dialog: any
    $notification: any
  }
}

export {}
