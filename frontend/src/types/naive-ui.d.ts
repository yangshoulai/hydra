/**
 * Naive UI 类型扩展
 */

import type { VNode } from 'vue'
import type { NInputSlotProps, NSelectSlotProps } from 'naive-ui'

declare module 'naive-ui' {
  interface NInputSlotProps {
    prefix?: () => VNode[]
    suffix?: () => VNode[]
  }

  interface NSelectSlotProps {
    prefix?: () => VNode[]
    suffix?: () => VNode[]
  }
}

export {}
