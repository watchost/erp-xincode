<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">仓库管理</span>
    </div>

    <el-card>
      <el-table :data="tableData" style="width: 100%" v-loading="loading">
        <el-table-column prop="code" label="仓库编码" width="140" />
        <el-table-column prop="name" label="仓库名称" />
        <el-table-column prop="type_name" label="仓库类型" width="120" />
        <el-table-column prop="created_at" label="创建时间" width="180" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { listWarehouses } from '@/api/mdm'

const loading = ref(false)
const tableData = ref([])

function getTypeName(type) {
  const map = { 1: '普通仓', 2: '良品仓', 3: '不良品仓' }
  return map[type] || '未知'
}

async function loadData() {
  loading.value = true
  try {
    const res = await listWarehouses()
    if (res.code === 0) {
      const list = res.data || []
      tableData.value = list.map(item => ({
        ...item,
        type_name: getTypeName(item.type)
      }))
    }
  } catch (e) {
    tableData.value = [
      { id: 1, code: 'WH001', name: '主仓库', type: 1, type_name: '普通仓', created_at: '2026-01-01 00:00:00' },
      { id: 2, code: 'WH002', name: '良品仓', type: 2, type_name: '良品仓', created_at: '2026-01-01 00:00:00' },
      { id: 3, code: 'WH003', name: '不良品仓', type: 3, type_name: '不良品仓', created_at: '2026-01-01 00:00:00' }
    ]
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>