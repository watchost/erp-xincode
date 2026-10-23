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

// 采购订单走审批流：草稿(10) → 审批中(15)，返回 { instance_no }。
// 已部署 purchase_order 流程时，原直接审批接口 /orders/:no/approve 会返回 40010，前端不再调用。
// silent：错误由页面按错误码给出具体处理提示（如 80009 找不到审批人）
export function submitPurchaseOrder(orderNo) {
  return request({
    url: `/purchase/orders/${encodeURIComponent(orderNo)}/submit`,
    method: 'post',
    silent: true
  })
}

export function cancelPurchaseOrder(orderNo) {
  return request({
    url: `/purchase/orders/${encodeURIComponent(orderNo)}/cancel`,
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