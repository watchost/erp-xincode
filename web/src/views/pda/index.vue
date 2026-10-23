<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="pda-home">
    <!-- 用户作业状态卡片 -->
    <div class="user-status-card">
      <div class="user-info">
        <div class="avatar-box">
          <el-icon :size="24"><User /></el-icon>
        </div>
        <div class="info-texts">
          <div class="user-name">{{ userInfo?.real_name || '仓库作业员' }}</div>
          <div class="user-dept">{{ userInfo?.username || 'user' }} · 移动作业模式</div>
        </div>
      </div>
      <div class="network-tag" :class="isOnline ? 'online' : 'offline'">
        {{ isOnline ? '网络畅通' : '离线模式' }}
      </div>
    </div>

    <!-- 离线待同步快捷提醒条 -->
    <div v-if="pendingCount > 0" class="offline-alert-bar" @click="router.push('/pda/offline-queue')">
      <el-icon class="alert-icon"><WarningFilled /></el-icon>
      <span class="alert-text">存在 <strong>{{ pendingCount }}</strong> 条未同步的离线扫描记录</span>
      <el-button size="small" type="primary" round class="sync-now-btn">一键同步</el-button>
    </div>

    <!-- 快捷通用扫描栏 -->
    <div class="quick-scan-section">
      <BarcodeScanner listen-hardware @scan="handleGlobalScan">
        <template #trigger="{ open }">
          <div class="scanner-hero-btn" @click="open">
            <div class="hero-icon-pulse">
              <el-icon :size="28"><Aim /></el-icon>
            </div>
            <div class="hero-texts">
              <div class="hero-title">一键开启智能扫码</div>
              <div class="hero-subtitle">支持摄像头与 PDA 硬件激光扫码</div>
            </div>
            <el-icon class="arrow-right"><ArrowRight /></el-icon>
          </div>
        </template>
      </BarcodeScanner>
    </div>

    <!-- 核心作业功能九宫格（大触控按键） -->
    <div class="pda-grid-menu">
      <div class="grid-card inbound-card" @click="router.push('/pda/inbound')">
        <div class="card-icon-wrap inbound">
          <el-icon :size="32"><Download /></el-icon>
        </div>
        <div class="card-title">扫码入库</div>
        <div class="card-desc">物料到货上架与扫码入库</div>
      </div>

      <div class="grid-card outbound-card" @click="router.push('/pda/outbound')">
        <div class="card-icon-wrap outbound">
          <el-icon :size="32"><Upload /></el-icon>
        </div>
        <div class="card-title">扫码出库</div>
        <div class="card-desc">订单拣选与物料出库</div>
      </div>

      <div class="grid-card check-card" @click="router.push('/pda/stock-check')">
        <div class="card-icon-wrap check">
          <el-icon :size="32"><DocumentChecked /></el-icon>
        </div>
        <div class="card-title">盘点录入</div>
        <div class="card-desc">盲盘/明盘实物快速录入</div>
      </div>

      <div class="grid-card query-card" @click="router.push('/pda/inventory')">
        <div class="card-icon-wrap query">
          <el-icon :size="32"><Search /></el-icon>
        </div>
        <div class="card-title">快速查库</div>
        <div class="card-desc">扫码查库位物料与可用量</div>
      </div>

      <div class="grid-card queue-card" @click="router.push('/pda/offline-queue')">
        <div class="card-icon-wrap queue">
          <el-badge :value="pendingCount" :hidden="pendingCount === 0" :max="99">
            <el-icon :size="32"><Lightning /></el-icon>
          </el-badge>
        </div>
        <div class="card-title">离线队列</div>
        <div class="card-desc">{{ pendingCount > 0 ? `${pendingCount}条待同步` : '无挂起数据' }}</div>
      </div>

      <div v-if="canViewWorkflow" class="grid-card approval-card" @click="router.push('/pda/workflow/todo')">
        <div class="card-icon-wrap approval">
          <el-icon :size="32"><Stamp /></el-icon>
        </div>
        <div class="card-title">我的审批</div>
        <div class="card-desc">待办 / 已办 / 审批历史</div>
      </div>

      <div v-if="canViewMessages" class="grid-card message-card" @click="router.push('/pda/messages')">
        <div class="card-icon-wrap message">
          <el-icon :size="32"><ChatDotRound /></el-icon>
        </div>
        <div class="card-title">消息</div>
        <div class="card-desc">审批待办、库存预警</div>
      </div>

      <div class="grid-card sound-card" @click="testSoundFeedback">
        <div class="card-icon-wrap sound">
          <el-icon :size="32"><BellFilled /></el-icon>
        </div>
        <div class="card-title">声震测试</div>
        <div class="card-desc">PDA 蜂鸣器与震动校准</div>
      </div>
    </div>

    <!-- iOS 不支持一键安装，给出手动添加步骤 -->
    <div v-if="showIosInstallGuide" class="pwa-guide-card">
      <div class="pwa-guide-content">
        <el-icon :size="24" color="#1890ff"><Iphone /></el-icon>
        <div class="pwa-text">
          <div class="pwa-title">添加到主屏幕（iPhone / iPad）</div>
          <div class="pwa-desc">在 Safari 中点底部“分享”按钮，选择“添加到主屏幕”</div>
        </div>
      </div>
    </div>

    <!-- PWA 安装提示卡片 -->
    <div v-if="canInstallPwa" class="pwa-guide-card">
      <div class="pwa-guide-content">
        <el-icon :size="24" color="#1890ff"><Iphone /></el-icon>
        <div class="pwa-text">
          <div class="pwa-title">可添加到桌面作为独立 App</div>
          <div class="pwa-desc">获得原生应用级离线体验与全屏操作</div>
        </div>
      </div>
      <el-button type="primary" size="small" round @click="handleInstallPwa">立即添加</el-button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/store/user'
import { isOnline, canInstallPwa, promptInstallPwa, showIosInstallGuide } from '@/utils/pwa'
import { pendingCount } from '@/utils/pdaOffline'
import { hasPerm } from '@/utils/permission'
import { playSuccessSound } from '@/utils/sound'
import BarcodeScanner from '@/components/pda/BarcodeScanner.vue'
import { ElMessage } from 'element-plus'

const router = useRouter()
const userStore = useUserStore()

const userInfo = computed(() => userStore.userInfo || {})
const canViewWorkflow = computed(() => hasPerm(userStore.permissions, 'workflow:task'))
const canViewMessages = computed(() => hasPerm(userStore.permissions, 'message:view'))

function handleGlobalScan(code) {
  ElMessage.success(`识别到条码：${code}`)
  // 根据条码特征或跳转到查库页面
  router.push({ path: '/pda/inventory', query: { code } })
}

function testSoundFeedback() {
  playSuccessSound()
  ElMessage.success('PDA 声效与震动反馈正常')
}

async function handleInstallPwa() {
  const installed = await promptInstallPwa()
  if (installed) {
    ElMessage.success('已添加到主屏幕！')
  }
}
</script>

<style scoped lang="scss">
.pda-home {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.user-status-card {
  background: #ffffff;
  border-radius: 12px;
  padding: 14px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);

  .user-info {
    display: flex;
    align-items: center;
    gap: 12px;

    .avatar-box {
      width: 44px;
      height: 44px;
      border-radius: 10px;
      background: linear-gradient(135deg, #1890ff, #096dd9);
      color: #fff;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .user-name {
      font-size: 16px;
      font-weight: 600;
      color: #1f2329;
    }

    .user-dept {
      font-size: 12px;
      color: #8f959e;
      margin-top: 2px;
    }
  }

  .network-tag {
    font-size: 12px;
    font-weight: 600;
    padding: 4px 10px;
    border-radius: 12px;

    &.online {
      background: #f6ffed;
      color: #52c41a;
      border: 1px solid #b7eb8f;
    }

    &.offline {
      background: #fff2f0;
      color: #ff4d4f;
      border: 1px solid #ffccc7;
    }
  }
}

.offline-alert-bar {
  background: #fffbe6;
  border: 1px solid #ffe58f;
  border-radius: 10px;
  padding: 10px 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;

  .alert-icon {
    color: #faad14;
    font-size: 18px;
  }

  .alert-text {
    flex: 1;
    font-size: 13px;
    color: #874d00;
    margin: 0 10px;
  }

  .sync-now-btn {
    font-size: 12px;
  }
}

.quick-scan-section {
  .scanner-hero-btn {
    background: linear-gradient(135deg, #001529 0%, #1f3a5c 100%);
    border-radius: 14px;
    padding: 16px 18px;
    color: #fff;
    display: flex;
    align-items: center;
    gap: 14px;
    cursor: pointer;
    box-shadow: 0 6px 18px rgba(0, 21, 41, 0.25);
    transition: transform 0.15s ease;

    &:active {
      transform: scale(0.98);
    }

    .hero-icon-pulse {
      width: 50px;
      height: 50px;
      border-radius: 12px;
      background: rgba(24, 144, 255, 0.25);
      border: 1px solid #1890ff;
      display: flex;
      align-items: center;
      justify-content: center;
      color: #40a9ff;
    }

    .hero-texts {
      flex: 1;

      .hero-title {
        font-size: 16px;
        font-weight: 600;
      }

      .hero-subtitle {
        font-size: 12px;
        color: #8c9bab;
        margin-top: 3px;
      }
    }

    .arrow-right {
      font-size: 18px;
      color: #8c9bab;
    }
  }
}

.pda-grid-menu {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;

  .grid-card {
    background: #ffffff;
    border-radius: 14px;
    padding: 18px 14px;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
    cursor: pointer;
    transition: all 0.2s ease;
    border: 1px solid transparent;

    &:active {
      transform: scale(0.96);
      background: #fafafa;
    }

    .card-icon-wrap {
      width: 58px;
      height: 58px;
      border-radius: 16px;
      display: flex;
      align-items: center;
      justify-content: center;
      margin-bottom: 10px;

      &.inbound {
        background: #e6f7ff;
        color: #1890ff;
      }

      &.outbound {
        background: #f6ffed;
        color: #52c41a;
      }

      &.check {
        background: #fff7e6;
        color: #fa8c16;
      }

      &.query {
        background: #f9f0ff;
        color: #722ed1;
      }

      &.queue {
        background: #fff0f6;
        color: #eb2f96;
      }

      &.sound {
        background: #e6fffb;
        color: #13c2c2;
      }

      &.approval {
        background: #fff7e6;
        color: #fa8c16;
      }

      &.message {
        background: #e6f4ff;
        color: #1677ff;
      }
    }

    .card-title {
      font-size: 15px;
      font-weight: 600;
      color: #1f2329;
    }

    .card-desc {
      font-size: 11px;
      color: #8f959e;
      margin-top: 4px;
    }
  }
}

.pwa-guide-card {
  background: #e6f7ff;
  border: 1px solid #91d5ff;
  border-radius: 12px;
  padding: 12px 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;

  .pwa-guide-content {
    display: flex;
    align-items: center;
    gap: 10px;

    .pwa-title {
      font-size: 13px;
      font-weight: 600;
      color: #003a8c;
    }

    .pwa-desc {
      font-size: 11px;
      color: #096dd9;
      margin-top: 2px;
    }
  }
}
</style>
