<template>
  <div ref="chartRef" class="qps-chart" :style="{ height: `${height}px` }"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, onUnmounted } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { LinearGradient } from 'echarts/lib/util/graphic'
import type { ECharts, ComposeOption } from 'echarts/core'
import type { LineSeriesOption } from 'echarts/charts'
import type { GridComponentOption, TooltipComponentOption } from 'echarts/components'
import type { QPSDataPoint } from '@/services/dashboardService'

echarts.use([CanvasRenderer, LineChart, GridComponent, TooltipComponent])

type ChartOption = ComposeOption<LineSeriesOption | GridComponentOption | TooltipComponentOption>

interface Props {
  data: QPSDataPoint[]
  height?: number
}

const props = withDefaults(defineProps<Props>(), {
  height: 300,
})

const chartRef = ref<HTMLDivElement>()
let chartInstance: ECharts | null = null

const initChart = () => {
  if (!chartRef.value) return

  chartInstance = echarts.init(chartRef.value)
  updateChart()
}

const updateChart = () => {
  if (!chartInstance || !props.data) return

  const timestamps = props.data.map((d) => d.timestamp)
  const qpsValues = props.data.map((d) => d.qps)

  const option: ChartOption = {
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
          color: new LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(64, 64, 64, 0.28)' },
            { offset: 1, color: 'rgba(64, 64, 64, 0.04)' },
          ]),
        },
        lineStyle: {
          color: '#262626',
          width: 2,
        },
        itemStyle: {
          color: '#262626',
        },
      },
    ],
  }

  chartInstance.setOption(option)
}

watch(
  () => props.data,
  () => {
    updateChart()
  },
  { deep: true }
)

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
