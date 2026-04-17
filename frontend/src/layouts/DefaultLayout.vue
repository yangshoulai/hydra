<template>
  <n-layout has-sider class="app-shell">
    <n-layout-sider
      bordered
      collapse-mode="width"
      :collapsed-width="siderCollapsedWidth"
      :width="siderWidth"
      :collapsed="collapsed"
      :show-trigger="false"
      :native-scrollbar="false"
      class="app-sider"
      :class="{ 'app-sider--collapsed': collapsed }"
    >
      <div
        class="app-brand"
        :class="{ 'app-brand--collapsed': collapsed }"
        role="button"
        tabindex="0"
        @click="router.push({ name: 'Dashboard' })"
        @keydown.enter.prevent="router.push({ name: 'Dashboard' })"
      >
        <div class="app-brand__logo">
          <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" class="w-5 h-5">
            <path d="M12 2L2 7L12 12L22 7L12 2Z" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M2 17L12 22L22 17" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M2 12L12 17L22 12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </div>
        <div v-show="!collapsed" class="app-brand__meta">
          <p class="app-brand__name">Hydra Console</p>
          <p class="app-brand__desc">API Gateway</p>
        </div>
      </div>

      <n-menu
        :collapsed="collapsed"
        :collapsed-width="siderCollapsedWidth"
        :collapsed-icon-size="20"
        :options="menuOptions"
        :value="currentRoute"
        class="app-menu"
        @update:value="handleMenuSelect"
      />
    </n-layout-sider>
    <div v-if="isMobile && !collapsed" class="app-sider-mask" @click="collapsed = true" />

    <n-layout class="app-main-layout">
      <n-layout-header bordered class="app-header" :style="{ zIndex: 1000 }">
        <div class="app-header__left">
          <n-button quaternary circle aria-label="展开或折叠菜单" @click="collapsed = !collapsed">
            <template #icon>
              <n-icon>
                <MenuOutline />
              </n-icon>
            </template>
          </n-button>
          <h1 class="app-header__title">{{ currentRouteMeta.title }}</h1>
        </div>
        <div class="app-header__right">
          <n-dropdown :options="userMenuOptions" @select="handleUserMenuSelect">
            <div class="user-trigger" role="button" tabindex="0" @keydown.enter.prevent="handleUserTriggerKeydown">
              <div class="user-trigger__avatar">
                <n-icon>
                  <PersonOutline />
                </n-icon>
              </div>
              <span class="user-trigger__name">管理员</span>
            </div>
          </n-dropdown>
        </div>
      </n-layout-header>

      <n-layout-content class="app-content custom-scrollbar">
        <router-view />
      </n-layout-content>
    </n-layout>
  </n-layout>

  <ChangePasswordDialog v-model:show="showChangePasswordDialog" />
</template>

<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  type DropdownOption,
  type MenuOption,
  NButton,
  NDropdown,
  NIcon,
  NLayout,
  NLayoutContent,
  NLayoutHeader,
  NLayoutSider,
  NMenu,
} from 'naive-ui'
import {
  BusinessOutline,
  CubeOutline,
  DocumentTextOutline,
  GridOutline,
  HomeOutline,
  KeyOutline,
  LockClosedOutline,
  LogOutOutline,
  MenuOutline,
  PersonOutline,
  SettingsOutline,
} from '@vicons/ionicons5'
import ChangePasswordDialog from '@/components/ChangePasswordDialog.vue'

const router = useRouter()
const route = useRoute()

const collapsed = ref(false)
const isMobile = ref(false)
const showChangePasswordDialog = ref(false)

const currentRoute = computed(() => route.name as string)
const siderCollapsedWidth = computed(() => (isMobile.value ? 0 : 64))
const siderWidth = computed(() => 220)

const currentRouteMeta = computed(() => ({
  title: (route.meta?.title as string) || 'Hydra Console',
}))

const menuOptions = computed<MenuOption[]>(() => [
  {
    label: '仪表盘',
    key: 'Dashboard',
    icon: () => h(NIcon, null, { default: () => h(HomeOutline) }),
  },
  {
    label: '渠道管理',
    key: 'ChannelList',
    icon: () => h(NIcon, null, { default: () => h(GridOutline) }),
  },
  {
    label: '模型管理',
    key: 'ModelManagement',
    icon: () => h(NIcon, null, { default: () => h(CubeOutline) }),
  },
  {
    label: '厂商管理',
    key: 'ProviderManagement',
    icon: () => h(NIcon, null, { default: () => h(BusinessOutline) }),
  },
  {
    label: '访问令牌',
    key: 'TokenManagement',
    icon: () => h(NIcon, null, { default: () => h(KeyOutline) }),
  },
  {
    label: '请求日志',
    key: 'RequestLogs',
    icon: () => h(NIcon, null, { default: () => h(DocumentTextOutline) }),
  },
  {
    label: '系统设置',
    key: 'Settings',
    icon: () => h(NIcon, null, { default: () => h(SettingsOutline) }),
  },
])

const userMenuOptions: DropdownOption[] = [
  {
    label: '修改密码',
    key: 'change-password',
    icon: () => h(NIcon, null, { default: () => h(LockClosedOutline) }),
  },
  {
    label: '退出登录',
    key: 'logout',
    icon: () => h(NIcon, null, { default: () => h(LogOutOutline) }),
  },
]

const handleMenuSelect = (key: string) => {
  router.push({ name: key })
  if (isMobile.value) {
    collapsed.value = true
  }
}

const handleUserMenuSelect = (key: string) => {
  if (key === 'logout') {
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
    router.push({ name: 'Login' })
    return
  }

  if (key === 'change-password') {
    showChangePasswordDialog.value = true
  }
}

const updateViewport = () => {
  const mobile = window.innerWidth <= 960
  isMobile.value = mobile
  if (mobile) {
    collapsed.value = true
  }
}

const handleUserTriggerKeydown = (event: KeyboardEvent) => {
  const current = event.currentTarget as HTMLElement | null
  current?.click()
}

onMounted(() => {
  updateViewport()
  window.addEventListener('resize', updateViewport)
})

onUnmounted(() => {
  window.removeEventListener('resize', updateViewport)
})
</script>

<style scoped>
.app-shell {
  height: 100vh;
  background: #f5f5f5;
}

.app-sider {
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e5e5e5;
  background: #ffffff;
  position: relative;
  z-index: 1100;
}

.app-sider :deep(.n-layout-sider-scroll-container),
.app-sider :deep(.n-layout-sider-children) {
  height: 100%;
}

.app-sider :deep(.n-layout-sider-children) {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.app-brand {
  height: 64px;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 14px;
  cursor: pointer;
  border-bottom: 1px solid #e5e5e5;
  flex-shrink: 0;
  overflow: hidden;
  white-space: nowrap;
}

.app-brand--collapsed {
  padding: 0;
  gap: 0;
  justify-content: center;
}

.app-brand__logo {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #111111;
  color: #fff;
  box-shadow: 0 8px 18px rgba(0, 0, 0, 0.2);
  flex-shrink: 0;
}

.app-brand__meta {
  min-width: 0;
  flex: 1;
}

.app-brand__name {
  margin: 0;
  color: #111111;
  font-weight: 720;
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-brand__desc {
  margin: 2px 0 0;
  color: #737373;
  font-size: 10px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-menu {
  flex: 1;
  min-height: 0;
  padding: 8px 6px;
  overflow-y: auto;
}

.app-sider--collapsed .app-menu {
  padding: 8px 0;
}

.app-menu :deep(.n-menu-item-content) {
  border-radius: 10px;
  min-height: 38px;
  box-shadow: none !important;
}

.app-menu :deep(.n-menu-item-content:not(.n-menu-item-content--collapsed)) {
  margin: 4px 0;
}

.app-menu :deep(.n-menu-item-content:hover) {
  background: #f0f0f0;
  box-shadow: none !important;
}

.app-menu :deep(.n-menu-item-content::before),
.app-menu :deep(.n-menu-item-content::after) {
  display: none !important;
}

.app-menu :deep(.n-menu-item-content__icon) {
  color: #595959;
}

.app-menu :deep(.n-menu-item-content:not(.n-menu-item-content--selected):hover .n-menu-item-content__icon) {
  color: #111111;
}

/* 折叠状态下：让整个菜单项居中，图标自然居中 */
.app-menu :deep(.n-menu-item-content--collapsed) {
  margin: 4px 8px !important;
  padding: 0 !important;
  justify-content: center !important;
}

.app-menu :deep(.n-menu-item-content--collapsed .n-menu-item-content__icon) {
  position: static !important;
  margin: 0 !important;
  transform: none !important;
  left: auto !important;
  top: auto !important;
}

.app-menu :deep(.n-menu-item-content--selected) {
  background: #f0f0f0 !important;
  box-shadow: none !important;
}

.app-menu :deep(.n-menu-item-content--selected .n-menu-item-content-header),
.app-menu :deep(.n-menu-item-content--selected .n-menu-item-content__icon) {
  color: #111111 !important;
  font-weight: 600;
}

/* 展开态：选中项左侧加一条细条指示器 */
.app-sider:not(.app-sider--collapsed) .app-menu :deep(.n-menu-item-content--selected) {
  position: relative;
}

.app-sider:not(.app-sider--collapsed) .app-menu :deep(.n-menu-item-content--selected)::before {
  content: "";
  position: absolute;
  left: 4px;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 16px;
  border-radius: 2px;
  background: #111111;
  display: block !important;
}

.app-main-layout {
  height: 100vh;
  overflow: hidden;
}

.app-sider-mask {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.3);
  z-index: 1090;
}

.app-header {
  height: 64px;
  padding: 0 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #ffffff;
}

.app-header__left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.app-header__title {
  margin: 0;
  font-size: 15px;
  line-height: 1.2;
  color: #111111;
  font-weight: 650;
}

.user-trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 10px 5px 5px;
  border-radius: 10px;
  cursor: pointer;
  transition: background 0.2s ease;
}

.user-trigger:hover {
  background: #f3f3f3;
}

.user-trigger__avatar {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #111111;
  color: #fff;
}

.user-trigger__name {
  font-size: 12px;
  color: #262626;
  font-weight: 600;
}

.app-content {
  padding: 14px;
  height: calc(100vh - 56px);
  overflow: auto;
}

@media (max-width: 960px) {
  .app-header {
    padding: 0 10px;
  }

  .user-trigger__name {
    display: none;
  }

  .app-content {
    padding: 10px;
  }
}
</style>
