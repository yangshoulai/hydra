<template>
  <div class="dashboard-container">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">仪表盘</h1>
      <n-text depth="3" class="page-subtitle">实时监控系统状态</n-text>
    </div>

    <!-- 顶部统计卡片 -->
    <n-grid :cols="4" :x-gap="16" :y-gap="16" responsive="screen" class="stats-grid">
      <!-- 当前 QPS -->
      <n-grid-item>
        <div class="metric-card metric-card--blue">
          <div class="metric-card__header">
            <div class="metric-card__icon">
              <n-icon size="20">
                <FlashIcon />
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
                <TrendingUpIcon />
              </n-icon>
            </div>
          </div>
          <div class="metric-card__value">
            <n-number-animation
              :from="0"
              :to="metrics?.total_requests_today || 0"
              :active="true"
              :format="formatNumber"
            />
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
                <CheckmarkCircleIcon />
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
                <ServerIcon />
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

    <!-- 主要内容区域 -->
    <n-grid :cols="1" :x-gap="16" :y-gap="16" responsive="screen" class="content-grid">
      <!-- QPS 趋势图 -->
      <n-grid-item span="1 s:1 xl:3">
        <n-card
          title="QPS 趋势"
          size="small"
          :bordered="false"
          class="chart-card"
        >
          <template #header-extra>
            <n-space>
              <n-tag :bordered="false" type="info" size="small">
                近 1 小时
              </n-tag>
            </n-space>
          </template>
          <QpsChart :data="metrics?.qps_time_series || []" />
        </n-card>
      </n-grid-item>

      <!-- 系统健康状态 -->
      <n-grid-item span="1 s:1 xl:1">
        <n-card
          title="系统健康"
          size="small"
          :bordered="false"
          class="health-card"
        >
          <n-space vertical :size="12">
            <!-- 渠道状态卡片 -->
            <div class="health-cards">
              <div class="health-card-item health-card-item--total">
                <div class="health-card-item__icon">
                  <n-icon size="16">
                    <ServerIcon />
                  </n-icon>
                </div>
                <div class="health-card-item__content">
                  <div class="health-card-item__value">
                    {{ metrics?.overall_health?.total_channels || 0 }}
                  </div>
                  <div class="health-card-item__label">总渠道</div>
                </div>
              </div>

              <div class="health-card-item health-card-item--healthy">
                <div class="health-card-item__icon">
                  <n-icon size="16">
                    <CheckmarkCircleIcon />
                  </n-icon>
                </div>
                <div class="health-card-item__content">
                  <div class="health-card-item__value">
                    {{ metrics?.overall_health?.healthy_channels || 0 }}
                  </div>
                  <div class="health-card-item__label">健康</div>
                </div>
              </div>

              <div class="health-card-item health-card-item--degraded">
                <div class="health-card-item__icon">
                  <n-icon size="16">
                    <WarningIcon />
                  </n-icon>
                </div>
                <div class="health-card-item__content">
                  <div class="health-card-item__value">
                    {{ metrics?.overall_health?.degraded_channels || 0 }}
                  </div>
                  <div class="health-card-item__label">降级</div>
                </div>
              </div>

              <div class="health-card-item health-card-item--unhealthy">
                <div class="health-card-item__icon">
                  <n-icon size="16">
                    <CloseCircleIcon />
                  </n-icon>
                </div>
                <div class="health-card-item__content">
                  <div class="health-card-item__value">
                    {{ metrics?.overall_health?.unhealthy_channels || 0 }}
                  </div>
                  <div class="health-card-item__label">异常</div>
                </div>
              </div>
            </div>

            <n-divider style="margin: 12px 0;" />

            <!-- 密钥状态 -->
            <div class="key-status">
              <div class="key-status__header">
                <div class="key-status__icon">
                  <n-icon size="18" color="#8b5cf6">
                    <KeyIcon />
                  </n-icon>
                </div>
                <div class="key-status__title">密钥状态</div>
              </div>
              <div class="key-status__metrics">
                <div class="key-status__metric">
                  <span class="key-status__metric-value key-status__metric-value--success">
                    {{ metrics?.overall_health?.healthy_keys || 0 }}
                  </span>
                  <span class="key-status__metric-label">健康</span>
                </div>
                <div class="key-status__divider"></div>
                <div class="key-status__metric">
                  <span class="key-status__metric-value">
                    {{ metrics?.overall_health?.total_keys || 0 }}
                  </span>
                  <span class="key-status__metric-label">总数</span>
                </div>
              </div>
            </div>

            <!-- 健康度进度条 -->
            <div class="health-overview">
              <div class="health-overview__label">
                整体健康度
                <span class="health-overview__percentage">
                  {{ calculatePercentage(metrics?.overall_health?.healthy_channels || 0, metrics?.overall_health?.total_channels || 1) }}%
                </span>
              </div>
              <n-progress
                type="line"
                :percentage="calculatePercentage(metrics?.overall_health?.healthy_channels || 0, metrics?.overall_health?.total_channels || 1)"
                :show-indicator="false"
                :color="getHealthColor(calculatePercentage(metrics?.overall_health?.healthy_channels || 0, metrics?.overall_health?.total_channels || 1))"
                :height="8"
                :border-radius="4"
              />
            </div>
          </n-space>
        </n-card>
      </n-grid-item>
    </n-grid>

    <!-- 渠道健康状态 -->
    <n-card
      title="渠道状态详情"
      size="small"
      :bordered="false"
      class="channels-card channels-card--spaced"
    >
      <template #header-extra>
        <n-tag :bordered="false" type="default" size="small">
          共 {{ metrics?.channel_health_list?.length || 0 }} 个渠道
        </n-tag>
      </template>
      <ChannelHealthWall :channels="metrics?.channel_health_list || []" />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import {
  NCard,
  NGrid,
  NGridItem,
  NIcon,
  NSpace,
  NProgress,
  NDivider,
  NNumberAnimation,
  NTag,
  NText
} from 'naive-ui'
import {
  Flash as FlashIcon,
  TrendingUp as TrendingUpIcon,
  CheckmarkCircle as CheckmarkCircleIcon,
  Server as ServerIcon,
  Warning as WarningIcon,
  CloseCircle as CloseCircleIcon,
  Key as KeyIcon
} from '@vicons/ionicons5'
import dashboardService from '@/services/dashboardService'
import { usePolling } from '@/composables/usePolling'
import type { DashboardMetrics } from '@/services/dashboardService'
import QpsChart from '@/components/QpsChart.vue'
import ChannelHealthWall from '@/components/ChannelHealthWall.vue'

// 使用轮询获取仪表盘数据（每 5 秒）
const { data: metrics, error } = usePolling<DashboardMetrics>(
  () => dashboardService.getMetrics(),
  5000,
  true
)

// 计算百分比
function calculatePercentage(value: number, total: number): number {
  if (total === 0) return 0
  return Math.round((value / total) * 100)
}

// 格式化数字（添加千分位）
function formatNumber(value: number): string {
  return value.toLocaleString()
}

// 根据健康度返回颜色
function getHealthColor(percentage: number): string {
  if (percentage >= 80) return '#10b981'
  if (percentage >= 50) return '#f59e0b'
  return '#ef4444'
}

onMounted(() => {
  if (error.value) {
    console.error('Failed to load dashboard metrics:', error.value)
  }
})
</script>

<style scoped>
/* 容器 */
.dashboard-container {
  padding: 0;
  margin: 0 auto;
}

/* 页面标题 */
.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px 0;
  letter-spacing: -0.5px;
}

.page-subtitle {
  font-size: 14px;
  color: #64748b;
}

/* 统计网格 */
.stats-grid {
  margin-bottom: 24px;
}

/* 内容网格 */
.content-grid {
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
.health-card,
.channels-card {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  transition: all 0.2s ease;
}

.chart-card:hover,
.health-card:hover,
.channels-card:hover {
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

/* 健康进度条 */
.health-progress {
  margin-bottom: 4px;
}

.health-progress__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.health-progress__label {
  font-size: 13px;
  font-weight: 500;
  color: #475569;
}

.health-progress__value {
  font-size: 14px;
  font-weight: 600;
}

.health-progress__value--success {
  color: #10b981;
}

.health-progress__value--warning {
  color: #f59e0b;
}

.health-progress__value--error {
  color: #ef4444;
}

/* 健康卡片网格 */
.health-cards {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.health-card-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background: #f8fafc;
  border-radius: 8px;
  border: 1px solid #e5e7eb;
  transition: all 0.2s ease;
}

.health-card-item:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.05);
  border-color: var(--item-color);
}

.health-card-item--total {
  --item-color: #64748b;
}

.health-card-item--healthy {
  --item-color: #10b981;
  background: linear-gradient(135deg, #f0fdf4 0%, #dcfce7 100%);
  border-color: #86efac;
}

.health-card-item--degraded {
  --item-color: #f59e0b;
  background: linear-gradient(135deg, #fffbeb 0%, #fef3c7 100%);
  border-color: #fcd34d;
}

.health-card-item--unhealthy {
  --item-color: #ef4444;
  background: linear-gradient(135deg, #fef2f2 0%, #fee2e2 100%);
  border-color: #fca5a5;
}

.health-card-item__icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: white;
  color: var(--item-color);
  flex-shrink: 0;
}

.health-card-item__content {
  flex: 1;
  min-width: 0;
}

.health-card-item__value {
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
  line-height: 1;
  margin-bottom: 2px;
}

.health-card-item__label {
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
}

/* 密钥状态 */
.key-status {
  padding: 16px;
  background: linear-gradient(135deg, #faf5ff 0%, #f3e8ff 100%);
  border-radius: 10px;
  border: 1px solid #e9d5ff;
}

.key-status__header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}

.key-status__icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: linear-gradient(135deg, #a78bfa 0%, #8b5cf6 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
}

.key-status__title {
  font-size: 14px;
  font-weight: 600;
  color: #6b21a8;
}

.key-status__metrics {
  display: flex;
  align-items: center;
  justify-content: space-around;
}

.key-status__metric {
  flex: 1;
  text-align: center;
}

.key-status__metric-value {
  display: block;
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
  line-height: 1;
  margin-bottom: 4px;
}

.key-status__metric-value--success {
  color: #10b981;
}

.key-status__metric-label {
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
}

.key-status__divider {
  width: 1px;
  height: 40px;
  background: #e9d5ff;
  margin: 0 20px;
}

/* 健康度概览 */
.health-overview {
  padding: 12px;
  background: #f8fafc;
  border-radius: 8px;
}

.health-overview__label {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
  font-weight: 500;
  color: #475569;
  margin-bottom: 12px;
}

.health-overview__percentage {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
}

/* 密钥统计 */
.key-stats {
  display: flex;
  align-items: center;
  justify-content: space-around;
  padding: 16px;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  border-radius: 8px;
}

.key-stats__item {
  flex: 1;
  text-align: center;
}

.key-stats__label {
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
  margin-bottom: 8px;
}

.key-stats__value {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
  line-height: 1;
}

.key-stats__value--success {
  color: #10b981;
}

.key-stats__divider {
  width: 1px;
  height: 40px;
  background: #e2e8f0;
  margin: 0 20px;
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

/* 响应式设计 */
@media (max-width: 1024px) {
  .stats-grid :deep(.n-grid) {
    grid-template-columns: repeat(2, 1fr) !important;
  }
}

@media (max-width: 640px) {
  .stats-grid :deep(.n-grid) {
    grid-template-columns: 1fr !important;
  }

  .health-cards {
    grid-template-columns: 1fr !important;
  }

  .page-title {
    font-size: 24px;
  }

  .metric-card {
    padding: 16px;
  }

  .metric-card__value {
    font-size: 28px;
  }

  .key-stats {
    flex-direction: column;
    gap: 16px;
  }

  .key-stats__divider {
    width: 100%;
    height: 1px;
    margin: 0;
  }

  .key-status__metrics {
    flex-direction: column;
    gap: 16px;
  }

  .key-status__divider {
    width: 100%;
    height: 1px;
    margin: 0;
  }
}
</style>
