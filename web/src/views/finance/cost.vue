<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">成本核算</span>
    </div>

    <el-row :gutter="20" class="stat-cards">
      <el-col :span="6">
        <el-card>
          <div class="stat-item">
            <p class="label">本月材料成本</p>
            <p class="value primary">¥{{ costSummary.material_cost || 0 }}</p>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <div class="stat-item">
            <p class="label">人工成本</p>
            <p class="value success">¥{{ costSummary.labor_cost || 0 }}</p>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <div class="stat-item">
            <p class="label">制造费用</p>
            <p class="value warning">¥{{ costSummary.overhead_cost || 0 }}</p>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <div class="stat-item">
            <p class="label">总成本</p>
            <p class="value danger">¥{{ costSummary.total_cost || 0 }}</p>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20">
      <el-col :span="14">
        <el-card>
          <template #header>
            <span>成本趋势</span>
          </template>
          <div ref="costChartRef" class="chart-container"></div>
        </el-card>
      </el-col>
      <el-col :span="10">
        <el-card>
          <template #header>
            <span>成本构成</span>
          </template>
          <div ref="costPieRef" class="chart-container"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-card style="margin-top: 20px">
      <template #header>
        <span>成本卡片列表</span>
      </template>
      <el-table :data="costCards" style="width: 100%">
        <el-table-column prop="cost_type_name" label="成本类型" width="120" />
        <el-table-column prop="material_name" label="物料名称" />
        <el-table-column prop="period" label="期间" width="120" />
        <el-table-column prop="amount" label="金额" width="150">
          <template #default="{ row }">
            ¥{{ row.amount }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import { listCostCards, getCostSummary } from '@/api/finance'

const costChartRef = ref(null)
const costPieRef = ref(null)
const costSummary = ref({})
const costCards = ref([])

let costChart = null
let costPie = null

async function loadData() {
  try {
    const [summaryRes, cardsRes] = await Promise.all([
      getCostSummary(),
      listCostCards({ page: 1, page_size: 20 })
    ])
    if (summaryRes.code === 0) {
      costSummary.value = summaryRes.data || {}
    }
    if (cardsRes.code === 0) {
      costCards.value = cardsRes.data?.list || []
    }
  } catch (e) {
    costSummary.value = {
      material_cost: 680000,
      labor_cost: 320000,
      overhead_cost: 150000,
      total_cost: 1150000
    }
    costCards.value = [
      { cost_type_name: '材料成本', material_name: '原材料A', period: '2026-07', amount: 280000 },
      { cost_type_name: '材料成本', material_name: '原材料B', period: '2026-07', amount: 200000 },
      { cost_type_name: '人工成本', material_name: '成品A', period: '2026-07', amount: 180000 },
      { cost_type_name: '制造费用', material_name: '成品A', period: '2026-07', amount: 150000 }
    ]
  }
  nextTick(() => {
    initCharts()
  })
}

function initCharts() {
  if (costChartRef.value) {
    costChart = echarts.init(costChartRef.value)
    costChart.setOption({
      tooltip: { trigger: 'axis' },
      legend: { data: ['材料成本', '人工成本', '制造费用'] },
      xAxis: {
        type: 'category',
        data: ['1月', '2月', '3月', '4月', '5月', '6月', '7月']
      },
      yAxis: { type: 'value' },
      series: [
        { name: '材料成本', type: 'line', smooth: true, data: [52, 58, 61, 55, 63, 70, 68], itemStyle: { color: '#409EFF' } },
        { name: '人工成本', type: 'line', smooth: true, data: [28, 30, 29, 31, 32, 30, 32], itemStyle: { color: '#67C23A' } },
        { name: '制造费用', type: 'line', smooth: true, data: [12, 13, 14, 13, 15, 14, 15], itemStyle: { color: '#E6A23C' } }
      ]
    })
  }

  if (costPieRef.value) {
    costPie = echarts.init(costPieRef.value)
    costPie.setOption({
      tooltip: { trigger: 'item' },
      series: [
        {
          type: 'pie',
          radius: '70%',
          data: [
            { value: 680, name: '材料成本' },
            { value: 320, name: '人工成本' },
            { value: 150, name: '制造费用' }
          ],
          color: ['#409EFF', '#67C23A', '#E6A23C']
        }
      ]
    })
  }
}

onMounted(() => {
  loadData()
})
</script>

<style scoped lang="scss">
.stat-cards {
  margin-bottom: 20px;

  .stat-item {
    text-align: center;

    .label {
      color: #909399;
      font-size: 13px;
      margin: 0 0 8px 0;
    }

    .value {
      font-size: 24px;
      font-weight: 600;
      margin: 0;

      &.primary { color: #409EFF; }
      &.success { color: #67C23A; }
      &.warning { color: #E6A23C; }
      &.danger { color: #F56C6C; }
    }
  }
}

.chart-container {
  height: 300px;
}
</style>