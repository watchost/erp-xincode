// Copyright 2026 zhouhouping. All Rights Reserved.

import request from '@/utils/request'

export function listDevices(params) {
  return request({
    url: '/device/list',
    method: 'get',
    params
  })
}

export function registerDevice(data) {
  return request({
    url: '/device/register',
    method: 'post',
    data
  })
}

export function getDevice(deviceCode) {
  return request({
    url: `/device/${deviceCode}`,
    method: 'get'
  })
}