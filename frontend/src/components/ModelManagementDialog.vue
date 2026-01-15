<template>
  <n-modal
      v-model:show="show"
      preset="card"
      :title="`模型管理 - ${channelName}`"
      style="width: 1600px"
      :mask-closable="false"
      :closable="true"
      @close="handleClose"
  >
    <template #header-extra>
      <n-space>
        <n-button
            type="info"
            @click="handleSyncModels"
            :loading="syncing"
            size="small"
        >
          <template #icon>
            <n-icon>
              <SyncOutline/>
            </n-icon>
          </template>
          同步上游模型
        </n-button>
        <n-button
            type="primary"
            @click="showAddModelDialog = true"
            size="small"
        >
          添加模型
        </n-button>
      </n-space>
    </template>

    <n-space vertical :size="16">
      <!-- 统计信息卡片 -->
      <n-card size="small" :bordered="false">
        <template #header>
          <n-text strong>统计信息</n-text>
        </template>
        <n-grid :cols="5" :x-gap="16" responsive="screen">
          <n-grid-item>
            <n-statistic label="本地已配置">
              <template #default>
                <n-text type="success" strong style="font-size: 24px">
                  {{ stats.localConfigured }}
                </n-text>
              </template>
            </n-statistic>
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="上游模型总数">
              <template #default>
                <n-text type="info" strong style="font-size: 24px">
                  {{ stats.upstreamTotal }}
                </n-text>
              </template>
            </n-statistic>
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="待添加">
              <template #default>
                <n-text type="info" strong style="font-size: 24px">
                  {{ stats.toAdd }}
                </n-text>
              </template>
            </n-statistic>
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="待删除">
              <template #default>
                <n-text type="warning" strong style="font-size: 24px">
                  {{ stats.toRemove }}
                </n-text>
              </template>
            </n-statistic>
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="已选中">
              <template #default>
                <n-text strong style="font-size: 24px">
                  {{ checkedKeys.length }}
                </n-text>
              </template>
            </n-statistic>
          </n-grid-item>
        </n-grid>
      </n-card>

      <!-- 说明信息 -->
      <n-alert
          v-if="!hasSynced"
          type="info"
          :show-icon="false"
      >
        <n-text>
          当前显示本地已配置的模型列表。点击"同步上游模型"按钮可获取渠道的最新模型列表并合并显示。
        </n-text>
      </n-alert>

      <n-alert
          v-else-if="syncFailed"
          type="warning"
          :show-icon="false"
      >
        <n-text>
          无法从上游渠道获取模型列表，仅显示本地已配置的模型。
        </n-text>
      </n-alert>

      <n-alert
          v-else
          type="info"
          :show-icon="false"
      >
        <n-text depth="3">
          <n-ul style="margin: 0; padding-left: 20px;">
            <n-li>
              <n-text strong>已配置模型</n-text>
              ：上游和本地都存在的模型，默认选中
            </n-li>
            <n-li>
              <n-text strong>待添加模型</n-text>
              ：仅上游存在的模型，需要手动勾选
            </n-li>
            <n-li>
              <n-text strong>待删除模型</n-text>
              ：仅本地存在的模型，默认选中且禁用（将被删除）
            </n-li>
          </n-ul>
          <n-text depth="3" style="margin-top: 8px; display: block">
            修改统一模型后，点击"保存更改"按钮批量保存选中的模型配置。
          </n-text>
        </n-text>
      </n-alert>

      <!-- 模型列表表格 -->
      <n-data-table
          :columns="columns"
          :data="displayModels"
          :loading="loading"
          :pagination="pagination"
          :row-key="(row: ModelDisplayType) => row.key"
          size="small"
          v-model:checked-row-keys="checkedKeys"
      />

      <!-- 操作按钮 -->
      <n-space justify="end">
        <n-button @click="handleClose">取消</n-button>
        <n-button type="primary" @click="handleSave" :loading="saving">
          保存更改 ({{ checkedKeys.length }})
        </n-button>
      </n-space>
    </n-space>

    <!-- 添加模型对话框 -->
    <n-modal v-model:show="showAddModelDialog" preset="dialog" title="添加模型配置">
      <n-form ref="modelFormRef" :model="modelForm" :rules="modelRules" label-placement="left" label-width="100px">
        <n-form-item label="上游模型" path="upstream_model">
          <n-input
              v-model:value="modelForm.upstream_model"
              placeholder="请输入上游模型名称，如：gpt-4-turbo-preview"
          />
        </n-form-item>
        <n-form-item label="统一模型" path="unified_model">
          <n-select
              v-model:value="modelForm.unified_model"
              :options="modelOptions"
              placeholder="请选择统一模型"
              :loading="loadingModels"
              filterable
          />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space>
          <n-button @click="showAddModelDialog = false">取消</n-button>
          <n-button type="primary" @click="handleAddModel">
            确定
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </n-modal>
</template>

<script setup lang="ts">
import {computed, h, nextTick, reactive, ref, watch} from 'vue'
import {
  type DataTableColumns,
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NIcon,
  NInput,
  NLi,
  NModal,
  NSelect,
  NSpace,
  NStatistic,
  NTag,
  NText,
  NUl
} from 'naive-ui'
import {SyncOutline} from '@vicons/ionicons5'
import {channelApi} from '../services/channelService'
import {modelApi} from '../services/modelService'
import type {SyncResult} from '../types/channel'
import type {Model} from '../types/model'

interface Props {
  channelId: number
  channelName: string
  modelValue: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  refresh: []
}>()

const show = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value)
})

// State
const loading = ref(false)
const syncing = ref(false)
const saving = ref(false)
const syncFailed = ref(false)
const hasSynced = ref(false)
const showAddModelDialog = ref(false)
const syncResult = ref<SyncResult | null>(null)
const localConfigs = ref<any[]>([])
const unifiedModels = ref<Model[]>([])
const loadingModels = ref(false)

// 选中的行
const checkedKeys = ref<string[]>([])

// 编辑状态：key -> unified_model
const editMap = ref<Record<string, string>>({})

// 表单
const modelForm = reactive({
  upstream_model: '',
  unified_model: ''
})
const modelRules = {
  upstream_model: {
    required: true,
    message: '请输入上游模型名称',
    trigger: ['blur', 'input']
  },
  unified_model: {
    required: true,
    message: '请选择统一模型',
    trigger: ['blur', 'change']
  }
}

// 统一模型下拉选项
const modelOptions = computed(() => {
  return unifiedModels.value.map(model => ({
    label: model.provider
      ? `${model.name} (${model.provider.name})`
      : model.name,
    value: model.name
  }))
})

// 统计信息
const stats = computed(() => {
  if (!hasSynced.value || syncFailed.value) {
    return {
      localConfigured: localConfigs.value.length,
      upstreamTotal: 0,
      toAdd: 0,
      toRemove: 0
    }
  }

  const diff = syncResult.value?.diff
  return {
    localConfigured: diff?.existing_count || 0,
    upstreamTotal: diff?.total_upstream_models || 0,
    toAdd: diff?.added_count || 0,
    toRemove: diff?.removed_count || 0
  }
})

// 模型显示类型
interface ModelDisplayType {
  key: string
  upstream_model: string
  unified_model: string
  status: 'configured' | 'to_add' | 'to_remove'
  disabled: boolean
}

// 显示的模型列表
const displayModels = computed<ModelDisplayType[]>(() => {
  if (!hasSynced.value || syncFailed.value || !syncResult.value) {
    // 仅显示本地配置
    return localConfigs.value.map(config => ({
      key: config.upstream_model,
      upstream_model: config.upstream_model,
      unified_model: config.unified_model,
      status: 'configured' as const,
      disabled: false
    }))
  }

  // 有同步结果，合并显示
  const models: ModelDisplayType[] = []
  const diff = syncResult.value.diff

  // 初始化编辑状态和选中状态
  if (!editMap.value || Object.keys(editMap.value).length === 0) {
    initEditMap()
  }

  diff.diffs.forEach((d) => {
    const unifiedModel = editMap.value[d.upstream_model] || d.upstream_model
    let status: 'configured' | 'to_add' | 'to_remove'
    let disabled = false

    if (d.type === 'existing') {
      status = 'configured'
      disabled = false
    } else if (d.type === 'added') {
      status = 'to_add'
      disabled = false
    } else {
      status = 'to_remove'
      disabled = true // 待删除的模型默认选中且禁用
    }

    models.push({
      key: d.upstream_model,
      upstream_model: d.upstream_model,
      unified_model: unifiedModel,
      status,
      disabled
    })
  })

  // 排序：已配置 > 待删除 > 待添加，同类型按上游模型名称排序
  const statusOrder = {configured: 1, to_remove: 2, to_add: 3}
  models.sort((a, b) => {
    const statusDiff = statusOrder[a.status] - statusOrder[b.status]
    if (statusDiff !== 0) return statusDiff
    return a.upstream_model.localeCompare(b.upstream_model)
  })

  return models
})

// 分页
const pagination = reactive({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  onChange: (page: number) => {
    pagination.page = page
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
  }
})

// 表格列定义
const columns: DataTableColumns<ModelDisplayType> = [
  {
    type: 'selection'
  },
  {
    title: '上游模型',
    key: 'upstream_model',
    width: 400,
    render(row) {
      return h(NText, {code: true}, {default: () => row.upstream_model})
    }
  },
  {
    title: '统一模型',
    key: 'unified_model',
    width: 400,
    render(row) {
      const value = editMap.value[row.key] || row.unified_model

      return h(NSelect, {
        value: value,
        options: modelOptions.value,
        placeholder: '请选择统一模型',
        loading: loadingModels.value,
        filterable: true,
        size: 'small',
        onUpdateValue: (val: string) => {
          editMap.value[row.key] = val
        }
      })
    }
  },
  {
    title: '状态',
    key: 'status',
    width: 120,
    align: 'center',
    render(row) {
      const statusConfig = {
        configured: {type: 'success' as const, text: '已配置'},
        to_add: {type: 'info' as const, text: '待添加'},
        to_remove: {type: 'warning' as const, text: '待删除'}
      }
      const config = statusConfig[row.status]
      return h(NTag, {type: config.type, size: 'small'}, {default: () => config.text})
    }
  }
]

// 初始化编辑状态和选中状态
function initEditMap() {
  editMap.value = {}
  const defaultChecked: string[] = []

  if (hasSynced.value && !syncFailed.value && syncResult.value) {
    // 有同步结果
    syncResult.value.diff.diffs.forEach((d) => {
      if (d.type === 'existing' && d.existing_config) {
        editMap.value[d.upstream_model] = d.existing_config.unified_model
        defaultChecked.push(d.upstream_model)
      } else if (d.type === 'added') {
        editMap.value[d.upstream_model] = d.upstream_model
        // 新增的模型默认不选中
      } else if (d.type === 'removed') {
        editMap.value[d.upstream_model] = d.existing_config?.unified_model || d.upstream_model
        // 删除的模型默认选中
        defaultChecked.push(d.upstream_model)
      }
    })
  } else {
    // 仅本地配置
    localConfigs.value.forEach(config => {
      editMap.value[config.upstream_model] = config.unified_model
      defaultChecked.push(config.upstream_model)
    })
  }

  checkedKeys.value = defaultChecked
}

// 加载本地配置
async function loadLocalConfigs() {
  loading.value = true
  try {
    const channel = await channelApi.get(props.channelId)
    localConfigs.value = channel.model_configs || []
  } catch (error: any) {
    console.error('Failed to load local configs:', error)
    window.$message?.error('加载本地配置失败')
  } finally {
    loading.value = false
  }
}

// 加载统一模型列表
async function loadUnifiedModels() {
  loadingModels.value = true
  try {
    unifiedModels.value = await modelApi.list()
  } catch (error: any) {
    console.error('Failed to load unified models:', error)
    window.$message?.error('加载统一模型列表失败')
  } finally {
    loadingModels.value = false
  }
}

// 同步上游模型
async function handleSyncModels() {
  syncing.value = true
  syncFailed.value = false

  try {
    const result = await channelApi.syncModels(props.channelId)
    syncResult.value = result
    hasSynced.value = true

    // 清空之前的编辑状态，重新初始化
    editMap.value = {}
    initEditMap()

    window.$message?.success('同步成功')
  } catch (error: any) {
    console.error('Failed to sync models:', error)
    syncFailed.value = true
    hasSynced.value = true
    syncResult.value = null
    window.$message?.error(error.response?.data?.error || '同步失败，仅显示本地配置')
  } finally {
    syncing.value = false
  }
}

// 添加模型配置
async function handleAddModel() {
  try {
    await channelApi.createModelConfig(
        props.channelId,
        modelForm.unified_model,
        modelForm.upstream_model
    )
    window.$message?.success('添加成功')
    showAddModelDialog.value = false
    modelForm.upstream_model = ''
    modelForm.unified_model = ''
    await loadLocalConfigs()
    emit('refresh')
  } catch (error: any) {
    console.error('Failed to add model config:', error)
    window.$message?.error(error.response?.data?.error || '添加失败')
  }
}

// 保存更改
async function handleSave() {
  saving.value = true

  try {
    if (!hasSynced.value || syncFailed.value) {
      // 没有同步或同步失败，只处理本地配置的修改
      const updatePromises: Promise<any>[] = []
      const deletePromises: Promise<void>[] = []

      // 遍历本地配置
      for (const config of localConfigs.value) {
        const isChecked = checkedKeys.value.includes(config.upstream_model)
        const currentUnifiedModel = editMap.value[config.upstream_model]

        if (!isChecked) {
          // 取消选中，删除配置
          deletePromises.push(channelApi.deleteModelConfig(config.id))
        } else if (currentUnifiedModel && currentUnifiedModel !== config.unified_model) {
          // 选中且统一模型有修改，更新配置
          updatePromises.push(
            channelApi.updateModelConfig(config.id, {
              unified_model: currentUnifiedModel
            })
          )
        }
      }

      // 执行所有删除和更新操作
      await Promise.all([...deletePromises, ...updatePromises])

      if (deletePromises.length > 0 || updatePromises.length > 0) {
        window.$message?.success('保存成功')
        emit('refresh')
      }
    } else {
      // 有同步结果，使用同步保存逻辑
      // 验证：检查所有选中的模型是否都选择了统一模型
      for (const key of checkedKeys.value) {
        const unifiedModel = editMap.value[key]
        if (!unifiedModel) {
          window.$message?.error(`请为上游模型 ${key} 选择统一模型`)
          saving.value = false
          return
        }
      }

      // 准备数据
      const addModels: any[] = []
      const updateModels: any[] = []
      const deleteModelIDs: number[] = []

      syncResult.value!.diff.diffs.forEach((d) => {
        const isChecked = checkedKeys.value.includes(d.upstream_model)
        const currentUnifiedModel = editMap.value[d.upstream_model]
        const existingUnifiedModel = d.existing_config?.unified_model

        if (d.type === 'existing') {
          // 已配置的模型
          if (isChecked) {
            // 选中了，检查是否需要更新
            if (currentUnifiedModel !== existingUnifiedModel) {
              updateModels.push({
                id: d.existing_config!.id,
                unified_model: currentUnifiedModel,
                upstream_model: d.upstream_model
              })
            }
          } else {
            // 没选中，删除
            deleteModelIDs.push(d.existing_config!.id)
          }
        } else if (d.type === 'added') {
          // 新增模型
          if (isChecked) {
            addModels.push({
              unified_model: currentUnifiedModel,
              upstream_model: d.upstream_model
            })
          }
        } else if (d.type === 'removed') {
          // 删除模型（默认选中且禁用）
          if (isChecked && d.existing_config) {
            deleteModelIDs.push(d.existing_config.id)
          }
        }
      })

      // 调用 API
      await channelApi.applySync(
        props.channelId,
        {
          add_models: addModels,
          update_models: updateModels,
          delete_model_ids: deleteModelIDs
        }
      )

      window.$message?.success('保存成功')
      emit('refresh')
    }

    handleClose()
  } catch (error: any) {
    console.error('Failed to save:', error)
    window.$message?.error(error.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

// 关闭对话框
function handleClose() {
  emit('update:modelValue', false)
}

// 监听对话框打开
watch(() => props.modelValue, async (newVal) => {
  if (newVal && props.channelId > 0) {
    // 重置状态
    syncResult.value = null
    syncFailed.value = false
    hasSynced.value = false
    editMap.value = {}
    checkedKeys.value = []
    localConfigs.value = []
    unifiedModels.value = []

    // 重置分页为第一页
    pagination.page = 1

    console.log('[ModelManagementDialog] Dialog opened, loading data...')

    try {
      // 先加载数据
      await Promise.all([
        loadLocalConfigs(),
        loadUnifiedModels()
      ])

      console.log('[ModelManagementDialog] Data loaded:', {
        localConfigs: localConfigs.value.length,
        unifiedModels: unifiedModels.value.length
      })

      // 数据加载完成后再初始化编辑状态和选中状态
      initEditMap()

      console.log('[ModelManagementDialog] Edit map initialized:', editMap.value)

      // 等待 Vue 更新视图
      await nextTick()

      console.log('[ModelManagementDialog] After nextTick, displayModels:', displayModels.value.length)
    } catch (error) {
      console.error('[ModelManagementDialog] Error loading data:', error)
    }
  }
})
</script>

<style scoped>
/* 使用 Tailwind CSS */
</style>
