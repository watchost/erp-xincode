<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">角色管理</span>
    </div>

    <el-card>
      <el-table :data="tableData" style="width: 100%">
        <el-table-column prop="code" label="角色编码" width="150" />
        <el-table-column prop="name" label="角色名称" width="150" />
        <el-table-column prop="data_scope_name" label="数据权限" width="120" />
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small">编辑</el-button>
            <el-button type="success" link size="small">分配权限</el-button>
            <el-button type="danger" link size="small">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'

const tableData = ref([])

function getDataScopeName(scope) {
  const map = { 1: '全部', 2: '本部门', 3: '本人' }
  return map[scope] || '未知'
}

async function loadData() {
  tableData.value = [
    { id: 1, code: 'admin', name: '超级管理员', data_scope: 1, data_scope_name: '全部', created_at: '2026-01-01 00:00:00' },
    { id: 2, code: 'manager', name: '管理层', data_scope: 1, data_scope_name: '全部', created_at: '2026-01-01 00:00:00' },
    { id: 3, code: 'warehouse', name: '仓库管理员', data_scope: 2, data_scope_name: '本部门', created_at: '2026-01-01 00:00:00' },
    { id: 4, code: 'purchase', name: '采购专员', data_scope: 2, data_scope_name: '本部门', created_at: '2026-01-01 00:00:00' },
    { id: 5, code: 'production', name: '生产计划员', data_scope: 2, data_scope_name: '本部门', created_at: '2026-01-01 00:00:00' },
    { id: 6, code: 'finance', name: '财务人员', data_scope: 2, data_scope_name: '本部门', created_at: '2026-01-01 00:00:00' }
  ]
}

onMounted(() => {
  loadData()
})
</script>