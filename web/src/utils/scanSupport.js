// Copyright 2026 zhouhouping. All Rights Reserved.
//
// 摄像头扫码能力检测。
// 摄像头识别依赖两项浏览器能力：
//   1. getUserMedia（且必须在 HTTPS / localhost 安全上下文中）
//   2. BarcodeDetector（Chrome/Edge Android 支持；iOS Safari、Firefox 等不支持）
// 任一不满足时，摄像头扫码不可用，必须降级为 PDA 硬件扫码键或手动输入。

let cached = null

export async function detectScanSupport() {
  if (cached) return cached

  const secure = typeof window !== 'undefined' && window.isSecureContext === true
  const camera = secure && !!(navigator.mediaDevices && navigator.mediaDevices.getUserMedia)
  let formats = []
  if (typeof window !== 'undefined' && 'BarcodeDetector' in window) {
    try {
      formats = window.BarcodeDetector.getSupportedFormats
        ? await window.BarcodeDetector.getSupportedFormats()
        : ['qr_code', 'code_128']
    } catch (e) {
      formats = []
    }
  }
  const detector = formats.length > 0

  let message = ''
  if (!secure) {
    message = '当前页面不是 HTTPS 连接，浏览器禁止调用摄像头。请使用 PDA 扫码键，或在输入框中手动输入条码。'
  } else if (!camera) {
    message = '当前浏览器无法调用摄像头。请使用 PDA 扫码键，或在输入框中手动输入条码。'
  } else if (!detector) {
    message = '当前浏览器（例如 iOS Safari、Firefox）不支持网页条码识别，摄像头扫码不可用。请使用 PDA 扫码键，或在输入框中手动输入条码。'
  }

  cached = {
    secure,
    camera,
    detector,
    formats,
    cameraScanAvailable: camera && detector,
    message
  }
  return cached
}
