// ExactPaginator.js
// 最终方案：布局前拼接缩进，Pretext 精确测量，安全行数防止溢出

import { prepareWithSegments, layoutWithLines } from '@chenglou/pretext'

function flushPage(lines, fontSize) {
  const pageText = lines.join('\n')
  const escaped = pageText
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
  return `<div style="width:100%;height:100%;padding:24px;box-sizing:border-box;font-family:'Inter',system-ui,sans-serif;font-size:${fontSize}px;line-height:${fontSize * 1.8}px;white-space:pre-wrap;word-wrap:break-word;text-align:justify;overflow:hidden;">${escaped}</div>`
}

export async function exactPaginate(text, fontSize, pageWidth, pageHeight, onProgress) {
  const pad = 24
  const contentWidth = pageWidth - pad * 2
  const contentHeight = pageHeight - pad * 2
  const lineHeight = fontSize * 1.8
  // ★ 安全行数：多减去 3 行，防止任何溢出
  const maxLinesPerPage = Math.max(1, Math.floor(contentHeight / lineHeight) - 2)
  if (maxLinesPerPage < 1) return [flushPage([text], fontSize)]

  const font = `${fontSize}px 'Inter', system-ui, sans-serif`
  const paragraphs = text.split('\n')
  const totalParagraphs = paragraphs.length

  const allLines = []

  // 异步分片处理段落
  await new Promise(resolve => {
    const CHUNK_SIZE = 200
    let paraIndex = 0

    function processChunk() {
      const end = Math.min(paraIndex + CHUNK_SIZE, totalParagraphs)
      for (let i = paraIndex; i < end; i++) {
        const para = paragraphs[i]

        if (para === '') {
          allLines.push('')
          continue
        }

        const trimmed = para.trimStart()
        if (trimmed === '') {
          allLines.push('')
          continue
        }

        // 缩进拼接
        const indentedPara = '\u3000\u3000' + trimmed

        const prepared = prepareWithSegments(indentedPara, font, { whiteSpace: 'pre-wrap' })
        const { lines } = layoutWithLines(prepared, contentWidth, lineHeight)

        for (const line of lines) {
          allLines.push(line.text)
        }
      }

      paraIndex = end

      if (onProgress) {
        const pct = Math.floor((paraIndex / totalParagraphs) * 70)
        onProgress(pct)
      }

      if (paraIndex < totalParagraphs) {
        setTimeout(processChunk, 0)
      } else {
        resolve()
      }
    }

    processChunk()
  })

  if (onProgress) onProgress(70)

  // 分页
  const pages = []
  let currentPageLines = []
  const totalLines = allLines.length
  let processed = 0

  await new Promise(resolve => {
    const CHUNK = 1500
    function chunk() {
      const end = Math.min(processed + CHUNK, totalLines)
      for (let i = processed; i < end; i++) {
        if (currentPageLines.length >= maxLinesPerPage) {
          pages.push(flushPage(currentPageLines, fontSize))
          currentPageLines = []
        }
        currentPageLines.push(allLines[i])
      }
      processed = end
      if (onProgress) {
        const pct = 70 + Math.floor((processed / totalLines) * 30)
        onProgress(pct)
      }
      if (processed < totalLines) {
        setTimeout(chunk, 0)
      } else {
        if (currentPageLines.length > 0) pages.push(flushPage(currentPageLines, fontSize))
        resolve()
      }
    }
    chunk()
  })

  if (onProgress) onProgress(100)
  return pages
}