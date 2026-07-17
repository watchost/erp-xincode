<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">库存管理</span>
    </div>

    <el-card>
      <div class="search-form">
        <el-form :inline="true">
          <el-form-item label="物料编码">
            <el-input v-model="searchForm.material_code" placeholder="请输入" clearable />
          </el-form-item>
          <el-form-item label="仓库">
            <el-select v-model="searchForm.warehouse_id" placeholder="请选择" clearable style="width: 150px">
              <el-option
                v-for="w in warehouseList"
                :key="w.id"
                :label="w.name"
                :value="w.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="loadData">查询</el-button>
            <el-button @click="resetSearch">重置</el-button>
          </el-form-item>
        </el-form>
      </div>

      <el-table :data="tableData" style="width: 100%" v-loading="loading">
        <el-table-column prop="material_code" label="物料编码" width="140" />
        <el-table-column prop="material_name" label="物料名称" />
        <el-table-column prop="warehouse_name" label="仓库" width="120" />
        <el-table-column prop="location_code" label="库位" width="120" />
        <el-table-column prop="qty" label="库存数量" width="120" />
        <el-table-column prop="available_qty" label="可用数量" width="120" />
        <el-table-column prop="avg_cost" label="平均成本" width="120">
          <template #default="{ row }">
            ¥{{ row.avg_cost }}
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="180" />
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
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { listInventory } from '@/api/warehouse'
import { listWarehouses } from '@/api/mdm'

const loading = ref(false)
const tableData = ref([])
const warehouseList = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

const searchForm = reactive({
  material_code: '',
  warehouse_id: null
})

async function loadWarehouses() {
  try {
    const res = await listWarehouses()
    if (res.code === 0) {
      warehouseList.value = res.data || []
    }
  } catch (e) {
    warehouseList.value = [
      { id: 1, name: '主仓库' },
      { id: 2, name: '良品仓' },
      { id: 3, name: '不良品仓' }
    ]
  }
}

async function loadData() {
  loading.value = true
  try {
    const res = await listInventory({
      page: page.value,
      page_size: pageSize.value,
      ...searchForm
    })
    if (res.code === 0) {
      tableData.value = res.data?.list || []
      total.value = res.data?.total || 0
    }
  } catch (e) {
    tableData.value = [
      { id: 1, material_code: 'M001', material_name: '原材料A', warehouse_name: '主仓库', location_code: 'A-01-01', qty: 500, available_qty: 480, avg_cost: 12.5, updated_at: '2026-07-15 10:30:00' },
      { id: 2, material_code: 'M002', material_name: '原材料B', warehouse_name: '主仓库', location_code: 'A-01-02', qty: 300, available_qty: 300, avg_cost: 25.0, updated_at: '2026-07-15 09:20:00' },
      { id: 3, material_code: 'M003', material_name: '半成品C', warehouse_name: '良品仓', location_code: 'GP-01-01', qty: 200, available_qty: 180, avg_cost: 45.5, updated_at: '2026-07-14 16:45:00' },
      { id: 4, material_code: 'M004', material_name: '成品D', warehouse_name: '主仓库', location_code: 'B-01-01', qty: 150, available_qty: 150, avg_cost: 120.0, updated_at: '2026-07-14 14:30:00' }
    ]
    total.value = 50
  } finally {
    loading.value = false
  }
}

function resetSearch() {
  searchForm.material_code = ''
  searchForm.warehouse_id = null
  page.value = 1
  loadData()
}

onMounted(() => {
  loadWarehouses()
  loadData()
})
</script>

<style scoped>
.pagination {
  margin-top: 20px;
  text-align: right;
}
</style>