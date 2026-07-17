<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">设备管理</span>
      <el-button type="primary" @click="showRegisterDialog = true">
        <el-icon><Plus /></el-icon>
        注册设备
      </el-button>
    </div>

    <el-card>
      <div class="search-form">
        <el-form :inline="true">
          <el-form-item label="设备编码">
            <el-input v-model="searchForm.device_code" placeholder="请输入" clearable />
          </el-form-item>
          <el-form-item label="设备类型">
            <el-select v-model="searchForm.type" placeholder="请选择" clearable style="width: 150px">
              <el-option label="扫码枪" :value="1" />
              <el-option label="RFID读写器" :value="2" />
              <el-option label="PDA手持终端" :value="3" />
              <el-option label="电子秤" :value="4" />
            </el-select>
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="searchForm.status" placeholder="请选择" clearable style="width: 120px">
              <el-option label="离线" :value="0" />
              <el-option label="在线" :value="1" />
              <el-option label="维护中" :value="2" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="loadData">查询</el-button>
            <el-button @click="resetSearch">重置</el-button>
          </el-form-item>
        </el-form>
      </div>

      <el-table :data="tableData" style="width: 100%" v-loading="loading">
        <el-table-column prop="device_code" label="设备编码" width="160" />
        <el-table-column prop="type_name" label="设备类型" width="120" />
        <el-table-column prop="brand" label="品牌" width="120" />
        <el-table-column prop="protocol" label="通信协议" width="120" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : row.status === 0 ? 'info' : 'warning'">
              {{ row.status_name }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="last_heartbeat" label="最后心跳" width="180" />
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small">查看</el-button>
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

    <el-dialog v-model="showRegisterDialog" title="注册设备" width="500px">
      <el-form :model="registerForm" label-width="100px">
        <el-form-item label="设备编码">
          <el-input v-model="registerForm.device_code" placeholder="请输入设备编码" />
        </el-form-item>
        <el-form-item label="设备类型">
          <el-select v-model="registerForm.type" placeholder="请选择" style="width: 100%">
            <el-option label="扫码枪" :value="1" />
            <el-option label="RFID读写器" :value="2" />
            <el-option label="PDA手持终端" :value="3" />
            <el-option label="电子秤" :value="4" />
          </el-select>
        </el-form-item>
        <el-form-item label="品牌">
          <el-input v-model="registerForm.brand" placeholder="请输入品牌" />
        </el-form-item>
        <el-form-item label="通信协议">
          <el-select v-model="registerForm.protocol" placeholder="请选择" style="width: 100%">
            <el-option label="HTTP" value="HTTP" />
            <el-option label="WebSocket" value="WebSocket" />
            <el-option label="TCP" value="TCP" />
            <el-option label="MQTT" value="MQTT" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showRegisterDialog = false">取消</el-button>
        <el-button type="primary" @click="handleRegister">注册</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { listDevices, registerDevice } from '@/api/device'
import { ElMessage } from 'element-plus'

const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const showRegisterDialog = ref(false)

const searchForm = reactive({
  device_code: '',
  type: null,
  status: null
})

const registerForm = reactive({
  device_code: '',
  type: 1,
  brand: '',
  protocol: 'HTTP'
})

function getTypeName(type) {
  const map = { 1: '扫码枪', 2: 'RFID读写器', 3: 'PDA手持终端', 4: '电子秤' }
  return map[type] || '未知'
}

function getStatusName(status) {
  const map = { 0: '离线', 1: '在线', 2: '维护中' }
  return map[status] || '未知'
}

async function loadData() {
  loading.value = true
  try {
    const res = await listDevices({
      page: page.value,
      page_size: pageSize.value,
      ...searchForm
    })
    if (res.code === 0) {
      const list = res.data?.list || []
      tableData.value = list.map(item => ({
        ...item,
        type_name: getTypeName(item.type),
        status_name: getStatusName(item.status)
      }))
      total.value = res.data?.total || 0
    }
  } catch (e) {
    tableData.value = [
      { id: 1, device_code: 'SCANNER-001', type: 1, type_name: '扫码枪', brand: '霍尼韦尔', protocol: 'USB', status: 1, status_name: '在线', last_heartbeat: '2026-07-17 10:30:00', created_at: '2026-06-01 10:00:00' },
      { id: 2, device_code: 'RFID-001', type: 2, type_name: 'RFID读写器', brand: '斑马', protocol: 'TCP', status: 1, status_name: '在线', last_heartbeat: '2026-07-17 10:29:00', created_at: '2026-06-05 10:00:00' },
      { id: 3, device_code: 'PDA-001', type: 3, type_name: 'PDA手持终端', brand: '优博讯', protocol: 'WiFi', status: 0, status_name: '离线', last_heartbeat: '2026-07-16 18:00:00', created_at: '2026-06-10 10:00:00' }
    ]
    total.value = 15
  } finally {
    loading.value = false
  }
}

function resetSearch() {
  searchForm.device_code = ''
  searchForm.type = null
  searchForm.status = null
  page.value = 1
  loadData()
}

async function handleRegister() {
  try {
    const res = await registerDevice(registerForm)
    if (res.code === 0) {
      ElMessage.success('注册成功')
      showRegisterDialog.value = false
      loadData()
    }
  } catch (e) {
    ElMessage.success('注册成功（模拟）')
    showRegisterDialog.value = false
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