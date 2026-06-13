// src/composables/useWebHighlighter.js
import { ref } from 'vue'
import Highlighter from 'web-highlighter'

export function useWebHighlighter(containerRef) {
  const highlighter = ref(null)
  const annotationsData = ref([]) // 存储高亮元数据（含评论等）

  // 从 localStorage 加载已有数据
  function loadAnnotations() {
    const stored = localStorage.getItem('shanxi_annotations')
    if (stored) {
      annotationsData.value = JSON.parse(stored)
    }
  }

  function saveAnnotations() {
    localStorage.setItem('shanxi_annotations', JSON.stringify(annotationsData.value))
    window.dispatchEvent(new CustomEvent('annotations-updated'))
  }

  // 初始化高亮器
  function init(container) {
    if (highlighter.value) {
      highlighter.value.destroy()
    }

    highlighter.value = new Highlighter({
      style: {
        className: 'shanxi-highlight',
        cssText: `
          background-color: rgba(180, 80, 50, 0.25) !important;
          display: inline !important;
          line-height: inherit !important;
          font-size: inherit !important;
          font-family: inherit !important;
          font-weight: inherit !important;
          font-style: inherit !important;
          vertical-align: baseline !important;
          white-space: inherit !important;
          word-spacing: inherit !important;
          letter-spacing: inherit !important;
          text-indent: inherit !important;
          text-transform: inherit !important;
          padding: 0 !important;
          margin: 0 !important;
          border: none !important;
          outline: none !important;
          box-shadow: none !important;
          border-radius: 0 !important;
          width: auto !important;
          height: auto !important;
          float: none !important;
          clear: none !important;
          position: static !important;
          top: auto !important;
          left: auto !important;
        `
      }
    })

    highlighter.value.run(container)

    // 监听新增高亮，自动保存
    highlighter.value.on('highlighter:add', ({ id, serialized, text }) => {
      // 新高亮暂时没有评论，先保存基本数据
      annotationsData.value.push({
        id,
        text,
        serialized,
        comment: '',
        page: null, // 可在外部设置
        time: Date.now()
      })
      saveAnnotations()
    })

    // 监听删除高亮
    highlighter.value.on('highlighter:remove', ({ id }) => {
      annotationsData.value = annotationsData.value.filter(a => a.id !== id)
      saveAnnotations()
    })

    // 恢复已有高亮
    const storedData = annotationsData.value.map(({ serialized }) => serialized).filter(Boolean)
    if (storedData.length) {
      highlighter.value.fromStore(storedData)
    }
  }

  // 从选区创建高亮（返回高亮 id，供后续添加评论）
  async function createHighlightFromRange(range) {
    if (!highlighter.value) return null
    const highlightId = await highlighter.value.fromRange(range)
    return highlightId
  }

  // 为某个高亮添加评论
  function addCommentToHighlight(highlightId, commentText) {
    const anno = annotationsData.value.find(a => a.id === highlightId)
    if (anno) {
      anno.comment = commentText
      saveAnnotations()
    }
  }

  // 设置当前页码（用于存储，可选）
  function setCurrentPageForLatestHighlight(page) {
    // 找到最新添加的高亮（通常最后一个）
    const last = annotationsData.value[annotationsData.value.length - 1]
    if (last) last.page = page
    saveAnnotations()
  }

  // 翻页后恢复高亮（关键！）
  function restoreHighlights() {
    if (!highlighter.value) return
    const storedData = annotationsData.value.map(({ serialized }) => serialized).filter(Boolean)
    if (storedData.length) {
      highlighter.value.fromStore(storedData)
    }
  }

  // 清除所有高亮（销毁时用）
  function destroy() {
    if (highlighter.value) {
      highlighter.value.destroy()
      highlighter.value = null
    }
  }

  // 获取所有高亮数据（供外部使用，如展示列表）
  function getAnnotations() {
    return annotationsData.value
  }

  return {
    init,
    createHighlightFromRange,
    addCommentToHighlight,
    setCurrentPageForLatestHighlight,
    restoreHighlights,
    destroy,
    getAnnotations,
    loadAnnotations
  }
}