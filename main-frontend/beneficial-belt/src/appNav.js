// appNav.js —— App 级功能导航定义（编码/公司/站点/网文/漫画/星迹/剪辑/API）。
// 独立文件避免 .vue 间互相 import（SFC 不能 export 命名常量给外部用）。
// App.vue 的折角 rail 和聊天页折叠栏的「功能」悬浮菜单共用这一份定义，
// 保证两处导航列表不漂移。

import { tr } from './composables/useI18n.js'
import router from './router.js'

/**
 * 应用内页面跳转的唯一入口（App 折角 rail / 聊天页「功能」悬浮菜单 / 引导弹窗都走它）。
 *
 * 必须用 vue-router 的 push（history.pushState，不发文档请求）。
 * 桌面版页面源是 https://wails.localhost，Wails AssetServer 只服务 embed 里的静态文件，
 * 没有 /social、/game 这类前端路由对应的文件：一旦用 window.location.href 或 <a href>
 * 做整页跳转，WebView2 会真的去请求那个地址 → 404 → 窗口被「找不到此 wails.localhost 页」
 * 顶掉，整个 App 不可用（2026-09-16 实锤：悬浮菜单里所有 tab 全中招）。
 */
export function navTo(path) {
  if (!path) return
  if (router.currentRoute.value.fullPath === path) return
  router.push(path)
}

export const railItemDefinitions = [
  { id: 'chat', label: tr('编码'), icon: 'mdi:code-tags', to: '/chat' },
  { id: 'company', label: tr('Agent 公司'), icon: 'mdi:domain', to: '/company' },
  { id: 'sites', label: tr('站点'), icon: 'mdi:web', to: '/sites' },
  { id: 'publish', label: tr('网文创作'), icon: 'mdi:book-open-page-variant-outline', to: '/publish' },
  { id: 'comic', label: tr('漫画创作'), icon: 'mdi:brush', to: '/comic' },
  { id: 'game', label: tr('星迹游戏'), icon: 'mdi:gamepad-variant', to: '/game' },
  { id: 'social', label: tr('搭子'), icon: 'mdi:account-group-outline', to: '/social' },
  { id: 'studio', label: tr('视频剪辑'), icon: 'mdi:movie-edit-outline', to: '/studio' },
  { id: 'agg', label: tr('聚合 API'), icon: 'mdi:api' }
]
