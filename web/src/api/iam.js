// Copyright 2026 zhouhouping. All Rights Reserved.

import request from '@/utils/request'

export function login(data) {
  return request({
    url: '/login',
    method: 'post',
    data
  })
}

export function getUserInfo() {
  return request({
    url: '/users/profile',
    method: 'get'
  })
}

export function getPermissions() {
  return request({
    url: '/users/permissions',
    method: 'get'
  })
}

export function listUsers(params) {
  return request({
    url: '/iam/users',
    method: 'get',
    params
  })
}

export function createUser(data) {
  return request({
    url: '/iam/users',
    method: 'post',
    data
  })
}

export function updateUser(id, data) {
  return request({
    url: `/iam/users/${id}`,
    method: 'put',
    data
  })
}