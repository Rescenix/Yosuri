// src/components/shanxi/EmotionEngine.ts

export type Emotion = 'calm' | 'happy' | 'thinking' | 'sad' | 'angry'

export interface EmotionState {
  current: Emotion
  color: string       // 主色调
  speed: number       // 动画周期（秒）
  intensity: number   // 缩放强度
  glowColor: string   // 光晕颜色
}

const emotionConfig: Record<Emotion, Omit<EmotionState, 'current'>> = {
  calm:     { color: '#f0a040', speed: 3.5, intensity: 1.0, glowColor: 'rgba(255, 140, 100, 0.4)' },
  happy:    { color: '#f5a623', speed: 2.2, intensity: 1.15, glowColor: 'rgba(255, 180, 50, 0.6)' },
  thinking: { color: '#a78bfa', speed: 2.8, intensity: 1.05, glowColor: 'rgba(167, 139, 250, 0.5)' },
  sad:      { color: '#60a5fa', speed: 5.0, intensity: 0.9, glowColor: 'rgba(96, 165, 250, 0.3)' },
  angry:    { color: '#ef4444', speed: 1.5, intensity: 1.3, glowColor: 'rgba(239, 68, 68, 0.7)' },
}

const keywordMap: [RegExp, Emotion][] = [
  [/谢谢|感谢|太棒|厉害|优秀|好棒/g, 'happy'],
  [/\?$|怎么|如何|为什么|想想|思考/g, 'thinking'],
  [/难过|伤心|失败|糟糕|不行/g, 'sad'],
  [/可恶|生气|愤怒|别烦/g, 'angry'],
]

export class EmotionEngine {
  private state: EmotionState
  private timer: ReturnType<typeof setTimeout> | null = null

  constructor(initial: Emotion = 'calm') {
    this.state = { current: initial, ...emotionConfig[initial] }
  }

  detectFromMessage(msg: string): EmotionState {
    for (const [regex, emotion] of keywordMap) {
      if (regex.test(msg)) {
        this.transition(emotion)
        return this.getState()
      }
    }
    this.transition('calm')
    return this.getState()
  }

  getState(): Readonly<EmotionState> {
    return { ...this.state }
  }

  private transition(target: Emotion) {
    if (this.timer) clearTimeout(this.timer)
    this.state.current = target
    Object.assign(this.state, emotionConfig[target])
    if (target !== 'calm') {
      this.timer = setTimeout(() => this.transition('calm'), 5000)
    }
  }
}