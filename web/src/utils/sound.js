// Copyright 2026 zhouhouping. All Rights Reserved.

// Web Audio API Synthesizer for PDA Hardware Sound Feedback
let audioCtx = null

function getAudioContext() {
  if (!audioCtx && typeof window !== 'undefined') {
    const AudioContextClass = window.AudioContext || window.webkitAudioContext
    if (AudioContextClass) {
      audioCtx = new AudioContextClass()
    }
  }
  if (audioCtx && audioCtx.state === 'suspended') {
    audioCtx.resume()
  }
  return audioCtx
}

// 成功提示音（清脆高频双音，仓库嘈杂环境易识别）
export function playSuccessSound() {
  try {
    const ctx = getAudioContext()
    if (!ctx) return

    const now = ctx.currentTime
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()

    osc.type = 'sine'
    osc.frequency.setValueAtTime(1400, now)
    osc.frequency.exponentialRampToValueAtTime(1800, now + 0.12)

    gain.gain.setValueAtTime(0.3, now)
    gain.gain.exponentialRampToValueAtTime(0.01, now + 0.15)

    osc.connect(gain)
    gain.connect(ctx.destination)

    osc.start(now)
    osc.stop(now + 0.15)

    // 震动反馈
    if (navigator.vibrate) {
      navigator.vibrate(80)
    }
  } catch (e) {
    console.warn('[Sound] playSuccessSound failed:', e)
  }
}

// 失败/错误提示音（低频警报音）
export function playErrorSound() {
  try {
    const ctx = getAudioContext()
    if (!ctx) return

    const now = ctx.currentTime
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()

    osc.type = 'sawtooth'
    osc.frequency.setValueAtTime(320, now)
    osc.frequency.setValueAtTime(260, now + 0.12)

    gain.gain.setValueAtTime(0.4, now)
    gain.gain.exponentialRampToValueAtTime(0.01, now + 0.28)

    osc.connect(gain)
    gain.connect(ctx.destination)

    osc.start(now)
    osc.stop(now + 0.28)

    // 震动反馈（双击长震）
    if (navigator.vibrate) {
      navigator.vibrate([120, 60, 180])
    }
  } catch (e) {
    console.warn('[Sound] playErrorSound failed:', e)
  }
}
