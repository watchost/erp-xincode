// Copyright 2026 zhouhouping. All Rights Reserved.

import request from '@/utils/request'

export function getOverview() {
  return request({
    url: '/dashboard/overview',
    method: 'get'
  })
}

export function getDashboardStockAlerts() {
  return request({
    url: '/dashboard/stock-alerts',
    method: 'get'
  })
}

export function getRecentOrders() {
  return request({
    url: '/dashboard/recent-orders',
    method: 'get'
  })
}

export function getLLMAnalysis(data) {
  return request({
    url: '/dashboard/llm/analysis',
    method: 'post',
    data
  })
}