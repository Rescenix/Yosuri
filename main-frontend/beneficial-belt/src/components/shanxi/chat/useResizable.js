import { reactive, watch } from 'vue'

// 拖拽期间临时挂载 mousemove/mouseup，松手就摘掉——不用常驻 document 监听器，
// 避免每个使用方组件卸载时还要记得清理一份全局监听器。
function runDrag(onMove, onUp) {
  function handleMove(e) { onMove(e) }
  function handleUp(e) {
    document.removeEventListener('mousemove', handleMove)
    document.removeEventListener('mouseup', handleUp)
    document.body.style.userSelect = ''
    if (onUp) onUp(e)
  }
  document.body.style.userSelect = 'none'
  document.addEventListener('mousemove', handleMove)
  document.addEventListener('mouseup', handleUp)
}

export function useResizableWidth(widthRef, { min = 300, max = 720, edge = 'right', persistKey } = {}) {
  const clamp = (n) => Math.min(max, Math.max(min, n))

  if (persistKey) {
    const saved = parseFloat(localStorage.getItem(persistKey))
    if (!Number.isNaN(saved)) widthRef.value = clamp(saved)
  } else {
    widthRef.value = clamp(widthRef.value)
  }

  function startDrag(e) {
    e.preventDefault()
    const startX = e.clientX
    const startWidth = widthRef.value
    // edge:'right' 手柄在面板右边界（面板本身贴左）——手柄右拖=变宽；
    // edge:'left' 手柄在面板左边界（面板本身贴右）——手柄左拖=变宽，符号相反。
    const dir = edge === 'right' ? 1 : -1

    runDrag(
      (moveEvent) => {
        widthRef.value = clamp(startWidth + dir * (moveEvent.clientX - startX))
      },
      () => {
        if (persistKey) localStorage.setItem(persistKey, String(widthRef.value))
      }
    )
  }

  return { startDrag }
}

export function useResizableSplit(panelKeysRef, { min = 0.15 } = {}) {
  const ratios = reactive({})

  function reequalize() {
    const keys = panelKeysRef.value
    Object.keys(ratios).forEach((k) => delete ratios[k])
    if (keys.length === 0) return
    const equal = 1 / keys.length
    keys.forEach((k) => { ratios[k] = equal })
  }

  // 只在面板数量变化时重新等分，顺序变化或增删同数量面板时保留用户已经拖过的比例
  watch(panelKeysRef, (keys, oldKeys) => {
    if (!oldKeys || keys.length !== oldKeys.length) reequalize()
  }, { immediate: true })

  function startDragBetween(i, j, e) {
    e.preventDefault()
    const keys = panelKeysRef.value
    const keyI = keys[i]
    const keyJ = keys[j]
    const startY = e.clientY
    const startRatioI = ratios[keyI]
    const startRatioJ = ratios[keyJ]
    const pairTotal = startRatioI + startRatioJ
    const containerHeight = e.currentTarget.parentElement.getBoundingClientRect().height

    runDrag((moveEvent) => {
      const deltaRatio = (moveEvent.clientY - startY) / containerHeight
      const nextI = Math.max(min, Math.min(pairTotal - min, startRatioI + deltaRatio))
      ratios[keyI] = nextI
      ratios[keyJ] = pairTotal - nextI
    })
  }

  return { ratios, startDragBetween }
}
