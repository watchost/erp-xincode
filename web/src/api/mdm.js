// Copyright 2026 zhouhouping. All Rights Reserved.

import request from '@/utils/request'

export function listMaterials(params) {
  return request({
    url: '/mdm/materials',
    method: 'get',
    params
  })
}

export function getMaterial(id) {
  return request({
    url: `/mdm/materials/${id}`,
    method: 'get'
  })
}

export function createMaterial(data) {
  return request({
    url: '/mdm/materials',
    method: 'post',
    data
  })
}

export function updateMaterial(id, data) {
  return request({
    url: `/mdm/materials/${id}`,
    method: 'put',
    data
  })
}

export function listSuppliers(params) {
  return request({
    url: '/mdm/suppliers',
    method: 'get',
    params
  })
}

export function createSupplier(data) {
  return request({
    url: '/mdm/suppliers',
    method: 'post',
    data
  })
}

export function updateSupplier(id, data) {
  return request({
    url: `/mdm/suppliers/${id}`,
    method: 'put',
    data
  })
}

export function listWarehouses() {
  return request({
    url: '/mdm/warehouses',
    method: 'get'
  })
}

export function listLocations(params) {
  return request({
    url: '/mdm/locations',
    method: 'get',
    params
  })
}