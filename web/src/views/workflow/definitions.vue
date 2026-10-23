<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<!-- 流程定义管理：按编码查看各版本、用设计器新建版本、部署。权限 workflow:definition。 -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">流程定义</span>
      <el-button type="primary" @click="openCreate(null)">
        <el-icon><Plus /></el-icon>
        新建流程
      </el-button>
    </div>

    <el-alert
      type="info"
      :closable="false"
      show-icon
      class="tip"
      title="当前版本只支持按顺序流转的审批节点（可设会签/或签和金额条件）；抄送、并行分支、条件分支节点后端尚未实现。保存会生成新的草稿版本，部署后才生效，运行中的流程继续按原版本流转。"
    />

    <el-card>
      <el-form :inline="true" @submit.prevent>
        <el-form-item label="流程编码">
          <el-input v-model="filterCode" placeholder="如 purchase_order" clearable @keyup.enter="load" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="load">查询</el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="loading" :data="list" style="width: 100%" empty-text="暂无流程定义">
        <el-table-column label="流程编码" prop="code" min-width="140" />
        <el-table-column label="名称" prop="name" min-width="160" />
        <el-table-column label="版本" width="80">
          <template #default="{ row }">v{{ row.version }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="DEF_STATUS[row.status]?.type || 'info'" size="small">{{ DEF_STATUS[row.status]?.text || row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="审批节点" min-width="260">
          <template #default="{ row }">{{ summarize(row.definition) }}</template>
        </el-table-column>
        <el-table-column label="创建时间" width="150">
          <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="210" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="openView(row)">查看</el-button>
            <el-button link type="primary" size="small" @click="openCreate(row)">基于此建新版本</el-button>
            <el-button v-if="row.status !== 1" link type="success" size="small" @click="handleDeploy(row)">部署</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新建版本 -->
    <el-dialog v-model="editVisible" :title="editTitle" width="760px" top="5vh" destroy-on-close>
      <el-form label-width="90px" @submit.prevent>
        <el-form-item label="流程编码" required>
          <el-input v-model="form.code" :disabled="!!baseRow" placeholder="即业务类型，采购订单为 purchase_order" />
        </el-form-item>
        <el-form-item label="流程名称" required>
          <el-input v-model="form.name" maxlength="128" placeholder="如：采购订单审批" />
        </el-form-item>
      </el-form>
      <el-alert v-if="designerError" type="warning" :closable="false" show-icon :title="designerError" class="tip" />
      <FlowDesigner ref="designerRef" v-model="form.definition" />
      <el-collapse class="json-preview">
        <el-collapse-item title="查看将提交的定义 JSON" name="json">
          <pre>{{ JSON.stringify(form.definition, null, 2) }}</pre>
        </el-collapse-item>
      </el-collapse>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">保存为草稿版本</el-button>
      </template>
    </el-dialog>

    <!-- 查看 -->
    <el-dialog v-model="viewVisible" :title="viewRow ? `${viewRow.name} v${viewRow.version}` : ''" width="640px">
      <el-steps v-if="viewNodes.length" direction="vertical" :active="viewNodes.length" class="view-steps">
        <el-step v-for="n in viewNodes" :key="n.code" :title="n.name || n.code" :description="describeNode(n)" />
      </el-steps>
      <el-collapse>
        <el-collapse-item title="原始定义 JSON" name="json">
          <pre>{{ JSON.stringify(viewRow?.definition, null, 2) }}</pre>
        </el-collapse-item>
      </el-collapse>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listDefinitions, createDefinition, deployDefinition } from '@/api/workflow'
import { ASSIGNEE_TYPES, fmtTime, workflowErrorText } from '@/utils/workflow'
import FlowDesigner from '@/components/workflow/FlowDesigner.vue'

const DEF_STATUS = {
  0: { text: '草稿', type: 'info' },
  1: { text: '已部署', type: 'success' },
  2: { text: '已停用', type: 'warning' }
}

const loading = ref(false)
const list = ref([])
const filterCode = ref('')

const editVisible = ref(false)
const saving = ref(false)
const baseRow = ref(null)
const designerRef = ref(null)
const designerError = ref('')
const form = reactive({ code: '', name: '', definition: null })

const viewVisible = ref(false)
const viewRow = ref(null)

const editTitle = computed(() => (baseRow.value ? `新建版本：${baseRow.value.code}（基于 v${baseRow.value.version}）` : '新建流程'))
const viewNodes = computed(() => parseDef(viewRow.value?.definition).nodes || [])

function parseDef(def) {
  if (!def) return {}
  if (typeof def === 'string') {
    try {
      return JSON.parse(def)
    } catch (e) {
      return {}
    }
  }
  return def
}

function assigneeLabel(a) {
  if (!a) return '未配置审批人'
  const t = ASSIGNEE_TYPES.find((x) => x.value === a.type)?.label || a.type
  if (a.value === undefined || a.value === null || a.value === '') return t
  return `${t}：${Array.isArray(a.value) ? a.value.join(',') : a.value}`
}

function describeNode(n) {
  if (n.type === 'start') return '发起'
  if (n.type === 'end') return '结束'
  if (n.type !== 'approval') return `不支持的节点类型：${n.type}`
  const parts = [assigneeLabel(n.assignee), n.sign_mode === 'all' ? '会签' : '或签']
  if (n.condition) parts.push(`条件：${n.condition.field} ${n.condition.op} ${n.condition.value}`)
  return parts.join(' · ')
}

function summarize(def) {
  const nodes = parseDef(def).nodes || []
  const approvals = nodes.filter((n) => n.type === 'approval')
  if (!approvals.length) return '-'
  return approvals.map((n) => n.name || n.code).join(' → ')
}

async function load() {
  loading.value = true
  try {
    const res = await listDefinitions({ code: filterCode.value.trim() || undefined })
    list.value = Array.isArray(res.data) ? res.data : res.data?.list || []
  } catch (e) {
    list.value = []
  } finally {
    loading.value = false
  }
}

async function openCreate(row) {
  baseRow.value = row
  designerError.value = ''
  form.code = row?.code || ''
  form.name = row?.name || ''
  form.definition = null
  editVisible.value = true
  await nextTick()
  if (row) {
    try {
      designerRef.value?.load(parseDef(row.definition))
    } catch (e) {
      designerError.value = `${e.message}。已改为空白设计，请重新配置。`
      designerRef.value?.load(null)
    }
  } else {
    designerRef.value?.load(null)
  }
}

function openView(row) {
  viewRow.value = row
  viewVisible.value = true
}

async function handleSave() {
  if (!/^[A-Za-z][A-Za-z0-9_]{0,63}$/.test(form.code.trim())) {
    ElMessage.warning('流程编码须以字母开头，只含字母、数字、下划线')
    return
  }
  if (!form.name.trim()) {
    ElMessage.warning('请填写流程名称')
    return
  }
  const errs = designerRef.value?.validate() || []
  if (errs.length) {
    ElMessage.warning(errs[0])
    return
  }
  saving.value = true
  try {
    const res = await createDefinition({
      code: form.code.trim(),
      name: form.name.trim(),
      definition: designerRef.value.toDefinition()
    })
    ElMessage.success(`已保存为草稿 v${res.data?.version ?? ''}，部署后生效`)
    editVisible.value = false
    filterCode.value = form.code.trim()
    load()
  } catch (err) {
    if (err?.bizCode === 80010) ElMessage.warning(workflowErrorText(err))
  } finally {
    saving.value = false
  }
}

async function handleDeploy(row) {
  try {
    await ElMessageBox.confirm(
      `部署 ${row.code} v${row.version} 后，新提交的单据将按此版本审批，同编码的其他已部署版本会自动停用；运行中的流程不受影响。`,
      '确认部署',
      { type: 'warning', confirmButtonText: '部署', cancelButtonText: '取消' }
    )
  } catch (e) {
    return
  }
  try {
    await deployDefinition(row.id)
    ElMessage.success('部署成功')
    load()
  } catch (err) {
    // 拦截器已提示
  }
}

onMounted(load)
</script>

<style scoped>
.tip {
  margin-bottom: 16px;
}
.json-preview {
  margin-top: 16px;
}
pre {
  background: #f5f7fa;
  padding: 10px;
  border-radius: 4px;
  font-size: 12px;
  max-height: 300px;
  overflow: auto;
}
.view-steps {
  margin-bottom: 16px;
}
</style>
