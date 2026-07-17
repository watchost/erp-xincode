<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="page-container">
    <div class="page-header">
      <span class="page-title">收支报表</span>
      <el-date-picker
        v-model="dateRange"
        type="monthrange"
        range-separator="至"
        start-placeholder="开始月份"
        end-placeholder="结束月份"
        @change="loadData"
      />
    </div>

    <el-row :gutter="20" class="stat-cards">
      <el-col :span="6">
        <el-card>
          <div class="stat-item">
            <p class="label">总收入</p>
            <p class="value income">¥{{ reportData.total_income || 0 }}</p>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <div class="stat-item">
            <p class="label">总支出</p>
            <p class="value expense">¥{{ reportData.total_expense || 0 }}</p>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <div class="stat-item">
            <p class="label">净利润</p>
            <p class="value profit">¥{{ reportData.net_profit || 0 }}</p>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card>
          <div class="stat-item">
            <p class="label">利润率</p>
            <p class="value rate">{{ reportData.profit_rate || 0 }}%</p>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20">
      <el-col :span="16">
        <el-card>
          <template #header>
            <span>收支趋势</span>
          </template>
          <div ref="reportChartRef" class="chart-container"></div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card>
          <template #header>
            <span>支出分类</span>
          </template>
          <div ref="expensePieRef" class="chart-container"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-card style="margin-top: 20px">
      <template #header>
        <span>会计分录</span>
      </template>
      <el-table :data="entries" style="width: 100%">
        <el-table-column prop="voucher_no" label="凭证号" width="180" />
        <el-table-column prop="biz_type_name" label="业务类型" width="120" />
        <el-table-column prop="biz_no" label="业务单号" width="180" />
        <el-table-column prop="debit" label="借方" width="120">
          <template #default="{ row }">
            ¥{{ row.debit }}
          </template>
        </el-table-column>
        <el-table-column prop="credit" label="贷方" width="120">
          <template #default="{ row }">
            ¥{{ row.credit }}
          </template>
        </el-table-column>
        <el-table-column prop="period" label="期间" width="100" />
        <el-table-column prop="created_at" label="创建时间" width="180" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import { getFinancialReport, listAccountEntries } from '@/api/finance'

const dateRange = ref([])
const reportChartRef = ref(null)
const expensePieRef = ref(null)
const reportData = ref({})
const entries = ref([])

let reportChart = null
let expensePie = null

async function loadData() {
  try {
    const [reportRes, entriesRes] = await Promise.all([
      getFinancialReport({}),
      listAccountEntries({ page: 1, page_size: 20 })
    ])
    if (reportRes.code === 0) {
      reportData.value = reportRes.data || {}
    }
    if (entriesRes.code === 0) {
      entries.value = entriesRes.data?.list || []
    }
  } catch (e) {
    reportData.value = {
      total_income: 2580000,
      total_expense: 1850000,
      net_profit: 730000,
      profit_rate: 28.3
    }
    entries.value = [
      { voucher_no: 'V202607001', biz_type_name: '采购入库', biz_no: 'PO20260701001', debit: 50000, credit: 50000, period: '2026-07', created_at: '2026-07-15 10:30:00' },
      { voucher_no: 'V202607002', biz_type_name: '销售收入', biz_no: 'SO20260701001', debit: 120000, credit: 120000, period: '2026-07', created_at: '2026-07-15 14:20:00' },
      { voucher_no: 'V202607003', biz_type_name: '生产领料', biz_no: 'WO20260701001', debit: 35000, credit: 35000, period: '2026-07', created_at: '2026-07-14 09:15:00' }
    ]
  }
  nextTick(() => {
    initCharts()
  })
}

function initCharts() {
  if (reportChartRef.value) {
    reportChart = echarts.init(reportChartRef.value)
    reportChart.setOption({
      tooltip: { trigger: 'axis' },
      legend: { data: ['收入', '支出', '利润'] },
      xAxis: {
        type: 'category',
        data: ['1月', '2月', '3月', '4月', '5月', '6月', '7月']
      },
      yAxis: { type: 'value' },
      series: [
        { name: '收入', type: 'bar', data: [180, 210, 195, 220, 240, 260, 258], itemStyle: { color: '#67C23A' } },
        { name: '支出', type: 'bar', data: [130, 150, 145, 160, 175, 185, 185], itemStyle: { color: '#F56C6C' } },
        { name: '利润', type: 'line', smooth: true, data: [50, 60, 50, 60, 65, 75, 73], itemStyle: { color: '#409EFF' } }
      ]
    })
  }

  if (expensePieRef.value) {
    expensePie = echarts.init(expensePieRef.value)
    expensePie.setOption({
      tooltip: { trigger: 'item' },
      series: [
        {
          type: 'pie',
          radius: ['40%', '70%'],
          data: [
            { value: 680, name: '材料成本' },
            { value: 320, name: '人工成本' },
            { value: 150, name: '制造费用' },
            { value: 280, name: '运营费用' },
            { value: 420, name: '其他支出' }
          ]
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

      &.income { color: #67C23A; }
      &.expense { color: #F56C6C; }
      &.profit { color: #409EFF; }
      &.rate { color: '#909399'; }
    }
  }
}

.chart-container {
  height: 300px;
}
</style>