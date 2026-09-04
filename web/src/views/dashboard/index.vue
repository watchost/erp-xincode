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
              <p class="stat-label">待处理采购订单</p>
              <p class="stat-value">{{ overviewData.pending_purchase_orders || 0 }}</p>
              <p class="stat-desc">待审批</p>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon icon-production">
              <el-icon><Operation /></el-icon>
            </div>
            <div class="stat-info">
              <p class="stat-label">进行中工单</p>
              <p class="stat-value">{{ overviewData.in_progress_work_orders || 0 }}</p>
              <p class="stat-desc">已下达/生产中</p>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon icon-stock">
              <el-icon><Money /></el-icon>
            </div>
            <div class="stat-info">
              <p class="stat-label">库存总值</p>
              <p class="stat-value">¥{{ formatAmount(overviewData.inventory_value) }}</p>
              <p class="stat-desc">结存成本金额</p>
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
              <p class="stat-value">{{ overviewData.stock_alert_count || 0 }}</p>
              <p class="stat-desc">可用量偏低</p>
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
            <span>库存预警</span>
          </template>
          <el-table :data="stockAlerts" style="width: 100%" size="small" max-height="300">
            <el-table-column prop="material_name" label="物料" min-width="120" />
            <el-table-column prop="warehouse_name" label="仓库" width="90" />
            <el-table-column prop="available_qty" label="可用量" width="80">
              <template #default="{ row }">
                <span :style="{ color: row.level === 'danger' ? '#F56C6C' : '#E6A23C', fontWeight: 600 }">
                  {{ row.available_qty }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="min_stock" label="安全库存" width="80" />
          </el-table>
          <el-empty v-if="!stockAlerts.length" description="库存水位正常" :image-size="60" />
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
            <el-table-column prop="status_desc" label="状态" width="100" />
            <el-table-column prop="created_at" label="创建时间" width="180" />
          </el-table>
        </el-card>
      </el-col>
      <el-col v-permission="'dashboard:llm'" :span="10">
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
import { getOverview, getRecentOrders, getDashboardStockAlerts, getLLMAnalysis } from '@/api/dashboard'

const trendChartRef = ref(null)
const chatMessagesRef = ref(null)

const overviewData = ref({})
const recentOrders = ref([])
const stockAlerts = ref([])
const question = ref('')
const analyzing = ref(false)
const chatMessages = ref([])

let trendChart = null

// 金额千分位 + 两位小数，避免大额库存值挤版
function formatAmount(v) {
  const n = Number(v || 0)
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

async function loadData() {
  try {
    const [overviewRes, ordersRes, alertsRes] = await Promise.all([
      getOverview(),
      getRecentOrders(),
      getDashboardStockAlerts()
    ])
    if (overviewRes.code === 0) {
      overviewData.value = overviewRes.data || {}
    }
    if (ordersRes.code === 0) {
      recentOrders.value = ordersRes.data || []
    }
    if (alertsRes.code === 0) {
      stockAlerts.value = alertsRes.data || []
    }
    nextTick(() => {
      initCharts()
    })
  } catch (e) {
    // 失败时响应拦截器已弹出错误提示；KPI 与列表置空，不编造数字
    console.error(e)
    overviewData.value = {}
    recentOrders.value = []
    stockAlerts.value = []
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
        data: []
      },
      yAxis: { type: 'value' },
      series: [
        {
          name: '入库量',
          type: 'line',
          smooth: true,
          data: [],
          itemStyle: { color: '#67C23A' }
        },
        {
          name: '出库量',
          type: 'line',
          smooth: true,
          data: [],
          itemStyle: { color: '#E6A23C' }
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
      // 业务失败：拦截器已弹错误，这里不编造 AI 结论
      chatMessages.value.push({ role: 'assistant', content: 'AI 分析暂时不可用，请稍后再试。' })
    }
  } catch (e) {
    // 网络/服务失败：拦截器已弹错误，这里不编造 AI 结论
    chatMessages.value.push({
      role: 'assistant',
      content: 'AI 分析暂时不可用，请稍后再试。'
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
        &.icon-production { background: linear-gradient(135deg, #43cea2, #185a9d); }
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