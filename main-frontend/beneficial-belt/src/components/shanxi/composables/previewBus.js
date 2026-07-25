import { reactive } from 'vue'

// 预览请求的共享通道。
//
// 后端在四态机里检测到前端文件被改动时会推 preview_open 事件（见
// agent_workflow_handler.go 的 isFrontendEdit）。事件由 useAgentWorkflow 接住，
// 但真正要联动的是两个互不相识的组件：ChatWidget（负责把 preview 面板挂进 dock）
// 和 PreviewBrowser（负责导航到那个地址）。与其从 SSE 层往下穿两层 props/emit，
// 不如照 contextBreakdown.js / sessionTokenStats.js 的既有惯例开一个共享单例。
//
// seq 是必须的：同一个 URL 连续请求两次时 url 本身不变，只 watch url 不会触发。
// url      —— 前端导航地址（file:// 或 http(s)://）
// cdp_ws   —— 当后端用 open_browser_preview 开了真实 Chromium target 时，
//             这里带上那个 target 的 WebSocket 调试地址；PreviewBrowser 据此走
//             CDP startScreencast 真实渲染，而不是 iframe。为空则降级 iframe。
// cdp_error —— 后端创建 CDP target 前已失败时的可见错误。
export const previewRequest = reactive({ url: '', cdp_ws: '', cdp_error: '', seq: 0 })

export function requestPreview(url, cdpWs, cdpError) {
    if (!url) return
    previewRequest.url = url
    previewRequest.cdp_ws = cdpWs || ''
    previewRequest.cdp_error = cdpError || ''
    previewRequest.seq++
}
