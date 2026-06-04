import { ref, onMounted, onBeforeUnmount } from 'vue'

export function useResponsiveSize(flipContainerRef) {
  const width = ref(550)
  const height = ref(700)

  function updateSize() {
    if (!flipContainerRef.value) return
    const wrapper = flipContainerRef.value.parentElement
    const maxWidth = wrapper?.clientWidth || window.innerWidth
    const maxHeight = wrapper?.clientHeight || window.innerHeight

    const ratio = 550 / 700
    let w = Math.floor(maxWidth * 0.92)
    let h = Math.floor(w / ratio)

    if (h > maxHeight * 0.9) {
      h = Math.floor(maxHeight * 0.9)
      w = Math.floor(h * ratio)
    }

    // 仅当变化超过阈值才更新，避免频繁重排
    if (Math.abs(w - width.value) > 30 || Math.abs(h - height.value) > 30) {
      width.value = w
      height.value = h
    }
  }

  let resizeObserver = null
  let resizeTimer = null

  function startObserving() {
    updateSize()
    // 监听窗口尺寸变化（防抖）
    const handleWindowResize = () => {
      clearTimeout(resizeTimer)
      resizeTimer = setTimeout(updateSize, 300)
    }
    window.addEventListener('resize', handleWindowResize)

    // 监听父容器尺寸变化（更精确）
    const wrapper = flipContainerRef.value?.parentElement
    if (wrapper) {
      resizeObserver = new ResizeObserver(() => {
        clearTimeout(resizeTimer)
        resizeTimer = setTimeout(updateSize, 200)
      })
      resizeObserver.observe(wrapper)
    }
  }

  function stopObserving() {
    window.removeEventListener('resize', updateSize)
    if (resizeObserver) {
      resizeObserver.disconnect()
      resizeObserver = null
    }
    clearTimeout(resizeTimer)
  }

  return { width, height, startObserving, stopObserving }
}