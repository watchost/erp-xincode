<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="pda-inventory-page">
    <ScanSupportBanner />
    <!-- 快捷搜索与扫码栏 -->
    <div class="search-card">
      <div class="search-input-group">
        <el-input
          ref="searchInputRef"
          v-model="searchQuery"
          placeholder="扫描物料/库位条码或输入关键字"
          size="large"
          class="pda-search-input"
          clearable
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>

        <BarcodeScanner listen-hardware @scan="onBarcodeScanned">
          <template #trigger="{ open }">
            <el-button type="primary" size="large" class="camera-btn" @click="open">
              <el-icon :size="20"><Aim /></el-icon>
            </el-button>
          </template>
        </BarcodeScanner>
      </div>

      <!-- 仓库筛选器 -->
      <div class="warehouse-filter-row">
        <el-select
          v-model="selectedWarehouseId"
          placeholder="全部仓库"
          size="default"
          clearable
          class="filter-select"
          @change="handleSearch"
        >
          <el-option label="全部仓库" :value="null" />
          <el-option
            v-for="w in warehouseList"
            :key="w.id"
            :label="w.name"
            :value="w.id"
          />
        </el-select>

        <el-button type="primary" size="default" :loading="loading" @click="handleSearch">
          查询
        </el-button>
      </div>
    </div>

    <!-- 查询结果列表 -->
    <div class="inventory-results">
      <div class="results-header">
        <span class="count-tag">共找到 <strong>{{ inventoryList.length }}</strong> 条库存记录</span>
        <el-button size="small" type="primary" link @click="handleSearch">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>

      <div v-if="loading" class="loading-state">
        <el-icon class="is-loading" :size="28"><Loading /></el-icon>
        <span>正在实时检索库存...</span>
      </div>

      <div v-else-if="inventoryList.length === 0" class="empty-state">
        <el-icon :size="48" color="#d9d9d9"><Box /></el-icon>
        <p>未找到匹配的物料库存记录</p>
        <span class="empty-hint">请扫描条码或调整搜索关键字</span>
      </div>

      <div v-else class="inventory-cards-list">
        <div v-for="item in inventoryList" :key="item.id" class="inv-card">
          <div class="card-header">
            <div class="mat-info">
              <span class="mat-code">{{ item.material_code }}</span>
              <span class="mat-name">{{ item.material_name || '物料' }}</span>
            </div>
            <div class="qty-badge" :class="Number(item.qty) <= 0 ? 'zero' : 'positive'">
              {{ item.qty }} <span class="unit">件</span>
            </div>
          </div>

          <div class="card-body">
            <div class="meta-row">
              <span class="meta-label">所在仓库:</span>
              <span class="meta-val">{{ item.warehouse_name || '仓库#' + item.warehouse_id }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">存放库位:</span>
              <span class="meta-val highlight">{{ item.location_code || '未分配' }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">可用库存:</span>
              <span class="meta-val avail">{{ item.available_qty ?? item.qty }}</span>
            </div>
            <div v-if="item.avg_cost" class="meta-row">
              <span class="meta-label">加权均价:</span>
              <span class="meta-val">¥{{ Number(item.avg_cost).toFixed(2) }}</span>
            </div>
          </div>

          <!-- 快速跳转操作 -->
          <div class="card-actions">
            <el-button size="small" type="primary" plain @click="quickInbound(item)">
              <el-icon><Download /></el-icon> 快捷入库
            </el-button>
            <el-button size="small" type="success" plain @click="quickOutbound(item)">
              <el-icon><Upload /></el-icon> 快捷出库
            </el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { listInventory } from '@/api/warehouse'
import { listWarehouses } from '@/api/mdm'
import { playSuccessSound } from '@/utils/sound'
import BarcodeScanner from '@/components/pda/BarcodeScanner.vue'
import ScanSupportBanner from '@/components/pda/ScanSupportBanner.vue'

const route = useRoute()
const router = useRouter()

const searchQuery = ref('')
const selectedWarehouseId = ref(null)
const warehouseList = ref([])
const inventoryList = ref([])
const loading = ref(false)
const searchInputRef = ref(null)

async function loadWarehouses() {
  try {
    const res = await listWarehouses()
    if (res.code === 0) {
      warehouseList.value = res.data || []
    }
  } catch (e) {
    warehouseList.value = []
  }
}

async function handleSearch() {
  loading.value = true
  try {
    const params = {
      page: 1,
      page_size: 50
    }
    if (selectedWarehouseId.value) {
      params.warehouse_id = selectedWarehouseId.value
    }
    const q = searchQuery.value.trim()
    if (q) {
      params.material_code = q
    }

    const res = await listInventory(params)
    if (res.code === 0) {
      inventoryList.value = res.data?.list || res.data || []
    }
  } catch (e) {
    inventoryList.value = []
  } finally {
    loading.value = false
  }
}

function onBarcodeScanned(code) {
  searchQuery.value = code.trim()
  playSuccessSound()
  handleSearch()
}

function quickInbound(item) {
  router.push({
    path: '/pda/inbound',
    query: {
      code: item.material_code,
      warehouse_id: item.warehouse_id,
      location: item.location_code
    }
  })
}

function quickOutbound(item) {
  router.push({
    path: '/pda/outbound',
    query: {
      code: item.material_code,
      warehouse_id: item.warehouse_id,
      location: item.location_code
    }
  })
}

onMounted(async () => {
  await loadWarehouses()
  if (route.query?.code) {
    searchQuery.value = String(route.query.code)
  }
  handleSearch()
})
</script>

<style scoped lang="scss">
.pda-inventory-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.search-card {
  background: #ffffff;
  border-radius: 14px;
  padding: 14px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
  display: flex;
  flex-direction: column;
  gap: 10px;

  .search-input-group {
    display: flex;
    gap: 8px;

    .pda-search-input {
      flex: 1;
      :deep(.el-input__wrapper) {
        height: 48px;
        font-size: 15px;
        border-radius: 8px;
      }
    }

    .camera-btn {
      width: 48px;
      height: 48px;
      padding: 0;
      border-radius: 8px;
      background: #722ed1;
      border-color: #722ed1;
    }
  }

  .warehouse-filter-row {
    display: flex;
    gap: 8px;

    .filter-select {
      flex: 1;
    }
  }
}

.inventory-results {
  .results-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 4px;
    margin-bottom: 8px;

    .count-tag {
      font-size: 12px;
      color: #8c8c8c;
    }
  }

  .loading-state,
  .empty-state {
    background: #ffffff;
    border-radius: 12px;
    padding: 40px 16px;
    text-align: center;
    color: #8c8c8c;
    font-size: 13px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;

    .empty-hint {
      font-size: 11px;
      color: #bfbfbf;
    }
  }

  .inventory-cards-list {
    display: flex;
    flex-direction: column;
    gap: 10px;

    .inv-card {
      background: #ffffff;
      border-radius: 12px;
      padding: 14px;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
      display: flex;
      flex-direction: column;
      gap: 10px;
      border: 1px solid #f0f0f0;

      .card-header {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        border-bottom: 1px dashed #f0f0f0;
        padding-bottom: 8px;

        .mat-info {
          display: flex;
          flex-direction: column;
          gap: 2px;

          .mat-code {
            font-size: 15px;
            font-weight: 700;
            color: #1f2329;
          }

          .mat-name {
            font-size: 12px;
            color: #8c8c8c;
          }
        }

        .qty-badge {
          font-size: 18px;
          font-weight: 800;
          padding: 2px 10px;
          border-radius: 20px;

          .unit {
            font-size: 11px;
            font-weight: normal;
          }

          &.positive {
            background: #e6f7ff;
            color: #1890ff;
          }

          &.zero {
            background: #fff1f0;
            color: #ff4d4f;
          }
        }
      }

      .card-body {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 6px;
        font-size: 12px;

        .meta-row {
          display: flex;
          gap: 4px;

          .meta-label {
            color: #8c8c8c;
          }

          .meta-val {
            color: #262626;
            font-weight: 500;

            &.highlight {
              color: #722ed1;
              font-weight: 600;
            }

            &.avail {
              color: #52c41a;
              font-weight: 600;
            }
          }
        }
      }

      .card-actions {
        display: flex;
        justify-content: flex-end;
        gap: 8px;
        padding-top: 6px;
        border-top: 1px solid #fafafa;
      }
    }
  }
}
</style>
