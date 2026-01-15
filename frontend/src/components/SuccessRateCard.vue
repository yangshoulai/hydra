<template>
  <div class="success-rate-card flex flex-col gap-1">
    <div class="label text-sm opacity-90">今日成功率</div>
    <div class="value text-4xl font-semibold leading-none">{{ formattedSuccessRate }}%</div>
    <div class="trend flex items-center gap-1 text-xs opacity-80" :class="trendClass">
      <svg v-if="props.successRate >= 95" class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
        <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
      </svg>
      <svg v-else-if="props.successRate >= 60" class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
        <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
      </svg>
      <svg v-else class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
      </svg>
      <span class="font-medium">{{ trendText }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  successRate: number
}

const props = defineProps<Props>()

// 格式化成功率（保留 2 位小数）
const formattedSuccessRate = computed(() => {
  return props.successRate.toFixed(2)
})

// 根据成功率确定趋势
const trendClass = computed(() => {
  if (props.successRate >= 95) return 'text-green-400'
  if (props.successRate >= 80) return 'text-emerald-400'
  if (props.successRate >= 60) return 'text-amber-400'
  return 'text-red-400'
})

const trendText = computed(() => {
  if (props.successRate >= 95) return '优秀'
  if (props.successRate >= 80) return '良好'
  if (props.successRate >= 60) return '一般'
  return '较差'
})
</script>

<style scoped>
/* 使用 Tailwind 完全样式化，无需额外 CSS */
</style>
