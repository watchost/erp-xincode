// Copyright 2026 zhouhouping. All Rights Reserved.

import { createRouter, createWebHistory } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/store/user'
import { hasPerm } from '@/utils/permission'

// meta.permission：进入该路由所需的权限码（与后端 sys_permission 种子一致）。
// 不写 permission 的路由（如 dashboard）登录即可访问。
const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/login/index.vue'),
    meta: { title: '登录' }
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/dashboard/index.vue'),
        meta: { title: '仪表页面', icon: 'Odometer', permission: 'dashboard:view' }
      },
      {
        path: 'warehouse/inbound',
        name: 'WarehouseInbound',
        component: () => import('@/views/warehouse/inbound.vue'),
        meta: { title: '入库作业', icon: 'Download', permission: 'warehouse:inbound' }
      },
      {
        path: 'warehouse/outbound',
        name: 'WarehouseOutbound',
        component: () => import('@/views/warehouse/outbound.vue'),
        meta: { title: '出库作业', icon: 'Upload', permission: 'warehouse:outbound' }
      },
      {
        path: 'warehouse/inventory',
        name: 'WarehouseInventory',
        component: () => import('@/views/warehouse/inventory.vue'),
        meta: { title: '库存管理', icon: 'Box', permission: 'warehouse:inventory' }
      },
      {
        path: 'purchase/orders',
        name: 'PurchaseOrders',
        component: () => import('@/views/purchase/orders.vue'),
        meta: { title: '采购订单', icon: 'ShoppingCart', permission: 'purchase:order:view' }
      },
      {
        path: 'purchase/inbound',
        name: 'PurchaseInbound',
        component: () => import('@/views/purchase/inbound.vue'),
        meta: { title: '采购入库', icon: 'Goods', permission: 'purchase:inbound' }
      },
      {
        path: 'production/work-orders',
        name: 'ProductionWorkOrders',
        component: () => import('@/views/production/work-orders.vue'),
        meta: { title: '生产工单', icon: 'Setting', permission: 'production:wo:view' }
      },
      {
        path: 'production/outbound',
        name: 'ProductionOutbound',
        component: () => import('@/views/production/outbound.vue'),
        meta: { title: '生产领料', icon: 'Tools', permission: 'production:outbound' }
      },
      {
        path: 'finance/cost',
        name: 'FinanceCost',
        component: () => import('@/views/finance/cost.vue'),
        meta: { title: '成本核算', icon: 'Money', permission: 'finance:cost' }
      },
      {
        path: 'finance/reports',
        name: 'FinanceReports',
        component: () => import('@/views/finance/reports.vue'),
        meta: { title: '收支报表', icon: 'DataAnalysis', permission: 'finance:reports' }
      },
      {
        path: 'mdm/materials',
        name: 'MdmMaterials',
        component: () => import('@/views/mdm/materials.vue'),
        meta: { title: '物料管理', icon: 'Briefcase', permission: 'mdm:material' }
      },
      {
        path: 'mdm/suppliers',
        name: 'MdmSuppliers',
        component: () => import('@/views/mdm/suppliers.vue'),
        meta: { title: '供应商管理', icon: 'OfficeBuilding', permission: 'mdm:supplier' }
      },
      {
        path: 'mdm/warehouses',
        name: 'MdmWarehouses',
        component: () => import('@/views/mdm/warehouses.vue'),
        meta: { title: '仓库管理', icon: 'Warehouse', permission: 'mdm:warehouse' }
      },
      {
        path: 'device/list',
        name: 'DeviceList',
        component: () => import('@/views/device/list.vue'),
        meta: { title: '设备管理', icon: 'Monitor', permission: 'device:manage' }
      },
      {
        path: 'iam/users',
        name: 'IamUsers',
        component: () => import('@/views/iam/users.vue'),
        meta: { title: '用户管理', icon: 'User', permission: 'iam:user' }
      },
      {
        path: 'iam/roles',
        name: 'IamRoles',
        component: () => import('@/views/iam/roles.vue'),
        meta: { title: '角色管理', icon: 'UserFilled', permission: 'iam:role' }
      }
    ]
  },
  {
    path: '/pda',
    component: () => import('@/layouts/PdaLayout.vue'),
    children: [
      {
        path: '',
        name: 'PdaHome',
        component: () => import('@/views/pda/index.vue'),
        meta: { title: 'PDA工作台' }
      },
      {
        path: 'inbound',
        name: 'PdaInbound',
        component: () => import('@/views/pda/inbound.vue'),
        meta: { title: '扫码入库', permission: 'warehouse:inbound' }
      },
      {
        path: 'outbound',
        name: 'PdaOutbound',
        component: () => import('@/views/pda/outbound.vue'),
        meta: { title: '扫码出库', permission: 'warehouse:outbound' }
      },
      {
        path: 'stock-check',
        name: 'PdaStockCheck',
        component: () => import('@/views/pda/stock-check.vue'),
        meta: { title: '盘点录入', permission: 'warehouse:check:entry' }
      },
      {
        path: 'inventory',
        name: 'PdaInventory',
        component: () => import('@/views/pda/inventory.vue'),
        meta: { title: '快速查库', permission: 'warehouse:inventory' }
      },
      {
        path: 'offline-queue',
        name: 'PdaOfflineQueue',
        component: () => import('@/views/pda/offline-queue.vue'),
        meta: { title: '离线同步队列' }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.path === '/login') {
    next(token ? '/' : undefined)
    return
  }
  if (!token) {
    next('/login')
    return
  }

  // 已登录：确保权限码已加载（刷新页面后 store 内存丢失，需重新拉取）。
  const userStore = useUserStore()
  if (!userStore.permsLoaded) {
    await userStore.fetchPermissions()
  }

  // 路由级权限：目标路由要求的权限码不满足时，回退到首页并提示。
  if (to.meta?.permission && !hasPerm(userStore.permissions, to.meta.permission)) {
    ElMessage.error('无权限访问该页面')
    next(from.path && from.path !== to.path ? from.path : '/dashboard')
    return
  }
  next()
})

export default router