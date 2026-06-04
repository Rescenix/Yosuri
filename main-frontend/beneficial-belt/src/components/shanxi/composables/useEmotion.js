import { ref } from 'vue'
import { emotionMap, defaultEmotion } from '../../../config/emotions'  // 修正路径

export function useEmotion() {
  const currentEmotion = ref({
    current: 'calm',
    color: '#f0a040',
    speed: 3.5,
    intensity: 1.0,
    glowColor: 'rgba(255, 140, 100, 0.4)'
  })

  function updateEmotion(emotionKey) {
    const emo = emotionMap[emotionKey] || defaultEmotion
    currentEmotion.value = { current: emotionKey, ...emo }
  }

  return { currentEmotion, updateEmotion }
}