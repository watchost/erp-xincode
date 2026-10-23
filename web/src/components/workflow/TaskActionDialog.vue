<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<!-- 审批动作弹窗：通过 / 驳回 / 转办 / 加签。驳回意见必填（后端 80008）。 -->
<template>
  <el-dialog
    :model-value="visible"
    :title="title"
    :width="isMobile ? '92%' : '480px'"
    append-to-body
    @update:model-value="(v) => emit('update:visible', v)"
    @closed="reset"
  >
    <div v-if="task" class="task-brief">
      <div><span class="k">单据：</span>{{ bizTypeText(task.biz_type) }} {{ task.biz_no }}</div>
      <div><span class="k">节点：</span>{{ task.node_name || task.node_code }}（{{ signModeText(task.sign_mode) }}）</div>
    </div>

    <el-alert
      v-if="action === 'add_sign'"
      type="info"
      :closable="false"
      show-icon
      title="加签后，本节点改为会签：你和加签人都通过后才会进入下一级。"
      class="dialog-alert"
    />

    <el-form label-position="top" @submit.prevent>
      <el-form-item v-if="needTarget" :label="action === 'transfer' ? '转办给' : '加签给'" required>
        <UserPicker v-model="targetUserId" />
      </el-form-item>
      <el-form-item :label="action === 'reject' ? '驳回意见（必填）' : '审批意见（选填）'" :required="action === 'reject'">
        <el-input
          v-model="comment"
          type="textarea"
          :rows="3"
          maxlength="1000"
          show-word-limit
          :placeholder="action === 'reject' ? '请说明驳回原因，发起人会看到这条意见' : '可选'"
        />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="emit('update:visible', false)">取消</el-button>
      <el-button :type="confirmType" :loading="submitting" @click="submit">{{ confirmText }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { approveTask, rejectTask, transferTask, addSignTask } from '@/api/workflow'
import { BIZ_TYPE, SIGN_MODE, workflowErrorText } from '@/utils/workflow'
import { useIsMobile } from '@/composables/useLayout'
import UserPicker from './UserPicker.vue'

const props = defineProps({
  visible: { type: Boolean, default: false },
  action: { type: String, default: 'approve' }, // approve | reject | transfer | add_sign
  task: { type: Object, default: null } // 至少包含 task_id（或 id）、biz_type、biz_no、node_name、sign_mode
})
const emit = defineEmits(['update:visible', 'done'])

const isMobile = useIsMobile()
const comment = ref('')
const targetUserId = ref(null)
const submitting = ref(false)

const META = {
  approve: { title: '审批通过', text: '通过', type: 'success' },
  reject: { title: '驳回', text: '驳回', type: 'danger' },
  transfer: { title: '转办', text: '转办', type: 'primary' },
  add_sign: { title: '加签', text: '加签', type: 'primary' }
}

const title = computed(() => META[props.action]?.title || '审批')
const confirmText = computed(() => META[props.action]?.text || '确定')
const confirmType = computed(() => META[props.action]?.type || 'primary')
const needTarget = computed(() => props.action === 'transfer' || props.action === 'add_sign')

function bizTypeText(t) {
  return BIZ_TYPE[t] || t
}
function signModeText(m) {
  return SIGN_MODE[m] || m || '或签'
}

function reset() {
  comment.value = ''
  targetUserId.value = null
  submitting.value = false
}

async function submit() {
  if (submitting.value || !props.task) return
  const taskId = props.task.task_id ?? props.task.id
  const text = comment.value.trim()
  if (props.action === 'reject' && !text) {
    ElMessage.warning('驳回时必须填写审批意见')
    return
  }
  if (needTarget.value && !targetUserId.value) {
    ElMessage.warning('请选择目标用户')
    return
  }

  submitting.value = true
  try {
    if (props.action === 'approve') await approveTask(taskId, text)
    else if (props.action === 'reject') await rejectTask(taskId, text)
    else if (props.action === 'transfer') await transferTask(taskId, targetUserId.value, text)
    else if (props.action === 'add_sign') await addSignTask(taskId, targetUserId.value, text)
    ElMessage.success(`${confirmText.value}成功`)
    emit('update:visible', false)
    emit('done', { action: props.action, taskId })
  } catch (err) {
    // 拦截器已弹出后端原文；这几类错误补充可操作的说明
    if ([80005, 80006, 80009, 80012].includes(err?.bizCode)) {
      ElMessage.warning(workflowErrorText(err))
    }
    if (err?.bizCode === 80006 || err?.bizCode === 80005) {
      emit('update:visible', false)
      emit('done', { action: 'stale', taskId })
    }
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.task-brief {
  background: #f5f7fa;
  border-radius: 6px;
  padding: 10px 12px;
  margin-bottom: 12px;
  font-size: 13px;
  line-height: 1.8;
}
.task-brief .k {
  color: #909399;
}
.dialog-alert {
  margin-bottom: 12px;
}
</style>
