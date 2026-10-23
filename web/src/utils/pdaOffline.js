// Copyright 2026 zhouhouping. All Rights Reserved.

import { ref } from 'vue'
import { playSuccessSound, playErrorSound } from './sound'

const STORAGE_KEY = 'erp_pda_offline_queue'

export const offlineQueue = ref(recoverInterrupted(loadQueue()))
export const pendingCount = ref(getPendingCount())

// 同步过程中页面被关闭会留下 syncing 状态，重新打开时恢复为待同步
function recoverInterrupted(queue) {
  if (!queue.some((i) => i.status === 'syncing')) return queue
  const fixed = queue.map((i) => (i.status === 'syncing' ? { ...i, status: 'pending' } : i))
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(fixed))
  } catch (e) {
    // ignore
  }
  return fixed
}

function loadQueue() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? JSON.parse(raw) : []
  } catch (e) {
    console.error('[OfflineQueue] load failed:', e)
    return []
  }
}

function saveQueue(queue) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(queue))
    offlineQueue.value = queue
    pendingCount.value = getPendingCount()
  } catch (e) {
    console.error('[OfflineQueue] save failed:', e)
  }
}

export function getPendingCount() {
  const q = loadQueue()
  return q.filter((item) => item.status === 'pending' || item.status === 'failed').length
}

export function newIdempotencyKey() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return 'pda-' + crypto.randomUUID()
  }
  return 'pda-' + Date.now() + '-' + Math.random().toString(36).substring(2, 12)
}

// 仅"请求没有到达服务器/没有拿到响应"才算离线，可以转入离线队列重发。
// 业务错误（库存不足、物料不存在等，HTTP 4xx/5xx 有响应）不能入队，否则重发也会失败。
// 超时也视为未确认，入队重发时复用同一个幂等键，服务端不会重复入账。
export function isNetworkError(err) {
  if (typeof navigator !== 'undefined' && navigator.onLine === false) return true
  if (!err) return false
  if (err.response) {
    // Service Worker 离线兜底响应
    return err.response.status === 503 && err.response.data?.offline === true
  }
  return err.isAxiosError === true || err.code === 'ECONNABORTED' || err.code === 'ERR_NETWORK'
}

export function addOfflineQueueItem(type, title, payload, idempotencyKey) {
  const item = {
    id: 'queue_' + Date.now() + '_' + Math.random().toString(36).substring(2, 8),
    type,
    title,
    payload,
    idempotencyKey: idempotencyKey || newIdempotencyKey(),
    createdAt: new Date().toLocaleString(),
    status: 'pending',
    retryCount: 0,
    lastError: null
  }
  const q = loadQueue()
  q.unshift(item)
  saveQueue(q)
  return item
}

export function removeOfflineQueueItem(id) {
  const q = loadQueue().filter((item) => item.id !== id)
  saveQueue(q)
}

export function clearOfflineQueue() {
  saveQueue([])
}

export function updateQueueItem(id, updates) {
  const q = loadQueue().map((item) => {
    if (item.id === id) {
      return { ...item, ...updates }
    }
    return item
  })
  saveQueue(q)
}

// 批量同步离线队列
export async function syncOfflineQueue(apiHandlers) {
  const q = loadQueue()
  const pendingItems = q.filter((item) => item.status === 'pending' || item.status === 'failed')
  if (pendingItems.length === 0) {
    return { total: 0, success: 0, failed: 0 }
  }

  let successCount = 0
  let failedCount = 0

  for (const item of pendingItems) {
    updateQueueItem(item.id, { status: 'syncing' })
    try {
      const handler = apiHandlers[item.type]
      if (!handler) {
        throw new Error(`未找到处理类型 [${item.type}] 的同步接口`)
      }

      await handler(item.payload, item.idempotencyKey)
      // 成功后移除该条记录
      removeOfflineQueueItem(item.id)
      successCount++
    } catch (err) {
      failedCount++
      updateQueueItem(item.id, {
        status: 'failed',
        retryCount: (item.retryCount || 0) + 1,
        lastError: err?.response?.data?.msg || err?.message || '网络或接口同步失败'
      })
    }
  }

  if (failedCount === 0) {
    playSuccessSound()
  } else {
    playErrorSound()
  }

  return {
    total: pendingItems.length,
    success: successCount,
    failed: failedCount
  }
}
