import { computed } from 'vue'

export function useBookmarks(reader, currentPage, flipContainerRef, pageFlip) {
  const isCurrentPageBookmarked = computed(() => {
    return reader.isBookmarked?.(currentPage.value) ?? false
  })

  function getCurrentPageText() {
    if (!flipContainerRef.value) return ''
    const pages = flipContainerRef.value.querySelectorAll('.flip-page')
    const index = pageFlip?.getCurrentPageIndex()
    if (index >= 0 && index < pages.length) {
      const text = pages[index].textContent.trim()
      return text.slice(0, 30) + (text.length > 30 ? '...' : '')
    }
    return ''
  }

  function removeCurrentBookmark() {
    const page = currentPage.value
    if (page !== undefined) {
      reader.toggleBookmark(page, getCurrentPageText())
    }
  }

  return { isCurrentPageBookmarked, getCurrentPageText, removeCurrentBookmark }
}