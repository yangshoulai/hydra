<template>
  <div class="channel-health-wall">
    <n-grid :cols="1" :x-gap="16" :y-gap="16" responsive="screen">
      <n-grid-item v-for="channel in channels" :key="channel.channel_id" span="1 s:1 xl:1">
        <div
          class="channel-card"
          :class="getHealthClass(channel.health_percentage)"
        >
          <!-- 卡片头部 -->
          <div class="channel-card__header">
            <div class="channel-card__title">
              <div class="channel-card__name">{{ channel.channel_name }}</div>
              <n-tag
                :type="getStatusType(channel.status)"
                size="small"
                :bordered="false"
              >
                {{ channel.status === 'active' ? '运行中' : '已禁用' }}
              </n-tag>
            </div>
            <div class="channel-card__health-score">
              <span class="channel-card__health-value">{{ channel.health_percentage.toFixed(1) }}%</span>
            </div>
          </div>

          <!-- 健康度进度条 -->
          <div class="channel-card__progress">
            <n-progress
              type="line"
              :percentage="channel.health_percentage"
              :show-indicator="false"
              :color="getProgressColor(channel.health_percentage)"
              :height="6"
              :border-radius="3"
            />
          </div>

          <!-- 统计数据 -->
          <div class="channel-card__stats">
            <div class="channel-card__stat">
              <div class="channel-card__stat-icon channel-card__stat-icon--success">
                <n-icon size="14">
                  <CheckmarkCircleIcon />
                </n-icon>
              </div>
              <div class="channel-card__stat-content">
                <div class="channel-card__stat-value">{{ channel.success_rate.toFixed(1) }}%</div>
                <div class="channel-card__stat-label">成功率</div>
              </div>
            </div>

            <div class="channel-card__stat">
              <div class="channel-card__stat-icon channel-card__stat-icon--info">
                <n-icon size="14">
                  <KeyIcon />
                </n-icon>
              </div>
              <div class="channel-card__stat-content">
                <div class="channel-card__stat-value">
                  {{ channel.healthy_keys }}/{{ channel.total_keys }}
                </div>
                <div class="channel-card__stat-label">密钥</div>
              </div>
            </div>

            <div class="channel-card__stat">
              <div class="channel-card__stat-icon channel-card__stat-icon--primary">
                <n-icon size="14">
                  <TrendingUpIcon />
                </n-icon>
              </div>
              <div class="channel-card__stat-content">
                <div class="channel-card__stat-value">
                  {{ channel.total_requests.toLocaleString() }}
                </div>
                <div class="channel-card__stat-label">今日请求</div>
              </div>
            </div>
          </div>
        </div>
      </n-grid-item>

      <!-- 空状态 -->
      <n-grid-item v-if="channels.length === 0">
        <div class="empty-state">
          <n-empty description="暂无渠道数据" size="large">
            <template #icon>
              <n-icon size="48" :depth="3">
                <ServerIcon />
              </n-icon>
            </template>
          </n-empty>
        </div>
      </n-grid-item>
    </n-grid>
  </div>
</template>

<script setup lang="ts">
import { NGrid, NGridItem, NTag, NProgress, NEmpty, NIcon } from 'naive-ui'
import {
  CheckmarkCircle as CheckmarkCircleIcon,
  Key as KeyIcon,
  TrendingUp as TrendingUpIcon,
  Server as ServerIcon
} from '@vicons/ionicons5'
import type { ChannelHealthInfo } from '@/services/dashboardService'

interface Props {
  channels: ChannelHealthInfo[]
}

defineProps<Props>()

// 根据健康度获取样式类
const getHealthClass = (percentage: number): string => {
  if (percentage >= 80) return 'health-excellent'
  if (percentage >= 60) return 'health-good'
  if (percentage >= 40) return 'health-warning'
  return 'health-poor'
}

// 获取状态标签类型
const getStatusType = (status: string) => {
  return status === 'active' ? 'success' : 'default'
}

// 获取进度条颜色
const getProgressColor = (percentage: number): string => {
  if (percentage >= 80) return '#10b981'
  if (percentage >= 60) return '#34d399'
  if (percentage >= 40) return '#f59e0b'
  return '#ef4444'
}
</script>

<style scoped>
.channel-health-wall {
  padding: 4px 0;
}

/* 渠道卡片 */
.channel-card {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 16px;
  transition: all 0.2s ease;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.channel-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 16px rgba(0, 0, 0, 0.1);
}

/* 健康状态颜色 */
.health-excellent {
  border-top: 3px solid #10b981;
}

.health-excellent:hover {
  border-top-color: #059669;
  box-shadow: 0 8px 16px rgba(16, 185, 129, 0.15);
}

.health-good {
  border-top: 3px solid #34d399;
}

.health-good:hover {
  border-top-color: #10b981;
  box-shadow: 0 8px 16px rgba(52, 211, 153, 0.15);
}

.health-warning {
  border-top: 3px solid #f59e0b;
}

.health-warning:hover {
  border-top-color: #d97706;
  box-shadow: 0 8px 16px rgba(245, 158, 11, 0.15);
}

.health-poor {
  border-top: 3px solid #ef4444;
}

.health-poor:hover {
  border-top-color: #dc2626;
  box-shadow: 0 8px 16px rgba(239, 68, 68, 0.15);
}

/* 卡片头部 */
.channel-card__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
}

.channel-card__title {
  flex: 1;
  min-width: 0;
}

.channel-card__name {
  font-size: 16px;
  font-weight: 600;
  color: #0f172a;
  margin-bottom: 6px;
}

.channel-card__health-score {
  text-align: right;
}

.channel-card__health-value {
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
}

/* 进度条 */
.channel-card__progress {
  margin-bottom: 16px;
}

/* 统计数据 */
.channel-card__stats {
  display: flex;
  gap: 16px;
}

.channel-card__stat {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
}

.channel-card__stat-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.channel-card__stat-icon--success {
  background: linear-gradient(135deg, #d1fae5 0%, #a7f3d0 100%);
  color: #059669;
}

.channel-card__stat-icon--info {
  background: linear-gradient(135deg, #dbeafe 0%, #bfdbfe 100%);
  color: #0284c7;
}

.channel-card__stat-icon--primary {
  background: linear-gradient(135deg, #e9d5ff 0%, #c4b5fd 100%);
  color: #7c3aed;
}

.channel-card__stat-content {
  flex: 1;
  min-width: 0;
}

.channel-card__stat-value {
  font-size: 16px;
  font-weight: 600;
  color: #0f172a;
  line-height: 1.2;
  margin-bottom: 2px;
}

.channel-card__stat-label {
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
}

/* 空状态 */
.empty-state {
  padding: 48px 24px;
  text-align: center;
}

/* 响应式设计 */
@media (max-width: 640px) {
  .channel-card__stats {
    flex-direction: column;
    gap: 12px;
  }

  .channel-card__stat {
    width: 100%;
  }
}
</style>
