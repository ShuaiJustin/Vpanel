/**
 * 用户前台门户路由配置
 * 使用动态导入实现代码分割和懒加载
 */

// 用户前台布局组件
const UserLayout = () => import(/* webpackChunkName: "user-layout" */ '../layouts/UserLayout.vue')
const AuthLayout = () => import(/* webpackChunkName: "user-auth" */ '../layouts/AuthLayout.vue')

// 用户认证页面
const UserLogin = () => import(/* webpackChunkName: "user-auth" */ '../views/user/Login.vue')
const UserRegister = () => import(/* webpackChunkName: "user-auth" */ '../views/user/Register.vue')
const ForgotPassword = () => import(/* webpackChunkName: "user-auth" */ '../views/user/ForgotPassword.vue')
const ResetPassword = () => import(/* webpackChunkName: "user-auth" */ '../views/user/ResetPassword.vue')

// 用户核心页面
const UserDashboard = () => import(/* webpackChunkName: "user-core" */ '../views/user/Dashboard.vue')
const UserNodes = () => import(/* webpackChunkName: "user-core" */ '../views/user/Nodes.vue')
const UserSubscription = () => import(/* webpackChunkName: "user-core" */ '../views/user/Subscription.vue')
const UserDevices = () => import(/* webpackChunkName: "user-core" */ '../views/Devices.vue')
const UserDownload = () => import(/* webpackChunkName: "user-core" */ '../views/user/Download.vue')
const UserSettings = () => import(/* webpackChunkName: "user-core" */ '../views/user/Settings.vue')

// 用户扩展页面
const Announcements = () => import(/* webpackChunkName: "user-extra" */ '../views/user/Announcements.vue')
const AnnouncementDetail = () => import(/* webpackChunkName: "user-extra" */ '../views/user/AnnouncementDetail.vue')
const Tickets = () => import(/* webpackChunkName: "user-extra" */ '../views/user/Tickets.vue')
const TicketDetail = () => import(/* webpackChunkName: "user-extra" */ '../views/user/TicketDetail.vue')
const TicketCreate = () => import(/* webpackChunkName: "user-extra" */ '../views/user/TicketCreate.vue')
const UserStats = () => import(/* webpackChunkName: "user-extra" */ '../views/user/Stats.vue')
const HelpCenter = () => import(/* webpackChunkName: "user-extra" */ '../views/user/HelpCenter.vue')
const HelpArticle = () => import(/* webpackChunkName: "user-extra" */ '../views/user/HelpArticle.vue')
const Terms = () => import(/* webpackChunkName: "user-extra" */ '../views/user/Terms.vue')
const Privacy = () => import(/* webpackChunkName: "user-extra" */ '../views/user/Privacy.vue')

// 商业化页面
const Plans = () => import(/* webpackChunkName: "user-commercial" */ '../views/user/Plans.vue')
const Orders = () => import(/* webpackChunkName: "user-commercial" */ '../views/user/Orders.vue')
const Payment = () => import(/* webpackChunkName: "user-commercial" */ '../views/user/Payment.vue')
const Balance = () => import(/* webpackChunkName: "user-commercial" */ '../views/user/Balance.vue')
const Invite = () => import(/* webpackChunkName: "user-commercial" */ '../views/user/Invite.vue')
const Invoices = () => import(/* webpackChunkName: "user-commercial" */ '../views/user/Invoices.vue')
const PlanUpgrade = () => import(/* webpackChunkName: "user-commercial" */ '../views/user/PlanUpgrade.vue')
const GiftCard = () => import(/* webpackChunkName: "user-commercial" */ '../views/user/GiftCard.vue')

/**
 * 用户前台路由配置
 */
export const userRoutes = [
  // 用户前台主路由
  {
    path: '/user',
    component: UserLayout,
    meta: { requiresUserAuth: true },
    children: [
      {
        path: '',
        redirect: '/user/dashboard'
      },
      {
        path: 'dashboard',
        name: 'UserDashboard',
        component: UserDashboard,
        meta: {
          title: '仪表板',
          requiresUserAuth: true
        }
      },
      {
        path: 'nodes',
        name: 'UserNodes',
        component: UserNodes,
        meta: {
          title: '节点列表',
          requiresUserAuth: true
        }
      },
      {
        path: 'subscription',
        name: 'UserSubscription',
        component: UserSubscription,
        meta: {
          title: '订阅管理',
          requiresUserAuth: true
        }
      },
      {
        path: 'devices',
        name: 'UserDevices',
        component: UserDevices,
        meta: {
          title: '在线设备',
          requiresUserAuth: true
        }
      },
      {
        path: 'download',
        name: 'UserDownload',
        component: UserDownload,
        meta: {
          title: '客户端下载',
          requiresUserAuth: true
        }
      },
      {
        path: 'settings',
        name: 'UserSettings',
        component: UserSettings,
        meta: {
          title: '个人设置',
          requiresUserAuth: true
        }
      },
      {
        path: 'announcements',
        name: 'Announcements',
        component: Announcements,
        meta: {
          title: '公告中心',
          requiresUserAuth: true
        }
      },
      {
        path: 'announcements/:id',
        name: 'AnnouncementDetail',
        component: AnnouncementDetail,
        meta: {
          title: '公告详情',
          requiresUserAuth: true
        }
      },
      {
        path: 'tickets',
        name: 'Tickets',
        component: Tickets,
        meta: {
          title: '工单列表',
          requiresUserAuth: true
        }
      },
      {
        path: 'tickets/create',
        name: 'TicketCreate',
        component: TicketCreate,
        meta: {
          title: '创建工单',
          requiresUserAuth: true
        }
      },
      {
        path: 'tickets/:id',
        name: 'TicketDetail',
        component: TicketDetail,
        meta: {
          title: '工单详情',
          requiresUserAuth: true
        }
      },
      {
        path: 'stats',
        name: 'UserStats',
        component: UserStats,
        meta: {
          title: '使用统计',
          requiresUserAuth: true
        }
      },
      {
        path: 'help',
        name: 'HelpCenter',
        component: HelpCenter,
        meta: {
          title: '帮助中心',
          requiresUserAuth: false // 帮助中心可以不登录访问
        }
      },
      {
        path: 'help/:slug',
        name: 'HelpArticle',
        component: HelpArticle,
        meta: {
          title: '帮助文章',
          requiresUserAuth: false
        }
      },
      // 商业化路由
      {
        path: 'plans',
        name: 'user-plans',
        component: Plans,
        meta: {
          title: '选择套餐',
          requiresUserAuth: true
        }
      },
      {
        path: 'orders',
        name: 'user-orders',
        component: Orders,
        meta: {
          title: '我的订单',
          requiresUserAuth: true
        }
      },
      {
        path: 'payment',
        name: 'user-payment',
        component: Payment,
        meta: {
          title: '订单支付',
          requiresUserAuth: true
        }
      },
      {
        path: 'balance',
        name: 'user-balance',
        component: Balance,
        meta: {
          title: '我的余额',
          requiresUserAuth: true
        }
      },
      {
        path: 'invite',
        name: 'user-invite',
        component: Invite,
        meta: {
          title: '邀请推广',
          requiresUserAuth: true
        }
      },
      {
        path: 'invoices',
        name: 'user-invoices',
        component: Invoices,
        meta: {
          title: '我的发票',
          requiresUserAuth: true
        }
      },
      {
        path: 'plan-upgrade',
        name: 'user-plan-upgrade',
        component: PlanUpgrade,
        meta: {
          title: '套餐升降级',
          requiresUserAuth: true
        }
      },
      {
        path: 'gift-card',
        name: 'user-gift-card',
        component: GiftCard,
        meta: {
          title: '礼品卡',
          requiresUserAuth: true
        }
      },
      {
        path: 'terms',
        name: 'user-terms',
        component: Terms,
        meta: {
          title: '服务条款'
        }
      },
      {
        path: 'privacy',
        name: 'user-privacy',
        component: Privacy,
        meta: {
          title: '隐私政策'
        }
      }
    ]
  },
  
  // 用户认证页面（独立布局）
  {
    path: '/user/login',
    component: AuthLayout,
    children: [
      {
        path: '',
        name: 'UserLogin',
        component: UserLogin,
        meta: {
          title: '用户登录',
          guest: true
        }
      }
    ]
  },
  {
    path: '/user/register',
    component: AuthLayout,
    children: [
      {
        path: '',
        name: 'UserRegister',
        component: UserRegister,
        meta: {
          title: '用户注册',
          guest: true
        }
      }
    ]
  },
  {
    path: '/user/forgot-password',
    component: AuthLayout,
    children: [
      {
        path: '',
        name: 'ForgotPassword',
        component: ForgotPassword,
        meta: {
          title: '忘记密码',
          guest: true
        }
      }
    ]
  },
  {
    path: '/user/reset-password',
    component: AuthLayout,
    children: [
      {
        path: '',
        name: 'ResetPassword',
        component: ResetPassword,
        meta: {
          title: '重置密码',
          guest: true
        }
      }
    ]
  }
]

/**
 * 用户前台路由守卫
 * @param {Object} to - 目标路由
 * @param {Object} from - 来源路由
 * @param {Function} next - 导航函数
 */
export function userRouteGuard(to, from, next) {
  const isUserAuthenticated = localStorage.getItem('userToken')
  const isAdminAuthenticated = localStorage.getItem('token')
  const userRole = localStorage.getItem('userRole') || 'user'
  
  // 更新页面标题
  if (to.meta.title) {
    document.title = `${to.meta.title} - V Panel`
  }
  
  // 需要用户认证的页面
  if (to.meta.requiresUserAuth && !isUserAuthenticated) {
    next({ name: 'UserLogin', query: { redirect: to.fullPath } })
    return
  }
  
  // 已登录用户访问登录/注册页面，跳转到用户仪表板
  if (to.meta.guest && isUserAuthenticated && to.path.startsWith('/user/')) {
    next('/user/dashboard')
    return
  }
  
  next()
}

export default userRoutes
