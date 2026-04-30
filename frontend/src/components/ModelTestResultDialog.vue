<template>
  <n-modal
    v-model:show="visible"
    preset="card"
    :title="title"
    style="width: min(1120px, calc(100vw - 48px))"
    :mask-closable="true"
    :closable="true"
  >
    <n-space vertical :size="12">
      <section class="metric-grid result-summary-grid">
        <div class="metric-tile">
          <div class="metric-tile__label">测试端点</div>
          <div class="metric-tile__value">{{ totalCount }}</div>
        </div>
        <div class="metric-tile">
          <div class="metric-tile__label">通过</div>
          <div class="metric-tile__value">{{ successCount }}</div>
        </div>
        <div class="metric-tile">
          <div class="metric-tile__label">失败</div>
          <div class="metric-tile__value">{{ failureCount }}</div>
        </div>
        <div class="metric-tile">
          <div class="metric-tile__label">已触发流式</div>
          <div class="metric-tile__value">{{ streamTestedCount }}</div>
        </div>
      </section>

      <section class="panel-card">
        <header class="panel-card__header">
          <h3 class="panel-card__title">测试结果</h3>
        </header>
        <div class="panel-card__body">
          <n-empty v-if="!items.length" description="暂无测试结果" />
          <n-data-table
            v-else
            :columns="columns"
            :data="items"
            :pagination="false"
            :row-key="(row: ModelTestResultItem) => row.id"
            :single-line="false"
            :scroll-x="tableScrollX"
          />
        </div>
      </section>
    </n-space>

    <template #footer>
      <n-space justify="end">
        <n-button type="primary" @click="visible = false">关闭</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, type VNodeChild } from 'vue'
import {
  type DataTableColumns,
  NButton,
  NDataTable,
  NEmpty,
  NIcon,
  NImage,
  NModal,
  NSpace,
  NText,
} from 'naive-ui'
import { CheckmarkOutline, CloseOutline, RemoveOutline } from '@vicons/ionicons5'
import type { TestModeResult, TestResponseContent } from '@/types/channel'
import type { ModelTestResultItem } from '@/types/modelTest'

interface Props {
  show: boolean
  title: string
  items: ModelTestResultItem[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const visible = computed({
  get: () => props.show,
  set: (value: boolean) => emit('update:show', value),
})

const totalCount = computed(() => props.items.length)
const successCount = computed(() => props.items.filter((item) => item.success).length)
const failureCount = computed(() => props.items.filter((item) => !item.success).length)
const streamTestedCount = computed(() => props.items.filter((item) => item.stream.tested).length)

const showChannelColumn = computed(() => {
  const channelNames = new Set(props.items.map((item) => item.channelName).filter(Boolean))
  return channelNames.size > 1
})

const showModelColumn = computed(() => {
  const itemsWithModelName = props.items.filter((item) => !!item.modelName)
  if (!itemsWithModelName.length) return false

  const modelNames = new Set(itemsWithModelName.map((item) => item.modelName as string))
  if (modelNames.size > 1) return true

  return itemsWithModelName.some((item) => item.modelName !== item.channelModel)
})

const channelColumnWidth = 150
const modelColumnWidth = 140
const channelModelColumnWidth = 160
const endpointColumnWidth = 180
const resultColumnWidth = 80
const modeColumnWidth = 120
const responseContentColumnWidth = 420
const detailColumnWidth = 340

const tableScrollX = computed(() => {
  let width =
    channelModelColumnWidth +
    endpointColumnWidth +
    resultColumnWidth +
    modeColumnWidth * 2 +
    responseContentColumnWidth +
    detailColumnWidth
  if (showChannelColumn.value) width += channelColumnWidth
  if (showModelColumn.value) width += modelColumnWidth
  return width
})

const columns = computed<DataTableColumns<ModelTestResultItem>>(() => {
  const result: DataTableColumns<ModelTestResultItem> = []

  if (showChannelColumn.value) {
    result.push({
      title: '渠道',
      key: 'channelName',
      width: channelColumnWidth,
      ellipsis: { tooltip: true },
      render: (row) => h(NText, null, { default: () => row.channelName || '-' }),
    })
  }

  if (showModelColumn.value) {
    result.push({
      title: '模型',
      key: 'modelName',
      width: modelColumnWidth,
      ellipsis: { tooltip: true },
      render: (row) => h(NText, { code: true }, { default: () => row.modelName || '-' }),
    })
  }

  result.push(
    {
      title: '渠道模型',
      key: 'channelModel',
      width: channelModelColumnWidth,
      ellipsis: { tooltip: true },
      render: (row) => h(NText, { code: true }, { default: () => row.channelModel }),
    },
    {
      title: '端点',
      key: 'endpointType',
      width: endpointColumnWidth,
      render: (row) => h('div', { class: 'endpoint-cell inline-code' }, row.endpointType),
    },
    {
      title: '结果',
      key: 'success',
      width: resultColumnWidth,
      align: 'center',
      render: (row) => renderStatusPill(row.success ? '通过' : '失败', row.success ? 'success' : 'error'),
    },
    {
      title: '非流式',
      key: 'nonStream',
      width: modeColumnWidth,
      render: (row) => renderModeCell(row.nonStream),
    },
    {
      title: '流式',
      key: 'stream',
      width: modeColumnWidth,
      render: (row) => renderModeCell(row.stream),
    },
    {
      title: '返回内容',
      key: 'content',
      width: responseContentColumnWidth,
      className: 'response-content-table-cell',
      render: (row) => renderResponseContent(row.content),
    },
    {
      title: '详情',
      key: 'detail',
      width: detailColumnWidth,
      render: (row) =>
        h(
          'div',
          { class: 'result-detail' },
          row.detail
            .split('\n')
            .filter(Boolean)
            .map((line) => h('div', { class: 'result-detail__line' }, line)),
        ),
    },
  )

  return result
})

function renderResponseContent(content?: TestResponseContent) {
  if (!content || (!content.text && !content.image_url && !content.raw)) {
    return h(NText, { depth: 3 }, { default: () => '—' })
  }

  const previewMaxWidth = `${responseContentColumnWidth - 32}px`
  const nodes: VNodeChild[] = []
  if (content.image_url) {
    nodes.push(
      h(
        'div',
        { class: 'response-preview__image-wrap', style: { maxWidth: previewMaxWidth } },
        h(NImage, {
          src: content.image_url,
          width: 120,
          height: 120,
          objectFit: 'cover',
        }),
      ),
    )
  }

  const text = content.text || (!content.image_url ? content.raw : '')
  if (text) {
    nodes.push(
      h(
        'pre',
        {
          class: 'response-preview__text',
          style: {
            maxWidth: previewMaxWidth,
            whiteSpace: 'pre-wrap',
            overflowWrap: 'anywhere',
            wordBreak: 'break-word',
          },
        },
        text,
      ),
    )
  }

  return h('div', { class: 'response-preview', style: { maxWidth: previewMaxWidth } }, nodes)
}

function renderModeCell(mode: TestModeResult) {
  const state = !mode.tested ? 'neutral' : mode.success ? 'success' : 'error'
  const label = !mode.tested ? '跳过' : mode.success ? '通过' : '失败'
  const icon = !mode.tested ? RemoveOutline : mode.success ? CheckmarkOutline : CloseOutline

  return h(
    'div',
    { class: 'mode-cell' },
    [
      renderStatusPill(label, state, icon),
      mode.latency ? h('span', { class: 'mode-cell__meta' }, mode.latency) : null,
    ].filter(Boolean),
  )
}

function renderStatusPill(label: string, state: 'success' | 'error' | 'neutral', icon?: any) {
  return h(
    'span',
    { class: ['result-pill', `result-pill--${state}`] },
    [
      icon
        ? h(
          NIcon,
          { class: 'result-pill__icon' },
          {
            default: () => h(icon),
          },
        )
        : null,
      h('span', null, label),
    ].filter(Boolean),
  )
}
</script>

<style scoped>
.result-summary-grid {
  grid-template-columns: repeat(4, minmax(140px, 1fr));
}

.endpoint-cell {
  font-size: 12px;
  line-height: 1.45;
  color: var(--hydra-text-secondary);
  word-break: break-word;
}

.mode-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: flex-start;
}

.mode-cell__meta {
  font-size: 11px;
  line-height: 1;
  color: var(--hydra-text-tertiary);
}

.result-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-height: 24px;
  padding: 0 10px;
  border-radius: 999px;
  border: 1px solid var(--hydra-border-strong);
  background: var(--hydra-bg-subtle);
  font-size: 12px;
  font-weight: 650;
  line-height: 1;
  color: var(--hydra-text);
  white-space: nowrap;
}

.result-pill--success {
  background: #111111;
  border-color: #111111;
  color: #ffffff;
}

.result-pill--error {
  background: #f3f3f3;
  border-style: dashed;
}

.result-pill--neutral {
  background: #ffffff;
}

.result-pill__icon {
  font-size: 12px;
}

.result-detail {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.result-detail__line {
  font-size: 12px;
  line-height: 1.55;
  color: var(--hydra-text-secondary);
  white-space: normal;
  word-break: break-word;
}

.response-preview {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 10px;
  width: 100%;
  min-width: 0;
  max-width: 100%;
}

.response-preview__image-wrap {
  display: flex;
  justify-content: center;
  width: 100%;
  min-width: 0;
}

.response-preview__image-wrap :deep(.n-image) {
  overflow: hidden;
  border: 1px solid var(--hydra-border);
  border-radius: 10px;
  background: var(--hydra-bg-subtle);
}

.response-preview__text {
  display: block;
  width: 100%;
  min-width: 0;
  max-width: 100%;
  margin: 0;
  font-family: inherit;
  font-size: 12px;
  line-height: 1.65;
  color: var(--hydra-text-secondary);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
}

:deep(.response-content-table-cell) {
  white-space: normal !important;
  overflow: hidden;
}

:deep(.response-content-table-cell .response-preview),
:deep(.response-content-table-cell .response-preview__text) {
  min-width: 0;
  max-width: 100%;
  white-space: pre-wrap !important;
  overflow-wrap: anywhere !important;
  word-break: break-word !important;
}

:deep(.n-data-table td) {
  vertical-align: middle;
}

:deep(.n-data-table .n-data-table-th),
:deep(.n-data-table .n-data-table-td) {
  padding-top: 10px !important;
  padding-bottom: 10px !important;
}
</style>
