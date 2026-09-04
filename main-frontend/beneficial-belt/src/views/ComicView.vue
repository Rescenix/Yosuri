<template>
  <main class="comic-view">
    <header class="comic-header">
      <div class="comic-header-copy">
        <span class="comic-kicker">COMIC FACTORY · 漫画工坊</span>
        <h1>从小说章节，到一页漫画。</h1>
        <p>输入小说章节，AI 自动拆成分镜、生成画面、拼成漫画页。角色库确保每格角色一致。</p>
      </div>
      <div class="comic-header-status">
        <span>当前状态</span>
        <strong>{{ statusText }}</strong>
        <div><i :class="{ online: sdOnline, starting: engineStarting }"></i>{{ sdOnline ? `绘图引擎已就绪${sdModel ? ' · ' + sdModel : ''}` : engineStarting ? '正在唤醒绘图引擎' : '绘图引擎待命' }}</div>
      </div>
    </header>
    <section class="comic-input-section">
      <div class="comic-input-left">
        <label>小说章节 / 故事梗概</label>
        <textarea v-model="chapter" rows="6" placeholder="粘贴小说章节正文，或从创作tab导入"></textarea>
        <div class="source-row"><label>从书架导入 <select v-model="selectedBookId" @change="selectBook"><option value="">选择书籍</option><option v-for="book in books" :key="book.id" :value="book.id">{{ book.title }}</option></select></label><label>选择章节 <select v-model="selectedChapterId" @change="selectChapter"><option value="">选择已有章节</option><option v-for="item in selectedBook?.chapters || []" :key="item.id" :value="item.id">{{ item.title }}</option></select></label></div>
        <div class="comic-input-meta">
          <label>标题 <input v-model="title" placeholder="章节名" /></label>
          <label>风格 <input v-model="style" placeholder="画风，如：日式漫画" /></label>
          <label>格数 <select v-model.number="panelCount"><option :value="1">1格</option><option :value="2">2格</option><option :value="4" selected>4格</option><option :value="6">6格</option><option :value="9">9格</option></select></label>
        </div>
      </div>
      <div class="comic-input-right">
        <label>角色设定</label>
        <div class="comic-char-list">
          <div v-for="ch in characters" :key="ch.name" class="char-chip" :class="{ active: selectedChar === ch.name }" @click="selectedChar = ch.name">
            <span class="char-avatar" :style="{ background: ch.gender === '女' ? '#f0d6e8' : '#d6e0f0' }"><Icon :icon="ch.gender === '女' ? 'mdi:face-woman' : 'mdi:face-man'" width="16" /></span>
            <span><b>{{ ch.name }}</b><small>{{ (ch.look || '').slice(0, 12) || '未设定' }}</small></span>
          </div>
          <button class="char-add" @click="showCharCreator = true"><Icon icon="mdi:plus" width="16" /> 新建角色</button>
        </div>
        <div class="comic-actions">
          <button class="primary" :disabled="!chapter.trim() || generating" @click="startGenerate">
            <Icon :icon="generating ? 'mdi:loading' : 'mdi:auto-fix'" :class="{ spin: generating }" width="17" />
            {{ generating ? (engineStarting ? '正在启动绘图引擎…' : 'AI 正在创作…') : '开始生成漫画' }}
          </button>
          <button class="secondary" type="button" :disabled="engineStarting" @click="ensureSD"><Icon :icon="engineStarting ? 'mdi:loading' : 'mdi:power'" :class="{ spin: engineStarting }" width="16" /> {{ sdOnline ? '绘图引擎已就绪' : engineStarting ? '正在启动…' : '启动绘图引擎' }}</button>
        </div>
      </div>
    </section>
    <section v-if="panels.length" class="comic-page-section">
      <div class="section-head">
        <span>COMIC PAGE</span>
        <h2>{{ pageTitle }}</h2>
        <div class="section-actions">
          <span v-if="renderingPanels" class="render-progress">{{ renderedCount }}/{{ panels.length }} 格已完成</span>
          <button v-if="panels.some(panel => !panel.imageUrl)" :disabled="renderingPanels || !sdOnline" @click="renderAllPanels">生成全部画面</button>
          <button v-if="panels.length && panels.every(panel => panel.imageUrl)" :disabled="assembling" @click="assemblePage">{{ assembling ? '正在拼页…' : '自动拼成漫画页' }}</button>
          <button @click="exportPage">导出 PNG</button>
          <button @click="resetPage">重新生成</button>
        </div>
      </div>
      <div class="comic-page" :class="'layout-' + layout">
        <div v-for="(panel, idx) in panels" :key="idx" class="comic-panel" :class="{ 'is-rendering': panel.rendering, 'has-error': panel.renderError }" :style="panelStyle(idx)" @click="openPanelDetail(idx)">
          <img v-if="panel.imageUrl" class="panel-image" :src="panel.imageUrl" :alt="panel.scene" @error="handlePanelImageError(idx)" />
          <div v-else class="panel-placeholder">
            <Icon :icon="panel.rendering ? 'mdi:loading' : panel.renderError ? 'mdi:image-refresh-outline' : 'mdi:image-auto-outline'" :class="{ spin: panel.rendering }" width="32" />
            <strong>{{ panel.rendering ? '正在绘制这一格' : panel.renderError ? '这一格需要重绘' : panel.scene }}</strong>
            <button v-if="panel.renderError && !panel.rendering" type="button" @click.stop="generatePanel(idx)">重绘此格</button>
          </div>
          <div v-if="panel.dialogue" class="manga-copy" :class="[isNarration(panel) ? 'is-narration' : 'is-dialogue', idx % 2 ? 'on-left' : 'on-right']"><span>{{ panel.dialogue }}</span></div>
          <div class="panel-number">{{ idx + 1 }}</div>
        </div>
      </div>
      <div v-if="assembledImage" class="assembled-result"><span>最终漫画页</span><img :src="assetUrl(assembledImage)" alt="最终漫画页" /></div>
    </section>
    <div v-if="showSdGuide" class="comic-modal" @click.self="showSdGuide = false">
      <section class="sd-guide">
        <button class="modal-close" @click="showSdGuide = false"><Icon icon="mdi:close" width="19" /></button>
        <h2>还没有检测到绘图引擎</h2>
        <p class="sd-guide-sub">漫画工坊用本机的 Stable Diffusion 出图。选一种方式装好绘图引擎，装完回到这里点「启动绘图引擎」即可。</p>
        <div class="sd-guide-list">
          <a v-for="link in sdLinks" :key="link.url" class="sd-guide-item" :class="{ primary: link.primary }" :href="link.url" target="_blank" rel="noopener">
            <span class="sd-guide-name">{{ link.name }}</span>
            <span class="sd-guide-desc">{{ link.desc }}</span>
            <span class="sd-guide-open">打开<Icon icon="mdi:arrow-top-right" width="13" /></span>
          </a>
        </div>
        <p class="sd-guide-tip">装好后回到这里点「启动绘图引擎」会自动识别。用秋叶整合包的话，请在启动器设置里打开「启用 API」开关，否则无法出图。装在非常见位置时，可设置环境变量 RESCENE_SD_LAUNCHER 指向启动文件。</p>
        <div class="sd-guide-actions">
          <button class="primary" @click="retrySDFromGuide"><Icon icon="mdi:refresh" width="15" /> 我装好了，重新检测</button>
          <button class="ghost" @click="showSdGuide = false">暂不安装</button>
        </div>
      </section>
    </div>
    <div v-if="showCharCreator" class="comic-modal" @click.self="showCharCreator = false">
      <section class="char-creator">
        <button class="modal-close" @click="showCharCreator = false"><Icon icon="mdi:close" width="19" /></button>
        <h2>新建角色</h2>
        <label>角色名 <input v-model="newChar.name" placeholder="如：林雪" /></label>
        <label>性别 <select v-model="newChar.gender"><option value="女">女</option><option value="男">男</option></select></label>
        <label>外貌描述 <textarea v-model="newChar.look" rows="3" placeholder="英文描述，用于AI生图保持一致"></textarea></label>
        <label>画风 <input v-model="newChar.style" placeholder="如：anime, manga style" /></label>
        <button class="primary" :disabled="!newChar.name.trim()" @click="saveCharacter">保存角色</button>
      </section>
    </div>
    <div v-if="showPanelDetail" class="comic-modal" @click.self="showPanelDetail = false">
      <section class="panel-detail">
        <button class="modal-close" @click="showPanelDetail = false"><Icon icon="mdi:close" width="19" /></button>
        <h2>第{{ detailPanel.index + 1 }}格 · {{ detailPanel.scene }}</h2>
        <div class="detail-grid">
          <div><label>镜头</label><span>{{ detailPanel.camera }}</span></div>
          <div><label>角色</label><span>{{ detailPanel.character }}</span></div>
          <div><label>动作</label><span>{{ detailPanel.action }}</span></div>
          <div><label>对话</label><span>{{ detailPanel.dialogue || '无' }}</span></div>
        </div>
        <div class="detail-prompt"><label>生图提示词</label><textarea :value="detailPanel.promptEn" rows="4" readonly></textarea></div>
        <button class="primary" :disabled="panels[detailPanel.index]?.rendering" @click="generatePanel(detailPanel.index)">{{ detailPanel.imageUrl ? '重新绘制此格' : '生成此格图片' }}</button>
      </section>
    </div>
  </main>
</template>

<script setup>
import { computed, reactive, ref, onMounted, nextTick } from 'vue'
import { Icon } from '@iconify/vue'

const chapter = ref('')
const title = ref('')
const style = ref('日式漫画，细腻线条，柔和光影')
const panelCount = ref(4)
const selectedChar = ref('')
const characters = ref([])
const panels = ref([])
const pageTitle = ref('')
const layout = ref('manga')
const generating = ref(false)
const renderingPanels = ref(false)
const assembling = ref(false)
const sdOnline = ref(false)
const sdModel = ref('')
const engineStarting = ref(false)
const pageId = ref('')
const assembledImage = ref('')
const books = ref([])
const selectedBookId = ref('')
const selectedChapterId = ref('')
const statusText = ref('等待输入')
const showCharCreator = ref(false)
const showPanelDetail = ref(false)
const showSdGuide = ref(false)
const sdLinks = ref([])
const detailPanel = ref({ index: 0, scene: '', dialogue: '', character: '', action: '', camera: '', promptEn: '' })
const charCount = computed(() => characters.value.length)
const renderedCount = computed(() => panels.value.filter(panel => panel.imageUrl).length)
const selectedBook = computed(() => books.value.find(book => book.id === selectedBookId.value) || null)
const newChar = reactive({ name: '', gender: '女', look: '', style: '' })

function apiFetch(path, options) {
  if (['localhost', '127.0.0.1'].includes(window.location.hostname))
    return fetch('http://127.0.0.1:8081' + path, options)
  return fetch(path, options)
}

async function readAPIResponse(res, fallback = '请求失败') {
  const raw = await res.text()
  if (!raw) return {}
  try { return JSON.parse(raw) }
  catch {
    if (res.status === 404 || raw.toLowerCase().includes('page not found')) throw new Error('漫画服务版本过旧，请重启应用后再试')
    throw new Error(res.ok ? '服务返回了无法识别的数据' : `${fallback}（HTTP ${res.status}）`)
  }
}

onMounted(() => { document.title = '杉汐 | 漫画工坊'; loadCharacters(); loadBooks(); checkSD() })

function assetUrl(path) { if (!path) return ''; if (/^(data:|https?:)/.test(path)) return path; return ['localhost','127.0.0.1'].includes(window.location.hostname) ? 'http://127.0.0.1:8081' + path : path }

async function checkSD() {
  try { const res = await apiFetch('/api/comic/status'); const data = await res.json(); sdOnline.value = Boolean(data.online); sdModel.value = data.model || ''; engineStarting.value = Boolean(data.starting); return data }
  catch (e) { sdOnline.value = false; statusText.value = 'SD 检查失败: ' + e.message }
}

function wait(ms) { return new Promise(resolve => setTimeout(resolve, ms)) }
async function loadSdLinks() {
  try { const res = await apiFetch('/api/comic/sd-download'); const data = await res.json(); sdLinks.value = data.links || [] } catch {}
}

async function retrySDFromGuide() {
  const state = await checkSD()
  if (state?.online) { showSdGuide.value = false; statusText.value = '绘图引擎已就绪'; return }
  statusText.value = '仍未检测到绘图引擎，确认已安装并启动后再试'
}

async function ensureSD() {
  const current = await checkSD()
  if (current?.online) return true
  engineStarting.value = true; statusText.value = '正在自动启动绘图引擎，首次加载模型可能需要几分钟…'
  try {
    const start = await apiFetch('/api/comic/start-sd', { method: 'POST' }); const started = await readAPIResponse(start, '绘图引擎启动失败')
    if (!start.ok) {
      // needInstall = 本机没装绘图引擎：拉取下载引导并弹窗，让用户选择国内直达链接安装
      if (start.status === 404 || started.needInstall) { await loadSdLinks(); showSdGuide.value = true; throw new Error('本机未安装绘图引擎') }
      throw new Error(started.error || '绘图引擎启动失败')
    }
    if (started.needManualApi) {
      // 图形启动器（如秋叶整合包）需要用户手动开启 API 开关
      statusText.value = started.message || '请在绘图引擎启动器中开启 API 开关'
      engineStarting.value = false
      return false
    }
    for (let attempt = 0; attempt < 180; attempt++) {
      await wait(2000)
      const state = await checkSD()
      if (state?.online) { statusText.value = '绘图引擎已就绪'; engineStarting.value = false; return true }
      if (state && !state.starting && state.error) throw new Error(state.error)
    }
    throw new Error('模型加载超时，请查看绘图引擎日志')
  } catch (e) { engineStarting.value = false; statusText.value = '绘图引擎启动失败: ' + e.message; return false }
}

async function loadBooks() {
  try { const res = await apiFetch('/api/publish/books'); const data = await res.json(); books.value = data.books || []; if (books.value.length) { selectedBookId.value = books.value[0].id; selectBook() } } catch {}
}
function selectBook() { selectedChapterId.value = ''; const book = selectedBook.value; if (!book) return; style.value = book.style || style.value; const last = book.chapters?.[book.chapters.length - 1]; if (last) { selectedChapterId.value = last.id; selectChapter() } }
function selectChapter() { const item = selectedBook.value?.chapters?.find(ch => ch.id === selectedChapterId.value); if (!item) return; title.value = item.title || ''; chapter.value = String(item.content || '').replace(/^#\s+.*\n?/, '').trim() }

async function loadCharacters() {
  try { const res = await apiFetch('/api/comic/characters'); const data = await res.json(); characters.value = data.characters || [] } catch {}
}

async function saveCharacter() {
  try {
    const res = await apiFetch('/api/comic/characters', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(newChar) })
    const data = await res.json()
    if (res.ok) { characters.value = data.characters; showCharCreator.value = false; Object.assign(newChar, { name: '', gender: '女', look: '', style: '' }) }
  } catch (e) { alert('保存失败: ' + e.message) }
}

async function startGenerate() {
  generating.value = true; statusText.value = '正在准备绘图引擎…'
  try {
    const sdReady = ensureSD()
    statusText.value = 'AI 正在拆解分镜…'
    const character = characters.value.find(item => item.name === selectedChar.value)
    const characterBrief = character ? [character.name, character.look, character.style].filter(Boolean).join('；') : selectedChar.value
    const res = await apiFetch('/api/comic/generate', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ title: title.value || '未命名章节', chapter: chapter.value, style: style.value, character: characterBrief, panels: panelCount.value }) })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error)
    panels.value = data.panels || []; pageTitle.value = data.title; pageId.value = data.pageId || ''; assembledImage.value = ''; statusText.value = '已生成 ' + panels.value.length + ' 格分镜'
    layout.value = panels.value.length <= 4 ? 'manga' : 'dense'
    if (await sdReady) await renderAllPanels()
  } catch (e) { statusText.value = '生成失败: ' + e.message; alert(e.message) }
  finally { generating.value = false }
}

async function generatePanel(index) {
  const panel = panels.value[index]; if (!panel || panel.rendering) return false
  panel.rendering = true; panel.renderError = ''; statusText.value = '正在生成第 ' + (index + 1) + ' 格图片…'
  try {
    const character = characters.value.find(item => item.name === selectedChar.value)
    const renderPanel = { ...panel, promptEn: [panel.promptEn, character?.look, character?.style, style.value].filter(Boolean).join(', ') }
    const res = await apiFetch('/api/comic/render-panel', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ pageId: pageId.value, panel: renderPanel, genType: 'sd' }) })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error)
    panel.imageUrl = assetUrl(data.imageUrl || '')
    statusText.value = '第 ' + (index + 1) + ' 格已生成'
    return true
  } catch (e) { panel.imageUrl = ''; panel.renderError = e.message || '出图失败'; statusText.value = '第 ' + (index + 1) + ' 格生成失败，稍后可重绘'; return false }
  finally { panel.rendering = false }
}

async function renderAllPanels() {
  if (!pageId.value || renderingPanels.value) return
  renderingPanels.value = true
  try {
    for (let index = 0; index < panels.value.length; index++) {
      if (panels.value[index].imageUrl) continue
      let done = await generatePanel(index)
      if (!done) { await wait(900); done = await generatePanel(index) }
    }
    if (panels.value.every(panel => panel.imageUrl)) await assemblePage()
    else statusText.value = '部分画面需要重绘，已保留其他完成内容'
  }
  finally { renderingPanels.value = false }
}
async function assemblePage() {
  assembling.value = true; statusText.value = '正在拼成漫画页…'
  try {
    const res = await apiFetch('/api/comic/assemble', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ pageId: pageId.value, title: pageTitle.value, dialogue: panels.value.map(panel => panel.dialogue || ''), panels: panels.value.map(panel => panel.imageUrl || '') }) }); const data = await res.json()
    if (!res.ok || data.error) throw new Error(data.error || '拼页失败')
    assembledImage.value = await captureComicPage()
    statusText.value = '漫画页已完成'
  }
  catch (e) { statusText.value = '拼页失败: ' + e.message }
  finally { assembling.value = false }
}

function openPanelDetail(index) { detailPanel.value = { ...panels.value[index], index }; showPanelDetail.value = true }
function isNarration(panel) { return !String(panel.character || '').trim() || /^(旁白|叙述|画外音)[：:]/.test(String(panel.dialogue || '')) }
function handlePanelImageError(index) { const panel = panels.value[index]; if (!panel) return; panel.imageUrl = ''; panel.renderError = '图片加载失败'; statusText.value = '第 ' + (index + 1) + ' 格图片加载失败，请重绘' }
function panelStyle(index) {
  const t = panels.value.length
  if (t <= 1) return { gridColumn: '1 / -1', gridRow: '1 / -1' }
  if (t === 2) return index === 0 ? { gridColumn: '1 / -1', gridRow: '1' } : { gridColumn: '1 / -1', gridRow: '2' }
  if (t === 4) { const p = [{ gridColumn: '1', gridRow: '1 / 3' }, { gridColumn: '2', gridRow: '1' }, { gridColumn: '2', gridRow: '2' }, { gridColumn: '1 / -1', gridRow: '3' }]; return p[index] || {} }
  if (t === 6) return { gridColumn: String(index % 2 + 1), gridRow: String(Math.floor(index / 2) + 1) }
  return { gridColumn: String(index % 3 + 1), gridRow: String(Math.floor(index / 3) + 1) }
}
async function captureComicPage() {
  await nextTick()
  const el = document.querySelector('.comic-page'); if (!el) throw new Error('漫画页面尚未渲染')
  const h = await import('html2canvas')
  const canvas = await h.default(el, { scale: 2, useCORS: true, backgroundColor: '#ffffff' })
  return canvas.toDataURL('image/png')
}
async function exportPage() {
  try {
    const image = assembledImage.value || await captureComicPage()
    const a = document.createElement('a'); a.download = (pageTitle.value || 'comic') + '.png'; a.href = image; a.click()
  } catch (e) { alert('导出失败: ' + e.message) }
}
function resetPage() { panels.value = []; pageTitle.value = ''; pageId.value = ''; assembledImage.value = ''; statusText.value = '等待输入' }
</script>

<style scoped>
.comic-view { min-height: 100vh; padding: 24px 78px 64px 24px; color: #152019; background: #f0f2f5; }
.comic-header { max-width: 1360px; margin: 0 auto 18px; padding: 28px 30px; border-radius: 20px; background: linear-gradient(135deg,#f0e8f0,#e8f0f5); box-shadow: 0 14px 40px rgba(30,40,50,.08); }
.comic-kicker { color: #8b5e7c; font: 700 10px Georgia,serif; letter-spacing: .18em; }
.comic-header h1 { margin: 8px 0 6px; font: 500 clamp(28px,3.5vw,40px) Georgia,'Noto Serif SC','Songti SC',serif; }
.comic-header p { margin: 0; color: #6a7a70; font-size: 13px; }
.comic-header-status { margin-top: 14px; display: flex; align-items: center; gap: 12px; padding: 10px 14px; border-radius: 10px; background: rgba(255,255,255,.5); font-size: 11px; }
.comic-header-status i { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #bbb; margin-right: 4px; }
.comic-header-status i.online { background: #5da87d; box-shadow: 0 0 0 4px rgba(93,168,125,.15); }
.comic-header-status i.starting { background: #c58b57; box-shadow: 0 0 0 4px rgba(197,139,87,.15); animation: enginePulse 1.2s ease-in-out infinite; }
@keyframes enginePulse { 50% { transform: scale(1.35); opacity: .55; } }
.comic-input-section { display: grid; grid-template-columns: 1fr 320px; gap: 16px; max-width: 1360px; margin: 0 auto 18px; }
.comic-input-left, .comic-input-right { padding: 20px; border-radius: 16px; background: #fff; box-shadow: 0 8px 25px rgba(0,0,0,.04); }
.comic-input-left > label, .comic-input-right > label { display: block; margin-bottom: 8px; color: #4a5a50; font-size: 11px; font-weight: 800; }
.comic-input-left textarea { box-sizing: border-box; width: 100%; min-height: 140px; padding: 12px; border: 1px solid #dee3e0; border-radius: 10px; font: inherit; font-size: 13px; line-height: 1.7; resize: vertical; outline: none; }
.comic-input-left textarea:focus { border-color: #8b7a9e; box-shadow: 0 0 0 3px rgba(139,122,158,.1); }
.source-row { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin: 11px 0 2px; padding: 10px; border: 1px solid #e7e1ea; border-radius: 10px; background: #faf8fc; }.source-row label { display: flex; align-items: center; gap: 7px; color: #695d70; font-size: 10px; font-weight: 800; }.source-row select { min-width: 0; flex: 1; height: 32px; border: 1px solid #ded6e3; border-radius: 7px; background: #fff; }
.comic-input-meta { display: grid; grid-template-columns: 1fr 1fr 80px; gap: 10px; margin-top: 12px; }
.comic-input-meta label { display: flex; flex-direction: column; gap: 4px; color: #4a5a50; font-size: 10px; font-weight: 800; }
.comic-input-meta input, .comic-input-meta select { height: 34px; padding: 0 10px; border: 1px solid #dee3e0; border-radius: 8px; font: inherit; font-size: 12px; outline: none; }
.comic-char-list { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 14px; }
.char-chip { display: flex; align-items: center; gap: 6px; padding: 6px 10px; border: 1px solid #e2e6e3; border-radius: 8px; background: #fafbfa; cursor: pointer; font-size: 10px; }
.char-chip.active { border-color: #8b7a9e; background: #f5f0fa; }
.char-avatar { display: grid; place-items: center; width: 24px; height: 24px; border-radius: 50%; }
.char-chip b { display: block; color: #2c3a32; }
.char-chip small { display: block; color: #8a9a90; margin-top: 1px; }
.char-add { display: inline-flex; align-items: center; gap: 4px; padding: 6px 10px; border: 1px dashed #bcc5bf; border-radius: 8px; color: #6a7a70; background: transparent; font-size: 10px; cursor: pointer; }
.comic-actions { display: flex; gap: 8px; margin-top: 10px; }
.comic-actions button { display: inline-flex; align-items: center; gap: 6px; min-height: 40px; padding: 0 16px; border: 0; border-radius: 10px; color: #fff; background: linear-gradient(135deg,#5a7d8a,#3d5f6b); font-size: 12px; font-weight: 800; cursor: pointer; }
.comic-actions button.secondary { border: 1px solid #d6dce0; color: #596870; background: #fff; }
.comic-actions button:disabled { opacity: .45; cursor: not-allowed; }
.comic-page-section { max-width: 1360px; margin: 0 auto; }
.section-head { display: flex; align-items: center; gap: 16px; margin-bottom: 14px; }
.section-head span { color: #8b7a9e; font: 700 9px Georgia,serif; letter-spacing: .16em; }
.section-head h2 { margin: 0; font: 500 20px Georgia,'Noto Serif SC','Songti SC',serif; color: #2c3a32; }
.section-actions { margin-left: auto; display: flex; gap: 8px; }
.render-progress { align-self: center; color: #718078 !important; font: 700 9px ui-monospace,monospace !important; letter-spacing: 0 !important; }
.section-actions button { padding: 6px 12px; border: 1px solid #d0d6d2; border-radius: 8px; color: #4a5a50; background: #fff; font-size: 10px; font-weight: 800; cursor: pointer; }
.comic-page { display: grid; grid-auto-flow: dense; gap: 5px; padding: 5px; border-radius: 12px; background: #fff; box-shadow: 0 12px 35px rgba(0,0,0,.12); }
.comic-page.layout-manga { grid-template-columns: 1fr 1fr; grid-auto-rows: minmax(230px,28vw); }
.comic-page.layout-dense { grid-template-columns: 1fr 1fr 1fr; grid-auto-rows: minmax(210px,24vw); }
.comic-panel { position: relative; min-width: 0; min-height: 0; overflow: hidden; border: 3px solid #171717; border-radius: 2px; background: #f3f0e9; cursor: pointer; transition: transform .15s, box-shadow .15s; }
.comic-panel:hover { transform: scale(1.02); z-index: 2; }
.comic-panel.has-error { border-style: dashed; border-color: #b98989; }
.panel-image { display: block; width: 100%; height: 100%; object-fit: cover; object-position: center; }
.panel-placeholder { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px; height: 100%; padding: 24px; color: #786f68; background: repeating-linear-gradient(-45deg,#f8f5ef,#f8f5ef 10px,#f2eee7 10px,#f2eee7 20px); font-size: 11px; text-align: center; }
.panel-placeholder strong { max-width: 22em; font-weight: 700; line-height: 1.6; }
.panel-placeholder button { min-height: 36px; padding: 0 14px; border: 1px solid #a88d91; border-radius: 999px; color: #76545d; background: #fff; font-weight: 800; cursor: pointer; }
.manga-copy { position: absolute; z-index: 2; top: 18px; display: grid; place-items: center; max-height: calc(100% - 60px); color: #111; background: rgba(255,255,255,.97); box-shadow: 0 3px 14px rgba(0,0,0,.18); font-family: 'Noto Serif SC','Songti SC','Yu Mincho',serif; }
.manga-copy.on-right { right: 18px; }
.manga-copy.on-left { left: 18px; }
.manga-copy span { writing-mode: vertical-rl; text-orientation: upright; letter-spacing: .08em; line-height: 1.65; }
.manga-copy.is-dialogue { min-width: 46px; padding: 17px 12px 22px; border: 2px solid #171717; border-radius: 48% 52% 46% 54% / 15% 18% 82% 85%; font-size: clamp(13px,1.05vw,17px); }
.manga-copy.is-dialogue::after { content: ''; position: absolute; bottom: -13px; width: 18px; height: 24px; border-right: 2px solid #171717; border-bottom: 2px solid #171717; background: #fff; transform: rotate(28deg) skew(-12deg); }
.manga-copy.on-right.is-dialogue::after { right: 14px; }
.manga-copy.on-left.is-dialogue::after { left: 14px; transform: rotate(60deg) skew(-12deg); }
.manga-copy.is-narration { padding: 14px 11px; border: 1.5px solid #171717; border-radius: 1px; font-size: clamp(12px,.95vw,15px); }
.panel-number { position: absolute; bottom: 6px; right: 6px; width: 20px; height: 20px; display: grid; place-items: center; border-radius: 50%; color: #fff; background: rgba(0,0,0,.5); font: 800 9px ui-monospace; }
.assembled-result { margin-top: 18px; padding: 18px; border-radius: 15px; background: #fff; box-shadow: 0 10px 30px rgba(0,0,0,.06); }.assembled-result span { display: block; margin-bottom: 10px; color: #8b5e7c; font: 800 10px Georgia,serif; letter-spacing: .14em; }.assembled-result img { display: block; width: min(800px,100%); margin: auto; border-radius: 8px; box-shadow: 0 12px 28px rgba(0,0,0,.18); }
.comic-modal { position: fixed; z-index: 10050; inset: 0; display: grid; place-items: center; padding: 24px; background: rgba(40,40,50,.5); backdrop-filter: blur(8px); }
.char-creator, .panel-detail { position: relative; width: min(480px,92vw); padding: 28px; border-radius: 20px; background: #fff; box-shadow: 0 24px 60px rgba(0,0,0,.18); }
.modal-close { position: absolute; top: 12px; right: 12px; display: grid; place-items: center; width: 32px; height: 32px; border: 0; border-radius: 50%; color: #666; background: rgba(0,0,0,.04); cursor: pointer; }
.char-creator h2, .panel-detail h2 { margin: 0 0 18px; font: 500 20px Georgia,'Noto Serif SC',serif; color: #2c3a32; }
.char-creator label, .panel-detail label { display: block; margin-bottom: 12px; color: #4a5a50; font-size: 11px; font-weight: 800; }
.char-creator input, .char-creator textarea, .char-creator select, .panel-detail textarea { box-sizing: border-box; width: 100%; margin-top: 4px; padding: 0 10px; border: 1px solid #dee3e0; border-radius: 8px; font: inherit; font-size: 12px; outline: none; }
.char-creator input, .char-creator select { height: 36px; }
.char-creator textarea { padding: 8px 10px; min-height: 60px; resize: vertical; }
.char-creator .primary, .panel-detail .primary { margin-top: 10px; min-height: 38px; padding: 0 14px; border: 0; border-radius: 9px; color: #fff; background: linear-gradient(135deg,#5a7d8a,#3d5f6b); font-size: 11px; font-weight: 800; cursor: pointer; }
.sd-guide { position: relative; width: min(560px, 92vw); max-height: 86vh; overflow-y: auto; padding: 28px; border-radius: 20px; background: #fff; box-shadow: 0 24px 60px rgba(0,0,0,.18); }
.sd-guide h2 { margin: 0 0 8px; font: 500 20px Georgia,'Noto Serif SC',serif; color: #2c3a32; }
.sd-guide-sub { margin: 0 0 16px; color: #6a7a70; font-size: 12px; line-height: 1.7; }
.sd-guide-list { display: flex; flex-direction: column; gap: 10px; margin-bottom: 14px; }
.sd-guide-item { display: grid; grid-template-columns: 1fr auto; grid-template-rows: auto auto; column-gap: 10px; padding: 12px 14px; border: 1px solid #e2e6e3; border-radius: 12px; background: #fafbfa; text-decoration: none; transition: border-color .15s, background .15s; }
.sd-guide-item:hover { border-color: #8b7a9e; background: #f5f0fa; }
.sd-guide-item.primary { border-color: #8b7a9e; background: #f5f0fa; }
.sd-guide-name { color: #2c3a32; font-size: 13px; font-weight: 800; }
.sd-guide-desc { grid-column: 1; color: #8a9a90; font-size: 11px; line-height: 1.6; margin-top: 3px; }
.sd-guide-open { grid-column: 2; grid-row: 1 / 3; align-self: center; display: inline-flex; align-items: center; gap: 3px; color: #8b7a9e; font-size: 11px; font-weight: 800; white-space: nowrap; }
.sd-guide-tip { margin: 0 0 14px; padding: 10px 12px; border-radius: 10px; background: #f4f6f4; color: #59685e; font-size: 11px; line-height: 1.7; }
.sd-guide-actions { display: flex; gap: 10px; }
.sd-guide-actions button { display: inline-flex; align-items: center; gap: 6px; min-height: 38px; padding: 0 16px; border-radius: 9px; font-size: 11px; font-weight: 800; cursor: pointer; }
.sd-guide-actions .primary { border: 0; color: #fff; background: linear-gradient(135deg,#5a7d8a,#3d5f6b); }
.sd-guide-actions .ghost { border: 1px solid #d6dce0; color: #596870; background: #fff; }
.detail-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 14px; }
.detail-grid label { display: block; color: #8a9a90; font-size: 9px; margin-bottom: 2px; }
.detail-grid span { color: #2c3a32; font-size: 12px; }
.detail-prompt textarea { width: 100%; min-height: 80px; padding: 8px; border: 1px solid #dee3e0; border-radius: 8px; font-size: 11px; resize: vertical; color: #555; background: #f8f9f8; }
.spin { animation: spin .8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 760px) { .comic-view { padding: 12px 12px 68px; }.comic-input-section { grid-template-columns: 1fr; }.comic-input-meta,.source-row { grid-template-columns: 1fr; }.section-head { align-items: flex-start; flex-direction: column; }.section-actions { margin-left: 0; flex-wrap: wrap; } }
</style>
