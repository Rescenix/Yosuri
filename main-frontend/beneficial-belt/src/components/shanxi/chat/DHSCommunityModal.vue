<template>
  <Teleport to="body">
    <div class="dhs-backdrop" @click.self="$emit('close')">
      <section class="dhs-dialog" role="dialog" aria-modal="true" aria-labelledby="dhs-title">
        <header class="dhs-head">
          <div class="dhs-brand">
            <span class="dhs-logo"><Icon icon="simple-icons:deepseek" width="23" /></span>
            <div>
              <div class="dhs-kicker">DEEPSEEK HARNESS</div>
              <h2 id="dhs-title">DHS 安全插件生态</h2>
              <div class="dhs-steps">
                <span><Icon icon="mdi:magnify" width="12" />社区发现</span>
                <i></i>
                <span><Icon icon="mdi:shield-search" width="12" />代码审计</span>
                <i></i>
                <span><Icon icon="mdi:cube-scan" width="12" />执行沙盒</span>
                <i></i>
                <span><Icon icon="mdi:check-decagram-outline" width="12" />人工安装</span>
              </div>
            </div>
          </div>
          <div class="dhs-head-side">
            <div class="dhs-search-row">
              <form class="dhs-search" @submit.prevent="searchCommunity(true)">
                <Icon icon="mdi:forum-outline" width="19" />
                <input v-model="query" type="search" maxlength="80" placeholder="搜索 DHS 原生插件，例如：审计、Langfuse、模型路由" />
                <button type="submit" :disabled="loading">
                  <Icon v-if="loading" icon="mdi:loading" width="17" class="spin" />
                  <span>{{ loading ? '验证中' : '搜索 DHS' }}</span>
                </button>
              </form>
              <button class="dhs-close" type="button" title="关闭" @click="$emit('close')">
                <Icon icon="mdi:close" width="20" />
              </button>
            </div>
            <div class="dhs-sources">
              <span v-for="source in sources" :key="source.id" :class="{ offline: source.status !== 'live' }">
                <i></i>{{ source.name }}{{ source.status === 'live' ? ' 已连接' : ' 暂不可用' }}
              </span>
            </div>
            <div v-if="searched" class="dhs-coverage">
              <Icon icon="mdi:checkbox-multiple-marked-outline" width="13" />
              {{ coverage }}
              <b class="dhs-picked">精选 {{ items.length }} 条</b>
            </div>
          </div>
        </header>

        <nav class="dhs-src-tabs" aria-label="插件来源地区">
          <button type="button" :class="{ on: sourceTab === 'domestic' }" @click="sourceTab = 'domestic'">
            <Icon icon="mdi:home-city-outline" width="14" />国内 <b>{{ countBySource('domestic') }}</b>
          </button>
          <button type="button" :class="{ on: sourceTab === 'foreign' }" @click="sourceTab = 'foreign'">
            <Icon icon="mdi:earth" width="14" />国外 <b>{{ countBySource('foreign') }}</b>
          </button>
          <span class="dhs-rating-note">国内 = B站 · DSH 视频；国外 = npm / GitHub / Awesome</span>
        </nav>

        <nav class="dhs-filters" aria-label="插件信誉筛选">
          <button v-for="filter in reputationFilters" :key="filter.id" type="button" :class="{ on: activeFilter === filter.id }" @click="activeFilter = filter.id">
            {{ filter.label }} <b>{{ reputationCount(filter.id) }}</b>
          </button>
          <span class="dhs-rating-note">信誉随 Agent 审计结果动态更新</span>
        </nav>

        <div v-if="error" class="dhs-alert is-error"><Icon icon="mdi:alert-circle-outline" width="17" />{{ error }}</div>
        <div v-if="notice" class="dhs-alert is-ok"><Icon icon="mdi:check-circle-outline" width="17" />{{ notice }}</div>

        <main class="dhs-body">
          <div v-if="loading" class="dhs-loading-state">
            <Icon icon="simple-icons:deepseek" width="34" class="dhs-pulse" />
            <strong>正在聚合外部社区插件</strong>
            <span>正在验证每个候选包的 dsh.bundle.patch，普通 Skills 不会入库</span>
          </div>
          <div v-else-if="!searched" class="dhs-empty">
            <span class="dhs-empty-whale"><Icon icon="simple-icons:deepseek" width="42" /></span>
            <h3>从社区发现能力，不从社区直接执行代码</h3>
            <p>搜索结果默认不可信。选择候选后，Agent 会固定源码版本并生成双层审计报告。</p>
          </div>
          <div v-else-if="searched && !visibleItems.length" class="dhs-empty compact">
            <Icon icon="mdi:forum-remove-outline" width="40" />
            <h3>{{ !items.length ? '没有找到匹配的外部插件' : (sourceTab === 'domestic' ? '国内源暂时还没有发现插件' : '国外源没有匹配的插件') }}</h3>
            <p>{{ !items.length ? '换一个功能关键词，或稍后再试。' : (sourceTab === 'domestic' ? '看看国外 tab，或换个关键词搜索。' : '看看国内 tab，或换个关键词搜索。') }}</p>
          </div>

          <article v-for="item in visibleItems" :key="item.repo" class="dhs-card">
            <div class="dhs-card-top">
              <div class="dhs-repo-icon"><Icon icon="mdi:npm" width="20" /></div>
              <div class="dhs-card-copy">
                <div class="dhs-card-title-row">
                  <h3>{{ item.package_name || item.name }}</h3>
                  <span class="dhs-source-badge">{{ item.source_name || '外部社区' }}</span>
                  <span class="dhs-reputation" :class="'is-' + reputationFor(item).id">
                    <Icon :icon="reputationFor(item).icon" width="12" />{{ reputationFor(item).label }}
                  </span>
                </div>
                <div class="dhs-repo-path">{{ item.repo }}</div>
                <p>{{ descCache[item.repo] || item.description || '社区作者未提供说明。' }}</p>
                <div class="dhs-meta">
                  <span v-if="item.installed" class="dhs-installed"><Icon icon="mdi:check-decagram" width="14" />已装到 DHS</span>
                  <span v-if="item.version"><Icon icon="mdi:package-variant-closed" width="14" />v{{ item.version }}</span>
                  <span v-else class="dhs-no-npm"><Icon icon="mdi:package-variant-remove" width="14" />未发布 npm 包 · 仅源码</span>
                  <span v-if="item.format === 'dsh-bundle'"><Icon icon="mdi:puzzle-check-outline" width="14" />DHS Bundle</span>
                  <span><Icon icon="mdi:gauge" width="14" />质量信号 {{ item.quality_score || 0 }}</span>
                  <span>{{ formatDate(item.updated_at) }}</span>
                </div>
                <div v-if="item.quality_signals?.length" class="dhs-signals">
                  <span v-for="signal in item.quality_signals.slice(0, 3)" :key="signal">{{ signal }}</span>
                </div>
                <button class="dhs-preview-toggle" type="button" :disabled="previewLoading === item.repo" @click="togglePreview(item)">
                  <Icon :icon="previewLoading === item.repo ? 'mdi:loading' : 'mdi:text-box-search-outline'" width="14" :class="{ spin: previewLoading === item.repo }" />
                  {{ previews[item.repo] ? (previewOpen[item.repo] ? '收起内容预览' : '查看内容预览') : '读取内容预览' }}
                </button>
              </div>
              <button class="dhs-audit-btn" type="button" :disabled="busyRepo === item.repo" @click="auditRepo(item)">
                <Icon :icon="busyRepo === item.repo ? 'mdi:loading' : 'mdi:shield-search'" width="17" :class="{ spin: busyRepo === item.repo }" />
                {{ auditByRepo[item.repo] ? '重新审计' : 'Agent 审计' }}
              </button>
            </div>

            <div v-if="previewOpen[item.repo]" class="dhs-preview">
              <div><Icon icon="mdi:file-document-outline" width="15" /><strong>{{ previews[item.repo]?.path || '仓库说明' }}</strong></div>
              <p>{{ previews[item.repo]?.preview || previewErrors[item.repo] }}</p>
            </div>

            <div v-if="auditByRepo[item.repo]" class="dhs-report" :class="'is-' + auditByRepo[item.repo].status">
              <div class="dhs-report-head">
                <strong>固定版本 {{ auditByRepo[item.repo].commit.slice(0, 10) }}</strong>
                <span>{{ auditByRepo[item.repo].file_count }} 文件 · {{ formatBytes(auditByRepo[item.repo].total_bytes) }}</span>
              </div>
              <div class="dhs-gates">
                <span :class="'gate-' + auditByRepo[item.repo].code_status">
                  <Icon icon="mdi:file-code-outline" width="15" />代码层 {{ statusLabel(auditByRepo[item.repo].code_status) }}
                </span>
                <span :class="'gate-' + auditByRepo[item.repo].execution_status">
                  <Icon icon="mdi:cube-scan" width="15" />执行层 {{ statusLabel(auditByRepo[item.repo].execution_status) }}
                </span>
              </div>
              <ul v-if="auditByRepo[item.repo].findings.length" class="dhs-findings">
                <li v-for="(finding, index) in auditByRepo[item.repo].findings.slice(0, 4)" :key="index" :class="'sev-' + finding.severity">
                  <b>{{ finding.layer === 'execution' ? '执行层' : '代码层' }}</b>
                  <span>{{ finding.message }}</span>
                  <code v-if="finding.path">{{ finding.path }}</code>
                </li>
              </ul>
              <p v-else class="dhs-clean"><Icon icon="mdi:shield-check-outline" width="16" />未发现越权指令、脚本、二进制或目录逃逸。</p>
              <div class="dhs-report-foot">
                <span>{{ auditByRepo[item.repo].skill_path }}</span>
                <button
                  v-if="item.format === 'dsh-bundle' && item.installed"
                  type="button"
                  class="dhs-uninstall-btn"
                  :disabled="uninstallingDHSRepo === item.repo"
                  @click="uninstallDHS(item)"
                >
                  <Icon :icon="uninstallingDHSRepo === item.repo ? 'mdi:loading' : 'mdi:package-variant-closed-remove'" width="16" :class="{ spin: uninstallingDHSRepo === item.repo }" />
                  {{ uninstallingDHSRepo === item.repo ? '卸载中' : '卸载' }}
                </button>
                <button
                  v-else-if="item.format === 'dsh-bundle'"
                  type="button"
                  :disabled="auditByRepo[item.repo].status === 'blocked' || installingDHSRepo === item.repo || !item.version"
                  :title="!item.version ? '该插件没有可固定的 npm 版本，无法安全安装到 DHS' : ''"
                  @click="installDHS(item, auditByRepo[item.repo])"
                >
                  <Icon :icon="installingDHSRepo === item.repo ? 'mdi:loading' : (auditByRepo[item.repo].status === 'blocked' ? 'mdi:lock-outline' : 'mdi:package-down')" width="16" :class="{ spin: installingDHSRepo === item.repo }" />
                  {{ auditByRepo[item.repo].status === 'blocked' ? '已阻断' : (installingDHSRepo === item.repo ? '安装到 DHS 中' : '安装到 DHS') }}
                </button>
                <button
                  v-else
                  type="button"
                  :disabled="auditByRepo[item.repo].status === 'blocked' || installingRepo === item.repo"
                  @click="installRepo(item, auditByRepo[item.repo])"
                >
                  <Icon :icon="installingRepo === item.repo ? 'mdi:loading' : (auditByRepo[item.repo].status === 'blocked' ? 'mdi:lock-outline' : 'mdi:download-lock-outline')" width="16" :class="{ spin: installingRepo === item.repo }" />
                  {{ auditByRepo[item.repo].status === 'blocked' ? '已阻断' : (installingRepo === item.repo ? '安全安装中' : '确认安装') }}
                </button>
              </div>
            </div>
            <button class="dhs-fav" type="button" :class="{ liked: favorites.has(item.repo) }" :title="favorites.has(item.repo) ? '已收藏（云端）' : '收藏到云端'" @click="toggleFavorite(item)">
              <Icon :icon="favorites.has(item.repo) ? 'mdi:heart' : 'mdi:heart-outline'" width="16" />
            </button>
          </article>
        </main>
      </section>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, onMounted, ref, reactive } from 'vue'
import { Icon } from '@iconify/vue'

defineEmits(['close'])

const query = ref('')
const items = ref([])
const loading = ref(false)
const searched = ref(false)
const error = ref('')
const notice = ref('')
const busyRepo = ref('')
const installingRepo = ref('')
const installingDHSRepo = ref('')
const uninstallingDHSRepo = ref('')
const previewLoading = ref('')
const activeFilter = ref('all')
const sources = ref([
  { id: 'npm-dsh', name: 'npm DHS Registry', status: 'loading' },
  { id: 'github-topic', name: 'GitHub dsh-plugin 话题', status: 'loading' },
  { id: 'awesome-dsh', name: 'Awesome DSH Plugin', status: 'loading' },
  { id: 'bilibili-video', name: 'B站 · DSH 视频', status: 'loading' }
])
const coverage = ref('仅显示 DHS 原生 Bundle；通用 Agent Skills 已排除')
const auditByRepo = reactive({})
const previews = reactive({})
const previewErrors = reactive({})
const previewOpen = reactive({})

const reputationFilters = [
  { id: 'all', label: '全部' },
  { id: 'excellent', label: '优秀' },
  { id: 'trusted', label: '可信' },
  { id: 'untrusted', label: '不可信' }
]

// 来源地区 tab：国内 = B站；国外 = npm / GitHub / Awesome。国内排第一且默认选中。
const sourceTab = ref('domestic')
const isDomestic = item => item.source_id === 'bilibili-video'
function countBySource(tab) {
  return items.value.filter(i => tab === 'domestic' ? isDomestic(i) : !isDomestic(i)).length
}

const visibleItems = computed(() => {
  let list = items.value.filter(i => sourceTab.value === 'domestic' ? isDomestic(i) : !isDomestic(i))
  if (activeFilter.value !== 'all') list = list.filter(item => reputationFor(item).id === activeFilter.value)
  return list
})

async function searchCommunity() {
  loading.value = true
  error.value = ''
  notice.value = ''
  // 清掉上一轮本地审计结果，然后从云端账本恢复（审计过的插件可信标签不丢）
  Object.keys(auditByRepo).forEach(key => delete auditByRepo[key])
  try {
    const params = new URLSearchParams()
    if (query.value.trim()) params.set('q', query.value.trim())
    const response = await fetch('/api/dhs/community?' + params)
    const data = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(data.error || '社区搜索失败')
    items.value = data.items || []
    sources.value = data.sources || sources.value
    coverage.value = data.coverage || coverage.value
    searched.value = true
  } catch (e) {
    items.value = []
    searched.value = true
    error.value = e.message
  } finally {
    await loadCloudAudits()
    translateDescriptions(items.value)
    loading.value = false
  }
}

// ── 插件描述汉化：社区作者多为英文描述，自动翻译成中文显示（并发 3 + localStorage 缓存）──
const descCache = reactive({})
const descCacheKey = repo => 'dhs-desc-' + repo
// 预载本地已缓存的中文描述：否则缓存命中时会跳过翻译、但 descCache 为空 → 显示英文原文
function preloadDescCache() {
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i)
      if (key && key.startsWith('dhs-desc-')) {
        const repo = key.slice('dhs-desc-'.length)
        const val = localStorage.getItem(key)
        if (val && !descCache[repo]) descCache[repo] = val
      }
    }
  } catch { /* 隐私模式等异常静默 */ }
}
function looksEnglish(text) {
  if (!text || /[\u4e00-\u9fff]/.test(text)) return false
  // 去掉 URL/版本号/标点后，仍有较多 ASCII 字母才算英文
  const letters = (text.match(/[a-zA-Z]/g) || []).length
  return letters >= 12
}
async function translateDescriptions(list) {
  const need = (list || []).filter(it =>
    it.description && looksEnglish(it.description) &&
    !descCache[it.repo] && !localStorage.getItem(descCacheKey(it.repo))
  )
  if (!need.length) return
  // 批量翻译：一次请求翻完全部（后端串行限速 + 多模型降级），翻完全部缓存本地，
  // 下次加载直接读缓存立即中文，不再逐条异步。
  try {
    const res = await fetch('/api/translate/batch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ texts: need.map(it => it.description), target_lang: 'zh' })
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || '批量翻译失败')
    const results = data.results || []
    let ok = 0
    results.forEach((t, i) => {
      if (t && need[i]) {
        descCache[need[i].repo] = t
        localStorage.setItem(descCacheKey(need[i].repo), t)
        ok++
      }
    })
    if (ok < need.length) {
      notice.value = `描述汉化：成功 ${ok}/${need.length}${ok < need.length ? '，失败 ' + (need.length - ok) + ' 条（回退原文）' : ''}`
    }
  } catch (e) {
    console.warn('[dhs-translate]', e)
    notice.value = `描述汉化失败：${e.message}`
  }
}

// loadCloudAudits 从云端账本拉取已审计插件的报告（公开读），填充本地 auditByRepo。
// 这样「可信 / 优秀」标签跨设备、跨刷新保留，审计过的插件不再随重搜蒸发。
async function loadCloudAudits() {
  try {
    const response = await fetch('/api/dhs/audits')
    if (!response.ok) return
    const data = await response.json().catch(() => ({}))
    for (const rec of (data.audits || [])) {
      try {
        auditByRepo[rec.repo] = typeof rec.report === 'string' ? JSON.parse(rec.report) : rec.report
      } catch { /* 坏数据跳过 */ }
    }
  } catch { /* 云端不可达静默：本地审计结果照常显示 */ }
}

// reportAuditToCloud 把审计报告上报云端账本（需登录，失败静默不影响本地展示）。
// 付费预留：云端按 audited_by(uid) 记录谁审计的。
async function reportAuditToCloud(item, report) {
  try {
    const token = localStorage.getItem('token') || ''
    if (!token) return
    await fetch('/api/dhs/audits', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
      body: JSON.stringify({ ...report, quality_score: Number(item.quality_score || 0) })
    })
  } catch { /* 云端不可达静默 */ }
}

// 打开面板即从云端账本恢复已审计插件（不必等搜索）+ 预载本地中文描述缓存
onMounted(() => { preloadDescCache(); loadCloudAudits(); loadFavorites() })

// ── 爱心收藏：本地即时响应 + 云端按 uid 持久化（登录后跨设备恢复）──
const favorites = reactive(new Set())
const favLocalKey = 'dhs-favs'
function loadFavorites() {
  try {
    const local = JSON.parse(localStorage.getItem(favLocalKey) || '[]')
    if (Array.isArray(local)) local.forEach(r => favorites.add(r))
  } catch { /* 坏缓存忽略 */ }
  const token = localStorage.getItem('token') || ''
  if (!token) return
  // 登录：以云端为准覆盖本地
  fetch('/api/dhs/favorites')
    .then(r => r.json().catch(() => ({})))
    .then(d => {
      if (!Array.isArray(d.favorites)) return
      favorites.clear()
      d.favorites.forEach(r => favorites.add(r))
      localStorage.setItem(favLocalKey, JSON.stringify([...favorites]))
    })
    .catch(() => { /* 云端不可达：用本地缓存 */ })
}
async function toggleFavorite(item) {
  const repo = item.repo || item.package_name
  const liked = !favorites.has(repo)
  if (liked) favorites.add(repo); else favorites.delete(repo)
  localStorage.setItem(favLocalKey, JSON.stringify([...favorites]))
  const token = localStorage.getItem('token') || ''
  if (!token) {
    notice.value = liked ? '已收藏 ❤（登录后自动同步到云端）' : '已取消收藏'
    return
  }
  try {
    const res = await fetch('/api/dhs/favorites/toggle', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
      body: JSON.stringify({ repo })
    })
    if (!res.ok) {
      if (liked) favorites.delete(repo); else favorites.add(repo)
      localStorage.setItem(favLocalKey, JSON.stringify([...favorites]))
      throw new Error('云端同步失败')
    }
  } catch (e) {
    error.value = `收藏同步：${e.message}`
  }
}

async function togglePreview(item) {
  if (previews[item.repo] || previewErrors[item.repo]) {
    // 已缓存：若预览还是英文（热更新前缓存的旧数据），补触发翻译
    if (previewNeedsTranslate(previews[item.repo]?.preview)) {
      translatePreview(item, previews[item.repo])
    }
    previewOpen[item.repo] = !previewOpen[item.repo]
    return
  }
  previewLoading.value = item.repo
  delete previewErrors[item.repo]
  try {
    const params = new URLSearchParams({ repo: item.repo })
    const response = await fetch('/api/dhs/community/preview?' + params)
    const data = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(data.error || '暂时无法读取仓库说明')
    previews[item.repo] = data
    translatePreview(item, data) // README 预览也汉化（异步，翻译完自动更新）
  } catch (e) {
    previewErrors[item.repo] = e.message
  } finally {
    previewOpen[item.repo] = true
    previewLoading.value = ''
  }
}

// translatePreview README 内容预览汉化：英文预览翻译成中文后覆盖显示。
// 判定「英文为主」才翻译（中英混排 README 也要翻，纯中文跳过）
function previewNeedsTranslate(text) {
  if (!text || text.length < 12) return false
  const cn = (text.match(/[\u4e00-\u9fff]/g) || []).length
  const letters = (text.match(/[a-zA-Z]/g) || []).length
  return letters >= 12 && letters > cn * 2
}
async function translatePreview(item, data) {
  const text = data?.preview || ''
  if (!previewNeedsTranslate(text)) return
  try {
    const res = await fetch('/api/translate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text, target_lang: 'zh' })
    })
    const t = ((await res.json().catch(() => ({}))).translated || '').trim()
    if (t && t !== text && previews[item.repo]) {
      previews[item.repo] = { ...previews[item.repo], preview: t }
    }
  } catch { /* 翻译失败保留原文 */ }
}

async function auditRepo(item) {
  busyRepo.value = item.repo
  error.value = ''
  notice.value = ''
  try {
    const response = await fetch('/api/dhs/community/audit', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ repo: item.repo, package_name: item.package_name, version: item.version })
    })
    const data = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(data.error || 'Agent 审计失败')
    auditByRepo[item.repo] = data.report
    // 上报云端账本（登录后持久化；游客静默跳过，本地照样显示）
    reportAuditToCloud(item, data.report)
  } catch (e) {
    error.value = `${item.repo}：${e.message}`
  } finally {
    busyRepo.value = ''
  }
}

async function installRepo(item, report) {
  const reviewText = report.status === 'review' ? '\n\n审计发现中风险项，请确认你已经阅读报告。' : ''
  if (!window.confirm(`安装 DHS 插件「${item.repo}」？\n\n将固定安装 commit ${report.commit.slice(0, 10)}，不会执行仓库脚本。${reviewText}`)) return
  installingRepo.value = item.repo
  error.value = ''
  notice.value = ''
  try {
    const response = await fetch('/api/dhs/community/install', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ repo: item.repo, package_name: item.package_name, version: item.version, skill_path: report.skill_path, commit: report.commit })
    })
    const data = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(data.error || '安全安装失败')
    notice.value = `${item.repo} 已通过双层审计并安装到本地 Harness。`
  } catch (e) {
    error.value = `${item.repo}：${e.message}`
  } finally {
    installingRepo.value = ''
  }
}

// installDHS 一键安装到 DHS 本体（npm bundle 原生插件）：审计 → pnpm add → 写 bundles。
async function installDHS(item, report) {
  if (!window.confirm(`安装 DHS 插件「${item.package_name}@${item.version}」？\n\n将固定 npm 版本 ${item.version} 并重新审计，通过后写入 DHS profile 并自动重启 dsh 生效。`)) return
  installingDHSRepo.value = item.repo
  error.value = ''
  notice.value = ''
  try {
    const response = await fetch('/api/dhs/community/install-dhs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ package_name: item.package_name, version: item.version, repo: item.repo })
    })
    const data = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(data.error || '安装到 DHS 失败')
    notice.value = data.already_installed
      ? `${item.package_name} 已安装过（profile ${data.profile}）。`
      : `${data.note || `${item.package_name} 已装到 DHS profile「${data.profile}」。`}${data.warning ? `（${data.warning}）` : ''}`
  } catch (e) {
    error.value = `${item.package_name}：${e.message}`
  } finally {
    installingDHSRepo.value = ''
  }
}

function statusLabel(value) {
  return ({ passed: '通过', review: '需复核', blocked: '阻断', contained: '已隔离' })[value] || value
}

// uninstallDHS 从 DHS profile 卸载插件：pnpm remove + 从 bundles 剔除 → 刷新列表。
async function uninstallDHS(item) {
  if (!window.confirm(`卸载 DHS 插件「${item.package_name}」？\n\n将从 DHS profile 移除该插件（pnpm remove + bundles 剔除）并自动重启 dsh 生效。`)) return
  uninstallingDHSRepo.value = item.repo
  error.value = ''
  notice.value = ''
  try {
    const response = await fetch('/api/dhs/community/uninstall-dhs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ package_name: item.package_name })
    })
    const data = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(data.error || '卸载失败')
    notice.value = `${data.note || `${item.package_name} 已从 DHS profile「${data.profile}」卸载。`}${data.warning ? `（${data.warning}）` : ''}`
    item.installed = false
  } catch (e) {
    error.value = `${item.package_name}：${e.message}`
  } finally {
    uninstallingDHSRepo.value = ''
  }
}

function reputationFor(item) {
  const report = auditByRepo[item.repo]
  if (report?.status === 'passed' && Number(item.quality_score || 0) >= 70) {
    return { id: 'excellent', label: '优秀', icon: 'mdi:medal-outline' }
  }
  if (report?.status === 'passed') {
    return { id: 'trusted', label: '可信', icon: 'mdi:shield-check-outline' }
  }
  return { id: 'untrusted', label: '不可信', icon: 'mdi:shield-alert-outline' }
}

function reputationCount(id) {
  if (id === 'all') return items.value.length
  return items.value.filter(item => reputationFor(item).id === id).length
}

function formatBytes(bytes) {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1024 / 1024).toFixed(1) + ' MB'
}

function formatDate(value) {
  if (!value) return '更新时间未知'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

onMounted(() => searchCommunity())
</script>

<style scoped>
.dhs-backdrop { position: fixed; inset: 0; z-index: 100020; display: grid; place-items: center; padding: 0; background: rgba(16, 18, 28, .46); backdrop-filter: blur(12px); animation: dhs-fade .18s ease; }
.dhs-dialog { width: 100vw; height: 100vh; max-height: none; display: flex; flex-direction: column; overflow: hidden; color: var(--app-text); background: color-mix(in srgb, var(--app-surface) 97%, #fff); border: 1px solid color-mix(in srgb, var(--app-accent) 18%, var(--app-border)); border-radius: 0; box-shadow: 0 28px 80px rgba(10, 12, 20, .28); animation: dhs-rise .22s cubic-bezier(.22, 1, .36, 1); }
.dhs-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 26px; padding: 20px 26px 16px; border-bottom: 1px solid var(--app-border-soft); }
.dhs-brand { display: flex; gap: 14px; min-width: 0; }
.dhs-head-side { display: flex; flex-direction: column; align-items: flex-end; gap: 8px; flex: 1; min-width: 0; max-width: 54vw; }
.dhs-search-row { display: flex; align-items: center; gap: 12px; width: 100%; }
.dhs-search { display: flex; align-items: center; gap: 10px; flex: 1; min-width: 0; margin: 0; padding: 4px 6px 4px 14px; background: var(--app-surface-2); border: 1px solid var(--app-border); border-radius: 14px; }
.dhs-logo { width: 46px; height: 46px; flex: 0 0 46px; display: grid; place-items: center; color: #fff; background: linear-gradient(145deg, #7445ff, #a847ff); border-radius: 14px; box-shadow: 0 10px 24px rgba(139, 69, 255, .26); }
.dhs-kicker { margin: 1px 0 3px; color: var(--app-accent); font-size: 10px; font-weight: 800; letter-spacing: .16em; }
.dhs-head h2 { margin: 0; font-size: 20px; line-height: 1.25; }
.dhs-head p { margin: 5px 0 0; color: var(--app-text-faint); font-size: 12px; line-height: 1.55; }
.dhs-steps { display: flex; align-items: center; gap: 7px; margin-top: 9px; color: var(--app-text-faint); font-size: 11px; }
.dhs-steps span { display: inline-flex; align-items: center; gap: 4px; white-space: nowrap; }
.dhs-steps i { width: 1px; height: 10px; background: color-mix(in srgb, var(--app-accent) 30%, transparent); }
.dhs-close { width: 38px; height: 38px; display: grid; place-items: center; color: var(--app-text-soft); background: transparent; border: 0; border-radius: 10px; cursor: pointer; }
.dhs-close:hover { background: var(--app-surface-3); }
.dhs-search { display: flex; align-items: center; gap: 10px; flex: 1; min-width: 0; margin: 0; padding: 4px 6px 4px 14px; background: var(--app-surface-2); border: 1px solid var(--app-border); border-radius: 14px; }
.dhs-search > svg { color: var(--app-text-faint); flex: none; }
.dhs-search input { min-width: 0; flex: 1; height: 36px; color: var(--app-text); background: transparent; border: 0; outline: 0; font: inherit; font-size: 13px; }
.dhs-search button { min-width: 104px; height: 38px; display: inline-flex; align-items: center; justify-content: center; gap: 7px; color: #fff; background: linear-gradient(135deg, #7d4dff, #a546ff); border: 0; border-radius: 10px; font-weight: 700; cursor: pointer; box-shadow: 0 7px 18px rgba(137, 70, 255, .2); }
.dhs-search button:disabled { opacity: .66; cursor: wait; }
.dhs-sources { display: flex; align-items: center; gap: 12px; color: var(--app-text-faint); font-size: 10.5px; flex-shrink: 0; }
.dhs-sources span { display: inline-flex; align-items: center; gap: 5px; white-space: nowrap; }
.dhs-sources i { width: 6px; height: 6px; background: #12b76a; border-radius: 50%; box-shadow: 0 0 0 3px rgba(18, 183, 106, .1); }
.dhs-sources .offline i { background: #f79009; box-shadow: 0 0 0 3px rgba(247, 144, 9, .1); }
.dhs-coverage { display: flex; align-items: center; gap: 6px; color: var(--app-text-faint); font-size: 10.5px; line-height: 1.5; }
.dhs-coverage .dhs-picked { margin-left: 4px; padding: 1px 8px; color: var(--app-accent); background: color-mix(in srgb, var(--app-accent) 10%, var(--app-surface)); border: 1px solid color-mix(in srgb, var(--app-accent) 22%, var(--app-border)); border-radius: 999px; font-size: 9.5px; white-space: nowrap; }
.dhs-filters { display: flex; align-items: center; gap: 6px; margin: 0 24px 6px; padding: 18px 0 8px; border-bottom: 1px solid var(--app-border-soft); }
.dhs-src-tabs { display: flex; align-items: center; gap: 6px; margin: 14px 24px 0; }
.dhs-src-tabs button { min-height: 32px; display: inline-flex; align-items: center; gap: 5px; padding: 0 14px; color: var(--app-text-soft); background: transparent; border: 1px solid transparent; border-radius: 9px; font-size: 11px; font-weight: 700; cursor: pointer; }
.dhs-src-tabs button:first-child { margin-right: 2px; }
.dhs-src-tabs button:hover { background: var(--app-surface-2); }
.dhs-src-tabs button.on { color: var(--app-accent); background: color-mix(in srgb, var(--app-accent) 10%, var(--app-surface)); border-color: color-mix(in srgb, var(--app-accent) 22%, var(--app-border)); }
.dhs-src-tabs b, .dhs-filters b { min-width: 17px; padding: 1px 5px; background: var(--app-surface-3); border-radius: 999px; font-size: 9px; }
.dhs-filters button { min-height: 30px; display: inline-flex; align-items: center; gap: 5px; padding: 0 10px; color: var(--app-text-soft); background: transparent; border: 1px solid transparent; border-radius: 8px; font-size: 10.5px; cursor: pointer; }
.dhs-filters button:hover { background: var(--app-surface-2); }
.dhs-filters button.on { color: var(--app-accent); background: color-mix(in srgb, var(--app-accent) 8%, var(--app-surface)); border-color: color-mix(in srgb, var(--app-accent) 18%, var(--app-border)); }
.dhs-rating-note { margin-left: auto; color: var(--app-text-faint); font-size: 9.5px; }
.dhs-alert { display: flex; align-items: center; gap: 7px; margin: 0 24px 8px; padding: 9px 11px; border-radius: 9px; font-size: 12px; }
.dhs-alert.is-error { color: #b42318; background: #fff1f0; border: 1px solid #ffd5d2; }
.dhs-alert.is-ok { color: #067647; background: #ecfdf3; border: 1px solid #abefc6; }
.dhs-body { min-height: 260px; flex: 1; overflow: auto; padding: 2px 24px 24px; }
.dhs-empty, .dhs-loading-state { min-height: 260px; display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; color: var(--app-text-faint); }
.dhs-empty.compact { min-height: 220px; }
.dhs-loading-state { gap: 8px; }
.dhs-loading-state strong { color: var(--app-text); font-size: 14px; }
.dhs-loading-state span { font-size: 11px; }
.dhs-pulse { color: var(--app-accent); animation: dhs-pulse 1.2s ease-in-out infinite; }
.dhs-empty-whale { width: 78px; height: 78px; display: grid; place-items: center; margin-bottom: 16px; color: var(--app-accent); background: color-mix(in srgb, var(--app-accent) 10%, var(--app-surface)); border: 1px solid color-mix(in srgb, var(--app-accent) 20%, var(--app-border)); border-radius: 24px; }
.dhs-empty h3 { margin: 0; color: var(--app-text); font-size: 15px; }
.dhs-empty p { max-width: 540px; margin: 8px 0 0; font-size: 12px; line-height: 1.65; }
.dhs-card { position: relative; margin-top: 10px; padding: 15px; background: var(--app-surface-2); border: 1px solid var(--app-border); border-radius: 14px; transition: border-color .18s ease, box-shadow .18s ease; }
.dhs-card:hover { border-color: color-mix(in srgb, var(--app-accent) 35%, var(--app-border)); box-shadow: 0 8px 24px rgba(22, 24, 34, .05); }
.dhs-card-top { display: flex; align-items: flex-start; gap: 12px; }
.dhs-repo-icon { width: 38px; height: 38px; flex: 0 0 38px; display: grid; place-items: center; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 10px; }
.dhs-card-copy { min-width: 0; flex: 1; }
.dhs-card-title-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.dhs-fav { position: absolute; right: 12px; bottom: 12px; width: 28px; height: 28px; display: grid; place-items: center; color: var(--app-text-faint); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 9px; cursor: pointer; transition: color .15s ease, transform .12s ease, border-color .15s ease; }
.dhs-fav:hover { color: #ff5d8f; transform: scale(1.1); border-color: rgba(255, 93, 143, .45); }
.dhs-fav.liked { color: #ff3b6b; border-color: rgba(255, 59, 107, .4); background: color-mix(in srgb, #ff3b6b 8%, var(--app-surface)); }
.dhs-card h3 { margin: 0; font-size: 13.5px; overflow-wrap: anywhere; }
.dhs-repo-path { margin-top: 3px; color: var(--app-text-faint); font: 9.5px/1.35 ui-monospace, SFMono-Regular, Consolas, monospace; }
.dhs-source-badge { padding: 2px 7px; color: var(--app-text-soft); background: var(--app-surface-3); border: 1px solid var(--app-border-soft); border-radius: 999px; font-size: 9px; font-weight: 650; }
.dhs-reputation { display: inline-flex; align-items: center; gap: 3px; padding: 2px 7px; border: 1px solid; border-radius: 999px; font-size: 9px; font-weight: 750; }
.dhs-reputation.is-excellent { color: #6941c6; background: #f4f0ff; border-color: #d9d0ff; }
.dhs-reputation.is-trusted { color: #067647; background: #ecfdf3; border-color: #abefc6; }
.dhs-reputation.is-untrusted { color: #b54708; background: #fffaeb; border-color: #fedf89; }
.dhs-card-copy > p { margin: 5px 0 7px; color: var(--app-text-faint); font-size: 11.5px; line-height: 1.55; }
.dhs-meta { display: flex; align-items: center; gap: 12px; color: var(--app-text-faint); font-size: 10px; }
.dhs-meta span { display: inline-flex; align-items: center; gap: 3px; }
.dhs-meta .dhs-no-npm { color: #f79009; font-weight: 700; }
.dhs-meta .dhs-installed { color: #12b76a; font-weight: 700; }
.dhs-uninstall-btn { min-height: 28px; display: inline-flex; align-items: center; gap: 5px; padding: 0 10px; color: #b42318; background: #fff1f0; border: 1px solid #ffd5d2; border-radius: 8px; font-size: 10.5px; font-weight: 700; cursor: pointer; }
.dhs-uninstall-btn:hover:not(:disabled) { background: #ffe4e2; }
.dhs-uninstall-btn:disabled { opacity: .55; cursor: wait; }
.dhs-signals { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 7px; }
.dhs-signals span { padding: 2px 6px; color: var(--app-text-faint); background: var(--app-surface); border: 1px solid var(--app-border-soft); border-radius: 6px; font-size: 9px; }
.dhs-preview-toggle { min-height: 28px; display: inline-flex; align-items: center; gap: 5px; margin-top: 7px; padding: 0; color: var(--app-accent); background: transparent; border: 0; font-size: 10px; font-weight: 650; cursor: pointer; }
.dhs-preview-toggle:disabled { opacity: .6; cursor: wait; }
.dhs-preview { margin: 11px 0 0 50px; padding: 11px 12px; color: var(--app-text-soft); background: color-mix(in srgb, var(--app-accent) 4%, var(--app-surface)); border: 1px solid color-mix(in srgb, var(--app-accent) 15%, var(--app-border)); border-radius: 10px; }
.dhs-preview > div { display: flex; align-items: center; gap: 5px; color: var(--app-text); font-size: 10px; }
.dhs-preview p { margin: 7px 0 0; font-size: 10.5px; line-height: 1.65; }
.dhs-audit-btn { min-width: 104px; height: 36px; display: inline-flex; align-items: center; justify-content: center; gap: 6px; flex: none; color: var(--app-accent); background: color-mix(in srgb, var(--app-accent) 8%, var(--app-surface)); border: 1px solid color-mix(in srgb, var(--app-accent) 28%, var(--app-border)); border-radius: 10px; font-size: 11px; font-weight: 700; cursor: pointer; }
.dhs-audit-btn:hover:not(:disabled) { background: color-mix(in srgb, var(--app-accent) 14%, var(--app-surface)); }
.dhs-audit-btn:disabled { opacity: .65; }
.dhs-report { margin-top: 13px; padding: 12px; background: var(--app-surface); border: 1px solid var(--app-border-soft); border-radius: 11px; }
.dhs-report.is-blocked { border-color: #fda29b; }
.dhs-report.is-review { border-color: #fedf89; }
.dhs-report-head, .dhs-report-foot { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.dhs-report-head strong { font-size: 11px; }
.dhs-report-head span { color: var(--app-text-faint); font-size: 10px; }
.dhs-gates { display: flex; gap: 7px; margin-top: 9px; }
.dhs-gates span { display: inline-flex; align-items: center; gap: 5px; padding: 4px 8px; border-radius: 7px; font-size: 10px; font-weight: 700; }
.gate-passed, .gate-contained { color: #067647; background: #ecfdf3; }
.gate-review { color: #b54708; background: #fffaeb; }
.gate-blocked { color: #b42318; background: #fef3f2; }
.dhs-findings { display: grid; gap: 5px; margin: 9px 0 0; padding: 0; list-style: none; }
.dhs-findings li { display: grid; grid-template-columns: 46px minmax(0, 1fr); gap: 4px 8px; padding: 7px 8px; background: var(--app-surface-2); border-radius: 7px; font-size: 10px; }
.dhs-findings b { color: var(--app-text-soft); }
.dhs-findings code { grid-column: 2; overflow: hidden; color: var(--app-text-faint); text-overflow: ellipsis; white-space: nowrap; }
.sev-high { border-left: 3px solid #f04438; }
.sev-medium { border-left: 3px solid #f79009; }
.dhs-clean { display: flex; align-items: center; gap: 6px; margin: 9px 0 0; color: #067647; font-size: 10.5px; }
.dhs-report-foot { margin-top: 11px; padding-top: 10px; border-top: 1px solid var(--app-border-soft); }
.dhs-report-foot > span { min-width: 0; overflow: hidden; color: var(--app-text-faint); font: 10px/1.3 ui-monospace, SFMono-Regular, Consolas, monospace; text-overflow: ellipsis; white-space: nowrap; }
.dhs-report-foot button { height: 34px; display: inline-flex; align-items: center; gap: 6px; flex: none; padding: 0 12px; color: #fff; background: #111827; border: 0; border-radius: 9px; font-size: 10.5px; font-weight: 700; cursor: pointer; }
.dhs-report-foot button:disabled { color: var(--app-text-faint); background: var(--app-surface-3); cursor: not-allowed; }
.spin { animation: dhs-spin .8s linear infinite; }
@keyframes dhs-spin { to { transform: rotate(360deg); } }
@keyframes dhs-pulse { 50% { opacity: .45; transform: scale(.92); } }
@keyframes dhs-fade { from { opacity: 0; } }
@keyframes dhs-rise { from { opacity: 0; transform: translateY(10px) scale(.985); } }
@media (max-width: 720px) {
  .dhs-backdrop { padding: 8px; }
  .dhs-dialog { width: 100%; max-height: 96vh; border-radius: 16px; }
  .dhs-head { padding: 18px; }
  .dhs-head-side { max-width: 60vw; }
  .dhs-sources { width: 100%; justify-content: flex-end; }
  .dhs-search button { min-width: 44px; width: 44px; }
  .dhs-search button span { display: none; }
  .dhs-filters { margin: 0 14px 6px; overflow-x: auto; }
  .dhs-rating-note { display: none; }
  .dhs-body { padding: 6px 14px 18px; }
  .dhs-card-top { flex-wrap: wrap; }
  .dhs-card-copy { flex-basis: calc(100% - 52px); }
  .dhs-audit-btn { width: 100%; min-height: 44px; }
  .dhs-preview { margin-left: 0; }
}
</style>
