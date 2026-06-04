<template>
  <div class="cards-grid">
    <!-- 项目一：多模态 RAG 知识库 -->
    <div class="tool-card" @click="goRag">
      <div class="card-icon"><Icon icon="ph:database-duotone" width="32" color="var(--primary)" /></div>
      <div class="card-content">
        <h3>多模态 RAG 知识库</h3>
        <p>混合检索（向量+关键词）+ 长期记忆，支持文档问答与智能召回。</p>
      </div>
      <button class="card-btn" @click.stop="goRag">了解</button>
    </div>

    <!-- 项目二：全栈脚手架 CLI -->
    <div class="tool-card" @click="goCli">
      <div class="card-icon"><Icon icon="ph:terminal-duotone" width="32" color="var(--primary)" /></div>
      <div class="card-content">
        <h3>全栈脚手架 CLI</h3>
        <p>一键生成前后端一体化项目骨架，集成 Gin + Vue3 模板与常用中间件。</p>
      </div>
      <button class="card-btn" @click.stop="goCli">了解</button>
    </div>

    <!-- 项目三：Prism 自研数据引擎 -->
    <div class="tool-card" @click="goPrism">
      <div class="card-icon"><Icon icon="ph:cube-transparent-duotone" width="32" color="var(--primary)" /></div>
      <div class="card-content">
        <h3>Prism 数据引擎</h3>
        <p>嵌入式混合存储引擎，支持 KV、文档、向量模型，替代 JSON 文件，性能提升百倍。</p>
      </div>
      <button class="card-btn" @click.stop="goPrism">了解</button>
    </div>

    <!-- 阅读小屋（原样保留，放在最后） -->
    <div class="tool-card" @click="goReading">
      <div class="card-icon"><Icon icon="ph:books-duotone" width="32" color="var(--primary)" /></div>
      <div class="card-content">
        <h3>阅读小屋</h3>
        <p>上传一本书，杉汐陪你一起读，随时讨论书中内容，还能为你朗读。</p>
      </div>
      <button class="card-btn" @click.stop="goReading">开启</button>
    </div>


  </div>
</template>

<script setup>
import { Icon } from '@iconify/vue';

const STORAGE_KEY = 'recent_hut';

function saveRecent(title, link, icon) {
  const data = {
    title,
    link,
    icon,
    timestamp: Date.now(),
  };
  localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
  console.log('[HutCards] 已写入最近使用:', title);
  // 可选：同时派发同页面事件，便于当前页面内的 RecentHutCard 立即更新（如果有）
  window.dispatchEvent(new CustomEvent('recent-hut-update', { detail: data }));
}

const goRag = () => {
  // 判断当前环境
  const isLocalhost = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1';
  const aetherUrl = isLocalhost ? 'http://localhost' : '/aether/';
  
  saveRecent('Aether 知识库', aetherUrl, 'ph:database-duotone');
  window.open(aetherUrl, '_blank');
};
const goCli = () => {
  saveRecent('全栈脚手架 CLI', 'https://github.com/your-repo/fullstack-scaffold', 'ph:terminal-duotone');
  window.open('https://github.com/your-repo/fullstack-scaffold', '_blank');
};
const goPrism = () => {
  saveRecent('Prism 自研数据引擎', 'https://github.com/your-repo/prism-engine', 'ph:cube-transparent-duotone');
  window.open('https://github.com/your-repo/prism-engine', '_blank');
};
const goReading = () => {
  saveRecent('阅读小屋', '/reading-hut', 'ph:books-duotone');
  window.location.href = '/reading-hut';
};


</script>

<style scoped>
.cards-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 20px;
}

.tool-card {
  display: flex;
  align-items: center;
  gap: 20px;
  background: var(--bg-card, #f8fafc);
  border: 1px solid var(--border, #e2e8f0);
  border-radius: 14px;
  padding: 24px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.tool-card:hover {
  box-shadow: 0 4px 20px rgba(37, 99, 235, 0.1);
  transform: translateY(-2px);
  border-color: var(--primary, #2563eb);
}

.card-icon {
  font-size: 2rem;
  flex-shrink: 0;
}

.card-content {
  flex: 1;
}

.card-content h3 {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary, #0f172a);
  margin: 0 0 6px;
}

.card-content p {
  font-size: 0.85rem;
  color: var(--text-secondary, #64748b);
  margin: 0;
  line-height: 1.5;
}

.card-btn {
  flex-shrink: 0;
  padding: 8px 20px;
  background: var(--primary, #2563eb);
  color: #fff;
  border: none;
  border-radius: 8px;
  font-size: 0.85rem;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s ease;
}

.card-btn:hover {
  background: var(--primary-hover, #1d4ed8);
}
</style>