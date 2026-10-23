// Copyright 2026 zhouhouping. All Rights Reserved.
//
// 审批工作流接口，约定见 docs/development/api.md §12。

import request from '@/utils/request'

// ---- 流程定义（权限 workflow:definition）----
export function listDefinitions(params) {
  return request({ url: '/workflow/definitions', method: 'get', params })
}

export function createDefinition(data) {
  return request({ url: '/workflow/definitions', method: 'post', data })
}

export function deployDefinition(id) {
  return request({ url: `/workflow/definitions/${id}/deploy`, method: 'post' })
}

// ---- 实例（权限 workflow:task）----
export function getInstance(instanceNo) {
  return request({ url: `/workflow/instances/${encodeURIComponent(instanceNo)}`, method: 'get' })
}

export function getInstanceByBiz(bizType, bizNo) {
  return request({
    url: '/workflow/instances/by-biz',
    method: 'get',
    params: { biz_type: bizType, biz_no: bizNo }
  })
}

export function cancelInstance(instanceNo, comment) {
  return request({
    url: `/workflow/instances/${encodeURIComponent(instanceNo)}/cancel`,
    method: 'post',
    data: { comment: comment || '' }
  })
}

// ---- 待办 / 已办 ----
export function listTodo(params) {
  return request({ url: '/workflow/tasks/todo', method: 'get', params })
}

export function listDone(params) {
  return request({ url: '/workflow/tasks/done', method: 'get', params })
}

// 待办角标用：只取总数，失败不弹提示
export function countTodo() {
  return request({
    url: '/workflow/tasks/todo',
    method: 'get',
    params: { page: 1, page_size: 1 },
    silent: true
  })
}

// ---- 审批动作 ----
export function approveTask(taskId, comment) {
  return request({ url: `/workflow/tasks/${taskId}/approve`, method: 'post', data: { comment: comment || '' } })
}

export function rejectTask(taskId, comment) {
  return request({ url: `/workflow/tasks/${taskId}/reject`, method: 'post', data: { comment } })
}

export function transferTask(taskId, targetUserId, comment) {
  return request({
    url: `/workflow/tasks/${taskId}/transfer`,
    method: 'post',
    data: { target_user_id: targetUserId, comment: comment || '' }
  })
}

export function addSignTask(taskId, targetUserId, comment) {
  return request({
    url: `/workflow/tasks/${taskId}/add-sign`,
    method: 'post',
    data: { target_user_id: targetUserId, comment: comment || '' }
  })
}
