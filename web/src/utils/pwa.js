// Copyright 2026 zhouhouping. All Rights Reserved.

import { ref } from 'vue'

export const isOnline = ref(navigator.onLine)
export const canInstallPwa = ref(false)
export const deferredInstallPrompt = ref(null)

// Register Service Worker
export function registerServiceWorker() {
  if ('serviceWorker' in navigator && process.env.NODE_ENV !== 'development') {
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
