<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<!-- 页面级提示：进入扫码页即告知摄像头扫码是否可用，不必等点了扫码按钮才发现。 -->
<template>
  <el-alert
    v-if="message"
    class="scan-support-banner"
    type="warning"
    :closable="false"
    show-icon
    :title="message"
  />
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { detectScanSupport } from '@/utils/scanSupport'

const message = ref('')

onMounted(async () => {
  const support = await detectScanSupport()
  message.value = support.cameraScanAvailable ? '' : support.message
})
</script>

<style scoped>
.scan-support-banner {
  border-radius: 10px;
}
</style>
