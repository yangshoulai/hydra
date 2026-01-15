import { ref, onUnmounted } from 'vue'

/**
 * 轮询 composable
 * @param fetchFn 数据获取函数
 * @param interval 轮询间隔（毫秒），默认 5000ms
 * @param autoStart 是否自动启动，默认 true
 */
export function usePolling<T>(
  fetchFn: () => Promise<T>,
  interval: number = 5000,
  autoStart: boolean = true
) {
  const data = ref<T | null>(null)
  const error = ref<Error | null>(null)
  const isLoading = ref(false)
  const isPolling = ref(false)

  let timer: number | null = null

  // 执行数据获取
  const fetch = async () => {
    if (isLoading.value) return

    isLoading.value = true
    error.value = null

    try {
      const result = await fetchFn()
      data.value = result
    } catch (err) {
      error.value = err as Error
      console.error('Polling fetch error:', err)
    } finally {
      isLoading.value = false
    }
  }

  // 启动轮询
  const start = () => {
    if (isPolling.value) return

    isPolling.value = true

    // 立即执行一次
    fetch()

    // 设置定时器
    timer = window.setInterval(() => {
      fetch()
    }, interval)
  }

  // 停止轮询
  const stop = () => {
    if (!isPolling.value) return

    isPolling.value = false

    if (timer !== null) {
      clearInterval(timer)
      timer = null
    }
  }

  // 手动刷新
  const refresh = async () => {
    await fetch()
  }

  // 组件卸载时自动停止轮询
  onUnmounted(() => {
    stop()
  })

  // 如果设置了自动启动，则立即开始
  if (autoStart) {
    start()
  }

  return {
    data,
    error,
    isLoading,
    isPolling,
    start,
    stop,
    refresh,
  }
}
