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
      :before-close="handleClose"
    >
      <div class="camera-viewport">
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
            @click="handleClose"
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
          placeholder="或手动输入/扫码枪条码"
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
import { playSuccessSound, playErrorSound } from '@/utils/sound'

const props = defineProps({
  // 是否自动开启全局硬件扫码枪键盘监听
  listenHardware: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['scan', 'close'])

const visible = ref(false)
const videoEl = ref(null)
const manualInputRef = ref(null)
const manualCode = ref('')
const hasTorch = ref(false)
const torchActive = ref(false)
const currentFacingMode = ref('environment') // 默认后置环境摄像头

let mediaStream = null
let scanAnimationId = null
let barcodeDetector = null

// 硬件扫码枪缓冲记录
let keyBuffer = ''
let lastKeyTime = 0

// 开启扫描器
async function openScanner() {
  visible.value = true
  manualCode.value = ''
  await nextTick()
  initBarcodeDetector()
  await startCamera()
}

// 初始化 BarcodeDetector API (现代浏览器 / Android PDA 原生支持)
function initBarcodeDetector() {
  if ('BarcodeDetector' in window) {
    try {
      barcodeDetector = new window.BarcodeDetector({
        formats: ['qr_code', 'code_128', 'code_39', 'ean_13', 'ean_8', 'upc_a', 'data_matrix']
      })
    } catch (e) {
      console.warn('[Scanner] BarcodeDetector init failed:', e)
    }
  }
}

// 启动摄像头视频流
async function startCamera() {
  stopCamera()
  try {
    const constraints = {
      video: {
        facingMode: currentFacingMode.value,
        width: { ideal: 1280 },
        height: { ideal: 720 }
      },
      audio: false
    }

    mediaStream = await navigator.mediaDevices.getUserMedia(constraints)
    if (videoEl.value) {
      videoEl.value.srcObject = mediaStream
      await videoEl.value.play()

      // 检测闪光灯能力
      const track = mediaStream.getVideoTracks()[0]
      const capabilities = track.getCapabilities ? track.getCapabilities() : {}
      hasTorch.value = !!capabilities.torch

      startDecodingLoop()
    }
  } catch (err) {
    console.error('[Scanner] startCamera failed:', err)
    ElMessage.warning('无法打开摄像头，您可使用手动输入或PDA激光扫码枪')
  }
}

// 持续解码帧循环
function startDecodingLoop() {
  const decodeFrame = async () => {
    if (!visible.value || !videoEl.value) return

    if (videoEl.value.readyState >= 2) {
      if (barcodeDetector) {
        try {
          const barcodes = await barcodeDetector.detect(videoEl.value)
          if (barcodes && barcodes.length > 0) {
            const rawVal = barcodes[0].rawValue.trim()
            if (rawVal) {
              onScanSuccess(rawVal)
              return // 扫码成功，中断本轮循环
            }
          }
        } catch (e) {
          // ignore frame decode errors
        }
      }
    }

    scanAnimationId = requestAnimationFrame(decodeFrame)
  }

  scanAnimationId = requestAnimationFrame(decodeFrame)
}

// 切换闪光灯
async function toggleTorch() {
  if (!mediaStream) return
  const track = mediaStream.getVideoTracks()[0]
  if (track && hasTorch.value) {
    try {
      torchActive.value = !torchActive.value
      await track.applyConstraints({
        advanced: [{ torch: torchActive.value }]
      })
    } catch (e) {
      console.warn('[Scanner] toggleTorch failed:', e)
    }
  }
}

// 切换前后摄像头
async function switchCamera() {
  currentFacingMode.value = currentFacingMode.value === 'environment' ? 'user' : 'environment'
  await startCamera()
}

// 停止摄像头
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
}

// 扫码成功回调
function onScanSuccess(code) {
  playSuccessSound()
  emit('scan', code)
  handleClose()
}

// 手动输入确认
function handleManualSubmit() {
  const code = manualCode.value.trim()
  if (!code) {
    ElMessage.warning('请输入有效条码')
    return
  }
  onScanSuccess(code)
}

function handleClose() {
  stopCamera()
  visible.value = false
  emit('close')
}

// 硬件激光扫码枪键盘事件拦截（按键间隔 < 50ms 且以 Enter 结尾通常为扫码枪）
function onHardwareKeyDown(e) {
  if (!props.listenHardware) return
  // 忽略常见系统功能键
  if (e.ctrlKey || e.altKey || e.metaKey) return

  const now = Date.now()
  if (now - lastKeyTime > 200) {
    keyBuffer = '' // 超时重置缓冲
  }
  lastKeyTime = now

  if (e.key === 'Enter') {
    if (keyBuffer.length >= 2) {
      const scannedCode = keyBuffer.trim()
      keyBuffer = ''
      // 若当前弹窗打开则关闭
      if (visible.value) {
        handleClose()
      }
      playSuccessSound()
      emit('scan', scannedCode)
      e.preventDefault()
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
  close: handleClose
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

.manual-input-bar {
  padding: 14px 16px;
  background: #1f2329;
  border-top: 1px solid #333;
}
</style>
