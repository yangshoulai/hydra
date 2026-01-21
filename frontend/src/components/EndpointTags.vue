<template>
  <n-space v-if="endpointInfos.length > 0" :size="4">
    <n-tag
      v-for="endpoint in endpointInfos"
      :key="endpoint.type"
      :bordered="false"
      size="small"
      :style="{ backgroundColor: endpoint.color + '20', color: endpoint.color, border: `1px solid ${endpoint.color}40` }"
    >
      {{ endpoint.name }}
    </n-tag>
  </n-space>
  <n-spin v-else-if="loading" :size="14" />
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { NSpace, NTag, NSpin } from 'naive-ui'
import { useEndpointStore } from '@/stores/endpoint'

interface Props {
  types?: string[]
  paths?: string[]
}

const props = defineProps<Props>()

const endpointStore = useEndpointStore()

const loading = computed(() => endpointStore.loading)

const endpointInfos = computed(() => {
  const infos: any[] = []

  // 根据 types 查找
  if (props.types) {
    props.types.forEach(type => {
      const endpoint = endpointStore.getEndpointByType(type)
      if (endpoint && !infos.find(e => e.type === endpoint.type)) {
        infos.push(endpoint)
      }
    })
  }

  // 根据 paths 查找
  if (props.paths) {
    props.paths.forEach(path => {
      const endpoint = endpointStore.getEndpointByPath(path)
      if (endpoint && !infos.find(e => e.type === endpoint.type)) {
        infos.push(endpoint)
      }
    })
  }

  return infos
})

onMounted(async () => {
  await endpointStore.fetchEndpoints()
})
</script>
