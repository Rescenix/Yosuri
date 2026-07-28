<template>
  <section class="git-delivery-agent" aria-label="交付 Agent Git">
    <header class="gda-header">
      <div class="gda-title">
        <Icon icon="mdi:source-branch-check" width="18" />
        <div><strong>{{ agentName }}</strong><span>交付前只读审查</span></div>
      </div>
      <div class="gda-header-actions">
        <button type="button" title="Git Agent 设置" @click="showSettings = !showSettings"><Icon icon="mdi:cog-outline" width="17" /></button>
        <button type="button" title="关闭" @click="$emit('close')"><Icon icon="mdi:close" width="18" /></button>
      </div>
    </header>

    <div v-if="showSettings" class="gda-settings">
      <label>Agent 名称<input v-model.trim="agentName" maxlength="40" /></label>
      <label>审查提示词<textarea v-model="agentPrompt" rows="7" /></label>
      <button type="button" @click="resetPrompt">恢复内置提示词</button>
    </div>

    <div class="gda-body">
      <div class="gda-summary">
        <span>{{ branch || '当前分支' }}</span>
        <span>{{ files.length }} 个文件</span>
        <span class="add">+{{ additions }}</span><span class="del">−{{ deletions }}</span>
      </div>

      <div v-if="loading" class="gda-state"><Icon icon="mdi:loading" class="spin" width="16" /> 正在读取本次改动…</div>
      <template v-else>
        <div v-if="signals.length" class="gda-section warning">
          <div class="gda-section-title"><Icon icon="mdi:shield-alert-outline" width="15" /> 需要确认</div>
          <div v-for="signal in signals" :key="signal.path + signal.message" class="gda-signal"><code>{{ signal.path }}</code><span>{{ signal.message }}</span></div>
        </div>
        <div v-else class="gda-section clean"><Icon icon="mdi:shield-check-outline" width="16" /> 未发现常见密钥、构建产物或二进制风险。</div>

        <div class="gda-section">
          <div class="gda-section-title">改动文件</div>
          <div v-for="file in files.slice(0, 8)" :key="file.path" class="gda-file"><code>{{ file.path }}</code><span class="add">+{{ file.additions }}</span><span class="del">−{{ file.deletions }}</span></div>
          <div v-if="files.length > 8" class="gda-more">另有 {{ files.length - 8 }} 个文件</div>
        </div>

        <div class="gda-section gda-review">
          <div class="gda-section-title"><Icon :icon="reviewing ? 'mdi:loading' : 'mdi:robot-outline'" :class="{ spin: reviewing }" width="15" /> {{ reviewing ? 'Git Agent 正在审查…' : 'Git Agent 结论' }}</div>
          <p v-if="reviewError" class="gda-error">{{ reviewError }}</p>
          <p v-else-if="review" class="gda-review-text">{{ review }}</p>
          <p v-else class="gda-muted">正在准备审查任务…</p>
        </div>
      </template>
    </div>
    <footer><button type="button" :disabled="loading || reviewing" @click="refresh"><Icon icon="mdi:refresh" width="15" /> 重新审查</button></footer>
  </section>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'

const emit = defineEmits(['close'])
const DEFAULT_NAME = 'Git Agent'
const DEFAULT_PROMPT = `你是交付前的 Git 审查 Agent。只读检查当前工作树的改动，绝不执行写入、暂存、提交、推送或删除操作。重点找出：泄露的密钥/环境变量、二进制或构建产物、无意加入的垃圾文件、危险脚本、明显的回归和缺失测试。先说明本次改动的目的，再按严重程度列出可操作问题；没有问题时明确写“可以交付”，但不要编造问题。回答使用简洁中文。`
const agentName = ref(localStorage.getItem('gitDeliveryAgentName') || DEFAULT_NAME)
const agentPrompt = ref(localStorage.getItem('gitDeliveryAgentPrompt') || DEFAULT_PROMPT)
const showSettings = ref(false)
const loading = ref(true)
const reviewing = ref(false)
const review = ref('')
const reviewError = ref('')
const branch = ref('')
const files = ref([])
let reviewStream = null

watch(agentName, value => localStorage.setItem('gitDeliveryAgentName', value || DEFAULT_NAME))
watch(agentPrompt, value => localStorage.setItem('gitDeliveryAgentPrompt', value || DEFAULT_PROMPT))
const additions = computed(() => files.value.reduce((n, file) => n + (file.additions || 0), 0))
const deletions = computed(() => files.value.reduce((n, file) => n + (file.deletions || 0), 0))
const signals = computed(() => files.value.flatMap(file => {
  const path = file.path || ''
  const lower = path.toLowerCase()
  if (/(^|\/)node_modules\/|(^|\/)dist\/|(^|\/)build\/|\.cache\//.test(lower)) return [{ path, message: '疑似构建产物或依赖目录，不建议交付。' }]
  if (/(^|\/)\.env($|\.)|\.pem$|\.key$|\.p12$|id_rsa|credentials|secret/.test(lower)) return [{ path, message: '可能包含密钥或凭据，交付前必须核实。' }]
  if (/\.(exe|dll|so|dylib|jar|zip|tar|gz|7z)$/i.test(path) || file.binary) return [{ path, message: '二进制或归档文件，确认是否应纳入版本控制。' }]
  if (/package-lock\.json|pnpm-lock\.yaml|yarn\.lock/.test(lower) && file.additions + file.deletions > 500) return [{ path, message: '锁文件改动较大，确认没有意外依赖变更。' }]
  return []
}))

function resetPrompt() { agentPrompt.value = DEFAULT_PROMPT }
function stopReview() { reviewStream?.close(); reviewStream = null }
function reviewTask() {
  const summary = files.value.map(f => `${f.status || 'M'} ${f.path} (+${f.additions || 0}/-${f.deletions || 0})`).join('\n')
  return `${agentPrompt.value}\n\n本次工作树改动摘要：\n分支：${branch.value || 'unknown'}\n${summary || '没有检测到改动'}\n\n请基于实际 git diff 完成审查。`
}
function startReview() {
  stopReview()
  reviewing.value = true
  review.value = ''
  reviewError.value = ''
  reviewStream = new EventSource('/api/code/workflow?mode=ask&max_rounds=4&task=' + encodeURIComponent(reviewTask()))
  reviewStream.addEventListener('result', event => {
    try { review.value = JSON.parse(event.data).content || JSON.parse(event.data).result || '' } catch { review.value = event.data }
  })
  reviewStream.addEventListener('flow_error', event => {
    try { reviewError.value = JSON.parse(event.data).message || 'Git Agent 审查失败' } catch { reviewError.value = 'Git Agent 审查失败' }
  })
  reviewStream.addEventListener('workflow_done', () => { reviewing.value = false; stopReview() })
  reviewStream.onerror = () => { if (reviewing.value) { reviewError.value = '无法连接 Git Agent，请检查模型配置。'; reviewing.value = false; stopReview() } }
}
async function refresh() {
  stopReview()
  loading.value = true
  review.value = ''
  reviewError.value = ''
  try {
    const res = await fetch('/api/git/working-diff')
    if (!res.ok) throw new Error('无法读取 Git diff')
    const data = await res.json()
    branch.value = data.branch || ''
    files.value = data.files || []
  } catch (error) {
    reviewError.value = error.message || '无法读取 Git diff'
  } finally {
    loading.value = false
  }
  startReview()
}
onMounted(refresh)
onUnmounted(stopReview)
</script>

<style scoped>
.git-delivery-agent { position: fixed; right: 24px; bottom: 24px; z-index: 2100; width: min(390px, calc(100vw - 32px)); max-height: min(640px, calc(100vh - 48px)); display: flex; flex-direction: column; overflow: hidden; color: #242424; background: #fff; border: 1px solid #e7e7e7; border-radius: 14px; box-shadow: 0 18px 52px rgba(0,0,0,.18); font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
.gda-header { display:flex; align-items:center; justify-content:space-between; padding:14px 14px 12px; border-bottom:1px solid #eee; }.gda-title,.gda-header-actions { display:flex; align-items:center; gap:9px; }.gda-title strong,.gda-title span { display:block; }.gda-title strong { font-size:14px; }.gda-title span { color:#747474; font-size:11px; margin-top:2px; }.gda-header-actions button, footer button { display:inline-flex; align-items:center; justify-content:center; gap:5px; border:0; background:transparent; color:#606060; cursor:pointer; border-radius:7px; }.gda-header-actions button { width:28px; height:28px; }.gda-header-actions button:hover,footer button:hover { background:#f0f0f0; color:#242424; }
.gda-settings { display:grid; gap:9px; padding:12px 14px; background:#fafafa; border-bottom:1px solid #eee; }.gda-settings label { display:grid; gap:4px; font-size:11px; color:#666; }.gda-settings input,.gda-settings textarea { box-sizing:border-box; width:100%; border:1px solid #ddd; border-radius:7px; padding:7px; font:12px/1.45 inherit; color:#333; background:#fff; resize:vertical; }.gda-settings button { justify-self:start; border:0; padding:0; color:#80563d; background:transparent; font-size:11px; cursor:pointer; }
.gda-body { overflow:auto; padding:12px 14px; }.gda-summary { display:flex; align-items:center; gap:7px; font-size:11px; color:#666; margin-bottom:12px; }.gda-summary span:first-child { max-width:150px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.add { color:#168247!important; }.del { color:#c43c38!important; }.gda-section { margin-top:12px; padding-top:12px; border-top:1px solid #f0f0f0; }.gda-section:first-child { margin-top:0; }.gda-section-title { display:flex; align-items:center; gap:6px; font-size:12px; font-weight:650; }.gda-section.clean { display:flex; align-items:center; gap:7px; padding:9px; border:0; border-radius:8px; color:#176b40; background:#f0faf4; font-size:11px; }.gda-section.warning { border:0; padding:9px; border-radius:8px; background:#fff8ea; }.gda-signal { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:8px; margin-top:7px; font-size:11px; color:#72511e; }.gda-signal code,.gda-file code { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font:11px ui-monospace,SFMono-Regular,Consolas,monospace; }.gda-file { display:grid; grid-template-columns:minmax(0,1fr) auto auto; gap:7px; margin-top:7px; font-size:11px; }.gda-more,.gda-muted { color:#777; font-size:11px; margin:8px 0 0; }.gda-review-text { margin:7px 0 0; white-space:pre-wrap; font-size:12px; line-height:1.5; }.gda-error { color:#b53b37; font-size:11px; line-height:1.45; }.gda-state { display:flex; justify-content:center; align-items:center; gap:7px; min-height:120px; color:#777; font-size:12px; }footer { display:flex; justify-content:flex-end; padding:9px 12px; border-top:1px solid #eee; }footer button { padding:6px 9px; font-size:12px; }footer button:disabled { opacity:.45; cursor:default; }.spin { animation:gda-spin .9s linear infinite; }@keyframes gda-spin { to { transform:rotate(360deg); } }
</style>
