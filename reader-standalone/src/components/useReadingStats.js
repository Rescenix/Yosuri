import { ref, onBeforeUnmount } from 'vue'

export function useReadingStats(textProvider, currentPage, totalPages, flipContainerRef) {
  const currentTime = ref('')
  let timeTimer = null

  // 滑动窗口：最近5页的阅读秒数
  const recentDurations = []
  const MAX_SAMPLES = 5
  let pageEnterTime = null

  const readingSpeed = ref(300)
  const remainingTime = ref(0)

  function updateCurrentTime() {
    const now = new Date()
    currentTime.value = now.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  }

  function markPageEnter() {
    pageEnterTime = Date.now()
  }

  function recordPageTurn() {
    if (pageEnterTime) {
      const duration = (Date.now() - pageEnterTime) / 1000
      if (duration > 0.5 && duration < 1800) {
        recentDurations.push(duration)
        if (recentDurations.length > MAX_SAMPLES) {
          recentDurations.shift()
        }
      }
    }
    markPageEnter()

    if (recentDurations.length > 0) {
      const avgSeconds = recentDurations.reduce((a, b) => a + b, 0) / recentDurations.length
      const fullText = textProvider()
      const totalPagesVal = totalPages.value || 1
      const avgCharsPerPage = fullText.length / totalPagesVal
      readingSpeed.value = Math.round((avgCharsPerPage / avgSeconds) * 60)
      const remainingPages = totalPagesVal - (currentPage.value || 0)
      remainingTime.value = Math.ceil((remainingPages * avgSeconds) / 60)
    } else {
      readingSpeed.value = 300
      const remainingPages = (totalPages.value || 1) - (currentPage.value || 0)
      remainingTime.value = Math.ceil(remainingPages / (300 / 100))
    }
  }

  function startClock() {
    updateCurrentTime()
    timeTimer = setInterval(updateCurrentTime, 60000)
  }

  function destroy() {
    clearInterval(timeTimer)
  }

  return {
    currentTime,
    readingSpeed,
    remainingTime,
    markPageEnter,
    recordPageTurn,
    startClock,
    destroy
  }
}