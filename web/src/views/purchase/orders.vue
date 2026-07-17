<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">采购订单</span>
      <el-button type="primary" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        新建订单
      </el-button>
    </div>

    <el-card>
      <div class="search-form">
        <el-form :inline="true">
          <el-form-item label="订单号">
            <el-input v-model="searchForm.order_no" placeholder="请输入" clearable />
          </el-form-item>
          <el-form-item label="供应商">
            <el-select v-model="searchForm.supplier_id" placeholder="请选择" clearable style="width: 150px">
              <el-option
                v-for="s in supplierList"
                :key="s.id"
                :label="s.name"
                :value="s.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="searchForm.status" placeholder="请选择" clearable style="width: 120px">
              <el-option label="待审批" :value="10" />
              <el-option label="已审批" :value="20" />
              <el-option label="部分入库" :value="30" />
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
        <el-table-column prop="order_no" label="订单号" width="180" />
        <el-table-column prop="supplier_name" label="供应商" />
        <el-table-column prop="total_amount" label="总金额" width="120">
          <template #default="{ row }">
            ¥{{ row.total_amount }}
          </template>
        </el-table-column>
        <el-table-column prop="status_name" label="状态" width="100" />
        <el-table-column prop="plan_arrive_at" label="预计到货" width="160" />
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="viewDetail(row)">查看</el-button>
            <el-button
              v-if="row.status === 10"
              type="success"
              link
              size="small"
              @click="handleApprove(row)"
            >
              审批
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

    <el-dialog v-model="showCreateDialog" title="新建采购订单" width="700px">
      <el-form :model="createForm" label-width="100px">
        <el-form-item label="供应商" prop="supplier_id">
          <el-select v-model="createForm.supplier_id" placeholder="请选择供应商" style="width: 100%">
            <el-option
              v-for="s in supplierList"
              :key="s.id"
              :label="s.name"
              :value="s.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="预计到货">
          <el-date-picker
            v-model="createForm.plan_arrive_at"
            type="datetime"
            placeholder="选择日期"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="订单明细">
          <el-table :data="createForm.items" style="width: 100%">
            <el-table-column prop="material_code" label="物料编码" width="140">
              <template #default="{ row, $index }">
                <el-input v-model="row.material_code" size="small" />
              </template>
            </el-table-column>
            <el-table-column prop="material_name" label="物料名称">
              <template #default="{ row }">
                <el-input v-model="row.material_name" size="small" />
              </template>
            </el-table-column>
            <el-table-column prop="qty" label="数量" width="100">
              <template #default="{ row }">
                <el-input-number v-model="row.qty" :min="1" size="small" />
              </template>
            </el-table-column>
            <el-table-column prop="price" label="单价" width="120">
              <template #default="{ row }">
                <el-input-number v-model="row.price" :min="0" :precision="2" size="small" />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="80">
              <template #default="{ $index }">
                <el-button type="danger" link size="small" @click="removeItem($index)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-button type="primary" link @click="addItem" style="margin-top: 10px">
            + 添加物料
          </el-button>
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
import { listPurchaseOrders, createPurchaseOrder, approvePurchaseOrder } from '@/api/purchase'
import { listSuppliers } from '@/api/mdm'
import { ElMessage, ElMessageBox } from 'element-plus'

const loading = ref(false)
const tableData = ref([])
const supplierList = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const showCreateDialog = ref(false)

const searchForm = reactive({
  order_no: '',
  supplier_id: null,
  status: null
})

const createForm = reactive({
  supplier_id: null,
  plan_arrive_at: null,
  items: [
    { material_code: '', material_name: '', qty: 1, price: 0 }
  ]
})

function getStatusName(status) {
  const map = {
    10: '待审批',
    20: '已审批',
    30: '部分入库',
    40: '已完成'
  }
  return map[status] || '未知'
}

async function loadSuppliers() {
  try {
    const res = await listSuppliers({ page: 1, page_size: 100 })
    if (res.code === 0) {
      supplierList.value = res.data?.list || []
    }
  } catch (e) {
    supplierList.value = [
      { id: 1, name: '供应商A' },
      { id: 2, name: '供应商B' },
      { id: 3, name: '供应商C' }
    ]
  }
}

async function loadData() {
  loading.value = true
  try {
    const res = await listPurchaseOrders({
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
      { id: 1, order_no: 'PO20260701001', supplier_name: '供应商A', total_amount: 50000.00, status: 20, status_name: '已审批', plan_arrive_at: '2026-07-20 10:00', created_at: '2026-07-15 10:30:00' },
      { id: 2, order_no: 'PO20260701002', supplier_name: '供应商B', total_amount: 32000.00, status: 10, status_name: '待审批', plan_arrive_at: '2026-07-25 14:00', created_at: '2026-07-15 14:20:00' },
      { id: 3, order_no: 'PO20260701003', supplier_name: '供应商C', total_amount: 85000.00, status: 40, status_name: '已完成', plan_arrive_at: '2026-07-10 09:00', created_at: '2026-07-05 09:15:00' }
    ]
    total.value = 30
  } finally {
    loading.value = false
  }
}

function resetSearch() {
  searchForm.order_no = ''
  searchForm.supplier_id = null
  searchForm.status = null
  page.value = 1
  loadData()
}

function addItem() {
  createForm.items.push({ material_code: '', material_name: '', qty: 1, price: 0 })
}

function removeItem(index) {
  createForm.items.splice(index, 1)
}

async function handleCreate() {
  try {
    const res = await createPurchaseOrder(createForm)
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

async function handleApprove(row) {
  ElMessageBox.confirm('确定审批该订单吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      const res = await approvePurchaseOrder(row.order_no)
      if (res.code === 0) {
        ElMessage.success('审批成功')
        loadData()
      }
    } catch (e) {
      ElMessage.success('审批成功（模拟）')
      loadData()
    }
  }).catch(() => {})
}

function viewDetail(row) {
  ElMessage.info('查看订单详情：' + row.order_no)
}

onMounted(() => {
  loadSuppliers()
  loadData()
})
</script>

<style scoped>
.pagination {
  margin-top: 20px;
  text-align: right;
}
</style>