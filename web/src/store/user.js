// Copyright 2026 zhouhouping. All Rights Reserved.

import { defineStore } from 'pinia'
import api from '@/utils/request'
import { hasPerm } from '@/utils/permission'

// 安全读取本地缓存的用户信息：旧版本可能写入了非法 JSON（如字符串 "undefined"），
// 直接 JSON.parse 会抛异常导致整个 store 初始化失败、页面白屏。
function loadUserInfo() {
  try {
    return JSON.parse(localStorage.getItem('userInfo') || '{}') || {}
  } catch (e) {
    return {}
  }
}

export const useUserStore = defineStore('user', {
  state: () => ({
    token: localStorage.getItem('token') || '',
    userInfo: loadUserInfo(),
    // 当前用户权限码集合，来自 GET /users/permissions；刷新页面后需重新拉取。
    permissions: [],
    permsLoaded: false
  }),
  getters: {
    isLoggedIn: (state) => !!state.token
  },
  actions: {
    async login(credentials) {
      const res = await api.post('/login', credentials)
      if (res.code === 0) {
        this.token = res.data.access_token
        localStorage.setItem('token', res.data.access_token)
        // 登录接口只返回 token，用户信息 / 权限通过独立接口获取
        await Promise.all([this.fetchUserInfo(), this.fetchPermissions()])
      }
      return res
    },
    async fetchUserInfo() {
      try {
        const res = await api.get('/users/profile')
        if (res.code === 0 && res.data) {
          this.userInfo = res.data
          localStorage.setItem('userInfo', JSON.stringify(res.data))
        }
      } catch (e) {
        if (!this.userInfo) this.userInfo = {}
      }
    },
    // 拉取当前用户权限码；路由守卫在进入受控页面前确保已加载。
    async fetchPermissions() {
      try {
        const res = await api.get('/users/permissions')
        if (res.code === 0) {
          this.permissions = Array.isArray(res.data) ? res.data : []
        }
      } catch (e) {
        this.permissions = []
      } finally {
        this.permsLoaded = true
      }
    },
    hasPermission(code) {
      return hasPerm(this.permissions, code)
    },
    logout() {
      this.token = ''
      this.userInfo = {}
      this.permissions = []
      this.permsLoaded = false
      localStorage.removeItem('token')
      localStorage.removeItem('userInfo')
    }
  }
})
