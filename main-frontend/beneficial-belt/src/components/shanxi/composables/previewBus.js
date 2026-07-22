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
export const previewRequest = reactive({ url: '', seq: 0 })

export function requestPreview(url) {
    if (!url) return
    previewRequest.url = url
    previewRequest.seq++
}
