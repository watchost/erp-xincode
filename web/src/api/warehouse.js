// Copyright 2026 zhouhouping. All Rights Reserved.

import request from '@/utils/request'

export function inboundScan(data) {
  return request({
    url: '/warehouse/inbound/scan',
    method: 'post',
    data
  })
}

export function outboundScan(data) {
  return request({
    url: '/warehouse/outbound/scan',
    method: 'post',
    data
  })
}

export function listInventory(params) {
  return request({
    url: '/warehouse/inventory',
    method: 'get',
    params
  })
}

export function getStockAlerts() {
  return request({
    url: '/warehouse/stock-alerts',
    method: 'get'
  })
}