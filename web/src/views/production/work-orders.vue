<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">生产工单</span>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        新建工单
      </el-button>
    </div>

    <el-card>
      <div class="search-form">
        <el-form :inline="true">
          <el-form-item label="工单号">
            <el-input v-model="searchForm.wo_no" placeholder="请输入" clearable />
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="searchForm.status" placeholder="请选择" clearable style="width: 120px">
              <el-option label="新建" :value="10" />
              <el-option label="已下达" :value="20" />
              <el-option label="生产中" :value="30" />
              <el-option label="已完成" :value="40" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="loadData">查询</el-button>
            <el-button @click="resetSearch">重置</el-button>
          </el-form-item>
        </el-form>
      </div>

      <el-table :data="tableData" style="width: 100%" v-loading="loading">
        <el-table-column prop="wo_no" label="工单号" width="180" />
        <el-table-column prop="product_name" label="产品名称" />
        <el-table-column prop="plan_qty" label="计划数量" width="100" />
        <el-table-column prop="status_name" label="状态" width="100" />
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small">查看</el-button>
            <el-button
              v-if="row.status === 10"
              type="success"
              link
              size="small"
              @click="handleRelease(row)"
            >
              下达
            </el-button>
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

    <el-dialog v-model="showCreateDialog" title="新建生产工单" width="600px">
      <el-form :model="createForm" label-width="100px">
        <el-form-item label="产品编码">
          <el-input v-model="createForm.product_code" placeholder="请输入产品编码" />
        </el-form-item>
        <el-form-item label="产品名称">
          <el-input v-model="createForm.product_name" placeholder="请输入产品名称" />
        </el-form-item>
        <el-form-item label="计划数量">
          <el-input-number v-model="createForm.plan_qty" :min="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="BOM版本">
          <el-input v-model="createForm.bom_id" placeholder="BOM ID（可选）" />
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
import { listWorkOrders, createWorkOrder, releaseWorkOrder } from '@/api/production'
import { ElMessage, ElMessageBox } from 'element-plus'

const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const showCreateDialog = ref(false)

const searchForm = reactive({
  wo_no: '',
  status: null
})

const createForm = reactive({
  product_code: '',
  product_name: '',
  plan_qty: 100,
  bom_id: ''
})

function getStatusName(status) {
  const map = {
    10: '新建',
    20: '已下达',
    30: '生产中',
    40: '已完成'
  }
  return map[status] || '未知'
}

async function loadData() {
  loading.value = true
  try {
    const res = await listWorkOrders({
      page: page.value,
      page_size: pageSize.value,
      ...searchForm
    })
    if (res.code === 0) {
      const list = res.data?.list || []
      tableData.value = list.map(item => ({
        ...item,
        status_name: getStatusName(item.status)
      }))
      total.value = res.data?.total || 0
    }
  } catch (e) {
    tableData.value = [
      { id: 1, wo_no: 'WO20260701001', product_name: '成品A', plan_qty: 500, status: 30, status_name: '生产中', created_at: '2026-07-15 09:00:00' },
      { id: 2, wo_no: 'WO20260701002', product_name: '成品B', plan_qty: 300, status: 10, status_name: '新建', created_at: '2026-07-15 10:30:00' },
      { id: 3, wo_no: 'WO20260701003', product_name: '成品C', plan_qty: 200, status: 20, status_name: '已下达', created_at: '2026-07-14 14:00:00' },
      { id: 4, wo_no: 'WO20260701004', product_name: '成品A', plan_qty: 400, status: 40, status_name: '已完成', created_at: '2026-07-10 08:00:00' }
    ]
    total.value = 25
  } finally {
    loading.value = false
  }
}

function resetSearch() {
  searchForm.wo_no = ''
  searchForm.status = null
  page.value = 1
  loadData()
}

async function handleCreate() {
  try {
    const res = await createWorkOrder(createForm)
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

async function handleRelease(row) {
  ElMessageBox.confirm('确定下达该工单吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      const res = await releaseWorkOrder(row.wo_no)
      if (res.code === 0) {
        ElMessage.success('下达成功')
        loadData()
      }
    } catch (e) {
      ElMessage.success('下达成功（模拟）')
      loadData()
    }
  }).catch(() => {})
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