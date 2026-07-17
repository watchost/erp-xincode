// Copyright 2026 zhouhouping. All Rights Reserved.

import request from '@/utils/request'

export function listCostCards(params) {
  return request({
    url: '/finance/cost-cards',
    method: 'get',
    params
  })
}

export function getCostSummary() {
  return request({
    url: '/finance/cost-summary',
    method: 'get'
  })
}

export function listAccountEntries(params) {
  return request({
    url: '/finance/account-entries',
    method: 'get',
    params
  })
}

export function getFinancialReport(params) {
  return request({
    url: '/finance/financial-report',
    method: 'get',
    params
  })
}

export function listBudgets(params) {
  return request({
    url: '/finance/budgets',
    method: 'get',
    params
  })
}