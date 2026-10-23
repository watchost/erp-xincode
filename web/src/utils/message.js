// Copyright 2026 zhouhouping. All Rights Reserved.
//
// 站内消息前端常量，口径与 docs/development/api.md §13.3 一致。

import { ref } from 'vue'

export const MESSAGE_CATEGORIES = [
  { value: 'approval_todo', label: '审批待办', type: 'warning' },
  { value: 'stock_low', label: '库存预警', type: 'danger' },
  { value: 'system', label: '系统通知', type: 'info' }
]

export function categoryMeta(c) {
  return MESSAGE_CATEGORIES.find((x) => x.value === c) || { value: c, label: c || '其他', type: 'info' }
}

export const CHANNELS = [
  { value: 'webhook', label: 'Webhook', placeholder: 'https://...', note: '向该 https 地址 POST JSON' },
  { value: 'dingtalk', label: '钉钉群机器人', placeholder: 'https://oapi.dingtalk.com/robot/send?access_token=...', note: '暂不支持钉钉“加签”安全设置，请改用关键词或 IP 白名单；尚未在真实钉钉环境验证' },
  { value: 'wecom', label: '企业微信群机器人', placeholder: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...', note: '尚未在真实企业微信环境验证' },
  { value: 'email', label: '邮件', placeholder: 'name@example.com', note: '后端尚未接入发信服务，订阅后消息会被记为“跳过”，不会真正发出邮件' }
]

// 消息接口是否可用。后端消息路由若未挂载（返回 404），铃铛和消息页显示“暂不可用”而不是反复报错。
export const messageApiUnavailable = ref(false)

export function markMessageApiState(err) {
  if (err?.response?.status === 404) messageApiUnavailable.value = true
}
