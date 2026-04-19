<template>
  <div class="app-page dashboard-page">
    <div class="dashboard-toolbar">
      <span class="dashboard-toolbar__meta">
        <n-icon :component="TimeOutline" />
        最近更新 {{ lastUpdated }}
      </span>
      <n-button quaternary circle size="small" aria-label="刷新仪表盘数据" @click="handleRefresh">
        <template #icon>
          <n-icon>
            <RefreshOutline />
          </n-icon>
        </template>
      </n-button>
    </div>

    <n-alert v-if="error" type="error" :bordered="false">
      <n-space align="center" justify="space-between" style="width: 100%">
        <span>指标拉取失败，请检查网络或后端服务状态。</span>
        <n-button text type="error" @click="handleRefresh">重试</n-button>
      </n-space>
    </n-alert>

    <section v-if="isLoading && !metrics" class="panel-card">
      <div class="panel-card__body dashboard-skeleton">
        <n-skeleton text :repeat="10" />
      </div>
    </section>

    <template v-if="metrics || !isLoading">
      <section class="metric-cards">
        <div class="metric-card">
          <div class="metric-card__icon metric-card__icon--neutral">
            <n-icon :component="FlashOutline" />
          </div>
          <div class="metric-card__content">
            <div class="metric-card__label">当前 QPS</div>
            <div class="metric-card__value">
              <n-number-animation :from="0" :to="metrics?.current_qps || 0" :precision="2" :active="true" />
            </div>
            <div class="metric-card__subtext">实时指标</div>
          </div>
        </div>

        <div class="metric-card">
          <div class="metric-card__icon" :class="successRateIconClass">
            <n-icon :component="CheckmarkCircleOutline" />
          </div>
          <div class="metric-card__content">
            <div class="metric-card__label">成功率</div>
            <div class="metric-card__value">
              <n-number-animation :from="0" :to="metrics?.success_rate || 0" :precision="1" :active="true" />
              <span class="metric-card__unit">%</span>
            </div>
            <div class="metric-card__subtext">失败率 {{ (100 - (metrics?.success_rate || 0)).toFixed(1) }}%</div>
          </div>
        </div>

        <div class="metric-card">
          <div class="metric-card__icon metric-card__icon--neutral">
            <n-icon :component="TrendingUpOutline" />
          </div>
          <div class="metric-card__content">
            <div class="metric-card__label">近 24h 请求量</div>
            <div class="metric-card__value">{{ formatCompactNumber(metrics?.total_requests || 0) }}</div>
            <div class="metric-card__subtext">模型总请求 {{ formatCompactNumber(metrics?.model_stats?.total_requests || 0) }}</div>
          </div>
        </div>

        <div class="metric-card">
          <div class="metric-card__icon" :class="healthIconClass">
            <n-icon :component="ShieldCheckmarkOutline" />
          </div>
          <div class="metric-card__content">
            <div class="metric-card__label">整体健康度</div>
            <div class="metric-card__value">{{ formatPercent(overallHealthValue) }}</div>
            <div class="metric-card__subtext">
              <span v-if="resolvedUnhealthyKeys > 0" class="metric-card__trend--down">
                异常密钥 {{ resolvedUnhealthyKeys }}
              </span>
              <span v-else class="metric-card__trend--up">全部健康</span>
            </div>
          </div>
        </div>

        <div class="metric-card">
          <div class="metric-card__icon metric-card__icon--neutral">
            <n-icon :component="GridOutline" />
          </div>
          <div class="metric-card__content">
            <div class="metric-card__label">活跃渠道</div>
            <div class="metric-card__value">
              {{ metrics?.active_channels || 0 }}
              <span class="metric-card__unit">/ {{ metrics?.total_channels || 0 }}</span>
            </div>
            <div class="metric-card__subtext">启用 / 总数</div>
          </div>
        </div>

        <div class="metric-card">
          <div class="metric-card__icon metric-card__icon--neutral">
            <n-icon :component="CubeOutline" />
          </div>
          <div class="metric-card__content">
            <div class="metric-card__label">统一模型</div>
            <div class="metric-card__value">{{ metrics?.model_stats?.active_models || 0 }}</div>
            <div class="metric-card__subtext">已启用模型数</div>
          </div>
        </div>

        <div class="metric-card">
          <div class="metric-card__icon" :class="keyHealthIconClass">
            <n-icon :component="KeyOutline" />
          </div>
          <div class="metric-card__content">
            <div class="metric-card__label">密钥健康</div>
            <div class="metric-card__value">
              {{ resolvedHealthyKeys }}
              <span class="metric-card__unit">/ {{ resolvedTotalKeys }}</span>
            </div>
            <div class="metric-card__subtext">健康 / 总数</div>
          </div>
        </div>

        <div class="metric-card">
          <div class="metric-card__icon metric-card__icon--neutral">
            <n-icon :component="StatsChartOutline" />
          </div>
          <div class="metric-card__content">
            <div class="metric-card__label">Token 用量</div>
            <div class="metric-card__value metric-card__value--compact">
              {{ formatCompactNumber(metrics?.total_prompt_tokens || 0) }}
              <span class="metric-card__unit">/ {{ formatCompactNumber(metrics?.total_completion_tokens || 0) }}</span>
            </div>
            <div class="metric-card__subtext">输入 / 输出</div>
          </div>
        </div>
      </section>

      <section class="panel-card">
        <header class="panel-card__header">
          <h3 class="panel-card__title">QPS 趋势</h3>
          <n-space size="small" align="center">
            <span class="range-switch__label">时间范围</span>
            <div class="range-switch">
              <n-button
                v-for="option in qpsRangeOptions"
                :key="option.value"
                size="small"
                :type="qpsRange === option.value ? 'primary' : 'default'"
                :quaternary="qpsRange !== option.value"
                @click="handleQPSRangeChange(option.value)"
              >
                {{ option.label }}
              </n-button>
            </div>
          </n-space>
        </header>
        <div class="panel-card__body">
          <QpsChart :data="metrics?.qps_trend || []" :height="260" />
        </div>
      </section>

      <section class="panel-card">
        <header class="panel-card__header">
          <h3 class="panel-card__title">渠道健康（近 24 小时）</h3>
        </header>
        <div class="panel-card__body">
          <n-data-table
            :columns="channelColumns"
            :data="metrics?.channel_health_list || []"
            :loading="isLoading"
            :locale="tableLocale"
            :pagination="false"
            :single-line="false"
            striped
            :scroll-x="860"
          />
        </div>
      </section>

      <section class="panel-card">
        <header class="panel-card__header">
          <h3 class="panel-card__title">熔断状态</h3>
          <n-tag :bordered="false" size="small">{{ circuits.length }} 项冷却中</n-tag>
        </header>
        <div class="panel-card__body">
          <n-data-table
            :columns="circuitColumns"
            :data="circuits"
            :loading="isLoading"
            :locale="{
              emptyText: isLoading ? '指标加载中...' : '当前全部渠道密钥与模型运行正常',
            }"
            :pagination="false"
            :single-line="false"
            striped
            :scroll-x="860"
          />
        </div>
      </section>

      <section class="panel-card">
        <header class="panel-card__header">
          <h3 class="panel-card__title">模型表现（近 24 小时）</h3>
        </header>
        <div class="panel-card__body">
          <n-data-table
            :columns="modelColumns"
            :data="metrics?.model_stats?.model_list || []"
            :loading="isLoading"
            :locale="tableLocale"
            :pagination="false"
            :single-line="false"
            striped
            :scroll-x="700"
          />
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue'
import {
  type DataTableColumns,
  NAlert,
  NButton,
  NDataTable,
  NIcon,
  NNumberAnimation,
  NSkeleton,
  NSpace,
  NTag,
  NText,
} from 'naive-ui'
import {
  CheckmarkCircleOutline,
  CubeOutline,
  FlashOutline,
  GridOutline,
  KeyOutline,
  RefreshOutline,
  ShieldCheckmarkOutline,
  StatsChartOutline,
  TimeOutline,
  TrendingUpOutline,
} from '@vicons/ionicons5'
import type {
  ChannelHealthInfo,
  CircuitSnapshot,
  DashboardFrame,
  DashboardMetrics,
  DashboardQPSRange,
  ModelDetailInfo,
} from '@/services/dashboardService'
import dashboardService from '@/services/dashboardService'
import { useEventStream } from '@/composables/useEventStream'
import QpsChart from '@/components/QpsChart.vue'

const metrics = ref<DashboardMetrics | null>(null)
const circuits = ref<CircuitSnapshot[]>([])
const isLoading = ref(false)
const fallbackError = ref<Error | null>(null)
const lastUpdatedAt = ref('')
const qpsRange = ref<DashboardQPSRange>('1h')

const qpsRangeOptions: Array<{ label: string; value: DashboardQPSRange }> = [
  { label: '1h', value: '1h' },
  { label: '6h', value: '6h' },
  { label: '24h', value: '24h' },
]

const {
  error: streamError,
  isConnected,
  reconnect,
} = useEventStream<DashboardFrame>({
  url: () => `/admin/api/dashboard/metrics/stream?qps_range=${encodeURIComponent(qpsRange.value)}`,
  event: 'metrics',
  onMessage: (frame) => {
    metrics.value = frame.metrics
    circuits.value = frame.circuits || []
    lastUpdatedAt.value = new Date().toISOString()
    fallbackError.value = null
    isLoading.value = false
  },
})

const error = computed(() => fallbackError.value || streamError.value)

const lastUpdated = computed(() => {
  const source = metrics.value?.generated_at || lastUpdatedAt.value
  if (!source) return '刚刚'
  const date = new Date(source)
  if (Number.isNaN(date.getTime())) return '刚刚'
  return date.toLocaleString('zh-CN')
})

const resolvedTotalKeys = computed(() => {
  if (typeof metrics.value?.total_keys === 'number' && metrics.value.total_keys > 0) {
    return metrics.value.total_keys
  }
  return (metrics.value?.channel_health_list || []).reduce((sum, item) => sum + (item.total_keys || 0), 0)
})

const resolvedHealthyKeys = computed(() => {
  if (typeof metrics.value?.overall_health?.healthy_keys === 'number') {
    return metrics.value.overall_health.healthy_keys
  }
  return (metrics.value?.channel_health_list || []).reduce((sum, item) => sum + (item.healthy_keys || 0), 0)
})

const resolvedUnhealthyKeys = computed(() => {
  if (typeof metrics.value?.overall_health?.unhealthy_keys === 'number') {
    return metrics.value.overall_health.unhealthy_keys
  }
  return Math.max(0, resolvedTotalKeys.value - resolvedHealthyKeys.value)
})

const overallHealthValue = computed(() => {
  if (typeof metrics.value?.overall_health?.overall_health === 'number') {
    return metrics.value.overall_health.overall_health
  }
  if (resolvedTotalKeys.value > 0) {
    return (resolvedHealthyKeys.value / resolvedTotalKeys.value) * 100
  }
  const channelList = metrics.value?.channel_health_list || []
  if (!channelList.length) return 0
  const total = channelList.reduce((sum, item) => sum + (item.health_percentage || 0), 0)
  return total / channelList.length
})

const healthIconClass = computed(() => {
  const health = overallHealthValue.value
  if (health >= 90) return 'metric-card__icon--success'
  if (health >= 60) return 'metric-card__icon--warning'
  return 'metric-card__icon--error'
})

const successRateIconClass = computed(() => {
  const successRate = metrics.value?.success_rate || 0
  if (successRate >= 95) return 'metric-card__icon--success'
  if (successRate >= 80) return 'metric-card__icon--warning'
  return 'metric-card__icon--error'
})

const keyHealthIconClass = computed(() => {
  return resolvedUnhealthyKeys.value === 0 ? 'metric-card__icon--success' : 'metric-card__icon--warning'
})

const tableLocale = computed(() => ({
  emptyText: isLoading.value ? '指标加载中...' : '暂无统计数据',
}))

const channelNameMap = computed(() => {
  const map = new Map<number, string>()
  for (const item of metrics.value?.channel_health_list || []) {
    map.set(item.channel_id, item.channel_name)
  }
  return map
})

const channelColumns: DataTableColumns<ChannelHealthInfo> = [
  {
    title: '渠道',
    key: 'channel_name',
    minWidth: 180,
    ellipsis: { tooltip: true },
  },
  {
    title: '状态',
    key: 'status',
    width: 120,
    align: 'center',
    render: (row) =>
      h(
        NTag,
        { type: row.status === 'active' ? 'success' : 'default', size: 'small', bordered: false },
        { default: () => (row.status === 'active' ? '启用' : '停用') },
      ),
  },
  {
    title: '请求量',
    key: 'total_requests',
    width: 120,
    align: 'right',
    render: (row) => formatCompactNumber(row.total_requests),
  },
  {
    title: 'Token（入/出）',
    key: 'token_usage',
    width: 160,
    align: 'right',
    render: (row) => `${formatCompactNumber(row.prompt_tokens)} / ${formatCompactNumber(row.completion_tokens)}`,
  },
  {
    title: '成功率',
    key: 'success_rate',
    width: 120,
    align: 'right',
    render: (row) => `${row.success_rate.toFixed(2)}%`,
  },
  {
    title: '密钥健康',
    key: 'keys',
    width: 120,
    align: 'right',
    render: (row) => `${row.healthy_keys} / ${row.total_keys}`,
  },
]

const circuitColumns: DataTableColumns<CircuitSnapshot> = [
  {
    title: '类型',
    key: 'kind',
    width: 100,
    align: 'center',
    render: (row) =>
      h(
        NTag,
        {
          size: 'small',
          bordered: false,
          type: row.kind === 'key' ? 'warning' : 'info',
        },
        { default: () => (row.kind === 'key' ? '密钥' : '模型') },
      ),
  },
  {
    title: '渠道',
    key: 'channel_id',
    minWidth: 160,
    ellipsis: { tooltip: true },
    render: (row) => channelNameMap.value.get(row.channel_id) || `渠道 #${row.channel_id}`,
  },
  {
    title: '标识',
    key: 'identifier',
    minWidth: 150,
    ellipsis: { tooltip: true },
    render: (row) => {
      if (row.kind === 'key') {
        const tail = String(row.id).slice(-4).padStart(4, '0')
        return h(NText, { code: true }, { default: () => `Key ****${tail}` })
      }
      return h(NText, { code: true }, { default: () => `ModelConfig #${row.id}` })
    },
  },
  {
    title: '连续失败',
    key: 'failure_count',
    width: 120,
    align: 'right',
    render: (row) => row.failure_count || 0,
  },
  {
    title: '状态',
    key: 'state',
    width: 120,
    align: 'center',
    render: (row) =>
      h(
        NTag,
        {
          size: 'small',
          bordered: false,
          type: row.state === 'cooling' ? 'warning' : 'default',
        },
        { default: () => (row.state === 'cooling' ? '冷却中' : '停用') },
      ),
  },
  {
    title: '剩余冷却',
    key: 'remaining_sec',
    width: 120,
    align: 'right',
    render: (row) => (row.state === 'cooling' ? formatRemainingSeconds(row.remaining_sec) : '-'),
  },
]

const modelColumns: DataTableColumns<ModelDetailInfo> = [
  {
    title: '模型名称',
    key: 'model_name',
    minWidth: 240,
    ellipsis: { tooltip: true },
    render: (row) => h(NText, { code: true }, { default: () => row.model_name }),
  },
  {
    title: '总请求',
    key: 'total_requests',
    width: 120,
    align: 'right',
    render: (row) => formatCompactNumber(row.total_requests),
  },
  {
    title: '成功',
    key: 'success_requests',
    width: 120,
    align: 'right',
    render: (row) => formatCompactNumber(row.success_requests),
  },
  {
    title: '失败',
    key: 'failed_requests',
    width: 120,
    align: 'right',
    render: (row) => formatCompactNumber(row.failed_requests),
  },
  {
    title: '成功率',
    key: 'success_rate',
    width: 120,
    align: 'right',
    render: (row) => `${row.success_rate.toFixed(2)}%`,
  },
]

async function loadFallbackData() {
  isLoading.value = true
  try {
    const [metricsData, circuitData] = await Promise.all([
      dashboardService.getMetrics(qpsRange.value),
      dashboardService.getCircuitStatus(),
    ])
    metrics.value = metricsData
    circuits.value = circuitData
    lastUpdatedAt.value = new Date().toISOString()
    fallbackError.value = null
  } catch (fallbackErr) {
    fallbackError.value = fallbackErr instanceof Error ? fallbackErr : new Error(String(fallbackErr))
  } finally {
    isLoading.value = false
  }
}

async function handleRefresh() {
  await loadFallbackData()
  reconnect()
}

async function handleQPSRangeChange(value: DashboardQPSRange) {
  if (qpsRange.value === value) return
  qpsRange.value = value
  await loadFallbackData()
  reconnect()
}

function formatRemainingSeconds(totalSeconds: number): string {
  const safe = Math.max(0, Number.isFinite(totalSeconds) ? Math.floor(totalSeconds) : 0)
  const minutes = Math.floor(safe / 60)
  const seconds = safe % 60
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

function formatCompactNumber(value: number): string {
  if (!Number.isFinite(value)) return '0'

  const units = [
    { value: 1e12, symbol: 'T' },
    { value: 1e9, symbol: 'B' },
    { value: 1e6, symbol: 'M' },
    { value: 1e3, symbol: 'K' },
  ]

  const sign = value < 0 ? '-' : ''
  const absValue = Math.abs(value)

  for (const unit of units) {
    if (absValue >= unit.value) {
      return `${sign}${(absValue / unit.value).toFixed(2)}${unit.symbol}`
    }
  }

  return `${sign}${Math.round(absValue)}`
}

function formatPercent(value: number): string {
  return `${Math.max(0, Math.min(100, value)).toFixed(1)}%`
}

watch(isConnected, (connected) => {
  if (!connected && !metrics.value) {
    void loadFallbackData()
  }
})

onMounted(() => {
  void loadFallbackData()
})
</script>

<style scoped>
.dashboard-page {
  --metric-neutral-bg: #f4f4f5;
  --metric-neutral-fg: #171717;
  --metric-success-bg: #f4f4f5;
  --metric-success-fg: #171717;
  --metric-warning-bg: #ededed;
  --metric-warning-fg: #262626;
  --metric-error-bg: #e3e3e3;
  --metric-error-fg: #111111;
}

.dashboard-skeleton {
  min-height: 280px;
}

.dashboard-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 2px 4px;
}

.dashboard-toolbar__meta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #737373;
}

.range-switch__label {
  font-size: 12px;
  color: #737373;
}

.range-switch {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.metric-cards {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.metric-card {
  position: relative;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 18px;
  background: #ffffff;
  border: 1px solid #e3e3e3;
  border-radius: 14px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02);
  transition: box-shadow 0.2s ease, transform 0.2s ease;
  overflow: hidden;
}

.metric-card::after {
  content: "";
  position: absolute;
  right: -30px;
  top: -30px;
  width: 100px;
  height: 100px;
  border-radius: 50%;
  opacity: 0;
  transition: opacity 0.25s ease;
  pointer-events: none;
}

.metric-card:hover {
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.06);
  transform: translateY(-2px);
  border-color: #d4d4d4;
}

.metric-card__icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  flex-shrink: 0;
}

.metric-card__icon--neutral {
  background: var(--metric-neutral-bg);
  color: var(--metric-neutral-fg);
}

.metric-card__icon--success {
  background: var(--metric-success-bg);
  color: var(--metric-success-fg);
}

.metric-card__icon--warning {
  background: var(--metric-warning-bg);
  color: var(--metric-warning-fg);
}

.metric-card__icon--error {
  background: var(--metric-error-bg);
  color: var(--metric-error-fg);
}

.metric-card__content {
  min-width: 0;
  flex: 1;
}

.metric-card__label {
  font-size: 11px;
  color: #737373;
  font-weight: 580;
  margin-bottom: 6px;
  letter-spacing: 0.2px;
}

.metric-card__value {
  font-size: 22px;
  font-weight: 720;
  line-height: 1.1;
  color: #111111;
  font-variant-numeric: tabular-nums;
  display: flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
}

.metric-card__value--compact {
  font-size: 17px;
}

.metric-card__unit {
  font-size: 12px;
  font-weight: 500;
  color: #737373;
}

.metric-card__subtext {
  margin-top: 4px;
  font-size: 11px;
  color: #737373;
  display: flex;
  align-items: center;
  gap: 4px;
}

.metric-card__trend--up {
  color: #171717;
}

.metric-card__trend--down {
  color: #525252;
}

@media (max-width: 1280px) {
  .metric-cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .metric-cards {
    grid-template-columns: 1fr;
  }
}
</style>
