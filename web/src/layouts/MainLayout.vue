<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <el-container class="main-layout">
    <el-aside width="240px" class="sidebar">
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
        <el-sub-menu index="dashboard">
          <template #title>
            <el-icon><Odometer /></el-icon>
            <span>仪表页面</span>
          </template>
          <el-menu-item index="/dashboard">数据概览</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="warehouse">
          <template #title>
            <el-icon><Box /></el-icon>
            <span>仓库作业</span>
          </template>
          <el-menu-item index="/warehouse/inbound">入库作业</el-menu-item>
          <el-menu-item index="/warehouse/outbound">出库作业</el-menu-item>
          <el-menu-item index="/warehouse/inventory">库存管理</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="purchase">
          <template #title>
            <el-icon><ShoppingCart /></el-icon>
            <span>采购管理</span>
          </template>
          <el-menu-item index="/purchase/orders">采购订单</el-menu-item>
          <el-menu-item index="/purchase/inbound">采购入库</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="production">
          <template #title>
            <el-icon><Setting /></el-icon>
            <span>生产管理</span>
          </template>
          <el-menu-item index="/production/work-orders">生产工单</el-menu-item>
          <el-menu-item index="/production/outbound">生产领料</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="finance">
          <template #title>
            <el-icon><Money /></el-icon>
            <span>财务管理</span>
          </template>
          <el-menu-item index="/finance/cost">成本核算</el-menu-item>
          <el-menu-item index="/finance/reports">收支报表</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="mdm">
          <template #title>
            <el-icon><Briefcase /></el-icon>
            <span>主数据</span>
          </template>
          <el-menu-item index="/mdm/materials">物料管理</el-menu-item>
          <el-menu-item index="/mdm/suppliers">供应商管理</el-menu-item>
          <el-menu-item index="/mdm/warehouses">仓库管理</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="device">
          <template #title>
            <el-icon><Monitor /></el-icon>
            <span>设备管理</span>
          </template>
          <el-menu-item index="/device/list">设备列表</el-menu-item>
        </el-sub-menu>

        <el-sub-menu index="iam">
          <template #title>
            <el-icon><User /></el-icon>
            <span>系统管理</span>
          </template>
          <el-menu-item index="/iam/users">用户管理</el-menu-item>
          <el-menu-item index="/iam/roles">角色管理</el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="header">
        <div class="header-left">
          <el-icon class="collapse-btn"><Menu /></el-icon>
        </div>
        <div class="header-right">
          <el-dropdown @command="handleCommand">
            <span class="user-info">
              <el-icon><Avatar /></el-icon>
              {{ userInfo.real_name || '管理员' }}
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">个人中心</el-dropdown-item>
                <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'
import { ElMessageBox } from 'element-plus'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const activeMenu = computed(() => route.path)
const userInfo = computed(() => userStore.userInfo)

function handleCommand(cmd) {
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
}
</style>