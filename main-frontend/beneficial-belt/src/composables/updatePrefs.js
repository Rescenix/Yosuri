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
