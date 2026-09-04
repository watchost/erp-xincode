<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">采购订单</span>
      <el-button v-permission="'purchase:order:create'" type="primary" @click="showCreateDialog = true">
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
              <el-option label="草稿" :value="1" />
              <el-option label="已审批" :value="2" />
              <el-option label="部分收货" :value="5" />
              <el-option label="已完成" :value="3" />
              <el-option label="已取消" :value="4" />
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
        <el-table-column prop="status_desc" label="状态" width="100" />
        <el-table-column prop="plan_arrive_at" label="预计到货" width="160" />
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="viewDetail(row)">查看</el-button>
            <el-button
              v-if="row.status === 1"
              v-permission="'purchase:order:approve'"
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
            <el-table-column prop="material_id" label="物料" min-width="220">
              <template #default="{ row }">
                <el-select
                  v-model="row.material_id"
                  filterable
                  placeholder="请选择物料"
                  size="small"
                  style="width: 100%"
                >
                  <el-option
                    v-for="m in materialList"
                    :key="m.id"
                    :label="m.material_code + ' ' + m.name"
                    :value="m.id"
                  />
                </el-select>
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
import { listSuppliers, listMaterials } from '@/api/mdm'
import { ElMessage, ElMessageBox } from 'element-plus'

const loading = ref(false)
const tableData = ref([])
const supplierList = ref([])
const materialList = ref([])
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
    { material_id: null, qty: 1, price: 0 }
  ]
})

async function loadSuppliers() {
  try {
    const res = await listSuppliers({ page: 1, page_size: 100 })
    if (res.code === 0) {
      supplierList.value = res.data?.list || []
    }
  } catch (e) {
    supplierList.value = []
  }
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

async function loadData() {
  loading.value = true
  try {
    const res = await listPurchaseOrders({
      page: page.value,
      page_size: pageSize.value,
      ...searchForm
    })
    if (res.code === 0) {
      // 状态文案直接用后端返回的 status_desc
      tableData.value = res.data?.list || []
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
  searchForm.order_no = ''
  searchForm.supplier_id = null
  searchForm.status = null
  page.value = 1
  loadData()
}

function addItem() {
  createForm.items.push({ material_id: null, qty: 1, price: 0 })
}

function removeItem(index) {
  createForm.items.splice(index, 1)
}

async function handleCreate() {
  if (!createForm.supplier_id) {
    ElMessage.warning('请选择供应商')
    return
  }
  const validItems = createForm.items.filter(i => i.material_id && i.qty > 0 && i.price > 0)
  if (validItems.length === 0) {
    ElMessage.warning('请至少填写一行有效的物料明细（物料、数量、单价）')
    return
  }
  try {
    const res = await createPurchaseOrder({
      supplier_id: createForm.supplier_id,
      plan_arrive_at: createForm.plan_arrive_at
        ? new Date(createForm.plan_arrive_at).toISOString()
        : '',
      items: validItems.map(i => ({
        material_id: i.material_id,
        qty: i.qty,
        price: i.price
      }))
    })
    if (res.code === 0) {
      ElMessage.success('创建成功')
      showCreateDialog.value = false
      // 重置明细为一行空行，避免下次打开残留
      createForm.items = [{ material_id: null, qty: 1, price: 0 }]
      loadData()
    }
  } catch (e) {
    // 失败时拦截器已提示错误，保留对话框与表单供用户重试
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
      // 失败时拦截器已提示错误，不刷新列表
    }
  }).catch(() => {})
}

function viewDetail(row) {
  ElMessage.info('查看订单详情：' + row.order_no)
}

onMounted(() => {
  loadSuppliers()
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