import axios from 'axios'
import { feedback } from '@/services/feedback'

export function getErrorMessage(err: unknown, fallback = '操作失败'): string {
  if (axios.isAxiosError(err)) {
    const data = err.response?.data as { error?: string; message?: string } | undefined
    if (data?.error) return data.error
    if (data?.message) return data.message
    if (err.message) return err.message
  }
  if (err instanceof Error) return err.message
  if (typeof err === 'string') return err
  return fallback
}

export function toastApiError(err: unknown, prefix = ''): void {
  const msg = getErrorMessage(err)
  feedback.message?.error(prefix ? `${prefix}：${msg}` : msg)
}
