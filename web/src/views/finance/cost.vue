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
        <el-table-column prop="product_code" label="产品编码" width="140" />
        <el-table-column prop="product_name" label="产品名称" />
        <el-table-column prop="cost_date" label="成本日期" width="120" />
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
      // 后端返回按产品汇总的数组，这里聚合成总成本对象供卡片与饼图使用（真实数据）
      const rows = Array.isArray(summaryRes.data) ? summaryRes.data : []
      costSummary.value = rows.reduce(
        (acc, r) => ({
          material_cost: (acc.material_cost || 0) + (r.material_cost || 0),
          labor_cost: (acc.labor_cost || 0) + (r.labor_cost || 0),
          overhead_cost: (acc.overhead_cost || 0) + (r.overhead_cost || 0),
          total_cost: (acc.total_cost || 0) + (r.total_cost || 0)
        }),
        {}
      )
    }
    if (cardsRes.code === 0) {
      costCards.value = cardsRes.data?.list || []
    }
  } catch (e) {
    // 失败时响应拦截器已弹出错误提示；置空，不使用假数据
    costSummary.value = {}
    costCards.value = []
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
        data: []
      },
      yAxis: { type: 'value' },
      series: [
        // 后端暂未返回月度成本趋势序列，data 置空，不编造数字
        { name: '材料成本', type: 'line', smooth: true, data: [], itemStyle: { color: '#409EFF' } },
        { name: '人工成本', type: 'line', smooth: true, data: [], itemStyle: { color: '#67C23A' } },
        { name: '制造费用', type: 'line', smooth: true, data: [], itemStyle: { color: '#E6A23C' } }
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
          // 成本构成取自 cost-summary 接口聚合后的真实金额
          data: [
            { value: costSummary.value.material_cost || 0, name: '材料成本' },
            { value: costSummary.value.labor_cost || 0, name: '人工成本' },
            { value: costSummary.value.overhead_cost || 0, name: '制造费用' }
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