import { layout, prepareWithSegments, walkLineRanges, type PreparedTextWithSegments } from '../../src/layout.ts'

export type WrapMetrics = {
  lineCount: number
  height: number
  maxLineWidth: number
}

export type PreparedBubble = {
  prepared: PreparedTextWithSegments
}

export type BubbleRenderWidths = {
  cssWidth: number
  tightWidth: number
}

export type BubbleRenderState = {
  chatWidth: number
  bubbleMaxWidth: number
  totalWastedPixels: number
  widths: BubbleRenderWidths[]
}

export const FONT = '15px "Helvetica Neue", Helvetica, Arial, sans-serif'
export const LINE_HEIGHT = 20
export const PADDING_H = 12
export const PADDING_V = 8
export const BUBBLE_MAX_RATIO = 0.8
export const PAGE_MAX_WIDTH = 1080
export const DESKTOP_PAGE_MARGIN = 32
export const MOBILE_PAGE_MARGIN = 20
export const GRID_GAP = 16
export const PANEL_PADDING_X = 36

export function prepareBubbleTexts(texts: string[]): PreparedBubble[] {
  return texts.map(text => ({
    prepared: prepareWithSegments(text, FONT),
  }))
}

export function getMaxChatWidth(minWidth: number, viewportWidth: number): number {
  const pageWidth = Math.min(PAGE_MAX_WIDTH, viewportWidth - (viewportWidth <= 760 ? MOBILE_PAGE_MARGIN : DESKTOP_PAGE_MARGIN))
  const columnWidth = viewportWidth <= 760 ? pageWidth : (pageWidth - GRID_GAP) / 2
  const panelContentWidth = Math.max(1, Math.floor(columnWidth - PANEL_PADDING_X))
  return Math.max(minWidth, panelContentWidth)
}

export function collectWrapMetrics(prepared: PreparedTextWithSegments, maxWidth: number): WrapMetrics {
  let maxLineWidth = 0
  const lineCount = walkLineRanges(prepared, maxWidth, line => {
    if (line.width > maxLineWidth) maxLineWidth = line.width
  })
  return {
    lineCount,
    height: lineCount * LINE_HEIGHT,
    maxLineWidth,
  }
}

export function findTightWrapMetrics(prepared: PreparedTextWithSegments, maxWidth: number): WrapMetrics {
  const initial = collectWrapMetrics(prepared, maxWidth)
  let lo = 1
  let hi = Math.max(1, Math.ceil(maxWidth))

  while (lo < hi) {
    const mid = Math.floor((lo + hi) / 2)
    const midLineCount = layout(prepared, mid, LINE_HEIGHT).lineCount
    if (midLineCount <= initial.lineCount) {
      hi = mid
    } else {
      lo = mid + 1
    }
  }

  return collectWrapMetrics(prepared, lo)
}

export function computeBubbleRender(preparedBubbles: PreparedBubble[], chatWidth: number): BubbleRenderState {
  const bubbleMaxWidth = Math.floor(chatWidth * BUBBLE_MAX_RATIO)
  const contentMaxWidth = bubbleMaxWidth - PADDING_H * 2
  let totalWastedPixels = 0
  const widths: BubbleRenderWidths[] = []

  for (let index = 0; index < preparedBubbles.length; index++) {
    const bubble = preparedBubbles[index]!
    const cssMetrics = collectWrapMetrics(bubble.prepared, contentMaxWidth)
    const tightMetrics = findTightWrapMetrics(bubble.prepared, contentMaxWidth)

    const cssWidth = Math.ceil(cssMetrics.maxLineWidth) + PADDING_H * 2
    const tightWidth = Math.ceil(tightMetrics.maxLineWidth) + PADDING_H * 2
    const cssHeight = cssMetrics.height + PADDING_V * 2
    totalWastedPixels += Math.max(0, cssWidth - tightWidth) * cssHeight
    widths.push({ cssWidth, tightWidth })
  }

  return {
    chatWidth,
    bubbleMaxWidth,
    totalWastedPixels,
    widths,
  }
}

export function formatPixelCount(value: number): string {
  return `${Math.round(value).toLocaleString()}`
}
