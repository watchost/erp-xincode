// Copyright 2026 zhouhouping. All Rights Reserved.
//
// 盘点录入接口 POST /warehouse/stock-checks/:id/items 要求 material_id（必填）和 location_id，
// 而 PDA 扫到的是物料编码 / 库位编码，这里负责把编码解析成主键 ID。

import { listMaterials, listLocations } from '@/api/mdm'

// 物料列表接口按编码做模糊匹配，这里必须在结果里精确比对编码，避免 "M01" 命中 "M010"。
export async function resolveMaterialByCode(code) {
  const target = String(code || '').trim()
  if (!target) throw new Error('物料编码为空')
  const res = await listMaterials({ material_code: target, page: 1, page_size: 100 })
  const list = res.data?.list || []
  const hit = list.find((m) => m.material_code === target)
  if (!hit) throw new Error(`物料编码 [${target}] 不存在`)
  return hit
}

export async function resolveLocationIdByCode(warehouseId, locationCode) {
  if (!locationCode) return null
  const res = await listLocations({ warehouse_id: warehouseId })
  const list = res.data || []
  const hit = list.find((l) => l.location_code === locationCode)
  if (!hit) throw new Error(`库位 [${locationCode}] 不属于该盘点仓库`)
  return hit.id
}

// 把 PDA 侧保存的编码型盘点记录转换成后端要求的请求体。
export async function buildStockCheckEntry(raw) {
  const material = await resolveMaterialByCode(raw.material_code)
  const locationId = await resolveLocationIdByCode(raw.warehouse_id, raw.location_code)
  return {
    items: [
      {
        material_id: material.id,
        location_id: locationId,
        actual_qty: raw.actual_qty
      }
    ]
  }
}

export function apiErrorMessage(err) {
  return err?.response?.data?.msg || err?.message || '接口错误'
}
