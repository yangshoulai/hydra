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
    $dialog: {
      warning: (options: any) => Promise<boolean>
      error: (options: any) => Promise<boolean>
      info: (options: any) => Promise<boolean>
      success: (options: any) => Promise<boolean>
    }
    $notification: {
      success: (options: any) => void
      error: (options: any) => void
      warning: (options: any) => void
      info: (options: any) => void
    }
  }
}

export {}
