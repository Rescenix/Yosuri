import JSZip from 'jszip'

function textNodes(node) {
  return Array.from(node?.getElementsByTagNameNS('*', 't') || []).map(item => item.textContent || '').join('')
}

function columnIndex(reference) {
  const letters = String(reference || '').match(/^[A-Z]+/)?.[0] || 'A'
  let index = 0
  for (const letter of letters) index = index * 26 + letter.charCodeAt(0) - 64
  return index - 1
}

export async function parseXlsxPreview(buffer, maxRows = 40, maxCols = 14) {
  const zip = await JSZip.loadAsync(buffer)
  const parser = new DOMParser()
  const shared = []
  const sharedFile = zip.file('xl/sharedStrings.xml')
  if (sharedFile) {
    const xml = parser.parseFromString(await sharedFile.async('text'), 'application/xml')
    for (const item of Array.from(xml.getElementsByTagNameNS('*', 'si'))) shared.push(textNodes(item))
  }
  const sheetPath = Object.keys(zip.files).filter(path => /^xl\/worksheets\/sheet\d+\.xml$/.test(path)).sort()[0]
  if (!sheetPath) return { rows: [], truncated: false }
  const sheet = parser.parseFromString(await zip.file(sheetPath).async('text'), 'application/xml')
  const rows = []
  const xmlRows = Array.from(sheet.getElementsByTagNameNS('*', 'row')).slice(0, maxRows)
  for (const row of xmlRows) {
    const values = Array(maxCols).fill('')
    for (const cell of Array.from(row.getElementsByTagNameNS('*', 'c'))) {
      const index = columnIndex(cell.getAttribute('r'))
      if (index >= maxCols) continue
      const type = cell.getAttribute('t')
      const value = cell.getElementsByTagNameNS('*', 'v')[0]?.textContent || ''
      const formula = cell.getElementsByTagNameNS('*', 'f')[0]?.textContent || ''
      if (type === 's') values[index] = shared[Number(value)] ?? value
      else if (type === 'inlineStr') values[index] = textNodes(cell)
      else if (type === 'b') values[index] = value === '1' ? 'TRUE' : 'FALSE'
      else values[index] = formula ? `=${formula} → ${value}` : value
    }
    while (values.length && values[values.length - 1] === '') values.pop()
    rows.push(values)
  }
  return { rows, truncated: sheet.getElementsByTagNameNS('*', 'row').length > maxRows }
}

export function parseDelimitedPreview(text, delimiter = ',', maxRows = 40, maxCols = 14) {
  const rows = String(text || '').split(/\r?\n/).filter(Boolean).slice(0, maxRows).map(line => line.split(delimiter).slice(0, maxCols))
  return { rows, truncated: String(text || '').split(/\r?\n/).length > maxRows }
}
