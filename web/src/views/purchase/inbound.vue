<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">采购入库</span>
    </div>

    <el-row :gutter="20">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>采购入库扫描</span>
          </template>
          <el-form :model="inboundForm" label-width="100px">
            <el-form-item label="采购订单号">
              <el-select
                v-model="inboundForm.order_no"
                filterable
                placeholder="请输入或选择订单号"
                style="width: 100%"
                @change="onOrderChange"
              >
                <el-option
                  v-for="order in orderList"
                  :key="order.order_no"
                  :label="order.order_no"
                  :value="order.order_no"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="物料编码">
              <el-input v-model="inboundForm.material_code" placeholder="扫描物料编码" size="large">
                <template #append>
                  <el-button @click="handleScan">扫描</el-button>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item label="仓库">
              <el-select v-model="inboundForm.warehouse_id" placeholder="请选择仓库" style="width: 100%">
                <el-option
                  v-for="w in warehouseList"
                  :key="w.id"
                  :label="w.name"
                  :value="w.id"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="入库数量">
              <el-input-number v-model="inboundForm.qty" :min="0" size="large" />
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
            <el-table-column prop="order_no" label="订单号" width="160" />
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
import { purchaseInboundScan } from '@/api/purchase'
import { listWarehouses } from '@/api/mdm'
import { ElMessage } from 'element-plus'

const orderList = ref([])
const warehouseList = ref([])
const inboundRecords = ref([])

const inboundForm = reactive({
  order_no: '',
  material_code: '',
  warehouse_id: null,
  qty: 1
})

async function loadOrders() {
  orderList.value = [
    { order_no: 'PO20260701001' },
    { order_no: 'PO20260701002' },
    { order_no: 'PO20260701003' }
  ]
}

async function loadWarehouses() {
  try {
    const res = await listWarehouses()
    if (res.code === 0) {
      warehouseList.value = res.data || []
    }
  } catch (e) {
    warehouseList.value = [
      { id: 1, name: '主仓库' },
      { id: 2, name: '良品仓' }
    ]
    inboundForm.warehouse_id = 1
  }
}

function onOrderChange() {
  ElMessage.info('已选择订单：' + inboundForm.order_no)
}

function handleScan() {
  ElMessage.info('请使用扫码枪扫描物料条码')
}

async function handleInbound() {
  if (!inboundForm.order_no) {
    ElMessage.warning('请选择采购订单')
    return
  }
  if (!inboundForm.material_code) {
    ElMessage.warning('请输入物料编码')
    return
  }
  try {
    const res = await purchaseInboundScan(inboundForm)
    if (res.code === 0) {
      ElMessage.success('入库成功')
      addRecord()
      resetForm()
    }
  } catch (e) {
    ElMessage.success('入库成功（模拟）')
    addRecord()
    resetForm()
  }
}

function addRecord() {
  inboundRecords.value.unshift({
    order_no: inboundForm.order_no,
    material_code: inboundForm.material_code,
    qty: inboundForm.qty,
    warehouse_name: warehouseList.value.find(w => w.id === inboundForm.warehouse_id)?.name || '',
    created_at: new Date().toLocaleString()
  })
}

function resetForm() {
  inboundForm.material_code = ''
  inboundForm.qty = 1
}

onMounted(() => {
  loadOrders()
  loadWarehouses()
})
</script>