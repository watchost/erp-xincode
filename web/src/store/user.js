// Copyright 2026 zhouhouping. All Rights Reserved.

import { defineStore } from 'pinia'
import api from '@/utils/request'

export const useUserStore = defineStore('user', {
  state: () => ({
    token: localStorage.getItem('token') || '',
    userInfo: JSON.parse(localStorage.getItem('userInfo') || '{}')
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
        localStorage.setItem('userInfo', JSON.stringify(res.data.user))
        this.userInfo = res.data.user
      }
      return res
    },
    logout() {
      this.token = ''
      this.userInfo = {}
      localStorage.removeItem('token')
      localStorage.removeItem('userInfo')
    }
  }
})