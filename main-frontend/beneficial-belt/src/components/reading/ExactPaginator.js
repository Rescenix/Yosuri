// ExactPaginator.js
import { prepareWithSegments, layoutWithLines } from '@chenglou/pretext'

function flushPage(lines, fontSize) {
  const pageText = lines.join('\n')
  const escaped = pageText
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
  return `<div style="width:100%;height:100%;padding:24px;box-sizing:border-box;font-family:'Inter',system-ui,sans-serif;font-size:${fontSize}px;line-height:${fontSize * 1.8}px;white-space:pre-wrap;word-wrap:break-word;text-align:left;overflow:hidden;">${escaped}</div>`
}

export async function exactPaginate(text, fontSize, pageWidth, pageHeight, onProgress) {
  const pad = 24
  const contentWidth = pageWidth - pad * 2
  const contentHeight = pageHeight - pad * 2
  const lineHeight = fontSize * 1.8
  const maxLinesPerPage = Math.max(1, Math.floor(contentHeight / lineHeight) - 2)
  if (maxLinesPerPage < 1) return [flushPage([text], fontSize)]

  const font = `${fontSize}px 'Inter', system-ui, sans-serif`
  const paragraphs = text.split('\n')
  const totalParagraphs = paragraphs.length

  const allLines = []

  // 异步分片处理段落，并压缩连续空行为一行
  await new Promise(resolve => {
    const CHUNK_SIZE = 200
    let paraIndex = 0
    let prevLineWasEmpty = false

    function processChunk() {
      const end = Math.min(paraIndex + CHUNK_SIZE, totalParagraphs)
      for (let i = paraIndex; i < end; i++) {
        const para = paragraphs[i]

        if (para === '') {
          if (!prevLineWasEmpty) {
            allLines.push('')
            prevLineWasEmpty = true
          }
          continue
        }

        const trimmed = para.trimStart()
        if (trimmed === '') {
          if (!prevLineWasEmpty) {
            allLines.push('')
            prevLineWasEmpty = true
          }
          continue
        }

        prevLineWasEmpty = false
        const indentedPara = '\u3000\u3000' + trimmed

        const prepared = prepareWithSegments(indentedPara, font, { whiteSpace: 'pre-wrap' })
        let { lines } = layoutWithLines(prepared, contentWidth, lineHeight)

        // 修复行尾丢字
        const allLineText = lines.map(l => l.text).join('')
        if (allLineText !== indentedPara) {
          const missing = indentedPara.slice(allLineText.length)
          if (lines.length > 0) {
            lines[lines.length - 1].text += missing
          } else {
            lines.push({ text: missing })
          }
        }

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

  // 分页，带页首清理和页末空行上移（后续内容自动下移）
  const pages = []
  let currentPageLines = []
  const totalLines = allLines.length
  let processed = 0

  function adjustPage(lines) {
    // 1. 去掉开头的空行
    while (lines.length > 0 && lines[0] === '') {
      lines.shift()
    }
    // 2. 页末空行上移至上一个空行处（后续行自动下移）
    let trailingEmpties = 0
    for (let i = lines.length - 1; i >= 0 && lines[i] === ''; i--) {
      trailingEmpties++
    }
    if (trailingEmpties > 0) {
      const contentEnd = lines.length - trailingEmpties
      let insertPos = -1
      for (let i = contentEnd - 1; i >= 0; i--) {
        if (lines[i] === '') {
          insertPos = i
          break
        }
      }
      if (insertPos !== -1) {
        const moved = lines.splice(contentEnd, trailingEmpties)
        // 插入到找到的空行之后，后续行自动下移
        lines.splice(insertPos + 1, 0, ...moved)
      } else {
        // 无内部空行，直接丢弃末尾空行
        lines.splice(contentEnd, trailingEmpties)
      }
    }
    return lines
  }

  await new Promise(resolve => {
    const CHUNK = 1500
    function chunk() {
      const end = Math.min(processed + CHUNK, totalLines)
      for (let i = processed; i < end; i++) {
        if (currentPageLines.length >= maxLinesPerPage) {
          currentPageLines = adjustPage(currentPageLines)
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
        if (currentPageLines.length > 0) {
          currentPageLines = adjustPage(currentPageLines)
          pages.push(flushPage(currentPageLines, fontSize))
        }
        resolve()
      }
    }
    chunk()
  })

  if (onProgress) onProgress(100)
  return pages
}