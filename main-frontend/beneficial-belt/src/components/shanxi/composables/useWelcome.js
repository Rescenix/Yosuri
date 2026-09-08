import { ref } from 'vue'

// 欢迎语：后端 /api/memory/welcome 已随双轨记忆层重构删除（a81a0828），
// 前端调用只会吃 404，这里收敛为本地默认文案（2026-09-08 dogfood 实测）。
export function useWelcome() {
  const welcomeMessage = ref('你好！我是杉汐，你的数字伙伴。')
  const welcomeLoading = ref(false)
  return { welcomeMessage, welcomeLoading }
}
