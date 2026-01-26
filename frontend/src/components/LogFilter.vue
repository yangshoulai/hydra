<template>
  <div class="log-filter-container">
    <n-card class="filter-card" size="small" :bordered="false">
      <template #header>
        <div class="filter-header">
          <n-space align="center">
            <n-icon size="18" color="#60a5fa">
              <FilterIcon/>
            </n-icon>
            <span class="filter-title">日志筛选</span>
          </n-space>
        </div>
      </template>

      <n-form inline ref="formRef" :label-width="80" :model="formData" :label-placement="'left'" :label-align="'left'" :show-feedback="false"
              @keyup.enter="handleQuery">
        <n-grid :cols="24" :x-gap="24" :y-gap="24" responsive="screen">
          <n-form-item-gi :span="6" label="Trace ID" :show-label="true" label-placement="left" :label-width="80">
            <n-input
                v-model:value="formData.trace_id"
                placeholder="输入 Trace ID"
                clearable>
              <template #prefix>
                <n-icon color="#60a5fa">
                  <PulseOutline/>
                </n-icon>
              </template>
            </n-input>
          </n-form-item-gi>
          <n-form-item-gi :span="6" label="访问令牌">
            <n-input
                v-model:value="formData.access_token"
                placeholder="输入令牌名称"
                clearable>
              <template #prefix>
                <n-icon color="#10b981">
                  <KeyOutline/>
                </n-icon>
              </template>
            </n-input>
          </n-form-item-gi>
          <n-form-item-gi :span="6" label="请求模型">
            <n-select
                v-model:value="formData.requested_model"
                placeholder="选择模型"
                clearable
                :options="modelOptions"
                :loading="loadingModels"
                filterable
            />
          </n-form-item-gi>

          <n-form-item-gi :span="6" label="上游渠道">
            <n-select
                v-model:value="formData.channel_id"
                placeholder="选择渠道"
                clearable
                :options="channelOptions"
                :loading="loadingChannels"
                filterable
            />
          </n-form-item-gi>

          <n-form-item-gi :span="6" label="状态码">
            <n-select
                v-model:value="formData.status_code"
                placeholder="选择状态码"
                clearable
                :options="statusCodeOptions"
                filterable
            />
          </n-form-item-gi>

          <n-form-item-gi :span="6" label="请求状态">
            <n-select
                v-model:value="formData.is_success"
                placeholder="全部"
                clearable
                :options="successOptions"
                filterable
            />
          </n-form-item-gi>

          <n-form-item-gi :span="12" label="时间范围">
            <n-date-picker
                v-model:value="dateRange"
                type="datetimerange"
                :clearable="false"
                @update:value="handleDateChange"
            />
          </n-form-item-gi>

          <n-form-item-gi :span="6">
            <n-space>
              <n-button type="primary" @click="handleQuery">
                <template #icon>
                  <n-icon>
                    <SearchOutline/>
                  </n-icon>
                </template>
                查询
              </n-button>
              <n-button @click="handleReset">
                <template #icon>
                  <n-icon>
                    <RefreshOutline/>
                  </n-icon>
                </template>
                重置
              </n-button>
            </n-space>
          </n-form-item-gi>
        </n-grid>
      </n-form>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import {onMounted, reactive, ref} from 'vue'
import {NButton, NCard, NDatePicker, NForm, NFormItemGi, NGrid, NIcon, NInput, NSelect, NSpace, type SelectOption} from 'naive-ui'
import {FilterOutline, KeyOutline, PulseOutline, RefreshOutline, SearchOutline} from '@vicons/ionicons5'
import type {LogQueryRequest} from '../types/log'
import {channelApi} from '../services/channelService'
import {modelApi} from '../services/modelService'

interface Emits {
  (e: 'filter', data: LogQueryRequest): void
}

const emit = defineEmits<Emits>()

const FilterIcon = FilterOutline

// 表单数据
const formData = reactive<{
  trace_id: string
  access_token: string
  requested_model?: string | null
  channel_id?: number | null
  status_code?: number | null
  is_success?: string | null
  start_time?: string | null
  end_time?: string | null
}>({
  trace_id: '',
  access_token: '',
  requested_model: null,
  channel_id: null,
  status_code: null,
  is_success: null,
  start_time: null,
  end_time: null
})

// 日期范围
const dateRange = ref<[number, number] | null>(null)
const lastValidRange = ref<[number, number] | null>(null)
const maxRangeMs = 30 * 24 * 60 * 60 * 1000

// 渠道相关
const loadingChannels = ref(false)
const channelOptions = ref<SelectOption[]>([])

// 模型相关
const loadingModels = ref(false)
const modelOptions = ref<SelectOption[]>([])

// 状态码选项
const statusCodeOptions: SelectOption[] = [
  {label: '200 OK', value: 200},
  {label: '400 Bad Request', value: 400},
  {label: '401 Unauthorized', value: 401},
  {label: '404 Not Found', value: 404},
  {label: '500 Internal Server Error', value: 500},
  {label: '502 Bad Gateway', value: 502},
  {label: '503 Service Unavailable', value: 503}
]

// 成功状态选项
const successOptions = [
  {label: '成功', value: 'true'},
  {label: '失败', value: 'false'}
]

// 加载渠道列表
async function loadChannels() {
  loadingChannels.value = true
  try {
    const result = await channelApi.list({page: 1, page_size: 1000}) // 获取所有渠道
    channelOptions.value = result.items.map((channel) => ({
      label: channel.name,
      value: channel.id
    }))
  } catch (error) {
    console.error('Failed to load channels:', error)
  } finally {
    loadingChannels.value = false
  }
}

// 加载模型列表
async function loadModels() {
  loadingModels.value = true
  try {
    const result = await modelApi.list({page: 1, page_size: 1000}) // 获取所有模型
    modelOptions.value = result.items.map((model) => ({
      label: model.name,
      value: model.name
    }))
  } catch (error) {
    console.error('Failed to load models:', error)
  } finally {
    loadingModels.value = false
  }
}

// 处理日期变化
function handleDateChange(value: [number, number] | null) {
  if (!value) {
    if (lastValidRange.value) {
      const [start, end] = lastValidRange.value
      dateRange.value = [start, end]
      formData.start_time = new Date(start).toISOString()
      formData.end_time = new Date(end).toISOString()
    }
    return
  }

  let [start, end] = value
  if (end - start > maxRangeMs) {
    end = start + maxRangeMs
  }

  dateRange.value = [start, end]
  lastValidRange.value = [start, end]
  formData.start_time = new Date(start).toISOString()
  formData.end_time = new Date(end).toISOString()
}

// 处理查询
function handleQuery() {
  const queryData: LogQueryRequest = {
    ...formData,
    is_success: formData.is_success === 'true' ? true : (formData.is_success === 'false' ? false : undefined),
    requested_model: formData.requested_model || undefined,
    channel_id: formData.channel_id || undefined,
    status_code: formData.status_code || undefined,
    start_time: formData.start_time || undefined,
    end_time: formData.end_time || undefined
  }
  emit('filter', queryData)
}

// 重置筛选条件
function handleReset() {
  formData.trace_id = ''
  formData.access_token = ''
  formData.requested_model = null
  formData.channel_id = null
  formData.status_code = null
  formData.is_success = null
  formData.start_time = null
  formData.end_time = null
  setDefaultDateTime()

  // 重置后自动查询
  const queryData: LogQueryRequest = {
    ...formData,
    is_success: undefined,
    requested_model: undefined,
    channel_id: undefined,
    status_code: undefined,
    start_time: formData.start_time || undefined,
    end_time: formData.end_time || undefined
  }
  emit('filter', queryData)
}

// 暴露方法供父组件调用
defineExpose({
  reset: handleReset
})

// 初始化
onMounted(() => {
  loadChannels()
  loadModels()
  setDefaultDateTime()

})

function setDefaultDateTime() {
  // 设置默认时间范围为前24小时到后24小时（共2天）
  const now = Date.now()
  const dayAgo = now - 24 * 60 * 60 * 1000  // 24小时前
  const dayAhead = now + 24 * 60 * 60 * 1000 // 24小时后
  dateRange.value = [dayAgo, dayAhead]
  lastValidRange.value = [dayAgo, dayAhead]
  formData.start_time = new Date(dayAgo).toISOString()
  formData.end_time = new Date(dayAhead).toISOString()
}

</script>

<style scoped>
.log-filter-container {
  margin-bottom: 16px;
}

.filter-card {
  background: linear-gradient(135deg, #ffffff 0%, #f8fafc 100%);
  border: 1px solid #e2e8f0;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  border-radius: 12px;
  transition: all 0.3s ease;
}

.filter-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.filter-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.filter-title {
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

:deep(.n-card__header) {
  border-bottom: 1px solid #e2e8f0;
  padding: 16px 20px;
}

:deep(.n-card__content) {
  padding: 20px;
}

</style>
