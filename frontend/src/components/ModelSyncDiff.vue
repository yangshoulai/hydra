<template>
  <div class="model-sync-diff">
    <n-space vertical :size="16">
      <!-- 统计信息 -->
      <n-space :size="24">
        <n-statistic label="上游模型总数" :value="syncResult.diff.total_upstream_models" />
        <n-statistic label="本地配置总数" :value="syncResult.diff.total_local_models" />
        <n-statistic label="已选模型" :value="checkedKeys.length">
          <template #suffix>
            <n-text depth="3" style="font-size: 12px">
              / {{ syncResult.diff.diffs.length }}
            </n-text>
          </template>
        </n-statistic>
        <n-statistic label="已测试" :value="testStats.total">
          <template #suffix>
            <n-text depth="3" style="font-size: 12px">
              / {{ syncResult.diff.diffs.length }}
            </n-text>
          </template>
        </n-statistic>
        <n-statistic label="测试成功">
          <template #default>
            <n-text type="success" strong>{{ testStats.success }}</n-text>
          </template>
        </n-statistic>
        <n-statistic label="测试失败">
          <template #default>
            <n-text type="error" strong>{{ testStats.failed }}</n-text>
          </template>
        </n-statistic>
        <n-statistic label="新增模型">
          <template #default>
            <n-text type="success" strong>{{ syncResult.diff.added_count }}</n-text>
          </template>
        </n-statistic>
        <n-statistic label="存量模型">
          <template #default>
            <n-text type="info" strong>{{ syncResult.diff.existing_count }}</n-text>
          </template>
        </n-statistic>
        <n-statistic label="移除模型">
          <template #default>
            <n-text type="warning" strong>{{ syncResult.diff.removed_count }}</n-text>
          </template>
        </n-statistic>
      </n-space>

      <n-divider />

      <!-- 说明文字 -->
      <n-alert type="info" :show-icon="false">
        <n-text depth="3">
          <n-ul style="margin: 0; padding-left: 20px;">
            <n-li>
              <n-text strong>存量模型</n-text>：本地已配置的模型，默认勾选，可编辑统一模型名称
            </n-li>
            <n-li>
              <n-text strong>新增模型</n-text>：上游存在但本地未配置的模型，需要手动勾选才添加
            </n-li>
            <n-li>
              <n-text strong>移除模型</n-text>：本地已配置但上游不存在的模型，需要手动勾选才删除
            </n-li>
          </n-ul>
          <n-text depth="3" style="margin-top: 8px; display: block">
            统一模型名称默认使用上游模型名称，可修改为其他值。只有勾选的模型才会被保存。
          </n-text>
        </n-text>
      </n-alert>

      <!-- 模型配置表格 -->
      <n-data-table
        :columns="columns"
        :data="paginatedData"
        :pagination="false"
        :bordered="true"
        size="small"
        :row-key="(row: ModelDiffType) => row.upstream_model"
        v-model:checked-row-keys="checkedKeys"
      />

      <!-- 前端分页 -->
      <n-pagination
        v-model:page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :page-count="pageCount"
        :page-sizes="[10, 20, 50, 100]"
        show-size-picker
      />

      <!-- 操作按钮 -->
      <n-space justify="end">
        <n-button @click="handleCancel">取消</n-button>
        <n-button type="primary" @click="handleSave" :loading="saving">
          保存配置
        </n-button>
      </n-space>
    </n-space>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, h, onMounted } from 'vue'
import {
  NSpace,
  NStatistic,
  NText,
  NDivider,
  NAlert,
  NDataTable,
  NPagination,
  NButton,
  NSelect,
  NTag,
  NIcon,
  NUl,
  NLi,
  type DataTableColumns
} from 'naive-ui'
import {
  AddCircle,
  CheckmarkCircle,
  RemoveCircle,
  Checkmark as CheckIcon,
  Close as FailIcon,
  Refresh as RetryIcon
} from '@vicons/ionicons5'
import type { SyncResult, ModelDiffType } from '../types/channel'
import { channelApi } from '../services/channelService'
import { modelApi } from '../services/modelService'
import type { Model } from '../types/model'

interface Props {
  syncResult: SyncResult
  channelId: number
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'saved'): void
}>()

// 状态
const saving = ref(false)
const loadingModels = ref(false)
const unifiedModels = ref<Model[]>([])

// 编辑状态：upstream_model -> unified_model
const editMap = ref<Record<string, string>>({})

// 选中的行
const checkedKeys = ref<string[]>([])

// 测试结果状态：upstream_model -> { success: boolean, message: string, latency: string }
const testResults = ref<Record<string, { success: boolean; message: string; latency: string }>>({})

// 下拉选项
const modelOptions = computed(() => {
  return unifiedModels.value.map(model => ({
    label: `${model.name} (${model.provider})`,
    value: model.name
  }))
})

// 测试统计
const testStats = computed(() => {
  const results = Object.values(testResults.value)
  return {
    total: results.length,
    success: results.filter(r => r.success).length,
    failed: results.filter(r => r.success === false).length
  }
})

// 初始化编辑状态和选中状态
function initEditMap() {
  editMap.value = {}
  const defaultChecked: string[] = []

  props.syncResult.diff.diffs.forEach((diff) => {
    // 对于存量模型，使用现有的 unified_model
    if (diff.type === 'existing' && diff.existing_config) {
      editMap.value[diff.upstream_model] = diff.existing_config.unified_model
      // 存量模型默认选中
      defaultChecked.push(diff.upstream_model)
    } else {
      // 对于新增模型，默认使用上游模型名称作为统一模型名称
      editMap.value[diff.upstream_model] = diff.upstream_model
    }
  })

  // 设置默认选中的模型（存量模型）
  checkedKeys.value = defaultChecked
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

// 初始化
onMounted(async () => {
  await loadUnifiedModels()
  initEditMap()
})

// 分页
const pagination = reactive({
  page: 1,
  pageSize: 20
})

const pageCount = computed(() => Math.ceil(props.syncResult.diff.diffs.length / pagination.pageSize))

const paginatedData = computed(() => {
  // 按上游模型名称排序
  const sortedDiffs = [...props.syncResult.diff.diffs].sort((a, b) => {
    return a.upstream_model.localeCompare(b.upstream_model)
  })
  const start = (pagination.page - 1) * pagination.pageSize
  const end = start + pagination.pageSize
  return sortedDiffs.slice(start, end)
})

// 获取状态标签
function getStatusTag(diff: ModelDiffType) {
  if (diff.type === 'added') {
    return h(
      NTag,
      { type: 'success', size: 'small' },
      {
        default: () => '新增',
        icon: () => h(NIcon, null, { default: () => h(AddCircle) })
      }
    )
  } else if (diff.type === 'existing') {
    return h(
      NTag,
      { type: 'info', size: 'small' },
      {
        default: () => '存量',
        icon: () => h(NIcon, null, { default: () => h(CheckmarkCircle) })
      }
    )
  } else {
    return h(
      NTag,
      { type: 'warning', size: 'small' },
      {
        default: () => '移除',
        icon: () => h(NIcon, null, { default: () => h(RemoveCircle) })
      }
    )
  }
}

// 表格列
const columns: DataTableColumns<ModelDiffType> = [
  {
    type: 'selection'
  },
  {
    title: '上游模型',
    key: 'upstream_model',
    width: 400,
    render: (row) => {
      return h(NText, { code: true }, { default: () => row.upstream_model })
    }
  },
  {
    title: '统一模型名称',
    key: 'unified_model',
    width: 400,
    render: (row) => {
      // 对于移除的模型，显示现有的 unified_model（不可编辑）
      if (row.type === 'removed' && row.existing_config) {
        return h(NText, null, {
          default: () => row.existing_config!.unified_model
        })
      }

      // 对于新增和存量模型，显示下拉选择
      const value = editMap.value[row.upstream_model]

      return h(NSelect, {
        value: value,
        options: modelOptions.value,
        placeholder: '请选择统一模型',
        loading: loadingModels.value,
        filterable: true,
        clearable: false,
        onUpdateValue: (val: string) => {
          editMap.value[row.upstream_model] = val
        },
        disabled: row.type === 'removed' // 移除的模型不可编辑
      })
    }
  },
  {
    title: '状态',
    key: 'status',
    width: 120,
    align: 'center',
    render: (row) => getStatusTag(row)
  },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    align: 'center',
    fixed: 'right',
    render: (row) => {
      const result = testResults.value[row.upstream_model]
      const isTesting = testingModel.value === row.upstream_model

      // 如果有测试结果，显示状态图标
      if (result && !isTesting) {
        return h(
          NSpace,
          { size: 'small', style: 'justify-content: flex-end' },
          {
            default: () => [
              result.success
                ? h(NIcon, { size: 20, color: '#18a058' }, { default: () => h(CheckIcon) })
                : h(NIcon, { size: 20, color: '#d03050' }, { default: () => h(FailIcon) }),
              h(
                NButton,
                {
                  size: 'tiny',
                  quaternary: true,
                  onClick: () => handleTestModel(row),
                  style: 'margin-left: 4px'
                },
                { default: () => h(NIcon, { size: 16 }, { default: () => h(RetryIcon) }) }
              )
            ]
          }
        )
      }

      // 正在测试或未测试
      return h(
        NButton,
        {
          size: 'small',
          onClick: () => handleTestModel(row),
          loading: isTesting
        },
        { default: () => isTesting ? '测试中' : '测试' }
      )
    }
  }
]

// 测试单个模型的状态
const testingModel = ref<string | null>(null)

// 测试单个模型
async function handleTestModel(row: ModelDiffType) {
  testingModel.value = row.upstream_model

  try {
    const unifiedModel = editMap.value[row.upstream_model] || row.upstream_model

    // 调用后端 API 测试模型
    const result = await channelApi.testModel(props.channelId, row.upstream_model, unifiedModel)

    // 保存测试结果
    testResults.value[row.upstream_model] = {
      success: result.success,
      message: result.message,
      latency: result.latency || ''
    }

    if (result.success) {
      window.$message?.success(`模型 ${row.upstream_model} 测试成功 (${result.latency})`)
    } else {
      window.$message?.warning(`模型 ${row.upstream_model} 测试失败: ${result.message}`)
    }
  } catch (error: any) {
    console.error('Failed to test model:', error)

    // 保存失败结果
    testResults.value[row.upstream_model] = {
      success: false,
      message: error.response?.data?.error || '测试失败',
      latency: ''
    }

    window.$message?.error(error.response?.data?.error || `测试模型 ${row.upstream_model} 失败`)
  } finally {
    testingModel.value = null
  }
}

// 取消
function handleCancel() {
  emit('cancel')
}

// 保存
async function handleSave() {
  saving.value = true

  try {
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

    props.syncResult.diff.diffs.forEach((diff) => {
      // 只处理选中的模型
      if (!checkedKeys.value.includes(diff.upstream_model)) {
        return
      }

      if (diff.type === 'added') {
        // 新增模型：必须选择统一模型
        const unifiedModel = editMap.value[diff.upstream_model]
        if (!unifiedModel) {
          throw new Error(`上游模型 ${diff.upstream_model} 未选择统一模型`)
        }

        addModels.push({
          unified_model: unifiedModel,
          upstream_model: diff.upstream_model,
          remark: ''
        })
      } else if (diff.type === 'existing') {
        // 检查是否需要更新
        const currentUnifiedModel = editMap.value[diff.upstream_model]
        const existingUnifiedModel = diff.existing_config?.unified_model

        if (currentUnifiedModel !== existingUnifiedModel) {
          updateModels.push({
            id: diff.existing_config!.id,
            unified_model: currentUnifiedModel,
            upstream_model: diff.upstream_model,
            remark: diff.existing_config?.remark || ''
          })
        }
      } else if (diff.type === 'removed') {
        // 删除模型
        if (diff.existing_config) {
          deleteModelIDs.push(diff.existing_config.id)
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

    window.$message?.success('模型配置保存成功')
    emit('saved')
  } catch (error: any) {
    console.error('Failed to save sync:', error)
    window.$message?.error(error.response?.data?.error || error.message || '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.model-sync-diff {
  padding: 8px 0;
}

:deep(.n-input) {
  min-width: 200px;
}
</style>
