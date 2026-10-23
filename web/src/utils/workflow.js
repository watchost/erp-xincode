// Copyright 2026 zhouhouping. All Rights Reserved.
//
// 审批工作流前端常量与文案，口径与 docs/development/api.md §12 一致。

export const INSTANCE_STATUS = {
  1: { text: '审批中', type: 'primary' },
  2: { text: '已通过', type: 'success' },
  3: { text: '已驳回', type: 'danger' },
  4: { text: '已撤销', type: 'info' }
}

export const TASK_STATUS = {
  1: { text: '待办', type: 'warning' },
  2: { text: '已通过', type: 'success' },
  3: { text: '已转办', type: 'info' },
  4: { text: '已驳回', type: 'danger' },
  5: { text: '已取消', type: 'info' }
}

export const SIGN_MODE = {
  any: '或签（任一人通过）',
  all: '会签（全部通过）'
}

export const ACTION_TEXT = {
  start: '提交',
  approve: '通过',
  reject: '驳回',
  transfer: '转办',
  add_sign: '加签',
  cancel: '撤销'
}

export const BIZ_TYPE = {
  purchase_order: '采购订单'
}

// 后端当前只支持这 5 种审批人规则；抄送、并行分支、条件分支节点尚未实现，设计器不提供。
export const ASSIGNEE_TYPES = [
  { value: 'user', label: '指定人', needValue: true, hint: '填写用户 ID，多个用英文逗号分隔' },
  { value: 'role', label: '角色', needValue: true, hint: '填写角色编码，如 manager' },
  { value: 'dept_leader', label: '部门负责人', needValue: false, hint: '发起人所在部门负责人；发起人本人是负责人时取上级部门负责人' },
  { value: 'dept_leader_up', label: '上级部门负责人', needValue: false, hint: '发起人所在部门的上级部门负责人' },
  {
    value: 'initiator_leader',
    label: '发起人上级',
    needValue: false,
    hint: '用户表没有直属上级字段，后端按部门负责人口径处理（与“部门负责人”相同）'
  }
]

export const CONDITION_OPS = ['>', '>=', '<', '<=', '==', '!=']

// 采购订单启动变量只有 amount（订单总额）
export const CONDITION_FIELDS = [{ value: 'amount', label: '单据金额 amount' }]

const ERROR_HINTS = {
  80002: '该单据类型没有已部署的审批流程，请联系管理员在“流程定义”中部署。',
  80005: '你不是这条待办的审批人，可能已被转办给他人。',
  80006: '这条待办已经处理过了，列表已刷新。',
  80007: '该单据已有运行中的审批流程，不能重复提交。',
  80008: '驳回时必须填写审批意见。',
  80009: '找不到可用的审批人。请管理员确认：发起人所在部门已设置负责人，且至少有一名启用的用户拥有 manager 角色。',
  80010: '流程定义不合法',
  80011: '只有发起人可以撤销流程。',
  80012: '目标用户不存在、已停用，或已经是本节点的审批人。'
}

// 优先给出可操作的说明，没有对应说明时用后端原文
export function workflowErrorText(err) {
  const code = err?.bizCode
  const raw = err?.bizMsg || err?.response?.data?.msg || err?.message || '操作失败'
  if (code === 80010) return `流程定义不合法：${raw}`
  return ERROR_HINTS[code] || raw
}

export function fmtTime(t) {
  if (!t) return '-'
  const d = new Date(t)
  if (Number.isNaN(d.getTime())) return String(t)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// 业务单据跳转：目前只接入了采购订单
export function bizLink(bizType, bizNo) {
  if (bizType === 'purchase_order') return { path: '/purchase/orders', query: { order_no: bizNo } }
  return null
}
