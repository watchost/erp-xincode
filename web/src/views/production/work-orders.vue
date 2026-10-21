<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">生产工单</span>
      <el-button v-permission="'production:wo:create'" type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        新建工单
      </el-button>
    </div>

    <el-card>
      <div class="search-form">
        <el-form :inline="true">
          <el-form-item label="工单号">
            <el-input v-model="searchForm.work_order_no" placeholder="请输入" clearable />
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="searchForm.status" placeholder="请选择" clearable style="width: 120px">
              <el-option label="草稿" :value="10" />
              <el-option label="已下达" :value="20" />
              <el-option label="生产中" :value="30" />
              <el-option label="已完成" :value="40" />
              <el-option label="已关闭" :value="50" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="loadData">查询</el-button>
            <el-button @click="resetSearch">重置</el-button>
          </el-form-item>
        </el-form>
      </div>

      <el-table :data="tableData" style="width: 100%" v-loading="loading">
        <el-table-column prop="work_order_no" label="工单号" width="180" />
        <el-table-column prop="product_name" label="产品名称" />
        <el-table-column prop="plan_qty" label="计划数量" width="100" />
        <el-table-column prop="status_desc" label="状态" width="100" />
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small">查看</el-button>
            <el-button
              v-if="row.status === 10"
              v-permission="'production:wo:create'"
              type="success"
              link
              size="small"
              @click="handleRelease(row)"
            >
              下达
            </el-button>
            <el-button
              v-if="row.status === 20 || row.status === 30 || row.status === 40"
              v-permission="'production:wo:create'"
              type="danger"
              link
              size="small"
              @click="handleClose(row)"
            >
              关闭
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
        <el-form-item label="产品">
          <el-select
            v-model="createForm.product_id"
            filterable
            placeholder="请选择产品"
            style="width: 100%"
          >
            <el-option
              v-for="m in materialList"
              :key="m.id"
              :label="m.material_code + ' ' + m.name"
              :value="m.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="计划数量">
          <el-input-number v-model="createForm.plan_qty" :min="1" style="width: 100%" />
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
import { listWorkOrders, createWorkOrder, releaseWorkOrder, closeWorkOrder } from '@/api/production'
import { listMaterials } from '@/api/mdm'
import { ElMessage, ElMessageBox } from 'element-plus'

const loading = ref(false)
const tableData = ref([])
const materialList = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const showCreateDialog = ref(false)

const searchForm = reactive({
  work_order_no: '',
  status: null
})

const createForm = reactive({
  product_id: null,
  plan_qty: 100
})

// 后端返回 RFC3339 时间，统一展示为 YYYY-MM-DD HH:mm
function formatTime(t) {
  if (!t) return ''
  return String(t).replace('T', ' ').slice(0, 16)
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
      // 状态文案直接用后端 status_desc；时间格式化展示
      tableData.value = list.map(item => ({
        ...item,
        created_at: formatTime(item.created_at)
      }))
      total.value = res.data?.total || 0
    }
  } catch (e) {
    tableData.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function resetSearch() {
  searchForm.work_order_no = ''
  searchForm.status = null
  page.value = 1
  loadData()
}

async function loadMaterials() {
  try {
    const res = await listMaterials({ page: 1, page_size: 100 })
    if (res.code === 0) {
      materialList.value = res.data?.list || []
    }
  } catch (e) {
    materialList.value = []
  }
}

async function handleCreate() {
  if (!createForm.product_id) {
    ElMessage.warning('请选择产品')
    return
  }
  if (!createForm.plan_qty || createForm.plan_qty <= 0) {
    ElMessage.warning('计划数量必须大于 0')
    return
  }
  try {
    const res = await createWorkOrder({
      product_id: createForm.product_id,
      plan_qty: createForm.plan_qty
    })
    if (res.code === 0) {
      ElMessage.success('创建成功')
      showCreateDialog.value = false
      createForm.product_id = null
      createForm.plan_qty = 100
      loadData()
    }
  } catch (e) {
    // 失败时拦截器已提示错误，保留对话框与表单内容供用户重试
  }
}

async function handleRelease(row) {
  ElMessageBox.confirm('确定下达该工单吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      const res = await releaseWorkOrder(row.work_order_no)
      if (res.code === 0) {
        ElMessage.success('下达成功')
        loadData()
      }
    } catch (e) {
      // 失败时拦截器已提示错误，不刷新列表
    }
  }).catch(() => {})
}

async function handleClose(row) {
  ElMessageBox.confirm('确定关闭该工单吗？已领料/已入库的工单无法关闭。', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      const res = await closeWorkOrder(row.work_order_no)
      if (res.code === 0) {
        ElMessage.success('关闭成功')
        loadData()
      }
    } catch (e) {
      // 失败时拦截器已提示错误，不刷新列表
    }
  }).catch(() => {})
}

onMounted(() => {
  loadMaterials()
  loadData()
})
</script>

<style scoped>
.pagination {
  margin-top: 20px;
  text-align: right;
}
</style>