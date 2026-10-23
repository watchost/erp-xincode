<!-- Copyright 2026 zhouhouping. All Rights Reserved. -->
<template>
  <div class="scanner-container">
    <!-- 物理硬件扫码枪全局监听（PDA激光扫码头通常模拟快速键盘输入） -->
    <slot name="trigger" :open="openScanner">
      <el-button type="primary" size="large" class="pda-scan-btn" @click="openScanner">
        <el-icon><Aim /></el-icon>
        <span>摄像头扫码</span>
      </el-button>
    </slot>

    <!-- 摄像头扫码全屏/弹窗界面 -->
    <el-dialog
      v-model="visible"
      title="条码/二维码扫描"
      fullscreen
      destroy-on-close
      append-to-body
      class="pda-scanner-dialog"
      :before-close="handleBeforeClose"
    >
      <!-- 降级提示：浏览器不支持摄像头识别 / 非 HTTPS / 摄像头被拒绝 -->
      <div v-if="fallbackMessage" class="scan-fallback">
        <el-alert
          :title="fallbackMessage"
          type="warning"
          :closable="false"
          show-icon
        />
        <ul class="fallback-tips">
          <li>PDA 设备：直接按机身侧面的扫码键，条码会自动填入</li>
          <li>手机 / 其他设备：在下方输入框中手动输入条码，按回车或点“确认”</li>
        </ul>
      </div>

      <div v-else class="camera-viewport">
        <!-- 视频流区域 -->
        <video
          ref="videoEl"
          class="camera-video"
          autoplay
          playsinline
          muted
        ></video>

        <!-- 扫码框取景辅助器 -->
        <div class="scanner-overlay">
          <div class="scanner-guide-box">
            <div class="corner top-left"></div>
            <div class="corner top-right"></div>
            <div class="corner bottom-left"></div>
            <div class="corner bottom-right"></div>
            <div class="scan-laser-line"></div>
          </div>
          <p class="scan-hint">将条码 / 二维码放入框内即可自动对焦扫描</p>
        </div>

        <!-- 摄像头控制浮动工具条 -->
        <div class="scanner-toolbar">
          <el-button
            circle
            size="large"
            :type="torchActive ? 'warning' : 'info'"
            :disabled="!hasTorch"
            @click="toggleTorch"
          >
            <el-icon><Sunny /></el-icon>
          </el-button>

          <el-button
            circle
            size="large"
            type="info"
            @click="switchCamera"
          >
            <el-icon><Refresh /></el-icon>
          </el-button>

          <el-button
            circle
            size="large"
            type="danger"
            @click="closeDialog"
          >
            <el-icon><Close /></el-icon>
          </el-button>
        </div>
      </div>

      <!-- 底部手动输入应急托底 -->
      <div class="manual-input-bar">
        <el-input
          ref="manualInputRef"
          v-model="manualCode"
          placeholder="手动输入条码，或按 PDA 扫码键"
          size="large"
          clearable
          @keyup.enter="handleManualSubmit"
        >
          <template #append>
            <el-button type="primary" @click="handleManualSubmit">确认</el-button>
          </template>
        </el-input>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { playSuccessSound } from '@/utils/sound'
import { detectScanSupport } from '@/utils/scanSupport'

const props = defineProps({
  // 是否监听 PDA 硬件扫码键（扫码头模拟快速键盘输入 + 回车）。
  // 默认关闭：同一页面只应让一个扫描器监听，否则一次扫码会触发多次。
  listenHardware: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['scan', 'close'])

const WANTED_FORMATS = ['qr_code', 'code_128', 'code_39', 'ean_13', 'ean_8', 'upc_a', 'data_matrix']

const visible = ref(false)
const videoEl = ref(null)
const manualInputRef = ref(null)
const manualCode = ref('')
const hasTorch = ref(false)
const torchActive = ref(false)
const currentFacingMode = ref('environment')
// 非空时不显示摄像头画面，改为显示降级提示 + 手动输入
const fallbackMessage = ref('')

let mediaStream = null
let scanAnimationId = null
let barcodeDetector = null
let handled = false

let keyBuffer = ''
let lastKeyTime = 0

async function openScanner() {
  manualCode.value = ''
  fallbackMessage.value = ''
  handled = false

  const support = await detectScanSupport()
  if (!support.cameraScanAvailable) {
    fallbackMessage.value = support.message
  }

  visible.value = true
  await nextTick()

  if (fallbackMessage.value) {
    manualInputRef.value?.focus()
    return
  }

  try {
    const formats = WANTED_FORMATS.filter((f) => support.formats.includes(f))
    barcodeDetector = new window.BarcodeDetector({ formats: formats.length ? formats : support.formats })
  } catch (e) {
    fallbackMessage.value = '条码识别组件初始化失败，摄像头扫码不可用。请使用 PDA 扫码键，或手动输入条码。'
    await nextTick()
    manualInputRef.value?.focus()
    return
  }

  await startCamera()
}

function cameraErrorMessage(err) {
  const name = err?.name || ''
  if (name === 'NotAllowedError' || name === 'SecurityError') {
    return '摄像头权限被拒绝。请在浏览器设置中允许本站使用摄像头，或使用 PDA 扫码键 / 手动输入条码。'
  }
  if (name === 'NotFoundError' || name === 'OverconstrainedError') {
    return '未检测到可用的摄像头。请使用 PDA 扫码键，或手动输入条码。'
  }
  if (name === 'NotReadableError') {
    return '摄像头正被其他应用占用。请关闭其他应用后重试，或使用 PDA 扫码键 / 手动输入条码。'
  }
  return '无法打开摄像头。请使用 PDA 扫码键，或手动输入条码。'
}

async function startCamera() {
  stopCamera()
  try {
    mediaStream = await navigator.mediaDevices.getUserMedia({
      video: {
        facingMode: currentFacingMode.value,
        width: { ideal: 1280 },
        height: { ideal: 720 }
      },
      audio: false
    })
    if (!visible.value) {
      stopCamera()
      return
    }
    if (videoEl.value) {
      videoEl.value.srcObject = mediaStream
      await videoEl.value.play()
      const track = mediaStream.getVideoTracks()[0]
      const capabilities = track?.getCapabilities ? track.getCapabilities() : {}
      hasTorch.value = !!capabilities.torch
      startDecodingLoop()
    }
  } catch (err) {
    console.error('[Scanner] startCamera failed:', err)
    stopCamera()
    fallbackMessage.value = cameraErrorMessage(err)
    await nextTick()
    manualInputRef.value?.focus()
  }
}

function startDecodingLoop() {
  let busy = false
  const decodeFrame = async () => {
    if (!visible.value || !videoEl.value || handled) return
    if (!busy && videoEl.value.readyState >= 2 && barcodeDetector) {
      busy = true
      try {
        const barcodes = await barcodeDetector.detect(videoEl.value)
        const rawVal = barcodes?.[0]?.rawValue?.trim()
        if (rawVal) {
          onScanSuccess(rawVal)
          return
        }
      } catch (e) {
        // 单帧识别失败忽略，继续下一帧
      } finally {
        busy = false
      }
    }
    scanAnimationId = requestAnimationFrame(decodeFrame)
  }
  scanAnimationId = requestAnimationFrame(decodeFrame)
}

async function toggleTorch() {
  const track = mediaStream?.getVideoTracks()[0]
  if (!track || !hasTorch.value) return
  try {
    torchActive.value = !torchActive.value
    await track.applyConstraints({ advanced: [{ torch: torchActive.value }] })
  } catch (e) {
    torchActive.value = false
  }
}

async function switchCamera() {
  currentFacingMode.value = currentFacingMode.value === 'environment' ? 'user' : 'environment'
  await startCamera()
}

function stopCamera() {
  if (scanAnimationId) {
    cancelAnimationFrame(scanAnimationId)
    scanAnimationId = null
  }
  if (mediaStream) {
    mediaStream.getTracks().forEach((track) => track.stop())
    mediaStream = null
  }
  torchActive.value = false
  hasTorch.value = false
}

function onScanSuccess(code) {
  if (handled) return
  handled = true
  playSuccessSound()
  closeDialog()
  emit('scan', code)
}

function handleManualSubmit() {
  const code = manualCode.value.trim()
  if (!code) {
    ElMessage.warning('请输入有效条码')
    return
  }
  onScanSuccess(code)
}

function closeDialog() {
  stopCamera()
  visible.value = false
  emit('close')
}

function handleBeforeClose(done) {
  stopCamera()
  emit('close')
  done()
}

function isEditableTarget(el) {
  if (!el) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable
}

// PDA 硬件扫码键：扫码头以极短间隔输入字符并以回车结尾。
// 光标在输入框内时不拦截——由输入框自身接收条码并通过回车提交，避免重复提交。
function onHardwareKeyDown(e) {
  if (visible.value) return
  if (e.ctrlKey || e.altKey || e.metaKey) return
  if (isEditableTarget(e.target)) {
    keyBuffer = ''
    return
  }

  const now = Date.now()
  if (now - lastKeyTime > 100) keyBuffer = ''
  lastKeyTime = now

  if (e.key === 'Enter') {
    if (keyBuffer.length >= 2) {
      const code = keyBuffer.trim()
      keyBuffer = ''
      e.preventDefault()
      playSuccessSound()
      emit('scan', code)
    }
  } else if (e.key && e.key.length === 1) {
    keyBuffer += e.key
  }
}

onMounted(() => {
  if (props.listenHardware) {
    window.addEventListener('keydown', onHardwareKeyDown, true)
  }
})

onBeforeUnmount(() => {
  stopCamera()
  if (props.listenHardware) {
    window.removeEventListener('keydown', onHardwareKeyDown, true)
  }
})

defineExpose({
  open: openScanner,
  close: closeDialog
})
</script>

<style scoped lang="scss">
.scanner-container {
  display: inline-block;
  width: 100%;
}

.pda-scan-btn {
  width: 100%;
  height: 52px;
  font-size: 17px;
  font-weight: 600;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: linear-gradient(135deg, #1890ff 0%, #096dd9 100%);
  border: none;
  box-shadow: 0 4px 12px rgba(24, 144, 255, 0.35);

  &:active {
    transform: scale(0.98);
  }
}

:deep(.pda-scanner-dialog) {
  .el-dialog__header {
    margin-right: 0;
    padding: 16px 20px;
    background: #001529;
    color: #fff;
    .el-dialog__title {
      color: #fff;
      font-size: 18px;
      font-weight: 600;
    }
    .el-dialog__headerbtn .el-dialog__close {
      color: #fff;
      font-size: 20px;
    }
  }

  .el-dialog__body {
    padding: 0;
    background: #000;
    height: calc(100vh - 60px);
    display: flex;
    flex-direction: column;
  }
}

.camera-viewport {
  position: relative;
  flex: 1;
  background: #000;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}

.camera-video {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.scanner-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.45);
  pointer-events: none;
}

.scanner-guide-box {
  position: relative;
  width: 260px;
  height: 260px;
  border: 1px solid rgba(255, 255, 255, 0.3);
  box-shadow: 0 0 0 4000px rgba(0, 0, 0, 0.4);
  border-radius: 12px;

  .corner {
    position: absolute;
    width: 24px;
    height: 24px;
    border-color: #1890ff;
    border-style: solid;
  }

  .top-left {
    top: -2px;
    left: -2px;
    border-width: 4px 0 0 4px;
    border-top-left-radius: 12px;
  }
  .top-right {
    top: -2px;
    right: -2px;
    border-width: 4px 4px 0 0;
    border-top-right-radius: 12px;
  }
  .bottom-left {
    bottom: -2px;
    left: -2px;
    border-width: 0 0 4px 4px;
    border-bottom-left-radius: 12px;
  }
  .bottom-right {
    bottom: -2px;
    right: -2px;
    border-width: 0 4px 4px 0;
    border-bottom-right-radius: 12px;
  }
}

.scan-laser-line {
  position: absolute;
  top: 0;
  left: 5%;
  right: 5%;
  height: 3px;
  background: linear-gradient(90deg, rgba(255, 77, 79, 0) 0%, #ff4d4f 50%, rgba(255, 77, 79, 0) 100%);
  box-shadow: 0 0 8px #ff4d4f;
  animation: laserScan 2.2s infinite ease-in-out;
}

@keyframes laserScan {
  0% {
    top: 5%;
    opacity: 0.8;
  }
  50% {
    top: 90%;
    opacity: 1;
  }
  100% {
    top: 5%;
    opacity: 0.8;
  }
}

.scan-hint {
  color: #fff;
  font-size: 14px;
  margin-top: 24px;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.8);
}

.scanner-toolbar {
  position: absolute;
  bottom: 24px;
  display: flex;
  gap: 20px;
  background: rgba(0, 0, 0, 0.6);
  padding: 8px 18px;
  border-radius: 30px;
  backdrop-filter: blur(10px);
}

.scan-fallback {
  flex: 1;
  padding: 20px 16px;
  background: #1f2329;
  color: #e5e6eb;

  .fallback-tips {
    margin: 16px 0 0 18px;
    font-size: 14px;
    line-height: 1.9;
  }
}

.manual-input-bar {
  padding: 14px 16px;
  background: #1f2329;
  border-top: 1px solid #333;
}
</style>
