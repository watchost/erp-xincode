// Copyright 2026 zhouhouping. All Rights Reserved.

import { createRouter, createWebHistory } from 'vue-router'

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
        meta: { title: '仪表页面', icon: 'Odometer' }
      },
      {
        path: 'warehouse/inbound',
        name: 'WarehouseInbound',
        component: () => import('@/views/warehouse/inbound.vue'),
        meta: { title: '入库作业', icon: 'Download' }
      },
      {
        path: 'warehouse/outbound',
        name: 'WarehouseOutbound',
        component: () => import('@/views/warehouse/outbound.vue'),
        meta: { title: '出库作业', icon: 'Upload' }
      },
      {
        path: 'warehouse/inventory',
        name: 'WarehouseInventory',
        component: () => import('@/views/warehouse/inventory.vue'),
        meta: { title: '库存管理', icon: 'Box' }
      },
      {
        path: 'purchase/orders',
        name: 'PurchaseOrders',
        component: () => import('@/views/purchase/orders.vue'),
        meta: { title: '采购订单', icon: 'ShoppingCart' }
      },
      {
        path: 'purchase/inbound',
        name: 'PurchaseInbound',
        component: () => import('@/views/purchase/inbound.vue'),
        meta: { title: '采购入库', icon: 'Goods' }
      },
      {
        path: 'production/work-orders',
        name: 'ProductionWorkOrders',
        component: () => import('@/views/production/work-orders.vue'),
        meta: { title: '生产工单', icon: 'Setting' }
      },
      {
        path: 'production/outbound',
        name: 'ProductionOutbound',
        component: () => import('@/views/production/outbound.vue'),
        meta: { title: '生产领料', icon: 'Tools' }
      },
      {
        path: 'finance/cost',
        name: 'FinanceCost',
        component: () => import('@/views/finance/cost.vue'),
        meta: { title: '成本核算', icon: 'Money' }
      },
      {
        path: 'finance/reports',
        name: 'FinanceReports',
        component: () => import('@/views/finance/reports.vue'),
        meta: { title: '收支报表', icon: 'DataAnalysis' }
      },
      {
        path: 'mdm/materials',
        name: 'MdmMaterials',
        component: () => import('@/views/mdm/materials.vue'),
        meta: { title: '物料管理', icon: 'Briefcase' }
      },
      {
        path: 'mdm/suppliers',
        name: 'MdmSuppliers',
        component: () => import('@/views/mdm/suppliers.vue'),
        meta: { title: '供应商管理', icon: 'OfficeBuilding' }
      },
      {
        path: 'mdm/warehouses',
        name: 'MdmWarehouses',
        component: () => import('@/views/mdm/warehouses.vue'),
        meta: { title: '仓库管理', icon: 'Warehouse' }
      },
      {
        path: 'device/list',
        name: 'DeviceList',
        component: () => import('@/views/device/list.vue'),
        meta: { title: '设备管理', icon: 'Monitor' }
      },
      {
        path: 'iam/users',
        name: 'IamUsers',
        component: () => import('@/views/iam/users.vue'),
        meta: { title: '用户管理', icon: 'User' }
      },
      {
        path: 'iam/roles',
        name: 'IamRoles',
        component: () => import('@/views/iam/roles.vue'),
        meta: { title: '角色管理', icon: 'UserFilled' }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.path === '/login') {
    if (token) {
      next('/')
    } else {
      next()
    }
  } else {
    if (token) {
      next()
    } else {
      next('/login')
    }
  }
})

export default router