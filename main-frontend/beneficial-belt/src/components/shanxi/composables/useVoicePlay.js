import { ref } from 'vue'

export function useVoicePlay() {
  const isVoicePlaying = ref(false)

  async function playVoice(text) {
    if (isVoicePlaying.value) return
    isVoicePlaying.value = true

    try {
      const res = await fetch('/api/tts', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text })
      })
      if (res.ok) {
        const blob = await res.blob()
        const url = URL.createObjectURL(blob)
        const audio = new Audio(url)
        audio.onended = () => {
          isVoicePlaying.value = false
          URL.revokeObjectURL(url)
        }
        audio.play()
      }
    } catch (e) {
      isVoicePlaying.value = false
    }
  }

  return { playVoice }
}