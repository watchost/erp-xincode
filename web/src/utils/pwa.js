// Copyright 2026 zhouhouping. All Rights Reserved.

import { ref } from 'vue'

export const isOnline = ref(navigator.onLine)
export const canInstallPwa = ref(false)
export const deferredInstallPrompt = ref(null)

// iOS 的 Safari 不会触发 beforeinstallprompt，只能引导用户手动“分享 → 添加到主屏幕”
const ua = typeof navigator !== 'undefined' ? navigator.userAgent : ''
export const isIos =
  /iPad|iPhone|iPod/.test(ua) || (ua.includes('Macintosh') && typeof document !== 'undefined' && 'ontouchend' in document)
export const isStandalone = ref(
  (typeof window !== 'undefined' && window.matchMedia?.('(display-mode: standalone)').matches) ||
    (typeof navigator !== 'undefined' && navigator.standalone === true)
)
export const showIosInstallGuide = ref(isIos && !isStandalone.value)

// Register Service Worker（仅生产构建注册，开发环境避免缓存干扰热更新）
export function registerServiceWorker() {
  if ('serviceWorker' in navigator && import.meta.env.PROD) {
    window.addEventListener('load', () => {
      navigator.serviceWorker
        .register('/sw.js')
        .then((reg) => {
          console.log('[PWA] Service Worker registered successfully, scope:', reg.scope)
          
          // Check for updates
          reg.addEventListener('updatefound', () => {
            const installingWorker = reg.installing
            if (installingWorker) {
              installingWorker.addEventListener('statechange', () => {
                if (installingWorker.state === 'installed' && navigator.serviceWorker.controller) {
                  console.log('[PWA] New content is available; please refresh.')
                }
              })
            }
          })
        })
        .catch((err) => {
          console.warn('[PWA] Service Worker registration failed:', err)
        })
    })
  }

  // Network Status Listeners
  window.addEventListener('online', () => {
    isOnline.value = true
    console.log('[PWA] Network status: Online')
  })

  window.addEventListener('offline', () => {
    isOnline.value = false
    console.log('[PWA] Network status: Offline')
  })

  // Capture PWA Install Prompt
  window.addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault()
    deferredInstallPrompt.value = e
    canInstallPwa.value = true
    console.log('[PWA] beforeinstallprompt captured')
  })

  window.addEventListener('appinstalled', () => {
    canInstallPwa.value = false
    deferredInstallPrompt.value = null
    isStandalone.value = true
    console.log('[PWA] App successfully installed to home screen')
  })
}

// Trigger PWA installation prompt
export async function promptInstallPwa() {
  if (!deferredInstallPrompt.value) return false
  deferredInstallPrompt.value.prompt()
  const { outcome } = await deferredInstallPrompt.value.userChoice
  if (outcome === 'accepted') {
    canInstallPwa.value = false
    deferredInstallPrompt.value = null
    return true
  }
  return false
}
