<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">入库作业</span>
    </div>

    <el-row :gutter="20">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>扫码入库</span>
          </template>
          <el-form :model="inboundForm" label-width="100px">
            <el-form-item label="物料编码">
              <el-input v-model="inboundForm.material_code" placeholder="扫描或输入物料编码" size="large">
                <template #append>
                  <el-button @click="handleScan">扫描</el-button>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item label="仓库">
              <el-select
                v-model="inboundForm.warehouse_code"
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
              <el-select v-model="inboundForm.location_code" placeholder="请选择库位" style="width: 100%">
                <el-option
                  v-for="l in locationList"
                  :key="l.id"
                  :label="l.location_code"
                  :value="l.location_code"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="入库数量">
              <el-input-number v-model="inboundForm.qty" :min="1" size="large" />
            </el-form-item>
            <el-form-item label="成本单价">
              <el-input-number v-model="inboundForm.unit_cost" :min="0" :precision="2" size="large" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="large" @click="handleInbound">确认入库</el-button>
              <el-button size="large" @click="resetForm">重置</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card>
          <template #header>
            <span>入库记录</span>
          </template>
          <el-table :data="inboundRecords" style="width: 100%">
            <el-table-column prop="material_code" label="物料编码" width="140" />
            <el-table-column prop="material_name" label="物料名称" />
            <el-table-column prop="change_qty" label="数量" width="100" />
            <el-table-column prop="warehouse_name" label="仓库" width="100" />
            <el-table-column prop="created_at" label="时间" width="160" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { inboundScan } from '@/api/warehouse'
import { listWarehouses, listLocations } from '@/api/mdm'
import { ElMessage } from 'element-plus'

const warehouseList = ref([])
const locationList = ref([])
const inboundRecords = ref([])

const inboundForm = reactive({
  material_code: '',
  warehouse_code: '',
  location_code: '',
  qty: 1,
  unit_cost: 0
})

async function loadWarehouses() {
  try {
    const res = await listWarehouses()
    if (res.code === 0) {
      warehouseList.value = res.data || []
      if (warehouseList.value.length > 0) {
        inboundForm.warehouse_code = warehouseList.value[0].code
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
      // 切换仓库后默认选中第一个库位，避免提交空库位
      inboundForm.location_code = locationList.value.length > 0 ? locationList.value[0].location_code : ''
    }
  } catch (e) {
    locationList.value = []
    inboundForm.location_code = ''
  }
}

// 切换仓库：重置库位并加载该仓库下的库位
function onWarehouseChange() {
  inboundForm.location_code = ''
  const w = warehouseList.value.find(x => x.code === inboundForm.warehouse_code)
  locationList.value = []
  if (w) loadLocations(w.id)
}

function handleScan() {
  ElMessage.info('请使用扫码枪扫描物料条码')
}

async function handleInbound() {
  if (!inboundForm.material_code) {
    ElMessage.warning('请输入物料编码')
    return
  }
  if (!inboundForm.warehouse_code) {
    ElMessage.warning('请选择仓库')
    return
  }
  if (!inboundForm.location_code) {
    ElMessage.warning('请选择库位')
    return
  }
  try {
    const res = await inboundScan({
      material_code: inboundForm.material_code,
      warehouse_code: inboundForm.warehouse_code,
      location_code: inboundForm.location_code,
      qty: inboundForm.qty,
      unit_cost: inboundForm.unit_cost
    })
    if (res.code === 0) {
      ElMessage.success('入库成功')
      inboundRecords.value.unshift({
        material_code: inboundForm.material_code,
        material_name: res.data?.matched ? inboundForm.material_code : '物料',
        change_qty: inboundForm.qty,
        warehouse_name: warehouseList.value.find(w => w.code === inboundForm.warehouse_code)?.name || '',
        created_at: new Date().toLocaleString()
      })
      resetForm()
    }
  } catch (e) {
    // 失败时拦截器已提示错误，不追加记录、不重置表单，供用户重试
  }
}

function resetForm() {
  inboundForm.material_code = ''
  inboundForm.qty = 1
  inboundForm.unit_cost = 0
}

onMounted(() => {
  loadWarehouses()
})
</script>