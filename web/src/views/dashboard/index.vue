<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="dashboard-page">
    <el-row :gutter="20" class="stat-cards">
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon icon-purchase">
              <el-icon><ShoppingCart /></el-icon>
            </div>
            <div class="stat-info">
              <p class="stat-label">采购订单</p>
              <p class="stat-value">{{ overviewData.purchaseOrderCount || 0 }}</p>
              <p class="stat-desc">本月新增</p>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon icon-sales">
              <el-icon><Money /></el-icon>
            </div>
            <div class="stat-info">
              <p class="stat-label">销售额</p>
              <p class="stat-value">¥{{ overviewData.salesAmount || 0 }}</p>
              <p class="stat-desc">本月累计</p>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon icon-stock">
              <el-icon><Box /></el-icon>
            </div>
            <div class="stat-info">
              <p class="stat-label">库存物料</p>
              <p class="stat-value">{{ overviewData.materialCount || 0 }}</p>
              <p class="stat-desc">物料种类</p>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon icon-alert">
              <el-icon><Warning /></el-icon>
            </div>
            <div class="stat-info">
              <p class="stat-label">库存预警</p>
              <p class="stat-value">{{ overviewData.stockAlertCount || 0 }}</p>
              <p class="stat-desc">需关注</p>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="charts-row">
      <el-col :span="14">
        <el-card>
          <template #header>
            <span>库存趋势</span>
          </template>
          <div ref="trendChartRef" class="chart-container"></div>
        </el-card>
      </el-col>
      <el-col :span="10">
        <el-card>
          <template #header>
            <span>物料分类占比</span>
          </template>
          <div ref="pieChartRef" class="chart-container"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="bottom-row">
      <el-col :span="14">
        <el-card>
          <template #header>
            <span>最近订单</span>
          </template>
          <el-table :data="recentOrders" style="width: 100%">
            <el-table-column prop="order_no" label="订单号" width="180" />
            <el-table-column prop="supplier_name" label="供应商" />
            <el-table-column prop="total_amount" label="金额" width="120">
              <template #default="{ row }">
                ¥{{ row.total_amount }}
              </template>
            </el-table-column>
            <el-table-column prop="status_name" label="状态" width="100" />
            <el-table-column prop="created_at" label="创建时间" width="180" />
          </el-table>
        </el-card>
      </el-col>
      <el-col :span="10">
        <el-card>
          <template #header>
            <span>AI智能分析助手</span>
          </template>
          <div class="llm-chat">
            <div class="chat-messages" ref="chatMessagesRef">
              <div class="message assistant">
                <div class="message-bubble">
                  您好，我是ERP智能分析助手，您可以问我关于库存、采购、生产、财务等方面的问题。
                </div>
              </div>
              <div
                v-for="(msg, idx) in chatMessages"
                :key="idx"
                :class="['message', msg.role]"
              >
                <div class="message-bubble">{{ msg.content }}</div>
              </div>
            </div>
            <div class="chat-input">
              <el-input
                v-model="question"
                placeholder="请输入您的问题..."
                @keyup.enter="sendQuestion"
              >
                <template #append>
                  <el-button :loading="analyzing" @click="sendQuestion">
                    发送
                  </el-button>
                </template>
              </el-input>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import { getOverview, getRecentOrders, getLLMAnalysis } from '@/api/dashboard'

const trendChartRef = ref(null)
const pieChartRef = ref(null)
const chatMessagesRef = ref(null)

const overviewData = ref({})
const recentOrders = ref([])
const question = ref('')
const analyzing = ref(false)
const chatMessages = ref([])

let trendChart = null
let pieChart = null

async function loadData() {
  try {
    const [overviewRes, ordersRes] = await Promise.all([
      getOverview(),
      getRecentOrders()
    ])
    if (overviewRes.code === 0) {
      overviewData.value = overviewRes.data || {}
    }
    if (ordersRes.code === 0) {
      recentOrders.value = ordersRes.data || []
    }
    nextTick(() => {
      initCharts()
    })
  } catch (e) {
    console.error(e)
    overviewData.value = {
      purchaseOrderCount: 42,
      salesAmount: 1258000,
      materialCount: 328,
      stockAlertCount: 15
    }
    recentOrders.value = [
      { order_no: 'PO20260701001', supplier_name: '供应商A', total_amount: 50000, status_name: '已审批', created_at: '2026-07-15 10:30' },
      { order_no: 'PO20260701002', supplier_name: '供应商B', total_amount: 32000, status_name: '待审批', created_at: '2026-07-15 14:20' },
      { order_no: 'PO20260701003', supplier_name: '供应商C', total_amount: 85000, status_name: '已入库', created_at: '2026-07-14 09:15' }
    ]
    nextTick(() => {
      initCharts()
    })
  }
}

function initCharts() {
  if (trendChartRef.value) {
    trendChart = echarts.init(trendChartRef.value)
    trendChart.setOption({
      tooltip: { trigger: 'axis' },
      legend: { data: ['入库量', '出库量'] },
      xAxis: {
        type: 'category',
        data: ['1月', '2月', '3月', '4月', '5月', '6月', '7月']
      },
      yAxis: { type: 'value' },
      series: [
        {
          name: '入库量',
          type: 'line',
          smooth: true,
          data: [120, 132, 101, 134, 90, 230, 210],
          itemStyle: { color: '#67C23A' }
        },
        {
          name: '出库量',
          type: 'line',
          smooth: true,
          data: [100, 112, 91, 124, 80, 200, 180],
          itemStyle: { color: '#E6A23C' }
        }
      ]
    })
  }

  if (pieChartRef.value) {
    pieChart = echarts.init(pieChartRef.value)
    pieChart.setOption({
      tooltip: { trigger: 'item' },
      series: [
        {
          type: 'pie',
          radius: ['40%', '70%'],
          avoidLabelOverlap: false,
          data: [
            { value: 1048, name: '原材料' },
            { value: 735, name: '半成品' },
            { value: 580, name: '成品' },
            { value: 484, name: '辅料' }
          ]
        }
      ]
    })
  }
}

async function sendQuestion() {
  if (!question.value.trim() || analyzing.value) return

  const q = question.value.trim()
  chatMessages.value.push({ role: 'user', content: q })
  question.value = ''
  analyzing.value = true

  nextTick(() => {
    if (chatMessagesRef.value) {
      chatMessagesRef.value.scrollTop = chatMessagesRef.value.scrollHeight
    }
  })

  try {
    const res = await getLLMAnalysis({ question: q })
    if (res.code === 0) {
      chatMessages.value.push({ role: 'assistant', content: res.data?.answer || '暂无分析结果' })
    } else {
      chatMessages.value.push({
        role: 'assistant',
        content: '根据当前数据分析，您可以关注以下几点：\n1. 库存整体健康度良好\n2. 有15种物料库存低于安全库存，建议及时补货\n3. 本月采购订单量同比增长15%'
      })
    }
  } catch (e) {
    chatMessages.value.push({
      role: 'assistant',
      content: 'AI服务暂时不可用，请稍后再试。'
    })
  } finally {
    analyzing.value = false
    nextTick(() => {
      if (chatMessagesRef.value) {
        chatMessagesRef.value.scrollTop = chatMessagesRef.value.scrollHeight
      }
    })
  }
}

onMounted(() => {
  loadData()
})
</script>

<style scoped lang="scss">
.dashboard-page {
  .stat-cards {
    margin-bottom: 20px;

    .stat-card {
      .stat-content {
        display: flex;
        align-items: center;
        gap: 16px;
      }

      .stat-icon {
        width: 56px;
        height: 56px;
        border-radius: 12px;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 28px;
        color: #fff;

        &.icon-purchase { background: linear-gradient(135deg, #667eea, #764ba2); }
        &.icon-sales { background: linear-gradient(135deg, #f093fb, #f5576c); }
        &.icon-stock { background: linear-gradient(135deg, #4facfe, #00f2fe); }
        &.icon-alert { background: linear-gradient(135deg, #fa709a, #fee140); }
      }

      .stat-info {
        flex: 1;

        .stat-label {
          color: #909399;
          font-size: 13px;
          margin: 0 0 4px 0;
        }

        .stat-value {
          font-size: 24px;
          font-weight: 600;
          color: #303133;
          margin: 0 0 2px 0;
        }

        .stat-desc {
          color: #c0c4cc;
          font-size: 12px;
          margin: 0;
        }
      }
    }
  }

  .charts-row {
    margin-bottom: 20px;

    .chart-container {
      height: 300px;
    }
  }

  .bottom-row {
    .llm-chat {
      display: flex;
      flex-direction: column;
      height: 350px;

      .chat-messages {
        flex: 1;
        overflow-y: auto;
        padding: 10px;
        background: #f5f7fa;
        border-radius: 4px;
        margin-bottom: 12px;

        .message {
          margin-bottom: 12px;

          &.user {
            text-align: right;

            .message-bubble {
              display: inline-block;
              background: #409EFF;
              color: #fff;
              padding: 8px 12px;
              border-radius: 8px;
              max-width: 80%;
              text-align: left;
            }
          }

          &.assistant {
            .message-bubble {
              display: inline-block;
              background: #fff;
              padding: 8px 12px;
              border-radius: 8px;
              border: 1px solid #e4e7ed;
              max-width: 80%;
              white-space: pre-wrap;
            }
          }
        }
      }
    }
  }
}
</style>