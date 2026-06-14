// 情绪总控文件 - 新增情绪只需在这里加一行

export interface EmotionConfig {
  /** 情绪标识（对应后端 Prompt 中的标签） */
  key: string
  /** 显示名称（可留空，用于调试） */
  label?: string
  /** 头像文件名（不含路径和扩展名） */
  avatar: string
  /** 光晕颜色（RGB/RGBA） */
  glowColor: string
  /** 动画速度（秒，心跳周期） */
  speed: number
  /** 缩放强度（心跳幅度） */
  intensity: number
}

export const emotions: EmotionConfig[] = [
  { key: 'calm',     label: '平静', avatar: 'calm',     glowColor: 'rgba(240, 160, 100, 0.9)', speed: 3.5, intensity: 1.0 },
  { key: 'happy',    label: '开心', avatar: 'happy',    glowColor: 'rgba(255, 180, 50, 0.9)',  speed: 2.2, intensity: 1.15 },
  { key: 'thinking', label: '思考', avatar: 'thinking', glowColor: 'rgba(167, 139, 250, 0.9)', speed: 2.8, intensity: 1.05 },
  { key: 'sad',      label: '伤心', avatar: 'sad',      glowColor: 'rgba(96, 165, 250, 0.9)',  speed: 5.0, intensity: 0.9 },
  { key: 'angry',    label: '生气', avatar: 'angry',    glowColor: 'rgba(239, 68, 68, 0.9)',   speed: 1.5, intensity: 1.3 },
  { key: 'loving',   label: '爱意', avatar: 'loving',   glowColor: 'rgba(244, 114, 182, 0.9)', speed: 1.0, intensity: 1.35 },
  // ===== 在这里新增情绪 =====
  // { key: 'surprised', label: '惊讶', avatar: 'surprised', glowColor: 'rgba(255, 220, 100, 0.9)', speed: 1.8, intensity: 1.25 },
  // { key: 'shy',       label: '害羞', avatar: 'shy',       glowColor: 'rgba(255, 150, 150, 0.9)', speed: 2.0, intensity: 1.1 },
  // { key: 'serious',   label: '严肃', avatar: 'serious',   glowColor: 'rgba(180, 180, 220, 0.9)', speed: 3.0, intensity: 0.95 },
]

// 快速查找表（由上面的数组自动生成，无需手动维护）
export const emotionMap: Record<string, EmotionConfig> = {}
emotions.forEach(e => { emotionMap[e.key] = e })

// 默认情绪
export const defaultEmotion = emotionMap['calm']

// 生成后端 Prompt 中情绪标签的枚举
export const emotionLabelList = emotions.map(e => `${e.key}（${e.label}）`).join('、')