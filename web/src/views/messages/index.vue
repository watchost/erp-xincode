<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<!-- 消息中心：站内信列表（按类别 / 未读筛选、已读、跳转）+ 外部渠道订阅设置。 -->
<template>
  <div class="page-container msg-center" :class="{ mobile: isMobile }">
    <div v-if="!inPda" class="page-header">
      <span class="page-title">消息中心</span>
    </div>

    <el-alert
      v-if="messageApiUnavailable"
      type="warning"
      :closable="false"
      show-icon
      class="block"
      title="消息服务暂不可用：后端消息接口未开放（请求返回 404）。审批待办请到“我的审批”查看。"
    />

    <el-card :body-style="{ padding: isMobile ? '10px' : '20px' }">
      <el-tabs v-model="tab">
        <el-tab-pane label="站内信" name="inbox" />
        <el-tab-pane label="外部通知设置" name="subs" />
      </el-tabs>

      <!-- 站内信 -->
      <template v-if="tab === 'inbox'">
        <div class="toolbar">
          <el-radio-group v-model="category" size="small" @change="reload">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button v-for="c in MESSAGE_CATEGORIES" :key="c.value" :value="c.value">{{ c.label }}</el-radio-button>
          </el-radio-group>
          <div class="toolbar-right">
            <el-checkbox v-model="unreadOnly" @change="reload">只看未读</el-checkbox>
            <el-button size="small" @click="readAll">全部已读</el-button>
          </div>
        </div>

        <div v-loading="loading" class="msg-list">
          <el-empty v-if="!loading && list.length === 0" description="暂无消息" :image-size="80" />
          <div v-for="m in list" :key="m.id" class="msg-row" :class="{ unread: !m.is_read }" @click="openMessage(m)">
            <span v-if="!m.is_read" class="dot" aria-label="未读" />
            <div class="msg-main">
              <div class="msg-title">
                <el-tag :type="categoryMeta(m.category).type" size="small" effect="plain">{{ categoryMeta(m.category).label }}</el-tag>
                <span>{{ m.title }}</span>
              </div>
              <div class="msg-content">{{ m.content }}</div>
              <div class="msg-meta">
                <span>{{ fmtTime(m.created_at) }}</span>
                <span v-if="m.biz_no">· {{ m.biz_no }}</span>
              </div>
            </div>
            <el-button v-if="!m.is_read" link size="small" @click.stop="markRead(m)">标为已读</el-button>
          </div>
        </div>

        <div class="pagination">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :total="total"
            :layout="isMobile ? 'prev, pager, next' : 'total, prev, pager, next'"
            :pager-count="isMobile ? 5 : 7"
            small
            @current-change="load"
          />
        </div>
      </template>

      <!-- 订阅设置 -->
      <template v-else>
        <el-alert
          type="info"
          :closable="false"
          show-icon
          class="block"
          title="站内信默认都会收到。这里设置的是额外推送到外部渠道（地址必须是 https://）。"
        />
        <div v-loading="subsLoading">
          <div v-for="c in MESSAGE_CATEGORIES" :key="c.value" class="sub-group">
            <div class="sub-group-title">{{ c.label }}</div>
            <div v-for="ch in CHANNELS" :key="ch.value" class="sub-row">
              <div class="sub-label">{{ ch.label }}</div>
              <el-input
                v-model="subForm[key(c.value, ch.value)].target"
                :placeholder="ch.placeholder"
                size="small"
                class="sub-input"
              />
              <el-switch v-model="subForm[key(c.value, ch.value)].enabled" size="small" />
              <el-button
                size="small"
                type="primary"
                plain
                :loading="savingKey === key(c.value, ch.value)"
                @click="saveSub(c.value, ch.value)"
              >保存</el-button>
              <div class="sub-note">{{ ch.note }}</div>
            </div>
          </div>
        </div>
      </template>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { listMessages, readMessage, readAllMessages, listSubscriptions, saveSubscription } from '@/api/message'
import { MESSAGE_CATEGORIES, CHANNELS, categoryMeta, messageApiUnavailable, markMessageApiState } from '@/utils/message'
import { fmtTime } from '@/utils/workflow'
import { notifyBadgeRefresh } from '@/utils/badgeBus'
import { useIsMobile, useLayoutPaths } from '@/composables/useLayout'

const router = useRouter()
const isMobile = useIsMobile()
const { inPda, resolveMessageLink } = useLayoutPaths()

const tab = ref('inbox')
const category = ref('')
const unreadOnly = ref(false)
const list = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const subsLoading = ref(false)
const savingKey = ref('')
const key = (cat, ch) => `${cat}|${ch}`
const subForm = reactive(
  Object.fromEntries(
    MESSAGE_CATEGORIES.flatMap((c) => CHANNELS.map((ch) => [key(c.value, ch.value), { target: '', enabled: false }]))
  )
)

async function load() {
  loading.value = true
  try {
    const res = await listMessages({
      category: category.value || undefined,
      unread: unreadOnly.value ? 1 : undefined,
      page: page.value,
      page_size: pageSize.value
    })
    list.value = res.data?.list || []
    total.value = res.data?.total || 0
  } catch (err) {
    markMessageApiState(err)
    list.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function reload() {
  page.value = 1
  load()
}

async function markRead(m) {
  try {
    await readMessage(m.id)
    m.is_read = true
    notifyBadgeRefresh()
  } catch (e) {
    // 拦截器已提示
  }
}

async function openMessage(m) {
  if (!m.is_read) await markRead(m)
  const link = resolveMessageLink(m.link)
  if (link) router.push(link)
}

async function readAll() {
  try {
    const res = await readAllMessages()
    ElMessage.success(`已将 ${res.data?.updated ?? 0} 条标为已读`)
    notifyBadgeRefresh()
    load()
  } catch (e) {
    // 拦截器已提示
  }
}

async function loadSubs() {
  subsLoading.value = true
  try {
    const res = await listSubscriptions()
    ;(res.data || []).forEach((s) => {
      const k = key(s.category, s.channel)
      if (subForm[k]) {
        subForm[k].target = s.target || ''
        subForm[k].enabled = !!s.enabled
      }
    })
  } catch (err) {
    markMessageApiState(err)
  } finally {
    subsLoading.value = false
  }
}

function validTarget(channel, target) {
  if (channel === 'email') return /^[^\s@]+@[^\s@]+$/.test(target)
  return /^https:\/\/\S+$/.test(target)
}

async function saveSub(cat, channel) {
  const k = key(cat, channel)
  const item = subForm[k]
  const target = item.target.trim()
  if (item.enabled && !validTarget(channel, target)) {
    ElMessage.warning(channel === 'email' ? '请填写有效的邮箱地址' : '地址必须以 https:// 开头')
    return
  }
  savingKey.value = k
  try {
    await saveSubscription({ category: cat, channel, target, enabled: item.enabled })
    ElMessage.success('已保存')
  } catch (e) {
    // 拦截器已提示（如 82004 订阅目标不合法）
  } finally {
    savingKey.value = ''
  }
}

watch(tab, (t) => {
  if (t === 'subs') loadSubs()
})

onMounted(load)
</script>

<style scoped lang="scss">
.block {
  margin-bottom: 16px;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  .toolbar-right {
    display: flex;
    align-items: center;
    gap: 12px;
  }
}
.msg-list {
  min-height: 120px;
}
.msg-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 12px 8px;
  border-bottom: 1px solid #f0f2f5;
  cursor: pointer;
  &:hover {
    background: #f5f7fa;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #f56c6c;
    margin-top: 6px;
    flex-shrink: 0;
  }
  .msg-main {
    flex: 1;
    min-width: 0;
  }
  .msg-title {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 14px;
    color: #606266;
  }
  &.unread .msg-title {
    color: #303133;
    font-weight: 600;
  }
  .msg-content {
    font-size: 13px;
    color: #606266;
    margin-top: 4px;
    word-break: break-all;
  }
  .msg-meta {
    font-size: 12px;
    color: #909399;
    margin-top: 4px;
  }
}
.pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
.mobile .pagination {
  justify-content: center;
}
.sub-group {
  margin-bottom: 18px;
  .sub-group-title {
    font-weight: 600;
    margin-bottom: 8px;
  }
}
.sub-row {
  display: grid;
  grid-template-columns: 130px 1fr auto auto;
  align-items: center;
  gap: 4px 10px;
  margin-bottom: 10px;
  .sub-note {
    grid-column: 2 / -1;
    font-size: 12px;
    color: #909399;
  }
}
.mobile .sub-row {
  grid-template-columns: 1fr auto auto;
  .sub-label {
    grid-column: 1 / -1;
  }
  .sub-note {
    grid-column: 1 / -1;
  }
}
</style>
