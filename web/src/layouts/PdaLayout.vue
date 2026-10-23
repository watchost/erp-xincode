<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="pda-layout">
    <!-- PDA 顶部导航栏 -->
    <header class="pda-header">
      <div class="header-left">
        <el-button
          v-if="showBackButton"
          circle
          text
          class="back-btn"
          @click="goBack"
        >
          <el-icon :size="22"><ArrowLeft /></el-icon>
        </el-button>
        <span class="pda-logo-icon" v-else>
          <el-icon :size="20"><Box /></el-icon>
        </span>
        <h1 class="pda-title">{{ currentTitle }}</h1>
      </div>

      <div class="header-right">
        <!-- PWA 安装提示按钮 -->
        <el-button
          v-if="canInstallPwa"
          size="small"
          type="warning"
          round
          class="pwa-install-btn"
          @click="handleInstallPwa"
        >
          <el-icon><Download /></el-icon>
          <span>安装App</span>
        </el-button>

        <!-- 网络状态胶囊 -->
        <div class="status-badge" :class="isOnline ? 'online' : 'offline'">
          <span class="status-dot"></span>
          <span class="status-text">{{ isOnline ? '在线' : '离线' }}</span>
        </div>

        <!-- 离线待同步记录角标入口 -->
        <div class="queue-link" @click="router.push('/pda/offline-queue')">
          <el-badge :value="pendingCount" :hidden="pendingCount === 0" :max="99" class="badge-item">
            <el-button circle text class="queue-btn">
              <el-icon :size="20"><Lightning /></el-icon>
            </el-button>
          </el-badge>
        </div>

        <!-- 用户与退出菜单 -->
        <el-dropdown trigger="click" @command="handleUserCommand">
          <div class="user-avatar-btn">
            <el-icon :size="18"><UserFilled /></el-icon>
          </div>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item disabled>
                <strong>{{ userInfo?.real_name || '作业员' }}</strong>
              </el-dropdown-item>
              <el-dropdown-item command="desktop" divided>
                <el-icon><Monitor /></el-icon>
                <span>切换至桌面端</span>
              </el-dropdown-item>
              <el-dropdown-item command="queue">
                <el-icon><Files /></el-icon>
                <span>离线数据同步</span>
              </el-dropdown-item>
              <el-dropdown-item command="logout" divided>
                <el-icon><SwitchButton /></el-icon>
                <span>退出登录</span>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </header>

    <!-- PDA 主工作区域 -->
    <main class="pda-main">
      <router-view />
    </main>

    <!-- PDA 底部触控快捷导航 -->
    <nav class="pda-bottom-nav">
      <router-link to="/pda" class="nav-item" :class="{ active: route.path === '/pda' }">
        <el-icon :size="22"><HomeFilled /></el-icon>
        <span>工作台</span>
      </router-link>
      <router-link to="/pda/inbound" class="nav-item" :class="{ active: route.path === '/pda/inbound' }">
        <el-icon :size="22"><Download /></el-icon>
        <span>扫码入库</span>
      </router-link>
      <router-link to="/pda/outbound" class="nav-item" :class="{ active: route.path === '/pda/outbound' }">
        <el-icon :size="22"><Upload /></el-icon>
        <span>扫码出库</span>
      </router-link>
      <router-link to="/pda/stock-check" class="nav-item" :class="{ active: route.path === '/pda/stock-check' }">
        <el-icon :size="22"><DocumentChecked /></el-icon>
        <span>盘点录入</span>
      </router-link>
      <router-link to="/pda/inventory" class="nav-item" :class="{ active: route.path === '/pda/inventory' }">
        <el-icon :size="22"><Search /></el-icon>
        <span>快速查库</span>
      </router-link>
    </nav>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'
import { isOnline, canInstallPwa, promptInstallPwa } from '@/utils/pwa'
import { pendingCount } from '@/utils/pdaOffline'
import { ElMessageBox, ElMessage } from 'element-plus'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const userInfo = computed(() => userStore.userInfo || {})
const showBackButton = computed(() => route.path !== '/pda')
const currentTitle = computed(() => route.meta?.title || 'PDA仓储作业')

function goBack() {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/pda')
  }
}

async function handleInstallPwa() {
  const installed = await promptInstallPwa()
  if (installed) {
    ElMessage.success('已成功添加到主屏幕！')
  }
}

function handleUserCommand(cmd) {
  if (cmd === 'desktop') {
    router.push('/dashboard')
  } else if (cmd === 'queue') {
    router.push('/pda/offline-queue')
  } else if (cmd === 'logout') {
    ElMessageBox.confirm('确定要退出当前作业端吗？', '提示', {
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
.pda-layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 100vw;
  max-width: 600px;
  margin: 0 auto;
  background-color: #f5f7fa;
  position: relative;
  overflow: hidden;
  box-shadow: 0 0 20px rgba(0, 0, 0, 0.1);
}

.pda-header {
  height: 56px;
  background: #001529;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  flex-shrink: 0;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  z-index: 100;

  .header-left {
    display: flex;
    align-items: center;
    gap: 8px;

    .back-btn {
      color: #fff;
      padding: 6px;
    }

    .pda-logo-icon {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 32px;
      height: 32px;
      background: #1890ff;
      border-radius: 8px;
    }

    .pda-title {
      font-size: 17px;
      font-weight: 600;
      margin: 0;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      max-width: 140px;
    }
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 8px;

    .pwa-install-btn {
      font-size: 12px;
      padding: 4px 10px;
      height: 28px;
    }

    .status-badge {
      display: flex;
      align-items: center;
      gap: 5px;
      padding: 3px 8px;
      border-radius: 12px;
      font-size: 11px;
      font-weight: 600;

      .status-dot {
        width: 7px;
        height: 7px;
        border-radius: 50%;
      }

      &.online {
        background: rgba(82, 196, 26, 0.2);
        color: #52c41a;
        .status-dot {
          background: #52c41a;
          box-shadow: 0 0 6px #52c41a;
        }
      }

      &.offline {
        background: rgba(255, 77, 79, 0.2);
        color: #ff4d4f;
        .status-dot {
          background: #ff4d4f;
          box-shadow: 0 0 6px #ff4d4f;
        }
      }
    }

    .queue-link {
      cursor: pointer;
      display: flex;
      align-items: center;

      .queue-btn {
        color: #fff;
        padding: 6px;
      }
    }

    .user-avatar-btn {
      width: 32px;
      height: 32px;
      border-radius: 50%;
      background: #1f3a5c;
      display: flex;
      align-items: center;
      justify-content: center;
      cursor: pointer;
      color: #fff;
      border: 1px solid rgba(255, 255, 255, 0.2);
    }
  }
}

.pda-main {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
  padding-bottom: 72px; /* Leave space for bottom nav */
  -webkit-overflow-scrolling: touch;
}

.pda-bottom-nav {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 60px;
  background: #ffffff;
  border-top: 1px solid #ebeef5;
  display: flex;
  align-items: center;
  justify-content: space-around;
  z-index: 100;
  box-shadow: 0 -2px 10px rgba(0, 0, 0, 0.05);
  padding-bottom: env(safe-area-inset-bottom, 0);

  .nav-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-decoration: none;
    color: #909399;
    font-size: 11px;
    font-weight: 500;
    gap: 3px;
    flex: 1;
    height: 100%;
    transition: all 0.2s ease;

    &:active {
      transform: scale(0.92);
    }

    &.active {
      color: #1890ff;
      font-weight: 600;

      .el-icon {
        transform: translateY(-2px);
      }
    }
  }
}
</style>
