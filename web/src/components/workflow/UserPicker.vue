<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<!--
  转办 / 加签目标用户选择。
  用户列表接口 /iam/users 需要 iam:user 权限，普通审批人通常没有，
  拉取失败时改为手动输入用户 ID，并说明原因。
-->
<template>
  <div class="user-picker">
    <el-select
      v-if="mode === 'list'"
      :model-value="modelValue"
      filterable
      remote
      :remote-method="search"
      :loading="loading"
      placeholder="输入用户名搜索"
      style="width: 100%"
      @update:model-value="(v) => emit('update:modelValue', v)"
    >
      <el-option
        v-for="u in options"
        :key="u.id"
        :label="`${u.real_name || u.username}（${u.username}，ID ${u.id}）${u.dept_name ? ' · ' + u.dept_name : ''}`"
        :value="u.id"
        :disabled="u.status !== undefined && u.status !== 1"
      />
    </el-select>

    <template v-else>
      <el-input-number
        :model-value="modelValue"
        :min="1"
        :controls="false"
        placeholder="目标用户 ID"
        style="width: 100%"
        @update:model-value="(v) => emit('update:modelValue', v)"
      />
      <div class="picker-hint">你没有查看用户列表的权限，请直接填写目标用户的 ID（可向管理员索取）。</div>
    </template>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import request from '@/utils/request'

defineProps({
  modelValue: { type: Number, default: null }
})
const emit = defineEmits(['update:modelValue'])

const mode = ref('list')
const loading = ref(false)
const options = ref([])

async function fetchUsers(username) {
  const res = await request({
    url: '/iam/users',
    method: 'get',
    params: { page: 1, page_size: 50, username: username || undefined },
    silent: true
  })
  return res.data?.list || []
}

async function search(keyword) {
  if (mode.value !== 'list') return
  loading.value = true
  try {
    options.value = await fetchUsers(keyword)
  } catch (e) {
    mode.value = 'manual'
  } finally {
    loading.value = false
  }
}

onMounted(() => search(''))
</script>

<style scoped>
.user-picker {
  width: 100%;
}
.picker-hint {
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.5;
  color: #909399;
}
</style>
