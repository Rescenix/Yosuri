<template>
  <router-view />
  <UpdateModal v-if="showUpdate" :update="updateInfo" @close="showUpdate = false" />
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useAuth } from './composables/useAuth.js'
import { getSkippedVersion, isUpdateNotifyDisabled } from './composables/updatePrefs.js'
import UpdateModal from './components/shanxi/chat/UpdateModal.vue'

const auth = useAuth()

const showUpdate = ref(false)
const updateInfo = ref(null)

// GitHub OAuth 回调回收：GitHubCallback 把 JWT 通过 ?token= 带回前端首页。
// 先存好 token 再派发 auth-change，由 useAuth 统一去 /api/auth/me 验真并拉用户名/头像；
// 伪造/过期的 token 会被 useAuth 清掉，不会误判登录成功。URL 里的 token 立即清掉防泄露。
onMounted(() => {
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token')
  if (token) {
    const url = new URL(window.location.href)
    url.searchParams.delete('token')
    window.history.replaceState({}, document.title, url.pathname + url.search)

    localStorage.setItem('token', token)
    window.dispatchEvent(new Event('auth-change'))
  }
})

// 启动时检查更新：GitHub 最新 release 比当前版本新就弹窗（附更新内容）。
// 检查失败（离线/限流）静默跳过，不打扰；设置了「不提示」或跳过过该版本也不弹。
onMounted(async () => {
  try {
    if (isUpdateNotifyDisabled()) return
    const res = await fetch('/api/update/check')
    const data = await res.json()
    if (data.ok && data.update?.has_update) {
      if (getSkippedVersion() === data.update.latest_version) return
      updateInfo.value = data.update
      showUpdate.value = true
    }
  } catch {
    /* 静默：无网络或后端未就绪时不打扰用户 */
  }
})
</script>
