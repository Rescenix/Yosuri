import JSZip from 'jszip'

const EMU_PER_INCH = 914400

function childrenByLocalName(node, name) {
  return Array.from(node?.getElementsByTagNameNS('*', name) || [])
}

function first(node, name) {
  return childrenByLocalName(node, name)[0] || null
}

function parseBox(node) {
  const xfrm = first(node, 'xfrm')
  const off = first(xfrm, 'off')
  const ext = first(xfrm, 'ext')
  return {
    x: Number(off?.getAttribute('x') || 0),
    y: Number(off?.getAttribute('y') || 0),
    w: Number(ext?.getAttribute('cx') || 1),
    h: Number(ext?.getAttribute('cy') || 1),
  }
}

function mediaMime(path) {
  const ext = path.split('.').pop()?.toLowerCase()
  return ({ png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', gif: 'image/gif', svg: 'image/svg+xml', webp: 'image/webp' })[ext] || 'application/octet-stream'
}

function resolveTarget(base, target) {
  const parts = `${base}/${target}`.split('/')
  const output = []
  for (const part of parts) {
    if (!part || part === '.') continue
    if (part === '..') output.pop()
    else output.push(part)
  }
  return output.join('/')
}

export async function parsePptxPreview(buffer) {
  const zip = await JSZip.loadAsync(buffer)
  const parser = new DOMParser()
  const presentationText = await zip.file('ppt/presentation.xml')?.async('text')
  const presentation = presentationText ? parser.parseFromString(presentationText, 'application/xml') : null
  const sldSz = first(presentation, 'sldSz')
  const size = {
    w: Number(sldSz?.getAttribute('cx') || 13.333 * EMU_PER_INCH),
    h: Number(sldSz?.getAttribute('cy') || 7.5 * EMU_PER_INCH),
  }
  const slidePaths = Object.keys(zip.files)
    .filter(path => /^ppt\/slides\/slide\d+\.xml$/.test(path))
    .sort((a, b) => Number(a.match(/\d+/)?.[0]) - Number(b.match(/\d+/)?.[0]))

  const slides = []
  for (const path of slidePaths) {
    const xml = parser.parseFromString(await zip.file(path).async('text'), 'application/xml')
    const relPath = path.replace('ppt/slides/', 'ppt/slides/_rels/') + '.rels'
    const relFile = zip.file(relPath)
    const relationships = new Map()
    if (relFile) {
      const relXml = parser.parseFromString(await relFile.async('text'), 'application/xml')
      for (const rel of childrenByLocalName(relXml, 'Relationship')) {
        relationships.set(rel.getAttribute('Id'), resolveTarget('ppt/slides', rel.getAttribute('Target') || ''))
      }
    }
    const tree = first(xml, 'spTree')
    const elements = []
    for (const node of Array.from(tree?.children || [])) {
      const local = node.localName
      const box = parseBox(node)
      if (local === 'sp') {
        const text = childrenByLocalName(node, 't').map(item => item.textContent || '').join('\n').trim()
        if (!text) continue
        const color = first(first(node, 'solidFill'), 'srgbClr')?.getAttribute('val') || '172033'
        const fontSize = Number(first(node, 'rPr')?.getAttribute('sz') || first(node, 'defRPr')?.getAttribute('sz') || 1800) / 100
        elements.push({ type: 'text', text, color: `#${color}`, fontSize, ...box })
      } else if (local === 'pic') {
        const blip = first(node, 'blip')
        const relId = blip?.getAttributeNS('http://schemas.openxmlformats.org/officeDocument/2006/relationships', 'embed') || blip?.getAttribute('r:embed')
        const mediaPath = relationships.get(relId)
        const media = mediaPath && zip.file(mediaPath)
        if (!media) continue
        const data = await media.async('base64')
        elements.push({ type: 'image', src: `data:${mediaMime(mediaPath)};base64,${data}`, ...box })
      }
    }
    slides.push({ elements })
  }
  return { size, slides }
}

export function pptxElementStyle(element, size) {
  const percent = (value, total) => `${(value / total) * 100}%`
  return {
    left: percent(element.x, size.w), top: percent(element.y, size.h),
    width: percent(element.w, size.w), height: percent(element.h, size.h),
    color: element.color, fontSize: `${Math.max(10, element.fontSize || 18)}px`,
  }
}
