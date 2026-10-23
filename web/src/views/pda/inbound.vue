<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="pda-scan-page">
    <!-- 顶部模式状态条 -->
    <div class="scan-mode-header">
      <div class="mode-info">
        <span class="mode-badge inbound">📥 入库扫码</span>
        <span class="online-status" :class="isOnline ? 'online' : 'offline'">
          {{ isOnline ? '在线实时过账' : '离线本地暂存' }}
        </span>
      </div>
      <div class="auto-submit-switch">
        <span class="switch-label">扫码自动提交</span>
        <el-switch v-model="autoSubmit" size="small" />
      </div>
    </div>

    <!-- 扫码录入核心表单区 -->
    <div class="pda-form-card">
      <!-- 仓库与库位选择 -->
      <div class="form-row-grid">
        <div class="form-field">
          <label class="pda-label">目标仓库 <span class="required">*</span></label>
          <el-select
            v-model="form.warehouse_code"
            placeholder="选择仓库"
            size="large"
            class="pda-select"
            @change="onWarehouseChange"
          >
            <el-option
              v-for="w in warehouseList"
              :key="w.id"
              :label="w.name"
              :value="w.code"
            />
          </el-select>
        </div>

        <div class="form-field">
          <label class="pda-label">存放库位 <span class="required">*</span></label>
          <el-select
            v-model="form.location_code"
            placeholder="选择库位"
            size="large"
            class="pda-select"
          >
            <el-option
              v-for="l in locationList"
              :key="l.id"
              :label="l.location_code"
              :value="l.location_code"
            />
          </el-select>
        </div>
      </div>

      <!-- 物料条码扫描输入 -->
      <div class="form-field">
        <label class="pda-label">物料编码 / 条码 <span class="required">*</span></label>
        <div class="barcode-input-group">
          <el-input
            ref="materialInputRef"
            v-model="form.material_code"
            placeholder="对准条码扫码或手动输入"
            size="large"
            class="pda-big-input"
            clearable
            @keyup.enter="handleEnterKey"
          >
            <template #prefix>
              <el-icon class="input-icon"><Box /></el-icon>
            </template>
          </el-input>

          <BarcodeScanner @scan="onBarcodeScanned">
            <template #trigger="{ open }">
              <el-button type="primary" size="large" class="camera-btn" @click="open">
                <el-icon :size="20"><Aim /></el-icon>
              </el-button>
            </template>
          </BarcodeScanner>
        </div>
      </div>

      <!-- 数量步进控制器（大尺寸触摸） -->
      <div class="form-field">
        <label class="pda-label">入库数量 <span class="required">*</span></label>
        <div class="stepper-wrap">
          <div class="stepper-quick-btns">
            <button class="step-btn" @click="adjustQty(-10)">-10</button>
            <button class="step-btn" @click="adjustQty(-1)">-1</button>
          </div>
          <el-input-number
            v-model="form.qty"
            :min="1"
            :max="999999"
            size="large"
            class="pda-qty-input"
          />
          <div class="stepper-quick-btns">
            <button class="step-btn plus" @click="adjustQty(1)">+1</button>
            <button class="step-btn plus" @click="adjustQty(10)">+10</button>
          </div>
        </div>
      </div>

      <!-- 单价与辅助信息（折叠式选填） -->
      <div class="advanced-section">
        <div class="adv-toggle" @click="showAdvanced = !showAdvanced">
          <span>更多参数（单价/备注）</span>
          <el-icon><component :is="showAdvanced ? 'ArrowUp' : 'ArrowDown'" /></el-icon>
        </div>
        <div v-show="showAdvanced" class="adv-body">
          <div class="form-field">
            <label class="pda-label">采购单价 (元)</label>
            <el-input-number
              v-model="form.unit_cost"
              :min="0"
              :precision="2"
              size="large"
              placeholder="0.00"
              style="width: 100%"
            />
          </div>
        </div>
      </div>

      <!-- 底部大尺寸提交按钮 -->
      <div class="action-buttons">
        <el-button
          type="primary"
          size="large"
          class="pda-submit-btn inbound"
          :loading="submitting"
          @click="submitInbound"
        >
          <el-icon :size="20"><CircleCheckFilled /></el-icon>
          <span>确认入库 ({{ form.qty }}件)</span>
        </el-button>

        <el-button size="large" class="pda-reset-btn" @click="resetForm">
          重置
        </el-button>
      </div>
    </div>

    <!-- 最近入库作业流水卡片 -->
    <div class="recent-records-card">
      <div class="records-header">
        <span class="records-title">本次作业记录 ({{ recentRecords.length }})</span>
        <el-button v-if="recentRecords.length > 0" text type="primary" size="small" @click="recentRecords = []">
          清空记录
        </el-button>
      </div>

      <div v-if="recentRecords.length === 0" class="empty-records">
        暂无扫码记录，请扫描条码开始入库作业
      </div>

      <div v-else class="records-list">
        <div
          v-for="(item, idx) in recentRecords"
          :key="idx"
          class="record-item"
          :class="{ offline: item.isOffline }"
        >
          <div class="record-top">
            <span class="record-code">{{ item.material_code }}</span>
            <span class="record-qty">+{{ item.qty }}</span>
          </div>
          <div class="record-bottom">
            <span class="record-loc">{{ item.warehouse_code }} / {{ item.location_code }}</span>
            <span class="record-time">{{ item.time }}</span>
          </div>
          <div v-if="item.isOffline" class="offline-tag">离线暂存</div>
          <div v-else-if="item.afterQty !== undefined" class="after-qty-tag">
            结存: {{ item.afterQty }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, nextTick } from 'vue'
import { listWarehouses, listLocations } from '@/api/mdm'
import { inboundScan } from '@/api/warehouse'
import { isOnline } from '@/utils/pwa'
import { addOfflineQueueItem } from '@/utils/pdaOffline'
import { playSuccessSound, playErrorSound } from '@/utils/sound'
import BarcodeScanner from '@/components/pda/BarcodeScanner.vue'
import { ElMessage } from 'element-plus'

const warehouseList = ref([])
const locationList = ref([])
const materialInputRef = ref(null)
const autoSubmit = ref(true)
const showAdvanced = ref(false)
const submitting = ref(false)
const recentRecords = ref([])

const form = reactive({
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
        form.warehouse_code = warehouseList.value[0].code
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
      form.location_code = locationList.value.length > 0 ? locationList.value[0].location_code : ''
    }
  } catch (e) {
    locationList.value = []
    form.location_code = ''
  }
}

function onWarehouseChange() {
  form.location_code = ''
  const w = warehouseList.value.find((x) => x.code === form.warehouse_code)
  if (w) loadLocations(w.id)
}

function adjustQty(delta) {
  form.qty = Math.max(1, form.qty + delta)
}

function onBarcodeScanned(code) {
  form.material_code = code.trim()
  if (autoSubmit.value) {
    submitInbound()
  }
}

function handleEnterKey() {
  if (form.material_code) {
    submitInbound()
  }
}

async function submitInbound() {
  const materialCode = form.material_code.trim()
  if (!materialCode) {
    playErrorSound()
    ElMessage.warning('请先扫描或输入物料编码')
    materialInputRef.value?.focus()
    return
  }
  if (!form.warehouse_code) {
    playErrorSound()
    ElMessage.warning('请选择目标仓库')
    return
  }
  if (!form.location_code) {
    playErrorSound()
    ElMessage.warning('请选择存放库位')
    return
  }

  const payload = {
    material_code: materialCode,
    warehouse_code: form.warehouse_code,
    location_code: form.location_code,
    qty: form.qty,
    unit_cost: form.unit_cost || 0
  }

  submitting.value = true

  // 若当前离线，直接写入离线队列
  if (!isOnline.value) {
    addOfflineQueueItem('inbound', `入库: ${materialCode} × ${form.qty}`, payload)
    playSuccessSound()
    ElMessage.success('当前处于离线模式，入库记录已暂存至离线队列')
    pushRecentRecord(materialCode, form.qty, true)
    postSubmitClean()
    submitting.value = false
    return
  }

  try {
    const res = await inboundScan(payload)
    if (res.code === 0) {
      playSuccessSound()
      ElMessage.success(`入库成功！库存增加至 ${res.data?.after_qty || form.qty}`)
      pushRecentRecord(materialCode, form.qty, false, res.data?.after_qty)
      postSubmitClean()
    }
  } catch (err) {
    playErrorSound()
    // 网络异常时支持转存离线队列
    if (!navigator.onLine || err.message?.includes('Network Error')) {
      addOfflineQueueItem('inbound', `入库: ${materialCode} × ${form.qty}`, payload)
      ElMessage.warning('网络中断，入库记录已自动保存至离线队列')
      pushRecentRecord(materialCode, form.qty, true)
      postSubmitClean()
    }
  } finally {
    submitting.value = false
  }
}

function pushRecentRecord(material_code, qty, isOffline, afterQty) {
  recentRecords.value.unshift({
    material_code,
    qty,
    warehouse_code: form.warehouse_code,
    location_code: form.location_code,
    time: new Date().toLocaleTimeString(),
    isOffline,
    afterQty
  })
}

function postSubmitClean() {
  form.material_code = ''
  form.qty = 1
  nextTick(() => {
    materialInputRef.value?.focus()
  })
}

function resetForm() {
  form.material_code = ''
  form.qty = 1
  form.unit_cost = 0
}

onMounted(() => {
  loadWarehouses()
})
</script>

<style scoped lang="scss">
.pda-scan-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.scan-mode-header {
  background: #ffffff;
  border-radius: 10px;
  padding: 10px 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);

  .mode-info {
    display: flex;
    align-items: center;
    gap: 8px;

    .mode-badge {
      font-size: 13px;
      font-weight: 700;
      padding: 3px 8px;
      border-radius: 6px;

      &.inbound {
        background: #e6f7ff;
        color: #1890ff;
      }
    }

    .online-status {
      font-size: 11px;
      font-weight: 500;

      &.online {
        color: #52c41a;
      }
      &.offline {
        color: #ff4d4f;
      }
    }
  }

  .auto-submit-switch {
    display: flex;
    align-items: center;
    gap: 6px;
    .switch-label {
      font-size: 12px;
      color: #595959;
    }
  }
}

.pda-form-card {
  background: #ffffff;
  border-radius: 14px;
  padding: 16px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-row-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;

  .pda-label {
    font-size: 13px;
    font-weight: 600;
    color: #1f2329;

    .required {
      color: #ff4d4f;
    }
  }

  .pda-select {
    width: 100%;
    :deep(.el-input__wrapper) {
      height: 44px;
      border-radius: 8px;
    }
  }
}

.barcode-input-group {
  display: flex;
  gap: 8px;

  .pda-big-input {
    flex: 1;
    :deep(.el-input__wrapper) {
      height: 48px;
      font-size: 16px;
      font-weight: 600;
      border-radius: 8px;
    }
  }

  .camera-btn {
    width: 48px;
    height: 48px;
    padding: 0;
    border-radius: 8px;
    background: #1890ff;
  }
}

.stepper-wrap {
  display: flex;
  align-items: center;
  gap: 8px;

  .pda-qty-input {
    flex: 1;
    :deep(.el-input__wrapper) {
      height: 48px;
      font-size: 18px;
      font-weight: 700;
      border-radius: 8px;
    }
  }

  .stepper-quick-btns {
    display: flex;
    gap: 4px;

    .step-btn {
      width: 42px;
      height: 48px;
      border-radius: 8px;
      border: 1px solid #d9d9d9;
      background: #fafafa;
      font-size: 13px;
      font-weight: 700;
      color: #595959;
      cursor: pointer;

      &:active {
        background: #e8e8e8;
      }

      &.plus {
        background: #e6f7ff;
        border-color: #91d5ff;
        color: #1890ff;
        &:active {
          background: #bae7ff;
        }
      }
    }
  }
}

.advanced-section {
  border-top: 1px dashed #ebeef5;
  padding-top: 10px;

  .adv-toggle {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 12px;
    color: #8c8c8c;
    cursor: pointer;
  }

  .adv-body {
    margin-top: 10px;
  }
}

.action-buttons {
  display: flex;
  gap: 10px;
  margin-top: 4px;

  .pda-submit-btn {
    flex: 1;
    height: 52px;
    font-size: 16px;
    font-weight: 700;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;

    &.inbound {
      background: linear-gradient(135deg, #1890ff 0%, #096dd9 100%);
      border: none;
      box-shadow: 0 4px 12px rgba(24, 144, 255, 0.35);
    }

    &:active {
      transform: scale(0.98);
    }
  }

  .pda-reset-btn {
    width: 70px;
    height: 52px;
    border-radius: 10px;
    font-size: 14px;
  }
}

.recent-records-card {
  background: #ffffff;
  border-radius: 14px;
  padding: 14px 16px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);

  .records-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 10px;

    .records-title {
      font-size: 14px;
      font-weight: 600;
      color: #1f2329;
    }
  }

  .empty-records {
    text-align: center;
    color: #8c8c8c;
    font-size: 12px;
    padding: 20px 0;
  }

  .records-list {
    display: flex;
    flex-direction: column;
    gap: 8px;

    .record-item {
      background: #f5f7fa;
      border-radius: 8px;
      padding: 10px 12px;
      position: relative;

      &.offline {
        border-left: 4px solid #faad14;
      }

      .record-top {
        display: flex;
        justify-content: space-between;
        font-weight: 600;

        .record-code {
          font-size: 14px;
          color: #1f2329;
        }

        .record-qty {
          font-size: 15px;
          color: #52c41a;
        }
      }

      .record-bottom {
        display: flex;
        justify-content: space-between;
        font-size: 11px;
        color: #8c8c8c;
        margin-top: 4px;
      }

      .offline-tag {
        position: absolute;
        top: 8px;
        right: 60px;
        font-size: 10px;
        background: #fffbe6;
        color: #d46b08;
        border: 1px solid #ffe58f;
        padding: 1px 4px;
        border-radius: 4px;
      }

      .after-qty-tag {
        font-size: 11px;
        color: #1890ff;
        margin-top: 3px;
      }
    }
  }
}
</style>
