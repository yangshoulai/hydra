import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

// 布局组件
const Layout = () => import('@/layouts/DefaultLayout.vue')

// 页面组件（懒加载）
const Dashboard = () => import('@/pages/Dashboard.vue')
const ChannelList = () => import('@/pages/ChannelList.vue')
const LogQuery = () => import('@/pages/LogQuery.vue')
const Settings = () => import('@/pages/Settings.vue')
const TokenManagement = () => import('@/pages/TokenManagement.vue')
const ModelManagement = () => import('@/pages/ModelManagement.vue')
const ProviderManagement = () => import('@/pages/ProviderManagement.vue')
const Login = () => import('@/pages/Login.vue')

// 路由配置
const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: Login,
    meta: {
      title: '登录',
      requiresAuth: false,
    },
  },
  {
    path: '/',
    component: Layout,
    redirect: '/dashboard',
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: Dashboard,
        meta: {
          title: '仪表盘',
          icon: 'home-outline',
        },
      },
      {
        path: 'channels',
        name: 'ChannelList',
        component: ChannelList,
        meta: {
          title: '渠道管理',
          icon: 'list-outline',
        },
      },
      {
        path: 'models',
        name: 'ModelManagement',
        component: ModelManagement,
        meta: {
          title: '模型管理',
          icon: 'cube-outline',
        },
      },
      {
        path: 'providers',
        name: 'ProviderManagement',
        component: ProviderManagement,
        meta: {
          title: '厂商管理',
          icon: 'business-outline',
        },
      },
      {
        path: 'logs',
        name: 'LogQuery',
        component: LogQuery,
        meta: {
          title: '日志查询',
          icon: 'document-text-outline',
        },
      },
      {
        path: 'settings',
        name: 'Settings',
        component: Settings,
        meta: {
          title: '系统设置',
          icon: 'settings-outline',
        },
      },
      {
        path: 'tokens',
        name: 'TokenManagement',
        component: TokenManagement,
        meta: {
          title: '访问令牌',
          icon: 'key-outline',
        },
      },
    ],
  },
  {
    // 404 重定向
    path: '/:pathMatch(.*)*',
    redirect: '/dashboard',
  },
]

// 创建路由实例
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior() {
    return { top: 0 }
  },
})

// 路由守卫
router.beforeEach((to, _from, next) => {
  // 设置页面标题
  if (to.meta?.title) {
    document.title = `${to.meta.title} - Hydra API Gateway`
  }

  // 检查认证状态
  const token = localStorage.getItem('access_token')
  const requiresAuth = to.meta?.requiresAuth !== false

  if (requiresAuth && !token) {
    // 需要认证但未登录，重定向到登录页
    next({
      name: 'Login',
      query: { redirect: to.fullPath },
    })
  } else if (to.name === 'Login' && token) {
    // 已登录用户访问登录页，重定向到仪表盘
    next({ name: 'Dashboard' })
  } else {
    next()
  }
})

export default router
