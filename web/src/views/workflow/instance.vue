<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<!-- 审批实例详情：基本信息、当前节点、我的待办操作、审批历史时间线、发起人撤销。 -->
<template>
  <div class="page-container wf-instance" :class="{ mobile: isMobile }">
    <div v-if="!inPda" class="page-header">
      <span class="page-title">审批详情</span>
      <el-button @click="router.back()">返回</el-button>
    </div>

    <div v-loading="loading">
      <el-result v-if="loadError" icon="warning" :title="loadError">
        <template #extra>
          <el-button @click="load">重试</el-button>
        </template>
      </el-result>

      <template v-else-if="inst">
        <el-card class="block" :body-style="{ padding: isMobile ? '12px' : '20px' }">
          <div class="head">
            <div>
              <div class="biz-type">{{ bizTypeText(inst.biz_type) }}</div>
              <div class="biz-no">
                <el-link v-if="bizRoute" type="primary" :underline="false" @click="router.push(bizRoute)">{{ inst.biz_no }}</el-link>
                <span v-else>{{ inst.biz_no }}</span>
              </div>
            </div>
            <el-tag :type="instStatus(inst.status).type" size="large">{{ instStatus(inst.status).text }}</el-tag>
          </div>
          <el-descriptions :column="isMobile ? 1 : 3" size="small" border class="desc">
            <el-descriptions-item label="流程号">{{ inst.instance_no }}</el-descriptions-item>
            <el-descriptions-item label="发起人 ID">{{ inst.starter_id }}</el-descriptions-item>
            <el-descriptions-item label="当前节点">{{ currentNodeText }}</el-descriptions-item>
            <el-descriptions-item label="发起时间">{{ fmtTime(inst.created_at) }}</el-descriptions-item>
            <el-descriptions-item label="结束时间">{{ fmtTime(inst.finished_at) }}</el-descriptions-item>
            <el-descriptions-item v-if="amountText" label="单据金额">{{ amountText }}</el-descriptions-item>
          </el-descriptions>

          <div v-if="myTodoTask || canCancel" class="actions">
            <template v-if="myTodoTask">
              <el-button type="success" :size="btnSize" @click="openAction('approve')">通过</el-button>
              <el-button type="danger" :size="btnSize" @click="openAction('reject')">驳回</el-button>
              <el-button :size="btnSize" @click="openAction('transfer')">转办</el-button>
              <el-button :size="btnSize" @click="openAction('add_sign')">加签</el-button>
            </template>
            <el-button v-if="canCancel" type="warning" plain :size="btnSize" @click="handleCancel">撤销流程</el-button>
          </div>
        </el-card>

        <el-card class="block" :body-style="{ padding: isMobile ? '12px' : '20px' }">
          <template #header><span class="card-title">当前待办人</span></template>
          <el-empty v-if="pendingTasks.length === 0" description="没有待处理的审批人" :image-size="60" />
          <div v-else class="pending-list">
            <el-tag v-for="t in pendingTasks" :key="t.id" :type="t.assignee_id === myId ? 'warning' : 'info'" class="pending-tag">
              {{ t.node_name || t.node_code }} · 用户 {{ t.assignee_id }}{{ t.assignee_id === myId ? '（我）' : '' }}
              · {{ t.sign_mode === 'all' ? '会签' : '或签' }}
            </el-tag>
          </div>
        </el-card>

        <el-card class="block" :body-style="{ padding: isMobile ? '12px' : '20px' }">
          <template #header><span class="card-title">审批历史</span></template>
          <el-empty v-if="timeline.length === 0" description="暂无审批记录" :image-size="60" />
          <el-timeline v-else>
            <el-timeline-item
              v-for="a in timeline"
              :key="a.id"
              :timestamp="fmtTime(a.created_at)"
              :type="actionColor(a.action)"
              placement="top"
            >
              <div class="tl-line">
                <strong>{{ actionText(a.action) }}</strong>
                <span class="tl-node">{{ nodeName(a.node_code) }}</span>
              </div>
              <div class="tl-meta">
                操作人：用户 {{ a.operator_id }}
                <template v-if="a.target_id">→ 用户 {{ a.target_id }}</template>
              </div>
              <div v-if="a.comment" class="tl-comment">“{{ a.comment }}”</div>
            </el-timeline-item>
          </el-timeline>
        </el-card>
      </template>
    </div>

    <TaskActionDialog v-model:visible="dialogVisible" :action="dialogAction" :task="dialogTask" @done="onActionDone" />
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getInstance, cancelInstance } from '@/api/workflow'
import { useUserStore } from '@/store/user'
import { INSTANCE_STATUS, BIZ_TYPE, ACTION_TEXT, fmtTime, bizLink, workflowErrorText } from '@/utils/workflow'
import { useIsMobile, useLayoutPaths } from '@/composables/useLayout'
import { notifyBadgeRefresh } from '@/utils/badgeBus'
import TaskActionDialog from '@/components/workflow/TaskActionDialog.vue'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const isMobile = useIsMobile()
const { inPda } = useLayoutPaths()

const loading = ref(false)
const loadError = ref('')
const inst = ref(null)
const tasks = ref([])
const approvals = ref([])

const dialogVisible = ref(false)
const dialogAction = ref('approve')

const btnSize = computed(() => (isMobile.value ? 'large' : 'default'))
const myId = computed(() => userStore.userInfo?.id)
const bizTypeText = (t) => BIZ_TYPE[t] || t
const instStatus = (s) => INSTANCE_STATUS[s] || { text: String(s ?? '-'), type: 'info' }
const actionText = (a) => ACTION_TEXT[a] || a
const bizRoute = computed(() => (inPda.value || !inst.value ? null : bizLink(inst.value.biz_type, inst.value.biz_no)))

const nodeNames = computed(() => {
  const m = {}
  tasks.value.forEach((t) => {
    if (t.node_name) m[t.node_code] = t.node_name
  })
  return m
})
const nodeName = (code) => nodeNames.value[code] || code || ''

const pendingTasks = computed(() => tasks.value.filter((t) => t.status === 1))
const myTodoTask = computed(() =>
  inst.value?.status === 1 ? pendingTasks.value.find((t) => t.assignee_id === myId.value) || null : null
)
const canCancel = computed(() => inst.value?.status === 1 && myId.value && inst.value.starter_id === myId.value)
const currentNodeText = computed(() => {
  if (!inst.value) return '-'
  if (inst.value.status !== 1) return '已结束'
  return nodeName(inst.value.current_node) || inst.value.current_node || '-'
})
const amountText = computed(() => {
  const v = inst.value?.variables
  if (!v || typeof v !== 'object' || v.amount === undefined) return ''
  return '¥' + v.amount
})
const timeline = computed(() => [...approvals.value].sort((a, b) => new Date(a.created_at) - new Date(b.created_at)))

const dialogTask = computed(() => {
  const t = myTodoTask.value
  if (!t || !inst.value) return null
  return { task_id: t.id, biz_type: inst.value.biz_type, biz_no: inst.value.biz_no, node_name: t.node_name, node_code: t.node_code, sign_mode: t.sign_mode }
})

function actionColor(a) {
  if (a === 'approve') return 'success'
  if (a === 'reject') return 'danger'
  if (a === 'cancel') return 'info'
  return 'primary'
}

async function load() {
  const no = route.params.no
  if (!no) return
  loading.value = true
  loadError.value = ''
  try {
    if (!userStore.userInfo?.id) await userStore.fetchUserInfo()
    const res = await getInstance(no)
    inst.value = res.data?.instance || null
    tasks.value = res.data?.tasks || []
    approvals.value = res.data?.approvals || []
    if (!inst.value) loadError.value = '流程实例不存在'
  } catch (err) {
    inst.value = null
    loadError.value = err?.bizCode === 80003 ? '流程实例不存在' : workflowErrorText(err)
  } finally {
    loading.value = false
  }
}

function openAction(action) {
  dialogAction.value = action
  dialogVisible.value = true
}

function onActionDone() {
  notifyBadgeRefresh()
  load()
}

async function handleCancel() {
  let comment = ''
  try {
    const r = await ElMessageBox.prompt('撤销后单据退回草稿，可修改后重新提交。可填写撤销原因：', '撤销流程', {
      confirmButtonText: '确定撤销',
      cancelButtonText: '取消',
      inputPlaceholder: '撤销原因（选填）',
      inputValidator: (v) => !v || v.length <= 1000 || '不能超过 1000 字'
    })
    comment = r.value || ''
  } catch (e) {
    return
  }
  try {
    await cancelInstance(inst.value.instance_no, comment)
    ElMessage.success('已撤销')
    notifyBadgeRefresh()
    load()
  } catch (err) {
    if (err?.bizCode === 80011) ElMessage.warning(workflowErrorText(err))
  }
}

watch(() => route.params.no, (v, old) => {
  if (v && v !== old) load()
})
onMounted(load)
</script>

<style scoped lang="scss">
.block {
  margin-bottom: 16px;
}
.head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 14px;
  .biz-type {
    font-size: 12px;
    color: #909399;
  }
  .biz-no {
    font-size: 20px;
    font-weight: 600;
    word-break: break-all;
  }
}
.actions {
  margin-top: 16px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  .el-button {
    margin: 0;
  }
}
.mobile .actions .el-button {
  flex: 1 1 40%;
}
.card-title {
  font-weight: 600;
}
.pending-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.pending-tag {
  height: auto;
  padding: 4px 8px;
  white-space: normal;
}
.tl-line {
  display: flex;
  gap: 8px;
  align-items: baseline;
  .tl-node {
    color: #606266;
    font-size: 13px;
  }
}
.tl-meta {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}
.tl-comment {
  margin-top: 6px;
  padding: 6px 10px;
  background: #f5f7fa;
  border-radius: 4px;
  color: #303133;
  font-size: 13px;
  word-break: break-all;
}
</style>
