<template>
  <main class="publish-view">
    <header class="publish-command">
      <div class="command-copy">
        <span class="command-kicker">CREATOR'S GARDEN · 创作者灵感花园</span>
        <h1>从一个脑洞，到连载发布。</h1>
        <p>梳理设定、搭建大纲、续写章节、校对润色；最后一步，由你决定发往哪家小说平台。</p>
      </div>
      <div class="command-status">
        <span>当前任务</span>
        <strong>{{ projectName }}</strong>
        <div><i :class="{ online: edgeReady }"></i>{{ edgeReady ? '发布通道已连接' : '发布通道待检查' }}</div>
      </div>
      <div class="command-metrics">
        <div><span>创作旅程</span><strong>{{ completedCount }}/4</strong><small>位搭档已完成</small></div>
        <div><span>文字生长</span><strong>{{ contentLength }}</strong><small>个字符</small></div>
        <div><span>准备去往</span><strong>{{ picked.length }}</strong><small>个内容平台</small></div>
        <div class="ready"><span>发布状态</span><strong>{{ publishReady ? '可以出发' : '继续创作' }}</strong><small>由你最后确认</small></div>
      </div>
    </header>

    <section class="bookshelf-card">
      <div class="shelf-heading">
        <div><span>MY BOOKSHELF</span><h2>先选一本书，再开始写新章节</h2><p>默认打开最近编辑的书，设定和文风会自动带入本次创作。</p></div>
        <button type="button" class="new-book-button" @click="showBookCreator = true"><Icon icon="mdi:book-plus-outline" width="17" /> 创建新书</button>
      </div>
      <div v-if="books.length" class="book-strip">
        <div v-for="book in books" :key="book.id" class="book-item" :class="{ active: selectedBook?.id === book.id }">
          <button type="button" class="book-chip" @click="selectBook(book)"><span class="mini-cover" :style="coverStyle(book)"><img v-if="book.cover" :src="book.cover" alt="" /><Icon v-else icon="mdi:book-open-page-variant-outline" width="22" /></span><span><b>{{ book.title }}</b><small>{{ book.genre || '未设置题材' }} · {{ book.chapters?.length || 0 }} 章</small><em>{{ selectedBook?.id === book.id ? '当前创作' : formatBookTime(book.lastOpenedAt) }}</em></span></button>
          <div class="book-actions"><button type="button" title="编辑书籍" @click="editBook(book)"><Icon icon="mdi:pencil-outline" width="15" /></button><button type="button" title="删除书籍" @click="deleteBook(book)"><Icon icon="mdi:trash-can-outline" width="15" /></button></div>
        </div>
      </div>
      <button v-else type="button" class="empty-shelf" @click="showBookCreator = true"><Icon icon="mdi:bookshelf" width="28" /><span><b>书架还是空的</b><small>创建第一本书，之后每次会默认回到最近创作的作品</small></span></button>
    </section>

    <section class="pipeline-card">
      <div class="section-head">
        <div><span>CREATIVE JOURNEY</span><h2>让灵感沿着这条路，慢慢长成作品</h2></div>
        <span class="progress-label">创作进度 {{ Math.round((completedCount / 4) * 100) }}%</span>
      </div>
      <div class="stage-track">
        <button v-for="(stage, index) in stages" :key="stage.key" type="button" class="stage-step"
          :class="{ done: isStageDone(stage.key), current: activeStage === index, locked: isStageLocked(index) }"
          :style="{ '--stage-color': stage.color, '--stage-rgb': stage.rgb }" @click="selectStage(index)">
          <span class="stage-line"><i></i></span>
          <span class="stage-body">
            <span class="stage-icon"><Icon :icon="stage.icon" width="18" /></span>
            <span><b>{{ stage.name }}</b><small>{{ stage.agent }}</small></span>
            <em>0{{ index + 1 }}</em>
          </span>
        </button>
      </div>
    </section>

    <div class="workspace-grid auto-workspace">
      <section class="editor-panel">
        <div class="section-head editor-head">
          <div><span>{{ currentStage.eyebrow }}</span><h2>{{ currentStage.title }}</h2></div>
          <div class="editor-head-actions"><button type="button" class="file-button" :disabled="!content.trim()" @click="showPreview = true"><Icon icon="mdi:book-open-variant" width="17" /> 阅读预览</button><label class="file-button"><Icon icon="mdi:file-upload-outline" width="17" /> 导入文稿<input type="file" accept=".md,.txt,.markdown" @change="onFileChange" /></label></div>
        </div>

        <div v-if="activeStage === 0 && !outputs.ideation" class="brief-editor">
          <label>这次想写什么故事</label>
          <textarea v-model="brief" rows="7" placeholder="例如：题材与时代背景、主角身份与欲望、金手指或核心设定、开篇冲突、预计篇幅。只有一个脑洞也可以先写下来。"></textarea>
          <div class="brief-options">
            <label><span>作品定位</span><input v-model="audience" placeholder="例如：女频古言、男频都市、悬疑推理、轻小说" /></label>
            <label><span>文风与创作要求</span><input v-model="style" placeholder="由你自由填写，例如：轻松沙雕、第一人称、感情线慢热、每章结尾留钩子" /></label>
          </div>
        </div>

        <div v-else-if="activeStage < 4" class="artifact-editor">
          <div class="artifact-meta">
            <span><i></i>{{ currentStage.agent }} 正在负责</span>
            <span>{{ currentArtifact.length }} 字</span>
          </div>
          <textarea v-model="currentArtifactModel" :placeholder="currentStage.placeholder"></textarea>
        </div>

        <div v-else class="final-editor">
          <label>最终标题</label>
          <input v-model="title" placeholder="小说名或本章标题" />
          <label>最终正文</label>
          <textarea v-model="content" placeholder="完成创作流水线，或直接导入已有小说章节"></textarea>
          <div v-if="fileName" class="file-proof"><Icon icon="mdi:file-check-outline" width="17" /> {{ fileName }} · {{ contentLength }} 字</div>
        </div>

        <div class="editor-actions">
          <button v-if="activeStage > 0" type="button" class="secondary" @click="activeStage--"><Icon icon="mdi:arrow-left" width="16" /> 上一步</button>
          <span class="action-hint">{{ currentStage.hint }}</span>
          <button v-if="activeStage < 4" type="button" class="primary" :disabled="composing || !brief.trim()" @click="startAutoPipeline">
            <Icon :icon="composing ? 'mdi:loading' : 'mdi:auto-fix'" :class="{ spin: composing }" width="17" />
            {{ composing ? `${currentStage.agent} 正在接力…` : completedCount ? '重新启动自动接力' : '开始创作，自动完成四步' }}
          </button>
          <button v-else type="button" class="primary publish-button" :disabled="busy || !publishReady" @click="publish">
            <Icon :icon="busy ? 'mdi:loading' : 'mdi:rocket-launch-outline'" :class="{ spin: busy }" width="17" />
            {{ busy ? '正在发布…' : `确认并发布到 ${picked.length} 个平台` }}
          </button>
          <button v-if="activeStage === 4" type="button" class="secondary save-draft-button" :disabled="savingChapter || !selectedBook || !title.trim() || !content.trim()" @click="approveChapter"><Icon :icon="savingChapter ? 'mdi:loading' : 'mdi:check-decagram-outline'" :class="{ spin: savingChapter }" width="17" /> {{ savingChapter ? '保存中…' : '审批通过并保存' }}</button>
        </div>
        <div v-if="activeStage === 4" class="human-review"><label>不满意？写下修改建议，整条创作链会自动重新接力</label><textarea v-model="reviewFeedback" placeholder="例如：主角反应太平静，加强雨夜压迫感；保留结尾悬念，但不要提前暴露凶手。"></textarea><button type="button" class="reject-review" :disabled="composing || !reviewFeedback.trim()" @click="rejectAndRerun"><Icon icon="mdi:backup-restore" width="16" /> 打回并自动重写</button></div>
        <div v-if="notice" class="notice" :class="{ error: noticeError }">{{ notice }}</div>
      </section>

      <section class="floating-manuscript" @wheel.prevent="switchPreviewByWheel">
        <div class="paper-shadow"></div><div class="paper-back"></div>
        <article class="paper-sheet">
          <header><span>{{ previewStage + 1 }}/4 · {{ stages[previewStage].name }}</span><em>滚动滚轮切换版本</em></header>
          <div class="paper-book"><small>{{ selectedBook?.title || '尚未选择书籍' }}</small><h2>{{ previewTitle }}</h2></div>
          <div class="paper-content">{{ previewContent || '稿纸会一直留在这里。启动创作后，构思、提纲、初稿和润色稿会依次同步到纸上。' }}</div>
          <footer><button v-for="(_, index) in stages.slice(0,4)" :key="index" type="button" :class="{ active: previewStage === index, ready: Boolean(outputs[stages[index].key]) }" @click="previewStage = index">0{{ index + 1 }}</button></footer>
        </article>
      </section>

      <aside class="publish-sidebar">
        <section class="agent-roster">
          <div class="aside-title"><span>我的创作搭档</span><button type="button" @click="resetPipeline">重新开始</button></div>
          <div v-for="(stage, index) in stages.slice(0, 4)" :key="stage.key" class="agent-row" :class="{ active: activeStage === index, running: composing && activeStage === index, done: isStageDone(stage.key) }" :style="{ '--agent-color': stage.color, '--agent-rgb': stage.rgb }">
            <span class="agent-avatar" :style="{ color: stage.color, background: `rgba(${stage.rgb}, .1)` }"><Icon :icon="stage.icon" width="18" /></span>
            <span><b>{{ stage.agent }}</b><small>{{ stage.responsibility }}</small></span>
            <span class="agent-state"><Icon :icon="isStageDone(stage.key) ? 'mdi:check-circle' : composing && activeStage === index ? 'mdi:lightning-bolt' : 'mdi:circle-outline'" width="17" /></span>
            <i v-if="index < 3" class="handoff-current"></i>
          </div>
        </section>

        <section class="channel-panel">
          <div class="aside-title"><span>作品准备去往哪里</span><button type="button" @click="checkEdge">{{ checkingEdge ? '检查中…' : '检查连接' }}</button></div>
          <div class="channel-tools"><button type="button" @click="pickAll">全选</button><button type="button" @click="pickNone">清空</button><span>{{ picked.length }}/{{ platforms.length }}</span></div>
          <div class="channel-list">
            <button v-for="platform in platforms" :key="platform.id" type="button" :class="{ selected: picked.includes(platform.id), warning: platform.minLen > contentLength }" @click="toggle(platform.id)">
              <span class="channel-check"><Icon :icon="picked.includes(platform.id) ? 'mdi:check' : 'mdi:plus'" width="14" /></span>
              <span><b>{{ platform.name }}</b><small>{{ platform.genre }}<template v-if="platform.minLen"> · ≥{{ platform.minLen }} 字</template></small></span>
            </button>
          </div>
          <p v-if="loginMsg" class="edge-message" :class="{ ok: edgeReady }">{{ loginMsg }}</p>
        </section>

        <section v-if="results.length" class="result-panel">
          <div class="aside-title"><span>发布小回执</span></div>
          <div v-for="result in results" :key="result.platform" class="result-row" :class="{ ok: result.ok }">
            <Icon :icon="result.ok ? 'mdi:check-circle' : 'mdi:alert-circle'" width="17" />
            <span><b>{{ result.name }}</b><small>{{ result.message || (result.ok ? '发布成功' : '发布失败') }}</small></span>
          </div>
        </section>
      </aside>
    </div>

    <div v-if="showBookCreator" class="novel-modal" @click.self="closeBookEditor">
      <section class="book-creator">
        <button class="modal-close" type="button" @click="closeBookEditor"><Icon icon="mdi:close" width="19" /></button>
        <div class="creator-cover">
          <label :style="draftCover ? { backgroundImage: `url(${draftCover})` } : {}"><input type="file" accept="image/png,image/jpeg,image/webp" @change="onCoverChange" /><Icon v-if="!draftCover" icon="mdi:image-plus-outline" width="28" /><span>{{ draftCover ? '更换封面' : '上传书籍封面' }}</span><small>JPG / PNG / WEBP，建议 3:4</small></label>
        </div>
        <div class="creator-fields"><span>{{ editingBookId ? 'EDIT STORY' : 'CREATE A NEW STORY' }}</span><h2>{{ editingBookId ? '编辑书籍资料' : '把新故事放上书架' }}</h2><label>书名<input v-model="newBook.title" placeholder="输入书名" /></label><label>题材定位<input v-model="newBook.genre" placeholder="例如：古言权谋、都市异能、无限流" /></label><label>故事简介<textarea v-model="newBook.summary" placeholder="主角是谁、想要什么、最大的阻碍是什么"></textarea></label><label>长期文风与禁忌<input v-model="newBook.style" placeholder="自由填写，后续新章节默认继承，也可以随时修改" /></label><button type="button" class="primary create-book-submit" :disabled="creatingBook || !newBook.title.trim()" @click="saveBookEditor">{{ creatingBook ? '正在保存…' : editingBookId ? '保存书籍修改' : '创建并开始第一章' }}</button></div>
      </section>
    </div>

    <div v-if="showPreview" class="novel-modal preview-modal" @click.self="showPreview = false">
      <article class="reader-preview">
        <button class="modal-close" type="button" @click="showPreview = false"><Icon icon="mdi:close" width="19" /></button>
        <header><span class="reader-cover" :style="coverStyle(selectedBook || {})"><img v-if="selectedBook?.cover" :src="selectedBook.cover" alt="书籍封面" /><Icon v-else icon="mdi:book-open-page-variant-outline" width="30" /></span><span><small>{{ selectedBook?.title || '未选择书籍' }}</small><h1>{{ cleanChapterTitle }}</h1><em>{{ contentLength }} 字 · 草稿预览</em></span></header>
        <div class="reader-body">{{ cleanChapterContent }}</div>
      </article>
    </div>
  </main>
</template>

<script setup>
import { computed, reactive, ref, onMounted } from 'vue'
import { Icon } from '@iconify/vue'

const platforms = ref([])
const picked = ref([])
const title = ref('')
const content = ref('')
const brief = ref('')
const audience = ref('')
const style = ref('')
const outputs = reactive({ ideation: '', outline: '', draft: '', polish: '' })
const activeStage = ref(0)
const results = ref([])
const busy = ref(false)
const composing = ref(false)
const notice = ref('')
const noticeError = ref(false)
const fileName = ref('')
const loginMsg = ref('')
const edgeReady = ref(false)
const checkingEdge = ref(false)
const books = ref([])
const selectedBook = ref(null)
const showBookCreator = ref(false)
const showPreview = ref(false)
const creatingBook = ref(false)
const savingChapter = ref(false)
const draftCover = ref('')
const newBook = reactive({ title: '', genre: '', summary: '', style: '' })
const editingBookId = ref('')
const previewStage = ref(0)
const reviewFeedback = ref('')

const stages = [
  { key: 'ideation', name: '构思', agent: '灵感策划师', responsibility: '题材卖点 · 主角冲突', icon: 'mdi:lightbulb-on-outline', color: '#b76f92', rgb: '183,111,146', eyebrow: '01 · 灵感萌芽', title: '把脑洞变成能展开的故事设定', placeholder: '人物、世界观、核心冲突和差异化卖点会出现在这里，也可以人工修改。', hint: '把现有脑洞交给灵感策划师，先补齐故事支点。', action: '生成故事构思' },
  { key: 'outline', name: '提纲', agent: '故事结构师', responsibility: '主线推进 · 章节钩子', icon: 'mdi:file-tree-outline', color: '#db936f', rgb: '219,147,111', eyebrow: '02 · 故事成形', title: '搭建主线、人物弧光和章节节奏', placeholder: '先完成故事构思，再生成卷纲或章节细纲。', hint: '可先修改人物设定和冲突，再交给故事结构师。', action: '生成小说提纲' },
  { key: 'draft', name: '成稿', agent: '文字创作师', responsibility: '场景描写 · 章节正文', icon: 'ph:pen-nib-bold', color: '#7b83c5', rgb: '123,131,197', eyebrow: '03 · 文字生长', title: '把提纲续写成完整小说章节', placeholder: '完整章节初稿会出现在这里。', hint: '章节会基于提纲推进，不擅自改变核心设定。', action: '续写章节正文' },
  { key: 'polish', name: '润色', agent: '风格编辑师', responsibility: '人设一致 · 节奏语言', icon: 'mdi:creation-outline', color: '#70a895', rgb: '112,168,149', eyebrow: '04 · 细节打磨', title: '校对人设、节奏与叙述口吻', placeholder: '校对润色后的完整章节会出现在这里。', hint: '保留你的文风要求，重点修复重复、跳戏和设定冲突。', action: '校对并润色' },
  { key: 'publish', name: '发布', agent: '由我确认', responsibility: '渠道选择 · 最终确认', icon: 'mdi:rocket-launch-outline', color: '#c6576f', rgb: '198,87,111', eyebrow: '05 · 准备出发', title: '最终检查，然后一键发布', hint: '真实发布是外部操作，最后一步由你决定。' },
]

const currentStage = computed(() => stages[activeStage.value])
const projectName = computed(() => title.value.trim() || brief.value.trim().split(/[。！？\n]/)[0]?.slice(0, 28) || '未命名发布任务')
const contentLength = computed(() => Array.from(content.value || '').length)
const completedCount = computed(() => stages.slice(0, 4).filter(stage => outputs[stage.key].trim()).length)
const publishReady = computed(() => Boolean(title.value.trim() && content.value.trim() && picked.value.length))
const cleanChapterTitle = computed(() => title.value.trim() || '未命名章节')
const cleanChapterContent = computed(() => content.value.replace(/^#\s+.*\n?/, '').trim())
const previewContent = computed(() => outputs[stages[previewStage.value].key] || '')
const previewTitle = computed(() => previewStage.value < 2 ? stages[previewStage.value].title : (extractTitle(previewContent.value) || title.value || stages[previewStage.value].title))
const currentArtifact = computed(() => outputs[currentStage.value.key] || '')
const currentArtifactModel = computed({ get: () => outputs[currentStage.value.key] || '', set: value => { outputs[currentStage.value.key] = value } })
const canRunCurrent = computed(() => {
  if (activeStage.value === 0) return Boolean(brief.value.trim() || content.value.trim())
  const previous = stages[activeStage.value - 1]
  return Boolean(outputs[previous.key]?.trim() || (activeStage.value >= 2 && content.value.trim()))
})

function isStageDone(key) { return key === 'publish' ? results.value.some(item => item.ok) : Boolean(outputs[key]?.trim()) }
function isStageLocked(index) { return index > 0 && index < 4 && !outputs[stages[index - 1].key]?.trim() }
function selectStage(index) { if (!isStageLocked(index) || index === 4) activeStage.value = index }
function extractTitle(markdown) { return String(markdown || '').split('\n').find(line => /^#\s+/.test(line))?.replace(/^#\s+/, '').trim() || '' }
function formatBookTime(value) { if (!value) return '尚未打开'; return new Date(value).toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' }) + ' 编辑' }
function coverStyle(book) { if (book?.cover) return {}; const palettes = [['#f2d9e3','#a75378'],['#dedcf1','#777db5'],['#e8d8ca','#a66d52']]; const pair = palettes[Math.abs(String(book?.title || '').length) % palettes.length]; return { background: `linear-gradient(145deg,${pair[0]},${pair[1]})` } }
function switchPreviewByWheel(event) { previewStage.value = Math.max(0, Math.min(3, previewStage.value + (event.deltaY > 0 ? 1 : -1))) }

function apiFetch(path, options) {
  if (['localhost', '127.0.0.1'].includes(window.location.hostname)) return fetch(`http://127.0.0.1:8081${path}`, options)
  return fetch(path, options)
}

async function loadBooks() {
  try { const response = await apiFetch('/api/publish/books'); const data = await response.json(); if (!response.ok) throw new Error(data.error); books.value = data.books || []; if (books.value.length) selectBook(books.value[0], false) }
  catch (error) { notice.value = `书架加载失败：${error.message}`; noticeError.value = true }
}
async function selectBook(book, touch = true) {
  selectedBook.value = book
  audience.value = book.genre || ''
  style.value = book.style || ''
  brief.value = book.summary || ''
  picked.value = Array.isArray(book.platforms) ? [...book.platforms] : []
  if (touch) { try { await apiFetch(`/api/publish/books/${encodeURIComponent(book.id)}/open`, { method: 'POST' }); books.value = [book, ...books.value.filter(item => item.id !== book.id)] } catch {} }
}
function onCoverChange(event) {
  const file = event.target.files?.[0]; if (!file) return
  if (file.size > 3 * 1024 * 1024) { notice.value = '封面不能超过 3MB'; noticeError.value = true; return }
  const reader = new FileReader(); reader.onload = () => { draftCover.value = String(reader.result || '') }; reader.readAsDataURL(file)
}
function editBook(book) { editingBookId.value = book.id; Object.assign(newBook, { title: book.title, genre: book.genre || '', summary: book.summary || '', style: book.style || '' }); draftCover.value = book.cover || ''; showBookCreator.value = true }
function closeBookEditor() { showBookCreator.value = false; editingBookId.value = ''; Object.assign(newBook, { title: '', genre: '', summary: '', style: '' }); draftCover.value = '' }
async function saveBookEditor() { if (editingBookId.value) return updateBook(); return createBook() }
async function updateBook() {
  creatingBook.value = true
  try { const response = await apiFetch(`/api/publish/books/${encodeURIComponent(editingBookId.value)}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...newBook, cover: draftCover.value, platforms: selectedBook.value?.id === editingBookId.value ? picked.value : books.value.find(book => book.id === editingBookId.value)?.platforms || [] }) }); const data = await response.json(); if (!response.ok) throw new Error(data.error); books.value = books.value.map(book => book.id === data.book.id ? data.book : book); if (selectedBook.value?.id === data.book.id) await selectBook(data.book, false); notice.value = `《${data.book.title}》资料已更新`; noticeError.value = false; closeBookEditor() }
  catch (error) { notice.value = `修改失败：${error.message}`; noticeError.value = true }
  finally { creatingBook.value = false }
}
async function deleteBook(book) {
  if (!window.confirm(`确定删除《${book.title}》及其 ${book.chapters?.length || 0} 个章节吗？此操作无法撤销。`)) return
  try { const response = await apiFetch(`/api/publish/books/${encodeURIComponent(book.id)}`, { method: 'DELETE' }); const data = await response.json(); if (!response.ok) throw new Error(data.error); books.value = books.value.filter(item => item.id !== book.id); if (selectedBook.value?.id === book.id) { selectedBook.value = null; resetChapter(); if (books.value.length) await selectBook(books.value[0]) }; notice.value = `《${book.title}》已删除`; noticeError.value = false }
  catch (error) { notice.value = `删除失败：${error.message}`; noticeError.value = true }
}
async function createBook() {
  creatingBook.value = true
  try { const response = await apiFetch('/api/publish/books', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...newBook, cover: draftCover.value, platforms: picked.value }) }); const data = await response.json(); if (!response.ok) throw new Error(data.error); books.value = [data.book, ...books.value]; await selectBook(data.book, false); closeBookEditor(); resetChapter(); notice.value = `《${data.book.title}》已创建，可以开始第一章`; noticeError.value = false }
  catch (error) { notice.value = `创建失败：${error.message}`; noticeError.value = true }
  finally { creatingBook.value = false }
}
function resetChapter() { title.value = ''; content.value = ''; fileName.value = ''; Object.keys(outputs).forEach(key => { outputs[key] = '' }); activeStage.value = 0; results.value = [] }
async function saveChapter() {
  if (!selectedBook.value) return
  savingChapter.value = true
  try { const response = await apiFetch(`/api/publish/books/${encodeURIComponent(selectedBook.value.id)}/chapters`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ title: title.value, content: content.value }) }); const data = await response.json(); if (!response.ok) throw new Error(data.error); selectedBook.value = data.book; books.value = [data.book, ...books.value.filter(book => book.id !== data.book.id)]; notice.value = `《${data.book.title}》已保存“${data.chapter.title}”草稿`; noticeError.value = false }
  catch (error) { notice.value = `章节保存失败：${error.message}`; noticeError.value = true }
  finally { savingChapter.value = false }
}
async function approveChapter() { await saveChapter(); if (!noticeError.value) notice.value = `${notice.value}，已通过人工审批` }

async function runCurrentStage() {
  if (!canRunCurrent.value || activeStage.value >= 4) return
  composing.value = true
  notice.value = ''
  noticeError.value = false
  const stage = currentStage.value
  const source = stage.key === 'ideation' ? (brief.value || content.value) : outputs[stages[activeStage.value - 1].key]
  try {
    const request = { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ stage: stage.key, brief: brief.value, audience: audience.value, style: style.value, title: title.value, content: source }) }
    let response = await fetch('/api/publish/compose', request)
    // 本地旧桌面后台尚未包含新路由时，只为开发预览回退到新版联调服务。
    if (response.status === 404 && ['localhost', '127.0.0.1'].includes(window.location.hostname)) {
      response = await fetch('http://127.0.0.1:8081/api/publish/compose', request)
    }
    const data = await response.json()
    if (!response.ok) throw new Error(data.error || `请求失败 (${response.status})`)
    outputs[stage.key] = data.content || ''
    if (stage.key === 'draft' || stage.key === 'polish') {
      content.value = outputs[stage.key]
      title.value = extractTitle(outputs[stage.key]) || title.value
    }
    activeStage.value = Math.min(activeStage.value + 1, 4)
  } catch (error) {
    notice.value = error.message
    noticeError.value = true
  } finally { composing.value = false }
}

async function requestStage(index, feedback = '') {
  activeStage.value = index
  previewStage.value = index
  const stage = stages[index]
  const source = index === 0 ? brief.value : outputs[stages[index - 1].key]
  const request = { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ stage: stage.key, brief: `${brief.value}${feedback ? `\n\n人工打回建议：${feedback}` : ''}`, audience: audience.value, style: style.value, title: title.value, content: source }) }
  const response = await apiFetch('/api/publish/compose', request)
  const data = await response.json()
  if (!response.ok) throw new Error(data.error || `${stage.agent} 接力失败`)
  outputs[stage.key] = data.content || ''
  previewStage.value = index
  if (index >= 2) { content.value = outputs[stage.key]; title.value = extractTitle(outputs[stage.key]) || title.value }
}
async function startAutoPipeline(feedback = '') {
  if (!brief.value.trim() || composing.value) return
  composing.value = true; notice.value = ''; noticeError.value = false; results.value = []
  Object.keys(outputs).forEach(key => { outputs[key] = '' }); content.value = ''; title.value = ''
  try { for (let index = 0; index < 4; index++) await requestStage(index, feedback); activeStage.value = 4; previewStage.value = 3; notice.value = '四位创作搭档已完成接力，请预览后审批或打回修改。' }
  catch (error) { notice.value = `自动接力中断：${error.message}`; noticeError.value = true }
  finally { composing.value = false }
}
async function rejectAndRerun() { const feedback = reviewFeedback.value.trim(); reviewFeedback.value = ''; await startAutoPipeline(feedback) }

function onFileChange(event) {
  const file = event.target.files?.[0]
  if (!file) return
  fileName.value = file.name
  const reader = new FileReader()
  reader.onload = () => {
    const text = String(reader.result || '')
    title.value = extractTitle(text) || file.name.replace(/\.(md|txt|markdown)$/i, '')
    content.value = text
    outputs.draft = text
    outputs.polish = text
    activeStage.value = 4
  }
  reader.readAsText(file, 'utf-8')
}

async function loadPlatforms() {
  try {
    const response = await fetch('/api/publish/platforms')
    const data = await response.json()
    if (!response.ok) throw new Error(data.error || '平台加载失败')
    platforms.value = data.platforms || []
  } catch (error) { notice.value = `加载平台失败：${error.message}`; noticeError.value = true }
}
async function checkEdge() {
  checkingEdge.value = true
  try {
    const response = await fetch('/api/publish/login-edge', { method: 'POST' })
    const data = await response.json()
    edgeReady.value = Boolean(data.ok)
    loginMsg.value = data.message || ''
  } catch (error) { edgeReady.value = false; loginMsg.value = `检查失败：${error.message}` }
  finally { checkingEdge.value = false }
}
function toggle(id) { picked.value = picked.value.includes(id) ? picked.value.filter(item => item !== id) : [...picked.value, id] }
function pickAll() { picked.value = platforms.value.map(platform => platform.id) }
function pickNone() { picked.value = [] }
function resetPipeline() { resetChapter(); brief.value = selectedBook.value?.summary || ''; audience.value = selectedBook.value?.genre || ''; style.value = selectedBook.value?.style || ''; notice.value = '' }

async function publish() {
  if (!publishReady.value) return
  busy.value = true
  notice.value = ''
  noticeError.value = false
  results.value = []
  try {
    const response = await fetch('/api/publish', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ title: title.value, content: content.value, platforms: picked.value }) })
    const data = await response.json()
    if (!response.ok) throw new Error(data.error || `发布失败 (${response.status})`)
    results.value = data.results || []
    const successCount = results.value.filter(item => item.ok).length
    notice.value = `发布完成：${successCount}/${results.value.length} 个平台成功`
    noticeError.value = successCount !== results.value.length
  } catch (error) { notice.value = `发布失败：${error.message}`; noticeError.value = true }
  finally { busy.value = false }
}

onMounted(() => { document.title = '杉汐 | 小说创作'; loadPlatforms(); loadBooks() })
</script>

<style scoped>
.publish-view { min-height: 100vh; padding: 24px 78px 64px 24px; color: #152019; background: #f3f5f2; font-family: -apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Microsoft YaHei',sans-serif; }
.publish-command { position: relative; overflow: hidden; max-width: 1360px; margin: 0 auto 18px; padding: 30px 30px 0; border: 1px solid #263249; border-radius: 22px; color: #fff; background: #101827; box-shadow: 0 18px 50px rgba(15,23,42,.16); }
.publish-command::after { position: absolute; top: -180px; right: -80px; width: 480px; height: 480px; border: 1px solid rgba(74,222,128,.12); border-radius: 50%; box-shadow: 0 0 0 72px rgba(74,222,128,.025),0 0 0 144px rgba(74,222,128,.018); content: ''; }
.command-copy { position: relative; z-index: 1; padding-right: 330px; }
.command-kicker { color: #4ade80; font-size: 9px; font-weight: 900; letter-spacing: .18em; }
.command-copy h1 { margin: 9px 0 9px; font-size: clamp(30px,4vw,48px); line-height: 1.05; letter-spacing: -.05em; }
.command-copy p { margin: 0 0 30px; color: #9eacbe; font-size: 13px; }
.command-status { position: absolute; z-index: 2; top: 30px; right: 30px; width: 250px; padding: 16px; border: 1px solid rgba(148,163,184,.2); border-radius: 14px; background: rgba(8,15,27,.48); backdrop-filter: blur(10px); }
.command-status > span { color: #718098; font-size: 9px; font-weight: 900; letter-spacing: .12em; }
.command-status strong { display: block; overflow: hidden; margin: 6px 0 14px; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.command-status div { display: flex; align-items: center; gap: 7px; color: #8c9bad; font-size: 10px; }
.command-status i { width: 7px; height: 7px; border-radius: 50%; background: #f59e0b; }
.command-status i.online { background: #4ade80; box-shadow: 0 0 0 4px rgba(74,222,128,.12); }
.command-metrics { position: relative; z-index: 1; display: grid; grid-template-columns: repeat(4,1fr); margin: 0 -30px; border-top: 1px solid rgba(148,163,184,.18); }
.command-metrics > div { padding: 17px 30px 20px; border-right: 1px solid rgba(148,163,184,.18); }
.command-metrics span,.command-metrics small { display: block; color: #8492a6; font-size: 9px; }
.command-metrics strong { display: block; margin: 5px 0 2px; font-size: 24px; }
.command-metrics small { font-size: 8px; font-weight: 900; letter-spacing: .12em; }
.command-metrics .ready strong { color: #4ade80; }
.pipeline-card,.editor-panel,.publish-sidebar > section { border: 1px solid #dfe5e1; border-radius: 18px; background: #fff; box-shadow: 0 10px 30px rgba(15,23,42,.045); }
.bookshelf-card { max-width: 1360px; margin: 0 auto 18px; padding: 22px 26px; border: 1px solid rgba(185,155,168,.28); border-radius: 18px; background: rgba(255,253,252,.88); box-shadow: 0 14px 38px rgba(112,80,96,.065),inset 0 1px 0 #fff; backdrop-filter: blur(14px); }
.shelf-heading { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin-bottom: 16px; }.shelf-heading span { color: #ab718b; font: 700 9px Georgia,serif; letter-spacing: .16em; }.shelf-heading h2 { margin: 4px 0; color: #493942; font: 500 20px Georgia,'Noto Serif SC','Songti SC',serif; }.shelf-heading p { margin: 0; color: #9a8791; font-size: 10px; }.new-book-button { display: inline-flex; align-items: center; gap: 6px; min-height: 39px; padding: 0 13px; border: 1px solid #dabac8; border-radius: 10px; color: #9c5373; background: #fff8fa; font-size: 10px; font-weight: 800; cursor: pointer; }
.book-strip { display: grid; grid-auto-columns: minmax(240px,290px); grid-auto-flow: column; gap: 10px; overflow-x: auto; padding: 2px 0 6px; scrollbar-width: thin; scrollbar-color: #d4a2b8 transparent; }.book-item { position: relative; overflow: hidden; border: 1px solid #eee1e6; border-radius: 13px; background: rgba(255,250,249,.8); }.book-item.active { border-color: #d19aaf; background: linear-gradient(115deg,#fff0f5,#f5f0fb); box-shadow: 0 7px 20px rgba(154,78,113,.09); }.book-chip { display: grid; grid-template-columns: 52px minmax(0,1fr); align-items: center; gap: 11px; width: 100%; min-width: 0; padding: 9px 76px 9px 9px; border: 0; color: #6d5963; background: transparent; text-align: left; cursor: pointer; }.book-actions { position: absolute; top: 8px; right: 8px; display: flex; gap: 3px; opacity: 0; transition: opacity .18s; }.book-item:hover .book-actions,.book-item.active .book-actions { opacity: 1; }.book-actions button { display: grid; place-items: center; width: 28px; height: 28px; border: 1px solid #eadbe1; border-radius: 8px; color: #9b6179; background: rgba(255,255,255,.82); cursor: pointer; }.book-actions button:last-child:hover { color: #bd4d5c; background: #fff0f0; }.mini-cover { display: grid; place-items: center; width: 52px; aspect-ratio: 3/4; overflow: hidden; border-radius: 7px; color: #fff; box-shadow: 0 5px 12px rgba(69,45,57,.15); }.mini-cover img,.reader-cover img { width: 100%; height: 100%; object-fit: cover; }.book-chip b,.book-chip small,.book-chip em { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.book-chip b { color: #4d3c44; font-size: 11px; }.book-chip small { margin-top: 5px; color: #9b8791; font-size: 8px; }.book-chip em { margin-top: 6px; color: #b46989; font-size: 8px; font-style: normal; }.empty-shelf { display: flex; align-items: center; justify-content: center; gap: 12px; width: 100%; min-height: 82px; border: 1px dashed #d9bcc8; border-radius: 13px; color: #a75d7d; background: #fff9fa; cursor: pointer; }.empty-shelf span { text-align: left; }.empty-shelf b,.empty-shelf small { display: block; }.empty-shelf small { margin-top: 4px; color: #a9959e; font-size: 9px; }
.pipeline-card { max-width: 1360px; margin: 0 auto 18px; padding: 22px 26px 24px; }
.section-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; margin-bottom: 20px; }
.section-head > div > span,.aside-title > span { color: #738078; font-size: 9px; font-weight: 900; letter-spacing: .15em; }
.section-head h2 { margin: 5px 0 0; font-size: 19px; letter-spacing: -.025em; }
.progress-label { padding: 7px 10px; border-radius: 999px; color: #176b47; background: #edf9f1; font-size: 9px; font-weight: 900; }
.stage-track { display: grid; grid-template-columns: repeat(5,minmax(0,1fr)); gap: 7px; }
.stage-step { min-width: 0; padding: 0; border: 0; color: inherit; background: transparent; text-align: left; cursor: pointer; }
.stage-step.locked { opacity: .52; cursor: not-allowed; }
.stage-line { position: relative; display: block; height: 7px; margin-bottom: 11px; border-radius: 999px; background: #e8ede9; }
.stage-line i { position: absolute; top: 50%; right: 0; width: 10px; height: 10px; border: 3px solid #fff; border-radius: 50%; background: #bfc9c2; box-shadow: 0 0 0 1px #d6ded8; transform: translate(2px,-50%); }
.stage-step.done .stage-line { background: var(--stage-color); }
.stage-step.current .stage-line { background: linear-gradient(90deg,rgba(var(--stage-rgb),.25),var(--stage-color),rgba(var(--stage-rgb),.35)); background-size: 180% 100%; box-shadow: 0 0 13px rgba(var(--stage-rgb),.34); animation: stage-flow 1.4s linear infinite; }
.stage-step.current .stage-line i { background: var(--stage-color); animation: stage-pulse 1.4s ease-out infinite; }
.stage-body { display: grid; grid-template-columns: 34px minmax(0,1fr) auto; align-items: center; gap: 8px; padding: 8px; border-radius: 11px; }
.stage-step.current .stage-body { background: rgba(var(--stage-rgb),.07); }
.stage-icon { display: grid; place-items: center; width: 32px; height: 32px; border-radius: 9px; color: #859188; background: #eef2ef; }
.stage-step.done .stage-icon,.stage-step.current .stage-icon { color: var(--stage-color); background: rgba(var(--stage-rgb),.11); }
.stage-body b,.stage-body small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.stage-body b { font-size: 11px; }.stage-body small { margin-top: 3px; color: #97a29b; font-size: 8px; }.stage-body em { align-self: start; color: #b4beb8; font: 800 8px ui-monospace,monospace; font-style: normal; }
.workspace-grid { display: grid; grid-template-columns: minmax(0,1.55fr) minmax(330px,.72fr); align-items: start; gap: 18px; max-width: 1360px; margin: 0 auto; }
.auto-workspace { grid-template-columns: minmax(390px,1.1fr) minmax(310px,.78fr) minmax(280px,.68fr); }
.editor-panel { padding: 24px 26px; }.editor-head { align-items: center; }
.editor-head-actions { display: flex; gap: 7px; }.file-button { display: inline-flex; align-items: center; gap: 6px; min-height: 38px; padding: 0 12px; border: 1px solid #dce3de; border-radius: 9px; color: #536159; background: #fff; font-size: 10px; font-weight: 800; cursor: pointer; }.file-button input { display: none; }.file-button:disabled { opacity: .42; cursor: not-allowed; }
.brief-editor > label,.final-editor > label { display: block; margin: 0 0 7px; color: #47564e; font-size: 11px; font-weight: 800; }
.brief-editor textarea,.artifact-editor textarea,.final-editor textarea,.final-editor input,.brief-options input,.brief-options select { box-sizing: border-box; width: 100%; border: 1px solid #dce3de; border-radius: 12px; color: #1f2c24; background: #fbfcfb; font: inherit; outline: none; transition: border-color .18s,box-shadow .18s; }
.brief-editor textarea,.artifact-editor textarea,.final-editor textarea { padding: 16px; resize: vertical; line-height: 1.75; }
.brief-editor textarea { min-height: 210px; font-size: 14px; }.artifact-editor textarea { min-height: 470px; font-size: 13px; }.final-editor textarea { min-height: 410px; margin-bottom: 10px; font-size: 13px; }.final-editor input { height: 46px; margin-bottom: 18px; padding: 0 14px; font-size: 15px; font-weight: 700; }
.brief-editor textarea:focus,.artifact-editor textarea:focus,.final-editor textarea:focus,.final-editor input:focus,.brief-options input:focus,.brief-options select:focus { border-color: #5aac7d; box-shadow: 0 0 0 3px rgba(90,172,125,.1); }
.brief-options { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-top: 14px; }.brief-options label span { display: block; margin-bottom: 6px; color: #738078; font-size: 10px; font-weight: 800; }.brief-options input,.brief-options select { height: 42px; padding: 0 12px; font-size: 12px; }
.artifact-meta { display: flex; justify-content: space-between; margin-bottom: 9px; color: #849188; font-size: 9px; }.artifact-meta span:first-child { display: flex; align-items: center; gap: 6px; }.artifact-meta i { width: 7px; height: 7px; border-radius: 50%; background: #22c55e; }
.file-proof { display: flex; align-items: center; gap: 6px; color: #16824f; font-size: 10px; }
.editor-actions { display: flex; align-items: center; gap: 10px; margin-top: 18px; padding-top: 16px; border-top: 1px solid #e9eeeb; }.editor-actions button { display: inline-flex; align-items: center; justify-content: center; gap: 6px; min-height: 42px; padding: 0 15px; border-radius: 10px; font-size: 11px; font-weight: 800; cursor: pointer; }.editor-actions button:disabled { opacity: .45; cursor: not-allowed; }.primary { margin-left: auto; border: 1px solid #16271d; color: #fff; background: #16271d; }.secondary { border: 1px solid #dce3de; color: #536159; background: #fff; }.publish-button { background: #be3f31; border-color: #be3f31; }.action-hint { color: #8b978f; font-size: 9px; }
.notice { margin-top: 12px; padding: 10px 12px; border-radius: 9px; color: #176b47; background: #edf9f1; font-size: 10px; }.notice.error { color: #a7362b; background: #fff0ee; }
.publish-sidebar { display: flex; flex-direction: column; gap: 14px; }.publish-sidebar > section { padding: 18px; }.aside-title { display: flex; align-items: center; justify-content: space-between; margin-bottom: 13px; }.aside-title button,.channel-tools button { padding: 0; border: 0; color: #16824f; background: transparent; font-size: 9px; font-weight: 800; cursor: pointer; }
.agent-row { position: relative; display: grid; grid-template-columns: 34px minmax(0,1fr) 18px; align-items: center; gap: 9px; padding: 10px; border-top: 1px solid #edf0ee; border-radius: 10px; color: #a2aca5; }.agent-row.active { color: var(--agent-color); background: rgba(var(--agent-rgb),.06); }.agent-row.done .agent-state { color: var(--agent-color); }.agent-avatar { display: grid; place-items: center; width: 32px; height: 32px; border-radius: 9px; }.agent-row b,.agent-row small { display: block; }.agent-row b { color: #2d3a32; font-size: 10px; }.agent-row small { margin-top: 3px; color: #929e96; font-size: 8px; }.agent-state { display: grid; place-items: center; }.agent-row.running::before { position: absolute; z-index: -1; inset: -1px; padding: 1px; border-radius: 11px; content: ''; background: conic-gradient(from var(--electric-angle),transparent 0 55%,rgba(var(--agent-rgb),.25) 68%,var(--agent-color) 76%,#fff 79%,transparent 87%); animation: electric-border 1.45s linear infinite; mask: linear-gradient(#000 0 0) content-box,linear-gradient(#000 0 0); mask-composite: exclude; }.agent-row.running { isolation: isolate; box-shadow: 0 0 20px rgba(var(--agent-rgb),.12); }.agent-row.running .agent-state { color: var(--agent-color); animation: electric-icon .8s ease-in-out infinite alternate; }.handoff-current { position: absolute; z-index: 2; right: 17px; bottom: -8px; width: 2px; height: 15px; background: #eadfe4; }.agent-row.done .handoff-current { background: linear-gradient(var(--agent-color),#eadfe4); box-shadow: 0 0 7px rgba(var(--agent-rgb),.4); }
.channel-tools { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }.channel-tools span { margin-left: auto; color: #9aa59e; font-size: 9px; }.channel-list { display: grid; gap: 6px; max-height: 300px; overflow-y: auto; }.channel-list > button { display: grid; grid-template-columns: 25px minmax(0,1fr); align-items: center; gap: 8px; padding: 9px; border: 1px solid #e4e9e5; border-radius: 10px; color: #657169; background: #fbfcfb; text-align: left; cursor: pointer; }.channel-list > button.selected { border-color: #8cd3a5; color: #176b47; background: #f1fbf4; }.channel-list > button.warning.selected { border-color: #f5c98a; background: #fff9ef; }.channel-check { display: grid; place-items: center; width: 23px; height: 23px; border-radius: 7px; background: #e9eeeb; }.selected .channel-check { color: #fff; background: #22a465; }.channel-list b,.channel-list small { display: block; }.channel-list b { color: #26332c; font-size: 10px; }.channel-list small { margin-top: 3px; color: #929e96; font-size: 8px; }.edge-message { margin: 12px 0 0; padding: 9px; border-radius: 8px; color: #95651e; background: #fff7e8; font-size: 9px; line-height: 1.5; }.edge-message.ok { color: #176b47; background: #edf9f1; }
.result-row { display: grid; grid-template-columns: 18px 1fr; gap: 8px; padding: 9px 0; border-top: 1px solid #edf0ee; color: #d04a3a; }.result-row.ok { color: #22a465; }.result-row b,.result-row small { display: block; }.result-row b { color: #334139; font-size: 10px; }.result-row small { margin-top: 2px; color: #87948c; font-size: 8px; }
.novel-modal { position: fixed; z-index: 10050; inset: 0; display: grid; place-items: center; padding: 24px; background: rgba(54,39,46,.46); backdrop-filter: blur(9px); }.book-creator { position: relative; display: grid; grid-template-columns: 230px minmax(0,1fr); gap: 28px; width: min(760px,94vw); padding: 30px; border: 1px solid rgba(255,255,255,.7); border-radius: 24px; background: linear-gradient(135deg,#fffaf8,#f8eef3 58%,#f1effa); box-shadow: 0 30px 90px rgba(55,31,43,.25); }.modal-close { position: absolute; z-index: 2; top: 14px; right: 14px; display: grid; place-items: center; width: 34px; height: 34px; border: 0; border-radius: 50%; color: #806b75; background: rgba(255,255,255,.7); cursor: pointer; }.creator-cover label { display: flex; align-items: center; justify-content: center; flex-direction: column; width: 100%; aspect-ratio: 3/4; border: 1px dashed #c99aaf; border-radius: 15px; color: #a6597a; background: linear-gradient(145deg,#f5dce6,#aaa6d5); background-size: cover; background-position: center; box-shadow: 0 18px 35px rgba(91,57,74,.18); cursor: pointer; }.creator-cover input { display: none; }.creator-cover span { margin-top: 10px; font-size: 11px; font-weight: 800; }.creator-cover small { margin-top: 5px; font-size: 8px; }.creator-fields > span { color: #ab718b; font: 700 9px Georgia,serif; letter-spacing: .16em; }.creator-fields h2 { margin: 6px 0 18px; color: #493942; font: 500 25px Georgia,'Noto Serif SC','Songti SC',serif; }.creator-fields label { display: block; margin-top: 11px; color: #725d68; font-size: 10px; font-weight: 800; }.creator-fields input,.creator-fields textarea { box-sizing: border-box; width: 100%; margin-top: 6px; padding: 0 12px; border: 1px solid #e3d5db; border-radius: 10px; color: #4d3e45; background: rgba(255,255,255,.78); font: inherit; outline: none; }.creator-fields input { height: 40px; }.creator-fields textarea { min-height: 70px; padding-top: 10px; resize: vertical; }.create-book-submit { width: 100%; min-height: 43px; margin-top: 17px; }
.reader-preview { position: relative; width: min(760px,94vw); max-height: 88vh; overflow-y: auto; padding: 44px clamp(28px,6vw,76px) 68px; border-radius: 22px; color: #41373b; background: #fffdf9; box-shadow: 0 30px 90px rgba(55,31,43,.25); }.reader-preview header { display: grid; grid-template-columns: 70px minmax(0,1fr); align-items: center; gap: 18px; padding-bottom: 25px; border-bottom: 1px solid #eee4e2; }.reader-cover { display: grid; place-items: center; width: 70px; aspect-ratio: 3/4; overflow: hidden; border-radius: 8px; color: #fff; box-shadow: 0 8px 20px rgba(69,45,57,.18); }.reader-preview header small { color: #a66a84; font-size: 10px; }.reader-preview header h1 { margin: 6px 0; font: 600 clamp(22px,4vw,30px) Georgia,'Noto Serif SC','Songti SC',serif; }.reader-preview header em { color: #a4969c; font-size: 9px; font-style: normal; }.reader-body { padding-top: 30px; color: #4b4245; font: 16px/2 Georgia,'Noto Serif SC','Songti SC',serif; white-space: pre-wrap; }
.floating-manuscript { position: sticky; top: 18px; min-height: 650px; padding: 12px 12px 20px; perspective: 900px; cursor: ns-resize; }.paper-shadow,.paper-back,.paper-sheet { position: absolute; inset: 12px; border-radius: 5px 13px 8px 5px; }.paper-shadow { transform: translate(12px,14px) rotate(1.8deg); background: rgba(83,54,68,.13); filter: blur(12px); }.paper-back { transform: translate(7px,8px) rotate(1deg); border: 1px solid #e7dbd6; background: #f9f3ee; }.paper-sheet { z-index: 2; display: flex; overflow: hidden; flex-direction: column; padding: 28px 28px 20px; border: 1px solid #eadfda; background: linear-gradient(90deg,transparent 0 35px,rgba(199,102,132,.13) 35px 36px,transparent 36px),repeating-linear-gradient(#fffdfa 0 31px,#ede6df 32px); box-shadow: 0 22px 45px rgba(85,53,68,.14); transform: rotate(-.45deg); }.paper-sheet::after { position: absolute; right: 10px; bottom: 9px; color: #cfbdc3; content: '✦'; }.paper-sheet > header { display: flex; justify-content: space-between; padding-bottom: 12px; color: #ae6a87; font: 800 9px ui-monospace,monospace; letter-spacing: .08em; }.paper-sheet > header em { color: #b4a4aa; font-size: 8px; font-style: normal; }.paper-book { padding: 6px 8px 16px; border-bottom: 1px solid #eadfda; }.paper-book small { color: #aa7188; font-size: 9px; }.paper-book h2 { margin: 6px 0 0; color: #493840; font: 600 21px Georgia,'Noto Serif SC','Songti SC',serif; }.paper-content { flex: 1; overflow-y: auto; padding: 20px 10px 30px; color: #55494e; font: 13px/2.45 Georgia,'Noto Serif SC','Songti SC',serif; white-space: pre-wrap; scrollbar-width: thin; scrollbar-color: #d4a2b8 transparent; }.paper-sheet footer { display: flex; justify-content: center; gap: 7px; padding-top: 12px; border-top: 1px solid #eadfda; }.paper-sheet footer button { width: 28px; height: 24px; border: 1px solid #e2d5da; border-radius: 999px; color: #b5a5ab; background: #fffaf8; font: 800 8px ui-monospace,monospace; cursor: pointer; }.paper-sheet footer button.ready { color: #a85c7d; }.paper-sheet footer button.active { border-color: #bf7896; color: #fff; background: #b76f92; box-shadow: 0 4px 10px rgba(183,111,146,.2); }
.human-review { display: grid; grid-template-columns: minmax(0,1fr) auto; gap: 8px; margin-top: 14px; padding: 13px; border: 1px solid #ebdce2; border-radius: 12px; background: #fff9fa; }.human-review label { grid-column: 1/-1; color: #80636f; font-size: 10px; font-weight: 800; }.human-review textarea { min-height: 68px; padding: 10px 12px; border: 1px solid #e5d5dc; border-radius: 9px; background: #fff; resize: vertical; }.reject-review { display: inline-flex; align-items: center; justify-content: center; gap: 5px; padding: 0 12px; border: 1px solid #d99aab; border-radius: 9px; color: #a94c68; background: #fff1f3; font-size: 10px; font-weight: 800; cursor: pointer; }.reject-review:disabled { opacity: .4; cursor: not-allowed; }

/* Creator's Garden — deliberately softer than the operations-focused Company view. */
.publish-view { position: relative; color: #42383e; background:
  radial-gradient(circle at 9% 7%, rgba(241,201,214,.42), transparent 25%),
  radial-gradient(circle at 91% 28%, rgba(210,205,241,.42), transparent 27%),
  linear-gradient(145deg,#fbf7f3 0%,#f8f3f5 48%,#f5f4fa 100%); }
.publish-view::before { position: fixed; inset: 0; z-index: 0; pointer-events: none; opacity: .46; content: ''; background-image: radial-gradient(rgba(154,119,136,.18) .65px,transparent .65px); background-size: 18px 18px; mask-image: linear-gradient(to bottom,#000,transparent 70%); }
.publish-view > * { position: relative; z-index: 1; }
.publish-command { min-height: 250px; padding: 42px 38px 0; border-color: rgba(197,151,169,.32); color: #49353f; background:
  radial-gradient(circle at 85% 10%,rgba(255,255,255,.86),transparent 25%),
  linear-gradient(118deg,rgba(255,247,241,.97),rgba(244,221,230,.95) 53%,rgba(225,220,246,.95)); box-shadow: 0 24px 65px rgba(127,86,108,.14); }
.publish-command::before { position: absolute; top: -68px; right: 250px; width: 210px; height: 210px; border: 1px solid rgba(183,111,146,.16); border-radius: 48% 52% 62% 38%; content: ''; transform: rotate(22deg); }
.publish-command::after { top: -220px; right: -60px; width: 540px; height: 540px; border-color: rgba(183,111,146,.12); box-shadow: 0 0 0 68px rgba(255,255,255,.08),0 0 0 136px rgba(123,131,197,.035); }
.command-kicker { color: #a04f74; font-family: Georgia,'Times New Roman',serif; font-size: 10px; letter-spacing: .2em; }
.command-copy h1 { max-width: 680px; margin-top: 13px; color: #402e37; font-family: Georgia,'Noto Serif SC','Songti SC',serif; font-weight: 500; letter-spacing: -.045em; }
.command-copy p { color: #806d77; font-size: 14px; }
.command-status { border-color: rgba(167,121,143,.24); color: #543f49; background: rgba(255,255,255,.52); box-shadow: inset 0 1px 0 rgba(255,255,255,.82); }
.command-status > span,.command-status div { color: #8d7782; }.command-status i { background: #d39b61; }.command-status i.online { background: #79a892; box-shadow: 0 0 0 5px rgba(112,168,149,.14); }
.command-metrics { border-color: rgba(152,110,131,.16); background: rgba(255,255,255,.28); backdrop-filter: blur(12px); }
.command-metrics > div { border-color: rgba(152,110,131,.16); }.command-metrics span,.command-metrics small { color: #917b86; }.command-metrics strong { color: #54414a; font-family: Georgia,'Noto Serif SC',serif; }.command-metrics .ready strong { color: #a04f74; font-size: 18px; }
.pipeline-card,.editor-panel,.publish-sidebar > section { border-color: rgba(185,155,168,.28); background: rgba(255,253,252,.88); box-shadow: 0 16px 45px rgba(112,80,96,.075),inset 0 1px 0 #fff; backdrop-filter: blur(14px); }
.pipeline-card { padding: 26px 30px 28px; }.section-head > div > span,.aside-title > span { color: #ab718b; font-family: Georgia,serif; letter-spacing: .16em; }.section-head h2 { color: #493942; font-family: Georgia,'Noto Serif SC','Songti SC',serif; font-size: 21px; font-weight: 500; }
.progress-label { color: #98526f; background: #f9eaf0; box-shadow: inset 0 0 0 1px rgba(183,111,146,.12); }
.stage-line { height: 5px; background: #eee6e8; }.stage-line i { border-color: #fffaf8; background: #cfc1c6; }.stage-body { padding: 10px 9px; border: 1px solid transparent; transition: transform .2s ease,background .2s ease,border-color .2s ease; }.stage-step:not(.locked):hover .stage-body { transform: translateY(-2px); }.stage-step.current .stage-body { border-color: rgba(var(--stage-rgb),.16); background: linear-gradient(135deg,rgba(var(--stage-rgb),.12),rgba(255,255,255,.42)); }.stage-icon { border-radius: 50%; background: #f1eaec; }.stage-body b { color: #514149; font-size: 12px; }.stage-body small { color: #9b8992; }
.editor-panel { padding: 29px 30px; }.file-button { border-color: #e5d8dd; color: #8c6175; background: #fff9f8; }.brief-editor > label,.final-editor > label { color: #715b66; }
.brief-editor textarea,.artifact-editor textarea,.final-editor textarea,.final-editor input,.brief-options input,.brief-options select { border-color: #e5dadd; color: #473b41; background: rgba(255,252,250,.86); box-shadow: inset 0 1px 5px rgba(104,71,87,.025); }.brief-editor textarea:focus,.artifact-editor textarea:focus,.final-editor textarea:focus,.final-editor input:focus,.brief-options input:focus,.brief-options select:focus { border-color: #c47b9b; box-shadow: 0 0 0 4px rgba(183,111,146,.1); }
.artifact-meta i { background: #b76f92; box-shadow: 0 0 0 4px rgba(183,111,146,.1); }.editor-actions { border-color: #eee4e7; }.primary { border-color: #a95479; background: linear-gradient(135deg,#b8678b,#995174); box-shadow: 0 9px 22px rgba(166,82,121,.2); }.primary:not(:disabled):hover { transform: translateY(-1px); box-shadow: 0 12px 27px rgba(166,82,121,.28); }.secondary { border-color: #e2d5da; color: #806371; background: #fffaf8; }.publish-button { border-color: #b65170; background: linear-gradient(135deg,#d16f88,#a84668); }.action-hint { color: #a08c95; }
.notice { color: #7d5367; background: #f9edf2; }.notice.error { color: #a54e5f; background: #fff0f0; }
.aside-title button,.channel-tools button { color: #a65377; }.agent-row { border-color: #f0e7ea; }.agent-row.active { color: #a65377; background: linear-gradient(90deg,#fbecf2,#f8f3fb); }.agent-row.done > svg { color: #b76f92; }.agent-row b { color: #55444c; }.agent-row small { color: #a08d96; }
.channel-list > button { border-color: #eee3e7; color: #826d77; background: rgba(255,251,250,.75); transition: transform .18s ease,border-color .18s ease,box-shadow .18s ease; }.channel-list > button:hover { transform: translateY(-1px); box-shadow: 0 7px 18px rgba(117,80,99,.07); }.channel-list > button.selected { border-color: #d6a2b8; color: #9b4d70; background: linear-gradient(115deg,#fff1f5,#f5f0fc); }.channel-check { background: #f1e8eb; }.selected .channel-check { background: #b76f92; }.channel-list b { color: #55434c; }.edge-message.ok { color: #557968; background: #edf7f1; }
.publish-view,.channel-list,.artifact-editor textarea,.final-editor textarea { scrollbar-width: thin; scrollbar-color: #d4a2b8 transparent; }
.publish-view::-webkit-scrollbar,.channel-list::-webkit-scrollbar,.artifact-editor textarea::-webkit-scrollbar,.final-editor textarea::-webkit-scrollbar { width: 8px; height: 8px; }
.publish-view::-webkit-scrollbar-track,.channel-list::-webkit-scrollbar-track,.artifact-editor textarea::-webkit-scrollbar-track,.final-editor textarea::-webkit-scrollbar-track { margin-block: 10px; border-radius: 999px; background: rgba(128,92,109,.055); }
.publish-view::-webkit-scrollbar-thumb,.channel-list::-webkit-scrollbar-thumb,.artifact-editor textarea::-webkit-scrollbar-thumb,.final-editor textarea::-webkit-scrollbar-thumb { min-height: 42px; border: 2px solid transparent; border-radius: 999px; background: linear-gradient(#dba8bc,#aaa4d4) padding-box; }
.publish-view::-webkit-scrollbar-thumb:hover,.channel-list::-webkit-scrollbar-thumb:hover,.artifact-editor textarea::-webkit-scrollbar-thumb:hover,.final-editor textarea::-webkit-scrollbar-thumb:hover { background: linear-gradient(#c77f9d,#8c88c2) padding-box; }
.spin { animation: spin .8s linear infinite; }@keyframes spin { to { transform: rotate(360deg); } }@keyframes stage-flow { from { background-position: 180% 0; } to { background-position: -80% 0; } }@keyframes stage-pulse { 0% { box-shadow: 0 0 0 0 rgba(var(--stage-rgb),.5); } 70%,100% { box-shadow: 0 0 0 9px rgba(var(--stage-rgb),0); } }
@property --electric-angle { syntax: '<angle>'; initial-value: 0deg; inherits: false; }@keyframes electric-border { to { --electric-angle: 360deg; } }@keyframes electric-icon { from { filter: drop-shadow(0 0 1px var(--agent-color)); transform: scale(.9); } to { filter: drop-shadow(0 0 7px var(--agent-color)); transform: scale(1.15); } }
@media (prefers-reduced-motion: reduce) { .stage-step.current .stage-line,.stage-step.current .stage-line i,.spin { animation: none; } }
@media (max-width: 1180px) { .auto-workspace { grid-template-columns: minmax(0,1fr) minmax(300px,.75fr); }.publish-sidebar { grid-column: 1/-1; display: grid; grid-template-columns: 1fr 1fr; }.result-panel { grid-column: 1/-1; } }
@media (max-width: 1050px) { .workspace-grid { grid-template-columns: 1fr; }.floating-manuscript { position: relative; top: auto; min-height: 620px; }.publish-sidebar { display: grid; grid-template-columns: 1fr 1fr; }.result-panel { grid-column: 1/-1; } }
@media (max-width: 760px) { .publish-view { padding: 12px 12px 76px; }.publish-command { padding: 22px 20px 0; }.command-copy { padding-right: 0; }.command-status { position: relative; top: auto; right: auto; width: auto; margin-bottom: 20px; }.command-metrics { grid-template-columns: 1fr 1fr; margin: 0 -20px; }.command-metrics > div { padding: 14px 20px; }.stage-track { grid-template-columns: 1fr 1fr; row-gap: 14px; }.workspace-grid { display: block; }.publish-sidebar { display: flex; margin-top: 14px; }.brief-options { grid-template-columns: 1fr; }.editor-actions { align-items: stretch; flex-direction: column; }.primary { margin-left: 0; }.action-hint { order: -1; }.editor-panel,.pipeline-card { padding: 18px; } }
</style>
