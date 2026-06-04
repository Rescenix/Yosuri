<template>
  <div>
    <h1>领袖的个人简历</h1>

    <div class="section">
      <h2>关于我</h2>
      <p>
        曾独立完成智慧种植系统（获全国三等奖）、公网聊天室等项目。2023年首批调用ChatGPT
        API的先锋开发者。具备极强的自学能力、拆解问题的逻辑和向内求索的定力。短期休整后，现已准备好重新投入战斗。
      </p>
    </div>

    <div class="section">
      <h2>核心技能</h2>
      <span class="tag">Java</span>
      <span class="tag">Spring Cloud</span>
      <span class="tag">Redis</span>
      <span class="tag">Docker</span>
      <span class="tag">Vue.js</span>
      <span class="tag">Python</span>
      <span class="tag">MySQL</span>
      <span class="tag">Linux运维</span>
      <span class="tag">UML系统设计</span>
    </div>

    <div class="section">
      <h2>联系方式</h2>
      <p>📧 你的邮箱地址</p>
      <p>🌐 <a href="https://shanca.me">shanca.me</a></p>
    </div>
  </div>
</template>

<script setup>
// 简历组件，暂时无需额外逻辑
</script>

<style scoped>
body {
  font-family: Arial, sans-serif;
  max-width: 800px;
  margin: 50px auto;
  padding: 0 20px;
  line-height: 1.6;
  color: #333;
}
h1 {
  border-bottom: 2px solid #333;
  padding-bottom: 10px;
}
h2 {
  color: #555;
  margin-top: 30px;
}
.section {
  margin-bottom: 20px;
}
.tag {
  display: inline-block;
  background: #f0f0f0;
  padding: 2px 8px;
  margin: 2px;
  border-radius: 3px;
}
</style>