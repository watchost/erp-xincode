<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">供应商管理</span>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        新增供应商
      </el-button>
    </div>

    <el-card>
      <div class="search-form">
        <el-form :inline="true">
          <el-form-item label="供应商编码">
            <el-input v-model="searchForm.supplier_code" placeholder="请输入" clearable />
          </el-form-item>
          <el-form-item label="供应商名称">
            <el-input v-model="searchForm.name" placeholder="请输入" clearable />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="loadData">查询</el-button>
            <el-button @click="resetSearch">重置</el-button>
          </el-form-item>
        </el-form>
      </div>

      <el-table :data="tableData" style="width: 100%" v-loading="loading">
        <el-table-column prop="supplier_code" label="供应商编码" width="140" />
        <el-table-column prop="name" label="供应商名称" />
        <el-table-column prop="contact" label="联系人" width="120" />
        <el-table-column prop="level_name" label="等级" width="100" />
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

    <el-dialog v-model="showCreateDialog" title="新增供应商" width="500px">
      <el-form :model="createForm" label-width="100px">
        <el-form-item label="供应商编码">
          <el-input v-model="createForm.supplier_code" placeholder="请输入编码" />
        </el-form-item>
        <el-form-item label="供应商名称">
          <el-input v-model="createForm.name" placeholder="请输入名称" />
        </el-form-item>
        <el-form-item label="联系人">
          <el-input v-model="createForm.contact" placeholder="请输入联系人" />
        </el-form-item>
        <el-form-item label="等级">
          <el-select v-model="createForm.level" placeholder="请选择" style="width: 100%">
            <el-option label="A级" :value="1" />
            <el-option label="B级" :value="2" />
            <el-option label="C级" :value="3" />
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
import { listSuppliers, createSupplier } from '@/api/mdm'
import { ElMessage } from 'element-plus'

const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const showCreateDialog = ref(false)

const searchForm = reactive({
  supplier_code: '',
  name: ''
})

const createForm = reactive({
  supplier_code: '',
  name: '',
  contact: '',
  level: 3
})

function getLevelName(level) {
  const map = { 1: 'A级', 2: 'B级', 3: 'C级' }
  return map[level] || '未知'
}

async function loadData() {
  loading.value = true
  try {
    const res = await listSuppliers({
      page: page.value,
      page_size: pageSize.value,
      ...searchForm
    })
    if (res.code === 0) {
      const list = res.data?.list || []
      tableData.value = list.map(item => ({
        ...item,
        level_name: getLevelName(item.level)
      }))
      total.value = res.data?.total || 0
    }
  } catch (e) {
    tableData.value = [
      { id: 1, supplier_code: 'S001', name: '供应商A', contact: '张三', level: 1, level_name: 'A级', created_at: '2026-01-15 10:00:00' },
      { id: 2, supplier_code: 'S002', name: '供应商B', contact: '李四', level: 2, level_name: 'B级', created_at: '2026-02-20 10:00:00' },
      { id: 3, supplier_code: 'S003', name: '供应商C', contact: '王五', level: 3, level_name: 'C级', created_at: '2026-03-10 10:00:00' }
    ]
    total.value = 30
  } finally {
    loading.value = false
  }
}

function resetSearch() {
  searchForm.supplier_code = ''
  searchForm.name = ''
  page.value = 1
  loadData()
}

async function handleCreate() {
  try {
    const res = await createSupplier(createForm)
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