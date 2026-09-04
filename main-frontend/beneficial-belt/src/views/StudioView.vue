<template>
  <div class="studio-shell">
    <!-- 左侧导航（即梦式） -->
    <aside class="studio-sidebar">
      <div class="side-brand">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none"><path d="M12 2L2 7l10 5 10-5-10-5z" fill="var(--app-accent)"/><path d="M2 17l10 5 10-5M2 12l10 5 10-5" stroke="var(--app-accent)" stroke-width="2" stroke-linecap="round"/></svg>
      </div>
      <nav class="side-menu">
        <button class="side-nav" :class="{ active: studioTab === 'create' }" @click="studioTab = 'create'" title="创作">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none"><path d="M4 20l1.5-4.5L17 4a2.1 2.1 0 013 3L8.5 18.5 4 20z" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/></svg>
        </button>
        <button class="side-nav" :class="{ active: studioTab === 'inspire' }" @click="studioTab = 'inspire'" title="灵感广场">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none"><path d="M9 18h6M10 21h4M12 3a7 7 0 00-4 12.7c.6.5 1 1.3 1 2.1V18h6v-.2c0-.8.4-1.6 1-2.1A7 7 0 0012 3z" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
        </button>
        <button class="side-nav" :class="{ active: studioTab === 'assets' }" @click="studioTab = 'assets'" title="素材库">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none"><path d="M3 7l9-4 9 4-9 4-9-4zM3 7v10l9 4 9-4V7M12 11v10" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/></svg>
        </button>
        <button class="side-nav" :class="{ active: studioTab === 'canvas' }" @click="studioTab = 'canvas'" title="画布">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none"><rect x="3" y="3" width="8" height="8" rx="1.5" stroke="currentColor" stroke-width="1.8"/><rect x="13" y="3" width="8" height="8" rx="1.5" stroke="currentColor" stroke-width="1.8"/><rect x="3" y="13" width="8" height="8" rx="1.5" stroke="currentColor" stroke-width="1.8"/><rect x="13" y="13" width="8" height="8" rx="1.5" stroke="currentColor" stroke-width="1.8"/></svg>
        </button>
      </nav>
      <div class="side-bottom">
        <button class="side-tool" title="API">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none"><path d="M8 5h8l4 7-4 7H8l-4-7 4-7z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/><path d="M12 8v8M8 12h8" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>
        </button>
        <button class="side-tool" title="API 编辑">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none"><path d="M4 20l1.5-4.5L17 4a2.1 2.1 0 013 3L8.5 18.5 4 20z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>
        </button>
        <button class="side-tool" title="API 菜单">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none"><circle cx="5" cy="12" r="1.6" fill="var(--app-accent)"/><circle cx="12" cy="12" r="1.6" fill="var(--app-accent)"/><circle cx="19" cy="12" r="1.6" fill="var(--app-accent)"/></svg>
        </button>
        <button class="side-avatar" title="账号">👤</button>
      </div>
    </aside>

    <main class="studio-main">
      <Transition name="toast">
        <div v-if="toastMsg" class="studio-toast">{{ toastMsg }}</div>
      </Transition>
      <!-- 资产 tab -->
      <template v-if="studioTab === 'assets'">
              <div class="assets-head">
                <h2>素材库</h2>
              </div>
              <div class="assets-filter">
                              <button class="filter-tab" :class="{ active: assetFilter === 'all' }" @click="assetFilter = 'all'">全部</button>
                              <button class="filter-tab" :class="{ active: assetFilter === 'image' }" @click="assetFilter = 'image'">图片</button>
                              <button class="filter-tab" :class="{ active: assetFilter === 'video' }" @click="assetFilter = 'video'">视频</button>
                              <span class="filter-divider"></span>
                              <button class="filter-tab" :class="{ active: assetFilter === 'private' }" @click="assetFilter = 'private'">
                                <Icon icon="mdi:lock" width="13" /> 私密
                              </button>
                            </div>
              <div class="assets-section">
                <div class="section-date">昨天</div>
                <div class="assets-grid">
                  <div v-for="(a, i) in visibleAssets" :key="i" class="asset-card" @click="openAssetPreview(a)">
                    <div class="asset-thumb-wrap">
                      <img v-if="a.kind === 'image'" :src="a.src" class="asset-thumb" />
                      <video v-else-if="a.kind === 'video'" :src="a.src" class="asset-thumb" muted></video>
                      <span v-if="a.dur" class="asset-dur">{{ a.dur }}</span>
                      <div class="asset-ops" @click.stop>
                                              <button class="op-btn" title="删除" @click="askDelete(a)"><Icon icon="mdi:trash-can-outline" width="13" /></button>
                                              <button v-if="assetFilter === 'private'" class="op-btn" title="取消私密" @click="togglePrivate(a)"><Icon icon="mdi:lock-open-variant" width="13" /></button>
                                              <button v-else class="op-btn" title="设为私密" @click="togglePrivate(a)"><Icon icon="mdi:lock-outline" width="13" /></button>
                                            </div>
                    </div>
                    <div class="asset-name">{{ a.name }}</div>
                  </div>
                  <div v-if="!libraryAssets.length" class="empty-state">
                    <div class="empty-art"><Icon icon="mdi:view-grid-outline" width="34" /></div>
                    <p>素材库是空的<br />生成视频后自动入库</p>
                  </div>
                </div>
              </div>
            </template>

      <!-- 灵感广场 tab -->
      <template v-else-if="studioTab === 'inspire'">
        <div class="assets-head">
          <h2>💡 灵感广场</h2>
          <span class="assets-sub">热门 AI 短剧创作灵感，一键填入</span>
        </div>
        <div class="template-row">
          <div v-for="t in inspireCards" :key="t.id" class="template-card" @click="applyTemplate(t)">
            <div class="template-img-wrap">
              <img :src="t.thumb" class="template-thumb" />
              <span class="template-model-tag">🔥 {{ t.hot }}</span>
            </div>
            <div class="template-info">
              <span class="template-title">{{ t.title }}</span>
              <div class="template-foot">
                <span class="template-try">去看看</span>
              </div>
              <span class="template-desc">{{ t.desc }}</span>
            </div>
          </div>
        </div>
      </template>

      <!-- 画布 tab -->
      <template v-else-if="studioTab === 'canvas'">
        <div class="assets-head">
          <h2><Icon icon="mdi:view-grid-outline" width="18" /> 画布</h2>
          <span class="assets-sub">多镜头时间线编辑</span>
        </div>
        <div class="empty-state">
          <div class="empty-art"><Icon icon="mdi:view-grid-outline" width="34" /></div>
          <p>画布编辑开发中<br />拖拽分镜、调整首尾帧、预览成片</p>
        </div>
      </template>

      <!-- 创作 tab（即梦式） -->
      <template v-else>
              <div v-show="!busy && !result" class="studio-top">
                      <button class="top-assets" @click="studioTab = 'assets'">
                        <svg width="15" height="15" viewBox="0 0 24 24" fill="none"><path d="M3 7l9-4 9 4-9 4-9-4zM3 7v10l9 4 9-4V7" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/></svg>
                        素材库
                      </button>
                    </div>

              <div class="create-wrap">
                              <h1 v-show="!busy && !result" class="create-hello">你好，想创作什么？</h1>

                              <!-- 生成中 / 结果视图（整页切换） -->
                              <div v-if="busy || result" class="gen-view">
                                <div class="gen-view-preview">
                                  <img v-if="selectedRef && selectedRef.kind === 'image' && selectedRef.src" :src="selectedRef.src" class="gen-view-ref" />
                                  <div v-else class="gen-view-ref-placeholder">
                                    <svg width="56" height="56" viewBox="0 0 24 24" fill="none"><path d="M5 5l14 7-14 7V5z" fill="var(--app-accent)"/></svg>
                                    <span class="gen-view-ref-hint">未选择参考图<br />生成纯文生视频</span>
                                  </div>
                                </div>
                                <div class="gen-view-info">
                                  <div class="gen-view-prompt">{{ text }}</div>
                                  <div class="gen-card-params">
                                    <span class="gen-param">{{ videoSpec.replace('landscape', '16:9').replace('portrait', '9:16').replace('-720', ' 720P').replace('-1080', ' 1080P') }}</span>
                                    <span class="gen-param">{{ videoSeconds }}s</span>
                                    <span v-if="selectedRef" class="gen-param"><Icon icon="mdi:paperclip" width="12" /> {{ selectedRef.name }}</span>
                                  </div>
                                  <div v-if="busy" class="gen-view-loading">
                                    <div class="gen-shimmer-bar"><div class="gen-shimmer"></div></div>
                                    <span class="gen-loading-text">已等待 {{ elapsed }}s（约 1-2 分钟）</span>
                                    <button class="gen-view-regen" style="margin-top: 14px" @click="busy = false; clearInterval(elapsedTimer); clearInterval(pollTimer); logLines.push('× 已取消生成')">取消生成</button>
                                    <div v-if="logLines.length" class="gen-log">
                                      <div v-for="(l, i) in logLines" :key="i" class="log-line" :class="{ err: l.startsWith('×') }">{{ l }}</div>
                                    </div>
                                  </div>
                                  <div v-if="result" class="gen-view-result">
                                    <video :src="result.video" controls class="gen-video" autoplay></video>
                                    <div class="gen-video-meta">{{ result.name }} · {{ result.size }} · {{ result.seconds }}s</div>
                                    <div class="gen-view-actions">
                                      <button class="gen-view-regen" @click="result = null; busy = false; clearInterval(elapsedTimer); logLines = []"><Icon icon="mdi:arrow-left" width="14" /> 返回创作</button>
                                      <button class="gen-view-regen" @click="result = null; logLines = []; generate()"><Icon icon="mdi:refresh" width="14" /> 再生成一次</button>
                                    </div>
                                  </div>
                                </div>
                              </div>

                              <div v-show="!busy && !result" class="studio-content">
          <div class="template-row">
                      <div v-for="t in templates" :key="t.id" class="template-card" @click="applyTemplate(t)">
                        <div class="template-img-wrap">
                          <img :src="t.thumb" class="template-thumb" />
                          <span class="template-model-tag">免费模型</span>
                          <span class="template-try">试一试</span>
                        </div>
                        <div class="template-info">
                          <span class="template-title">{{ t.title }}</span>
                          <span class="template-desc">{{ t.desc }}</span>
                        </div>
                      </div>
                    </div>

          <!-- 底部创作输入框（即梦式） -->
                    <div v-if="logLines.length" class="compose-errors">
                      <div v-for="(l, i) in logLines.filter(x => x.startsWith('×'))" :key="i" class="compose-error-line">{{ l }}</div>
                    </div>
                    <div class="compose-box">
                      <div class="compose-ref">
                        <input ref="refInput" type="file" accept="image/*" style="display:none" @change="onRefFile" />
                        <div v-if="!selectedRef" class="ref-slot" @click="pickRef">
                          <Icon icon="mdi:plus" width="20" />
                          <span>参考</span>
                        </div>
                        <div v-else class="ref-slot ref-selected" @click="pickRef" :title="selectedRef.name">
                          <img v-if="selectedRef.kind === 'image'" :src="selectedRef.src" class="ref-thumb" />
                          <video v-else :src="selectedRef.src" class="ref-thumb" muted></video>
                          <span class="ref-remove" @click.stop="selectedRef = null">✕</span>
                        </div>
                      </div>
                      <div class="compose-main">
                                              <div class="compose-theme-row">
                                                <select v-model="selectedTheme" class="theme-select" @change="onThemeChange">
                                                  <option v-for="t in themePresets" :key="t.id" :value="t.id">{{ t.name }}</option>
                                                  <option value="custom">自定义主题</option>
                                                </select>
                                                <input v-if="selectedTheme === 'custom'" v-model="customTheme" class="theme-custom-input" placeholder="输入你的主题，比如：赛博朋克少女" @input="text = customTheme" />
                                              </div>
                                              <textarea v-model="text" class="compose-input" placeholder="输入想法、剧本或上传参考，支持 &quot;/&quot; 使用技能，@ 添加主体，和 Agent 一起创作"></textarea>
                        <div class="compose-toolbar">
                          <span class="tool-chip">
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none"><path d="M5 5l14 7-14 7V5z" fill="currentColor"/></svg>
                            视频生成
                          </span>
                          <select v-model="agenModel" class="tool-select" title="模型">
                            <option value="agnes-video-2.5-flash">Agnes 2.5 flash 免费</option>
                            <option value="agnes-video-2.0">Agnes 2.0 1080p 免费</option>
                          </select>
                          <select v-model="refMode" class="tool-select" title="参考模式" @change="onRefModeChange">
                            <option value="reference">全能参考</option>
                            <option value="keyframe">首尾帧</option>
                            <option value="multi">智能多帧</option>
                            <option value="edit">智能编辑</option>
                            <option value="long">超长视频</option>
                          </select>
                          <select v-model="videoSpec" class="tool-select" title="比例/分辨率">
                            <option value="landscape-720">16:9 720P</option>
                            <option value="landscape-1080">16:9 1080P</option>
                            <option value="portrait-720">9:16 720P</option>
                            <option value="portrait-1080">9:16 1080P</option>
                          </select>
                          <select v-model="videoSeconds" class="tool-select" title="时长">
                                                      <option value="4">4s</option>
                                                      <option value="5">5s</option>
                                                      <option value="8">8s</option>
                                                      <option value="10">10s</option>
                                                      <option value="12">12s</option>
                                                    </select>
                                                    <select v-if="refMode === 'long'" v-model="segments" class="tool-select" title="链式段数（每段末尾自动接续下一段）">
                                                      <option :value="2">2 段</option>
                                                      <option :value="3">3 段</option>
                                                      <option :value="4">4 段</option>
                                                      <option :value="6">6 段</option>
                                                    </select>
                                                    <input v-model="seedInput" class="seed-input" placeholder="Seed（留空随机）" title="随机种子，相同 seed+prompt 出片一致" />
                          <div class="toolbar-right">
                            <button class="compose-send" :disabled="busy" @click="generate">
                              <svg width="18" height="18" viewBox="0 0 24 24" fill="none"><path d="M12 19V5M5 12l7-7 7 7" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>
                            </button>
                          </div>
                        </div>
                      </div>
                    </div>
        </div>
        </div>
      </template>
    </main>

    <!-- 参考素材选择弹窗 -->
    <div v-if="showRefPicker" class="ref-picker-modal" @click.self="showRefPicker = false">
      <div class="ref-picker">
        <div class="ref-picker-head">
          <span class="ref-picker-title">选择参考素材</span>
          <button class="ref-picker-upload" @click="uploadRef">上传新素材</button>
          <button class="ref-picker-close" @click="showRefPicker = false"><Icon icon="mdi:close" width="14" /></button>
        </div>
        <div class="ref-picker-grid">
          <div v-for="a in libraryAssets.filter(a => a.kind === 'image' && !privateAssets.includes(a.name))" :key="a.src" class="ref-picker-item" @click="selectRefAsset(a)">
            <div class="ref-picker-thumb-wrap">
              <img v-if="a.kind === 'image'" :src="a.src" class="ref-picker-thumb" />
              <video v-else :src="a.src" class="ref-picker-thumb" muted></video>
              <span v-if="a.dur" class="ref-picker-dur">{{ a.dur }}</span>
            </div>
            <span class="ref-picker-name">{{ a.name }}</span>
          </div>
          <div v-if="!libraryAssets.length" class="empty-state">
            <div class="empty-art"><Icon icon="mdi:folder-open-outline" width="34" /></div>
            <p>素材库是空的<br />点右上角上传，或先去生成素材</p>
          </div>
        </div>
      </div>
    </div>

    <!-- 删除确认弹窗（轻量） -->
    <div v-if="confirmMsg" class="confirm-modal" @click.self="confirmMsg = null">
      <div class="confirm-box">
        <div class="confirm-title">删除素材</div>
        <p class="confirm-text">确定删除「{{ confirmMsg }}」吗？此操作不可恢复。</p>
        <div class="confirm-actions">
          <button class="confirm-cancel" @click="confirmMsg = null">取消</button>
          <button class="confirm-danger" @click="doDelete">删除</button>
        </div>
      </div>
    </div>

    <!-- 素材预览详情（即梦式） -->
    <div v-if="assetPreview" class="asset-preview-modal" @click.self="assetPreview = null">
      <div class="asset-preview">
        <div class="preview-left">
                                  <video v-if="assetPreview.kind === 'video'" :src="assetPreview.src" class="preview-video" controls autoplay ref="previewVideo"></video>
                                  <img v-else :src="assetPreview.src" class="preview-video" />
                                </div>
        <div class="preview-right">
          <div class="preview-head">
            <div class="preview-head-left">
              <button class="pv-icon-btn" @click="assetPreview = null"><svg width="16" height="16" viewBox="0 0 24 24" fill="none"><path d="M18 6L6 18M6 6l12 12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg></button>
            </div>
            <button class="pv-download" @click="downloadAsset">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none"><path d="M12 3v12m0 0l-5-5m5 5l5-5M4 21h16" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
              下载
            </button>
            <button class="pv-icon-btn"><svg width="16" height="16" viewBox="0 0 24 24" fill="none"><path d="M12 3l2.7 5.5 6 .9-4.3 4.2 1 6-5.4-2.9-5.4 2.9 1-6L3.3 9.4l6-.9L12 3z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg></button>
            <button class="pv-icon-btn"><svg width="16" height="16" viewBox="0 0 24 24" fill="none"><circle cx="5" cy="12" r="1.6" fill="currentColor"/><circle cx="12" cy="12" r="1.6" fill="currentColor"/><circle cx="19" cy="12" r="1.6" fill="currentColor"/></svg></button>
          </div>
          <div class="pv-prompt">
            <div class="pv-label">视频提示词</div>
            <p class="pv-prompt-text">{{ assetPreview.name }}</p>
            <div class="pv-meta">Agnes 2.5 免费 | {{ assetPreview.dur || '5s' }} | 16:9 | 720P</div>
            <div class="pv-detail"><svg width="13" height="13" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.6"/><path d="M12 11v5M12 8v.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg> 详细信息</div>
          </div>
          <div class="pv-actions">
                      <button class="pv-act" @click="useAsReference">
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none"><rect x="2" y="5" width="20" height="14" rx="3" stroke="currentColor" stroke-width="1.6"/><path d="M10 9l5 3-5 3V9z" fill="currentColor"/></svg>
                        设为参考素材
                      </button>
                      <button v-if="assetPreview.kind === 'video'" class="pv-act" @click="extractFrame">
                        <Icon icon="mdi:camera-outline" width="15" /> 抽帧为图片参考
                      </button>
            <div class="pv-act-grid">
              <button class="pv-act"><svg width="15" height="15" viewBox="0 0 24 24" fill="none"><rect x="4" y="4" width="16" height="16" rx="2" stroke="currentColor" stroke-width="1.6"/><path d="M12 8v8M8 12h8" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>局部重绘</button>
              <button class="pv-act"><svg width="15" height="15" viewBox="0 0 24 24" fill="none"><path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8L12 3z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>画质增强</button>
              <button class="pv-act"><svg width="15" height="15" viewBox="0 0 24 24" fill="none"><path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>续写时长</button>
              <button class="pv-act"><svg width="15" height="15" viewBox="0 0 24 24" fill="none"><path d="M4 20l1.5-4.5L17 4a2.1 2.1 0 013 3L8.5 18.5 4 20z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>片段剪辑</button>
              <button class="pv-act"><svg width="15" height="15" viewBox="0 0 24 24" fill="none"><path d="M12 3l2.5 5 5.5.8-4 3.9.9 5.5-4.9-2.6-4.9 2.6.9-5.5-4-3.9L9.5 8 12 3z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>流畅补帧</button>
              <button class="pv-act"><svg width="15" height="15" viewBox="0 0 24 24" fill="none"><path d="M4 12a8 8 0 0116 0M7 12a5 5 0 0110 0M10 12a2 2 0 014 0" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>智能音效</button>
              <button class="pv-act"><svg width="15" height="15" viewBox="0 0 24 24" fill="none"><rect x="4" y="8" width="16" height="12" rx="3" stroke="currentColor" stroke-width="1.6"/><path d="M9 3h6M10 8v5M14 8v5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>口型同步</button>
              <button class="pv-act"><svg width="15" height="15" viewBox="0 0 24 24" fill="none"><path d="M9 18V5l12-2v13" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/><circle cx="6" cy="18" r="3" stroke="currentColor" stroke-width="1.6"/><circle cx="18" cy="16" r="3" stroke="currentColor" stroke-width="1.6"/></svg>智能配乐</button>
            </div>
            <button class="pv-act"><svg width="15" height="15" viewBox="0 0 24 24" fill="none"><path d="M4 4v16M4 4h16M4 4l16 16" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>再次编辑</button>
            <button class="pv-act"><svg width="15" height="15" viewBox="0 0 24 24" fill="none"><path d="M20 12a8 8 0 11-2.3-5.6M20 4v6h-6" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>重新生成</button>
            <button class="pv-act"><svg width="15" height="15" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="1.6"/><path d="M12 3v4M12 17v4M3 12h4M17 12h4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>查看生成记录</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, reactive, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import { API_BASE_URL } from '../config.js'
import { backendURL } from '../desktopTransport.js'

const topic = ref('')
const voice = ref('zh-TW-HsiaoChenNeural')  // 曉臻默认
const text = ref('')
const busy = ref(false)
const logLines = ref([])
const result = ref(null)
const plan = ref(null)
const segs = ref([])
const pexelsKey = ref(localStorage.getItem('pexels_key') || '')
const keySaved = ref(false)
const orientation = ref('landscape')
const genPlatform = ref('agnes')
// 视频生成参数：模型 / 比例分辨率 / 秒数
const agenModel = ref('agnes-video-2.5-flash')
const videoSpec = ref('landscape-720')
const videoSeconds = ref('5')
const compose = ref(false)
const transOpen = ref(false)
const transLang = ref('en')
const transBusy = ref(false)
const transResult = ref('')
const transError = ref('')
// 即梦式 tab：创作 / 资产
const studioTab = ref('create')
// 素材库：后端 /api/studio/library 扫描本地素材目录动态填充（不硬编码）
const libraryAssets = ref([])
// 私密素材文件名（localStorage 持久化，私密的不显示）
const privateAssets = ref(JSON.parse(localStorage.getItem('studio_private_assets') || '[]'))
function togglePrivate(a) {
  const i = privateAssets.value.indexOf(a.name)
  if (i >= 0) privateAssets.value.splice(i, 1)
  else privateAssets.value.push(a.name)
  localStorage.setItem('studio_private_assets', JSON.stringify(privateAssets.value))
  toastMsg.value = i >= 0 ? `🔓 ${a.name} 已设为公开` : `🔒 ${a.name} 已设为私密`
  clearTimeout(toastTimer); toastTimer = setTimeout(() => { toastMsg.value = '' }, 2200)
}
const confirmMsg = ref('')
const confirmTarget = ref(null)
function askDelete(a) { confirmTarget.value = a; confirmMsg.value = a.name }
async function doDelete() {
  const a = confirmTarget.value
  confirmMsg.value = ''
  if (!a) return
  try {
    await fetch(API_BASE_URL + '/api/studio/library/' + encodeURIComponent(a.name), { method: 'DELETE' })
    toastMsg.value = `🗑 已删除 ${a.name}`
  } catch (e) {
    toastMsg.value = `❌ 删除失败`
  }
  clearTimeout(toastTimer); toastTimer = setTimeout(() => { toastMsg.value = '' }, 2200)
  try {
    const r = await fetch(API_BASE_URL + '/api/studio/library')
        if (r.ok) { const d = await r.json(); if (d.assets) libraryAssets.value = d.assets.map(a => ({...a, src: backendURL(a.src)})) }
  } catch (e) {}
}
const assetFilter = ref('all') // all / image / video / private
const visibleAssets = computed(() => {
  const isPrivate = a => privateAssets.value.includes(a.name)
  if (assetFilter.value === 'private') return libraryAssets.value.filter(a => isPrivate(a))
  const list = libraryAssets.value.filter(a => !isPrivate(a))
  if (assetFilter.value === 'image') return list.filter(a => a.kind === 'image')
  if (assetFilter.value === 'video') return list.filter(a => a.kind === 'video')
  return list
})
function applyFromLibrary(a) {
  selectedAsset.value = a
  toastMsg.value = a.kind === 'video' ? `📎 已选素材：${a.name}` : `📎 已选基准图：${a.name}`
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toastMsg.value = '' }, 2200)
}
const selectedAsset = ref(null)
const toastMsg = ref('')
let toastTimer = null
// 素材预览弹窗
const assetPreview = ref(null)
function openAssetPreview(a) { assetPreview.value = a }
function downloadAsset() {
  const a = assetPreview.value
  if (a) {
    const link = document.createElement('a'); link.href = a.src; link.download = a.name; link.click()
  }
}
function useAsReference() {
  toastMsg.value = `已用作参考：${assetPreview.value?.name}`
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toastMsg.value = '' }, 2200)
}
// 视频抽帧（进度条当前位置）
const previewVideo = ref(null)
async function extractFrame() {
  const a = assetPreview.value
  if (!a || a.kind !== 'video') return
  const v = previewVideo.value
  const t = v && v.currentTime ? v.currentTime : 0
  toastMsg.value = `抽帧中…（${t.toFixed(1)}s）`
  try {
    const r = await fetch(API_BASE_URL + '/api/studio/frames', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ video: a.name, time: t })
    })
    const d = await r.json()
    if (d.ok) {
      toastMsg.value = `已抽帧：${d.name}（可作图片参考）`
      // 刷新素材库
      const rr = await fetch(API_BASE_URL + '/api/studio/library')
      if (rr.ok) { const dd = await rr.json(); if (dd.assets) libraryAssets.value = dd.assets.map(a => ({...a, src: backendURL(a.src)})) }
    } else {
      toastMsg.value = `抽帧失败：${d.error || ''}`
    }
  } catch (e) {
    toastMsg.value = '抽帧失败'
  }
  clearTimeout(toastTimer); toastTimer = setTimeout(() => { toastMsg.value = '' }, 2600)
}
// 即梦式模板卡（点"试一试"填入分镜）
const templates = ref([
  {
    id: 'sakura',
        title: '日系青春短片',
        desc: '清新柔光与胶片质感，温柔细腻的青春故事',
        icon: 'mdi:flower',
        thumb: '/studio_sakura.png',
        text: '红发少女站在樱花树下回眸，柔光\n她伸手接住飘落的花瓣\n她抱着小猫坐在石阶上微笑\n她在樱花雨中漫步，裙摆飘动',
  },
  {
    id: 'tokyo',
    title: '东京绘梨衣短剧',
    desc: '红发少女独自在东京寻找，场景快速切换',
    icon: 'mdi:city',
    thumb: '/studio_tokyo.png',
    text: '涩谷十字路口，红发少女逆着人潮而立\n东京塔黄昏，她仰望塔尖\n目黑川樱花步道，她沿河岸行走\n新宿歌舞伎町夜，霓虹映在她脸上',
  },
  {
    id: 'jk',
    title: 'JK 白丝日常',
    desc: '从裙摆到全身的细节特写机位，宅男最爱',
    icon: 'mdi:shoe-sneaker',
    thumb: '/studio_jk.png',
    text: 'JK红发少女白丝脚部特写，低角度仰拍\n镜头从脚踝上移，扫过腿部线条\n定格全身，JK制服红发飘动\n她回头微笑，樱花飘落',
  },
])
function applyTemplate(t) {
  text.value = t.text
  topic.value = t.title
}
// 灵感广场卡片（撑门面：热门灵感一键填入）
const inspireCards = ref([
  { id: 'i1', title: '银发少女东京樱花街景', desc: 'AI 虚拟人漫步东京，樱花与霓虹交错', hot: '1.2万', icon: 'mdi:city', thumb: '/studio_tokyo.png', text: '银发少女在东京街头漫步\n樱花飘落，霓虹闪烁\n她回头微笑' },
  { id: 'i2', title: '日系青春校园日常', desc: '清新柔光与胶片质感，青春朦胧故事', hot: '8900', icon: 'mdi:flower', thumb: '/studio_sakura.png', text: '少女在樱花树下回眸\n她伸手接住花瓣\n她在校园走廊奔跑' },
  { id: 'i3', title: 'JK 白丝氛围感短片', desc: '从裙摆到全身的细节特写机位', hot: '6500', icon: 'mdi:shoe-sneaker', thumb: '/studio_jk.png', text: 'JK少女白丝脚部特写\n镜头从脚踝上移\n定格全身微笑' },
])
function pickRef() {
  showRefPicker.value = true
}
function selectRefAsset(a) {
  selectedRef.value = { name: a.name, kind: a.kind, src: a.src, file: null }
  showRefPicker.value = false
  toastMsg.value = `已选参考：${a.name}`
  clearTimeout(toastTimer); toastTimer = setTimeout(() => { toastMsg.value = '' }, 2000)
}
function uploadRef() {
  showRefPicker.value = false
  refInput.value?.click()
}
function onRefFile(e) {
  const file = e.target.files?.[0]
  if (!file) return
  if (file.type.startsWith('video/')) {
    toastMsg.value = '仅支持图片参考'
    clearTimeout(toastTimer); toastTimer = setTimeout(() => { toastMsg.value = '' }, 2200)
    return
  }
  const kind = 'image'
  selectedRef.value = { name: file.name, kind, file, src: '' }
  // 上传到后端
  const fd = new FormData()
  fd.append('file', file)
  fetch(API_BASE_URL + '/api/studio/upload', { method: 'POST', body: fd })
    .then(r => r.json()).then(d => {
      if (d.ok) {
        selectedRef.value.src = d.src
        selectedRef.value.name = d.name
        toastMsg.value = `已上传参考：${d.name}`
      } else {
        toastMsg.value = '❌ 上传失败'
      }
    }).catch(() => { toastMsg.value = '❌ 上传失败' })
  clearTimeout(toastTimer); toastTimer = setTimeout(() => { toastMsg.value = '' }, 2200)
}
const selectedRef = ref(null)
const refInput = ref(null)
const showRefPicker = ref(false)
// 生成计时
const elapsed = ref(0)
let elapsedTimer = null
const taskId = ref('')
const seedInput = ref('')
const refMode = ref('reference') // reference / keyframe / multi / edit / long
const lastFrameRef = ref(null) // 尾帧图（首尾帧模式）
const segments = ref(3) // 超长视频：链式段数
function onRefModeChange() {
  if (refMode.value === 'keyframe') {
    toastMsg.value = '首尾帧：选首帧图（参考）+ 尾帧图'
    clearTimeout(toastTimer); toastTimer = setTimeout(() => { toastMsg.value = '' }, 2500)
  } else if (refMode.value === 'long') {
    toastMsg.value = '超长视频：分多段递进生成 + 抽帧接续 + 自动拼接成片'
    clearTimeout(toastTimer); toastTimer = setTimeout(() => { toastMsg.value = '' }, 2500)
  } else if (refMode.value !== 'reference') {
    toastMsg.value = '暂不支持，请使用「全能参考」或「首尾帧」'
    clearTimeout(toastTimer); toastTimer = setTimeout(() => { toastMsg.value = '' }, 2500)
    refMode.value = 'reference'
  }
}
// 预设主题（点选自动填入提示词 + 参数）
const themePresets = [
  {
    id: 'sakura', name: '日系青春',
    prompt: 'cinematic shot, a beautiful girl with long crimson red hair and ruby red eyes, wearing white kimono with red flower pattern, standing under cherry blossom tree at night in Tokyo, neon signs glowing in bokeh background, petals falling in slow motion, soft rim light, shallow depth of field, anime-real style, 8k',
    model: 'agnes-video-2.5-flash', seconds: '8', ratio: '16:9',
  },
  {
    id: 'tokyo', name: '东京绘梨衣',
    prompt: 'cinematic night shot, a girl with long crimson red hair in white kimono standing in Shibuya crossing, neon billboards reflecting, rain mist, cars light trails, cinematic slow motion, shallow depth of field, anime-real style, 8k',
    model: 'agnes-video-2.5-flash', seconds: '8', ratio: '16:9',
  },
  {
    id: 'jk', name: 'JK 白丝',
    prompt: 'low angle cinematic shot, camera tilt up from a girl\'s white thigh-high socks and legs, up her JK school uniform, ending on her beautiful face with crimson red hair, Tokyo street at dusk, cherry petals, anime-real style, 8k',
    model: 'agnes-video-2.5-flash', seconds: '5', ratio: '16:9',
  },
  {
    id: 'gufeng', name: '古风侠女',
    prompt: 'cinematic wuxia shot, a beautiful girl with long black hair in flowing red hanfu, standing on bamboo forest mountain at misty dawn, wind blowing her sleeves, sword in hand, soft golden light, shallow depth of field, anime-real style, 8k',
    model: 'agnes-video-2.5-flash', seconds: '8', ratio: '16:9',
  },
]
const selectedTheme = ref('sakura')
const customTheme = ref('')
function applyTheme(p) {
  selectedTheme.value = p.id
  text.value = p.prompt
  agenModel.value = p.model
  videoSeconds.value = p.seconds
  videoSpec.value = p.ratio === '9:16' ? 'portrait-720' : 'landscape-720'
}
function onThemeChange() {
  if (selectedTheme.value === 'custom') {
    text.value = customTheme.value
  } else {
    const p = themePresets.find(x => x.id === selectedTheme.value)
    if (p) applyTheme(p)
  }
}

// ---- 生成平台 API Key（localStorage 持久化）----
const KEY_STORE = 'studio_platform_keys'
const platformKeys = ref([
  { id: 'jimeng', label: '即梦', placeholder: '即梦 API Key / Cookie', show: false },
  { id: 'hailuo', label: '海螺 MiniMax', placeholder: '海螺 API Key', show: false },
  { id: 'agnes', label: 'Agnes', placeholder: 'Agnes API Key', show: false },
  { id: 'kling', label: 'Kling', placeholder: 'Kling API Key（留空则每日白嫖）', show: false },
])
const keysOpen = ref(false)
const platformKeysMap = ref(loadPlatformKeys())
const platformKeyCount = computed(() => Object.values(platformKeysMap.value).filter(v => v && v.trim()).length)

function loadPlatformKeys() {
  try {
    const raw = localStorage.getItem(KEY_STORE)
    if (raw) return JSON.parse(raw)
  } catch (e) {}
  return {}
}
// 监听变化持久化
watch(platformKeysMap, (v) => {
  localStorage.setItem(KEY_STORE, JSON.stringify(v))
}, { deep: true })

function saveKey() {
  localStorage.setItem('pexels_key', pexelsKey.value.trim())
  keySaved.value = true
  setTimeout(() => keySaved.value = false, 2000)
}

const SEG_COLORS = ['#ff6b6b', '#4ecdc4', '#ffd93d', '#6c5ce7', '#00b894', '#fd79a8', '#74b9ff', '#e17055']

function segW(seg) {
  const total = result.value?.duration || 1
  return Math.max(8, (seg.duration / total) * 100) + '%'
}
function srcShort(src) {
  if (!src) return ''
  const s = String(src)
  if (s.includes('Bing')) return '🌐 联网素材'
  if (s.includes('素材池')) return '📁 本地素材'
  return '🎨 动态背景'
}
function fmtDur(d) {
  const m = Math.floor(d / 60), s = Math.round(d % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}
function pushLog(l) { logLines.value.push(l) }

async function doTranslate() {
  const src = text.value.trim()
  if (!src) { transError.value = '× 先在上方贴入要翻译的文章'; return }
  transBusy.value = true
  transError.value = ''
  transResult.value = ''
  try {
    const resp = await fetch(`${API_BASE_URL}/api/translate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text: src, target_lang: transLang.value })
    })
    const data = await resp.json()
    if (!resp.ok || !data.ok) {
      transError.value = '× ' + (data.error || resp.status)
      return
    }
    transResult.value = data.translated
  } catch (e) {
    transError.value = '× 请求失败：' + e.message
  } finally {
    transBusy.value = false
  }
}

// 显示一条临时提示（错误/成功）
function showToast(msg, duration = 3200) {
  toastMsg.value = msg
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toastMsg.value = '' }, duration)
}

async function generate() {
  const prompt = text.value.trim()
  if (!prompt) { showToast('× 请填写提示词或选个主题'); return }
  busy.value = true
  result.value = null
  logLines.value = []
  // 生成计时
  elapsed.value = 0
  clearInterval(elapsedTimer)
  elapsedTimer = setInterval(() => { elapsed.value++ }, 1000)
  // 校验 seed：非空且为有效整数时才透传，否则随机
  let seed = Math.floor(Math.random() * 999999)
  if (seedInput.value) {
    const n = parseInt(seedInput.value, 10)
    if (!isNaN(n)) seed = n
  }
  const base = {
    prompt,
    ref_image: refMode.value === 'reference' ? (selectedRef.value?.src || '') : '',
    first_frame: refMode.value === 'keyframe' ? (selectedRef.value?.src || '') : '',
    last_frame: refMode.value === 'keyframe' ? (lastFrameRef.value?.src || '') : '',
    model: agenModel.value,
    seconds: videoSeconds.value,
    ratio: videoSpec.value.startsWith('portrait') ? '9:16' : '16:9',
    size: videoSpec.value.endsWith('1080') ? '1080P' : '720P',
    seed,
  }
  // 超长视频：走链式生成（多段递进 + 抽帧接续 + 拼接成片）
  const isChain = refMode.value === 'long'
  const endpoint = isChain ? '/api/studio/agnes/chain' : '/api/studio/agnes'
  if (isChain) {
    base.ref_image = selectedRef.value?.src || '' // 首段从参考图起
    delete base.first_frame
    delete base.last_frame
  }
  pushLog(isChain ? `Ameko 链式生成 ${segments.value} 段长视频中…` : 'Ameko 视频生成中…')
  try {
    const resp = await fetch(`${API_BASE_URL}${endpoint}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(isChain ? { ...base, segments: segments.value } : base)
    })
    const data = await resp.json()
    if (!resp.ok || !data.ok) {
      const err = data.error || `提交失败 (${resp.status})`
      pushLog('× ' + err)
      showToast('× ' + err)
      busy.value = false
      clearInterval(elapsedTimer)
      return
    }
    if (!data.task_id) {
      const err = '后端未返回任务 ID'
      pushLog('× ' + err)
      showToast('× ' + err)
      busy.value = false
      clearInterval(elapsedTimer)
      return
    }
    // 异步任务：存 localStorage（跳走聊天回来能恢复），开始轮询
    taskId.value = data.task_id
    localStorage.setItem('studio_agnes_task', JSON.stringify({ task_id: data.task_id, ts: Date.now() }))
    pushLog('已提交后台生成，等待期间可以去和 Ameko 聊天')
    pollTask()
  } catch (e) {
    const err = '提交失败：' + (e?.message || String(e))
    pushLog('× ' + err)
    showToast('× ' + err)
    busy.value = false
    clearInterval(elapsedTimer)
  }
}
// 轮询异步任务状态
let pollTimer = null
async function pollTask() {
  const id = taskId.value
  if (!id) return
  clearInterval(pollTimer)
  const stopAll = () => {
    clearInterval(elapsedTimer)
    clearInterval(pollTimer)
    busy.value = false
    localStorage.removeItem('studio_agnes_task')
  }
  const check = async () => {
    try {
      const r = await fetch(`${API_BASE_URL}/api/studio/agnes/status/${id}`)
      const d = await r.json().catch(() => ({}))
      if (!r.ok || d.status === 'lost') {
        // 任务记录丢失（应用重启/热更新换过进程）：必须明确报错并退出「生成中」，
        // 否则界面永远停在等待态，用户看到的就是点了没反应、也没有任何错误反馈。
        const err = d.error || `任务查询失败 (${r.status})`
        pushLog('× ' + err)
        showToast('× ' + err)
        stopAll()
        return
      }
      if (d.status === 'done') {
        result.value = { video: d.video, name: d.name || '', size: d.size || '', seconds: d.seconds || 0 }
        pushLog(`视频生成完成：${result.value.name}（用时 ${elapsed.value}s）`)
        stopAll()
      } else if (d.status === 'failed') {
        const err = d.error || '生成失败'
        pushLog('× ' + err)
        showToast('× ' + err)
        stopAll()
      } else if (elapsed.value > 720) {
        // 兜底超时：后端单次生成上限 6 分钟，超过 12 分钟仍 pending 视为已中断。
        const err = '生成超时，任务可能已中断，请重新生成'
        pushLog('× ' + err)
        showToast('× ' + err)
        stopAll()
      }
    } catch (e) {
      // 轮询接口异常时不打断用户，但累计多次后给出提示
      pushLog('× 轮询状态失败：' + (e?.message || String(e)))
    }
  }
  check()
  pollTimer = setInterval(check, 10000)
}

async function genShot(i) {
  const shot = plan.value.shots[i]
  if (!shot || shot.status === 'busy') return
  shot.status = 'busy'
  pushLog(`🎥 生成镜头 ${shot.shot_no}（${shot.platform}）…`)
  try {
    const resp = await fetch(`${API_BASE_URL}/api/studio/manga/shot`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        shot_no: shot.shot_no,
        prompt: shot.prompt,
        platform: genPlatform.value === 'auto' ? shot.platform : genPlatform.value,
        ref_image: ''
      })
    })
    const data = await resp.json()
    if (resp.ok && data.ok) {
      shot.status = 'done'
      pushLog(`镜头 ${shot.shot_no} 已提交生成`)
    } else {
      shot.status = ''
      pushLog('× ' + (data.error || '生成失败'))
    }
  } catch (e) {
    shot.status = ''
    pushLog('× 请求失败：' + e.message)
  }
}

function move(i, dir) {
  const j = i + dir
  if (j < 0 || j >= segs.value.length) return
  const arr = segs.value
  ;[arr[i], arr[j]] = [arr[j], arr[i]]
}
function remove(i) {
  segs.value.splice(i, 1)
}

onMounted(async () => {
  document.title = '短剧工作台'
  // 素材库：动态扫描本地素材目录（不硬编码）
  try {
    const r = await fetch(API_BASE_URL + '/api/studio/library')
    if (r.ok) {
      const d = await r.json()
      if (d.assets?.length) libraryAssets.value = d.assets.map(a => ({...a, src: backendURL(a.src)}))
    }
  } catch (e) { /* 后端未启动则保持空 */ }
  // 恢复未完成的异步生成任务（跳走去聊天后回来）
  try {
    const saved = JSON.parse(localStorage.getItem('studio_agnes_task') || 'null')
    if (saved?.task_id) {
      taskId.value = saved.task_id
      busy.value = true
      elapsed.value = Math.floor((Date.now() - saved.ts) / 1000)
      elapsedTimer = setInterval(() => { elapsed.value++ }, 1000)
      pollTask()
    }
  } catch (e) {}
})
// 组件卸载时清理定时器，避免跳走后轮询把 localStorage 任务记录清掉
onUnmounted(() => {
  clearInterval(pollTimer)
  clearInterval(elapsedTimer)
})
</script>

<style scoped>
.studio-shell { min-height: 100vh; background: #f7f8fa; color: #1f2329; display: flex; flex-direction: row; }
/* 左侧导航（即梦式） */
.studio-sidebar {
  width: 64px; flex: 0 0 64px; display: flex; flex-direction: column; align-items: center;
  padding: 12px 0; background: var(--app-surface); border-right: 1px solid var(--app-border-soft);
  position: sticky; top: 0; height: 100vh; box-sizing: border-box;
}
.side-brand { padding: 6px 0 14px; }
.side-menu { display: flex; flex-direction: column; gap: 4px; align-items: center; }
.side-nav {
  width: 40px; height: 40px; border: none; border-radius: 10px; background: transparent;
  color: var(--app-text-soft); cursor: pointer; transition: all .15s;
  display: flex; align-items: center; justify-content: center;
}
.side-nav:hover { background: var(--app-bg); }
.side-nav.active { background: color-mix(in srgb, var(--app-accent) 10%, transparent); color: var(--app-accent); }
.side-bottom { margin-top: auto; display: flex; flex-direction: column; gap: 8px; align-items: center; }
.side-avatar { width: 30px; height: 30px; border-radius: 50%; border: none; background: var(--app-bg); font-size: 13px; cursor: pointer; }
.side-tool {
  width: 34px; height: 34px; border: none; border-radius: 8px; background: transparent;
  color: var(--app-text-soft); cursor: pointer; display: flex; align-items: center; justify-content: center;
  transition: all .15s;
}
.side-tool:hover { background: var(--app-bg); color: var(--app-text); }
/* 主区 */
.studio-main {
  flex: 1; display: flex; flex-direction: column; padding: 20px 36px 40px; gap: 16px;
  max-width: 1400px; width: 100%; box-sizing: border-box; margin: 0 auto; overflow-y: auto; position: relative;
}
.studio-top { position: absolute; top: 22px; right: 36px; display: flex; justify-content: flex-end; align-items: center; }
.studio-content { display: flex; flex-direction: column; gap: 28px; }
.top-btn {
  border: none; background: #4f7cff; color: #fff; border-radius: 10px; padding: 10px 18px;
  font-size: 13.5px; font-weight: 600; cursor: pointer; display: inline-flex; align-items: center; gap: 7px;
}
.top-assets {
  border: 1px solid #e4e7ec; background: #fff; color: #1f2329; border-radius: 10px;
  padding: 10px 16px; font-size: 13px; font-weight: 500; cursor: pointer; display: inline-flex; align-items: center; gap: 6px;
}
/* 创作区 */
.create-wrap { flex: 1; display: flex; flex-direction: column; gap: 22px; }
.create-hello { font-size: 30px; font-weight: 700; margin: 26px 0 4px; }
.template-row { display: grid; grid-template-columns: repeat(3, 1fr); gap: 32px; }
.template-card {
  border-radius: 16px; overflow: hidden; cursor: pointer;
  border: 1px solid #eef0f3; background: #fff; box-shadow: 0 1px 6px rgba(0,0,0,.05);
  transition: transform .18s ease, box-shadow .18s ease;
}
.template-card:hover { transform: translateY(-3px); box-shadow: 0 10px 28px rgba(0,0,0,.1); }
.template-img-wrap { position: relative; aspect-ratio: 16/9; overflow: hidden; }
.template-thumb { width: 100%; height: 100%; object-fit: cover; display: block; transition: transform .3s ease; }
.template-card:hover .template-thumb { transform: scale(1.04); }
.template-model-tag {
  position: absolute; left: 12px; bottom: 12px; font-size: 11px; color: #fff;
  background: rgba(0,0,0,.5); padding: 4px 10px; border-radius: 999px; backdrop-filter: blur(4px);
}
.template-try {
  position: absolute; right: 12px; bottom: 12px; font-size: 12px; font-weight: 700;
  padding: 6px 14px; border-radius: 999px; background: #1f2329; color: #fff;
  transition: background .15s;
}
.template-card:hover .template-try { background: #4f7cff; }
.template-info { padding: 14px 16px 16px; }
.template-title { font-size: 14.5px; font-weight: 700; display: block; color: #1f2329; margin-bottom: 6px; }
.template-desc { font-size: 11.5px; color: #9aa1ab; line-height: 1.5; display: block; }
.compose-errors {
  display: flex; flex-direction: column; gap: 6px;
  background: #fff0f0; border: 1px solid #ffc9c9; border-radius: 12px;
  padding: 10px 14px; margin-bottom: 12px;
}
.compose-error-line {
  font-size: 12.5px; color: #c23a3a; line-height: 1.5;
}
/* 底部输入框 */
.compose-box {
  display: flex; gap: 14px; align-items: stretch; background: #fff;
  border: 1px solid #e4e7ec; border-radius: 18px; padding: 16px; box-shadow: 0 4px 18px rgba(0,0,0,.05);
}
.compose-ref { width: 96px; flex: 0 0 96px; }
.ref-slot {
  width: 96px; height: 96px; border: 1.5px dashed #d8dbe1; border-radius: 14px;
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  color: #9aa1ab; cursor: pointer; gap: 8px; font-size: 12px; position: relative; box-sizing: border-box;
}
.ref-slot span { font-size: 10.5px; }
.ref-slot.ref-selected { border-style: solid; border-color: var(--app-accent); }
.ref-thumb { width: 100%; height: 100%; object-fit: cover; border-radius: 12px; }
.ref-remove {
  position: absolute; right: -6px; top: -6px; width: 18px; height: 18px; border-radius: 50%;
  background: #e05252; color: #fff; font-size: 10px; display: flex; align-items: center; justify-content: center;
}
.compose-main { flex: 1; display: flex; flex-direction: column; }
.compose-input {
  flex: 1; border: none; outline: none; background: transparent; resize: none;
  font-size: 14px; color: #1f2329; min-height: 70px; font-family: inherit; line-height: 1.7;
}
.compose-toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; padding-top: 8px; border-top: 1px solid #f2f3f5; }
.toolbar-right { display: flex; align-items: center; gap: 10px; margin-left: auto; }
.tool-mic {
  width: 34px; height: 34px; border-radius: 50%; border: none; background: #f5f6f8;
  color: #5b6270; cursor: pointer; display: flex; align-items: center; justify-content: center;
}
.tool-chip {
  font-size: 12px; padding: 7px 13px; border-radius: 999px; background: #f5f6f8;
  color: #5b6270; white-space: nowrap; display: inline-flex; align-items: center; gap: 5px;
}
.tool-select {
  border: 1px solid #e4e7ec; border-radius: 9px; background: #fff; color: #1f2329;
  font-size: 12.5px; padding: 7px 10px;
}
.seed-input {
  border: 1px solid #e4e7ec; border-radius: 9px; background: #fff; color: #1f2329;
  font-size: 12.5px; padding: 7px 10px; width: 90px; outline: none;
}
.compose-send {
  width: 44px; height: 44px; border-radius: 50%; border: none; background: #3fd0a0;
  color: #fff; cursor: pointer; display: flex; align-items: center; justify-content: center; margin-left: auto;
}
.compose-send:disabled { opacity: .5; cursor: not-allowed; }
/* 资产 */
.assets-head h2 { font-size: 20px; font-weight: 700; margin: 8px 0 2px; }
.assets-sub { font-size: 13px; color: #9aa1ab; display: block; margin-bottom: 12px; }
.assets-filter { display: flex; align-items: center; gap: 8px; padding: 12px 0; border-bottom: 1px solid var(--app-border-soft); }
.filter-tab { border: none; background: transparent; color: var(--app-text-soft); font-size: 13px; padding: 6px 12px; border-radius: 8px; cursor: pointer; }
.filter-tab.active { background: var(--app-bg); color: var(--app-text); font-weight: 600; }
.filter-divider { width: 1px; height: 16px; background: var(--app-border-soft); margin: 0 4px; }
.filter-drop { font-size: 12.5px; color: var(--app-text-soft); cursor: pointer; padding: 4px 8px; }
.assets-section { margin-top: 18px; }
.section-date { font-size: 15px; font-weight: 700; margin-bottom: 12px; }
.assets-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 16px; }
.asset-card { border: 1px solid var(--app-border-soft); border-radius: 14px; overflow: hidden; background: var(--app-surface); cursor: pointer; transition: transform .15s; }
.asset-card:hover { transform: translateY(-2px); }
.asset-thumb-wrap { position: relative; aspect-ratio: 16/9; }
.asset-thumb { width: 100%; height: 100%; object-fit: cover; display: block; background: #000; }
.asset-dur {
  position: absolute; left: 8px; bottom: 8px; font-size: 11px; color: #fff;
  background: rgba(0,0,0,.55); padding: 2px 7px; border-radius: 6px;
}
.asset-ops {
  position: absolute; right: 8px; top: 8px; display: none; gap: 4px;
}
.asset-card:hover .asset-ops { display: flex; }
.op-btn {
  width: 24px; height: 24px; border: none; border-radius: 6px; background: rgba(0,0,0,.55);
  color: #fff; font-size: 12px; cursor: pointer; display: flex; align-items: center; justify-content: center;
}
.op-btn:hover { background: rgba(0,0,0,.8); }
.asset-name { font-size: 12.5px; font-weight: 600; padding: 9px 12px; }
.asset-card.selected { border-color: var(--app-accent); box-shadow: 0 0 0 2px color-mix(in srgb, var(--app-accent) 25%, transparent); }
.asset-check {
  position: absolute; right: 8px; top: 8px; width: 20px; height: 20px; border-radius: 50%;
  background: var(--app-accent); color: #fff; font-size: 12px; font-weight: 700;
  display: flex; align-items: center; justify-content: center;
}
.studio-toast {
  position: fixed; top: 18px; left: 50%; transform: translateX(-50%); z-index: 100;
  background: var(--app-text); color: var(--app-bg); padding: 10px 22px; border-radius: 999px;
  font-size: 13px; font-weight: 600; box-shadow: 0 4px 16px rgba(0,0,0,.18);
}
.toast-enter-active, .toast-leave-active { transition: all .25s ease; }
.toast-enter-from, .toast-leave-to { opacity: 0; transform: translate(-50%, -8px); }
/* 素材预览弹窗（即梦式） */
.asset-preview-modal {
  position: fixed; inset: 0; z-index: 200; background: rgba(0,0,0,.5);
  display: flex; align-items: center; justify-content: center; padding: 40px;
}
.asset-preview {
  background: #fff; border-radius: 18px; overflow: hidden; max-width: 1200px; width: 100%;
  display: flex; min-height: 480px; box-shadow: 0 20px 60px rgba(0,0,0,.3);
}
.preview-left { flex: 1; background: #000; display: flex; align-items: center; justify-content: center; position: relative; }
.preview-video { width: 100%; height: 100%; max-height: 600px; display: block; }
.preview-right { width: 340px; flex: 0 0 340px; display: flex; flex-direction: column; padding: 14px; box-sizing: border-box; overflow-y: auto; }
.preview-head { display: flex; align-items: center; gap: 8px; margin-bottom: 14px; }
.preview-head-left { flex: 1; }
.pv-icon-btn {
  width: 30px; height: 30px; border: none; border-radius: 8px; background: #f5f6f8;
  color: #1f2329; cursor: pointer; display: inline-flex; align-items: center; justify-content: center;
}
.pv-download {
  display: inline-flex; align-items: center; gap: 6px; border: none; background: #1f2329; color: #fff;
  border-radius: 8px; padding: 7px 14px; font-size: 12.5px; font-weight: 600; cursor: pointer;
}
.pv-prompt { margin-bottom: 16px; }
.pv-label { font-size: 12px; color: #9aa1ab; margin-bottom: 6px; }
.pv-prompt-text { font-size: 13.5px; color: #1f2329; line-height: 1.6; margin: 0 0 8px; }
.pv-meta { font-size: 12px; color: #5b6270; margin-bottom: 8px; }
.pv-detail { font-size: 12px; color: #4f7cff; display: inline-flex; align-items: center; gap: 4px; cursor: pointer; }
.pv-actions { display: flex; flex-direction: column; gap: 8px; }
.pv-act {
  display: flex; align-items: center; gap: 8px; width: 100%; border: 1px solid #eef0f3;
  background: #fff; color: #1f2329; border-radius: 10px; padding: 10px 12px; font-size: 12.5px;
  cursor: pointer; transition: background .15s;
}
.pv-act:hover { background: #f7f8fa; }
.pv-act-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
/* 参考素材选择弹窗 */
.ref-picker-modal {
  position: fixed; inset: 0; z-index: 350; background: rgba(0,0,0,.4);
  display: flex; align-items: center; justify-content: center;
}
.ref-picker {
  background: var(--app-surface); border-radius: 16px; width: 560px; max-width: 92vw;
  max-height: 80vh; display: flex; flex-direction: column; box-shadow: 0 20px 60px rgba(0,0,0,.3); overflow: hidden;
}
.ref-picker-head {
  display: flex; align-items: center; gap: 10px; padding: 14px 18px; border-bottom: 1px solid var(--app-border-soft);
}
.ref-picker-title { font-size: 15px; font-weight: 700; flex: 1; }
.ref-picker-upload {
  border: 1px solid var(--app-accent); color: var(--app-accent); background: transparent;
  border-radius: 9px; padding: 6px 14px; font-size: 12.5px; font-weight: 600; cursor: pointer;
}
.ref-picker-close {
  width: 28px; height: 28px; border: none; border-radius: 8px; background: var(--app-bg);
  color: var(--app-text); cursor: pointer; display: flex; align-items: center; justify-content: center;
}
.ref-picker-grid {
  display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; padding: 16px; overflow-y: auto;
}
.ref-picker-item { cursor: pointer; border: 1px solid var(--app-border-soft); border-radius: 12px; overflow: hidden; transition: border-color .15s; }
.ref-picker-item:hover { border-color: var(--app-accent); }
.ref-picker-thumb-wrap { position: relative; aspect-ratio: 16/9; background: #000; }
.ref-picker-thumb { width: 100%; height: 100%; object-fit: cover; display: block; }
.ref-picker-dur {
  position: absolute; left: 6px; bottom: 6px; font-size: 10px; color: #fff;
  background: rgba(0,0,0,.55); padding: 1px 6px; border-radius: 5px;
}
.ref-picker-name {
  display: block; font-size: 11px; color: var(--app-text-soft); padding: 7px 10px;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
/* 主题选择 + 生成结果 */
.compose-theme-row { display: flex; gap: 8px; align-items: center; margin-bottom: 6px; }
.theme-select {
  border: 1px solid var(--app-border-soft); border-radius: 9px; background: var(--app-bg);
  color: var(--app-text); font-size: 12.5px; padding: 6px 10px;
}
.theme-custom-input {
  flex: 1; border: 1px solid var(--app-border-soft); border-radius: 9px; background: var(--app-bg);
  color: var(--app-text); font-size: 12.5px; padding: 6px 10px; outline: none;
}
.gen-result { margin-top: 8px; }
/* 后台任务条（Hermes 同款：输入框上方） */
.gen-taskbar {
  background: var(--app-surface); border: 1px solid var(--app-border-soft); border-radius: 14px;
  padding: 12px 14px; margin-bottom: 4px;
}
.gen-taskbar-head { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.gen-taskbar-title { font-size: 13.5px; font-weight: 700; display: inline-flex; align-items: center; gap: 6px; }
.gen-taskbar-status { font-size: 12px; color: var(--app-accent); font-weight: 600; }
.gen-taskbar-status.done { color: #3fd0a0; }
.gen-taskbar-clear {
  margin-left: auto; width: 24px; height: 24px; border: none; border-radius: 7px;
  background: var(--app-bg); color: var(--app-text-soft); cursor: pointer; display: flex; align-items: center; justify-content: center;
}
.gen-taskbar-body { display: flex; flex-direction: column; gap: 8px; }
.gen-taskbar-prompt { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.gen-taskbar-ref { width: 52px; height: 52px; object-fit: cover; border-radius: 9px; flex-shrink: 0; }
.gen-taskbar-text {
  font-size: 12.5px; color: var(--app-text); line-height: 1.6; overflow: hidden;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical;
}
.gen-taskbar-meta { display: flex; align-items: center; gap: 16px; }
.gen-taskbar-result { border-radius: 10px; overflow: hidden; background: #000; }
.gen-video { width: 100%; display: block; }
.gen-video-meta { font-size: 11.5px; color: var(--app-text-faint); padding: 8px 12px; }
/* 生成整页视图（即梦式：参考图 + 提示词 + 进度） */
.gen-view {
  display: flex; gap: 24px; flex: 1; background: var(--app-surface);
  border: 1px solid var(--app-border-soft); border-radius: 18px; padding: 20px;
}
.gen-view-preview { flex: 1.2; border-radius: 14px; overflow: hidden; background: #000; min-height: 320px; display: flex; align-items: center; justify-content: center; }
.gen-view-ref { width: 100%; height: 100%; object-fit: contain; display: block; }
.gen-view-ref-placeholder { color: var(--app-accent); display: flex; flex-direction: column; align-items: center; gap: 12px; }
.gen-view-ref-hint { font-size: 13px; color: #9aa1ab; text-align: center; line-height: 1.7; }
.gen-view-info { flex: 1; display: flex; flex-direction: column; gap: 12px; }
.gen-view-prompt {
  font-size: 13.5px; color: var(--app-text); background: var(--app-bg); border-radius: 12px;
  padding: 12px 14px; line-height: 1.7; white-space: pre-wrap; word-break: break-word;
}
.gen-view-loading { display: flex; flex-direction: column; gap: 10px; flex: 1; justify-content: center; }
.gen-chat-link {
  display: inline-flex; align-items: center; gap: 6px; color: var(--app-accent);
  font-size: 13px; font-weight: 600; text-decoration: none; padding: 8px 0;
}
.gen-chat-link:hover { text-decoration: underline; }
.gen-view-result { display: flex; flex-direction: column; gap: 10px; }
.gen-view-actions { display: flex; gap: 10px; }
.gen-view-regen {
  border: 1px solid var(--app-border-soft); background: var(--app-bg); color: var(--app-text);
  border-radius: 10px; padding: 9px 16px; font-size: 12.5px; font-weight: 600; cursor: pointer;
  display: inline-flex; align-items: center; gap: 6px;
}
.gen-view-regen:hover { border-color: var(--app-accent); color: var(--app-accent); }
.gen-result-card {
  background: var(--app-surface); border: 1px solid var(--app-border-soft); border-radius: 16px;
  padding: 16px; margin-top: 10px;
}
.gen-card-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.gen-card-title { font-size: 14px; font-weight: 700; display: inline-flex; align-items: center; gap: 6px; }
.gen-card-status { font-size: 12px; color: var(--app-accent); font-weight: 600; }
.gen-card-status.done { color: #3fd0a0; }
.gen-card-prompt {
  font-size: 13px; color: var(--app-text); background: var(--app-bg); border-radius: 10px;
  padding: 10px 12px; line-height: 1.6; margin-bottom: 10px; white-space: pre-wrap; word-break: break-word;
}
.gen-card-params { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; }
.gen-param {
  font-size: 11.5px; color: var(--app-text-soft); background: var(--app-bg);
  border-radius: 999px; padding: 4px 10px;
}
.gen-card-loading { display: flex; flex-direction: column; gap: 8px; align-items: center; padding: 20px 0; }
.gen-shimmer-bar { width: 100%; height: 4px; border-radius: 2px; background: var(--app-bg); overflow: hidden; }
.gen-shimmer { width: 40%; height: 100%; background: linear-gradient(90deg, transparent, var(--app-accent), transparent); animation: genSlide 1.2s infinite; border-radius: 2px; }
.gen-loading-text { font-size: 12.5px; color: var(--app-text-soft); }
.gen-video-meta { font-size: 11.5px; color: var(--app-text-faint); padding: 8px 12px; }
.gen-loading { font-size: 13px; color: var(--app-text-soft); padding: 12px 0; }
.gen-video-wrap { border-radius: 14px; overflow: hidden; border: 1px solid var(--app-border-soft); background: #000; }
.gen-video { width: 100%; display: block; }
@keyframes genSlide { 0% { transform: translateX(-100%); } 100% { transform: translateX(350%); } }
/* 删除确认弹窗（轻量） */
.confirm-modal {
  position: fixed; inset: 0; z-index: 300; background: rgba(0,0,0,.35);
  display: flex; align-items: center; justify-content: center;
}
.confirm-box {
  background: var(--app-surface); border-radius: 14px; padding: 22px; width: 320px;
  box-shadow: 0 12px 40px rgba(0,0,0,.25); text-align: center;
}
.confirm-title { font-size: 15px; font-weight: 700; margin-bottom: 10px; }
.confirm-text { font-size: 13px; color: var(--app-text-soft); margin: 0 0 18px; line-height: 1.6; }
.confirm-actions { display: flex; gap: 10px; justify-content: center; }
.confirm-cancel {
  border: 1px solid var(--app-border-soft); background: transparent; color: var(--app-text);
  border-radius: 9px; padding: 8px 20px; font-size: 13px; cursor: pointer;
}
.confirm-danger {
  border: none; background: #e05252; color: #fff; border-radius: 9px; padding: 8px 20px;
  font-size: 13px; font-weight: 600; cursor: pointer;
}
/* 结果区 */
.studio-result-panel { background: #fff; border: 1px solid #eef0f3; border-radius: 16px; padding: 18px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; font-weight: 700; font-size: 14.5px; margin-bottom: 12px; }
.panel-hint { font-size: 12px; color: #9aa1ab; font-weight: 400; }
.empty-state { text-align: center; padding: 46px 20px; color: #9aa1ab; font-size: 13px; line-height: 1.8; }
.empty-art { font-size: 34px; color: #4f7cff; margin-bottom: 10px; }
.gen-log { background: #f7f8fa; border-radius: 10px; padding: 10px 14px; margin-bottom: 12px; max-height: 140px; overflow-y: auto; }
.log-line { font-size: 12px; color: #5b6270; padding: 2px 0; font-family: monospace; }
.log-line.err { color: #e05252; }
.manga-sec { margin-bottom: 16px; }
.timeline-head { font-size: 13.5px; font-weight: 700; margin-bottom: 10px; }
.char-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
.char-card { border: 1px solid #eef0f3; border-radius: 12px; padding: 12px; background: #f7f8fa; }
.char-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
.char-name { font-weight: 700; font-size: 14px; }
.char-role { font-size: 11px; color: #4f7cff; background: #eef3ff; padding: 2px 8px; border-radius: 999px; }
.char-line { font-size: 12px; color: #5b6270; margin: 3px 0; }
.char-personality { color: #1f2329; }
.char-prompt { margin-top: 8px; font-size: 11px; color: #9aa1ab; background: #fff; border: 1px dashed #e4e7ec; border-radius: 8px; padding: 8px; white-space: pre-wrap; word-break: break-all; }
.shot-list { display: flex; flex-direction: column; gap: 10px; }
.shot-card { border: 1px solid #eef0f3; border-radius: 12px; padding: 12px; background: #f7f8fa; }
.shot-card.busy { border-color: #4f7cff; }
.shot-card.done { border-color: #3fd0a0; }
.shot-top { display: flex; align-items: center; gap: 10px; margin-bottom: 6px; }
.shot-no { font-weight: 700; font-size: 12px; color: #4f7cff; }
.shot-desc { flex: 1; font-size: 13px; }
.shot-done { color: #3fd0a0; }
.shot-video { margin-top: 8px; border-radius: 10px; overflow: hidden; }
.shot-video video { width: 100%; display: block; background: #000; }
.shot-gen-btn { margin-top: 10px; border: 1px solid #4f7cff; color: #4f7cff; background: transparent; border-radius: 9px; padding: 8px 16px; font-size: 12.5px; font-weight: 600; cursor: pointer; }
.shot-gen-btn:disabled { opacity: .5; cursor: not-allowed; }
.export-row { margin-top: 14px; }
.export-path { color: #4f7cff; font-size: 13px; text-decoration: none; }
.spin { animation: spin 1s linear infinite; display: inline-block; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
