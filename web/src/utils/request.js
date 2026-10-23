// Copyright 2026 zhouhouping. All Rights Reserved.

import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 30000
})

request.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers['Authorization'] = 'Bearer ' + token
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 统一把后端业务错误码 / 文案挂到错误对象上（err.bizCode / err.bizMsg），
// 方便页面按错误码给出更具体的处理提示。
// 请求配置 silent: true 时不弹全局提示（用于后台轮询等场景），由调用方自行处理。
function bizError(msg, code) {
  const err = new Error(msg || 'Error')
  err.bizCode = code
  err.bizMsg = msg
  return err
}

request.interceptors.response.use(
  (response) => {
    const res = response.data
    if (res.code !== 0) {
      if (!response.config?.silent) {
        ElMessage.error(res.msg || res.message || '请求失败')
      }
      if (res.code === 10001) {
        localStorage.removeItem('token')
        router.push('/login')
      }
      return Promise.reject(bizError(res.msg || res.message, res.code))
    }
    return res
  },
  (error) => {
    const body = error.response?.data
    if (body && typeof body === 'object' && body.code !== undefined) {
      error.bizCode = body.code
      error.bizMsg = body.msg || body.message
    }
    if (!error.config?.silent) {
      ElMessage.error(error.bizMsg || error.message || '网络错误')
    }
    return Promise.reject(error)
  }
)

export default request
