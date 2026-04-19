import { onUnmounted, ref } from 'vue'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || ''

interface UseEventStreamOptions<T> {
  url: string | (() => string)
  event?: string
  autoStart?: boolean
  initialRetryDelay?: number
  maxRetryDelay?: number
  parse?: (payload: string, eventName: string) => T
  onMessage?: (data: T, eventName: string) => void
}

function resolveURL(path: string): string {
  if (/^https?:\/\//i.test(path)) return path
  return `${API_BASE_URL}${path}`
}

export function useEventStream<T>(options: UseEventStreamOptions<T>) {
  const data = ref<T | null>(null)
  const error = ref<Error | null>(null)
  const isConnected = ref(false)

  const targetEvent = options.event || 'message'
  const autoStart = options.autoStart !== false
  const initialRetryDelay = options.initialRetryDelay ?? 1000
  const maxRetryDelay = options.maxRetryDelay ?? 30000

  let retryDelay = initialRetryDelay
  let reconnectTimer: number | null = null
  let controller: AbortController | null = null
  let closedByUser = false
  let connecting = false

  const clearReconnectTimer = () => {
    if (reconnectTimer !== null) {
      window.clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
  }

  const parseFrame = (rawBlock: string): { eventName: string; payload: string } | null => {
    const lines = rawBlock.split('\n')
    let eventName = 'message'
    const dataLines: string[] = []

    for (const rawLine of lines) {
      const line = rawLine.trimEnd()
      if (!line || line.startsWith(':')) continue
      if (line.startsWith('event:')) {
        eventName = line.slice('event:'.length).trim() || 'message'
        continue
      }
      if (line.startsWith('data:')) {
        dataLines.push(line.slice('data:'.length).trimStart())
      }
    }

    if (!dataLines.length) {
      return null
    }

    return {
      eventName,
      payload: dataLines.join('\n'),
    }
  }

  const scheduleReconnect = () => {
    if (closedByUser) return
    clearReconnectTimer()
    reconnectTimer = window.setTimeout(() => {
      void connect()
    }, retryDelay)
    retryDelay = Math.min(maxRetryDelay, retryDelay * 2)
  }

  const connect = async () => {
    if (closedByUser || connecting) return
    connecting = true
    clearReconnectTimer()

    controller?.abort()
    controller = new AbortController()

    const targetURL = typeof options.url === 'function' ? options.url() : options.url

    const accessToken = localStorage.getItem('access_token') || ''
    const headers: Record<string, string> = {
      Accept: 'text/event-stream',
      'Cache-Control': 'no-cache',
    }
    if (accessToken) {
      headers.Authorization = `Bearer ${accessToken}`
    }

    try {
      const response = await fetch(resolveURL(targetURL), {
        method: 'GET',
        headers,
        signal: controller.signal,
      })

      if (!response.ok || !response.body) {
        throw new Error(`stream request failed: ${response.status}`)
      }

      retryDelay = initialRetryDelay
      isConnected.value = true
      error.value = null

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n')
        let splitIndex = buffer.indexOf('\n\n')
        while (splitIndex >= 0) {
          const block = buffer.slice(0, splitIndex)
          buffer = buffer.slice(splitIndex + 2)

          const frame = parseFrame(block)
          if (frame && frame.eventName === targetEvent) {
            try {
              const parsed = options.parse
                ? options.parse(frame.payload, frame.eventName)
                : (JSON.parse(frame.payload) as T)
              data.value = parsed
              options.onMessage?.(parsed, frame.eventName)
            } catch (parseError) {
              error.value = parseError instanceof Error ? parseError : new Error(String(parseError))
            }
          }

          splitIndex = buffer.indexOf('\n\n')
        }
      }

      isConnected.value = false
      if (!closedByUser) {
        scheduleReconnect()
      }
    } catch (streamError) {
      isConnected.value = false

      if (streamError instanceof Error && streamError.name === 'AbortError') {
        return
      }

      error.value = streamError instanceof Error ? streamError : new Error(String(streamError))
      scheduleReconnect()
    } finally {
      connecting = false
    }
  }

  const close = () => {
    closedByUser = true
    clearReconnectTimer()
    controller?.abort()
    controller = null
    isConnected.value = false
  }

  const reconnect = () => {
    closedByUser = false
    retryDelay = initialRetryDelay
    void connect()
  }

  if (autoStart) {
    reconnect()
  }

  onUnmounted(() => {
    close()
  })

  return {
    data,
    error,
    isConnected,
    close,
    reconnect,
  }
}
