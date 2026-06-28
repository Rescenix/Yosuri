import type { NeuronData } from '../types'

export function parseStatsFull(text: string): NeuronData[] {
  const neurons: NeuronData[] = []
  
  // 移除开头的 "OK\n"，并保证格式统一
  text = text.replace(/^OK\n/, '').trim()
  
  // 用 '── ID:' 作为分隔，每个块一个记忆
  const blocks = text.split('── ID:').slice(1)
  
  for (const block of blocks) {
    const lines = block.split('\n').map(l => l.trim()).filter(l => l !== '')
    if (lines.length === 0) continue

    // ★ 关键修复：直接从 block 开头提取 ID
    const idMatch = block.match(/^\s*(\d+)/)
    const id = idMatch ? parseInt(idMatch[1]) : 0
    if (id === 0) continue

    let role = ''
    let content = ''
    let conductance = 0.5
    let emotion = '-'
    let intensity = 0
    let eventType = '-'
    let cluster = ''

    for (const line of lines) {
      // 跳过第一行（已经是 ID，已提取）
      if (line.startsWith('──') || /^\d+/.test(line)) continue
      
      if (line.startsWith('Role:')) {
        role = line.slice(5).trim()
      } else if (line.startsWith('Content:')) {
        content = line.slice(8).trim()
      } else if (line.startsWith('Energy:')) {
        const energyMatch = line.match(/Energy:\s*([\d.]+)/)
        if (energyMatch) conductance = parseFloat(energyMatch[1])

        const emotionMatch = line.match(/Emotion:\s*(\S+)/)
        if (emotionMatch) emotion = emotionMatch[1]

        const intensityMatch = line.match(/Intensity:\s*([\d.]+)/)
        if (intensityMatch) intensity = parseFloat(intensityMatch[1])

        const eventMatch = line.match(/EventType:\s*(\S+)/)
        if (eventMatch) eventType = eventMatch[1]

        const clusterMatch = line.match(/Cluster:\s*(\S+)/)
        if (clusterMatch) cluster = clusterMatch[1]
      }
    }

    neurons.push({ id, role, content, conductance, emotion, intensity, eventType, cluster })
  }
  
  return neurons
}