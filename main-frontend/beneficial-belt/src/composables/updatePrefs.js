// 更新提示的本地偏好：三处共用（启动弹窗 / 弹窗跳过 / 设置页开关），key 集中定义防漂移。
export const SKIP_VERSION_KEY = 'rescene_skipped_update_version'
export const NOTIFY_DISABLED_KEY = 'rescene_update_notify_disabled'

export function getSkippedVersion() {
  return localStorage.getItem(SKIP_VERSION_KEY) || ''
}

export function setSkippedVersion(v) {
  if (v) localStorage.setItem(SKIP_VERSION_KEY, v)
  else localStorage.removeItem(SKIP_VERSION_KEY)
}

export function isUpdateNotifyDisabled() {
  return localStorage.getItem(NOTIFY_DISABLED_KEY) === '1'
}

export function setUpdateNotifyDisabled(v) {
  if (v) localStorage.setItem(NOTIFY_DISABLED_KEY, '1')
  else localStorage.removeItem(NOTIFY_DISABLED_KEY)
}

// ── 更新横幅重复提醒节流（2026-08-29 用户定稿：3 天一次太疏，没人更新）──
// 安装包已就绪但用户一直没装：同一版本 8 小时内只提醒一次（前台使用中一天最多弹 3 次），
// 每次轮询（60s）都检查是否过 8h，过了就再弹；新版本（版本串变化）立即提醒。
export const BANNER_KEY = 'rescene_banner_last_shown'
const BANNER_INTERVAL_MS = 8 * 60 * 60 * 1000 // 8 小时 = 一天最多 3 次

export function shouldShowUpdateBanner(version) {
  try {
    const raw = localStorage.getItem(BANNER_KEY)
    if (!raw) return true
    const rec = JSON.parse(raw)
    if (!rec || rec.version !== version) return true // 新版本立即提醒
    return Date.now() - (rec.ts || 0) >= BANNER_INTERVAL_MS
  } catch {
    return true
  }
}

export function markUpdateBannerShown(version) {
  try {
    localStorage.setItem(BANNER_KEY, JSON.stringify({ version, ts: Date.now() }))
  } catch { /* localStorage 不可用时忽略 */ }
}
