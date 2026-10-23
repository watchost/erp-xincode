// Copyright 2026 zhouhouping. All Rights Reserved.
//
// 站内消息接口，约定见 docs/development/api.md §13.3（权限 message:view）。

import request from '@/utils/request'

export function listMessages(params) {
  return request({ url: '/messages', method: 'get', params })
}

// 通知铃铛轮询：失败不弹提示，由铃铛组件自行降级
export function getUnreadCount() {
  return request({ url: '/messages/unread-count', method: 'get', silent: true })
}

export function readMessage(id) {
  return request({ url: `/messages/${id}/read`, method: 'post' })
}

export function readAllMessages() {
  return request({ url: '/messages/read-all', method: 'post' })
}

export function listSubscriptions() {
  return request({ url: '/messages/subscriptions', method: 'get' })
}

export function saveSubscription(data) {
  return request({ url: '/messages/subscriptions', method: 'put', data })
}
