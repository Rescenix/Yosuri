export function createPageElements(htmlPages, width, height) {
  return htmlPages.map(html => {
    const div = document.createElement('div')
    div.className = 'flip-page'
    div.style.width = width + 'px'
    div.style.height = height + 'px'
    div.innerHTML = html
    return div
  })
}