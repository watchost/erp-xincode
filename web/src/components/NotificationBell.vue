<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<!--
  通知铃铛：显示站内信未读数，点击弹出最近 5 条未读。
  每 30 秒轮询一次未读数；页面切到后台时暂停；审批/已读操作后立即刷新。
  轮询失败不弹错误提示；接口不存在（404）时停止轮询并标注“暂不可用”。
-->
<template>
  <el-popover
    v-model:visible="open"
    placement="bottom-end"
    :width="popoverWidth"
    trigger="click"
    popper-class="notify-popover"
    @show="loadLatest"
  >
    <template #reference>
      <button class="bell-btn" :class="{ dark }" type="button" :aria-label="ariaLabel">
        <el-badge :value="total" :max="99" :hidden="total === 0" class="bell-badge">
          <el-icon :size="size"><Bell /></el-icon>
        </el-badge>
      </button>
    </template>

    <div class="notify-panel">
      <div class="panel-head">
        <span class="panel-title">通知</span>
        <el-button v-if="total > 0" link type="primary" size="small" @click="readAll">全部已读</el-button>
      </div>

      <el-alert
        v-if="messageApiUnavailable"
        type="warning"
        :closable="false"
        title="消息服务暂不可用（后端消息接口未开放），审批待办请到“我的审批”查看。"
      />

      <template v-else>
        <div v-if="byCategoryList.length" class="cat-row">
          <el-tag v-for="c in byCategoryList" :key="c.value" :type="c.type" size="small" effect="plain">
            {{ c.label }} {{ c.count }}
          </el-tag>
        </div>
        <div v-loading="loadingList" class="msg-list">
          <el-empty v-if="!loadingList && latest.length === 0" description="没有未读消息" :image-size="60" />
          <div v-for="m in latest" :key="m.id" class="msg-item" @click="openMessage(m)">
            <div class="msg-title">
              <el-tag :type="categoryMeta(m.category).type" size="small" effect="plain">{{ categoryMeta(m.category).label }}</el-tag>
              <span>{{ m.title }}</span>
            </div>
            <div class="msg-content">{{ m.content }}</div>
            <div class="msg-time">{{ fmtTime(m.created_at) }}</div>
          </div>
        </div>
      </template>

      <div class="panel-foot">
        <el-button link type="primary" @click="go(todoPath)">我的审批</el-button>
        <el-button link type="primary" @click="go(messagesPath)">查看全部消息</el-button>
      </div>
    </div>
  </el-popover>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { getUnreadCount, listMessages, readMessage, readAllMessages } from '@/api/message'
import { categoryMeta, messageApiUnavailable, markMessageApiState, MESSAGE_CATEGORIES } from '@/utils/message'
import { fmtTime } from '@/utils/workflow'
import { badgeRefreshTick, notifyBadgeRefresh } from '@/utils/badgeBus'
import { useIsMobile, useLayoutPaths } from '@/composables/useLayout'

defineProps({
  size: { type: Number, default: 20 },
  dark: { type: Boolean, default: false }
})

const POLL_MS = 30000

const router = useRouter()
const isMobile = useIsMobile()
const { todoPath, messagesPath, resolveMessageLink } = useLayoutPaths()

const open = ref(false)
const total = ref(0)
const byCategory = ref({})
const latest = ref([])
const loadingList = ref(false)
let timer = null

const popoverWidth = computed(() => (isMobile.value ? Math.min(window.innerWidth - 24, 360) : 360))
const ariaLabel = computed(() => (total.value > 0 ? `通知，${total.value} 条未读` : '通知'))
const byCategoryList = computed(() =>
  MESSAGE_CATEGORIES.map((c) => ({ ...c, count: byCategory.value?.[c.value] || 0 })).filter((c) => c.count > 0)
)

async function refreshCount() {
  if (messageApiUnavailable.value) return
  try {
    const res = await getUnreadCount()
    total.value = res.data?.total || 0
    byCategory.value = res.data?.by_category || {}
  } catch (err) {
    markMessageApiState(err)
    if (messageApiUnavailable.value) stopPolling()
  }
}

async function loadLatest() {
  if (messageApiUnavailable.value) return
  loadingList.value = true
  try {
    const res = await listMessages({ unread: 1, page: 1, page_size: 5 })
    latest.value = res.data?.list || []
  } catch (err) {
    markMessageApiState(err)
    latest.value = []
  } finally {
    loadingList.value = false
  }
}

async function openMessage(m) {
  open.value = false
  if (!m.is_read) {
    try {
      await readMessage(m.id)
    } catch (e) {
      // 已读失败不影响跳转
    }
    notifyBadgeRefresh()
  }
  const link = resolveMessageLink(m.link)
  if (link) router.push(link)
}

async function readAll() {
  try {
    await readAllMessages()
    latest.value = []
    notifyBadgeRefresh()
  } catch (e) {
    // 拦截器已提示
  }
}

function go(path) {
  open.value = false
  router.push(path)
}

function startPolling() {
  stopPolling()
  if (messageApiUnavailable.value) return
  timer = setInterval(() => {
    if (document.visibilityState === 'visible') refreshCount()
  }, POLL_MS)
}

function stopPolling() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

function onVisibility() {
  if (document.visibilityState === 'visible') refreshCount()
}

watch(badgeRefreshTick, () => {
  refreshCount()
  if (open.value) loadLatest()
})

onMounted(() => {
  refreshCount()
  startPolling()
  document.addEventListener('visibilitychange', onVisibility)
})

onBeforeUnmount(() => {
  stopPolling()
  document.removeEventListener('visibilitychange', onVisibility)
})
</script>

<style scoped lang="scss">
.bell-btn {
  background: none;
  border: none;
  padding: 6px;
  cursor: pointer;
  color: #303133;
  display: inline-flex;
  align-items: center;
  border-radius: 6px;
  &:hover {
    background: rgba(0, 0, 0, 0.05);
  }
  &.dark {
    color: #fff;
    &:hover {
      background: rgba(255, 255, 255, 0.15);
    }
  }
}
.bell-badge :deep(.el-badge__content) {
  z-index: 1;
}
.notify-panel {
  .panel-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
    .panel-title {
      font-weight: 600;
    }
  }
  .cat-row {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 8px;
  }
  .msg-list {
    max-height: 360px;
    overflow-y: auto;
    min-height: 60px;
  }
  .msg-item {
    padding: 8px 4px;
    border-bottom: 1px solid #f0f2f5;
    cursor: pointer;
    &:hover {
      background: #f5f7fa;
    }
    .msg-title {
      display: flex;
      gap: 6px;
      align-items: center;
      font-size: 13px;
      font-weight: 600;
      color: #303133;
    }
    .msg-content {
      font-size: 12px;
      color: #606266;
      margin-top: 4px;
      overflow: hidden;
      text-overflow: ellipsis;
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
    }
    .msg-time {
      font-size: 11px;
      color: #909399;
      margin-top: 2px;
    }
  }
  .panel-foot {
    display: flex;
    justify-content: space-between;
    margin-top: 8px;
  }
}
</style>
