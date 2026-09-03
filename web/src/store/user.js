// Copyright 2026 zhouhouping. All Rights Reserved.

import { defineStore } from 'pinia'
import api from '@/utils/request'

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
    userInfo: loadUserInfo()
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
        // 登录接口只返回 token，用户信息通过 /users/profile 获取
        await this.fetchUserInfo()
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
    logout() {
      this.token = ''
      this.userInfo = {}
      localStorage.removeItem('token')
      localStorage.removeItem('userInfo')
    }
  }
})
