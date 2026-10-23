<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <el-container class="main-layout">
    <!-- 桌面端固定侧栏；窄屏改为抽屉 -->
    <el-aside v-if="!isMobile" width="240px" class="sidebar">
      <div class="logo">
        <h2>ERP管理系统</h2>
        <p class="copyright">© 2026 zhouhouping</p>
      </div>
      <el-menu
        :default-active="activeMenu"
        router
        background-color="#001529"
        text-color="#fff"
        active-text-color="#409EFF"
      >
        <el-sub-menu v-for="group in visibleMenus" :key="group.index" :index="group.index">
          <template #title>
            <el-icon><component :is="group.icon" /></el-icon>
            <span>{{ group.title }}</span>
          </template>
          <el-menu-item v-for="item in group.items" :key="item.index" :index="item.index">
            {{ item.label }}
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-aside>

    <el-drawer
      v-else
      v-model="drawerOpen"
      direction="ltr"
      size="240px"
      :with-header="false"
      class="sidebar-drawer"
    >
      <div class="sidebar">
        <div class="logo">
          <h2>ERP管理系统</h2>
          <p class="copyright">© 2026 zhouhouping</p>
        </div>
        <el-menu
          :default-active="activeMenu"
          router
          background-color="#001529"
          text-color="#fff"
          active-text-color="#409EFF"
          @select="drawerOpen = false"
        >
          <el-sub-menu v-for="group in visibleMenus" :key="group.index" :index="group.index">
            <template #title>
              <el-icon><component :is="group.icon" /></el-icon>
              <span>{{ group.title }}</span>
            </template>
            <el-menu-item v-for="item in group.items" :key="item.index" :index="item.index">
              {{ item.label }}
            </el-menu-item>
          </el-sub-menu>
        </el-menu>
      </div>
    </el-drawer>

    <el-container class="main-right">
      <el-header class="header" :class="{ mobile: isMobile }">
        <div class="header-left">
          <el-icon class="collapse-btn" @click="drawerOpen = true"><Menu /></el-icon>
        </div>
        <div class="header-right">
          <el-button
            v-if="!isMobile"
            type="primary"
            round
            size="small"
            class="pda-launcher-btn"
            @click="router.push('/pda')"
          >
            <el-icon><Iphone /></el-icon>
            <span>PDA作业端</span>
          </el-button>

          <NotificationBell v-if="canViewMessages" />

          <el-dropdown @command="handleCommand">
            <span class="user-info">
              <el-icon><Avatar /></el-icon>
              <span class="user-name">{{ userInfo?.real_name || '管理员' }}</span>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">个人中心</el-dropdown-item>
                <el-dropdown-item v-if="isMobile" command="pda">PDA作业端</el-dropdown-item>
                <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="main-content" :class="{ mobile: isMobile }">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'
import { ElMessageBox } from 'element-plus'
import { hasPerm } from '@/utils/permission'
import { useIsMobile } from '@/composables/useLayout'
import NotificationBell from '@/components/NotificationBell.vue'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const isMobile = useIsMobile()
const drawerOpen = ref(false)

const activeMenu = computed(() => route.path)
const userInfo = computed(() => userStore.userInfo || {})
const canViewMessages = computed(() => hasPerm(userStore.permissions, 'message:view'))

// 菜单定义：perm 为进入该菜单所需权限码（与后端 sys_permission 种子一致）。
const menus = [
  {
    index: 'dashboard', icon: 'Odometer', title: '仪表页面',
    items: [{ index: '/dashboard', label: '数据概览', perm: 'dashboard:view' }]
  },
  {
    index: 'warehouse', icon: 'Box', title: '仓库作业',
    items: [
      { index: '/warehouse/inbound', label: '入库作业', perm: 'warehouse:inbound' },
      { index: '/warehouse/outbound', label: '出库作业', perm: 'warehouse:outbound' },
      { index: '/warehouse/inventory', label: '库存管理', perm: 'warehouse:inventory' },
      { index: '/pda', label: 'PDA手持终端', perm: 'warehouse:inbound' }
    ]
  },
  {
    index: 'purchase', icon: 'ShoppingCart', title: '采购管理',
    items: [
      { index: '/purchase/orders', label: '采购订单', perm: 'purchase:order:view' },
      { index: '/purchase/inbound', label: '采购入库', perm: 'purchase:inbound' }
    ]
  },
  {
    index: 'production', icon: 'Setting', title: '生产管理',
    items: [
      { index: '/production/work-orders', label: '生产工单', perm: 'production:wo:view' },
      { index: '/production/outbound', label: '生产领料', perm: 'production:outbound' }
    ]
  },
  {
    index: 'finance', icon: 'Money', title: '财务管理',
    items: [
      { index: '/finance/cost', label: '成本核算', perm: 'finance:cost' },
      { index: '/finance/reports', label: '收支报表', perm: 'finance:reports' }
    ]
  },
  {
    index: 'mdm', icon: 'Briefcase', title: '主数据',
    items: [
      { index: '/mdm/materials', label: '物料管理', perm: 'mdm:material' },
      { index: '/mdm/suppliers', label: '供应商管理', perm: 'mdm:supplier' },
      { index: '/mdm/warehouses', label: '仓库管理', perm: 'mdm:warehouse' }
    ]
  },
  {
    index: 'device', icon: 'Monitor', title: '设备管理',
    items: [{ index: '/device/list', label: '设备列表', perm: 'device:manage' }]
  },
  {
    index: 'workflow', icon: 'Stamp', title: '审批中心',
    items: [
      { index: '/workflow/todo', label: '我的审批', perm: 'workflow:task' },
      { index: '/messages', label: '消息中心', perm: 'message:view' },
      { index: '/workflow/definitions', label: '流程定义', perm: 'workflow:definition' }
    ]
  },
  {
    index: 'iam', icon: 'User', title: '系统管理',
    items: [
      { index: '/iam/users', label: '用户管理', perm: 'iam:user' },
      { index: '/iam/roles', label: '角色管理', perm: 'iam:role' }
    ]
  }
]

// 按当前用户权限过滤菜单：无权限的菜单项隐藏，整个分组无可见项时隐藏分组。
const visibleMenus = computed(() =>
  menus
    .map((g) => ({ ...g, items: g.items.filter((i) => hasPerm(userStore.permissions, i.perm)) }))
    .filter((g) => g.items.length > 0)
)

// 刷新页面后用户信息可能缺失（或本地缓存损坏），有 token 时补拉一次。
// 权限码由路由守卫在进入页面前拉取，这里兜底补拉用户信息。
onMounted(() => {
  if (userStore.token && !userStore.userInfo?.real_name) {
    userStore.fetchUserInfo()
  }
})

function handleCommand(cmd) {
  if (cmd === 'pda') {
    router.push('/pda')
    return
  }
  if (cmd === 'logout') {
    ElMessageBox.confirm('确定要退出登录吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }).then(() => {
      userStore.logout()
      router.push('/login')
    }).catch(() => {})
  }
}
</script>

<style scoped lang="scss">
.main-layout {
  height: 100vh;
}

.sidebar {
  background-color: #001529;
  overflow-y: auto;

  .logo {
    height: 60px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    border-bottom: 1px solid #1f3a5c;

    h2 {
      color: #fff;
      font-size: 18px;
      margin: 0;
    }

    .copyright {
      color: #8c9bab;
      font-size: 11px;
      margin: 2px 0 0 0;
    }
  }

  :deep(.el-menu) {
    border-right: none;
  }
}

.header {
  background: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  box-shadow: 0 1px 4px rgba(0, 21, 41, 0.08);

  .header-left {
    .collapse-btn {
      font-size: 20px;
      cursor: pointer;
    }
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 16px;

    .pda-launcher-btn {
      font-size: 13px;
      font-weight: 600;
      background: linear-gradient(135deg, #1890ff, #096dd9);
      border: none;
      box-shadow: 0 2px 6px rgba(24, 144, 255, 0.3);
    }

    .user-info {
      display: flex;
      align-items: center;
      gap: 6px;
      cursor: pointer;
      color: #303133;
    }
  }
}

.main-content {
  background-color: #f0f2f5;
  padding: 20px;
  overflow-y: auto;

  &.mobile {
    padding: 10px;
  }
}

.main-right {
  min-width: 0;
}

.header.mobile {
  padding: 0 12px;

  .header-right {
    gap: 10px;
  }

  .user-name {
    max-width: 72px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.sidebar-drawer .sidebar {
  min-height: 100%;
}
</style>

<style lang="scss">
/* el-drawer 会被挂到 body 下，scoped 样式作用不到，单独写全局样式 */
.sidebar-drawer .el-drawer__body {
  padding: 0;
  background-color: #001529;
}
</style>