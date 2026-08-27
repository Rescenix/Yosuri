<template>
  <Teleport to="body">
    <!-- 右键悬浮卡片：输入框 → 剪切/复制/粘贴/全选；选中文本 → 复制/全选；代码块/终端 → 复制内容/全选 -->
    <div
      v-if="menu.show"
      ref="menuEl"
      class="ff-card ff-ctx"
      :style="{ left: menu.x + 'px', top: menu.y + 'px' }"
      @contextmenu.prevent
      @mousedown.stop
    >
      <template v-for="(it, i) in menu.items" :key="i">
        <div v-if="it.sep" class="ff-card-sep"></div>
        <button
          v-else
          class="ff-card-item"
          type="button"
          :disabled="it.disabled"
          @click="runAction(it)"
        >
          <span class="ff-item-label">{{ it.label }}</span>
          <kbd v-if="it.key" class="ff-item-key">{{ it.key }}</kbd>
        </button>
      </template>
    </div>

    <!-- 选中文本 → 悬浮复制按钮 -->
    <button
      v-if="sel.show"
      ref="selEl"
      class="ff-card ff-sel-tip"
      :class="{ copied: sel.copied }"
      type="button"
      :style="{ left: sel.x + 'px', top: sel.y + 'px' }"
      @mousedown.stop.prevent
      @click="copySelection"
    >
      <Icon v-if="!sel.copied" icon="mdi:content-copy" width="13" />
      <span>{{ sel.copied ? '✓ 已复制' : '复制' }}</span>
    </button>
  </Teleport>
</template>

<script setup>
import { reactive, ref, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { Icon } from '@iconify/vue'
import router from '../../../router.js'

const menu = reactive({ show: false, x: 0, y: 0, items: [], target: null, selText: '' })
const sel = reactive({ show: false, x: 0, y: 0, text: '', copied: false })
const menuEl = ref(null)
const selEl = ref(null)
let selTimer = null
// Chromium 右键会先把 input/textarea 里的选区坍缩成光标（实测 3-7 → 1-1），
// 所以在右键 mousedown（默认行为发生前）先缓存选区，动作执行时再恢复。
let lastEditableSel = null

const ED_FIELD = 'input, textarea, [contenteditable="true"], [contenteditable=""]'

function isEditableNode(n) {
  return !!(n && n.closest && n.closest(ED_FIELD))
}
function inMonaco(n) {
  return !!(n && n.closest && n.closest('.monaco-editor'))
}
function hasSelIn(el) {
  if (!el) return false
  if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') {
    return el.selectionStart != null && el.selectionEnd != null && el.selectionStart !== el.selectionEnd
  }
  if (el.isContentEditable) {
    const s = window.getSelection()
    return !!(s && !s.isCollapsed && el.contains(s.anchorNode))
  }
  return false
}

// ---------- 右键选区缓存（防 Chromium 右键坍缩选区） ----------
function captureEditableSel(el) {
  if (!el) return
  if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') {
    lastEditableSel = { el, start: el.selectionStart, end: el.selectionEnd }
  } else if (el.isContentEditable) {
    const s = window.getSelection()
    if (s && !s.isCollapsed && el.contains(s.anchorNode)) {
      lastEditableSel = { el, docSel: true, text: s.toString(), range: s.getRangeAt(0).cloneRange() }
    } else {
      lastEditableSel = null
    }
  } else {
    lastEditableSel = null
  }
}

function hasStoredSel(el) {
  if (!lastEditableSel || lastEditableSel.el !== el) return false
  if (lastEditableSel.docSel) return !!lastEditableSel.text
  return lastEditableSel.start != null && lastEditableSel.end != null && lastEditableSel.start !== lastEditableSel.end
}

function restoreStoredSel(el) {
  if (!lastEditableSel || lastEditableSel.el !== el) return
  if (lastEditableSel.docSel) {
    if (lastEditableSel.range) {
      const s = window.getSelection()
      s.removeAllRanges()
      s.addRange(lastEditableSel.range)
    }
  } else if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') {
    try { el.setSelectionRange(lastEditableSel.start, lastEditableSel.end) } catch (_) {}
  }
}

function execEditable(el, cmd) {
  if (!el) return
  el.focus({ preventScroll: true })
  restoreStoredSel(el)
  try { document.execCommand(cmd) } catch (_) {}
}

// ---------- 右键悬浮卡片 ----------
function onContextMenu(e) {
  const t = e.target
  if (inMonaco(t)) return // Monaco 编辑器有自己的右键菜单，不抢
  const editable = isEditableNode(t)
  const docSel = window.getSelection()
  const selText = docSel && !docSel.isCollapsed ? docSel.toString().trim() : ''

  if (editable) {
    e.preventDefault()
    const hasSel = hasSelIn(t) || hasStoredSel(t)
    menu.items = [
      { id: 'cut', label: '剪切', key: 'Ctrl+X', disabled: !hasSel },
      { id: 'copy', label: '复制', key: 'Ctrl+C', disabled: !hasSel },
      { id: 'paste', label: '粘贴', key: 'Ctrl+V' },
      { sep: true },
      { id: 'selectAll', label: '全选', key: 'Ctrl+A' }
    ]
    menu.target = t
    menu.selText = ''
    openMenu(e.clientX, e.clientY)
  } else if (selText) {
    e.preventDefault()
    menu.items = [
      { id: 'copySel', label: '复制', key: 'Ctrl+C' },
      { sep: true },
      { id: 'selectBlock', label: '全选', key: 'Ctrl+A' }
    ]
    menu.target = t
    menu.selText = selText
    openMenu(e.clientX, e.clientY)
  } else if (t && t.closest && t.closest('pre')) {
    // 代码块 / 终端输出：无选区时也能「复制内容」
    e.preventDefault()
    menu.items = [
      { id: 'copyPre', label: '复制内容', key: 'Ctrl+C' },
      { sep: true },
      { id: 'selectBlock', label: '全选', key: 'Ctrl+A' }
    ]
    menu.target = t.closest('pre')
    menu.selText = ''
    openMenu(e.clientX, e.clientY)
  }
  // 其余区域保持默认右键行为，不打扰已有自定义菜单
}

async function openMenu(x, y) {
  menu.show = false
  await nextTick()
  menu.x = x
  menu.y = y
  menu.show = true
  await nextTick()
  const el = menuEl.value
  if (!el) return
  const pad = 8
  menu.x = Math.max(pad, Math.min(x, window.innerWidth - el.offsetWidth - pad))
  menu.y = Math.max(pad, Math.min(y, window.innerHeight - el.offsetHeight - pad))
}

async function runAction(it) {
  const target = menu.target
  try {
    switch (it.id) {
      case 'cut': execEditable(target, 'cut'); break
      case 'copy': execEditable(target, 'copy'); break
      case 'paste': await doPaste(target); break
      case 'selectAll': selectAllIn(target); break
      case 'copySel': await writeClipboard(menu.selText); break
      case 'copyPre': await writeClipboard((target && target.innerText) || ''); break
      case 'selectBlock': selectNearestBlock(target); break
    }
  } catch (_) { /* 剪贴板被拒绝等情况静默 */ }
  lastEditableSel = null
  closeMenu()
}

async function doPaste(el) {
  if (!el) return
  // 先尝试读剪贴板图片（右键菜单粘贴 = Ctrl+V 一致）：剪贴板里是图就直接走
  // 附件流程，由 ChatWidget 监听 paste-image 事件接住。读不到图再退化成纯文本粘贴。
  try {
    if (navigator.clipboard && typeof navigator.clipboard.read === 'function') {
      const items = await navigator.clipboard.read()
      const imgItem = items.find(i => (i.types || []).some(t => t.startsWith('image/')))
      if (imgItem) {
        const type = imgItem.types.find(t => t.startsWith('image/')) || 'image/png'
        const blob = await imgItem.getType(type)
        const file = new File([blob], `paste-${Date.now()}.png`, { type })
        window.dispatchEvent(new CustomEvent('paste-image', { detail: { file } }))
        return
      }
    }
  } catch (_) { /* 剪贴板无权限等情况：继续走文本粘贴 */ }
  let text = ''
  try { text = await navigator.clipboard.readText() } catch (_) {}
  if (!text) {
    el.focus({ preventScroll: true })
    try { document.execCommand('paste'); return } catch (_) {}
    return
  }
  insertText(el, text)
}

function insertText(el, text) {
  if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') {
    const start = el.selectionStart ?? el.value.length
    const end = el.selectionEnd ?? el.value.length
    el.value = el.value.slice(0, start) + text + el.value.slice(end)
    const pos = start + text.length
    try { el.setSelectionRange(pos, pos) } catch (_) {}
    el.dispatchEvent(new Event('input', { bubbles: true }))
    el.focus({ preventScroll: true })
  } else if (el.isContentEditable) {
    el.focus({ preventScroll: true })
    document.execCommand('insertText', false, text)
  }
}

function selectAllIn(el) {
  if (!el) return
  if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') {
    el.focus({ preventScroll: true })
    el.select()
    return
  }
  selectNode(el)
}

function selectNearestBlock(t) {
  const block = t && t.closest
    ? t.closest('.message-row, .markdown-body, .assistant-message, .message-bubble, pre, article, .term-output')
    : null
  selectNode(block || t)
}

function selectNode(el) {
  if (!el) return
  const s = window.getSelection()
  const range = document.createRange()
  range.selectNodeContents(el)
  s.removeAllRanges()
  s.addRange(range)
}

async function writeClipboard(text) {
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    return
  } catch (_) {}
  // 兜底：隐藏 textarea + execCommand
  const ta = document.createElement('textarea')
  ta.value = text
  ta.style.cssText = 'position:fixed;left:-9999px;top:0;opacity:0'
  document.body.appendChild(ta)
  ta.select()
  try { document.execCommand('copy') } catch (_) {}
  document.body.removeChild(ta)
}

// ---------- 选中文本 → 悬浮复制按钮 ----------
function onMouseUp(e) {
  if (e.button !== 0) return
  const t = e.target
  if (t && t.closest && t.closest('.ff-card')) return
  clearTimeout(selTimer)
  selTimer = setTimeout(() => {
    const s = window.getSelection()
    const text = s && !s.isCollapsed ? s.toString().trim() : ''
    if (!text) { sel.show = false; return }
    const anchor = s.anchorNode
    if (isEditableNode(anchor) || inMonaco(anchor)) { sel.show = false; return }
    const range = s.rangeCount ? s.getRangeAt(0) : null
    if (!range) { sel.show = false; return }
    const rect = range.getBoundingClientRect()
    if (!rect || (rect.width === 0 && rect.height === 0)) { sel.show = false; return }
    positionSelTip(rect)
    sel.text = text
    sel.copied = false
    sel.show = true
  }, 30)
}

async function positionSelTip(rect) {
  await nextTick()
  const w = selEl.value ? selEl.value.offsetWidth : 92
  const h = selEl.value ? selEl.value.offsetHeight : 28
  let x = Math.max(8, Math.min(window.innerWidth - w - 8, rect.left + rect.width / 2 - w / 2))
  let y = rect.top - h - 8
  if (y < 8) y = rect.bottom + 8
  sel.x = x
  sel.y = y
}

async function copySelection() {
  try { await writeClipboard(sel.text) } catch (_) {}
  sel.copied = true
  clearTimeout(selTimer)
  selTimer = setTimeout(() => { sel.show = false }, 900)
}

// ---------- 关闭 ----------
function closeMenu() { menu.show = false }
function closeAll() { closeMenu(); sel.show = false }

function onMouseDown(e) {
  // 点在卡片本身上：不关菜单，也绝不清空已缓存的选区（动作执行要靠它恢复）
  if (e.target && e.target.closest && e.target.closest('.ff-card')) return
  // 右键 mousedown 时（浏览器默认「光标落点」发生前）缓存 input/textarea 里的选区，
  // 因为 Chromium 在 contextmenu 触发前就会把选区坍缩掉。
  if (e.button === 2 && isEditableNode(e.target)) {
    captureEditableSel(e.target.closest(ED_FIELD))
  } else {
    lastEditableSel = null
  }
  closeAll()
}

function onScroll(e) {
  if (e.target && e.target.closest && e.target.closest('.ff-card')) return
  closeAll()
}

function onKeyDown(e) {
  if (e.key === 'Escape') closeAll()
}

onMounted(() => {
  document.addEventListener('contextmenu', onContextMenu, true)
  document.addEventListener('mouseup', onMouseUp, true)
  document.addEventListener('mousedown', onMouseDown, true)
  window.addEventListener('scroll', onScroll, { capture: true, passive: true })
  document.addEventListener('keydown', onKeyDown)
  router.afterEach(closeAll)
})

onBeforeUnmount(() => {
  document.removeEventListener('contextmenu', onContextMenu, true)
  document.removeEventListener('mouseup', onMouseUp, true)
  document.removeEventListener('mousedown', onMouseDown, true)
  window.removeEventListener('scroll', onScroll, { capture: true })
  document.removeEventListener('keydown', onKeyDown)
  clearTimeout(selTimer)
})
</script>

<style scoped>
/* 悬浮卡片：白色圆角卡 + 浅阴影，跟随应用主题变量自动适配明暗 */
.ff-card {
  position: fixed;
  z-index: 2147483000;
  user-select: none;
  font-family: var(--app-font, 'Inter', system-ui, -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif);
}

.ff-ctx {
  min-width: 190px;
  padding: 5px;
  background: var(--app-surface, #ffffff);
  border: 1px solid var(--app-border, #e5e5e5);
  border-radius: 12px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.14), 0 2px 6px rgba(0, 0, 0, 0.08);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  animation: ff-pop 0.12s ease-out;
}

.ff-card-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  width: 100%;
  padding: 7px 10px;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--app-text, #1a1a1a);
  font-size: 13px;
  line-height: 1.3;
  cursor: pointer;
  text-align: left;
}

.ff-card-item:hover:not(:disabled) {
  background: var(--app-surface-3, #f4f4f5);
}

.ff-card-item:disabled {
  color: var(--app-text-faint, #a3a3a3);
  cursor: default;
}

.ff-item-key {
  font-family: inherit;
  font-size: 12px;
  color: var(--app-text-faint, #a3a3a3);
  background: transparent;
  border: none;
  padding: 0;
}

.ff-card-item:disabled .ff-item-key { color: var(--app-border, #e5e5e5); }

.ff-card-sep {
  height: 1px;
  margin: 4px 8px;
  background: var(--app-border-soft, #ececec);
}

/* 选中文本复制按钮：小胶囊，居中浮在选区上方 */
.ff-sel-tip {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 5px 12px;
  background: var(--app-surface, #ffffff);
  border: 1px solid var(--app-border, #e5e5e5);
  border-radius: 999px;
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.16);
  color: var(--app-text, #1a1a1a);
  font-size: 12.5px;
  cursor: pointer;
  animation: ff-pop 0.12s ease-out;
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
}

.ff-sel-tip:hover {
  border-color: var(--app-accent, #c96442);
  color: var(--app-accent, #c96442);
}

.ff-sel-tip.copied {
  color: #16a34a;
  border-color: rgba(22, 163, 74, 0.35);
}

@keyframes ff-pop {
  from { opacity: 0; transform: scale(0.96); }
  to { opacity: 1; transform: scale(1); }
}
</style>
