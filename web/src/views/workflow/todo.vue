<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<!-- 我的待办 / 我的已办。桌面端表格，窄屏（含 PDA 布局）用卡片。 -->
<template>
  <div class="page-container wf-todo" :class="{ mobile: isMobile }">
    <div v-if="!inPda" class="page-header">
      <span class="page-title">我的审批</span>
    </div>

    <el-card :body-style="{ padding: isMobile ? '10px' : '20px' }">
      <el-tabs v-model="tab" @tab-change="onTabChange">
        <el-tab-pane name="todo">
          <template #label>
            待办 <el-badge v-if="todoTotal > 0" :value="todoTotal" :max="99" class="tab-badge" />
          </template>
        </el-tab-pane>
        <el-tab-pane label="已办" name="done" />
      </el-tabs>

      <!-- 桌面端表格 -->
      <el-table v-if="!isMobile" v-loading="loading" :data="list" style="width: 100%" empty-text="暂无记录">
        <el-table-column label="单据" min-width="200">
          <template #default="{ row }">
            <div>{{ bizTypeText(row.biz_type) }}</div>
            <el-link type="primary" @click="openInstance(row)">{{ row.biz_no }}</el-link>
          </template>
        </el-table-column>
        <el-table-column label="当前节点" min-width="160">
          <template #default="{ row }">
            {{ row.node_name || row.node_code }}
            <el-tag size="small" effect="plain" class="ml6">{{ row.sign_mode === 'all' ? '会签' : '或签' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="发起人 ID" prop="starter_id" width="100" />
        <el-table-column v-if="tab === 'done'" label="我的处理" width="100">
          <template #default="{ row }">
            <el-tag :type="taskStatus(row.status).type" size="small">{{ taskStatus(row.status).text }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="流程状态" width="100">
          <template #default="{ row }">
            <el-tag :type="instStatus(row.instance_status).type" size="small">{{ instStatus(row.instance_status).text }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="tab === 'todo' ? '到达时间' : '处理时间'" width="150">
          <template #default="{ row }">{{ fmtTime(tab === 'todo' ? row.created_at : row.finished_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" :width="tab === 'todo' ? 260 : 100" fixed="right">
          <template #default="{ row }">
            <template v-if="tab === 'todo'">
              <el-button type="success" link size="small" @click="openAction('approve', row)">通过</el-button>
              <el-button type="danger" link size="small" @click="openAction('reject', row)">驳回</el-button>
              <el-button type="primary" link size="small" @click="openAction('transfer', row)">转办</el-button>
              <el-button type="primary" link size="small" @click="openAction('add_sign', row)">加签</el-button>
            </template>
            <el-button link size="small" @click="openInstance(row)">审批历史</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 窄屏卡片 -->
      <div v-else v-loading="loading" class="card-list">
        <el-empty v-if="!loading && list.length === 0" description="暂无记录" :image-size="80" />
        <div v-for="row in list" :key="row.task_id" class="task-card">
          <div class="card-head" @click="openInstance(row)">
            <div class="biz">
              <span class="biz-type">{{ bizTypeText(row.biz_type) }}</span>
              <span class="biz-no">{{ row.biz_no }}</span>
            </div>
            <el-tag :type="instStatus(row.instance_status).type" size="small">{{ instStatus(row.instance_status).text }}</el-tag>
          </div>
          <div class="card-body" @click="openInstance(row)">
            <div>节点：{{ row.node_name || row.node_code }}（{{ row.sign_mode === 'all' ? '会签' : '或签' }}）</div>
            <div>发起人 ID：{{ row.starter_id }}</div>
            <div v-if="tab === 'done'">我的处理：{{ taskStatus(row.status).text }}</div>
            <div>{{ tab === 'todo' ? '到达' : '处理' }}：{{ fmtTime(tab === 'todo' ? row.created_at : row.finished_at) }}</div>
          </div>
          <div v-if="tab === 'todo'" class="card-actions">
            <el-button type="success" size="large" @click="openAction('approve', row)">通过</el-button>
            <el-button type="danger" size="large" @click="openAction('reject', row)">驳回</el-button>
            <el-dropdown trigger="click" @command="(cmd) => openAction(cmd, row)">
              <el-button size="large">更多</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="transfer">转办</el-dropdown-item>
                  <el-dropdown-item command="add_sign">加签</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </div>
      </div>

      <div class="pagination">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :layout="isMobile ? 'prev, pager, next' : 'total, sizes, prev, pager, next'"
          :page-sizes="[10, 20, 50]"
          :pager-count="isMobile ? 5 : 7"
          small
          @size-change="loadData"
          @current-change="loadData"
        />
      </div>
    </el-card>

    <TaskActionDialog v-model:visible="dialogVisible" :action="dialogAction" :task="dialogTask" @done="onActionDone" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { listTodo, listDone } from '@/api/workflow'
import { INSTANCE_STATUS, TASK_STATUS, BIZ_TYPE, fmtTime } from '@/utils/workflow'
import { useIsMobile, useLayoutPaths } from '@/composables/useLayout'
import { notifyBadgeRefresh } from '@/utils/badgeBus'
import TaskActionDialog from '@/components/workflow/TaskActionDialog.vue'

const router = useRouter()
const route = useRoute()
const isMobile = useIsMobile()
const { inPda, instancePath } = useLayoutPaths()

const tab = ref(route.query.tab === 'done' ? 'done' : 'todo')
const list = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const todoTotal = ref(0)

const dialogVisible = ref(false)
const dialogAction = ref('approve')
const dialogTask = ref(null)

const bizTypeText = (t) => BIZ_TYPE[t] || t
const instStatus = (s) => INSTANCE_STATUS[s] || { text: String(s ?? '-'), type: 'info' }
const taskStatus = (s) => TASK_STATUS[s] || { text: String(s ?? '-'), type: 'info' }

async function loadData() {
  loading.value = true
  try {
    const fn = tab.value === 'todo' ? listTodo : listDone
    const res = await fn({ page: page.value, page_size: pageSize.value })
    list.value = res.data?.list || []
    total.value = res.data?.total || 0
    if (tab.value === 'todo') todoTotal.value = total.value
  } catch (e) {
    list.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function onTabChange() {
  page.value = 1
  router.replace({ query: { ...route.query, tab: tab.value } })
  loadData()
}

function openAction(action, row) {
  dialogAction.value = action
  dialogTask.value = row
  dialogVisible.value = true
}

function openInstance(row) {
  router.push(instancePath(row.instance_no))
}

function onActionDone() {
  loadData()
  notifyBadgeRefresh()
}

onMounted(loadData)
</script>

<style scoped lang="scss">
.ml6 {
  margin-left: 6px;
}
.tab-badge {
  margin-left: 4px;
  vertical-align: middle;
}
.pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
.mobile .pagination {
  justify-content: center;
}
.card-list {
  min-height: 120px;
}
.task-card {
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 12px;
  margin-bottom: 10px;
  background: #fff;

  .card-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
    .biz-type {
      font-size: 12px;
      color: #909399;
      margin-right: 6px;
    }
    .biz-no {
      font-size: 16px;
      font-weight: 600;
      color: #303133;
      word-break: break-all;
    }
  }
  .card-body {
    font-size: 13px;
    color: #606266;
    line-height: 1.8;
  }
  .card-actions {
    display: flex;
    gap: 8px;
    margin-top: 10px;
    .el-button {
      flex: 1;
      margin: 0;
    }
    .el-dropdown {
      flex: 1;
      .el-button {
        width: 100%;
      }
    }
  }
}
</style>
