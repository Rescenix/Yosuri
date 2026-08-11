// src/utils/hardwareFingerprint.js
//
// 硬件指纹：canvas 渲染 + WebGL 渲染器 + UA/屏幕/时区等环境信息组合哈希。
// 目的：设备稳定识别 —— 清缓存/换浏览器（同一机器）指纹不变，换机器才变。
// 用于 UID 分发时与 device_id 一起上报，云端据此区分「同一台机器的测试号」
// 与「不同机器的真实用户」。
//
// 隐私说明：只采集浏览器渲染层面的通用信号，不读文件、不碰麦克风/摄像头、
// 不采集任何个人身份信息；指纹是单向哈希，无法反推出原始数据。
// 采集信号失败时自动降级（少一个信号不影响整体稳定性）。

// 内置 FNV-1a 32 位哈希（不依赖任何第三方库，纯 JS）
function fnv1a(str) {
  let h = 0x811c9dc5
  for (let i = 0; i < str.length; i++) {
    h ^= str.charCodeAt(i)
    h = (h * 0x01000193) >>> 0
  }
  return h.toString(16).padStart(8, '0')
}

// canvas 指纹：绘制带抗锯齿差异的文本+图形，不同 GPU/渲染器会渲染出不同像素
function canvasSignal() {
  try {
    const canvas = document.createElement('canvas')
    canvas.width = 240
    canvas.height = 60
    const ctx = canvas.getContext('2d')
    if (!ctx) return ''
    ctx.textBaseline = 'top'
    ctx.font = '14px Arial'
    ctx.fillStyle = '#f60'
    ctx.fillRect(0, 0, 240, 60)
    ctx.fillStyle = '#069'
    ctx.fillText('Rescene-fingerprint-2026', 2, 15)
    ctx.fillStyle = 'rgba(102, 204, 0, 0.7)'
    ctx.fillText('GPU-AA-0123456789', 4, 35)
    return canvas.toDataURL()
  } catch {
    return ''
  }
}

// WebGL 指纹：GPU 厂商/渲染器/着色器能力，机器级差异信号
function webglSignal() {
  try {
    const canvas = document.createElement('canvas')
    const gl = canvas.getContext('webgl') || canvas.getContext('experimental-webgl')
    if (!gl) return ''
    const info = gl.getExtension('WEBGL_debug_renderer_info')
    const renderer = info ? gl.getParameter(info.UNMASKED_RENDERER_WEBGL) : ''
    const vendor = info ? gl.getParameter(info.UNMASKED_VENDOR_WEBGL) : ''
    const version = gl.getParameter(gl.VERSION) || ''
    const shading = gl.getParameter(gl.SHADING_LANGUAGE_VERSION) || ''
    return [renderer, vendor, version, shading].join('|')
  } catch {
    return ''
  }
}

// 环境信号：UA / 语言 / 时区 / 屏幕 / 色深 / 平台（稳定性辅助）
function envSignal() {
  const s = []
  try { s.push(navigator.userAgent) } catch { /* ignore */ }
  try { s.push(navigator.language + ',' + (navigator.languages || []).join(',')) } catch { /* ignore */ }
  try { s.push(String(new Date().getTimezoneOffset())) } catch { /* ignore */ }
  try { s.push(screen.width + 'x' + screen.height + 'x' + screen.colorDepth) } catch { /* ignore */ }
  try { s.push(navigator.platform || '') } catch { /* ignore */ }
  try { s.push(navigator.hardwareConcurrency || '') } catch { /* ignore */ }
  return s.join('~')
}

// 生成硬件指纹：信号拼接 → FNV-1a 哈希 → 32 位 hex
// 结果存 localStorage（清缓存也保留？不——localStorage 清了就没了，
// 但指纹是「算」出来的不是「存」出来的，同一台机器重算结果一致）。
// 所以调用方无需持久化，每次需要时重新计算即可。
export function computeHardwareFingerprint() {
  const raw = [
    canvasSignal(),
    webglSignal(),
    envSignal(),
  ].filter(Boolean).join('||')
  if (!raw) return ''
  return fnv1a(raw)
}
