<template>
  <div class="dashboard-container">
    <!--    &lt;!&ndash; 页面标题 &ndash;&gt;-->
    <!--    <div class="page-header">-->
    <!--      <h1 class="page-title">仪表盘</h1>-->
    <!--      <n-text depth="3" class="page-subtitle">实时监控系统状态</n-text>-->
    <!--    </div>-->

    <!-- 顶部统计卡片 -->
    <n-grid :cols="4" :x-gap="16" :y-gap="16" responsive="screen" class="stats-grid">
      <!-- 当前 QPS -->
      <n-grid-item>
        <div class="metric-card metric-card--blue">
          <div class="metric-card__header">
            <div class="metric-card__icon">
              <n-icon size="20">
                <FlashIcon/>
              </n-icon>
            </div>
            <n-tag :bordered="false" type="info" size="small">
              实时
            </n-tag>
          </div>
          <div class="metric-card__value">
            <n-number-animation
                :from="0"
                :to="metrics?.current_qps || 0"
                :active="true"
            />
          </div>
          <div class="metric-card__label">当前 QPS</div>
        </div>
      </n-grid-item>

      <!-- 今日请求 -->
      <n-grid-item>
        <div class="metric-card metric-card--green">
          <div class="metric-card__header">
            <div class="metric-card__icon">
              <n-icon size="20">
                <TrendingUpIcon/>
              </n-icon>
            </div>
          </div>
          <div class="metric-card__value">
            {{ formatNumber(metrics?.total_requests_today || 0) }}
          </div>
          <div class="metric-card__label">今日请求总数</div>
        </div>
      </n-grid-item>

      <!-- 成功率 -->
      <n-grid-item>
        <div class="metric-card metric-card--emerald">
          <div class="metric-card__header">
            <div class="metric-card__icon">
              <n-icon size="20">
                <CheckmarkCircleIcon/>
              </n-icon>
            </div>
            <n-tag
                :bordered="false"
                :type="(metrics?.today_success_rate?.success_rate || 0) >= 95 ? 'success' : 'warning'"
                size="small"
            >
              {{ (metrics?.today_success_rate?.success_rate || 0).toFixed(1) }}%
            </n-tag>
          </div>
          <div class="metric-card__value">
            <n-number-animation
                :from="0"
                :to="metrics?.today_success_rate?.success_rate || 0"
                :precision="1"
                :active="true"
            />
            <span class="metric-card__unit">%</span>
          </div>
          <div class="metric-card__label">今日成功率</div>
        </div>
      </n-grid-item>

      <!-- 活跃渠道 -->
      <n-grid-item>
        <div class="metric-card metric-card--violet">
          <div class="metric-card__header">
            <div class="metric-card__icon">
              <n-icon size="20">
                <ServerIcon/>
              </n-icon>
            </div>
          </div>
          <div class="metric-card__value">
            {{ metrics?.active_channels || 0 }}
            <span class="metric-card__unit">/ {{ metrics?.total_channels || 0 }}</span>
          </div>
          <div class="metric-card__label">活跃渠道</div>
        </div>
      </n-grid-item>
    </n-grid>

    <!-- QPS 趋势图 -->
    <n-card
        title="QPS 趋势"
        size="small"
        :bordered="false"
        class="chart-card"
    >
      <template #header-extra>
        <n-tag :bordered="false" type="info" size="small">
          近 1 小时
        </n-tag>
      </template>
      <QpsChart :data="metrics?.qps_time_series || []"/>
    </n-card>

    <!-- 渠道统计概览 -->
    <n-card
        title="渠道统计"
        size="small"
        :bordered="false"
        class="channel-stats-card">
      <n-space vertical :size="24">
        <n-data-table
            :columns="channelColumns"
            :data="metrics?.channel_health_list || []"
            :pagination="false"
            :bordered="true"
            :single-line="true"
            striped
            size="small"
        />
      </n-space>
    </n-card>

    <!-- 模型统计 -->
    <n-card
        title="模型统计"
        size="small"
        :bordered="false"
        class="model-stats-card"
    >
      <n-space vertical :size="24">
        <n-grid :cols="4" :x-gap="16" :y-gap="16" responsive="screen" style="max-width: 1040px;">
          <n-grid-item>
            <div class="stat-item stat-item--primary">
              <div class="stat-item__icon">
                <n-icon size="24">
                  <CubeIcon/>
                </n-icon>
              </div>
              <div class="stat-item__content">
                <div class="stat-item__value">{{ metrics?.model_stats?.active_models || 0 }}</div>
                <div class="stat-item__label">活跃模型</div>
              </div>
            </div>
          </n-grid-item>
          <n-grid-item>
            <div class="stat-item stat-item--info">
              <div class="stat-item__icon">
                <n-icon size="24">
                  <StatsChartIcon/>
                </n-icon>
              </div>
              <div class="stat-item__content">
                <div class="stat-item__value">{{ formatNumber(metrics?.model_stats?.total_requests || 0) }}</div>
                <div class="stat-item__label">请求总数</div>
              </div>
            </div>
          </n-grid-item>
          <n-grid-item>
            <div class="stat-item stat-item--success">
              <div class="stat-item__icon">
                <n-icon size="24">
                  <CheckmarkCircleIcon/>
                </n-icon>
              </div>
              <div class="stat-item__content">
                <div class="stat-item__value">{{ formatNumber(metrics?.model_stats?.success_requests || 0) }}</div>
                <div class="stat-item__label">成功请求</div>
              </div>
            </div>
          </n-grid-item>
          <n-grid-item>
            <div class="stat-item stat-item--error">
              <div class="stat-item__icon">
                <n-icon size="24">
                  <CloseCircleIcon/>
                </n-icon>
              </div>
              <div class="stat-item__content">
                <div class="stat-item__value">{{ formatNumber(metrics?.model_stats?.failed_requests || 0) }}</div>
                <div class="stat-item__label">失败请求</div>
              </div>
            </div>
          </n-grid-item>
        </n-grid>

        <n-data-table
            :columns="modelColumns"
            :data="metrics?.model_stats?.model_list || []"
            :pagination="false"
            :bordered="true"
            :single-line="true"
            striped
            size="small"
        />
      </n-space>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import {h, onMounted} from 'vue'
import {
  type DataTableColumns,
  NCard,
  NDataTable,
  NGrid,
  NGridItem,
  NIcon,
  NNumberAnimation,
  NProgress,
  NSpace,
  NTag
} from 'naive-ui'
import {
  CheckmarkCircle as CheckmarkCircleIcon,
  CloseCircle as CloseCircleIcon,
  Cube as CubeIcon,
  Flash as FlashIcon,
  Server as ServerIcon,
  StatsChart as StatsChartIcon,
  TrendingUp as TrendingUpIcon
} from '@vicons/ionicons5'
import type {ChannelHealthInfo, DashboardMetrics, ModelDetailInfo} from '@/services/dashboardService'
import dashboardService from '@/services/dashboardService'
import {usePolling} from '@/composables/usePolling'
import QpsChart from '@/components/QpsChart.vue'

// 使用轮询获取仪表盘数据（每 5 秒）
const {data: metrics, error} = usePolling<DashboardMetrics>(
    () => dashboardService.getMetrics(),
    5000,
    true
)

// 渠道表格列定义
const channelColumns: DataTableColumns<ChannelHealthInfo> = [
  {
    title: '渠道名称',
    key: 'channel_name',
    width: 120,
    ellipsis: {tooltip: true},
    sorter: (a, b) => a.channel_name.localeCompare(b.channel_name)
  },
  {
    title: '状态',
    key: 'status',
    width: 80,
    align: 'center',
    render: (row) => {
      const statusMap: Record<string, { type: 'success' | 'warning' | 'error', text: string }> = {
        'active': {type: 'success', text: '激活'},
        'disabled': {type: 'warning', text: '禁用'}
      }
      const status = statusMap[row.status] || {type: 'error', text: '未知'}
      return h(NTag, {type: status.type, size: 'small', bordered: false}, {default: () => status.text})
    }
  },
  {
    title: '请求总数',
    key: 'total_requests',
    width: 80,
    align: 'right',
    render: (row) => formatNumber(row.total_requests),
    sorter: (a, b) => a.total_requests - b.total_requests
  },
  {
    title: '成功率',
    key: 'success_rate',
    align: 'center',
    width: 120,
    sorter: (a, b) => a.success_rate - b.success_rate,
    render: (row) => {
      const rate = row.success_rate
      const type = rate >= 95 ? 'success' : rate >= 80 ? 'warning' : 'error'
      return h('div', {class: 'flex items-center gap-2 pl-2 pr-2'}, [
        h(NProgress, {
          type: 'line',
          percentage: parseFloat(rate.toFixed(2)),
          showIndicator: true,
          height: 12,
          borderRadius: 3,
          indicatorPlacement: "inside",
          color: type === 'success' ? '#10b981' : type === 'warning' ? '#f59e0b' : '#ef4444'
        })
      ])
    }
  },
  {
    title: '密钥健康',
    key: 'healthy_keys',
    width: 80,
    align: 'right',
    render: (row) => `${row.healthy_keys} / ${row.total_keys}`
  },
  {
    title: '健康度',
    key: 'health_percentage',
    width: 80,
    align: 'right',
    render: (row) => {
      const percentage = row.health_percentage
      const type = percentage >= 80 ? 'success' : percentage >= 50 ? 'warning' : 'error'
      return h(NTag, {type, size: 'small', bordered: false}, {default: () => `${percentage.toFixed(0)}%`})
    }
  },
  {
    title: '优先级',
    key: 'priority',
    width: 80,
    align: 'right'
  },
  {
    title: '权重',
    key: 'weight',
    width: 80,
    align: 'right'
  }
]

// 模型表格列定义
const modelColumns: DataTableColumns<ModelDetailInfo> = [
  {
    title: '模型名称',
    key: 'model_name',
    width: 120,
    ellipsis: {tooltip: true},
    sorter: (a, b) => a.model_name.localeCompare(b.model_name)
  },
  {
    title: '请求总数',
    key: 'total_requests',
    width: 120,
    align: 'right',
    render: (row) => formatNumber(row.total_requests),
    sorter: (a, b) => a.total_requests - b.total_requests
  },
  {
    title: '成功请求',
    key: 'success_requests',
    width: 120,
    align: 'right',
    render: (row) => formatNumber(row.success_requests),
    sorter: (a, b) => a.success_requests - b.success_requests
  },
  {
    title: '失败请求',
    key: 'failed_requests',
    width: 120,
    align: 'right',
    render: (row) => formatNumber(row.failed_requests),
    sorter: (a, b) => a.failed_requests - b.failed_requests
  },
  {
    title: '成功率',
    key: 'success_rate',
    align: 'center',
    width: 120,
    sorter: (a, b) => a.success_rate - b.success_rate,
    render: (row) => {
      const rate = row.success_rate
      const type = rate >= 95 ? 'success' : rate >= 80 ? 'warning' : 'error'
      return h('div', {class: 'flex items-center gap-2 pl-2 pr-2'}, [
        h(NProgress, {
          type: 'line',
          percentage: parseFloat(rate.toFixed(2)),
          showIndicator: true,
          height: 12,
          borderRadius: 3,
          indicatorPlacement: "inside",
          color: type === 'success' ? '#10b981' : type === 'warning' ? '#f59e0b' : '#ef4444'
        })
      ])
    }
  }
]


// 格式化数字（添加千分位）
function formatNumber(value: number): string {
  return value.toLocaleString()
}


onMounted(() => {
  if (error.value) {
    console.error('Failed to load dashboard metrics:', error.value)
  }
})
</script>

<style scoped>

/* 统计网格 */
.stats-grid {
  margin-bottom: 24px;
  max-width: 1040px;
}

/* 卡片间距 */
.chart-card,
.channel-stats-card,
.channel-details-card,
.model-stats-card {
  margin-bottom: 24px;
}

/* 统计卡片 - 现代设计 */
.metric-card {
  position: relative;
  background: #ffffff;
  border-radius: 12px;
  padding: 20px;
  border: 1px solid #e5e7eb;
  transition: all 0.2s ease;
  overflow: hidden;
}

.metric-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: linear-gradient(90deg, var(--card-color), var(--card-color-light));
}

.metric-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 16px rgba(0, 0, 0, 0.08);
  border-color: var(--card-color);
}

.metric-card--blue {
  --card-color: #3b82f6;
  --card-color-light: #60a5fa;
}

.metric-card--green {
  --card-color: #22c55e;
  --card-color-light: #4ade80;
}

.metric-card--emerald {
  --card-color: #10b981;
  --card-color-light: #34d399;
}

.metric-card--violet {
  --card-color: #8b5cf6;
  --card-color-light: #a78bfa;
}

.metric-card__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.metric-card__icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--card-color), var(--card-color-light));
  color: white;
}

.metric-card__value {
  font-size: 32px;
  font-weight: 700;
  color: #0f172a;
  line-height: 1;
  margin-bottom: 8px;
  letter-spacing: -1px;
}

.metric-card__unit {
  font-size: 16px;
  font-weight: 500;
  color: #64748b;
  margin-left: 4px;
}

.metric-card__label {
  font-size: 13px;
  font-weight: 500;
  color: #64748b;
  text-transform: none;
  letter-spacing: 0;
}

/* 图表卡片 */
.chart-card,
.channel-stats-card,
.channel-details-card,
.model-stats-card {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  transition: all 0.2s ease;
}

.chart-card:hover,
.channel-stats-card:hover,
.channel-details-card:hover,
.model-stats-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

/* 卡片标题样式 */
:deep(.n-card__header) {
  padding: 16px 20px;
  border-bottom: 1px solid #f1f5f9;
}

:deep(.n-card__content) {
  padding: 20px;
}

/* 统计项样式 */
.stat-item {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  background: #f8fafc;
  border-radius: 12px;
  border: 1px solid #e5e7eb;
  transition: all 0.2s ease;
}

.stat-item:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
  border-color: var(--stat-color);
}

.stat-item--total {
  --stat-color: #64748b;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
}

.stat-item--healthy {
  --stat-color: #10b981;
  background: linear-gradient(135deg, #f0fdf4 0%, #dcfce7 100%);
  border-color: #86efac;
}

.stat-item--degraded {
  --stat-color: #f59e0b;
  background: linear-gradient(135deg, #fffbeb 0%, #fef3c7 100%);
  border-color: #fcd34d;
}

.stat-item--unhealthy {
  --stat-color: #ef4444;
  background: linear-gradient(135deg, #fef2f2 0%, #fee2e2 100%);
  border-color: #fca5a5;
}

.stat-item--primary {
  --stat-color: #3b82f6;
  background: linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%);
  border-color: #93c5fd;
}

.stat-item--info {
  --stat-color: #06b6d4;
  background: linear-gradient(135deg, #ecfeff 0%, #cffafe 100%);
  border-color: #67e8f9;
}

.stat-item--success {
  --stat-color: #10b981;
  background: linear-gradient(135deg, #f0fdf4 0%, #dcfce7 100%);
  border-color: #86efac;
}

.stat-item--error {
  --stat-color: #ef4444;
  background: linear-gradient(135deg, #fef2f2 0%, #fee2e2 100%);
  border-color: #fca5a5;
}

.stat-item__icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: white;
  color: var(--stat-color);
  flex-shrink: 0;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.stat-item__content {
  flex: 1;
  min-width: 0;
}

.stat-item__value {
  font-size: 28px;
  font-weight: 700;
  color: #0f172a;
  line-height: 1;
  margin-bottom: 6px;
  letter-spacing: -0.5px;
}

.stat-item__label {
  font-size: 13px;
  font-weight: 500;
  color: #64748b;
}


/* 数字动画样式 */
:deep(.n-number-animation) {
  font-variant-numeric: tabular-nums;
}

/* 标签样式 */
:deep(.n-tag) {
  border-radius: 6px;
  font-weight: 500;
}

/* 分隔线样式 */
:deep(.n-divider) {
  border-color: #f1f5f9;
}

/* 进度条样式 */
:deep(.n-progress .n-progress-graph-line-rail) {
  background: #f1f5f9;
}

</style>
