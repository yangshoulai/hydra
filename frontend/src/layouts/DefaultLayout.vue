<template>
  <n-layout has-sider class="h-screen">
    <!-- 侧边栏 -->
    <n-layout-sider
        bordered
        collapse-mode="width"
        :collapsed-width="64"
        :width="240"
        :collapsed="collapsed"
        show-trigger="arrow-circle"
        @collapse="collapsed = true"
        @expand="collapsed = false">
      <!-- Logo 区域 -->
      <div class="h-[70px] flex items-center px-6 gap-4 justify-center">
        <div
            class="w-10 h-10 flex items-center justify-center bg-gradient-to-br from-primary-500 to-secondary-600 rounded-xl text-white flex-shrink-0 shadow-lg shadow-primary-500/40 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-primary-500/50">
          <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" class="w-6 h-6">
            <path d="M12 2L2 7L12 12L22 7L12 2Z" stroke="currentColor" stroke-width="2" stroke-linecap="round"
                  stroke-linejoin="round"/>
            <path d="M2 17L12 22L22 17" stroke="currentColor" stroke-width="2" stroke-linecap="round"
                  stroke-linejoin="round"/>
            <path d="M2 12L12 17L22 12" stroke="currentColor" stroke-width="2" stroke-linecap="round"
                  stroke-linejoin="round"/>
          </svg>
        </div>
        <transition name="fade">
          <div v-if="!collapsed" class="flex-1 min-w-0">
            <div class="text-[20px] font-bold text-white leading-tight tracking-tight">Hydra</div>
            <div class="text-[11px] text-white/60 mt-0.5 uppercase tracking-wider font-medium">API Gateway</div>
          </div>
        </transition>
      </div>

      <!-- 菜单 -->
      <n-menu
          :collapsed="collapsed"
          :collapsed-width="64"
          :collapsed-icon-size="22"
          :options="menuOptions"
          :value="currentRoute"
          @update:value="handleMenuSelect"
      />
    </n-layout-sider>

    <!-- 主内容区 -->
    <n-layout class="h-screen">
      <!-- 顶部导航栏 -->
      <n-layout-header position="absolute" bordered class="h-16 flex items-center justify-end px-8" :style="{ zIndex: 1000 }">
        <n-dropdown :options="userMenuOptions" @select="handleUserMenuSelect">
          <div
              class="flex items-center gap-3 px-4 py-2 cursor-pointer rounded-xl transition-all duration-200 hover:bg-gray-50">
            <div
                class="w-9 h-9 flex items-center justify-center bg-gradient-to-br from-primary-500 to-secondary-600 rounded-full text-white">
              <n-icon>
                <PersonOutline/>
              </n-icon>
            </div>
            <span class="text-sm text-gray-900 font-medium">管理员</span>
          </div>
        </n-dropdown>
      </n-layout-header>

      <!-- 内容区域 -->
      <n-layout-content class="p-4 custom-scrollbar" :style="{ paddingTop: '72px' }">
        <router-view/>
      </n-layout-content>
    </n-layout>
  </n-layout>

  <!-- 修改密码对话框 -->
  <ChangePasswordDialog v-model:show="showChangePasswordDialog"/>
</template>

<script setup lang="ts">
import {computed, h, ref} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import {type MenuOption, NDropdown, NIcon, NLayout, NLayoutContent, NLayoutHeader, NLayoutSider, NMenu,} from 'naive-ui'
import {
  BusinessOutline,
  CubeOutline,
  DocumentTextOutline,
  HomeOutline,
  KeyOutline,
  ListOutline,
  LockClosedOutline,
  LogOutOutline as LogOutIcon,
  PersonOutline,
  SettingsOutline,
} from '@vicons/ionicons5'
import ChangePasswordDialog from '../components/ChangePasswordDialog.vue'

const router = useRouter()
const route = useRoute()

const collapsed = ref(false)
const showChangePasswordDialog = ref(false)

// 当前路由
const currentRoute = computed(() => route.name as string)

// 菜单选项
const menuOptions = computed<MenuOption[]>(() => [
  {
    label: '仪表盘',
    key: 'Dashboard',
    icon: () => h(NIcon, null, {default: () => h(HomeOutline)}),
  },
  {
    label: '渠道管理',
    key: 'ChannelList',
    icon: () => h(NIcon, null, {default: () => h(ListOutline)}),
  },
  {
    label: '模型管理',
    key: 'ModelManagement',
    icon: () => h(NIcon, null, {default: () => h(CubeOutline)}),
  },
  {
    label: '厂商管理',
    key: 'ProviderManagement',
    icon: () => h(NIcon, null, {default: () => h(BusinessOutline)}),
  },
  {
    label: '访问令牌',
    key: 'TokenManagement',
    icon: () => h(NIcon, null, {default: () => h(KeyOutline)}),
  },
  {
    label: '日志查询',
    key: 'LogQuery',
    icon: () => h(NIcon, null, {default: () => h(DocumentTextOutline)}),
  },
  {
    label: '系统设置',
    key: 'Settings',
    icon: () => h(NIcon, null, {default: () => h(SettingsOutline)}),
  },

])

// 用户菜单选项
const userMenuOptions = [
  {
    label: '修改密码',
    key: 'change-password',
    icon: () => h(NIcon, null, {default: () => h(LockClosedOutline)}),
  },
  {
    label: '退出登录',
    key: 'logout',
    icon: () => h(NIcon, null, {default: () => h(LogOutIcon)}),
  },
]

// 菜单选择处理
const handleMenuSelect = (key: string) => {
  router.push({name: key})
}

// 用户菜单选择处理
const handleUserMenuSelect = (key: string) => {
  if (key === 'logout') {
    localStorage.removeItem('access_token')
    router.push({name: 'Login'})
  } else if (key === 'change-password') {
    showChangePasswordDialog.value = true
  }
}
</script>

<style scoped>
/* 侧边栏菜单项样式修复 */
:deep(.n-menu) {
  background: transparent;
}

/* 修复菜单项 hover 样式 */
:deep(.n-menu-item),
:deep(.n-menu-item-content) {
  transition: all 0.2s ease;
}

/* hover 状态 - 使用浅灰色背景 */
:deep(.n-menu-item-content:hover::before) {
  background: rgba(255, 255, 255, 0.1) !important;
}

/* 选中状态 */
:deep(.n-menu-item.n-menu-item--selected .n-menu-item-content) {
  background: rgba(255, 255, 255, 0.15) !important;
  font-weight: 600;
}

/* 默认状态 - 确保文字为白色 */
:deep(.n-menu-item .n-menu-item-content) {
  color: rgba(255, 255, 255, 0.75);
}

/* 图标颜色 */
:deep(.n-menu-item .n-menu-item-content__icon) {
  color: rgba(255, 255, 255, 0.75);
}

/* hover 时的图标颜色 */
:deep(.n-menu-item:hover .n-menu-item-content__icon) {
  color: #ffffff !important;
}

/* 选中状态的图标颜色 */
:deep(.n-menu-item.n-menu-item--selected .n-menu-item-content__icon) {
  color: #ffffff !important;
}

:deep(.n-menu .n-menu-item-content:not(.n-menu-item-content--disabled).n-menu-item-content--selected:hover .n-menu-item-content-header),
:deep(.n-menu .n-menu-item-content.n-menu-item-content--selected .n-menu-item-content__icon),
:deep(.n-menu.n-menu--collapsed .n-menu-item-content .n-menu-item-content__icon) {
  color: #ffffff !important;
}
</style>
