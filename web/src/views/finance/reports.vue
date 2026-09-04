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
        <el-table-column prop="entry_no" label="分录号" width="180" />
        <el-table-column prop="account_code" label="科目编码" width="120" />
        <el-table-column prop="account_name" label="科目名称" />
        <el-table-column prop="description" label="摘要" />
        <el-table-column prop="biz_no" label="业务单号" width="170" />
        <el-table-column prop="debit_amount" label="借方" width="120">
          <template #default="{ row }">
            ¥{{ row.debit_amount }}
          </template>
        </el-table-column>
        <el-table-column prop="credit_amount" label="贷方" width="120">
          <template #default="{ row }">
            ¥{{ row.credit_amount }}
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="170" />
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

// 月份范围选择器给出 [起始月, 结束月]；后端 financial-report 接收单个 period(YYYY-MM)，
// 这里取起始月作为报表期间；未选择时不传，后端默认当前月。
function buildReportParams() {
  const range = dateRange.value
  if (Array.isArray(range) && range.length === 2 && range[0]) {
    const d = new Date(range[0])
    const period = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
    return { period }
  }
  return {}
}

async function loadData() {
  try {
    const [reportRes, entriesRes] = await Promise.all([
      getFinancialReport(buildReportParams()),
      listAccountEntries({ page: 1, page_size: 20 })
    ])
    if (reportRes.code === 0) {
      // 映射后端 FinancialReportVO 真实字段到卡片展示字段
      const d = reportRes.data || {}
      reportData.value = {
        total_income: d.total_revenue || 0,
        total_expense: d.total_cost || 0,
        net_profit: d.gross_profit || 0,
        profit_rate: d.gross_margin || 0
      }
    }
    if (entriesRes.code === 0) {
      entries.value = entriesRes.data?.list || []
    }
  } catch (e) {
    // 失败时响应拦截器已弹出错误提示；置空，不使用假数据
    reportData.value = {}
    entries.value = []
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
        data: []
      },
      yAxis: { type: 'value' },
      series: [
        // 后端仅返回单期汇总，暂无月度趋势序列，data 置空，不编造数字
        { name: '收入', type: 'bar', data: [], itemStyle: { color: '#67C23A' } },
        { name: '支出', type: 'bar', data: [], itemStyle: { color: '#F56C6C' } },
        { name: '利润', type: 'line', smooth: true, data: [], itemStyle: { color: '#409EFF' } }
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
          // 后端未返回支出分类构成序列，置空，不编造数字
          data: []
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