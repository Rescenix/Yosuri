<template>
  <main class="sites-view">
    <header class="sites-hero">
      <div>
        <p class="eyebrow"><Icon icon="mdi:web" width="17" /> AMEKO SITES · POWERED BY NETLIFY</p>
        <h1>把 Agent 写好的作品，<em>一键分享</em>出去</h1>
        <p class="intro">选择 Agent 已构建的网页，发布后会得到一个公开链接。朋友不需要安装 Ameko，也能直接打开。</p>
      </div>
      <div class="hero-actions">
        <button type="button" class="sites-return" @click="backToChat"><Icon icon="mdi:arrow-left" width="16" /> 回到聊天</button>
        <div class="hero-orbit" aria-hidden="true"><Icon icon="mdi:rocket-launch-outline" width="60" /></div>
      </div>
    </header>

    <section class="publish-card">
      <div class="section-heading"><span class="step">1</span><div><h2>发布一个站点</h2><p>{{ workdir ? `当前项目：${workdir}` : '先在编码页面选择一个项目文件夹' }}</p></div></div>
      <div v-if="!candidates.length" class="empty-build">
        <Icon icon="mdi:package-variant-closed" width="26" />
        <div><strong>还没有可发布的网页构建</strong><p>让 Agent 先运行构建命令，生成含有 <code>index.html</code> 的 <code>dist</code>、<code>build</code> 或 <code>out</code> 文件夹。</p></div>
      </div>
      <div v-else class="deploy-form">
        <label><span>公开地址名称</span><input v-model="name" autocomplete="off" placeholder="例如：my-travel-plan" /><small>将作为 <code>名称.netlify.app</code>，需在全球唯一。</small></label>
        <label><span>发布哪个构建目录</span><select v-model="source"><option v-for="item in candidates" :key="item.path" :value="item.path">{{ item.label }}</option></select></label>
        <label class="token-field"><span>Netlify Personal Access Token <a href="https://app.netlify.com/user/applications#personal-access-tokens" target="_blank" rel="noreferrer">去创建 ↗</a></span><div><input v-model="token" :type="showToken ? 'text' : 'password'" autocomplete="off" placeholder="nfp_..." /><button type="button" @click="showToken = !showToken" :aria-label="showToken ? '隐藏令牌' : '显示令牌'"><Icon :icon="showToken ? 'mdi:eye-off-outline' : 'mdi:eye-outline'" width="18" /></button></div><small>令牌只用于本次上传，不会由 Ameko 保存。</small></label>
        <label class="public-confirm"><input v-model="publishApproved" type="checkbox" /><span>我确认将此站点公开发布。任何拿到链接的人都可以访问。</span></label>
        <button class="deploy-button" type="button" :disabled="deploying || !name.trim() || !token.trim() || !publishApproved" @click="deploy(selectedSite)"><Icon :icon="deploying ? 'mdi:loading' : 'mdi:rocket-launch-outline'" width="19" :class="{ spinning: deploying }" />{{ deploying ? '正在发布…' : (selectedSite ? '确认并更新到 Netlify' : '确认并发布到 Netlify') }}</button>
      </div>
      <p v-if="notice" class="notice" :class="{ error: noticeError }"><Icon :icon="noticeError ? 'mdi:alert-circle-outline' : 'mdi:check-circle-outline'" width="17" />{{ notice }}</p>
    </section>

    <section class="sites-list">
      <div class="list-heading"><div><h2>已发布站点</h2><p>再次发布会更新同一个分享链接。</p></div><button type="button" class="refresh" @click="load"><Icon icon="mdi:refresh" width="17" />刷新</button></div>
      <div v-if="!sites.length" class="no-sites">第一次发布后，站点会显示在这里。</div>
      <article v-for="site in sites" :key="site.site_id" class="site-row">
        <div class="site-icon"><Icon icon="mdi:language-html5" width="24" /></div>
        <div class="site-info"><strong>{{ site.name }}</strong><a v-if="site.url" :href="site.url" target="_blank" rel="noreferrer">{{ site.url }}</a><small>最近发布 {{ formatDate(site.updated_at) }} · 来源 {{ site.source }}</small></div>
        <div class="site-actions"><button type="button" @click="copy(site.url)" :disabled="!site.url"><Icon icon="mdi:content-copy" width="17" />{{ copied === site.site_id ? '已复制' : '复制链接' }}</button><button type="button" class="update" @click="updateSite(site)"><Icon icon="mdi:upload-outline" width="17" />更新</button></div>
      </article>
    </section>
  </main>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Icon } from '@iconify/vue'

const router = useRouter()
const sites = ref([]); const candidates = ref([]); const workdir = ref('')
const name = ref(''); const source = ref('dist'); const token = ref(''); const showToken = ref(false)
const deploying = ref(false); const notice = ref(''); const noticeError = ref(false); const copied = ref(''); const selectedSite = ref(null); const publishApproved = ref(false)

async function load() {
  try {
    const res = await fetch('/api/sites'); const data = await res.json()
    if (!res.ok) throw new Error(data.error || '读取站点失败')
    sites.value = data.sites || []; candidates.value = data.candidates || []; workdir.value = data.workdir || ''
    if (!candidates.value.some(item => item.path === source.value)) source.value = candidates.value[0]?.path || 'dist'
    if (!name.value) name.value = (workdir.value || 'my-agent-site').split(/[\\/]/).filter(Boolean).pop() || 'my-agent-site'
  } catch (error) { notice.value = error.message; noticeError.value = true }
}
async function deploy(site) {
  deploying.value = true; notice.value = ''; noticeError.value = false
  try {
    const res = await fetch('/api/sites/deploy', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: name.value, source: source.value, token: token.value, site_id: site?.site_id || '', confirm_public: publishApproved.value }) })
    const data = await res.json()
    if (!res.ok) {
      if (data.suggested_name) { name.value = data.suggested_name; publishApproved.value = false; notice.value = `“${data.suggested_name}” 已填入建议名称。请检查后重新勾选公开发布。`; noticeError.value = true; return }
      throw new Error(data.error || '发布失败')
    }
    notice.value = `发布成功：${data.site.url}`; sites.value = [data.site, ...sites.value.filter(item => item.site_id !== data.site.site_id)]; selectedSite.value = data.site; publishApproved.value = false
  } catch (error) { notice.value = error.message; noticeError.value = true } finally { deploying.value = false }
}
function updateSite(site) { selectedSite.value = site; name.value = site.name; source.value = site.source; publishApproved.value = false; notice.value = '粘贴 Token 后点击“更新到 Netlify”，即可更新该站点。'; noticeError.value = false; window.scrollTo({ top: 0, behavior: 'smooth' }) }
async function copy(url) { if (!url) return; try { await navigator.clipboard.writeText(url); const match = sites.value.find(item => item.url === url); copied.value = match?.site_id || ''; setTimeout(() => copied.value = '', 1800) } catch { notice.value = '复制失败，请手动复制链接'; noticeError.value = true } }
function formatDate(value) { return value ? new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '刚刚' }
function backToChat() { router.push('/chat') }
onMounted(load)
</script>

<style scoped>
.sites-view{min-height:100vh;padding:44px 84px 72px 32px;color:#20302b;background:radial-gradient(circle at 85% 3%,#d9fbeb 0,transparent 27rem),#f6f8f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','Microsoft YaHei',sans-serif}.sites-hero,.publish-card,.sites-list{max-width:1080px;margin:0 auto}.sites-hero{position:relative;display:flex;justify-content:space-between;gap:32px;padding:34px 40px;margin-bottom:20px;overflow:hidden;border:1px solid #c7e2d2;border-radius:24px;color:#f4fff9;background:linear-gradient(120deg,#153a31,#1c6652)}.sites-hero:after{position:absolute;right:-85px;bottom:-140px;width:330px;height:330px;border:1px solid rgba(255,255,255,.18);border-radius:50%;box-shadow:0 0 0 46px rgba(255,255,255,.035),0 0 0 92px rgba(255,255,255,.025);content:''}.eyebrow{display:flex;align-items:center;gap:7px;margin:0 0 16px;color:#8ff0c6;font-size:11px;font-weight:800;letter-spacing:.08em}.sites-hero h1{max-width:650px;margin:0;font-size:clamp(30px,4vw,52px);line-height:1.1;letter-spacing:-.045em}.sites-hero em{color:#9af4d0;font-style:normal}.intro{max-width:620px;margin:16px 0 0;color:#d0e8dc;font-size:16px;line-height:1.65}.hero-actions{z-index:1;display:flex;align-items:flex-end;flex:0 0 104px;flex-direction:column;gap:13px}.sites-return{display:inline-flex;align-items:center;gap:6px;padding:6px 9px;border:1px solid rgba(159,245,208,.45);border-radius:8px;color:#d7f9e8;background:rgba(7,34,27,.22);font:700 11px inherit;white-space:nowrap;cursor:pointer;transition:background .18s,transform .18s}.sites-return:hover{background:rgba(129,240,188,.17);transform:translateX(-2px)}.hero-orbit{display:grid;place-items:center;flex:0 0 104px;width:104px;height:104px;border:1px solid rgba(159,245,208,.5);border-radius:50%;color:#a5f5d1;background:rgba(255,255,255,.07);box-shadow:0 0 0 13px rgba(255,255,255,.035)}.publish-card,.sites-list{border:1px solid #dce7e0;border-radius:18px;background:rgba(255,255,255,.86);box-shadow:0 12px 32px rgba(18,62,43,.06)}.publish-card{padding:28px 30px}.section-heading,.list-heading{display:flex;align-items:center;gap:13px}.section-heading h2,.list-heading h2{margin:0;color:#21322c;font-size:20px;letter-spacing:-.02em}.section-heading p,.list-heading p{margin:4px 0 0;color:#718078;font-size:13px}.step{display:grid;place-items:center;width:31px;height:31px;border-radius:50%;color:#126846;background:#d9f8e8;font-weight:800}.deploy-form{display:grid;grid-template-columns:1fr 220px;gap:16px;margin-top:24px}.deploy-form label{display:grid;gap:7px;color:#43544c;font-size:13px;font-weight:700}.deploy-form input,.deploy-form select{box-sizing:border-box;width:100%;height:46px;padding:0 13px;border:1px solid #cbdad1;border-radius:10px;outline:none;color:#24362e;background:#fff;font:inherit}.deploy-form input:focus,.deploy-form select:focus{border-color:#20a86e;box-shadow:0 0 0 3px rgba(32,168,110,.13)}.token-field{grid-column:1/-1}.token-field span{display:flex;gap:9px}.token-field a{color:#16855a;text-decoration:none}.token-field>div{position:relative}.token-field input{padding-right:48px}.token-field button{position:absolute;right:7px;top:6px;width:34px;height:34px;border:0;border-radius:8px;color:#486359;background:transparent;cursor:pointer}.token-field small{color:#7b8881;font-weight:400}.deploy-button{display:flex;align-items:center;justify-content:center;gap:8px;grid-column:1/-1;height:48px;border:0;border-radius:11px;color:#fff;background:linear-gradient(135deg,#198a5c,#0e6645);font-size:15px;font-weight:800;cursor:pointer;transition:transform .18s,box-shadow .18s}.deploy-button:not(:disabled):hover{transform:translateY(-1px);box-shadow:0 10px 20px rgba(20,116,76,.2)}.deploy-button:disabled{opacity:.5;cursor:not-allowed}.spinning{animation:spin 1s linear infinite}.notice{display:flex;align-items:center;gap:7px;margin:18px 0 0;padding:11px 13px;border-radius:9px;color:#14714c;background:#e8f9ef;font-size:13px}.notice.error{color:#a33436;background:#fff0f0}.empty-build{display:flex;gap:13px;align-items:flex-start;margin-top:24px;padding:17px;border:1px dashed #bfd2c7;border-radius:12px;color:#648076;background:#f8fbf9}.empty-build strong{color:#375149}.empty-build p{margin:5px 0 0;font-size:13px;line-height:1.6}.empty-build code{padding:1px 4px;border-radius:4px;background:#e7f0eb}.sites-list{margin-top:20px;padding:25px 30px}.list-heading{justify-content:space-between}.refresh,.site-actions button{display:inline-flex;align-items:center;justify-content:center;gap:6px;min-height:38px;padding:0 12px;border:1px solid #cddbd3;border-radius:9px;color:#3c584c;background:#fff;font-weight:700;cursor:pointer}.refresh:hover,.site-actions button:hover{border-color:#69ad8a;color:#12754b}.no-sites{margin-top:20px;padding:24px;border-radius:11px;color:#7b8982;background:#f6f9f7;text-align:center;font-size:14px}.site-row{display:flex;align-items:center;gap:13px;margin-top:17px;padding:15px;border:1px solid #e2ebe5;border-radius:13px}.site-icon{display:grid;place-items:center;flex:0 0 44px;height:44px;border-radius:11px;color:#179060;background:#e3f8ed}.site-info{display:grid;min-width:0;gap:3px}.site-info strong{color:#263a31}.site-info a{overflow:hidden;color:#17845a;font-size:13px;text-decoration:none;text-overflow:ellipsis;white-space:nowrap}.site-info small{color:#809087;font-size:12px}.site-actions{display:flex;gap:8px;margin-left:auto}.site-actions .update{border-color:#bde4cf;color:#14764d;background:#ecfbf2}.site-actions button:disabled{opacity:.45;cursor:not-allowed}@keyframes spin{to{transform:rotate(360deg)}}@media(max-width:720px){.sites-view{padding:18px 18px 78px}.sites-hero{padding:28px 24px}.hero-orbit{display:none}.hero-actions{flex:0 0 auto}.deploy-form{grid-template-columns:1fr}.sites-list,.publish-card{padding:22px 18px}.site-row{align-items:flex-start;flex-wrap:wrap}.site-actions{width:100%;margin-left:57px}.site-actions button{flex:1}}
.public-confirm{display:flex!important;align-items:flex-start;gap:10px;grid-column:1/-1;padding:12px 13px;border:1px solid #eadbb2;border-radius:10px;color:#5d4b1e!important;background:#fff9e9;font-weight:500!important;line-height:1.5}.public-confirm input{width:18px!important;height:18px!important;min-height:18px!important;margin:1px 0 0;accent-color:#198a5c}
</style>
