// Copyright 2026 zhouhouping. All Rights Reserved.
// ERP PWA Service Worker - v1.0.0

const CACHE_NAME = 'erp-pda-cache-v2'
const PRECACHE_ASSETS = [
  '/',
  '/index.html',
  '/manifest.json',
  '/icons/icon-192x192.png',
  '/icons/icon-512x512.png',
  '/icons/icon.svg'
]

// Install event - Pre-cache critical App Shell assets
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(PRECACHE_ASSETS).catch((err) => {
        console.warn('[SW] Pre-cache warning:', err)
      })
    }).then(() => self.skipWaiting())
  )
})

// Activate event - Clean up old cache versions
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames
          .filter((name) => name !== CACHE_NAME)
          .map((name) => caches.delete(name))
      )
    }).then(() => self.clients.claim())
  )
})

// Fetch event - Handle caching strategies
self.addEventListener('fetch', (event) => {
  const request = event.request
  const url = new URL(request.url)

  // Skip non-GET requests or browser extension schemes
  if (request.method !== 'GET' || !url.protocol.startsWith('http')) {
    return
  }

  // API Requests: Let network handle it (Offline queue handles failures in client app)
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(
      fetch(request).catch(() => {
        return new Response(
          JSON.stringify({
            code: 503,
            msg: '当前处于离线状态，操作已保存在离线队列中',
            data: null,
            offline: true
          }),
          {
            status: 503,
            headers: { 'Content-Type': 'application/json' }
          }
        )
      })
    )
    return
  }

  // Navigation Requests (SPA routes like /pda, /pda/inbound):
  // Network first -> 离线时统一回退到缓存的 index.html（SPA 外壳），由前端路由接管
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response && response.status === 200) {
            const responseClone = response.clone()
            caches.open(CACHE_NAME).then((cache) => cache.put('/index.html', responseClone))
          }
          return response
        })
        .catch(async () => {
          const shell = (await caches.match('/index.html')) || (await caches.match('/'))
          return shell || new Response('当前离线，且尚未缓存应用页面，请联网后重新打开。', {
            status: 503,
            headers: { 'Content-Type': 'text/plain; charset=utf-8' }
          })
        })
    )
    return
  }

  // Static Assets (JS, CSS, Images, Fonts): Stale-While-Revalidate
  event.respondWith(
    caches.match(request).then((cachedResponse) => {
      const fetchPromise = fetch(request)
        .then((networkResponse) => {
          if (networkResponse && networkResponse.status === 200) {
            const responseClone = networkResponse.clone()
            caches.open(CACHE_NAME).then((cache) => cache.put(request, responseClone))
          }
          return networkResponse
        })
        .catch(() => cachedResponse)

      return cachedResponse || fetchPromise
    })
  )
})
