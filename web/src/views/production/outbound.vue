<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">生产领料</span>
    </div>

    <el-row :gutter="20">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>领料出库扫描</span>
          </template>
          <el-form :model="outboundForm" label-width="100px">
            <el-form-item label="工单号">
              <el-select
                v-model="outboundForm.work_order_no"
                filterable
                placeholder="请输入或选择工单号"
                style="width: 100%"
              >
                <el-option
                  v-for="wo in workOrderList"
                  :key="wo.work_order_no"
                  :label="wo.work_order_no + (wo.product_name ? '（' + wo.product_name + '）' : '')"
                  :value="wo.work_order_no"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="物料编码">
              <el-input v-model="outboundForm.material_code" placeholder="扫描物料编码" size="large">
                <template #append>
                  <el-button @click="handleScan">扫描</el-button>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item label="仓库">
              <el-select
                v-model="outboundForm.warehouse_code"
                placeholder="请选择仓库"
                style="width: 100%"
                @change="onWarehouseChange"
              >
                <el-option
                  v-for="w in warehouseList"
                  :key="w.id"
                  :label="w.name"
                  :value="w.code"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="库位">
              <el-select v-model="outboundForm.location_code" placeholder="请选择库位" style="width: 100%">
                <el-option
                  v-for="l in locationList"
                  :key="l.id"
                  :label="l.location_code"
                  :value="l.location_code"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="领料数量">
              <el-input-number v-model="outboundForm.qty" :min="1" size="large" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="large" @click="handleOutbound">确认领料</el-button>
              <el-button size="large" @click="resetForm">重置</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card>
          <template #header>
            <span>领料记录</span>
          </template>
          <el-table :data="outboundRecords" style="width: 100%">
            <el-table-column prop="work_order_no" label="工单号" width="160" />
            <el-table-column prop="material_code" label="物料编码" width="120" />
            <el-table-column prop="qty" label="数量" width="80" />
            <el-table-column prop="warehouse_name" label="仓库" width="100" />
            <el-table-column prop="created_at" label="时间" width="140" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { materialIssueScan, listWorkOrders } from '@/api/production'
import { listWarehouses, listLocations } from '@/api/mdm'
import { ElMessage } from 'element-plus'

const workOrderList = ref([])
const warehouseList = ref([])
const locationList = ref([])
const outboundRecords = ref([])

const outboundForm = reactive({
  work_order_no: '',
  material_code: '',
  warehouse_code: '',
  location_code: '',
  qty: 1
})

async function loadWorkOrders() {
  try {
    const res = await listWorkOrders({ page: 1, page_size: 100 })
    if (res.code === 0) {
      workOrderList.value = res.data?.list || []
    }
  } catch (e) {
    workOrderList.value = []
  }
}

async function loadWarehouses() {
  try {
    const res = await listWarehouses()
    if (res.code === 0) {
      warehouseList.value = res.data || []
      if (warehouseList.value.length > 0) {
        outboundForm.warehouse_code = warehouseList.value[0].code
        loadLocations(warehouseList.value[0].id)
      }
    }
  } catch (e) {
    warehouseList.value = []
  }
}

async function loadLocations(warehouseId) {
  try {
    const res = await listLocations({ warehouse_id: warehouseId })
    if (res.code === 0) {
      locationList.value = res.data || []
      outboundForm.location_code = locationList.value.length > 0 ? locationList.value[0].location_code : ''
    }
  } catch (e) {
    locationList.value = []
    outboundForm.location_code = ''
  }
}

// 切换仓库：重置库位并加载该仓库下的库位
function onWarehouseChange() {
  outboundForm.location_code = ''
  const w = warehouseList.value.find(x => x.code === outboundForm.warehouse_code)
  locationList.value = []
  if (w) loadLocations(w.id)
}

function handleScan() {
  ElMessage.info('请使用扫码枪扫描物料条码')
}

async function handleOutbound() {
  if (!outboundForm.work_order_no) {
    ElMessage.warning('请选择工单号')
    return
  }
  if (!outboundForm.material_code) {
    ElMessage.warning('请输入物料编码')
    return
  }
  if (!outboundForm.warehouse_code) {
    ElMessage.warning('请选择仓库')
    return
  }
  if (!outboundForm.location_code) {
    ElMessage.warning('请选择库位')
    return
  }
  try {
    const res = await materialIssueScan({
      work_order_no: outboundForm.work_order_no,
      material_code: outboundForm.material_code,
      warehouse_code: outboundForm.warehouse_code,
      location_code: outboundForm.location_code,
      qty: outboundForm.qty
    })
    if (res.code === 0) {
      ElMessage.success('领料成功')
      addRecord()
      resetForm()
    }
  } catch (e) {
    // 失败时拦截器已提示错误，不追加记录、不重置表单，供用户重试
  }
}

function addRecord() {
  outboundRecords.value.unshift({
    work_order_no: outboundForm.work_order_no,
    material_code: outboundForm.material_code,
    qty: outboundForm.qty,
    warehouse_name: warehouseList.value.find(w => w.code === outboundForm.warehouse_code)?.name || '',
    created_at: new Date().toLocaleString()
  })
}

function resetForm() {
  outboundForm.material_code = ''
  outboundForm.qty = 1
}

onMounted(() => {
  loadWorkOrders()
  loadWarehouses()
})
</script>