// Copyright 2026 zhouhouping. All Rights Reserved.

import request from '@/utils/request'

export function listWorkOrders(params) {
  return request({
    url: '/production/work-orders',
    method: 'get',
    params
  })
}

export function createWorkOrder(data) {
  return request({
    url: '/production/work-orders',
    method: 'post',
    data
  })
}

export function releaseWorkOrder(workOrderNo) {
  return request({
    url: `/production/work-orders/${workOrderNo}/release`,
    method: 'post'
  })
}

export function materialIssueScan(data) {
  return request({
    url: '/production/material-issue/scan',
    method: 'post',
    data
  })
}

export function createBom(data) {
  return request({
    url: '/production/bom',
    method: 'post',
    data
  })
}