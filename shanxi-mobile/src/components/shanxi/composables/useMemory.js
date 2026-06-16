import { shouldSave } from '../../../utils/memoryFilter'

export function useMemory() {
  function saveMemory(role, content) {
    console.log('📝 saveMemory 被调用:', role, content?.substring(0, 30))

    if (!shouldSave(content)) {
      console.log('⏭ 被 shouldSave 过滤，跳过存储')
      return
    }

    const token = localStorage.getItem('token')
    if (!token) {
      console.log('⏭ 未登录，跳过记忆存储')
      return
    }

    const headers = { 'Content-Type': 'application/json' }
    headers['Authorization'] = `Bearer ${token}`

    fetch('/api/memory/save', {
      method: 'POST',
      headers,
      body: JSON.stringify({ role, content })
    }).then(res => {
      if (!res.ok) console.warn('记忆存档失败:', res.status)
      else console.log('✅ 记忆已存档')
    }).catch(err => console.error('记忆存档失败:', err))
  }

  return { saveMemory }
}