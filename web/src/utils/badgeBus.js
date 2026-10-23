// Copyright 2026 zhouhouping. All Rights Reserved.
//
// 审批 / 消息操作完成后通知铃铛立即刷新未读数，不必等下一次轮询。

import { ref } from 'vue'

export const badgeRefreshTick = ref(0)

export function notifyBadgeRefresh() {
  badgeRefreshTick.value++
}
