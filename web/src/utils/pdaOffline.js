// Copyright 2026 zhouhouping. All Rights Reserved.

import { ref } from 'vue'
import { playSuccessSound, playErrorSound } from './sound'

const STORAGE_KEY = 'erp_pda_offline_queue'

export const offlineQueue = ref(loadQueue())
export const pendingCount = ref(getPendingCount())

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

export function addOfflineQueueItem(type, title, payload) {
  const item = {
    id: 'queue_' + Date.now() + '_' + Math.random().toString(36).substring(2, 8),
    type,
    title,
    payload,
    idempotencyKey: 'idemp_' + Date.now() + '_' + Math.random().toString(36).substring(2, 10),
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
        lastError: err.message || '网络或接口同步失败'
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
