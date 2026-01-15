<template>
  <n-button size="tiny" @click="handleCopy">
    <template #icon>
      <n-icon>
        <CopyIcon v-if="!copied" />
        <CheckIcon v-else />
      </n-icon>
    </template>
    {{ copied ? '已复制' : '复制' }}
  </n-button>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { NButton, NIcon, useMessage } from 'naive-ui'
import { CopyOutline, CheckmarkCircle } from '@vicons/ionicons5'

interface Props {
  traceId: string
}

const props = defineProps<Props>()

const message = useMessage()
const copied = ref(false)

const CopyIcon = CopyOutline
const CheckIcon = CheckmarkCircle

async function handleCopy() {
  try {
    await navigator.clipboard.writeText(props.traceId)
    copied.value = true
    message.success('TraceID已复制到剪贴板')

    // 2秒后重置状态
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch (error) {
    message.error('复制失败，请手动复制')
  }
}
</script>
