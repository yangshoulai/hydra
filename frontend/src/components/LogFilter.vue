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

      <n-form ref="formRef" :model="formData" :show-label="false" @keyup.enter="handleQuery">
        <n-space vertical :size="16">
          <!-- 第一行：主要筛选条件 -->
          <n-space :size="12" wrap>
            <!-- Trace ID -->
            <n-form-item label="Trace ID" :show-label="true" label-placement="left" :label-width="80">
              <n-input
                  v-model:value="formData.trace_id"
                  placeholder="输入 Trace ID"
                  clearable
                  style="width: 200px"
              >
                <template #prefix>
                  <n-icon color="#60a5fa">
                    <PulseOutline/>
                  </n-icon>
                </template>
              </n-input>
            </n-form-item>

            <!-- 访问令牌 -->
            <n-form-item label="访问令牌" :show-label="true" label-placement="left" :label-width="80">
              <n-input
                  v-model:value="formData.access_token"
                  placeholder="输入令牌名称"
                  clearable
                  style="width: 180px"
              >
                <template #prefix>
                  <n-icon color="#10b981">
                    <KeyOutline/>
                  </n-icon>
                </template>
              </n-input>
            </n-form-item>

            <!-- 请求模型 -->
            <n-form-item label="请求模型" :show-label="true" label-placement="left" :label-width="80">
              <n-select
                  v-model:value="formData.requested_model"
                  placeholder="选择模型"
                  clearable
                  :options="modelOptions"
                  :loading="loadingModels"
                  filterable
                  style="width: 200px"
              />
            </n-form-item>

            <!-- 渠道 -->
            <n-form-item label="渠道" :show-label="true" label-placement="left" :label-width="80">
              <n-select
                  v-model:value="formData.channel_id"
                  placeholder="选择渠道"
                  clearable
                  :options="channelOptions"
                  :loading="loadingChannels"
                  filterable
                  style="width: 180px"
              />
            </n-form-item>
          </n-space>

          <!-- 第二行：状态和时间 -->
          <n-space :size="12" wrap>
            <!-- 状态码 -->
            <n-form-item label="状态码" :show-label="true" label-placement="left" :label-width="80">
              <n-select
                  v-model:value="formData.status_code"
                  placeholder="选择状态码"
                  clearable
                  :options="statusCodeOptions"
                  style="width: 160px"
              />
            </n-form-item>

            <!-- 成功状态 -->
            <n-form-item label="请求状态" :show-label="true" label-placement="left" :label-width="80">
              <n-select
                  v-model:value="formData.is_success"
                  placeholder="全部"
                  clearable
                  :options="successOptions"
                  style="width: 140px"
              />
            </n-form-item>

            <!-- 时间范围 -->
            <n-form-item label="时间范围" :show-label="true" label-placement="left" :label-width="80">
              <n-date-picker
                  v-model:value="dateRange"
                  type="datetimerange"
                  clearable
                  style="width: 400px"
                  @update:value="handleDateChange"
              />
            </n-form-item>
          </n-space>

          <!-- 操作按钮 -->
          <n-space :size="12" justify="center">
            <n-button type="primary" @click="handleQuery">
              <template #icon>
                <n-icon>
                  <SearchIcon/>
                </n-icon>
              </template>
              查询
            </n-button>
            <n-button @click="handleReset">
              <template #icon>
                <n-icon>
                  <RefreshIcon/>
                </n-icon>
              </template>
              重置
            </n-button>
          </n-space>
        </n-space>
      </n-form>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import {onMounted, reactive, ref} from 'vue'
import {
  NButton,
  NCard,
  NDatePicker,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NSelect,
  NSpace,
  type SelectOption
} from 'naive-ui'
import {FilterOutline, KeyOutline, PulseOutline, RefreshOutline, SearchOutline} from '@vicons/ionicons5'
import type {LogQueryRequest} from '../types/log'
import {channelApi} from '../services/channelService'
import {modelApi} from '../services/modelService'

interface Emits {
  (e: 'filter', data: LogQueryRequest): void
}

const emit = defineEmits<Emits>()

const FilterIcon = FilterOutline
const RefreshIcon = RefreshOutline
const SearchIcon = SearchOutline

// 表单数据
const formData = reactive<LogQueryRequest>({
  trace_id: '',
  access_token: '',
  requested_model: undefined,
  channel_id: undefined,
  status_code: undefined,
  is_success: undefined,
  start_time: undefined,
  end_time: undefined
})

// 日期范围
const dateRange = ref<[number, number] | null>(null)

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
const successOptions: SelectOption[] = [
  {label: '成功', value: true},
  {label: '失败', value: false}
]

// 加载渠道列表
async function loadChannels() {
  loadingChannels.value = true
  try {
    const result = await channelApi.list(1, 1000) // 获取所有渠道
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
    const models = await modelApi.list()
    modelOptions.value = models.map((model) => ({
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
  if (value) {
    formData.start_time = new Date(value[0]).toISOString()
    formData.end_time = new Date(value[1]).toISOString()
  } else {
    formData.start_time = undefined
    formData.end_time = undefined
  }
}

// 处理查询
function handleQuery() {
  emit('filter', {...formData})
}

// 重置筛选条件
function handleReset() {
  formData.trace_id = ''
  formData.access_token = ''
  formData.requested_model = undefined
  formData.channel_id = undefined
  formData.status_code = undefined
  formData.is_success = undefined
  formData.start_time = undefined
  formData.end_time = undefined
  dateRange.value = null

  // 重置后自动查询
  emit('filter', {...formData})
}

// 暴露方法供父组件调用
defineExpose({
  reset: handleReset
})

// 初始化
onMounted(() => {
  loadChannels()
  loadModels()
})
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
