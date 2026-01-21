import { defineStore } from 'pinia'
import { ref } from 'vue'
import { endpointApi } from '@/services/endpointService'
import type { EndpointInfo } from '@/types/endpoint'

export const useEndpointStore = defineStore('endpoint', () => {
  const endpoints = ref<EndpointInfo[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  async function fetchEndpoints() {
    if (loaded.value) return endpoints.value

    loading.value = true
    try {
      endpoints.value = await endpointApi.list()
      loaded.value = true
      return endpoints.value
    } finally {
      loading.value = false
    }
  }

  function getEndpointByType(type: string): EndpointInfo | undefined {
    return endpoints.value.find(ep => ep.type === type)
  }

  function getEndpointByPath(path: string): EndpointInfo | undefined {
    return endpoints.value.find(ep => ep.path === path)
  }

  return {
    endpoints,
    loading,
    loaded,
    fetchEndpoints,
    getEndpointByType,
    getEndpointByPath
  }
})
