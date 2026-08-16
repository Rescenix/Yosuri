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
