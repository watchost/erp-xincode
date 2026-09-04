// Copyright 2026 zhouhouping. All Rights Reserved.

// 按钮级权限指令：v-permission="'iam:user'" 或 v-permission="['a','b']"（任一满足即可）。
// 无权限时直接把元素从 DOM 中移除。权限码与后端 sys_permission 种子、路由 meta.permission 保持一致。
import { useUserStore } from '@/store/user'
import { hasPerm } from '@/utils/permission'

export { hasPerm, SUPER_ADMIN_PERM } from '@/utils/permission'

export const permissionDirective = {
  mounted(el, binding) {
    const store = useUserStore()
    if (!hasPerm(store.permissions, binding.value)) {
      el.parentNode && el.parentNode.removeChild(el)
    }
  }
}
