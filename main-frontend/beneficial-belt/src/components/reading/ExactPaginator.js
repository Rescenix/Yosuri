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
  const maxLinesPerPage = Math.max(1, Math.floor(contentHeight / lineHeight))
  if (maxLinesPerPage < 1) return [flushPage([text], fontSize)]

  const font = `${fontSize}px 'Inter', system-ui, sans-serif`
  const paragraphs = text.split('\n')
  const totalParagraphs = paragraphs.length

  const allLines = []
  let prevLineWasEmpty = false
  let prevParaWasContent = false // 上一个段落是否有内容

  // 异步分片处理段落，插入段落空行，压缩连续空行
  await new Promise(resolve => {
    const CHUNK_SIZE = 200
    let paraIndex = 0

    function processChunk() {
      const end = Math.min(paraIndex + CHUNK_SIZE, totalParagraphs)
      for (let i = paraIndex; i < end; i++) {
        const para = paragraphs[i]

        // 完全空段落（空字符串）
        if (para === '') {
          if (!prevLineWasEmpty) {
            allLines.push('')
            prevLineWasEmpty = true
          }
          prevParaWasContent = false
          continue
        }

        // 只有空白字符的段落
        const trimmed = para.trimStart()
        if (trimmed === '') {
          if (!prevLineWasEmpty) {
            allLines.push('')
            prevLineWasEmpty = true
          }
          prevParaWasContent = false
          continue
        }

        // 这是一个有内容的段落
        // 如果上一个段落有内容，且当前中间没有空行，则插入一个空行作为分隔
        if (prevParaWasContent && !prevLineWasEmpty) {
          allLines.push('')
          prevLineWasEmpty = true
        }

        prevLineWasEmpty = false
        prevParaWasContent = true
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

  // 移除全文开头的所有空行
  while (allLines.length > 0 && allLines[0] === '') {
    allLines.shift()
  }

  if (onProgress) onProgress(70)

  // 分页，确保每页首行顶格、尾行贴底，行数严格一致
  const pages = []
  let currentPageLines = []
  const totalLines = allLines.length
  let processed = 0

  function adjustPage(lines) {
    // 1. 去掉页首空行
    while (lines.length > 0 && lines[0] === '') {
      lines.shift()
    }

    // 2. 处理页末空行：保留一个空行（段落间距），多余空行移到内部
    let trailingEmpties = 0
    for (let i = lines.length - 1; i >= 0 && lines[i] === ''; i--) {
      trailingEmpties++
    }
    if (trailingEmpties > 1) {
      const contentEnd = lines.length - trailingEmpties
      const movedCount = trailingEmpties - 1   // 保留一个
      const moved = lines.splice(contentEnd + 1, movedCount)
      let insertPos = -1
      for (let i = 0; i < lines.length; i++) {
        if (lines[i] === '') {
          insertPos = i
          break
        }
      }
      if (insertPos !== -1) {
        lines.splice(insertPos + 1, 0, ...moved)
      }
      // 若无内部空行，多余空行直接丢弃
    }

    // 3. 填充不足的行数：将空行均匀分散到段落之间
    let needed = maxLinesPerPage - lines.length
    if (needed > 0) {
      const getEmptyIndices = () => {
        const indices = []
        for (let i = 0; i < lines.length; i++) {
          if (lines[i] === '') indices.push(i)
        }
        return indices
      }

      let emptyIndices = getEmptyIndices()
      if (emptyIndices.length === 0) {
        while (lines.length < maxLinesPerPage) lines.push('')
      } else {
        let idx = 0
        while (needed > 0) {
          const pos = emptyIndices[idx % emptyIndices.length]
          lines.splice(pos + 1, 0, '')
          needed--
          emptyIndices = getEmptyIndices()
          idx++
        }
      }
    } else if (needed < 0) {
      lines = lines.slice(0, maxLinesPerPage)
    }

    return lines
  }

  await new Promise(resolve => {
    const CHUNK = 1500
    function chunk() {
      const end = Math.min(processed + CHUNK, totalLines)
      for (let i = processed; i < end; i++) {
        const line = allLines[i]
        if (line === '' && currentPageLines.length === 0) {
          processed++
          continue
        }
        if (currentPageLines.length >= maxLinesPerPage) {
          currentPageLines = adjustPage(currentPageLines)
          pages.push(flushPage(currentPageLines, fontSize))
          currentPageLines = []
        }
        currentPageLines.push(line)
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