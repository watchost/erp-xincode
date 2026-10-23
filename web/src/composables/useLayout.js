// Copyright 2026 zhouhouping. All Rights Reserved.
//
// 审批 / 消息页面同时挂在桌面布局（/workflow、/messages）和 PDA 移动布局（/pda/...）下，
// 这里统一处理“当前是否窄屏”和“页面跳转路径加前缀”。

import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'

export function useIsMobile(breakpoint = 768) {
  const isMobile = ref(typeof window !== 'undefined' ? window.innerWidth < breakpoint : false)
  let mql = null
  const onChange = (e) => {
    isMobile.value = e.matches
  }
  onMounted(() => {
    if (!window.matchMedia) return
    mql = window.matchMedia(`(max-width: ${breakpoint - 1}px)`)
    isMobile.value = mql.matches
    mql.addEventListener ? mql.addEventListener('change', onChange) : mql.addListener(onChange)
  })
  onBeforeUnmount(() => {
    if (!mql) return
    mql.removeEventListener ? mql.removeEventListener('change', onChange) : mql.removeListener(onChange)
  })
  return isMobile
}

export function useLayoutPaths() {
  const route = useRoute()
  const inPda = computed(() => route.path.startsWith('/pda'))
  const prefix = computed(() => (inPda.value ? '/pda' : ''))
  const instancePath = (no) => `${prefix.value}/workflow/instances/${encodeURIComponent(no)}`
  const todoPath = computed(() => `${prefix.value}/workflow/todo`)
  const messagesPath = computed(() => `${prefix.value}/messages`)
  // 消息里的 link 是桌面端路径；在 PDA 布局下，审批详情改走 PDA 版本，其它页面保持原路径
  const resolveMessageLink = (link) => {
    if (!link) return ''
    if (inPda.value && link.startsWith('/workflow/')) return '/pda' + link
    return link
  }
  return { inPda, instancePath, todoPath, messagesPath, resolveMessageLink }
}
