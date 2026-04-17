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
import { computed, h } from 'vue'
import {
  type DataTableColumns,
  NButton,
  NDataTable,
  NEmpty,
  NIcon,
  NModal,
  NSpace,
  NText,
} from 'naive-ui'
import { CheckmarkOutline, CloseOutline, RemoveOutline } from '@vicons/ionicons5'
import type { TestModeResult } from '@/types/channel'
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

const tableScrollX = computed(() => {
  let width = 900
  if (showChannelColumn.value) width += 140
  if (showModelColumn.value) width += 160
  return width
})

const columns = computed<DataTableColumns<ModelTestResultItem>>(() => {
  const result: DataTableColumns<ModelTestResultItem> = []

  if (showChannelColumn.value) {
    result.push({
      title: '渠道',
      key: 'channelName',
      minWidth: 150,
      ellipsis: { tooltip: true },
      render: (row) => h(NText, null, { default: () => row.channelName || '-' }),
    })
  }

  if (showModelColumn.value) {
    result.push({
      title: '模型',
      key: 'modelName',
      minWidth: 140,
      ellipsis: { tooltip: true },
      render: (row) => h(NText, { code: true }, { default: () => row.modelName || '-' }),
    })
  }

  result.push(
    {
      title: '渠道模型',
      key: 'channelModel',
      width: 160,
      ellipsis: { tooltip: true },
      render: (row) => h(NText, { code: true }, { default: () => row.channelModel }),
    },
    {
      title: '端点',
      key: 'endpointType',
      width: 180,
      render: (row) => h('div', { class: 'endpoint-cell inline-code' }, row.endpointType),
    },
    {
      title: '结果',
      key: 'success',
      width: 80,
      align: 'center',
      render: (row) => renderStatusPill(row.success ? '通过' : '失败', row.success ? 'success' : 'error'),
    },
    {
      title: '非流式',
      key: 'nonStream',
      width: 120,
      render: (row) => renderModeCell(row.nonStream),
    },
    {
      title: '流式',
      key: 'stream',
      width: 120,
      render: (row) => renderModeCell(row.stream),
    },
    {
      title: '详情',
      key: 'detail',
      minWidth: 320,
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

:deep(.n-data-table td) {
  vertical-align: middle;
}

:deep(.n-data-table .n-data-table-th),
:deep(.n-data-table .n-data-table-td) {
  padding-top: 10px !important;
  padding-bottom: 10px !important;
}
</style>
