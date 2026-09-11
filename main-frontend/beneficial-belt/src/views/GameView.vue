<template>
  <div class="game-page">
    <div class="game-topbar">
      <button class="back-btn" @click="goBack">
        <Icon icon="mdi:arrow-left" /> {{ tr('返回') }}
      </button>
      <span class="game-title">{{ tr('星迹 · Star Trail') }}</span>
      <span class="game-tip">{{ tr('云端经济 · 独立部署') }}</span>
    </div>
    <!-- 游戏前端独立构建部署（不进入 re0 开源仓库），re0 只提供入口 -->
    <iframe
      :src="gameUrl"
      class="game-frame"
      allow="fullscreen"
      referrerpolicy="no-referrer"
    ></iframe>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
import { Icon } from '@iconify/vue'
import { useI18n, tr } from '../composables/useI18n.js'



const router = useRouter()
function goBack() { router.push('/chat') }

// 游戏 dist 独立部署在官网（ResceneSite/cloudflare），代码不进 re0 仓库
const gameUrl = 'https://yosuri.com/game/'
</script>

<style scoped>
.game-page {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #F5F8FF;
}
.game-topbar {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 8px 14px;
  border-bottom: 1px solid #d6e4ff;
  background: #fff;
  flex-shrink: 0;
}
.back-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  background: none;
  border: 1px solid #1950BE;
  color: #1950BE;
  border-radius: 8px;
  padding: 4px 12px;
  font-size: 13px;
  cursor: pointer;
}
.game-title { font-size: 15px; color: #1a2b4c; font-weight: 600; }
.game-tip { margin-left: auto; font-size: 12px; color: #6b7a99; }
.game-frame {
  flex: 1;
  width: 100%;
  border: none;
}
</style>