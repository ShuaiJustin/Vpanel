import { createRouter, createWebHistory } from 'vue-router'
import { userRoutes, userRouteGuard } from './user'

/**
 * 路由配置
 * 使用动态导入实现代码分割和懒加载
 * 按功能模块分组，优化加载性能
 */

// 布局组件 - 预加载
const MainLayout = () => import(/* webpackChunkName: "layout" */ '../layouts/MainLayout.vue')

// 核心页面 - 优先加载
const Dashboard = () => import(/* webpackChunkName: "core" */ '../views/Dashboard.vue')
const Profile = () => import(/* webpackChunkName: "core" */ '../views/Profile.vue')
const ChangePassword = () => import(/* webpackChunkName: "core" */ '../views/ChangePassword.vue')

// 代理管理 - 按需加载
const Inbounds = () => import(/* webpackChunkName: "proxy" */ '../views/Inbounds.vue')

// 用户管理 - 按需加载
const Users = () => import(/* webpackChunkName: "users" */ '../views/Users.vue')
const Roles = () => import(/* webpackChunkName: "users" */ '../views/Roles.vue')

// 监控统计 - 按需加载
const SystemMonitor = () => import(/* webpackChunkName: "monitor" */ '../views/SystemMonitor.vue')
const TrafficMonitor = () => import(/* webpackChunkName: "monitor" */ '../views/TrafficMonitor.vue')
const Stats = () => import(/* webpackChunkName: "monitor" */ '../views/Stats.vue')

// 系统管理 - 按需加载
const Settings = () => import(/* webpackChunkName: "system" */ '../views/Settings.vue')
const Certificates = () => import(/* webpackChunkName: "system" */ '../views/Certificates.vue')
const Logs = () => import(/* webpackChunkName: "system" */ '../views/Logs.vue')
const IPRestriction = () => import(/* webpackChunkName: "system" */ '../views/IPRestriction.vue')

// 订阅管理 - 按需加载
const Subscription = () => import(/* webpackChunkName: "subscription" */ '../views/Subscription.vue')
const AdminSubscriptions = () => import(/* webpackChunkName: "subscription" */ '../views/AdminSubscriptions.vue')

// 商业化管理 - 按需加载
const AdminPlans = () => import(/* webpackChunkName: "commercial-admin" */ '../views/AdminPlans.vue')
const AdminOrders = () => import(/* webpackChunkName: "commercial-admin" */ '../views/AdminOrders.vue')
const AdminCoupons = () => import(/* webpackChunkName: "commercial-admin" */ '../views/AdminCoupons.vue')
const AdminReports = () => import(/* webpackChunkName: "commercial-admin" */ '../views/AdminReports.vue')
const AdminGiftCards = () => import(/* webpackChunkName: "commercial-admin" */ '../views/AdminGiftCards.vue')
const AdminTrials = () => import(/* webpackChunkName: "commercial-admin" */ '../views/AdminTrials.vue')

// 节点管理 - 按需加载
const AdminNodes = () => import(/* webpackChunkName: "node-admin" */ '../views/AdminNodes.vue')
const NodeDetail = () => import(/* webpackChunkName: "node-admin" */ '../views/NodeDetail.vue')

// 法律文档 - 按需加载
const Terms = () => import(/* webpackChunkName: "legal" */ '../views/legal/Terms.vue')
const Privacy = () => import(/* webpackChunkName: "legal" */ '../views/legal/Privacy.vue')
const NodeForm = () => import(/* webpackChunkName: "node-admin" */ '../views/NodeForm.vue')
const AdminNodeGroups = () => import(/* webpackChunkName: "node-admin" */ '../views/AdminNodeGroups.vue')
const NodeDashboard = () => import(/* webpackChunkName: "node-admin" */ '../views/NodeDashboard.vue')
const NodeMap = () => import(/* webpackChunkName: "node-admin" */ '../views/NodeMap.vue')
const NodeComparison = () => import(/* webpackChunkName: "node-admin" */ '../views/NodeComparison.vue')

// 错误页面
const NotFound = () => import(/* webpackChunkName: "error" */ '../views/NotFound.vue')

const routes = [
  // 根路径 - 默认跳转到用户门户
  {
    path: '/',
    name: 'Home',
    redirect: () => {
      const isAuthenticated = localStorage.getItem('token')
      const isUserAuthenticated = localStorage.getItem('userToken')
      const userRole = localStorage.getItem('userRole') || 'user'
      
      // 管理员已登录，跳转到管理后台
      if (isAuthenticated && userRole === 'admin') {
        return '/admin/dashboard'
      }
      
      // 普通用户已登录，跳转到用户门户
      if (isUserAuthenticated) {
        return '/user/dashboard'
      }
      
      // 未登录，跳转到用户门户登录页
      return '/user/login'
    }
  },
  
  // 用户前台路由
  ...userRoutes,
  
  // 管理后台路由
  {
    path: '/admin',
    component: MainLayout,
    meta: { requiresAuth: true, roles: ['admin'] },
    children: [
      // 管理后台首页
      {
        path: '',
        redirect: '/admin/dashboard'
      },
      {
        path: 'dashboard',
        name: 'AdminDashboard',
        component: Dashboard,
        meta: { 
          requiresAuth: true,
          title: '管理仪表盘',
          roles: ['admin']
        }
      },
      {
        path: 'profile',
        name: 'AdminProfile',
        component: Profile,
        meta: { 
          requiresAuth: true,
          title: '个人资料',
          roles: ['admin']
        }
      },
      {
        path: 'change-password',
        name: 'AdminChangePassword',
        component: ChangePassword,
        meta: { 
          requiresAuth: true,
          title: '修改密码',
          roles: ['admin']
        }
      },
      
      // 代理管理
      {
        path: 'inbounds',
        name: 'AdminInbounds',
        component: Inbounds,
        meta: { 
          requiresAuth: true,
          title: '入站管理',
          roles: ['admin']
        }
      },
      
      // 订阅管理
      {
        path: 'subscriptions',
        name: 'AdminSubscriptions',
        component: AdminSubscriptions,
        meta: {
          requiresAuth: true,
          title: '订阅管理',
          roles: ['admin']
        }
      },
      
      // 用户管理
      {
        path: 'users',
        name: 'AdminUsers',
        component: Users,
        meta: { 
          requiresAuth: true,
          title: '用户管理',
          roles: ['admin']
        }
      },
      {
        path: 'roles',
        name: 'AdminRoles',
        component: Roles,
        meta: { 
          requiresAuth: true,
          title: '角色管理',
          roles: ['admin']
        }
      },
      
      // 监控统计
      {
        path: 'system-monitor',
        name: 'AdminSystemMonitor',
        component: SystemMonitor,
        meta: { 
          requiresAuth: true,
          title: '系统监控',
          roles: ['admin']
        }
      },
      {
        path: 'traffic-monitor',
        name: 'AdminTrafficMonitor',
        component: TrafficMonitor,
        meta: { 
          requiresAuth: true,
          title: '流量监控',
          roles: ['admin']
        }
      },
      {
        path: 'stats',
        name: 'AdminStats',
        component: Stats,
        meta: { 
          requiresAuth: true,
          title: '统计数据',
          roles: ['admin']
        }
      },
      
      // 系统管理
      {
        path: 'settings',
        name: 'AdminSettings',
        component: Settings,
        meta: {
          requiresAuth: true,
          title: '系统设置',
          roles: ['admin']
        }
      },
      {
        path: 'certificates',
        name: 'AdminCertificates',
        component: Certificates,
        meta: {
          requiresAuth: true,
          title: '证书管理',
          roles: ['admin']
        }
      },
      {
        path: 'logs',
        name: 'AdminLogs',
        component: Logs,
        meta: {
          requiresAuth: true,
          title: '日志管理',
          roles: ['admin']
        }
      },
      {
        path: 'ip-restriction',
        name: 'AdminIPRestriction',
        component: IPRestriction,
        meta: {
          requiresAuth: true,
          title: 'IP 限制管理',
          roles: ['admin']
        }
      },
      {
        path: 'ip-restrictions',
        redirect: '/admin/ip-restriction'
      },
      // 商业化管理
      {
        path: 'plans',
        name: 'AdminPlans',
        component: AdminPlans,
        meta: {
          requiresAuth: true,
          title: '套餐管理',
          roles: ['admin']
        }
      },
      {
        path: 'orders',
        name: 'AdminOrders',
        component: AdminOrders,
        meta: {
          requiresAuth: true,
          title: '订单管理',
          roles: ['admin']
        }
      },
      {
        path: 'coupons',
        name: 'AdminCoupons',
        component: AdminCoupons,
        meta: {
          requiresAuth: true,
          title: '优惠券管理',
          roles: ['admin']
        }
      },
      {
        path: 'reports',
        name: 'AdminReports',
        component: AdminReports,
        meta: {
          requiresAuth: true,
          title: '财务报表',
          roles: ['admin']
        }
      },
      {
        path: 'gift-cards',
        name: 'AdminGiftCards',
        component: AdminGiftCards,
        meta: {
          requiresAuth: true,
          title: '礼品卡管理',
          roles: ['admin']
        }
      },
      {
        path: 'trials',
        name: 'AdminTrials',
        component: AdminTrials,
        meta: {
          requiresAuth: true,
          title: '试用管理',
          roles: ['admin']
        }
      },
      // 节点管理
      {
        path: 'nodes',
        name: 'AdminNodes',
        component: AdminNodes,
        meta: {
          requiresAuth: true,
          title: '节点管理',
          roles: ['admin']
        }
      },
      {
        path: 'nodes/new',
        name: 'NodeCreate',
        component: NodeForm,
        meta: {
          requiresAuth: true,
          title: '添加节点',
          roles: ['admin']
        }
      },
      {
        path: 'nodes/:id',
        name: 'NodeDetail',
        component: NodeDetail,
        meta: {
          requiresAuth: true,
          title: '节点详情',
          roles: ['admin']
        }
      },
      {
        path: 'nodes/:id/edit',
        name: 'NodeEdit',
        component: NodeForm,
        meta: {
          requiresAuth: true,
          title: '编辑节点',
          roles: ['admin']
        }
      },
      {
        path: 'node-groups',
        name: 'AdminNodeGroups',
        component: AdminNodeGroups,
        meta: {
          requiresAuth: true,
          title: '节点分组',
          roles: ['admin']
        }
      },
      {
        path: 'node-dashboard',
        name: 'NodeDashboard',
        component: NodeDashboard,
        meta: {
          requiresAuth: true,
          title: '节点集群概览',
          roles: ['admin']
        }
      },
      {
        path: 'node-map',
        name: 'NodeMap',
        component: NodeMap,
        meta: {
          requiresAuth: true,
          title: '节点地理分布',
          roles: ['admin']
        }
      },
      {
        path: 'node-comparison',
        name: 'NodeComparison',
        component: NodeComparison,
        meta: {
          requiresAuth: true,
          title: '节点性能对比',
          roles: ['admin']
        }
      }
    ]
  },
  
  // 法律文档路由（无需认证）
  {
    path: '/legal/terms',
    name: 'Terms',
    component: Terms,
    meta: { title: '服务条款' }
  },
  {
    path: '/legal/privacy',
    name: 'Privacy',
    component: Privacy,
    meta: { title: '隐私政策' }
  },
  
  // 404 页面
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: NotFound,
    meta: { title: '页面未找到' }
  }
]

// 创建路由实例
const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) {
      return savedPosition
    }
    return { top: 0 }
  }
})

// 全局前置守卫
router.beforeEach((to, from, next) => {
  const isAuthenticated = localStorage.getItem('token')
  const isUserAuthenticated = localStorage.getItem('userToken')
  const userRole = localStorage.getItem('userRole') || 'user'
  
  // 处理根路径 - 根据登录状态和角色智能跳转
  if (to.path === '/') {
    if (isAuthenticated && userRole === 'admin') {
      next('/admin/dashboard')
      return
    } else if (isUserAuthenticated) {
      next('/user/dashboard')
      return
    } else {
      next('/user/login')
      return
    }
  }
  
  // 阻止直接访问旧的管理员登录页面，重定向到用户登录页
  if (to.path === '/login' || to.path === '/register') {
    next('/user/login')
    return
  }
  
  // 用户前台路由使用专门的守卫
  if (to.path.startsWith('/user')) {
    userRouteGuard(to, from, next)
    return
  }
  
  // 更新页面标题
  if (to.meta.title) {
    document.title = `${to.meta.title} - V Panel`
  }
  
  // 需要认证的管理后台页面
  if (to.meta.requiresAuth && !isAuthenticated) {
    // 未登录访问管理后台，跳转到用户登录页
    next({ name: 'UserLogin', query: { redirect: to.fullPath } })
    return
  }
  
  // 角色权限检查
  if (to.meta.roles && !to.meta.roles.includes(userRole)) {
    // 非管理员访问管理后台，跳转到用户门户或登录页
    if (isUserAuthenticated) {
      next('/user/dashboard')
    } else {
      next('/user/login')
    }
    return
  }
  
  next()
})

// 全局后置钩子 - 用于预加载
router.afterEach((to) => {
  // 预加载可能访问的下一个页面
  if (to.name === 'Dashboard') {
    // 预加载常用页面
    import(/* webpackChunkName: "proxy" */ '../views/Inbounds.vue')
    import(/* webpackChunkName: "monitor" */ '../views/SystemMonitor.vue')
  }
})

export default router
