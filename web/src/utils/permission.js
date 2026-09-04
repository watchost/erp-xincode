// Copyright 2026 zhouhouping. All Rights Reserved.

// 权限码工具：纯函数，无 store 依赖，供指令 / store / 路由守卫 / 菜单共用。
// 权限码与后端 sys_permission 种子、internal/routes/routes.go 保持一致。

export const SUPER_ADMIN_PERM = '*:*:*'

// hasPerm 判断权限码集合是否包含某（些）权限；超管码 *:*:* 直接放行。
// code 传字符串表示必须拥有该权限；传数组表示拥有任一即可；传空表示不限制。
export function hasPerm(perms, code) {
  if (!code) return true
  if (!perms || perms.length === 0) return false
  if (perms.includes(SUPER_ADMIN_PERM)) return true
  const codes = Array.isArray(code) ? code : [code]
  return codes.some((c) => perms.includes(c))
}
