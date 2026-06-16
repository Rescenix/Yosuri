import {
  computeBubbleRender,
  formatPixelCount,
  getMaxChatWidth,
  prepareBubbleTexts,
} from './bubbles-shared.ts'

type State = {
  requestedChatWidth: number
  events: {
    sliderValue: number | null
  }
}

const domCache = {
  root: document.documentElement,
  chatShrink: getRequiredDiv('chat-shrink'),
  slider: getRequiredInput('slider'),
  valLabel: getRequiredSpan('val'),
  cssWaste: getRequiredSpan('css-waste'),
  shrinkWaste: getRequiredSpan('shrink-waste'),
}

const shrinkNodes = getChatMessageNodes(domCache.chatShrink)
const preparedBubbles = prepareBubbleTexts(shrinkNodes.map(readNodeText))
const st: State = {
  requestedChatWidth: getInitialChatWidth(),
  events: {
    sliderValue: null,
  },
}
let scheduledRaf: number | null = null

domCache.slider.addEventListener('input', () => {
  st.events.sliderValue = Number.parseInt(domCache.slider.value, 10)
  scheduleRender()
})

window.addEventListener('resize', () => {
  scheduleRender()
})

document.fonts.ready.then(() => {
  scheduleRender()
})

scheduleRender()

function getRequiredDiv(id: string): HTMLDivElement {
  const element = document.getElementById(id)
  if (!(element instanceof HTMLDivElement)) throw new Error(`#${id} not found`)
  return element
}

function getRequiredInput(id: string): HTMLInputElement {
  const element = document.getElementById(id)
  if (!(element instanceof HTMLInputElement)) throw new Error(`#${id} not found`)
  return element
}

function getRequiredSpan(id: string): HTMLSpanElement {
  const element = document.getElementById(id)
  if (!(element instanceof HTMLSpanElement)) throw new Error(`#${id} not found`)
  return element
}

function getInitialChatWidth(): number {
  const datasetValue = domCache.root.dataset['bubblesChatWidth']
  const parsed = datasetValue === undefined ? Number.NaN : Number.parseInt(datasetValue, 10)
  if (Number.isFinite(parsed)) return parsed
  return Number.parseInt(domCache.slider.value, 10)
}

function getChatMessageNodes(chat: HTMLDivElement): HTMLDivElement[] {
  return Array.from(chat.querySelectorAll<HTMLDivElement>('.msg'))
}

function readNodeText(node: HTMLDivElement): string {
  return node.textContent ?? ''
}

function scheduleRender(): void {
  if (scheduledRaf !== null) return
  scheduledRaf = requestAnimationFrame(function renderBubblesFrame() {
    scheduledRaf = null
    render()
  })
}

function render(): void {
  const minWidth = Number.parseInt(domCache.slider.min, 10)
  let requestedChatWidth = st.requestedChatWidth
  if (st.events.sliderValue !== null) requestedChatWidth = st.events.sliderValue
  const maxWidth = getMaxChatWidth(minWidth, document.documentElement.clientWidth)
  const chatWidth = Math.min(requestedChatWidth, maxWidth)

  st.requestedChatWidth = requestedChatWidth
  st.events.sliderValue = null

  domCache.slider.max = String(maxWidth)
  domCache.slider.value = String(chatWidth)
  domCache.valLabel.textContent = `${chatWidth}px`
  updateBubbles(chatWidth)
}

function updateBubbles(chatWidth: number): void {
  const renderState = computeBubbleRender(preparedBubbles, chatWidth)
  domCache.root.style.setProperty('--chat-width', `${renderState.chatWidth}px`)
  domCache.root.style.setProperty('--bubble-max-width', `${renderState.bubbleMaxWidth}px`)

  for (let index = 0; index < shrinkNodes.length; index++) {
    const shrinkNode = shrinkNodes[index]!
    const widths = renderState.widths[index]!

    shrinkNode.style.maxWidth = `${renderState.bubbleMaxWidth}px`
    shrinkNode.style.width = `${widths.tightWidth}px`
  }

  domCache.cssWaste.textContent = formatPixelCount(renderState.totalWastedPixels)
  domCache.shrinkWaste.textContent = '0'
}
