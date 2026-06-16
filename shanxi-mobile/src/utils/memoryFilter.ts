export function shouldSave(content: string): boolean {
  const fillerWords = new Set([
    '你好', '再见', '谢谢', '哈哈', '嗯', '哦', '好的', 'ok', 'hi', 'hello', 'bye',
    '早', '晚安', '再见啦', '拜拜', '好', '对', '是', '行', '可以'
  ])
  const emoticonPattern = /^[（(][\u4e00-\u9fa5a-zA-Z]{1,4}[）)]$/
  const MIN_DENSITY = 0.3
  const MIN_UTF_LENGTH = 10

  const trimmed = content.trim()
  if (trimmed.length === 0) return false
  if (fillerWords.has(trimmed.toLowerCase())) return false
  if (emoticonPattern.test(trimmed)) return false
  const chineseChars = trimmed.match(/[\u4e00-\u9fa5]/g)
  if (!chineseChars) return false
  const density = chineseChars.length / trimmed.length
  if (trimmed.length < MIN_UTF_LENGTH && density < MIN_DENSITY) return false
  return true
}