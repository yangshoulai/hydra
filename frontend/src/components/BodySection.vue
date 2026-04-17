<template>
  <n-collapse :default-expanded-names="defaultClosed ? [] : ['self']" arrow-placement="right">
    <n-collapse-item :name="'self'">
      <template #header>
        <span class="section-header">
          <strong>{{ title }}</strong>
          <n-tag size="tiny" :bordered="false">{{ sizeLabel }}</n-tag>
          <n-tag v-if="prettyJSON" size="tiny" type="info" :bordered="false">JSON</n-tag>
        </span>
      </template>
      <div class="section-actions">
        <n-button v-if="showCurl" size="tiny" @click.stop="copyCurl">复制为 curl</n-button>
        <n-button size="tiny" @click.stop="downloadAll">下载原文</n-button>
      </div>

      <div v-if="headerEntries.length > 0" class="kv-block">
        <div class="kv-block__title">Headers</div>
        <table class="kv-table">
          <tbody>
            <tr v-for="[k, vs] in headerEntries" :key="k">
              <td class="kv-key">{{ k }}</td>
              <td class="kv-val">{{ (vs as string[]).join(', ') }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="kv-block" v-if="body">
        <div class="kv-block__title">Body</div>
        <pre class="body-pre" :class="{ 'body-pre--json': prettyJSON }">{{ prettyBody }}</pre>
      </div>
      <n-empty v-else size="small" description="（无 body）" />
    </n-collapse-item>
  </n-collapse>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NCollapse, NCollapseItem, NEmpty, NTag, useMessage } from 'naive-ui'

const props = withDefaults(
  defineProps<{
    title: string
    method?: string
    url?: string
    headersJson?: string
    body?: string
    bodySize?: number
    showCurl?: boolean
    filenamePrefix?: string
    defaultClosed?: boolean
  }>(),
  {
    method: '',
    url: '',
    headersJson: '',
    body: '',
    bodySize: 0,
    showCurl: false,
    filenamePrefix: 'body',
    defaultClosed: false,
  },
)

const message = useMessage()

const parsedHeaders = computed<Record<string, string[]> | null>(() => {
  if (!props.headersJson) return null
  try {
    return JSON.parse(props.headersJson) as Record<string, string[]>
  } catch {
    return null
  }
})

const headerEntries = computed(() => {
  if (!parsedHeaders.value) return []
  return Object.entries(parsedHeaders.value).sort(([a], [b]) => a.localeCompare(b))
})

// 尝试把 body 按 JSON 美化；检测 Content-Type 或首字符是否 { [
const prettyResult = computed<{ text: string; isJSON: boolean }>(() => {
  if (!props.body) return { text: '', isJSON: false }
  const raw = props.body
  const trimmed = raw.trim()
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) {
    return { text: raw, isJSON: false }
  }
  try {
    const obj = JSON.parse(trimmed)
    return { text: JSON.stringify(obj, null, 2), isJSON: true }
  } catch {
    return { text: raw, isJSON: false }
  }
})

const prettyBody = computed(() => prettyResult.value.text)
const prettyJSON = computed(() => prettyResult.value.isJSON)

const sizeLabel = computed(() => {
  const size = props.bodySize || props.body?.length || 0
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(2)} MB`
})

function downloadAll() {
  const filename = props.filenamePrefix
  const headersText = props.headersJson ? `# Headers\n${props.headersJson}\n\n` : ''
  const bodyText = props.body || ''
  const content = (headersText || bodyText)
    ? `${headersText}${bodyText ? '# Body\n' + bodyText : ''}`
    : ''
  if (!content) {
    message.info('内容为空，无需下载')
    return
  }
  const ext = prettyJSON.value ? 'json' : 'txt'
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `${filename}.${ext}`
  a.click()
  URL.revokeObjectURL(a.href)
}

function copyCurl() {
  if (!props.url) {
    message.warning('未记录请求 URL')
    return
  }
  const method = props.method || 'GET'
  const headerLines = parsedHeaders.value
    ? Object.entries(parsedHeaders.value)
        .flatMap(([k, vs]) => (vs as string[]).map((v) => `  -H '${escapeSingleQuote(`${k}: ${v}`)}'`))
        .join(' \\\n')
    : ''

  const lines: string[] = [`curl -X ${method} '${props.url}'`]
  if (headerLines) lines.push(headerLines)
  if (props.body) {
    lines.push(`  --data-raw '${escapeSingleQuote(props.body)}'`)
  }

  const curl = lines.join(' \\\n')
  navigator.clipboard.writeText(curl).then(
    () => message.success('已复制 curl，敏感头已脱敏为 *** 需手工替换'),
    () => message.error('复制失败'),
  )
}

function escapeSingleQuote(s: string): string {
  return s.replace(/'/g, "'\\''")
}
</script>

<style scoped>
.section-header {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.section-actions {
  display: flex;
  gap: 6px;
  margin: 4px 0 8px;
}

.kv-block {
  margin-bottom: 14px;
}

.kv-block__title {
  font-size: 12px;
  color: var(--n-text-color-3, #888);
  margin-bottom: 6px;
  font-weight: 500;
}

.kv-table {
  width: 100%;
  font-size: 12px;
  border-collapse: collapse;
  table-layout: fixed;
}

.kv-table td {
  padding: 5px 10px;
  border-bottom: 1px solid var(--n-border-color, rgba(0, 0, 0, 0.06));
  vertical-align: top;
  word-break: break-all;
}

.kv-key {
  width: 260px;
  color: var(--n-text-color-2, #555);
  font-family: ui-monospace, monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.kv-val {
  font-family: ui-monospace, monospace;
  white-space: pre-wrap;
}

.body-pre {
  margin: 0;
  padding: 12px 14px;
  background: var(--n-color-target, rgba(0, 0, 0, 0.04));
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.55;
  font-family: ui-monospace, monospace;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 560px;
  overflow-y: auto;
}

.body-pre--json {
  background: var(--n-color-target, rgba(0, 0, 0, 0.03));
  border: 1px solid var(--n-border-color, rgba(0, 0, 0, 0.06));
}
</style>
