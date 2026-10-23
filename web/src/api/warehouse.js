// Copyright 2026 zhouhouping. All Rights Reserved.

import request from '@/utils/request'

export function inboundScan(data, idempotencyKey) {
  const headers = {}
  if (idempotencyKey) {
    headers['Idempotency-Key'] = idempotencyKey
  }
  return request({
    url: '/warehouse/inbound/scan',
    method: 'post',
    headers,
    data
  })
}

export function outboundScan(data, idempotencyKey) {
  const headers = {}
  if (idempotencyKey) {
    headers['Idempotency-Key'] = idempotencyKey
  }
  return request({
    url: '/warehouse/outbound/scan',
    method: 'post',
    headers,
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

export function listStockChecks(params) {
  return request({
    url: '/warehouse/stock-checks',
    method: 'get',
    params
  })
}

export function getStockCheck(id) {
  return request({
    url: `/warehouse/stock-checks/${id}`,
    method: 'get'
  })
}

export function createStockCheck(data) {
  return request({
    url: '/warehouse/stock-checks',
    method: 'post',
    data
  })
}

export function entryStockCheck(id, data, idempotencyKey) {
  const headers = {}
  if (idempotencyKey) {
    headers['Idempotency-Key'] = idempotencyKey
  }
  return request({
    url: `/warehouse/stock-checks/${id}/items`,
    method: 'post',
    headers,
    data
  })
}