<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="pda-scan-page">
    <!-- 顶部状态栏 -->
    <div class="scan-mode-header">
      <div class="mode-info">
        <span class="mode-badge check">📋 盘点录入</span>
        <span class="online-status" :class="isOnline ? 'online' : 'offline'">
          {{ isOnline ? '在线实时核对' : '离线本地暂存' }}
        </span>
      </div>
      <el-button size="small" type="primary" link @click="loadActiveStockChecks">
        <el-icon><Refresh /></el-icon> 刷新单据
      </el-button>
    </div>

    <ScanSupportBanner />

    <!-- 盘点单选择与基础信息 -->
    <div class="pda-form-card">
      <div class="form-field">
        <label class="pda-label">当前盘点单 <span class="required">*</span></label>
        <div class="barcode-input-group">
          <el-select
            v-model="selectedCheckId"
            placeholder="请选择或扫描盘点单"
            size="large"
            class="pda-select"
            no-data-text="暂无可录入的盘点单（需草稿/盘点中/待审批状态）"
            @change="onCheckSheetChange"
          >
            <el-option
              v-for="c in checkSheetList"
              :key="c.id"
              :label="`${c.check_no}（${statusLabel(c.status)}）`"
              :value="c.id"
            />
          </el-select>

          <BarcodeScanner @scan="onCheckBarcodeScanned">
            <template #trigger="{ open }">
              <el-button type="info" size="large" class="camera-btn" @click="open">
                <el-icon :size="20"><Aim /></el-icon>
              </el-button>
            </template>
          </BarcodeScanner>
        </div>
      </div>

      <!-- 盘点位置 -->
      <div class="form-field" v-if="currentCheckInfo">
        <div class="sheet-info-banner">
          <div><strong>单号：</strong>{{ currentCheckInfo.check_no }}</div>
          <div><strong>仓库：</strong>{{ warehouseName(currentCheckInfo.warehouse_id) }}</div>
          <div><strong>状态：</strong>{{ statusLabel(currentCheckInfo.status) }}</div>
          <div v-if="currentCheckInfo.remark"><strong>备注：</strong>{{ currentCheckInfo.remark }}</div>
        </div>
      </div>

      <!-- 库位选择 -->
      <div class="form-field">
        <label class="pda-label">盘点库位</label>
        <el-select
          v-model="currentLocationCode"
          placeholder="不选则按仓库整体录入"
          size="large"
          class="pda-select"
          clearable
        >
          <el-option
            v-for="l in locationList"
            :key="l.id"
            :label="l.location_code"
            :value="l.location_code"
          />
        </el-select>
      </div>

      <!-- 物料条码扫描输入：扫码只填入编码并跳到数量，实盘数确认后再提交 -->
      <div class="form-field">
        <label class="pda-label">物料编码 / 条码 <span class="required">*</span></label>
        <div class="barcode-input-group">
          <el-input
            ref="materialInputRef"
            v-model="materialCode"
            placeholder="按 PDA 扫码键或手动输入"
            size="large"
            class="pda-big-input"
            clearable
            @keyup.enter="focusQty"
          >
            <template #prefix>
              <el-icon class="input-icon"><Box /></el-icon>
            </template>
          </el-input>

          <BarcodeScanner listen-hardware @scan="onMaterialScanned">
            <template #trigger="{ open }">
              <el-button type="primary" size="large" class="camera-btn" @click="open">
                <el-icon :size="20"><Aim /></el-icon>
              </el-button>
            </template>
          </BarcodeScanner>
        </div>
      </div>

      <!-- 实盘数量步进器 -->
      <div class="form-field">
        <label class="pda-label">实盘数量 <span class="required">*</span></label>
        <div class="stepper-wrap">
          <div class="stepper-quick-btns">
            <button class="step-btn" @click="adjustQty(-10)">-10</button>
            <button class="step-btn" @click="adjustQty(-1)">-1</button>
          </div>
          <el-input-number
            ref="qtyInputRef"
            v-model="actualQty"
            :min="0"
            :max="999999"
            size="large"
            class="pda-qty-input"
            @keyup.enter="submitStockCheckItem"
          />
          <div class="stepper-quick-btns">
            <button class="step-btn plus" @click="adjustQty(1)">+1</button>
            <button class="step-btn plus" @click="adjustQty(10)">+10</button>
            <button class="step-btn plus" @click="adjustQty(50)">+50</button>
          </div>
        </div>
      </div>

      <!-- 提交盘点录入按钮 -->
      <div class="action-buttons">
        <el-button
          type="primary"
          size="large"
          class="pda-submit-btn check"
          :loading="submitting"
          @click="submitStockCheckItem"
        >
          <el-icon :size="20"><DocumentChecked /></el-icon>
          <span>录入实盘 ({{ actualQty }}件)</span>
        </el-button>

        <el-button size="large" class="pda-reset-btn" @click="resetInputs">
          重置
        </el-button>
      </div>
    </div>

    <!-- 本次盘点录入清单卡片 -->
    <div class="recent-records-card">
      <div class="records-header">
        <span class="records-title">已录入实盘项 ({{ checkItemsRecords.length }})</span>
        <el-button v-if="checkItemsRecords.length > 0" text type="primary" size="small" @click="checkItemsRecords = []">
          清空列表
        </el-button>
      </div>

      <div v-if="checkItemsRecords.length === 0" class="empty-records">
        暂无录入项，请选择盘点单并扫描物料录入实盘数
      </div>

      <div v-else class="records-list">
        <div
          v-for="(item, idx) in checkItemsRecords"
          :key="idx"
          class="record-item"
          :class="{ offline: item.isOffline }"
        >
          <div class="record-top">
            <span class="record-code">{{ item.material_code }}</span>
            <span class="record-qty">实盘: {{ item.actual_qty }}</span>
          </div>
          <div class="record-bottom">
            <span class="record-loc">{{ item.check_no }} · {{ item.location_code || '全库位' }}</span>
            <span class="record-time">{{ item.time }}</span>
          </div>
          <div v-if="item.isOffline" class="offline-tag">离线暂存</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { listStockChecks, getStockCheck, entryStockCheck } from '@/api/warehouse'
import { listLocations, listWarehouses } from '@/api/mdm'
import { isOnline } from '@/utils/pwa'
import { addOfflineQueueItem, isNetworkError, newIdempotencyKey } from '@/utils/pdaOffline'
import { buildStockCheckEntry, apiErrorMessage } from '@/utils/pdaResolve'
import { playSuccessSound, playErrorSound } from '@/utils/sound'
import BarcodeScanner from '@/components/pda/BarcodeScanner.vue'
import ScanSupportBanner from '@/components/pda/ScanSupportBanner.vue'
import { ElMessage } from 'element-plus'

// 后端仅允许 草稿(10)/盘点中(20)/已录入待审批(30) 状态录入实盘
const ENTRY_STATUSES = [10, 20, 30]
const STATUS_TEXT = { 10: '草稿', 20: '盘点中', 30: '待审批', 40: '已完成', 50: '已取消' }

const checkSheetList = ref([])
const selectedCheckId = ref(null)
const currentCheckInfo = ref(null)
const warehouseList = ref([])
const locationList = ref([])
const currentLocationCode = ref('')
const materialCode = ref('')
const actualQty = ref(0)
const materialInputRef = ref(null)
const qtyInputRef = ref(null)
const submitting = ref(false)
const checkItemsRecords = ref([])

function statusLabel(status) {
  return STATUS_TEXT[status] || `状态${status}`
}

function warehouseName(id) {
  return warehouseList.value.find((w) => w.id === id)?.name || `仓库#${id}`
}

async function loadWarehouses() {
  try {
    const res = await listWarehouses()
    warehouseList.value = res.data || []
  } catch (e) {
    warehouseList.value = []
  }
}

async function loadActiveStockChecks() {
  try {
    const res = await listStockChecks({ page: 1, page_size: 100 })
    const list = res.data?.list || []
    checkSheetList.value = list.filter((c) => ENTRY_STATUSES.includes(c.status))
    const stillValid = checkSheetList.value.some((c) => c.id === selectedCheckId.value)
    if (!stillValid) {
      selectedCheckId.value = checkSheetList.value[0]?.id ?? null
      currentCheckInfo.value = null
      if (selectedCheckId.value) onCheckSheetChange()
    }
  } catch (e) {
    checkSheetList.value = []
  }
}

async function onCheckSheetChange() {
  if (!selectedCheckId.value) return
  currentLocationCode.value = ''
  const fromList = checkSheetList.value.find((c) => c.id === selectedCheckId.value) || null
  try {
    const res = await getStockCheck(selectedCheckId.value)
    currentCheckInfo.value = res.data || fromList
  } catch (e) {
    currentCheckInfo.value = fromList
  }
  if (currentCheckInfo.value?.warehouse_id) {
    loadLocations(currentCheckInfo.value.warehouse_id)
  }
}

async function loadLocations(warehouseId) {
  try {
    const res = await listLocations({ warehouse_id: warehouseId })
    locationList.value = res.data || []
  } catch (e) {
    locationList.value = []
  }
}

function onCheckBarcodeScanned(code) {
  const v = code.trim()
  const matched = checkSheetList.value.find((c) => c.check_no === v)
  if (matched) {
    selectedCheckId.value = matched.id
    onCheckSheetChange()
    playSuccessSound()
    ElMessage.success(`已切换至盘点单：${matched.check_no}`)
  } else {
    playErrorSound()
    ElMessage.warning(`未找到可录入的盘点单 [${v}]`)
  }
}

function onMaterialScanned(code) {
  materialCode.value = code.trim()
  focusQty()
}

function focusQty() {
  if (!materialCode.value.trim()) return
  nextTick(() => qtyInputRef.value?.focus())
}

function adjustQty(delta) {
  actualQty.value = Math.max(0, (actualQty.value || 0) + delta)
}

async function submitStockCheckItem() {
  if (submitting.value) return
  if (!selectedCheckId.value || !currentCheckInfo.value) {
    playErrorSound()
    ElMessage.warning('请先选择盘点单')
    return
  }
  const code = materialCode.value.trim()
  if (!code) {
    playErrorSound()
    ElMessage.warning('请扫描或输入物料编码')
    materialInputRef.value?.focus()
    return
  }
  if (actualQty.value === null || actualQty.value === undefined || actualQty.value < 0) {
    playErrorSound()
    ElMessage.warning('请输入实盘数量')
    return
  }

  // 编码型原始记录：离线时原样入队，同步时再解析成 material_id / location_id
  const raw = {
    check_id: selectedCheckId.value,
    check_no: currentCheckInfo.value.check_no,
    warehouse_id: currentCheckInfo.value.warehouse_id,
    material_code: code,
    location_code: currentLocationCode.value || '',
    actual_qty: actualQty.value
  }
  const title = `盘点[${raw.check_no}]: ${code} 实盘=${raw.actual_qty}`
  const idemKey = newIdempotencyKey()

  submitting.value = true
  try {
    if (!isOnline.value) {
      queueOffline(raw, title, idemKey, '当前离线，实盘录入已暂存到离线队列，联网后请同步')
      return
    }
    const body = await buildStockCheckEntry(raw)
    await entryStockCheck(raw.check_id, body, idemKey)
    playSuccessSound()
    ElMessage.success('实盘录入成功')
    pushCheckRecord(raw, false)
    postSubmitClean()
  } catch (err) {
    if (isNetworkError(err)) {
      queueOffline(raw, title, idemKey, '网络中断，实盘录入已暂存到离线队列，联网后请同步')
    } else {
      playErrorSound()
      ElMessage.error(`录入失败：${apiErrorMessage(err)}`)
    }
  } finally {
    submitting.value = false
  }
}

function queueOffline(raw, title, idemKey, msg) {
  addOfflineQueueItem('stock_check', title, raw, idemKey)
  playSuccessSound()
  ElMessage.warning(msg)
  pushCheckRecord(raw, true)
  postSubmitClean()
}

function pushCheckRecord(raw, isOffline) {
  checkItemsRecords.value.unshift({
    material_code: raw.material_code,
    actual_qty: raw.actual_qty,
    check_no: raw.check_no,
    location_code: raw.location_code,
    time: new Date().toLocaleTimeString(),
    isOffline
  })
}

function postSubmitClean() {
  materialCode.value = ''
  actualQty.value = 0
  nextTick(() => materialInputRef.value?.focus())
}

function resetInputs() {
  materialCode.value = ''
  actualQty.value = 0
}

onMounted(() => {
  loadWarehouses()
  loadActiveStockChecks()
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

      &.check {
        background: #fff7e6;
        color: #fa8c16;
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

.sheet-info-banner {
  background: #fafafa;
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  padding: 10px 12px;
  font-size: 12px;
  color: #595959;
  display: flex;
  flex-direction: column;
  gap: 4px;
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
    background: #fa8c16;
    border-color: #fa8c16;
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
      width: 40px;
      height: 48px;
      border-radius: 8px;
      border: 1px solid #d9d9d9;
      background: #fafafa;
      font-size: 12px;
      font-weight: 700;
      color: #595959;
      cursor: pointer;

      &:active {
        background: #e8e8e8;
      }

      &.plus {
        background: #fff7e6;
        border-color: #ffd591;
        color: #fa8c16;
        &:active {
          background: #ffe7ba;
        }
      }
    }
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

    &.check {
      background: linear-gradient(135deg, #fa8c16 0%, #d46b08 100%);
      border: none;
      box-shadow: 0 4px 12px rgba(250, 140, 22, 0.35);
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
          font-size: 14px;
          color: #fa8c16;
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
    }
  }
}
</style>
