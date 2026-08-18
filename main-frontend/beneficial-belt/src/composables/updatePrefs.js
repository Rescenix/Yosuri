// 更新提示的本地偏好：三处共用（启动弹窗 / 弹窗跳过 / 设置页开关），key 集中定义防漂移。
export const SKIP_VERSION_KEY = 'rescene_skipped_update_version'
export const NOTIFY_DISABLED_KEY = 'rescene_update_notify_disabled'
export const TEST_UPDATES_KEY = 'rescene_test_updates_enabled'

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

// 是否接收测试版（alpha/beta/rc）更新：默认开启（热更新测试版本），
// 关闭后只提示正式版更新（2026-08-16 设置页版本 tab 开关）。
export function isTestUpdatesEnabled() {
  return localStorage.getItem(TEST_UPDATES_KEY) !== '0'
}

export function setTestUpdatesEnabled(v) {
  if (v) localStorage.removeItem(TEST_UPDATES_KEY)
  else localStorage.setItem(TEST_UPDATES_KEY, '0')
}

// 版本串是否为预发布（alpha/beta/rc/dev）
export function isPrereleaseVersionString(v) {
  return /-(alpha|beta|rc|dev)/i.test(v || '')
}

// ── 更新横幅重复提醒节流（2026-08-18 用户定稿「没下载就三天弹一次」）──
// 安装包已就绪但用户一直没装：同一版本 3 天内只提醒一次，3 天后再次弹横幅；
// 新版本（版本串变化）立即提醒，不继承旧版本的节流记录。
export const BANNER_KEY = 'rescene_banner_last_shown'
const BANNER_INTERVAL_MS = 3 * 24 * 60 * 60 * 1000 // 3 天

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
