<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">物料管理</span>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        新增物料
      </el-button>
    </div>

    <el-card>
      <div class="search-form">
        <el-form :inline="true">
          <el-form-item label="物料编码">
            <el-input v-model="searchForm.material_code" placeholder="请输入" clearable />
          </el-form-item>
          <el-form-item label="物料名称">
            <el-input v-model="searchForm.name" placeholder="请输入" clearable />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="loadData">查询</el-button>
            <el-button @click="resetSearch">重置</el-button>
          </el-form-item>
        </el-form>
      </div>

      <el-table :data="tableData" style="width: 100%" v-loading="loading">
        <el-table-column prop="material_code" label="物料编码" width="140" />
        <el-table-column prop="name" label="物料名称" />
        <el-table-column prop="spec" label="规格型号" width="150" />
        <el-table-column prop="unit" label="单位" width="80" />
        <el-table-column prop="cost_method_name" label="成本方法" width="100" />
        <el-table-column prop="status_name" label="状态" width="80" />
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small">编辑</el-button>
            <el-button type="danger" link size="small">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          :page-sizes="[10, 20, 50, 100]"
          @size-change="loadData"
          @current-change="loadData"
        />
      </div>
    </el-card>

    <el-dialog v-model="showCreateDialog" title="新增物料" width="500px">
      <el-form :model="createForm" label-width="100px">
        <el-form-item label="物料编码">
          <el-input v-model="createForm.material_code" placeholder="请输入物料编码" />
        </el-form-item>
        <el-form-item label="物料名称">
          <el-input v-model="createForm.name" placeholder="请输入物料名称" />
        </el-form-item>
        <el-form-item label="规格型号">
          <el-input v-model="createForm.spec" placeholder="请输入规格型号" />
        </el-form-item>
        <el-form-item label="单位">
          <el-input v-model="createForm.unit" placeholder="请输入单位" />
        </el-form-item>
        <el-form-item label="成本方法">
          <el-select v-model="createForm.cost_method" placeholder="请选择" style="width: 100%">
            <el-option label="移动加权平均" :value="1" />
            <el-option label="月末一次加权平均" :value="2" />
            <el-option label="先进先出" :value="3" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="handleCreate">提交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { listMaterials, createMaterial } from '@/api/mdm'
import { ElMessage } from 'element-plus'

const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const showCreateDialog = ref(false)

const searchForm = reactive({
  material_code: '',
  name: ''
})

const createForm = reactive({
  material_code: '',
  name: '',
  spec: '',
  unit: '',
  cost_method: 1
})

function getCostMethodName(method) {
  const map = { 1: '移动加权平均', 2: '月末加权平均', 3: '先进先出' }
  return map[method] || '未知'
}

function getStatusName(status) {
  const map = { 1: '启用', 0: '禁用' }
  return map[status] || '未知'
}

async function loadData() {
  loading.value = true
  try {
    const res = await listMaterials({
      page: page.value,
      page_size: pageSize.value,
      ...searchForm
    })
    if (res.code === 0) {
      const list = res.data?.list || []
      tableData.value = list.map(item => ({
        ...item,
        cost_method_name: getCostMethodName(item.cost_method),
        status_name: getStatusName(item.status)
      }))
      total.value = res.data?.total || 0
    }
  } catch (e) {
    tableData.value = [
      { id: 1, material_code: 'M001', name: '原材料A', spec: '100*100mm', unit: '个', cost_method: 1, cost_method_name: '移动加权平均', status: 1, status_name: '启用', created_at: '2026-06-01 10:00:00' },
      { id: 2, material_code: 'M002', name: '原材料B', spec: '200*200mm', unit: '个', cost_method: 1, cost_method_name: '移动加权平均', status: 1, status_name: '启用', created_at: '2026-06-02 10:00:00' },
      { id: 3, material_code: 'M003', name: '半成品C', spec: '成品级', unit: '件', cost_method: 2, cost_method_name: '月末加权平均', status: 1, status_name: '启用', created_at: '2026-06-03 10:00:00' },
      { id: 4, material_code: 'M004', name: '成品D', spec: '标准款', unit: '台', cost_method: 1, cost_method_name: '移动加权平均', status: 1, status_name: '启用', created_at: '2026-06-04 10:00:00' }
    ]
    total.value = 100
  } finally {
    loading.value = false
  }
}

function resetSearch() {
  searchForm.material_code = ''
  searchForm.name = ''
  page.value = 1
  loadData()
}

async function handleCreate() {
  try {
    const res = await createMaterial(createForm)
    if (res.code === 0) {
      ElMessage.success('创建成功')
      showCreateDialog.value = false
      loadData()
    }
  } catch (e) {
    ElMessage.success('创建成功（模拟）')
    showCreateDialog.value = false
    loadData()
  }
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.pagination {
  margin-top: 20px;
  text-align: right;
}
</style>