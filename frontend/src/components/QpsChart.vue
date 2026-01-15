<template>
  <div ref="chartRef" class="qps-chart" :style="{ height: `${height}px` }"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, onUnmounted } from 'vue'
import * as echarts from 'echarts'
import type { QPSDataPoint } from '@/services/dashboardService'

interface Props {
  data: QPSDataPoint[]
  height?: number
}

const props = withDefaults(defineProps<Props>(), {
  height: 300,
})

const chartRef = ref<HTMLDivElement>()
let chartInstance: echarts.ECharts | null = null

// 初始化图表
const initChart = () => {
  if (!chartRef.value) return

  chartInstance = echarts.init(chartRef.value)
  updateChart()
}

// 更新图表数据
const updateChart = () => {
  if (!chartInstance || !props.data) return

  const timestamps = props.data.map((d) => d.timestamp)
  const qpsValues = props.data.map((d) => d.qps)

  const option: echarts.EChartsOption = {
    tooltip: {
      trigger: 'axis',
      formatter: (params: any) => {
        const param = params[0]
        return `时间: ${param.axisValue}<br/>QPS: ${param.value.toFixed(2)}`
      },
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: '10%',
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: timestamps,
      axisLine: {
        lineStyle: {
          color: '#e5e7eb',
        },
      },
      axisLabel: {
        color: '#6b7280',
        fontSize: 12,
      },
    },
    yAxis: {
      type: 'value',
      axisLine: {
        lineStyle: {
          color: '#e5e7eb',
        },
      },
      axisLabel: {
        color: '#6b7280',
        fontSize: 12,
        formatter: (value: number) => value.toFixed(1),
      },
      splitLine: {
        lineStyle: {
          color: '#f3f4f6',
        },
      },
    },
    series: [
      {
        name: 'QPS',
        type: 'line',
        smooth: true,
        data: qpsValues,
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(99, 102, 241, 0.3)' },
            { offset: 1, color: 'rgba(99, 102, 241, 0.05)' },
          ]),
        },
        lineStyle: {
          color: '#6366f1',
          width: 2,
        },
        itemStyle: {
          color: '#6366f1',
        },
      },
    ],
  }

  chartInstance.setOption(option)
}

// 监听数据变化
watch(
  () => props.data,
  () => {
    updateChart()
  },
  { deep: true }
)

// 响应式调整大小
const handleResize = () => {
  chartInstance?.resize()
}

onMounted(() => {
  initChart()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  chartInstance?.dispose()
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.qps-chart {
  width: 100%;
}
</style>
