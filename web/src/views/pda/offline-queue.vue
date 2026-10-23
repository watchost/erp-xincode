<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="pda-queue-page">
    <!-- 顶部同步状态卡片 -->
    <div class="sync-summary-card">
      <div class="summary-info">
        <div class="queue-status-text">
          <div class="queue-count">
            <strong>{{ queueList.length }}</strong> 条离线记录
          </div>
          <div class="net-status" :class="isOnline ? 'online' : 'offline'">
            <span class="dot"></span>
            <span>{{ isOnline ? '网络畅通，可执行同步' : '当前无网络，请联网后同步' }}</span>
          </div>
        </div>
      </div>

      <div class="summary-actions">
        <el-button
          type="primary"
          size="large"
          class="sync-all-btn"
          :loading="syncing"
          :disabled="queueList.length === 0 || !isOnline"
          @click="handleSyncAll"
        >
          <el-icon><Refresh /></el-icon>
          <span>一键全部同步</span>
        </el-button>
      </div>
    </div>

    <!-- 队列记录清单 -->
    <div class="queue-list-container">
      <div class="list-header">
        <span class="list-title">待同步队列列表</span>
        <el-button
          v-if="queueList.length > 0"
          type="danger"
          link
          size="small"
          :disabled="syncing"
          @click="confirmClearAll"
        >
          清空全部
        </el-button>
      </div>

      <div v-if="queueList.length === 0" class="empty-queue-card">
        <el-icon :size="48" color="#52c41a"><CircleCheckFilled /></el-icon>
        <p class="empty-title">离线队列已全部清空</p>
        <span class="empty-desc">所有移动扫码作业记录均已完成云端同步</span>
      </div>

      <div v-else class="queue-items-list">
        <div
          v-for="item in queueList"
          :key="item.id"
          class="queue-item-card"
          :class="item.status"
        >
          <div class="item-header">
            <div class="type-and-title">
              <span class="type-tag" :class="item.type">{{ getTypeLabel(item.type) }}</span>
              <span class="item-title">{{ item.title }}</span>
            </div>
            <span class="status-tag" :class="item.status">{{ getStatusLabel(item.status) }}</span>
          </div>

          <!-- Payload 详情 -->
          <div class="item-details">
            <div v-if="item.payload.material_code" class="detail-row">
              <span class="k">物料:</span>
              <span class="v">{{ item.payload.material_code }} ({{ item.payload.qty ?? item.payload.actual_qty }}件)</span>
            </div>
            <div v-if="item.payload.check_no" class="detail-row">
              <span class="k">盘点单:</span>
              <span class="v">{{ item.payload.check_no }} / {{ item.payload.location_code || '不指定库位' }}</span>
            </div>
            <div v-if="item.payload.warehouse_code" class="detail-row">
              <span class="k">仓库/库位:</span>
              <span class="v">{{ item.payload.warehouse_code }} / {{ item.payload.location_code }}</span>
            </div>
            <div class="detail-row">
              <span class="k">录入时间:</span>
              <span class="v">{{ item.createdAt }}</span>
            </div>
            <div v-if="item.lastError" class="error-tip">
              失败原因: {{ item.lastError }}
            </div>
          </div>

          <!-- 操作条 -->
          <div class="item-actions">
            <el-button
              size="small"
              type="primary"
              text
              :loading="item.status === 'syncing'"
              :disabled="!isOnline"
              @click="syncSingleItem(item)"
            >
              重试同步
            </el-button>
            <el-button
              size="small"
              type="danger"
              text
              :disabled="item.status === 'syncing'"
              @click="removeItem(item.id)"
            >
              删除
            </el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import {
  offlineQueue,
  removeOfflineQueueItem,
  clearOfflineQueue,
  syncOfflineQueue,
  updateQueueItem
} from '@/utils/pdaOffline'
import { isOnline } from '@/utils/pwa'
import { inboundScan, outboundScan, entryStockCheck } from '@/api/warehouse'
import { buildStockCheckEntry, apiErrorMessage } from '@/utils/pdaResolve'
import { ElMessage, ElMessageBox } from 'element-plus'

const syncing = ref(false)
const queueList = computed(() => offlineQueue.value || [])

// 映射各业务类型的实际 API Handler；重发时带上入队时生成的幂等键
const apiHandlers = {
  inbound: (payload, idempKey) => inboundScan(payload, idempKey),
  outbound: (payload, idempKey) => outboundScan(payload, idempKey),
  // 盘点记录离线时只存了编码，同步时再解析为 material_id / location_id
  stock_check: async (payload, idempKey) => {
    const body = await buildStockCheckEntry(payload)
    return entryStockCheck(payload.check_id, body, idempKey)
  }
}

function getTypeLabel(type) {
  switch (type) {
    case 'inbound':
      return '📥 扫码入库'
    case 'outbound':
      return '📤 扫码出库'
    case 'stock_check':
      return '📋 盘点录入'
    default:
      return '作业记录'
  }
}

function getStatusLabel(status) {
  switch (status) {
    case 'syncing':
      return '同步中...'
    case 'failed':
      return '同步失败'
    case 'pending':
    default:
      return '待同步'
  }
}

async function handleSyncAll() {
  if (!isOnline.value) {
    ElMessage.warning('当前处于离线状态，请连接网络后重试')
    return
  }

  syncing.value = true
  try {
    const res = await syncOfflineQueue(apiHandlers)
    if (res.total > 0) {
      if (res.failed === 0) {
        ElMessage.success(`成功同步全部 ${res.success} 条离线数据！`)
      } else {
        ElMessage.warning(`同步完成：成功 ${res.success} 条，失败 ${res.failed} 条`)
      }
    }
  } catch (err) {
    ElMessage.error(err.message || '批量同步出现异常')
  } finally {
    syncing.value = false
  }
}

async function syncSingleItem(item) {
  if (!isOnline.value) {
    ElMessage.warning('当前处于离线状态，无法同步')
    return
  }

  updateQueueItem(item.id, { status: 'syncing' })
  try {
    const handler = apiHandlers[item.type]
    if (!handler) {
      throw new Error(`未知业务类型: ${item.type}`)
    }
    await handler(item.payload, item.idempotencyKey)
    removeOfflineQueueItem(item.id)
    ElMessage.success(`[${item.title}] 同步成功！`)
  } catch (err) {
    updateQueueItem(item.id, {
      status: 'failed',
      retryCount: (item.retryCount || 0) + 1,
      lastError: apiErrorMessage(err)
    })
    ElMessage.error(`同步失败: ${apiErrorMessage(err)}`)
  }
}

function removeItem(id) {
  removeOfflineQueueItem(id)
  ElMessage.info('已移除该条记录')
}

function confirmClearAll() {
  ElMessageBox.confirm('确定要清空所有未同步的离线记录吗？此操作不可恢复。', '警告', {
    confirmButtonText: '确定清空',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    clearOfflineQueue()
    ElMessage.success('已清空离线队列')
  }).catch(() => {})
}
</script>

<style scoped lang="scss">
.pda-queue-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sync-summary-card {
  background: #ffffff;
  border-radius: 14px;
  padding: 16px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
  display: flex;
  flex-direction: column;
  gap: 14px;

  .summary-info {
    .queue-status-text {
      .queue-count {
        font-size: 18px;
        font-weight: 700;
        color: #1f2329;
      }

      .net-status {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 12px;
        margin-top: 4px;

        .dot {
          width: 8px;
          height: 8px;
          border-radius: 50%;
        }

        &.online {
          color: #52c41a;
          .dot {
            background: #52c41a;
          }
        }

        &.offline {
          color: #ff4d4f;
          .dot {
            background: #ff4d4f;
          }
        }
      }
    }
  }

  .sync-all-btn {
    width: 100%;
    height: 48px;
    font-size: 16px;
    font-weight: 700;
    border-radius: 10px;
    background: linear-gradient(135deg, #1890ff, #096dd9);
    border: none;
    box-shadow: 0 4px 12px rgba(24, 144, 255, 0.3);
  }
}

.queue-list-container {
  .list-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 4px;
    margin-bottom: 8px;

    .list-title {
      font-size: 14px;
      font-weight: 600;
      color: #1f2329;
    }
  }

  .empty-queue-card {
    background: #ffffff;
    border-radius: 14px;
    padding: 36px 16px;
    text-align: center;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);

    .empty-title {
      font-size: 15px;
      font-weight: 600;
      color: #1f2329;
      margin: 12px 0 4px;
    }

    .empty-desc {
      font-size: 12px;
      color: #8c8c8c;
    }
  }

  .queue-items-list {
    display: flex;
    flex-direction: column;
    gap: 10px;

    .queue-item-card {
      background: #ffffff;
      border-radius: 12px;
      padding: 14px;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
      border-left: 4px solid #1890ff;

      &.failed {
        border-left-color: #ff4d4f;
      }

      &.syncing {
        border-left-color: #faad14;
      }

      .item-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        border-bottom: 1px dashed #f0f0f0;
        padding-bottom: 8px;

        .type-and-title {
          display: flex;
          align-items: center;
          gap: 6px;

          .type-tag {
            font-size: 11px;
            font-weight: 700;
            padding: 2px 6px;
            border-radius: 4px;

            &.inbound {
              background: #e6f7ff;
              color: #1890ff;
            }
            &.outbound {
              background: #f6ffed;
              color: #52c41a;
            }
            &.stock_check {
              background: #fff7e6;
              color: #fa8c16;
            }
          }

          .item-title {
            font-size: 13px;
            font-weight: 600;
            color: #1f2329;
          }
        }

        .status-tag {
          font-size: 11px;
          font-weight: 600;

          &.pending {
            color: #faad14;
          }
          &.syncing {
            color: #1890ff;
          }
          &.failed {
            color: #ff4d4f;
          }
        }
      }

      .item-details {
        padding: 8px 0;
        font-size: 12px;
        color: #595959;
        display: flex;
        flex-direction: column;
        gap: 3px;

        .detail-row {
          display: flex;
          gap: 4px;
          .k {
            color: #8c8c8c;
          }
          .v {
            color: #262626;
            font-weight: 500;
          }
        }

        .error-tip {
          color: #ff4d4f;
          font-size: 11px;
          background: #fff2f0;
          padding: 4px 6px;
          border-radius: 4px;
          margin-top: 4px;
        }
      }

      .item-actions {
        display: flex;
        justify-content: flex-end;
        gap: 12px;
        border-top: 1px solid #fafafa;
        padding-top: 6px;
      }
    }
  }
}
</style>
