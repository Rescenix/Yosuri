<template>
  <section class="git-delivery-agent" :class="{ minimized: isMinimized }" aria-label="交付 Agent">
    <button
      v-if="isMinimized"
      type="button"
      class="gda-minimized-bar"
      title="恢复 Agent 工作台"
      aria-label="恢复 Agent 工作台"
      @click="isMinimized = false"
    >
      <span class="gda-minimized-icon">
        <Icon icon="mdi:source-branch-check" width="17" />
        <i v-if="reviewing" aria-label="正在运行"></i>
      </span>
      <span class="gda-minimized-name">{{ activeAgent.name }}</span>
      <span v-if="reviewing" class="gda-minimized-status">{{ elapsedSeconds }}s</span>
      <Icon icon="mdi:window-restore" width="17" />
    </button>
    <template v-else>
    <header class="gda-header">
      <div class="gda-tabs" role="tablist">
        <button
          v-for="agent in tabAgents"
          :key="agent.id"
          type="button"
          class="gda-tab"
          :class="{ active: agent.id === activeAgentId }"
          role="tab"
          :aria-selected="agent.id === activeAgentId"
          @click="selectAgent(agent.id)"
        ><Icon icon="mdi:source-branch-check" width="16" /> {{ agent.name }}</button>
        <button type="button" class="gda-add-tab" title="添加已配置 Agent" @click="showAgentPicker = !showAgentPicker"><Icon icon="mdi:plus" width="17" /></button>
        <div v-if="showAgentPicker" class="gda-agent-picker">
          <button v-for="agent in availableAgents" :key="agent.id" type="button" @click="openAgentTab(agent.id)"><Icon icon="mdi:source-branch-check" width="15" /> {{ agent.name }}</button>
          <p v-if="!availableAgents.length">所有已配置 Agent 都已打开</p>
          <button type="button" class="gda-manage-agents" @click="showAgentPicker = false; showSettings = true"><Icon icon="mdi:cog-outline" width="15" /> 管理 Agent…</button>
        </div>
      </div>
      <div class="gda-actions">
        <button type="button" title="最小化" aria-label="最小化 Agent 工作台" @click="minimizeAgent"><Icon icon="mdi:window-minimize" width="18" /></button>
        <button type="button" title="管理 Agent" @click="showSettings = true"><Icon icon="mdi:cog-outline" width="17" /></button>
        <button type="button" title="关闭" @click="$emit('close')"><Icon icon="mdi:close" width="18" /></button>
      </div>
    </header>

    <main ref="messagesRef" class="gda-messages">
      <div v-for="message in activeMessages" :key="message.id" class="gda-message" :class="message.role">
        <template v-if="message.role === 'activity'">
          <div class="gda-activity">{{ message.content }}</div>
        </template>
        <template v-else>
        <div v-if="message.role === 'agent'" class="gda-avatar"><Icon icon="mdi:source-branch-check" width="14" /></div>
        <div class="gda-bubble">{{ message.content }}</div>
        </template>
      </div>
      <div v-if="reviewing" class="gda-message agent"><div class="gda-avatar">…</div><div class="gda-bubble typing"><i></i><i></i><i></i></div></div>
      <section v-if="pendingApproval" class="gda-approval">
        <strong>等待你的批准</strong>
        <span>{{ pendingApproval.tool }}</span>
        <pre>{{ pendingApproval.args }}</pre>
        <div><button type="button" @click="respondApproval(false)">拒绝</button><button type="button" @click="respondApproval(true)">允许</button></div>
      </section>
    </main>

    <form class="gda-input" @submit.prevent="sendMessage()">
      <textarea v-model="draft" rows="1" placeholder="输入要交给 Agent 的任务…" @keydown.enter.exact.prevent="sendMessage()" @keydown.esc="$emit('close')"></textarea>
      <span v-if="reviewing" class="gda-running">{{ elapsedSeconds }}s</span>
      <button v-if="reviewing" type="button" class="gda-stop" title="停止审查" @click="cancelReview"><Icon icon="mdi:stop" width="15" /></button>
      <button type="submit" :disabled="!draft.trim() || reviewing" title="发送"><Icon icon="mdi:arrow-up" width="17" /></button>
    </form>

    <Teleport to="body">
      <div v-if="showSettings" class="gda-settings-backdrop" @click.self="showSettings = false">
        <section class="gda-settings-dialog" role="dialog" aria-modal="true" aria-label="Git Agent 设置">
          <header><strong>Agent 设置</strong><button type="button" @click="showSettings = false"><Icon icon="mdi:close" width="18" /></button></header>
          <div class="gda-agent-list">
            <article v-for="(agent, index) in agents" :key="agent.id" class="gda-agent-form">
              <div class="gda-agent-form-head"><span>Agent {{ index + 1 }}</span><button v-if="agents.length > 1" type="button" title="删除 Agent" @click="removeAgent(agent.id)"><Icon icon="mdi:trash-can-outline" width="15" /></button></div>
              <label>名称<input v-model.trim="agent.name" maxlength="32" placeholder="Agent 名称" /></label>
              <label>系统提示词<textarea v-model="agent.prompt" rows="6" /></label>
              <details class="gda-agent-ref">
                <summary>
                  <Icon icon="mdi:paperclip" width="14" /> 参考资料
                  <span v-if="agent.refContent" class="gda-ref-badge">{{ agent.refFileName }}</span>
                </summary>
                <div class="gda-ref-file">
                  <label class="gda-ref-file-btn">
                    <Icon icon="mdi:file-plus-outline" width="13" /> {{ agent.refContent ? '更换文件' : '选择文件' }}
                    <input type="file" class="gda-ref-file-input" accept=".txt,.md,.json,.yaml,.yml,.toml,.ini,.cfg,.cs,.xaml,.sln,.env" @change="onRefFileSelected(agent, $event)" />
                  </label>
                  <button v-if="agent.refContent" type="button" class="gda-ref-file-remove" @click="removeRefFile(agent)"><Icon icon="mdi:close" width="13" /></button>
                </div>
                <p class="gda-ref-hint">文件内容将作为可选参考资料随上下文提供给 Agent，由 Agent 自行决定是否需要参考。</p>
              </details>
            </article>
          </div>
          <footer><button type="button" class="gda-add-agent" @click="addAgent"><Icon icon="mdi:plus" width="16" /> 添加 Agent</button><button type="button" class="gda-settings-done" @click="showSettings = false">完成</button></footer>
        </section>
      </div>
    </Teleport>
    </template>
  </section>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'

defineEmits(['close'])
const STORAGE_KEY = 'gitDeliveryAgents'
const DEFAULT_PROMPT = `你是 Git Agent，负责交付前的只读代码审查。必须使用当前工作树的 git diff 作为事实依据，绝不写入、暂存、提交、推送或删除文件。运行环境是 Windows PowerShell：多条命令必须用分号 ; 分隔，绝不能使用 && 或 ||。单条命令失败时先解释错误，不要原样重复失败命令。用简洁中文说明改动、风险和下一步；重点关注密钥、垃圾文件、二进制构建产物、危险脚本、回归和测试缺口。没有问题时明确说明可以交付，不要编造问题。`
const GENERIC_PROMPT = `你是一个通用工作 Agent。先理解用户目标，再使用可用工具完成任务；执行命令前确认环境和参数，失败后解释原因并换用安全方案。用简洁中文汇报进展和结果。`
const GIT_REVIEW_TASK = '请审查当前工作树这次改动，确认是否适合交付。'
function defaultAgent() { return { id: 'git-agent', name: 'Git Agent', prompt: DEFAULT_PROMPT, refContent: '', refFileName: '' } }
function readAgents() {
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]')
    return Array.isArray(saved) && saved.length ? saved : [defaultAgent()]
  } catch { return [defaultAgent()] }
}
const agents = ref(readAgents())
const activeAgentId = ref(agents.value[0].id)
const openAgentIds = ref([agents.value[0].id])
const messagesByAgent = ref({})
const draft = ref('')
const reviewing = ref(false)
const elapsedSeconds = ref(0)
const pendingApproval = ref(null)
const showSettings = ref(false)
const showAgentPicker = ref(false)
const isMinimized = ref(false)
const messagesRef = ref(null)
let reviewStream = null
let elapsedTimer = null
let thinkingMessageId = ''
const tabAgents = computed(() => {
  const open = agents.value.filter(agent => openAgentIds.value.includes(agent.id))
  return open.length ? open : [agents.value[0]]
})
const availableAgents = computed(() => agents.value.filter(agent => !openAgentIds.value.includes(agent.id)))
const activeAgent = computed(() => tabAgents.value.find(agent => agent.id === activeAgentId.value) || tabAgents.value[0])
const activeMessages = computed(() => messagesByAgent.value[activeAgentId.value] || [])
watch(agents, value => localStorage.setItem(STORAGE_KEY, JSON.stringify(value)), { deep: true })
watch([activeAgentId, activeMessages, reviewing], scrollToBottom, { flush: 'post' })
function scrollToBottom() { nextTick(() => { if (messagesRef.value) messagesRef.value.scrollTop = messagesRef.value.scrollHeight }) }
function pushMessage(role, content) {
  const id = activeAgentId.value
  const next = { ...messagesByAgent.value }
  next[id] = [...(next[id] || []), { id: `${Date.now()}-${Math.random()}`, role, content }]
  messagesByAgent.value = next
}
function pushActivity(content, icon = 'mdi:circle-outline', loading = false) {
  const id = activeAgentId.value
  const next = { ...messagesByAgent.value }
  next[id] = [...(next[id] || []), { id: `${Date.now()}-${Math.random()}`, role: 'activity', content, icon, loading }]
  messagesByAgent.value = next
}
function appendThinking(content) {
  const agentId = activeAgentId.value
  const entries = messagesByAgent.value[agentId] || []
  const index = entries.findIndex(message => message.id === thinkingMessageId)
  if (index < 0) {
    pushActivity(`思考：${content}`)
    thinkingMessageId = messagesByAgent.value[agentId].at(-1).id
    return
  }
  const next = { ...messagesByAgent.value }
  next[agentId] = [...entries]
  next[agentId][index] = { ...entries[index], content: entries[index].content + content }
  messagesByAgent.value = next
}
function stopReview() {
  reviewStream?.close()
  reviewStream = null
  reviewing.value = false
  clearInterval(elapsedTimer)
  elapsedTimer = null
  elapsedSeconds.value = 0
  pendingApproval.value = null
}
function startReviewClock() {
  clearInterval(elapsedTimer)
  elapsedSeconds.value = 0
  elapsedTimer = setInterval(() => { elapsedSeconds.value++ }, 1000)
}
function cancelReview() {
  if (!reviewing.value) return
  stopReview()
  pushActivity('已停止本次审查。')
}
async function respondApproval(allow) {
  const approval = pendingApproval.value
  if (!approval) return
  try {
    const res = await fetch('/api/code/workflow/approve', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: approval.id, allow, remember: false, tool: approval.tool })
    })
    if (!res.ok) throw new Error(await res.text())
    pushActivity(allow ? `已允许工具：${approval.tool}` : `已拒绝工具：${approval.tool}`)
    pendingApproval.value = null
  } catch (error) {
    pushActivity(`审批提交失败：${error.message || '未知错误'}`)
  }
}
function taskFor(text) {
  let base = `执行环境规则：当前命令终端是 Windows PowerShell。多条命令只允许用分号 ; 分隔，绝不能使用 && 或 ||；命令失败后不要原样重试。\n\n${activeAgent.value.prompt}`
  if (activeAgent.value.refContent) {
    base += `\n\n---\n【可选参考资料 - 如需确认项目背景或规范时查阅，无关可忽略】\n${activeAgent.value.refContent}\n---`
  }
  return `${base}\n\n用户请求：${text}`
}
function sendMessage(initialText = '') {
  const text = (initialText || draft.value).trim()
  if (!text || reviewing.value) return
  draft.value = ''
  thinkingMessageId = ''
  pushMessage('user', text)
  reviewing.value = true
  startReviewClock()
  pushActivity('已启动审查，等待 Git Agent 响应…', 'mdi:clock-outline', true)
  reviewStream = new EventSource('/api/code/workflow?mode=ask&task=' + encodeURIComponent(taskFor(text)))
  reviewStream.addEventListener('workflow_start', event => {
    try { pushActivity(`工作流已启动：${JSON.parse(event.data).mode === 'ask' ? '安全审批模式' : '自动模式'}`) } catch {}
  })
  reviewStream.addEventListener('thinking', event => {
    try {
      const content = JSON.parse(event.data).content || ''
      if (content) appendThinking(content)
    } catch { pushActivity('Git Agent 正在思考…') }
  })
  reviewStream.addEventListener('action', event => {
    try {
      const data = JSON.parse(event.data)
      pushActivity(`正在调用工具：${data.name || 'unknown'}`, 'mdi:tools', true)
    } catch { pushActivity('正在调用工具…', 'mdi:tools', true) }
  })
  reviewStream.addEventListener('approval_request', event => {
    try {
      const data = JSON.parse(event.data)
      pendingApproval.value = { id: data.id, tool: data.tool || 'unknown', args: data.args || '' }
      pushActivity(`工具正在等待批准：${data.tool || 'unknown'}`)
    } catch { pushActivity('工具正在等待批准。') }
  })
  reviewStream.addEventListener('result', event => {
    try {
      const data = JSON.parse(event.data)
      const state = data.ok === false ? '工具失败' : '工具完成'
      pushActivity(`${state}：${data.name || 'unknown'}${data.output ? `\n${data.output}` : ''}`, data.ok === false ? 'mdi:alert-circle-outline' : 'mdi:check-circle-outline')
    } catch { pushActivity('工具已返回结果。', 'mdi:check-circle-outline') }
  })
  reviewStream.addEventListener('flow_error', event => {
    try { pushMessage('agent', `审查未完成：${JSON.parse(event.data).message || '模型服务不可用。'}`) } catch { pushMessage('agent', '审查未完成：模型服务不可用。') }
  })
  reviewStream.addEventListener('workflow_done', event => {
    try {
      const data = JSON.parse(event.data)
      if (data.final_output) pushMessage('agent', data.final_output)
      else if (data.status !== 'completed') pushMessage('agent', `审查未完成：${data.status || '工作流已停止'}。`)
    } catch { pushMessage('agent', '审查工作流已结束。') }
    stopReview()
  })
  reviewStream.onerror = () => { if (reviewing.value) { pushMessage('agent', '无法连接 Git Agent，请检查模型配置。'); stopReview() } }
}
function selectAgent(id) { if (id !== activeAgentId.value) { stopReview(); activeAgentId.value = id } }
function openAgentTab(id) {
  if (!openAgentIds.value.includes(id)) openAgentIds.value.push(id)
  showAgentPicker.value = false
  selectAgent(id)
  if (!(messagesByAgent.value[id] || []).length) draft.value = id === 'git-agent' ? GIT_REVIEW_TASK : '请协助我完成当前任务。'
}
function addAgent() {
  const agent = { id: `agent-${Date.now()}`, name: '新 Agent', prompt: GENERIC_PROMPT, refContent: '', refFileName: '' }
  agents.value.push(agent)
}
function removeAgent(id) {
  if (agents.value.length === 1) return
  agents.value = agents.value.filter(agent => agent.id !== id)
  openAgentIds.value = openAgentIds.value.filter(agentId => agentId !== id)
  if (activeAgentId.value === id) activeAgentId.value = agents.value[0].id
}
function minimizeAgent() {
  showAgentPicker.value = false
  showSettings.value = false
  isMinimized.value = true
}
function onRefFileSelected(agent, event) {
  const file = event.target.files?.[0]
  if (!file) return
  agent.refFileName = file.name
  const reader = new FileReader()
  reader.onload = e => { agent.refContent = e.target.result }
  reader.readAsText(file)
  event.target.value = ''
}
function removeRefFile(agent) {
  agent.refContent = ''
  agent.refFileName = ''
}
onMounted(() => { draft.value = GIT_REVIEW_TASK })
onUnmounted(stopReview)
</script>

<style scoped>
.git-delivery-agent { position:fixed; right:24px; bottom:24px; z-index:2100; width:min(420px,calc(100vw - 32px)); height:min(560px,calc(100vh - 48px)); display:flex; flex-direction:column; overflow:hidden; color:#252525; background:#fff; border:1px solid #e7e7e7; border-radius:14px; box-shadow:0 18px 52px rgba(0,0,0,.18); font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif; }
.gda-header { min-height:50px; display:flex; align-items:stretch; justify-content:space-between; border-bottom:1px solid #ededed; }.gda-tabs { position:relative; flex:1; min-width:0; display:flex; overflow:visible; }.gda-tab,.gda-add-tab { flex:0 1 auto; display:inline-flex; align-items:center; gap:7px; padding:0 16px; border:0; border-right:1px solid #eee; color:#777; background:#fafafa; font-size:12.5px; cursor:pointer; white-space:nowrap; }.gda-tab { max-width:145px; overflow:hidden; text-overflow:ellipsis; }.gda-tab.active { color:#242424; background:#fff; box-shadow:inset 0 -2px #242424; font-weight:650; }.gda-add-tab { flex:0 0 auto; width:34px; justify-content:center; padding:0; background:#fff; color:#666; }.gda-add-tab:hover { color:#242424; background:#f3f3f3; }.gda-agent-picker { position:absolute; top:50px; left:8px; z-index:31000; min-width:180px; padding:5px; border:1px solid #ddd; border-radius:9px; background:#fff; box-shadow:0 10px 28px rgba(0,0,0,.16); }.gda-agent-picker button { width:100%; display:flex; align-items:center; gap:7px; padding:8px; border:0; border-radius:6px; color:#333; background:transparent; text-align:left; font-size:12px; cursor:pointer; }.gda-agent-picker button:hover { background:#f2f2f2; }.gda-agent-picker p { margin:7px 8px; color:#777; font-size:11px; }.gda-agent-picker .gda-manage-agents { margin-top:3px; border-top:1px solid #eee; border-radius:0; color:#6c4934; }.gda-actions { flex:0 0 auto; display:flex; align-items:center; gap:2px; padding:0 8px; background:#fff; }.gda-actions button { width:29px; height:29px; display:inline-flex; align-items:center; justify-content:center; border:0; border-radius:7px; color:#666; background:transparent; cursor:pointer; }.gda-actions button:hover { color:#222; background:#f1f1f1; }
.gda-messages { flex:1; overflow:auto; padding:16px; background:#fff; }.gda-message { display:flex; gap:8px; margin:0 0 12px; }.gda-message.user { justify-content:flex-end; }.gda-message.activity { margin:4px 0 8px 34px; }.gda-activity { max-width:90%; display:flex; align-items:flex-start; gap:6px; color:#727272; font-size:11.5px; line-height:1.45; white-space:pre-wrap; }.gda-activity :deep(svg) { flex:0 0 auto; margin-top:1px; color:#8b6046; }.gda-avatar { width:26px; height:26px; flex:0 0 auto; display:grid; place-items:center; border-radius:8px; color:#79533c; background:#f6eee9; }.gda-bubble { max-width:82%; padding:9px 11px; border-radius:10px; color:#303030; background:#f5f5f5; font-size:12.5px; line-height:1.55; white-space:pre-wrap; }.gda-message.user .gda-bubble { color:#fff; background:#262626; }.gda-bubble.typing { display:flex; align-items:center; gap:4px; min-width:36px; }.typing i { width:4px; height:4px; border-radius:50%; background:#777; animation:blink 1.15s infinite ease-in-out; }.typing i:nth-child(2){animation-delay:.15s}.typing i:nth-child(3){animation-delay:.3s}
.gda-input { display:flex; align-items:flex-end; gap:8px; padding:10px 12px; border-top:1px solid #ececec; background:#fff; }.gda-input textarea { flex:1; min-height:20px; max-height:110px; padding:7px 0; resize:none; border:0; outline:0; color:#242424; font:13px/1.45 inherit; }.gda-input textarea::placeholder { color:#999; }.gda-input button { width:28px; height:28px; display:grid; place-items:center; flex:0 0 auto; border:0; border-radius:8px; color:#fff; background:#242424; cursor:pointer; }.gda-input .gda-stop { color:#6f352f; background:#f6e9e7; }.gda-input button:disabled { opacity:.35; cursor:default; }
.gda-approval { margin:4px 0 12px 34px; padding:10px; border:1px solid #e9cda9; border-radius:9px; background:#fff8ee; }.gda-approval strong,.gda-approval span { display:block; font-size:12px; }.gda-approval span { margin-top:3px; color:#754d25; font-family:ui-monospace,Consolas,monospace; }.gda-approval pre { max-height:90px; overflow:auto; margin:8px 0; padding:7px; border-radius:6px; color:#5b4b3b; background:rgba(255,255,255,.7); font:10px/1.4 ui-monospace,Consolas,monospace; white-space:pre-wrap; }.gda-approval div { display:flex; justify-content:flex-end; gap:7px; }.gda-approval button { border:0; border-radius:6px; padding:5px 9px; font-size:11px; cursor:pointer; }.gda-approval button:first-child { color:#753a34; background:#f6e9e7; }.gda-approval button:last-child { color:#fff; background:#262626; }.gda-settings-backdrop { position:fixed; inset:0; z-index:50000; display:grid; place-items:center; background:rgba(0,0,0,.22); }.gda-settings-dialog { width:min(520px,calc(100vw - 32px)); max-height:min(660px,calc(100vh - 32px)); display:flex; flex-direction:column; overflow:hidden; color:#292929; background:#fff; border-radius:14px; box-shadow:0 18px 56px rgba(0,0,0,.24); }.gda-settings-dialog > header,.gda-settings-dialog > footer { display:flex; align-items:center; justify-content:space-between; padding:13px 16px; border-bottom:1px solid #eee; }.gda-settings-dialog > header strong { font-size:14px; }.gda-settings-dialog > header button,.gda-agent-form-head button { border:0; color:#666; background:transparent; cursor:pointer; }.gda-agent-list { overflow:auto; padding:14px 16px; }.gda-agent-form { margin-bottom:14px; padding:12px; border:1px solid #e7e7e7; border-radius:10px; }.gda-agent-form-head { display:flex; justify-content:space-between; margin-bottom:10px; color:#555; font-size:12px; font-weight:650; }.gda-agent-form label { display:grid; gap:5px; margin-top:9px; color:#777; font-size:11px; }.gda-agent-form input,.gda-agent-form textarea { box-sizing:border-box; width:100%; padding:7px 8px; border:1px solid #ddd; border-radius:7px; color:#303030; font:12px/1.45 inherit; }.gda-agent-form textarea { resize:vertical; }.gda-settings-dialog > footer { border-top:1px solid #eee; border-bottom:0; }.gda-settings-dialog footer button { border:0; border-radius:8px; padding:7px 10px; cursor:pointer; font-size:12px; }.gda-add-agent { display:inline-flex; align-items:center; gap:5px; color:#60412f; background:#f5eee9; }.gda-settings-done { color:#fff; background:#242424; }.spin { animation:spin .9s linear infinite; }@keyframes spin { to { transform:rotate(360deg) } }@keyframes blink { 0%,80%,100%{opacity:.28}40%{opacity:1} }
.gda-agent-ref { margin-top:9px; border:1px solid #ececec; border-radius:7px; overflow:hidden; }
.gda-agent-ref summary { display:flex; align-items:center; gap:6px; padding:6px 8px; color:#666; font-size:11px; cursor:pointer; user-select:none; }
.gda-agent-ref summary:hover { background:#f6f6f6; }
.gda-ref-file { display:flex; align-items:center; gap:6px; padding:6px 8px; border-top:1px solid #ececec; }
.gda-ref-file-btn { display:inline-flex; align-items:center; gap:5px; padding:4px 8px; border:1px solid #ddd; border-radius:6px; color:#555; background:#fafafa; font-size:11px; cursor:pointer; user-select:none; }
.gda-ref-file-btn:hover { background:#f0f0f0; }
.gda-ref-file-input { display:none; }
.gda-ref-file-remove { display:inline-flex; align-items:center; justify-content:center; width:20px; height:20px; border:0; border-radius:4px; color:#999; background:transparent; cursor:pointer; }
.gda-ref-file-remove:hover { color:#753a34; background:#f6e9e7; }
.gda-ref-badge { margin-left:auto; max-width:120px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; padding:1px 6px; border-radius:5px; color:#6c4934; background:#f6eee9; font-size:10px; font-weight:500; }
.gda-ref-hint { margin:4px 8px 6px; color:#999; font-size:10.5px; line-height:1.4; }
.git-delivery-agent.minimized {
  right: 18px;
  bottom: 16px;
  width: min(232px, calc(100vw - 32px));
  height: 48px;
  border-color: #dedede;
  border-radius: 14px;
  box-shadow: 0 10px 32px rgba(0, 0, 0, .16);
  transition: width .2s ease, height .2s ease, box-shadow .2s ease;
}
.gda-minimized-bar {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 12px;
  border: 0;
  color: #2d2d2d;
  background: #fff;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  line-height: 1;
  text-align: left;
  cursor: pointer;
}
.gda-minimized-bar:hover { background: #f7f7f7; }
.gda-minimized-bar:focus-visible { outline: 2px solid #555; outline-offset: -3px; }
.gda-minimized-icon {
  position: relative;
  width: 28px;
  height: 28px;
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  border-radius: 9px;
  color: #704c36;
  background: #f6eee9;
}
.gda-minimized-icon i {
  position: absolute;
  right: -1px;
  bottom: -1px;
  width: 7px;
  height: 7px;
  border: 2px solid #fff;
  border-radius: 50%;
  background: #55a46c;
  animation: blink 1.4s ease-in-out infinite;
}
.gda-minimized-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.gda-minimized-status { color: #777; font-size: 11px; font-weight: 500; }
</style>
