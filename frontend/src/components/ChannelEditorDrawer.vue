<template>
  <n-drawer v-model:show="visible" :width="820" placement="right" :trap-focus="true" :mask-closable="true">
    <n-drawer-content :title="drawerTitle" closable>
      <n-space vertical :size="14">
        <section class="panel-card">
          <header class="panel-card__header">
            <h3 class="panel-card__title">基础信息</h3>
          </header>
          <div class="panel-card__body">
            <n-form ref="formRef" :model="formData" :rules="rules" label-placement="top" size="medium">
              <n-grid :cols="2" :x-gap="16" :y-gap="18" responsive="screen">
                <n-grid-item span="2 s:2 m:2 l:2">
                  <n-form-item label="渠道名称" path="name">
                    <n-input v-model:value="formData.name" placeholder="例如：OpenAI 官方渠道" />
                  </n-form-item>
                </n-grid-item>
                <n-grid-item span="2 s:2 m:2 l:2">
                  <n-form-item label="Base URL" path="base_url">
                    <n-input v-model:value="formData.base_url" placeholder="例如：https://api.openai.com" />
                  </n-form-item>
                </n-grid-item>
                <n-grid-item span="2 s:2 m:1 l:1">
                  <n-form-item label="权重" path="weight">
                    <n-input-number v-model:value="formData.weight" :min="1" :max="1000" style="width: 100%" placeholder="1-1000" />
                  </n-form-item>
                </n-grid-item>
                <n-grid-item span="2 s:2 m:1 l:1">
                  <n-form-item label="状态" path="status">
                    <n-select v-model:value="formData.status" :options="statusOptions" placeholder="请选择状态" />
                  </n-form-item>
                </n-grid-item>
                <n-grid-item span="2 s:2 m:2 l:2">
                  <n-form-item label="系统代理" path="use_proxy">
                    <div class="form-stack">
                      <n-space align="center">
                        <n-switch v-model:value="formData.use_proxy" />
                        <n-tag :type="formData.use_proxy ? 'success' : 'default'" :bordered="false" size="small">
                          {{ formData.use_proxy ? '本渠道启用' : '本渠道直连' }}
                        </n-tag>
                      </n-space>
                      <p class="form-hint">
                        关闭时，本渠道的模型测试、Key 测试和 API 代理都不走系统设置中的网络代理；开启后，仅当系统已配置网络代理地址时才会生效。
                      </p>
                    </div>
                  </n-form-item>
                </n-grid-item>
                <n-grid-item span="2 s:2 m:2 l:2">
                  <n-form-item label="描述" path="description">
                    <n-input v-model:value="formData.description" type="textarea" :autosize="{ minRows: 2, maxRows: 5 }" placeholder="可选描述" />
                  </n-form-item>
                </n-grid-item>
              </n-grid>
            </n-form>
          </div>
        </section>

        <section class="panel-card">
          <header class="panel-card__header">
            <h3 class="panel-card__title">渠道密钥</h3>
            <n-space size="small">
              <n-button v-if="isEdit" size="small" type="info" :loading="testing" @click="handleBatchTestKeys">
                批量测试
              </n-button>
              <n-tooltip>
                <template #trigger>
                  <n-button quaternary circle size="small" aria-label="新增分组" @click="openAddGroupDialog">
                    <template #icon>
                      <n-icon><AddOutline /></n-icon>
                    </template>
                  </n-button>
                </template>
                新增分组
              </n-tooltip>
            </n-space>
          </header>
          <div class="panel-card__body">
            <n-space vertical :size="10">
              <n-empty v-if="keyGroups.length === 0" description="暂无密钥分组">
                <template #extra>
                  <n-button size="small" type="primary" @click="openAddGroupDialog">新增分组</n-button>
                </template>
              </n-empty>

              <section v-for="group in keyGroups" :key="group.name" class="key-group-panel">
                <header class="key-group-panel__header">
                  <div class="key-group-panel__title">
                    <n-tag bordered size="small">{{ group.name }}</n-tag>
                    <n-text depth="3" style="font-size: 12px">{{ group.keys.length }} 个密钥</n-text>
                  </div>
                  <n-space size="small">
                    <n-tooltip>
                      <template #trigger>
                        <n-button
                          quaternary
                          circle
                          size="tiny"
                          :aria-label="`新增 ${group.name} 分组密钥`"
                          @click="openAddKeyDialog(group.name)"
                        >
                          <template #icon>
                            <n-icon><AddOutline /></n-icon>
                          </template>
                        </n-button>
                      </template>
                      新增密钥
                    </n-tooltip>
                    <n-tooltip v-if="canRemoveGroup(group)">
                      <template #trigger>
                        <n-button
                          quaternary
                          circle
                          size="tiny"
                          type="error"
                          :aria-label="`删除空分组 ${group.name}`"
                          @click="removeGroup(group.name)"
                        >
                          <template #icon>
                            <n-icon><TrashOutline /></n-icon>
                          </template>
                        </n-button>
                      </template>
                      删除空分组
                    </n-tooltip>
                  </n-space>
                </header>
                <div class="key-group-panel__body">
                  <n-empty v-if="group.keys.length === 0" size="small" description="该分组暂无密钥" />
                  <div v-else class="key-list">
                    <div v-for="item in group.keys" :key="item.uid" class="key-list-item">
                      <div class="key-list-item__left">
                        <n-text code class="inline-code">{{ item.preview }}</n-text>
                        <n-text v-if="item.remark" depth="3" style="font-size: 11px">{{ item.remark }}</n-text>
                      </div>
                      <div class="key-list-item__right">
                        <n-tag
                          v-if="item.kind === 'pending'"
                          type="warning"
                          :bordered="false"
                          size="small"
                        >
                          待保存
                        </n-tag>
                        <n-tag
                          v-else
                          :type="item.status === 'active' ? 'success' : 'default'"
                          :bordered="false"
                          size="small"
                        >
                          {{ item.status === 'active' ? '启用' : '停用' }}
                        </n-tag>

                        <n-tooltip>
                          <template #trigger>
                            <n-button
                              class="table-action-btn"
                              quaternary
                              circle
                              size="tiny"
                              :aria-label="`复制分组 ${group.name} 密钥`"
                              @click="handleCopyKey(item.value)"
                            >
                              <template #icon>
                                <n-icon><CopyOutline /></n-icon>
                              </template>
                            </n-button>
                          </template>
                          复制密钥
                        </n-tooltip>

                        <template v-if="item.kind === 'existing'">
                          <n-tooltip>
                            <template #trigger>
                              <n-button
                                class="table-action-btn"
                                quaternary
                                circle
                                size="tiny"
                                type="warning"
                                :aria-label="item.status === 'active' ? `停用密钥 ${item.id}` : `启用密钥 ${item.id}`"
                                @click="handleToggleChannelKeyStatus(item.id!, item.status === 'active' ? 'inactive' : 'active')"
                              >
                                <template #icon>
                                  <n-icon><ContrastOutline /></n-icon>
                                </template>
                              </n-button>
                            </template>
                            {{ item.status === 'active' ? '停用' : '启用' }}
                          </n-tooltip>
                          <n-tooltip>
                            <template #trigger>
                              <n-button
                                class="table-action-btn"
                                quaternary
                                circle
                                size="tiny"
                                type="error"
                                :aria-label="`删除密钥 ${item.id}`"
                                @click="handleDeleteChannelKey(item.id!)"
                              >
                                <template #icon>
                                  <n-icon><TrashOutline /></n-icon>
                                </template>
                              </n-button>
                            </template>
                            删除密钥
                          </n-tooltip>
                        </template>

                        <n-tooltip v-else>
                          <template #trigger>
                            <n-button
                              class="table-action-btn"
                              quaternary
                              circle
                              size="tiny"
                              type="error"
                              :aria-label="`移除待保存密钥 ${item.uid}`"
                              @click="removePendingKey(item.uid)"
                            >
                              <template #icon>
                                <n-icon><TrashOutline /></n-icon>
                              </template>
                            </n-button>
                          </template>
                          移除
                        </n-tooltip>
                      </div>
                    </div>
                  </div>
                </div>
              </section>
            </n-space>
          </div>
        </section>
      </n-space>

      <template #footer>
        <n-space justify="end">
          <n-button @click="handleCancel">取消</n-button>
          <n-button type="primary" :loading="saving" @click="handleSubmit">
            {{ isEdit ? '保存' : '创建' }}
          </n-button>
        </n-space>
      </template>
    </n-drawer-content>
  </n-drawer>

  <n-modal v-model:show="showAddGroupDialog" preset="card" title="新增密钥分组" :style="{ width: '420px' }">
    <n-form label-placement="top" size="medium">
      <n-form-item label="分组名称">
        <n-input v-model:value="addGroupName" maxlength="100" placeholder="例如：Production / Default / Backup" />
      </n-form-item>
    </n-form>
    <template #footer>
      <n-space justify="end">
        <n-button @click="showAddGroupDialog = false">取消</n-button>
        <n-button type="primary" @click="handleConfirmAddGroup">确认</n-button>
      </n-space>
    </template>
  </n-modal>

  <n-modal v-model:show="showAddKeyDialog" preset="card" title="新增分组密钥" :style="{ width: '560px' }">
    <n-form label-placement="top" size="medium">
      <n-form-item label="分组">
        <n-text code>{{ addKeyForm.group }}</n-text>
      </n-form-item>
      <n-form-item label="密钥（每行一个）">
        <n-input
          v-model:value="addKeyForm.keyLines"
          type="textarea"
          :autosize="{ minRows: 4, maxRows: 10 }"
          placeholder="例如：&#10;sk-xxxxxxxxxxxxxx&#10;sk-yyyyyyyyyyyyyy"
        />
      </n-form-item>
      <n-form-item label="备注（可选）">
        <n-input v-model:value="addKeyForm.remark" maxlength="200" placeholder="例如：生产主 Key" />
      </n-form-item>
    </n-form>
    <template #footer>
      <n-space justify="space-between" align="center" style="width: 100%">
        <n-text depth="3" style="font-size: 12px">
          目标分组：{{ addKeyForm.group }}，新增后同名分组会自动合并
        </n-text>
        <n-space>
          <n-button @click="showAddKeyDialog = false">取消</n-button>
          <n-button type="primary" @click="handleConfirmAddKey">确认新增</n-button>
        </n-space>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  AddOutline,
  ContrastOutline,
  CopyOutline,
  TrashOutline,
} from '@vicons/ionicons5'
import {
  type FormInst,
  type FormRules,
  NButton,
  NDrawer,
  NDrawerContent,
  NEmpty,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NIcon,
  NInput,
  NInputNumber,
  NModal,
  NSelect,
  NSpace,
  NSwitch,
  NTag,
  NText,
  NTooltip,
} from 'naive-ui'
import type { Channel, ChannelKey, CreateChannelRequest, UpdateChannelRequest } from '@/types/channel'
import { channelApi } from '@/services/channelService'
import { toastApiError } from '@/utils/error'
import { feedback } from '@/services/feedback'

interface Props {
  modelValue: boolean
  channel?: Channel | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

const formRef = ref<FormInst | null>(null)

const saving = ref(false)
const testing = ref(false)
const currentChannelId = ref(0)
const channelKeys = ref<ChannelKey[]>([])
const manualGroups = ref<string[]>(['Default'])

const formData = reactive({
  name: '',
  base_url: '',
  use_proxy: false,
  weight: 100,
  status: 'active' as 'active' | 'inactive',
  description: '',
})

type GroupedKeyItem =
  | {
      uid: string
      kind: 'existing'
      id: number
      value: string
      preview: string
      remark: string
      status: 'active' | 'inactive'
    }
  | {
      uid: string
      kind: 'pending'
      value: string
      preview: string
      remark: string
      status: 'active'
    }

interface PendingKeyItem {
  uid: string
  group: string
  value: string
  preview: string
  remark: string
}

interface KeyGroupPanel {
  name: string
  keys: GroupedKeyItem[]
}

const pendingKeys = ref<PendingKeyItem[]>([])

const showAddGroupDialog = ref(false)
const addGroupName = ref('')

const showAddKeyDialog = ref(false)
const addKeyForm = reactive({
  group: 'Default',
  keyLines: '',
  remark: '',
})

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '停用', value: 'inactive' },
]

const rules: FormRules = {
  name: {
    required: true,
    message: '请输入渠道名称',
    trigger: ['blur', 'input'],
  },
  base_url: {
    required: true,
    message: '请输入 Base URL',
    trigger: ['blur', 'input'],
  },
  weight: {
    required: true,
    type: 'number',
    message: '请输入权重',
    trigger: ['blur', 'change'],
  },
}

const isEdit = computed(() => !!props.channel)
const drawerTitle = computed(() => (isEdit.value ? `编辑渠道 · ${props.channel?.name}` : '新建渠道'))

function resetCreateState() {
  currentChannelId.value = 0
  formData.name = ''
  formData.base_url = ''
  formData.use_proxy = false
  formData.weight = 100
  formData.status = 'active'
  formData.description = ''
  channelKeys.value = []
  pendingKeys.value = []
  manualGroups.value = ['Default']
  showAddGroupDialog.value = false
  addGroupName.value = ''
  showAddKeyDialog.value = false
  addKeyForm.group = 'Default'
  addKeyForm.keyLines = ''
  addKeyForm.remark = ''
  formRef.value?.restoreValidation()
}

const keyGroups = computed<KeyGroupPanel[]>(() => {
  const groupMap = new Map<string, KeyGroupPanel>()

  const ensureGroup = (name: string) => {
    const normalized = normalizeGroupName(name)
    const identity = groupIdentity(normalized)
    if (!groupMap.has(identity)) {
      groupMap.set(identity, { name: normalized, keys: [] })
    }
    return groupMap.get(identity)!
  }

  manualGroups.value.forEach((name) => {
    ensureGroup(name)
  })

  channelKeys.value.forEach((item) => {
    const panel = ensureGroup(item.channel_key_group || 'Default')
    panel.keys.push({
      uid: `existing-${item.id}`,
      kind: 'existing',
      id: item.id,
      value: item.channel_key_value,
      preview: item.channel_key_preview || maskKey(item.channel_key_value),
      remark: item.remark || '',
      status: item.status,
    })
  })

  pendingKeys.value.forEach((item) => {
    const panel = ensureGroup(item.group)
    panel.keys.push({
      uid: item.uid,
      kind: 'pending',
      value: item.value,
      preview: item.preview,
      remark: item.remark,
      status: 'active',
    })
  })

  const groups = Array.from(groupMap.values())
  groups.forEach((group) => {
    group.keys.sort((a, b) => {
      if (a.kind !== b.kind) return a.kind === 'existing' ? -1 : 1
      if (a.kind === 'existing' && b.kind === 'existing') {
        return a.id - b.id
      }
      return a.uid.localeCompare(b.uid)
    })
  })

  groups.sort((a, b) => {
    const aDefault = groupIdentity(a.name) === 'default'
    const bDefault = groupIdentity(b.name) === 'default'
    if (aDefault && !bDefault) return -1
    if (!aDefault && bDefault) return 1
    return a.name.localeCompare(b.name)
  })
  return groups
})

function normalizeGroupName(name: string): string {
  const trimmed = name.trim()
  return trimmed || 'Default'
}

function groupIdentity(name: string): string {
  return normalizeGroupName(name).toLowerCase()
}

function maskKey(key: string): string {
  if (!key || key.length < 10) return key || ''
  return key.substring(0, 6) + '**********' + key.substring(key.length - 4)
}

function findGroupName(name: string): string | null {
  const targetIdentity = groupIdentity(name)
  for (const group of keyGroups.value) {
    if (groupIdentity(group.name) === targetIdentity) return group.name
  }
  return null
}

function ensureGroup(name: string): string {
  const normalized = normalizeGroupName(name)
  const existing = findGroupName(normalized)
  if (existing) return existing
  manualGroups.value.push(normalized)
  dedupeManualGroups()
  return normalized
}

function dedupeManualGroups() {
  const map = new Map<string, string>()
  manualGroups.value.forEach((name) => {
    const normalized = normalizeGroupName(name)
    const identity = groupIdentity(normalized)
    if (!map.has(identity)) {
      map.set(identity, normalized)
    }
  })
  manualGroups.value = Array.from(map.values())
}

function canRemoveGroup(group: KeyGroupPanel): boolean {
  return group.keys.length === 0 && groupIdentity(group.name) !== 'default'
}

function removeGroup(name: string) {
  const target = groupIdentity(name)
  manualGroups.value = manualGroups.value.filter((item) => groupIdentity(item) !== target)
}

function openAddGroupDialog() {
  addGroupName.value = ''
  showAddGroupDialog.value = true
}

function handleConfirmAddGroup() {
  const normalized = normalizeGroupName(addGroupName.value)
  const existing = findGroupName(normalized)
  if (existing) {
    feedback.message?.info(`分组“${existing}”已存在，已自动合并`)
    showAddGroupDialog.value = false
    return
  }
  manualGroups.value.push(normalized)
  dedupeManualGroups()
  showAddGroupDialog.value = false
  feedback.message?.success(`分组“${normalized}”已新增`)
}

function openAddKeyDialog(group: string) {
  addKeyForm.group = ensureGroup(group)
  addKeyForm.keyLines = ''
  addKeyForm.remark = ''
  showAddKeyDialog.value = true
}

function parseKeyLines(lines: string): string[] {
  if (!lines.trim()) return []
  const unique = new Set<string>()
  lines
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.length > 0)
    .forEach((line) => unique.add(line))
  return Array.from(unique)
}

function keyExistsInGroup(group: string, keyValue: string): boolean {
  const targetGroup = groupIdentity(group)
  if (
    channelKeys.value.some(
      (item) =>
        groupIdentity(item.channel_key_group || 'Default') === targetGroup &&
        item.channel_key_value === keyValue,
    )
  ) {
    return true
  }
  return pendingKeys.value.some((item) => groupIdentity(item.group) === targetGroup && item.value === keyValue)
}

async function handleConfirmAddKey() {
  const groupName = ensureGroup(addKeyForm.group)
  const keys = parseKeyLines(addKeyForm.keyLines)
  if (!keys.length) {
    feedback.message?.warning('请至少输入一个密钥')
    return
  }

  const toAdd = keys.filter((key) => !keyExistsInGroup(groupName, key))
  const skipped = keys.length - toAdd.length
  if (!toAdd.length) {
    feedback.message?.warning('输入的密钥已存在于该分组中')
    return
  }

  const remark = addKeyForm.remark.trim()
  if (isEdit.value && currentChannelId.value > 0) {
    try {
      await channelApi.batchAddChannelKeys(currentChannelId.value, toAdd, remark, groupName)
      await refreshChannelKeys()
      emit('saved')
      feedback.message?.success(`已新增 ${toAdd.length} 个密钥到分组“${groupName}”`)
      if (skipped > 0) {
        feedback.message?.info(`${skipped} 个重复密钥已自动跳过`)
      }
      showAddKeyDialog.value = false
      return
    } catch (err) {
      toastApiError(err, '新增密钥失败')
      return
    }
  }

  const timestamp = Date.now()
  toAdd.forEach((value, index) => {
    pendingKeys.value.push({
      uid: `pending-${timestamp}-${index}`,
      group: groupName,
      value,
      preview: maskKey(value),
      remark,
    })
  })
  feedback.message?.success(`已加入待保存密钥 ${toAdd.length} 个`)
  if (skipped > 0) {
    feedback.message?.info(`${skipped} 个重复密钥已自动跳过`)
  }
  showAddKeyDialog.value = false
}

function removePendingKey(uid: string) {
  pendingKeys.value = pendingKeys.value.filter((item) => item.uid !== uid)
}

watch(
  () => props.channel,
  (channel) => {
    if (!channel) {
      resetCreateState()
      return
    }

    currentChannelId.value = channel.id
    formData.name = channel.name
    formData.base_url = channel.base_url
    formData.use_proxy = !!channel.use_proxy
    formData.weight = channel.weight
    formData.status = channel.status
    formData.description = channel.description
    pendingKeys.value = []
    manualGroups.value = []
    refreshChannelKeys()
  },
  { immediate: true },
)

watch(
  () => props.modelValue,
  (show) => {
    if (!show) {
      if (!props.channel) {
        resetCreateState()
      }
      return
    }

    if (!props.channel) {
      resetCreateState()
      return
    }

    if (show && props.channel?.id) {
      currentChannelId.value = props.channel.id
      refreshChannelKeys()
    }
  },
)

async function refreshChannelKeys() {
  if (!currentChannelId.value) {
    channelKeys.value = []
    manualGroups.value = ['Default']
    return
  }

  try {
    const channel = await channelApi.get(currentChannelId.value)
    channelKeys.value = channel.channel_keys || []
    const groups = new Set<string>(manualGroups.value.map((name) => normalizeGroupName(name)))
    channelKeys.value.forEach((item) => {
      groups.add(normalizeGroupName(item.channel_key_group || 'Default'))
    })
    if (!groups.size) groups.add('Default')
    manualGroups.value = Array.from(groups)
    dedupeManualGroups()
  } catch {
    channelKeys.value = []
    if (!manualGroups.value.length) {
      manualGroups.value = ['Default']
    }
  }
}

function groupPendingKeys() {
  const batchMap = new Map<string, { group: string; remark: string; values: string[] }>()
  pendingKeys.value.forEach((item) => {
    const key = `${groupIdentity(item.group)}@@${item.remark}`
    if (!batchMap.has(key)) {
      batchMap.set(key, {
        group: item.group,
        remark: item.remark,
        values: [],
      })
    }
    batchMap.get(key)!.values.push(item.value)
  })
  return Array.from(batchMap.values())
}

async function handleSubmit() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
  } catch {
    return
  }

  saving.value = true
  try {
    let channelId = currentChannelId.value

    if (isEdit.value && channelId > 0) {
      const updatePayload: UpdateChannelRequest = {
        name: formData.name,
        base_url: formData.base_url,
        use_proxy: formData.use_proxy,
        weight: formData.weight,
        status: formData.status,
        description: formData.description,
      }
      await channelApi.update(channelId, updatePayload)
    } else {
      const createPayload: CreateChannelRequest = {
        name: formData.name,
        base_url: formData.base_url,
        use_proxy: formData.use_proxy,
        weight: formData.weight,
        status: formData.status,
        description: formData.description,
      }
      const created = await channelApi.create(createPayload)
      channelId = created.id
      currentChannelId.value = created.id
    }

    const pendingBatches = groupPendingKeys()
    if (pendingBatches.length > 0 && channelId > 0) {
      for (const batch of pendingBatches) {
        await channelApi.batchAddChannelKeys(
          channelId,
          batch.values,
          batch.remark,
          batch.group,
        )
      }
      pendingKeys.value = []
    }

    feedback.message?.success(isEdit.value ? '渠道更新成功' : '渠道创建成功')
    await refreshChannelKeys()

    emit('saved')

    if (!isEdit.value) {
      visible.value = false
    }
  } catch (err) {
    toastApiError(err, '保存失败')
  } finally {
    saving.value = false
  }
}

async function handleBatchTestKeys() {
  if (!currentChannelId.value) return

  testing.value = true
  try {
    const result = await channelApi.testChannelKeys(currentChannelId.value)
    feedback.message?.success(`批量测试完成：健康 ${result.healthy_channel_keys}/${result.total_channel_keys}`)
  } catch (err) {
    toastApiError(err, '批量测试失败')
  } finally {
    testing.value = false
  }
}

async function handleCopyKey(keyValue: string) {
  if (!keyValue) return
  try {
    await navigator.clipboard.writeText(keyValue)
    feedback.message?.success('密钥已复制到剪贴板')
  } catch {
    feedback.message?.error('复制失败，请手动复制')
  }
}

async function handleDeleteChannelKey(channelKeyId: number) {
  feedback.dialog?.warning({
    title: '确认删除',
    content: '确定要删除此渠道密钥吗？',
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await channelApi.deleteChannelKey(channelKeyId)
        feedback.message?.success('删除成功')
        await refreshChannelKeys()
        emit('saved')
      } catch (err) {
        toastApiError(err, '删除失败')
      }
    },
  })
}

async function handleToggleChannelKeyStatus(channelKeyId: number, targetStatus: 'active' | 'inactive') {
  const action = targetStatus === 'active' ? '启用' : '停用'
  feedback.dialog?.warning({
    title: `确认${action}`,
    content: `确定要${action}此密钥吗？`,
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await channelApi.resetChannelKeyStatus(channelKeyId, targetStatus)
        feedback.message?.success(`${action}成功`)
        await refreshChannelKeys()
        emit('saved')
      } catch (err) {
        toastApiError(err, `${action}失败`)
      }
    },
  })
}

function handleCancel() {
  if (!isEdit.value) {
    resetCreateState()
  }
  visible.value = false
}
</script>

<style scoped>
.key-group-panel {
  border: 1px solid #e8e8e8;
  border-radius: 10px;
  overflow: hidden;
  background: #ffffff;
}

.key-group-panel__header {
  padding: 8px 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  background: #fafafa;
  border-bottom: 1px solid #ececec;
}

.key-group-panel__title {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.key-group-panel__body {
  padding: 8px 10px;
}

.key-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.key-list-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 7px 8px;
  border: 1px solid #ededed;
  border-radius: 8px;
  background: #fcfcfc;
}

.key-list-item__left {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.key-list-item__right {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 4px;
}

@media (max-width: 820px) {
  .key-list-item {
    align-items: flex-start;
    flex-direction: column;
  }

  .key-list-item__right {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
