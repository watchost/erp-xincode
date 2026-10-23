<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<!--
  简化版流程设计器：开始 → 若干审批节点（按顺序流转）→ 结束。
  只提供后端已支持的能力：审批节点、5 种审批人规则、会签/或签、金额条件。
  抄送、并行分支、条件分支节点后端尚未实现，这里不提供。
-->
<template>
  <div class="flow-designer">
    <div class="flow-node fixed start">开始（提交人发起）</div>

    <template v-for="(node, idx) in nodes" :key="node.uid">
      <div class="flow-arrow">↓</div>
      <div class="flow-node approval" :class="{ invalid: errorsOf(node).length > 0 }">
        <div class="node-head">
          <span class="node-index">第 {{ idx + 1 }} 级</span>
          <div class="node-tools">
            <el-button link size="small" :disabled="idx === 0" @click="move(idx, -1)">上移</el-button>
            <el-button link size="small" :disabled="idx === nodes.length - 1" @click="move(idx, 1)">下移</el-button>
            <el-button link size="small" type="danger" :disabled="nodes.length <= 1" @click="remove(idx)">删除</el-button>
          </div>
        </div>

        <el-form label-position="top" size="small" class="node-form">
          <div class="grid">
            <el-form-item label="节点名称" required>
              <el-input v-model="node.name" maxlength="64" placeholder="如：部门负责人审批" />
            </el-form-item>
            <el-form-item label="节点编码" required>
              <el-input v-model="node.code" maxlength="64" placeholder="字母/数字/下划线，如 leader" />
            </el-form-item>
            <el-form-item label="审批人规则" required>
              <el-select v-model="node.assigneeType" style="width: 100%">
                <el-option v-for="a in ASSIGNEE_TYPES" :key="a.value" :label="a.label" :value="a.value" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="needValue(node)" :label="node.assigneeType === 'user' ? '用户 ID' : '角色编码'" required>
              <el-input
                v-model="node.assigneeValue"
                :placeholder="node.assigneeType === 'user' ? '如 3 或 3,5,8' : '如 manager'"
              />
            </el-form-item>
            <el-form-item label="多人审批方式">
              <el-radio-group v-model="node.signMode">
                <el-radio value="any">或签</el-radio>
                <el-radio value="all">会签</el-radio>
              </el-radio-group>
            </el-form-item>
          </div>
          <div class="assignee-hint">{{ assigneeHint(node) }}</div>

          <el-form-item class="cond-item">
            <el-checkbox v-model="node.condEnabled">仅在满足条件时经过本级（不满足则跳过）</el-checkbox>
          </el-form-item>
          <div v-if="node.condEnabled" class="cond-row">
            <el-select v-model="node.condField" style="width: 170px">
              <el-option v-for="f in CONDITION_FIELDS" :key="f.value" :label="f.label" :value="f.value" />
            </el-select>
            <el-select v-model="node.condOp" style="width: 80px">
              <el-option v-for="op in CONDITION_OPS" :key="op" :label="op" :value="op" />
            </el-select>
            <el-input v-model="node.condValue" placeholder="金额，如 10000" style="width: 140px" />
          </div>
        </el-form>

        <ul v-if="errorsOf(node).length" class="node-errors">
          <li v-for="e in errorsOf(node)" :key="e">{{ e }}</li>
        </ul>
      </div>
    </template>

    <div class="flow-arrow">↓</div>
    <el-button class="add-btn" @click="add">+ 添加审批节点</el-button>
    <div class="flow-arrow">↓</div>
    <div class="flow-node fixed end">结束（全部通过后执行单据审批）</div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { ASSIGNEE_TYPES, CONDITION_OPS, CONDITION_FIELDS } from '@/utils/workflow'

const props = defineProps({
  // 定义 JSON：{ nodes: [...] }
  modelValue: { type: Object, default: null }
})
const emit = defineEmits(['update:modelValue'])

let uidSeq = 0
const nodes = ref([])

function blankNode(i) {
  return {
    uid: ++uidSeq,
    code: `node_${i}`,
    name: `第${i}级审批`,
    assigneeType: 'dept_leader',
    assigneeValue: '',
    signMode: 'any',
    condEnabled: false,
    condField: 'amount',
    condOp: '>=',
    condValue: ''
  }
}

// 外部定义 JSON → 设计器节点。含不支持的节点类型时抛错，由调用方提示。
function fromDefinition(def) {
  const list = Array.isArray(def?.nodes) ? def.nodes : []
  const unsupported = list.filter((n) => !['start', 'approval', 'end'].includes(n.type))
  if (unsupported.length) {
    throw new Error('含有设计器不支持的节点类型：' + unsupported.map((n) => n.type).join(', '))
  }
  return list
    .filter((n) => n.type === 'approval')
    .map((n) => {
      const v = n.assignee?.value
      return {
        uid: ++uidSeq,
        code: n.code || '',
        name: n.name || '',
        assigneeType: n.assignee?.type || 'dept_leader',
        assigneeValue: Array.isArray(v) ? v.join(',') : v === undefined || v === null ? '' : String(v),
        signMode: n.sign_mode === 'all' ? 'all' : 'any',
        condEnabled: !!n.condition,
        condField: n.condition?.field || 'amount',
        condOp: n.condition?.op || '>=',
        condValue: n.condition?.value === undefined ? '' : String(n.condition.value)
      }
    })
}

function needValue(node) {
  return ASSIGNEE_TYPES.find((a) => a.value === node.assigneeType)?.needValue
}

function assigneeHint(node) {
  return ASSIGNEE_TYPES.find((a) => a.value === node.assigneeType)?.hint || ''
}

function parseUserIds(raw) {
  const parts = String(raw || '')
    .split(/[,，\s]+/)
    .filter(Boolean)
  const ids = parts.map((p) => Number(p))
  if (ids.length === 0 || ids.some((n) => !Number.isInteger(n) || n <= 0)) return null
  return ids
}

function errorsOf(node) {
  const errs = []
  if (!node.name.trim()) errs.push('请填写节点名称')
  if (!/^[A-Za-z][A-Za-z0-9_]{0,63}$/.test(node.code.trim())) errs.push('节点编码须以字母开头，只含字母、数字、下划线')
  if (['start', 'end'].includes(node.code.trim())) errs.push('节点编码不能使用 start / end')
  const dupes = nodes.value.filter((n) => n.code.trim() === node.code.trim())
  if (dupes.length > 1) errs.push('节点编码重复')
  if (node.assigneeType === 'user' && !parseUserIds(node.assigneeValue)) errs.push('用户 ID 必须是正整数，多个用逗号分隔')
  if (node.assigneeType === 'role' && !node.assigneeValue.trim()) errs.push('请填写角色编码')
  if (node.condEnabled && !/^-?\d+(\.\d+)?$/.test(String(node.condValue).trim())) errs.push('条件金额必须是数字')
  return errs
}

function validate() {
  if (nodes.value.length === 0) return ['至少需要一个审批节点']
  return nodes.value.flatMap((n, i) => errorsOf(n).map((e) => `第 ${i + 1} 级：${e}`))
}

function toDefinition() {
  const approvals = nodes.value.map((n) => {
    const node = {
      code: n.code.trim(),
      type: 'approval',
      name: n.name.trim(),
      assignee: { type: n.assigneeType },
      sign_mode: n.signMode
    }
    if (n.assigneeType === 'user') {
      const ids = parseUserIds(n.assigneeValue) || []
      node.assignee.value = ids.length === 1 ? ids[0] : ids
    } else if (n.assigneeType === 'role') {
      node.assignee.value = n.assigneeValue.trim()
    }
    if (n.condEnabled) {
      node.condition = { field: n.condField, op: n.condOp, value: String(n.condValue).trim() }
    }
    return node
  })
  return {
    nodes: [{ code: 'start', type: 'start', name: '开始' }, ...approvals, { code: 'end', type: 'end', name: '结束' }]
  }
}

function add() {
  nodes.value.push(blankNode(nodes.value.length + 1))
}
function remove(i) {
  nodes.value.splice(i, 1)
}
function move(i, d) {
  const j = i + d
  const arr = nodes.value
  ;[arr[i], arr[j]] = [arr[j], arr[i]]
}

function load(def) {
  const parsed = def ? fromDefinition(def) : []
  nodes.value = parsed.length ? parsed : [blankNode(1)]
}

watch(
  nodes,
  () => {
    emit('update:modelValue', toDefinition())
  },
  { deep: true }
)

// 初始化：外部传入的定义只在挂载时读取一次，之后以设计器为准
try {
  load(props.modelValue)
} catch (e) {
  nodes.value = [blankNode(1)]
}

defineExpose({ load, validate, toDefinition })
</script>

<style scoped lang="scss">
.flow-designer {
  display: flex;
  flex-direction: column;
  align-items: stretch;
}
.flow-arrow {
  text-align: center;
  color: #c0c4cc;
  font-size: 18px;
  line-height: 22px;
}
.flow-node {
  border-radius: 8px;
  padding: 10px 14px;
  &.fixed {
    text-align: center;
    font-weight: 600;
    color: #fff;
    align-self: center;
    min-width: 220px;
  }
  &.start {
    background: #67c23a;
  }
  &.end {
    background: #909399;
  }
  &.approval {
    border: 1px solid #dcdfe6;
    background: #fff;
  }
  &.invalid {
    border-color: #f56c6c;
  }
}
.node-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
  .node-index {
    font-weight: 600;
    color: #409eff;
  }
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 0 12px;
}
.assignee-hint {
  font-size: 12px;
  color: #909399;
  margin: -6px 0 6px;
}
.cond-item {
  margin-bottom: 4px;
}
.cond-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 6px;
}
.node-errors {
  margin: 4px 0 0 16px;
  color: #f56c6c;
  font-size: 12px;
  line-height: 1.7;
}
.add-btn {
  align-self: center;
  border-style: dashed;
}
</style>
