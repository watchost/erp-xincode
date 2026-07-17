// Copyright 2026 zhouhouping. All Rights Reserved.

import request from '@/utils/request'

export function listPurchaseOrders(params) {
  return request({
    url: '/purchase/orders',
    method: 'get',
    params
  })
}

export function createPurchaseOrder(data) {
  return request({
    url: '/purchase/orders',
    method: 'post',
    data
  })
}

export function approvePurchaseOrder(orderNo) {
  return request({
    url: `/purchase/orders/${orderNo}/approve`,
    method: 'post'
  })
}

export function purchaseInboundScan(data) {
  return request({
    url: '/purchase/inbound/scan',
    method: 'post',
    data
  })
}