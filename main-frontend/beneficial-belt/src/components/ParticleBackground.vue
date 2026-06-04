<!-- src/components/ParticleBackground.vue -->
<template>
  <div id="tsparticles-container">
    <div id="tsparticles"></div>
  </div>
</template>

<script setup>
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { loadSlim } from 'tsparticles-slim'

const container = ref(null)
let themeSwitchHandler = null

const themes = {
  '#60a5fa': {
    // 《复刻回忆》- 取消拖尾，改为飘浮光点
    particles: {
      number: { value: 80, density: { enable: true, area: 800 } },
      color: { value: ['#1e3a5f', '#60a5fa', '#c09e4b'] },
      shape: { type: 'circle' },
      opacity: {
        value: 0.6,
        random: true,
        anim: { enable: true, speed: 0.3, opacity_min: 0.2 }
      },
      size: {
        value: 5,
        random: true,
        anim: { enable: true, speed: 1.5, size_min: 2 }
      },
      move: {
        enable: true,
        speed: 1.2,               // 提速减少停留感
        direction: 'none',
        random: true,
        straight: false,
        outModes: 'bounce'
        // trail 完全移除
      },
      rotate: {
        random: { enable: true, min: 0, max: 360 },
        animation: { enable: true, speed: 1.5, sync: false }
      }
    },
    interactivity: {
      events: { onClick: { enable: false }, onHover: { enable: true, mode: 'bubble' }, resize: true },
      modes: {
        bubble: { distance: 150, size: 10, duration: 2, opacity: 0.8, speed: 3 }
      }
    }
  },
  '#10b981': {
    // 《Bamboo》- 保持不变，无拖尾
    particles: {
      number: { value: 120, density: { enable: true, area: 800 } },
      color: { value: ['#047857', '#10b981', '#6ee7b7'] },
      shape: { type: 'circle' },
      opacity: { value: 0.9, random: true },
      size: { value: 2, random: true, anim: { enable: true, speed: 1, size_min: 0.5 } },
      move: { enable: true, speed: 2, direction: 'bottom', random: false, straight: true, outModes: 'out' }
    },
    interactivity: {
      events: { onClick: { enable: true, mode: 'push' }, onHover: { enable: true, mode: 'repulse' }, resize: true },
      modes: { push: { quantity: 4 }, repulse: { distance: 100 } }
    }
  },
  '#c084fc': {
    // 《A Spiral Moon》- 拖尾再缩短为3，速度微降
    particles: {
      number: { value: 150, density: { enable: true, area: 800 } },
      color: { value: ['#c084fc', '#e0e7ff', '#ffffff'] },
      shape: { type: 'circle' },
      opacity: {
        value: 0.7,
        random: true,
        anim: { enable: true, speed: 0.8, opacity_min: 0.1 }
      },
      size: {
        value: 4,
        random: true,
        anim: { enable: true, speed: 2, size_min: 1 }
      },
      links: { enable: false },
      move: {
        enable: true,
        speed: 2.5,               // 微降，让运动更优雅
        direction: 'top',
        random: true,
        straight: false,
        outModes: 'out',
        trail: { enable: true, length: 3, fillColor: '#0f172a' }  // 拖尾极短，若有似无
      }
    },
    interactivity: {
      events: { onClick: { enable: true, mode: 'push' }, onHover: { enable: true, mode: 'attract' }, resize: true },
      modes: { push: { quantity: 5 }, attract: { distance: 200, duration: 0.4, factor: 5 } }
    }
  }
}

const bgMap = {
  '#60a5fa': '#0a1628',
  '#10b981': '#0a1f14',
  '#c084fc': '#1a1028'
}

const applyTheme = async (color) => {
  console.log('🎨 粒子场景切换:', color)
  const theme = themes[color]
  if (!theme || !container.value) {
    console.warn('主题不存在或容器未初始化')
    return
  }
  
  try {
    document.documentElement.style.setProperty('--theme-color', color)
    document.documentElement.style.setProperty('--theme-bg', bgMap[color] || '#0f172a')
    
    container.value.options.particles = { ...theme.particles }
    container.value.options.interactivity = { ...theme.interactivity }
    await container.value.refresh()
    console.log('✅ 粒子场景切换成功')
  } catch (error) {
    console.error('粒子场景切换失败:', error)
  }
}

onMounted(async () => {
  const { tsParticles } = await import('tsparticles-engine')
  await loadSlim(tsParticles)

  container.value = await tsParticles.load('tsparticles', {
    ...themes['#60a5fa'],
    background: { color: { value: 'transparent' } },
    fpsLimit: 60,
    detectRetina: true
  })
  console.log('粒子容器初始化完成')

  themeSwitchHandler = (event) => {
    console.log('📡 粒子收到全局事件:', event.detail)
    applyTheme(event.detail)
  }
  window.addEventListener('theme-switch', themeSwitchHandler)
})

onBeforeUnmount(() => {
  if (themeSwitchHandler) {
    window.removeEventListener('theme-switch', themeSwitchHandler)
  }
  if (container.value) {
    container.value.destroy()
    container.value = null
  }
})
</script>

<style scoped>
#tsparticles-container {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 0;
  pointer-events: none;
}
#tsparticles {
  width: 100%;
  height: 100%;
}
</style>