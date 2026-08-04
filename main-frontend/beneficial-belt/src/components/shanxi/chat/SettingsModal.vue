<template>
  <Teleport to="body">
    <div class="settings-modal-backdrop" @click="$emit('close')" @keydown.esc="$emit('close')">
      <div class="settings-modal-card" role="dialog" aria-modal="true" aria-label="Rescene 设置" tabindex="-1" @click.stop>
        <div class="settings-modal-header">
          <div class="settings-brand">
            <span class="settings-brand-mark"><Icon icon="lucide:sparkles" width="17" /></span>
            <span class="settings-brand-copy">
              <strong>Rescene</strong>
              <small>偏好设置</small>
            </span>
          </div>
          <div class="settings-header-actions">
            <span class="settings-privacy-badge">
              <Icon icon="lucide:shield-check" width="14" />本地优先
            </span>
            <button class="settings-modal-close" @click="$emit('close')" title="关闭">
              <Icon icon="mdi:close" width="18" />
            </button>
          </div>
        </div>
        <div class="settings-modal-body">
          <!-- 左侧边栏 -->
          <div class="settings-sidebar">
            <div class="settings-nav-label">模型与连接</div>
            <button class="settings-tab" :class="{ on: activeTab === 'models' }" @click="activeTab = 'models'">
              <Icon icon="mdi:brain" width="16" />模型</button>
            <div class="settings-tab-group">
              <button class="settings-tab" :class="{ on: activeTab === 'providers' }" @click="activeTab = 'providers'">
                <Icon icon="mdi:server-network-outline" width="16" />提供方
              </button>
              <div v-show="activeTab === 'providers'" class="settings-subtabs">
                <button
                  class="settings-subtab"
                  :class="{ on: providerSubTab === 'free' }"
                  type="button"
                  @click="providerSubTab = 'free'"
                >
                  <Icon icon="mdi:gift-outline" width="15" />免费模型
                </button>
                <button
                                  class="settings-subtab locked"
                                  type="button"
                                  @click="showCustomLockModal = true"
                                >
                                  <Icon icon="mdi:lock-outline" width="15" />自定义 API
                                </button>
              </div>
            </div>
            <button class="settings-tab" :class="{ on: activeTab === 'aggapi' }" @click="activeTab = 'aggapi'">
              <Icon icon="mdi:api" width="16" />聚合 API</button>
            <div class="settings-nav-label">体验与能力</div>
            <button class="settings-tab" :class="{ on: activeTab === 'appearance' }" @click="activeTab = 'appearance'">
              <Icon icon="mdi:palette-outline" width="16" />外观</button>
            <div class="settings-tab-group">
              <button class="settings-tab" :class="{ on: activeTab === 'mcp' }" @click="activeTab = 'mcp'; loadMCP()">
                <Icon icon="mdi:connection" width="16" />MCP
              </button>
              <div v-show="activeTab === 'mcp'" class="settings-subtabs">
                <button class="settings-subtab" :class="{ on: mcpSubTab === 'local' }" type="button" @click="mcpSubTab = 'local'; loadMCP()">
                  <Icon icon="mdi:laptop" width="15" />本地
                </button>
                <button class="settings-subtab" :class="{ on: mcpSubTab === 'external' }" type="button" @click="mcpSubTab = 'external'; loadMCPRegistry()">
                  <Icon icon="mdi:cloud-outline" width="15" />外部
                </button>
              </div>
            </div>
            <div class="settings-tab-group">
              <button class="settings-tab" :class="{ on: activeTab === 'skills' }" @click="activeTab = 'skills'; loadSkills()">
                <Icon icon="mdi:school-outline" width="16" />Skills
              </button>
              <div v-show="activeTab === 'skills'" class="settings-subtabs">
                <button class="settings-subtab" :class="{ on: skillsSubTab === 'local' }" type="button" @click="skillsSubTab = 'local'; loadSkills()">
                  <Icon icon="mdi:laptop" width="15" />本地
                </button>
                <button class="settings-subtab" :class="{ on: skillsSubTab === 'external' }" type="button" @click="skillsSubTab = 'external'; loadSkillRegistry()">
                  <Icon icon="mdi:cloud-outline" width="15" />外部
                </button>
              </div>
            </div>
            <button class="settings-tab" :class="{ on: activeTab === 'memory' }" @click="activeTab = 'memory'; loadMemoryInject()">
              <Icon icon="mdi:notebook-outline" width="16" />记忆</button>
            <div class="settings-nav-label">账户与产品</div>
            <button class="settings-tab" :class="{ on: activeTab === 'profile' }" @click="activeTab = 'profile'">
              <Icon icon="mdi:account-circle-outline" width="16" />我的</button>
            <button class="settings-tab" :class="{ on: activeTab === 'version' }" @click="activeTab = 'version'; loadVersion()">
              <Icon icon="mdi:update" width="16" />版本</button>
          </div>

          <!-- 右侧内容区 -->
          <div class="settings-content">
            <!-- ========== 模型 ========== -->
            <div v-show="activeTab === 'models'" class="settings-panel">
              <div class="settings-section-title">基础配置</div>
              <div class="settings-section-desc">
                统一模型：一个模型同时处理对话与识图。分开配置：文字对话、识图分析各用各的模型
                （识图模型由你自己选，选错了换成能识图的即可）。候选来自「提供方」里选为可用的模型。
              </div>

              <div class="param-row">
                <span class="param-label">配置方式</span>
                <div class="seg-control">
                  <button class="seg-btn" :class="{ on: modelMode === 'unified' }" type="button" @click="setModelMode('unified')">统一模型</button>
                  <button class="seg-btn" :class="{ on: modelMode === 'split' }" type="button" @click="setModelMode('split')">分开配置</button>
                </div>
              </div>

              <template v-if="modelMode === 'unified'">
                <div class="param-row">
                  <span class="param-label">主模型</span>
                  <select class="model-select" v-model="unifiedModelDraft" @change="setUnifiedModel(unifiedModelDraft)">
                    <option v-if="!chatList.length" value="">先去「提供方」选至少一个可用模型</option>
                    <option v-for="m in chatList" :key="m.value" :value="m.value">
                      {{ m.label }}{{ visionByID[m.value] ? ' · 识图' : '' }}
                    </option>
                  </select>
                </div>
                <div class="settings-section-desc" style="margin-top:6px">
                  标注"识图"的模型声明支持视觉分析（仅供参考）；未标注的也可能能识图，以实际效果为准。
                </div>
                <div class="param-row">
                  <span class="param-label">生图提供商</span>
                  <select class="model-select" v-model="imageProviderDraft" @change="setImageProvider(imageProviderDraft)">
                    <option value="pollinations">Pollinations（免费，无 key，速度快）</option>
                  </select>
                </div>
                <div class="param-row">
                  <span class="param-label">Firecrawl API Key</span>
                  <div class="search-model-row">
                    <input
                      v-model="firecrawlKeyDraft"
                      type="password"
                      class="vendor-key-input"
                      placeholder="fc- 开头的 Firecrawl API Key（联网搜索用）"
                      @keyup.enter="saveFirecrawlKey"
                    />
                    <button class="vendor-key-save" type="button" @click="saveFirecrawlKey">
                      {{ firecrawlKeySet ? '已配置 · 更新' : '保存' }}
                    </button>
                  </div>
                  <span v-if="firecrawlKeySet" class="firecrawl-key-status">✅ 联网搜索已启用（Firecrawl，模型自主触发）</span>
                </div>
                <div class="settings-section-desc" style="margin-top:6px">
                  联网搜索是给模型的一个工具：需要最新信息时它自己决定搜（免费额度 500 次/月，firecrawl.dev 获取 Key）。
                </div>
              </template>

              <template v-else>
                <div class="param-row">
                  <span class="param-label">文字模型</span>
                  <select class="model-select" v-model="textModelDraft" @change="setTextModel(textModelDraft)">
                    <option v-if="!chatList.length" value="">先去「提供方」选至少一个可用模型</option>
                    <option v-for="m in chatList" :key="m.value" :value="m.value">
                      {{ m.label }}{{ visionByID[m.value] ? ' · 识图' : '' }}
                    </option>
                  </select>
                </div>
                <div class="param-row">
                  <span class="param-label">识图模型</span>
                  <select class="model-select" v-model="visionModelDraft" @change="setVisionModel(visionModelDraft)">
                    <option v-if="!visionCapableChatList.length" value="">未配置识图模型</option>
                    <option v-for="m in visionCapableChatList" :key="m.value" :value="m.value">{{ m.label }}</option>
                  </select>
                </div>
                <div v-if="!visionCapableChatList.length" class="settings-section-desc" style="margin-top:6px">
                  未检测到可用模型。请先到「提供方」添加并启用至少一个模型。
                </div>
                <div class="param-row">
                  <span class="param-label">生图提供商</span>
                  <select class="model-select" v-model="imageProviderDraft" @change="setImageProvider(imageProviderDraft)">
                    <option value="pollinations">Pollinations（免费，无 key，速度快）</option>
                    <option value="siliconflow">SiliconFlow（免费额度，需 key）</option>
                  </select>
                </div>
                <div class="param-row">
                  <span class="param-label">Firecrawl API Key</span>
                  <div class="search-model-row">
                    <input
                      v-model="firecrawlKeyDraft"
                      type="password"
                      class="vendor-key-input"
                      placeholder="fc- 开头的 Firecrawl API Key（联网搜索用）"
                      @keyup.enter="saveFirecrawlKey"
                    />
                    <button class="vendor-key-save" type="button" @click="saveFirecrawlKey">
                      {{ firecrawlKeySet ? '已配置 · 更新' : '保存' }}
                    </button>
                  </div>
                  <span v-if="firecrawlKeySet" class="firecrawl-key-status">✅ 联网搜索已启用（Firecrawl，模型自主触发）</span>
                </div>
                <div class="settings-section-desc" style="margin-top:6px">
                  联网搜索是给模型的一个工具：需要最新信息时它自己决定搜（免费额度 500 次/月，firecrawl.dev 获取 Key）。
                </div>
              </template>
            </div>

            <!-- ========== 提供方 ========== -->
            <div v-show="activeTab === 'providers'" class="settings-panel">
              <template v-if="providerSubTab === 'free'">
                <div class="settings-section-title settings-section-title-row">
                  <span>免费模型</span>
                  <button class="auto-sort-btn" type="button" @click="showFreeOrderModal = true">
                    <Icon icon="mdi:auto-fix" width="13" /> Auto 自定义排序
                  </button>
                </div>
                <div class="settings-section-desc">配置提供方的 Key 后，它的全部模型会自动进入聊天下拉框；点击「官网获取 Key」打开官网登录即可免费领取 API Key，粘贴输入框即可使用；免 Key 提供方无需配置。</div>

                <div v-if="loading" class="settings-loading">加载中...</div>
                <template v-else>
                  <div v-for="grp in vendorGroups" :key="grp.vendor" class="vendor-group">
                    <div class="vendor-head">
                      <span class="vendor-logo" :style="{ '--vendor-color': vendorColor(grp.vendor) }">
                        <Icon :icon="vendorIcon(grp.vendor)" width="16" />
                      </span>
                      <span class="vendor-name">{{ grp.vendor }}</span>
                      <span class="vendor-count">{{ grp.items.length }} 个模型</span>
                      <span class="vendor-keystate" :class="{ on: grp.hasKey, free: grp.keyless }">{{ grp.keyless ? '免 Key' : (grp.hasKey ? '已配 Key' : '未配 Key') }}</span>
                      <a v-if="grp.keyUrl && !grp.keyless" class="vendor-key-btn vendor-key-link" :href="grp.keyUrl" target="_blank" rel="noopener" title="打开官网登录即可免费获取 API Key">官网获取 Key ↗</a>
                      <button v-if="!grp.keyless && editingVendor !== grp.vendor" class="vendor-key-btn" @click.stop="startEditVendor(grp)">{{ grp.hasKey ? '改 Key' : '填 Key' }}</button>
                      <button v-else-if="editingVendor === grp.vendor" class="vendor-key-btn" @click.stop="cancelVendorEdit">收起</button>
                    </div>
                    <div v-if="editingVendor === grp.vendor" class="vendor-key-inline">
                                          <template v-if="grp.dualKey">
                                            <input
                                              v-model="vendorKeyDraft"
                                              type="password"
                                              class="vendor-key-input"
                                              placeholder="Token ID"
                                              @keyup.enter="saveVendorKey(grp)"
                                            />
                                            <input
                                              v-model="vendorKeySecretDraft"
                                              type="password"
                                              class="vendor-key-input"
                                              placeholder="Token Secret"
                                              @keyup.enter="saveVendorKey(grp)"
                                            />
                                          </template>
                                          <template v-else>
                                            <input
                                              v-model="vendorKeyDraft"
                                              type="password"
                                              class="vendor-key-input"
                                              :placeholder="grp.hasKey ? '••••••••（留空则不修改）' : '输入 ' + grp.vendor + ' 的 API Key'"
                                              @keyup.enter="saveVendorKey(grp)"
                                            />
                                          </template>
                                          <button class="vendor-key-save" type="button" @click="saveVendorKey(grp)">保存</button>
                                          <button class="vendor-key-cancel" type="button" @click="cancelVendorEdit">取消</button>
                                        </div>
                    <div class="vendor-model-cards">
                      <div v-for="m in grp.items" :key="m.id" class="fm-card" :title="m.note || m.name">
                        <span class="fm-signal" :class="'sig-' + (m.signal == null ? -1 : m.signal)">
                          <i v-for="n in 4" :key="n" :class="{ on: (m.signal == null ? -1 : m.signal) >= n || n === 1 }"></i>
                        </span>
                        <span class="fm-name">{{ m.name }}</span>
                        <span v-if="m.context_window" class="fm-tag">{{ fmtCtx(m.context_window) }}</span>
                      </div>
                    </div>
                  </div>
                  <div class="vendor-thanks">💙 感谢所有免费模型提供方的赞助支持</div>
                </template>
              </template>

              <template v-else>
                <div class="settings-section-title">自定义 API</div>
                <div class="settings-section-desc">一条配置对应一个提供方。保存时会读取它的 /models，并把该提供方的全部模型加入聊天下拉框。</div>

                <div v-if="loading" class="settings-loading">加载中...</div>
                <template v-else>
                  <div v-for="cfg in configs" :key="cfg.id" class="api-config-card">
                    <div class="api-config-row">
                      <span class="api-config-name">{{ cfg.name || '未命名配置' }}</span>
                      <span v-if="cfg.is_default" class="api-config-default-badge">默认</span>
                      <div class="api-config-actions">
                        <button class="api-config-action-btn" :disabled="!!configBusy" @click="refreshConfigModels(cfg)">
                          {{ configBusy === cfg.id ? '获取中...' : '刷新模型' }}
                        </button>
                        <button v-if="!cfg.is_default" class="api-config-action-btn" @click="setDefault(cfg.id)">设为默认</button>
                        <button class="api-config-action-btn" @click="startEdit(cfg)">编辑</button>
                        <button class="api-config-action-btn danger" @click="removeConfig(cfg.id)">删除</button>
                      </div>
                    </div>
                    <div class="api-config-meta">
                      {{ cfg.endpoint }} · {{ providerModelCount(cfg) }} 个模型 · {{ cfg.api_key_set ? '已设置 Key' : '免 Key / 未设置 Key' }}
                    </div>
                  </div>

                  <div v-if="!editingConfig" class="api-config-add-btn" @click="startAdd">
                    <Icon icon="mdi:plus" width="15" /> 添加自定义提供方
                  </div>

                  <div v-else class="api-config-form">
                    <div class="api-preset-row">
                      <span class="api-preset-label">预设模板：</span>
                      <button v-for="p in PRESETS" :key="p.name" class="api-preset-btn" type="button" @click="applyPreset(p)">{{ p.name }}</button>
                    </div>
                    <label class="api-form-field">
                      <span>提供方名称</span>
                      <input v-model="editingConfig.name" type="text" placeholder="比如 DeepSeek" autocomplete="off" />
                    </label>
                    <label class="api-form-field">
                      <span>Endpoint</span>
                      <input v-model="editingConfig.endpoint" type="text" placeholder="https://api.example.com" autocomplete="off" />
                    </label>
                    <label class="api-form-field">
                      <span>API Key</span>
                      <input
                        v-model="editingConfig.api_key"
                        type="password"
                        autocomplete="new-password"
                        :placeholder="editingConfig.api_key_set ? '••••••••（留空则不修改）' : '输入 API Key'"
                      />
                    </label>
                    <div class="api-form-hint">无需逐个填写模型名；系统会从 Endpoint 的 /models 自动获取全部模型。</div>
                    <div class="api-form-actions">
                      <button class="api-form-btn cancel" type="button" @click="cancelEdit">取消</button>
                      <button class="api-form-btn save" type="button" :disabled="!!configBusy" @click="saveConfig">
                        {{ configBusy ? '正在获取模型...' : '保存并添加全部模型' }}
                      </button>
                    </div>
                  </div>
                </template>
              </template>
            </div>

            <!-- ========== 聚合 API ========== -->
            <div v-show="activeTab === 'aggapi'" class="settings-panel">
              <div class="settings-section-title">聚合 API</div>
              <div class="settings-section-desc">你配置的所有模型 key 聚合成一个 OpenAI 兼容端点，任何支持 OpenAI 兼容配置的客户端（Claude Code / Cursor / Codex）填上 Base URL 和 Key 即可使用，自动路由到信号最好的免费模型。</div>
              <div class="agg-api-card" style="margin-top:10px">
                <div class="agg-api-row">
                  <span class="agg-api-label">Base URL</span>
                  <code class="agg-api-code">http://localhost:8080/v1</code>
                  <button class="agg-api-copy" type="button" @click="copyAggText('http://localhost:8080/v1')">复制</button>
                </div>
                <div class="agg-api-row">
                  <span class="agg-api-label">API Key</span>
                  <code class="agg-api-code">sk-rescene-local</code>
                  <button class="agg-api-copy" type="button" @click="copyAggText('sk-rescene-local')">复制</button>
                </div>
              </div>
              <div class="agg-api-tip">已聚合 {{ freeModels.length + customModels.length }} 个模型（免费池 + 自定义）。model 填 <code class="agg-api-code">auto</code> 自动路由，或填任意模型 ID；key 可用 RESCENE_AGG_API_KEY 环境变量修改。</div>
            </div>

            <!-- ========== 外观 ========== -->
            <div v-show="activeTab === 'appearance'" class="settings-panel">
              <div class="settings-section-title">主题</div>
              <div class="settings-section-desc">选择你喜欢的主题色，设置会立即应用到整个界面。</div>
              <div class="param-row" style="align-items: flex-start;">
                <span class="param-label">主题色</span>
                <div class="theme-swatches">
                  <button
                    v-for="[key, p] in colorThemes"
                    :key="key"
                    class="theme-swatch"
                    :class="{ on: theme === key }"
                    type="button"
                    :title="p.label"
                    @click="selectTheme(key)"
                  >
                    <span class="theme-swatch-dot" :style="{ background: p.accent }"></span>
                    <span class="theme-swatch-label">{{ p.label }}</span>
                  </button>
                </div>
              </div>

              <div class="settings-section-title appearance-mode-title">显示模式</div>
              <div class="settings-section-desc">选择亮色、暗色，或自动跟随系统设置。</div>
              <div class="param-row">
                <span class="param-label">界面亮度</span>
                <div class="seg-control">
                  <button
                    v-for="opt in MODE_OPTIONS"
                    :key="opt.value"
                    class="seg-btn"
                    :class="{ on: mode === opt.value }"
                    type="button"
                    @click="mode = opt.value"
                  >{{ opt.label }}</button>
                </div>
              </div>

              <div class="settings-section-title appearance-preview-title">实时预览</div>
              <div class="settings-section-desc">预览会随当前主题色和显示模式同步更新。</div>
              <div
                class="theme-live-preview"
                :style="{ '--preview-accent': selectedTheme.accent, '--preview-accent-soft': selectedTheme.accentSoft }"
              >
                <div class="theme-live-topbar">
                  <span class="theme-live-brand"><Icon icon="lucide:sparkles" width="13" />Rescene</span>
                  <span class="theme-live-status"><i></i>{{ selectedTheme.label }} · {{ currentModeLabel }}</span>
                </div>
                <div class="theme-live-body">
                  <div class="theme-live-sidebar">
                    <span class="on"><Icon icon="lucide:message-square" width="13" />对话</span>
                    <span><Icon icon="lucide:folder" width="13" />项目</span>
                    <span><Icon icon="lucide:settings-2" width="13" />设置</span>
                  </div>
                  <div class="theme-live-main">
                    <div class="theme-live-heading">今天想创造什么？</div>
                    <div class="theme-live-copy">主题色会用于选中状态、按钮和重要提示。</div>
                    <div class="theme-live-message">界面预览已与当前设置同步。</div>
                    <div class="theme-live-composer"><span>输入消息...</span><b><Icon icon="lucide:arrow-up" width="13" /></b></div>
                  </div>
                </div>
              </div>
            </div>

            <!-- ========== MCP ========== -->
            <div v-show="activeTab === 'mcp'" class="settings-panel">
              <template v-if="mcpSubTab === 'local'">
                <div class="settings-section-title">
                  已接入的 MCP
                  <button class="inline-refresh" type="button" @click="loadMCP(true)" title="刷新"><Icon icon="mdi:refresh" width="14" :class="{ spin: mcpLoading }" /></button>
                </div>
                <div class="settings-section-desc">
                  本机配置与已安装的远程 MCP（读自 <code>{{ mcpConfigPath || 'mcp.json' }}</code>）。远程连接由应用内置的 Go 客户端完成。
                </div>
                <div v-if="mcpLoading" class="settings-loading">加载中...</div>
                <template v-else>
                  <div v-if="!mcpServers.length" class="settings-empty">还没有 MCP。可到「外部」从官方 Registry 一键接入。</div>
                  <div v-for="s in mcpServers" :key="s.name" class="entity-card">
                    <div class="entity-head">
                      <Icon :icon="s.transport === 'streamable-http' ? 'mdi:cloud-check-outline' : 'mdi:console'" width="15" />
                      <span class="entity-name">{{ s.registry_name || s.name }}</span>
                      <span class="entity-badge">{{ s.transport === 'streamable-http' ? '远程' : '本机' }}</span>
                      <span class="entity-badge mcp-state" :class="'is-' + s.status">{{ s.status === 'connected' ? '已连接' : '待配置' }}</span>
                      <span class="entity-badge">{{ s.tools.length }} 工具</span>
                    </div>
                    <div class="entity-meta">{{ s.url || (s.command + ' ' + (s.args || []).join(' ')) }}</div>
                    <div v-if="s.tools.length" class="entity-tags">
                      <span v-for="t in s.tools" :key="t" class="entity-tag">{{ t.replace('mcp__' + s.name + '__', '') }}</span>
                    </div>
                    <div v-if="s.source === 'official-registry'" class="skill-actions">
                      <button class="danger" type="button" :disabled="catalogBusy === 'mcp:' + s.name" @click="uninstallMCP(s)">
                        {{ catalogBusy === 'mcp:' + s.name ? '移除中…' : '移除' }}
                      </button>
                    </div>
                  </div>
                </template>
              </template>
              <template v-else>
                <div class="settings-section-title">MCP 官方 Registry</div>
                <div class="settings-section-desc">浏览官方托管目录，只展示可由应用直接连接的 Streamable HTTP 服务；无需 Node、Python 或 npx。</div>
                <div class="catalog-toolbar">
                  <label class="catalog-search">
                    <Icon icon="mdi:magnify" width="16" />
                    <input v-model="mcpRegistryQuery" type="search" placeholder="搜索远程 MCP" @keyup.enter="loadMCPRegistry(true)" />
                  </label>
                  <button class="catalog-search-btn" type="button" @click="loadMCPRegistry(true)">搜索</button>
                </div>
                <div v-if="mcpRegistryLoading" class="settings-loading">正在连接 MCP 官方 Registry…</div>
                <template v-else>
                  <div v-if="!mcpRegistryItems.length" class="settings-empty">没有找到可直接连接的远程 MCP。</div>
                  <div v-for="item in mcpRegistryItems" :key="item.name" class="catalog-card">
                    <div class="catalog-card-main">
                      <div class="entity-head">
                        <Icon icon="mdi:server-network-outline" width="15" />
                        <span class="entity-name">{{ item.title || item.name }}</span>
                        <span class="entity-badge">v{{ item.version }}</span>
                      </div>
                      <div class="catalog-id">{{ item.name }}</div>
                      <div class="catalog-desc">{{ item.description || '该服务未提供说明。' }}</div>
                      <div class="entity-meta">{{ item.url }}</div>
                    </div>
                    <button
                      class="catalog-install-btn"
                      :class="{ installed: item.installed }"
                      type="button"
                      :disabled="item.installed || catalogBusy === 'mcp-install:' + item.name"
                      @click="installMCP(item)"
                    >{{ item.installed ? '已接入' : (catalogBusy === 'mcp-install:' + item.name ? '连接中…' : '接入') }}</button>
                  </div>
                </template>
              </template>
            </div>

            <!-- ========== Skills ========== -->
            <div v-show="activeTab === 'skills'" class="settings-panel">
              <template v-if="skillsSubTab === 'local'">
                <div class="settings-section-title">
                  本地技能库
                  <button class="inline-refresh" type="button" @click="loadSkills(true)" title="刷新"><Icon icon="mdi:refresh" width="14" :class="{ spin: skillsLoading }" /></button>
                </div>
                <div class="settings-section-desc">
                  内置技能随客户端发布；Agent 学到的技能与从外部安装的 <code>SKILL.md</code> 都保存在用户数据目录，并可离线使用。
                </div>
                <div v-if="skillsLoading" class="settings-loading">加载中...</div>
                <template v-else>
                  <div v-if="!skills.length" class="settings-empty">还没有技能。完成复杂工作流后 Agent 会自动学习，也可从「外部」安装。</div>
                  <div v-for="sk in skills" :key="(sk.source || '') + ':' + sk.name" class="entity-card">
                    <div class="entity-head" @click="toggleSkill(sk.name)" style="cursor:pointer">
                      <Icon :icon="sk.source === 'builtin' ? 'mdi:package-variant-closed' : (sk.source === 'external' ? 'mdi:puzzle-outline' : 'mdi:school-outline')" width="15" />
                      <span class="entity-name">{{ sk.name }}</span>
                      <span class="entity-badge" :class="sk.source === 'external' ? 'src-ext' : 'src-learned'">
                        {{ sk.source === 'builtin' ? '内置' : (sk.source === 'external' ? '外部' : '自研') }}
                      </span>
                      <span v-if="sk.source !== 'external'" class="entity-badge skill-status" :class="'is-' + normalizedSkillStatus(sk)">{{ skillStatusLabel(sk) }}</span>
                      <span v-if="sk.source !== 'external'" class="entity-badge">{{ (sk.steps || []).length }} 步</span>
                      <Icon :icon="expandedSkill === sk.name ? 'mdi:chevron-up' : 'mdi:chevron-down'" width="16" style="margin-left:auto" />
                    </div>
                    <div class="entity-meta">{{ sk.description }}</div>
                    <ol v-if="expandedSkill === sk.name && sk.steps && sk.steps.length" class="skill-steps">
                      <li v-for="(st, i) in sk.steps" :key="i">{{ st }}</li>
                    </ol>
                    <div v-if="expandedSkill === sk.name && sk.source !== 'external'" class="skill-detail">
                      <div><b>何时使用</b> {{ sk.trigger || '旧版技能未填写' }}</div>
                      <div><b>如何验证</b> {{ sk.verification || '旧版技能未填写' }}</div>
                    </div>
                    <pre v-else-if="expandedSkill === sk.name && sk.body" class="skill-body">{{ sk.body }}</pre>
                    <div v-if="sk.source === 'learned'" class="skill-actions">
                      <button v-if="isSkillActive(sk)" type="button" @click.stop="setSkillStatus(sk, 'archived')">关闭</button>
                      <button v-else type="button" @click.stop="setSkillStatus(sk, 'active')">恢复启用</button>
                      <button class="danger" type="button" @click.stop="removeSkill(sk)">删除</button>
                    </div>
                  </div>
                </template>
              </template>
              <template v-else>
                <div class="settings-section-title">GitHub 技能仓库</div>
                <div class="settings-section-desc">通过 GitHub 公共 API 浏览 Anthropic、OpenAI 与 Vercel Labs 的技能仓库；安装后文件会完整保存到本地。</div>
                <div class="catalog-toolbar">
                  <select v-model="skillRegistrySource" class="catalog-source" @change="loadSkillRegistry(true)">
                    <option v-for="source in skillRegistrySources" :key="source.id" :value="source.id">{{ source.label }}</option>
                  </select>
                  <label class="catalog-search">
                    <Icon icon="mdi:magnify" width="16" />
                    <input v-model="skillRegistryQuery" type="search" placeholder="筛选技能" @keyup.enter="loadSkillRegistry(true)" />
                  </label>
                  <button class="catalog-search-btn" type="button" @click="loadSkillRegistry(true)">筛选</button>
                </div>
                <div v-if="skillRegistryLoading" class="settings-loading">正在读取 GitHub 技能仓库…</div>
                <template v-else>
                  <div v-if="!skillRegistryItems.length" class="settings-empty">该仓库中没有匹配的技能。</div>
                  <div v-for="item in skillRegistryItems" :key="item.source + ':' + item.path" class="catalog-card">
                    <div class="catalog-card-main">
                      <div class="entity-head">
                        <Icon icon="mdi:github" width="15" />
                        <span class="entity-name">{{ item.name }}</span>
                      </div>
                      <div class="catalog-id">{{ item.path }}</div>
                    </div>
                    <button
                      v-if="!item.installed"
                      class="catalog-install-btn"
                      type="button"
                      :disabled="catalogBusy === 'skill-install:' + item.path"
                      @click="installHostedSkill(item)"
                    >{{ catalogBusy === 'skill-install:' + item.path ? '安装中…' : '安装' }}</button>
                    <button
                      v-else
                      class="catalog-install-btn installed removable"
                      type="button"
                      :disabled="catalogBusy === 'skill-remove:' + item.external_id"
                      @click="uninstallHostedSkill(item)"
                    >{{ catalogBusy === 'skill-remove:' + item.external_id ? '移除中…' : '已安装 · 移除' }}</button>
                  </div>
                </template>
              </template>
            </div>


            <!-- ========== 记忆（当前注入上下文的内容，仿 Claude Memory） ========== -->
            <div v-show="activeTab === 'memory'" class="settings-panel">
              <div class="settings-section-title">记忆</div>
              <div class="param-row" style="align-items: center;">
                <span class="param-label">云端记忆同步</span>
                <label class="param-switch">
                  <input type="checkbox" v-model="memorySyncEnabled" :disabled="memorySyncEnvOverride" @change="saveMemorySyncSetting" />
                  <span class="param-switch-track"></span>
                </label>
                <span class="settings-section-desc" style="flex-basis: 100%; margin: 4px 0 10px;">
                  开启后，记忆（偏好 / 决策 / 索引）会随账号存到云端，换设备登录自动恢复。
                  <template v-if="memorySyncEnvOverride">（当前被部署环境变量 RESCENE_MEMORY_SYNC=off 强制关闭）</template>
                </span>
              </div>
              <div v-if="memoryLoading" class="settings-loading">加载中…</div>
              <template v-else-if="humanReadableMemoryMarkdown">
                <div class="memory-md markdown-body" v-html="renderMarkdown(humanReadableMemoryMarkdown)"></div>
              </template>
              <div v-else class="memory-empty">尚未配置任何记忆。</div>
            </div>

            <!-- ========== 我的（Profile + 自定义指令，仿 Claude Profile） ========== -->
            <div v-show="activeTab === 'profile'" class="settings-panel">
              <div class="settings-section-title">个人资料</div>
              <div class="profile-row">
                <span class="profile-label">头像</span>
                <div class="profile-avatar">{{ (profile.full_name || 'A').trim().charAt(0).toUpperCase() }}</div>
              </div>
              <div class="profile-row">
                <span class="profile-label">昵称</span>
                <input class="profile-input" v-model="profile.full_name" type="text" placeholder="你的名字，AI 会用它称呼你" />
              </div>
              <div class="profile-row">
                <span class="profile-label">你的职业 / 身份</span>
                <input class="profile-input" v-model="profile.work" type="text" placeholder="比如 软件工程师" />
              </div>
              <div class="profile-row">
                <span class="profile-label">账号 UID</span>
                <span v-if="auth.uid.value" class="profile-uid">UID {{ auth.uid.value }}</span>
                <span v-else class="profile-uid faint">登录后永久保留</span>
              </div>
              <div class="profile-row">
                <span class="profile-label">亲密等级</span>
                <div class="profile-intimacy">
                  <span class="intimacy-hearts">{{ heartsText }}</span>
                  <span class="intimacy-level">Lv.{{ intimacyLevel }}</span>
                  <span class="intimacy-progress">
                    <span class="intimacy-progress-bar"><span class="intimacy-progress-fill" :style="{ width: intimacyProgressPct + '%' }"></span></span>
                    <span class="intimacy-progress-text">{{ intimacyProgressPct }}%</span>
                  </span>
                </div>
              </div>

              <div class="settings-section-title" style="margin-top: 18px;">给 AI 的自定义指令</div>
              <div class="settings-section-desc">这些会跨对话注入系统提示词，影响 AI 的语气与行为。</div>
              <textarea class="profile-instructions" v-model="profile.instructions" rows="6" placeholder="例如：用温柔、清晰的语气；理性稳重，不要过度共情。"></textarea>

              <div class="profile-actions">
                <span v-if="profileSaved" class="profile-saved">已保存</span>
                <button class="api-form-btn save" type="button" @click="saveProfile" :disabled="profileSaving">{{ profileSaving ? '保存中…' : '保存' }}</button>
              </div>
            </div>

            <!-- ========== 版本与更新 ========== -->
            <div v-show="activeTab === 'version'" class="settings-panel">
              <div class="settings-section-title">版本与更新</div>
              <div class="settings-section-desc">版本以 GitHub Release 为基准，下载走官网直链。</div>

              <div class="param-row">
                <span class="param-label">当前版本</span>
                <span class="version-value">{{ versionInfo.current_version ? 'v' + versionInfo.current_version : (versionLoading ? '检查中…' : '未知') }}</span>
              </div>
              <div class="param-row">
                <span class="param-label">最新版本</span>
                <span v-if="versionLoading" class="version-value">检查中…</span>
                <span v-else-if="versionInfo.has_update" class="version-value version-new">{{ versionInfo.latest_version }}</span>
                <span v-else class="version-value">已是最新版本</span>
              </div>

              <div class="param-row" style="align-items: flex-start;">
                <span class="param-label">更新内容</span>
                <div v-if="versionLoading" class="settings-loading">检查中…</div>
                <div v-else-if="versionInfo.release_notes" class="update-notes" v-html="renderMarkdown(versionInfo.release_notes)"></div>
                <div v-else class="memory-empty">{{ versionInfo.has_update ? '本次更新没有附带更新说明。' : '—' }}</div>
              </div>

              <div class="profile-actions" style="margin-top: 14px;">
                <button
                  class="api-form-btn save"
                  type="button"
                  @click="openUpdate"
                  :disabled="!versionInfo.has_update || versionOpening"
                >{{ versionOpening ? '正在打开…' : '去官网更新' }}</button>
                <label class="param-switch" style="margin-left: auto;" title="关闭后启动不再检查/弹窗提示更新">
                  <input type="checkbox" v-model="notifyDisabled" @change="onNotifyDisabledChange" />
                  <span class="param-switch-track"></span>
                </label>
                <span class="param-value">不提示版本更新</span>
              </div>
            </div>
          </div>

          <div v-if="errorMsg" class="settings-error">{{ errorMsg }}</div>
        </div>
      </div>
    </div>
    <FreeOrderModal v-if="showFreeOrderModal" :openid="props.openid" @close="showFreeOrderModal = false" />
      </Teleport>

      <!-- 自定义 API 解锁弹窗 -->
      <Teleport to="body">
        <div v-if="showCustomLockModal" class="mm-backdrop" @click.self="showCustomLockModal = false">
          <div class="mm-card" style="max-width:380px;text-align:center">
            <div style="font-size:20px;margin-bottom:4px">🔒</div>
            <div class="mm-title">答应我，不白嫖的请划走</div>
            <div style="margin:12px 0">
              <input
                v-model="customLockKey"
                type="password"
                class="mm-input"
                placeholder="开发者密码"
                @keyup.enter="unlockCustom"
              />
            </div>
            <div v-if="customLockError" class="mm-error">{{ customLockError }}</div>
            <div class="mm-actions">
              <button class="mm-btn mm-btn-cancel" type="button" @click="showCustomLockModal = false">划走</button>
              <button class="mm-btn mm-btn-primary" type="button" @click="unlockCustom">解锁</button>
            </div>
          </div>
        </div>
      </Teleport>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import { theme, mode, MODE_OPTIONS, THEME_PRESETS } from '../composables/useTheme.js'

import { renderMarkdown } from './markdownRenderer.js'
import { isUpdateNotifyDisabled, setUpdateNotifyDisabled } from '../../../composables/updatePrefs.js'
import { useAuth } from '../../../composables/useAuth.js'
import FreeOrderModal from './FreeOrderModal.vue'

const props = defineProps({
  openid: { type: String, default: '' }
})
const emit = defineEmits(['close'])

// 左侧边栏当前 tab
const activeTab = ref('models')
const providerSubTab = ref('free')
const mcpSubTab = ref('local')
const skillsSubTab = ref('local')

const VENDOR_ICONS = [
  [/sensenova|商汤/i, 'lucide:sparkles'],
  [/opencode/i, 'lucide:code-xml'],
  [/ollama/i, 'simple-icons:ollama'],
  [/stepfun|阶跃/i, 'lucide:footprints'],
  [/modelscope|魔搭/i, 'lucide:gallery-vertical-end'],
  [/deepseek/i, 'simple-icons:deepseek'],
  [/openai/i, 'simple-icons:openai'],
  [/anthropic|claude/i, 'simple-icons:anthropic'],
  [/gemini|google/i, 'simple-icons:googlegemini'],
  [/qwen|通义/i, 'simple-icons:alibabacloud'],
  [/mistral/i, 'simple-icons:mistralai'],
  [/groq/i, 'simple-icons:groq'],
  [/openrouter/i, 'simple-icons:openrouter'],
  [/hugging/i, 'simple-icons:huggingface'],
  [/github/i, 'simple-icons:github'],
  [/cloudflare/i, 'simple-icons:cloudflare'],
  [/nvidia/i, 'simple-icons:nvidia']
]

function vendorIcon(name = '') {
  return VENDOR_ICONS.find(([pattern]) => pattern.test(name))?.[1] || 'lucide:box'
}

function vendorColor(name = '') {
  const palette = ['#5b6ee1', '#9b5de5', '#d35f5f', '#168f78', '#d17a22', '#4f7cac', '#b24c7c']
  const hash = Array.from(name).reduce((sum, char) => sum + char.charCodeAt(0), 0)
  return palette[hash % palette.length]
}

// ============ 亲密等级（我的 tab，爱心表示） ============
const auth = useAuth()
// 亲密等级：外显 Lv.N（无上限）。无缓存时显示 Lv.1。
const intimacyLevel = computed(() => auth.intimacyLevel.value || 1)
// 爱心表示：每个等级一颗爱心（Lv.3 = ♥♥♥）
const heartsText = computed(() => '♥'.repeat(intimacyLevel.value))
// 亲密等级到下一级的进度粉条（与后端同曲线 100*N*(N-1)/2）
const intimacyTotalFor = (n) => 100 * n * (n - 1) / 2
const intimacyProgressPct = computed(() => {
  const v = auth.intimacy.value || 0
  const L = intimacyLevel.value || 1  // 当前已显示等级
  const cur = intimacyTotalFor(L)
  const next = intimacyTotalFor(L + 1)
  const span = next - cur
  if (span <= 0) return 0
  return Math.min(100, Math.floor(((v - cur) / span) * 100))
})

// ============ 界面配色切换 ============
const colorThemes = computed(() => Object.entries(THEME_PRESETS))
const selectedTheme = computed(() => THEME_PRESETS[theme.value] || THEME_PRESETS.orange)
const currentModeLabel = computed(() => MODE_OPTIONS.find(option => option.value === mode.value)?.label || '亮色')

function selectTheme(key) {
  theme.value = key
}

const PRESETS = [
  { name: 'DeepSeek', endpoint: 'https://api.deepseek.com' }
]
const MASKED = '••••••••'

const configs = ref([])
const freeModels = ref([])
const customModels = ref([])
const loading = ref(true)
const errorMsg = ref('')
const editingConfig = ref(null)
const editingVendor = ref(null)
const vendorKeyDraft = ref('')
const vendorKeySecretDraft = ref('')
const isNew = ref(false)
const configBusy = ref('')

const vendorGroups = computed(() => {
  const map = new Map()
  for (const fm of freeModels.value) {
    if (fm.local) continue
    const v = fm.vendor || '其他'
    if (!map.has(v)) map.set(v, { vendor: v, items: [], hasKey: false, keyless: false, keyUrl: '', dualKey: false })
    const g = map.get(v)
    g.items.push(fm)
    if (fm.api_key_set) g.hasKey = true
    if (fm.keyless) g.keyless = true
    if (!g.keyUrl && fm.key_url) g.keyUrl = fm.key_url
    if (fm.vendor === 'Modal') g.dualKey = true
  }
  return Array.from(map.values())
})

// 上下文窗口展示：262144 → 256K，1048576 → 1M
function fmtCtx(n) {
  if (!n) return ''
  if (n >= 1000000) return (n / 1000000).toFixed(n % 1000000 === 0 ? 0 : 1) + 'M'
  if (n >= 1000) return Math.round(n / 1000) + 'K'
  return String(n)
}

// 聚合 API 卡片：复制 base_url / key 到剪贴板
function copyAggText(text) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).catch(() => {})
  }
}

// 自定义 API 解锁弹窗
const showCustomLockModal = ref(false)
const customLockKey = ref('')
const customLockError = ref('')
const CUSTOM_API_UNLOCK_KEY = 'rescene' // ← 开发者密码，改这里
function unlockCustom() {
  customLockError.value = ''
  if (customLockKey.value === CUSTOM_API_UNLOCK_KEY) {
    showCustomLockModal.value = false
    customLockKey.value = ''
    providerSubTab.value = 'custom'
  } else {
    customLockError.value = '密码不对，划走吧'
  }
}

// 「Auto 自定义排序」弹窗（提供方 → 免费模型 → 标题行按钮打开）
const showFreeOrderModal = ref(false)

function configUrl() {
  return `/api/models/config${props.openid ? '?openid=' + encodeURIComponent(props.openid) : ''}`
}

function discoverUrl() {
  return `/api/models/discover${props.openid ? '?openid=' + encodeURIComponent(props.openid) : ''}`
}

async function loadConfigs() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await fetch(configUrl())
    if (!res.ok) throw new Error('加载失败')
    const data = await res.json()
    configs.value = data.configs || []
    freeModels.value = data.free_models || []
    customModels.value = data.custom_models || []
    firecrawlKeySet.value = !!data.firecrawl_key_set
  } catch (e) {
    errorMsg.value = '加载配置失败，请稍后再试'
  } finally {
    loading.value = false
  }
}

async function persist(nextConfigs) {
  const res = await fetch(configUrl(), {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ configs: nextConfigs })
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.error || '保存失败')
  }
  window.dispatchEvent(new CustomEvent('model-config-changed'))
}

function startAdd() {
  isNew.value = true
  errorMsg.value = ''
  editingConfig.value = {
    id: 'cfg_' + Date.now().toString(36),
    name: '', endpoint: '', api_key: '', api_key_set: false,
    default_model: '', models: [], is_default: configs.value.length === 0
  }
}
function startEdit(cfg) {
  isNew.value = false
  errorMsg.value = ''
  editingConfig.value = { ...cfg, api_key: '' }
}
function startEditVendor(grp) {
  editingVendor.value = grp.vendor
    vendorKeyDraft.value = ''
    vendorKeySecretDraft.value = ''
}
function cancelVendorEdit() {
  editingVendor.value = null
  vendorKeyDraft.value = ''
  vendorKeySecretDraft.value = ''
}
async function saveVendorKey(grp) {
  const key = grp.dualKey
    ? (vendorKeyDraft.value + ':' + vendorKeySecretDraft.value)
    : vendorKeyDraft.value
  if (!key || !key.trim()) {
    errorMsg.value = '请输入 API Key'
    return
  }
  errorMsg.value = ''
  const ids = grp.items.map(fm => fm.id)
  const idSet = new Set(ids)
  const untouched = configs.value
    .filter(c => !idSet.has(c.id))
    .map(c => ({ ...c, api_key: MASKED }))
  const vendorEntries = ids.map(id => {
    const fm = grp.items.find(x => x.id === id)
    return {
      id,
      name: grp.vendor,
      endpoint: fm.endpoint,
      api_key: key,
      default_model: fm.model,
      is_default: false
    }
  })
  try {
    await persist([...untouched, ...vendorEntries])
    await loadConfigs()
    editingVendor.value = null
    vendorKeyDraft.value = ''
  } catch (e) {
    errorMsg.value = e.message
  }
}
function cancelEdit() {
  editingConfig.value = null
}
function applyPreset(p) {
  if (!editingConfig.value) return
  editingConfig.value.endpoint = p.endpoint
  if (!editingConfig.value.name) editingConfig.value.name = p.name
}

function providerModelCount(cfg) {
  if (Array.isArray(cfg.models) && cfg.models.length) return cfg.models.length
  return cfg.default_model ? 1 : 0
}

async function discoverProviderModels(cfg) {
  const res = await fetch(discoverUrl(), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      config_id: cfg.id,
      endpoint: cfg.endpoint,
      api_key: cfg.api_key || ''
    })
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.error || '获取模型列表失败')
  return data.models || []
}

async function saveConfig() {
  if (!editingConfig.value.name.trim()) {
    errorMsg.value = '提供方名称不能为空'
    return
  }
  if (!editingConfig.value.endpoint.trim()) {
    errorMsg.value = 'Endpoint 不能为空'
    return
  }
  errorMsg.value = ''
  configBusy.value = editingConfig.value.id
  try {
    const models = await discoverProviderModels(editingConfig.value)
    const previousDefault = editingConfig.value.default_model
    const defaultModel = models.some(model => model.id === previousDefault)
      ? previousDefault
      : (models[0]?.id || '')
    const entry = {
      ...editingConfig.value,
      name: editingConfig.value.name.trim(),
      endpoint: editingConfig.value.endpoint.trim(),
      api_key: editingConfig.value.api_key || MASKED,
      keyless: !editingConfig.value.api_key && !editingConfig.value.api_key_set,
      default_model: defaultModel,
      models
    }
    let next = isNew.value
      ? [...configs.value, entry]
      : configs.value.map(c => (c.id === entry.id ? entry : c))
    if (entry.is_default) {
      next = next.map(c => ({ ...c, api_key: c.id === entry.id ? entry.api_key : MASKED, is_default: c.id === entry.id }))
    }
    await persist(next)
    await loadConfigs()
    editingConfig.value = null
  } catch (e) {
    errorMsg.value = e.message
  } finally {
    configBusy.value = ''
  }
}

async function refreshConfigModels(cfg) {
  errorMsg.value = ''
  configBusy.value = cfg.id
  try {
    const models = await discoverProviderModels(cfg)
    const defaultModel = models.some(model => model.id === cfg.default_model)
      ? cfg.default_model
      : (models[0]?.id || '')
    const next = configs.value.map(c => ({
      ...c,
      api_key: MASKED,
      ...(c.id === cfg.id ? { models, default_model: defaultModel } : {})
    }))
    await persist(next)
    await loadConfigs()
  } catch (e) {
    errorMsg.value = e.message
  } finally {
    configBusy.value = ''
  }
}

async function removeConfig(id) {
  const next = configs.value.filter(c => c.id !== id).map(c => ({ ...c, api_key: MASKED }))
  try {
    await persist(next)
    await loadConfigs()
  } catch (e) {
    errorMsg.value = e.message
  }
}

async function setDefault(id) {
  const next = configs.value.map(c => ({ ...c, api_key: MASKED, is_default: c.id === id }))
  try {
    await persist(next)
    await loadConfigs()
  } catch (e) {
    errorMsg.value = e.message
  }
}

const chatList = computed(() => {
  const builtIn = freeModels.value
    .filter(model => model.local || model.keyless || model.api_key_set)
    .map(model => ({ label: model.name || model.id, value: model.id }))
  const custom = customModels.value
    .filter(model => model.keyless || model.api_key_set)
    .map(model => ({ label: `${model.vendor} · ${model.name}`, value: model.id }))
  // Auto 智能路由置顶：按免费模型池排序逐个尝试 + 熔断
  return [{ label: 'Auto 智能路由', value: 'auto' }, ...builtIn, ...custom]
})

// ============ 模型：统一 / 分开配置（文字 vs 识图） ============
// 纯 localStorage 读写，跟 agentMode（Yolo/Ask）同一套轻量约定——不用共享 store，
// 因为只有 ChatWidget 发送图片时才需要读一次（见 attachImageFile），不需要跨组件响应式。
const MODEL_MODE_KEY = 'modelMode'
const VISION_MODEL_KEY = 'visionModel'
const modelMode = ref(localStorage.getItem(MODEL_MODE_KEY) === 'split' ? 'split' : 'unified')
// 统一模式复用 selectedModel（跟 ChatWidget 顶部模型下拉同一个 key，改这里 = 改那里）；
// 分开模式下文字模型也是 selectedModel，只是识图另配一个 visionModel。
const unifiedModelDraft = ref(localStorage.getItem('selectedModel') || '')
const textModelDraft = ref(localStorage.getItem('selectedModel') || '')
const visionModelDraft = ref(localStorage.getItem(VISION_MODEL_KEY) || '')

function setModelMode(mode) {
  modelMode.value = mode
  localStorage.setItem(MODEL_MODE_KEY, mode)
}
function setUnifiedModel(id) {
  if (!id) return
  localStorage.setItem('selectedModel', id)
  textModelDraft.value = id // 切回分开配置时文字模型不用重选
}
function setTextModel(id) {
  if (!id) return
  localStorage.setItem('selectedModel', id)
}
function setVisionModel(id) {
  localStorage.setItem(VISION_MODEL_KEY, id || '')
}

const IMAGE_PROVIDER_KEY = 'imageProvider'
const imageProviderDraft = ref(localStorage.getItem(IMAGE_PROVIDER_KEY) || 'pollinations')
function setImageProvider(provider) {
  localStorage.setItem(IMAGE_PROVIDER_KEY, provider || 'pollinations')
}

// ============ Firecrawl 联网搜索（web_search 常驻工具，模型自主触发） ============
const FIRECRAWL_KEY_ID = 'firecrawl'
// 后端 /api/models/config 的 firecrawl_key_set 字段返回是否已配 Key
const firecrawlKeySet = ref(false)
const firecrawlKeyDraft = ref('')
async function saveFirecrawlKey() {
  const key = firecrawlKeyDraft.value
  if (!key || !key.trim()) {
    errorMsg.value = '请输入 Firecrawl API Key'
    return
  }
  errorMsg.value = ''
  const untouched = configs.value
    .filter(c => c.id !== FIRECRAWL_KEY_ID)
    .map(c => ({ ...c, api_key: MASKED }))
  await persist([...untouched, {
    id: FIRECRAWL_KEY_ID, name: 'Firecrawl', endpoint: 'https://api.firecrawl.dev',
    api_key: key, default_model: '', is_default: false
  }])
  await loadConfigs()
  firecrawlKeyDraft.value = ''
  firecrawlKeySet.value = true
}

// id → 是否支持识图，合并免费池 + 自定义配置两个来源（/api/models/config 都带 vision 字段）。
// 注意：这【只】用于 UI 上的“· 识图”角标提示，不再参与候选过滤——
// 识图模型由用户自己选，不需要维护“哪个模型能识图”的标签。
const visionByID = computed(() => {
  const map = {}
  for (const fm of freeModels.value) map[fm.id] = !!fm.vision
  for (const model of customModels.value) map[model.id] = !!model.vision
  return map
})
// 识图模型候选 = 全部可用模型，用户自己挑（不按 vision 标签过滤，
// 免得我们得一直维护哪个模型支持识图）。
// Auto 是虚拟路由 ID（后端识图按具体模型解析），排除在识图候选外。
const visionCapableChatList = computed(() => {
  return chatList.value.filter(m => m.value !== 'auto')
})

// 当用户没有显式选过识图模型，且当前有可用模型时，自动默认选中第一个。
watch(visionCapableChatList, (list) => {
  if (!visionModelDraft.value && list.length) {
    const pick = list[0]
    visionModelDraft.value = pick.value
    localStorage.setItem(VISION_MODEL_KEY, pick.value)
  }
}, { immediate: true })

// ============ MCP ============
const mcpServers = ref([])
const mcpLoading = ref(false)
const mcpConfigPath = ref('')
const mcpRegistryItems = ref([])
const mcpRegistryLoading = ref(false)
const mcpRegistryQuery = ref('')
const catalogBusy = ref('')
let mcpLoaded = false
async function loadMCP(force = false) {
  if (mcpLoaded && !force) return
  mcpLoaded = true
  mcpLoading.value = true
  try {
    const res = await fetch('/api/mcp')
    const data = await res.json()
    mcpServers.value = data.servers || []
    mcpConfigPath.value = data.config_path || ''
  } catch (e) {
    mcpServers.value = []
  } finally {
    mcpLoading.value = false
  }
}

async function loadMCPRegistry(force = false) {
  if (mcpRegistryLoading.value) return
  if (mcpRegistryItems.value.length && !force) return
  mcpRegistryLoading.value = true
  errorMsg.value = ''
  try {
    const params = new URLSearchParams({ limit: '36' })
    if (mcpRegistryQuery.value.trim()) params.set('q', mcpRegistryQuery.value.trim())
    const res = await fetch('/api/mcp/registry?' + params)
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || 'MCP Registry 加载失败')
    mcpRegistryItems.value = data.items || []
  } catch (e) {
    mcpRegistryItems.value = []
    errorMsg.value = e.message
  } finally {
    mcpRegistryLoading.value = false
  }
}

async function installMCP(item) {
  catalogBusy.value = 'mcp-install:' + item.name
  errorMsg.value = ''
  try {
    const res = await fetch('/api/mcp/registry/install', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: item.name, title: item.title, url: item.url })
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || 'MCP 接入失败')
    await Promise.all([loadMCP(true), loadMCPRegistry(true)])
  } catch (e) {
    errorMsg.value = e.message
  } finally {
    catalogBusy.value = ''
  }
}

async function uninstallMCP(server) {
  if (!window.confirm(`移除 MCP「${server.registry_name || server.name}」？`)) return
  catalogBusy.value = 'mcp:' + server.name
  errorMsg.value = ''
  try {
    const res = await fetch('/api/mcp/registry/' + encodeURIComponent(server.name), { method: 'DELETE' })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || 'MCP 移除失败')
    await Promise.all([loadMCP(true), loadMCPRegistry(true)])
  } catch (e) {
    errorMsg.value = e.message
  } finally {
    catalogBusy.value = ''
  }
}

// ============ Skills ============
const skills = ref([])
const skillsLoading = ref(false)
const skillsDir = ref('')
const skillsExtDir = ref('')
const expandedSkill = ref(null)
const skillRegistryItems = ref([])
const skillRegistryLoading = ref(false)
const skillRegistryQuery = ref('')
const skillRegistrySource = ref('anthropics/skills')
const skillRegistrySources = ref([
  { id: 'anthropics/skills', label: 'Anthropic Skills' },
  { id: 'openai/skills', label: 'OpenAI Skills' },
  { id: 'vercel-labs/skills', label: 'Vercel Labs Skills' }
])
let skillsLoaded = false
function toggleSkill(name) { expandedSkill.value = expandedSkill.value === name ? null : name }
function normalizedSkillStatus(skill) { return skill.status === 'archived' ? 'archived' : 'active' }
function skillStatusLabel(skill) { return normalizedSkillStatus(skill) === 'active' ? '已启用' : '已关闭' }
function isSkillActive(skill) { return normalizedSkillStatus(skill) === 'active' }
async function loadSkills(force = false) {
  if (skillsLoaded && !force) return
  skillsLoaded = true
  skillsLoading.value = true
  try {
    const res = await fetch('/api/skills')
    const data = await res.json()
    skills.value = data.skills || []
    skillsDir.value = data.dir || ''
    skillsExtDir.value = data.ext_dir || ''
  } catch (e) {
    skills.value = []
  } finally {
    skillsLoading.value = false
  }
}

async function loadSkillRegistry(force = false) {
  if (skillRegistryLoading.value) return
  if (skillRegistryItems.value.length && !force) return
  skillRegistryLoading.value = true
  errorMsg.value = ''
  try {
    const params = new URLSearchParams({ source: skillRegistrySource.value })
    if (skillRegistryQuery.value.trim()) params.set('q', skillRegistryQuery.value.trim())
    const res = await fetch('/api/skills/registry?' + params)
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || 'GitHub 技能仓库加载失败')
    skillRegistryItems.value = data.items || []
    if (Array.isArray(data.sources) && data.sources.length) skillRegistrySources.value = data.sources
  } catch (e) {
    skillRegistryItems.value = []
    errorMsg.value = e.message
  } finally {
    skillRegistryLoading.value = false
  }
}

async function installHostedSkill(item) {
  catalogBusy.value = 'skill-install:' + item.path
  errorMsg.value = ''
  try {
    const res = await fetch('/api/skills/registry/install', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ source: item.source, path: item.path })
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || '技能安装失败')
    await Promise.all([loadSkills(true), loadSkillRegistry(true)])
  } catch (e) {
    errorMsg.value = e.message
  } finally {
    catalogBusy.value = ''
  }
}

async function uninstallHostedSkill(item) {
  if (!window.confirm(`移除技能「${item.name}」？`)) return
  catalogBusy.value = 'skill-remove:' + item.external_id
  errorMsg.value = ''
  try {
    const res = await fetch('/api/skills/external/' + encodeURIComponent(item.external_id), { method: 'DELETE' })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || '技能移除失败')
    await Promise.all([loadSkills(true), loadSkillRegistry(true)])
  } catch (e) {
    errorMsg.value = e.message
  } finally {
    catalogBusy.value = ''
  }
}
async function setSkillStatus(skill, status) {
  try {
    const res = await fetch('/api/skills/' + encodeURIComponent(skill.name) + '/status', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ status })
    })
    if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || '更新失败')
    await loadSkills(true)
  } catch (e) { errorMsg.value = e.message }
}
async function removeSkill(skill) {
  if (!window.confirm(`删除技能「${skill.name}」？此操作不可恢复。`)) return
  try {
    const res = await fetch('/api/skills/' + encodeURIComponent(skill.name), { method: 'DELETE' })
    if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || '删除失败')
    if (expandedSkill.value === skill.name) expandedSkill.value = null
    await loadSkills(true)
  } catch (e) { errorMsg.value = e.message }
}
// ============ Profile ============
const profile = ref({ full_name: '', work: '', instructions: '' })

// 记忆 tab：直接渲染后端 /api/memory/inject 返回的真实注入段（system / memory 两段），
// 不再在前端重拼，杜绝「展示 ≠ 实际注入」的漂移。
const memorySegments = ref([])
const memoryLoading = ref(false)
// 云端记忆同步开关（记忆 tab）：默认开；env_override 时禁用（部署级强制关闭）
const memorySyncEnabled = ref(true)
const memorySyncEnvOverride = ref(false)
const humanReadableMemoryMarkdown = computed(() => {
  const parts = []
  for (const seg of memorySegments.value) {
    if (!seg || !seg.raw) continue
    const title = (seg.title || '').trim() || seg.key
    parts.push(`## ${title}\n\n${String(seg.raw).trim()}`)
  }
  return parts.join('\n\n').trim() || ''
})
async function loadMemoryInject() {
  memoryLoading.value = true
  loadMemorySyncSetting()
  try {
    const res = await fetch('/api/memory/inject')
    if (res.ok) {
      const data = await res.json()
      memorySegments.value = Array.isArray(data.segments) ? data.segments : []
    } else {
      memorySegments.value = []
    }
  } catch (e) {
    memorySegments.value = []
  } finally {
    memoryLoading.value = false
  }
}
const profileSaving = ref(false)
const profileSaved = ref(false)

// 云端记忆同步开关：读取当前状态 + 切换保存（记忆 tab 开关）
async function loadMemorySyncSetting() {
  try {
    const res = await fetch('/api/memory/sync/settings')
    if (res.ok) {
      const data = await res.json()
      memorySyncEnabled.value = data.enabled !== false
      memorySyncEnvOverride.value = !!data.env_override
    }
  } catch (e) {}
}
async function saveMemorySyncSetting() {
  try {
    await fetch('/api/memory/sync/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: memorySyncEnabled.value })
    })
  } catch (e) {}
}
async function loadProfile() {
  try {
    const res = await fetch('/api/profile')
    if (res.ok) {
      const data = await res.json()
      profile.value = {
        full_name: data.full_name || '',
        work: data.work || '',
        instructions: data.instructions || ''
      }
    }
  } catch (e) {}
}
async function saveProfile() {
  profileSaving.value = true
  profileSaved.value = false
  try {
    const res = await fetch('/api/profile', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(profile.value)
    })
    if (res.ok) {
      profileSaved.value = true
      setTimeout(() => { profileSaved.value = false }, 2000)
    }
  } catch (e) {} finally {
    profileSaving.value = false
  }
}

// ============ 版本与更新 ============
const versionLoading = ref(false)
const versionOpening = ref(false)
const versionInfo = ref({})
const notifyDisabled = ref(isUpdateNotifyDisabled())

async function loadVersion() {
  versionLoading.value = true
  try {
    const res = await fetch('/api/update/check')
    if (res.ok) {
      const data = await res.json()
      versionInfo.value = data.ok && data.update ? data.update : {}
    }
  } catch (e) {
    versionInfo.value = {}
  } finally {
    versionLoading.value = false
  }
}

function onNotifyDisabledChange() {
  setUpdateNotifyDisabled(notifyDisabled.value)
}

async function openUpdate() {
  const url = versionInfo.value.download_url || versionInfo.value.release_url
  if (!url) return
  versionOpening.value = true
  try {
    const res = await fetch('/api/update/open', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url })
    })
    if (!res.ok) window.open(url, '_blank')
  } catch (e) {
    window.open(url, '_blank')
  } finally {
    versionOpening.value = false
  }
}

function handleEsc(e) {
  if (e.key === 'Escape') emit('close')
}


onMounted(() => {
  loadConfigs()
  loadProfile()
  document.addEventListener('keydown', handleEsc)
  // 其他入口改了模型配置（如 FreeOrderModal 拖拽排序）→ 设置页也刷新顺序
  window.addEventListener('model-config-changed', loadConfigs)
})
onUnmounted(() => {
  document.removeEventListener('keydown', handleEsc)
  window.removeEventListener('model-config-changed', loadConfigs)
})
</script>

<style scoped>
.settings-modal-backdrop {
  position: fixed;
  inset: 0;
  background:
    radial-gradient(circle at 50% 12%, color-mix(in srgb, var(--app-accent) 12%, transparent), transparent 42%),
    rgba(14, 15, 18, 0.56);
  backdrop-filter: blur(14px) saturate(0.9);
  -webkit-backdrop-filter: blur(14px) saturate(0.9);
  z-index: 20000;
  display: flex;
  align-items: center;
  justify-content: center;
}
.settings-modal-card {
  width: min(1120px, calc(100vw - 48px));
  height: min(700px, calc(100vh - 48px));
  background: var(--app-surface);
  border: 1px solid color-mix(in srgb, var(--app-border) 82%, transparent);
  border-radius: 20px;
  box-shadow: 0 30px 100px rgba(0, 0, 0, 0.28), 0 1px 0 rgba(255, 255, 255, 0.08) inset;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: settings-enter 180ms cubic-bezier(.2,.8,.2,1);
}
@keyframes settings-enter {
  from { opacity: 0; transform: translateY(8px) scale(0.985); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}
.settings-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 72px;
  box-sizing: border-box;
  padding: 14px 18px 14px 20px;
  border-bottom: 1px solid var(--app-border);
  flex-shrink: 0;
}
.settings-brand, .settings-header-actions { display: flex; align-items: center; }
.settings-brand { gap: 11px; }
.settings-brand-mark {
  width: 36px; height: 36px; display: grid; place-items: center; flex: none;
  color: #fff; background: linear-gradient(145deg, var(--app-accent), var(--app-accent-hover));
  border-radius: 11px; box-shadow: 0 7px 18px var(--app-accent-soft), inset 0 1px 0 rgba(255,255,255,.22);
}
.settings-brand-copy { display: flex; flex-direction: column; gap: 1px; }
.settings-brand-copy strong { color: var(--app-text); font-size: 14px; line-height: 1.25; letter-spacing: .01em; }
.settings-brand-copy small { color: var(--app-text-faint); font-size: 11px; line-height: 1.25; }
.settings-header-actions { gap: 9px; }
.settings-privacy-badge {
  height: 28px; box-sizing: border-box; display: inline-flex; align-items: center; gap: 5px;
  padding: 0 9px; color: #16805f; background: color-mix(in srgb, #2db786 11%, var(--app-surface));
  border: 1px solid color-mix(in srgb, #2db786 23%, var(--app-border)); border-radius: 999px;
  font-size: 11px; font-weight: 650;
}
.settings-modal-close {
  display: flex; align-items: center; justify-content: center;
  width: 34px; height: 34px; border-radius: 9px; border: 1px solid transparent;
  background: transparent; cursor: pointer; color: var(--app-text-soft);
}
.settings-modal-close:hover { color: var(--app-text); background: var(--app-surface-3); border-color: var(--app-border-soft); }
.settings-modal-close:focus-visible, .settings-tab:focus-visible, .settings-subtab:focus-visible { outline: 2px solid var(--app-accent); outline-offset: 2px; }
.settings-modal-body {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
/* 左侧边栏 */
.settings-sidebar {
  width: 210px;
  flex-shrink: 0;
  border-right: 1px solid var(--app-border-soft);
  padding: 18px 14px 22px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  background: var(--app-surface-2);
  overflow-y: auto;
}
.settings-nav-label {
  margin: 14px 10px 6px; color: var(--app-text-faint); font-size: 10px; font-weight: 750;
  line-height: 1; letter-spacing: .1em; text-transform: uppercase; user-select: none;
}
.settings-nav-label:first-child { margin-top: 2px; }
.settings-tab {
  display: flex;
  align-items: center;
  gap: 8px;
  text-align: left;
  min-height: 40px;
  padding: 9px 11px;
  font-size: 13px;
  font-weight: 580;
  color: var(--app-text-soft);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.settings-tab :deep(svg) { flex: none; opacity: .78; }
.settings-tab:hover { color: var(--app-text); background: var(--app-surface-3); }
.settings-tab.on {
  color: var(--app-accent); background: var(--app-accent-soft);
  border-color: color-mix(in srgb, var(--app-accent) 18%, transparent); box-shadow: inset 3px 0 0 var(--app-accent);
}
.settings-tab.on :deep(svg) { opacity: 1; }
.settings-tab-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.settings-tab-group > .settings-tab { width: 100%; }
.settings-subtabs {
  display: flex;
  flex-direction: column;
  gap: 3px;
  margin: 2px 0 4px 17px;
  padding-left: 13px;
  border-left: 1px solid var(--app-border);
}
.settings-subtab {
  min-height: 34px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  color: var(--app-text-faint);
  background: transparent;
  border: none;
  border-radius: 7px;
  font-size: 12.5px;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
  transition: color 0.15s ease, background 0.15s ease;
}
.settings-subtab:hover {
  color: var(--app-text);
  background: var(--app-surface-3);
}
.settings-subtab.on {
  color: var(--app-text);
  background: var(--app-surface-3);
  font-weight: 650;
}
/* 右侧内容 */
.settings-content {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: 24px 30px 28px;
  background: var(--app-surface);
  color: var(--app-text);
}
.settings-panel { display: block; animation: settings-panel-in 160ms ease; }
@keyframes settings-panel-in { from { opacity: 0; transform: translateY(3px); } }

.settings-section-title { font-size: 14px; font-weight: 700; color: var(--app-text); margin-bottom: 5px; display: flex; align-items: center; gap: 6px; }
.settings-section-desc { max-width: 760px; font-size: 12px; color: var(--app-text-faint); margin-bottom: 18px; line-height: 1.65; }
.settings-section-desc code { font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; background: var(--app-code-bg); padding: 1px 5px; border-radius: 4px; }
.settings-error { font-size: 12px; color: #d94834; padding: 8px 0; }
.settings-loading { font-size: 12.5px; color: var(--app-text-faint); padding: 8px 0; }
.settings-empty { font-size: 12.5px; color: var(--app-text-faint); padding: 20px 0; text-align: center; }
.inline-refresh { margin-left: auto; border: none; background: transparent; color: var(--app-text-faint); cursor: pointer; display: inline-flex; padding: 2px; border-radius: 5px; }
.inline-refresh:hover { background: var(--app-surface-3); }
.spin { animation: sm-spin 0.9s linear infinite; }
@keyframes sm-spin { to { transform: rotate(360deg); } }

.model-select {
  margin-left: auto; max-width: 320px; flex: 1;
  font-size: 12.5px; color: var(--app-text); background: var(--app-surface-2);
  min-height: 38px; box-sizing: border-box;
  border: 1px solid var(--app-border); border-radius: 9px; padding: 7px 10px;
  cursor: pointer;
}
.model-select:focus { outline: none; border-color: var(--app-accent); box-shadow: 0 0 0 3px var(--app-accent-soft); }

.model-pick-btn {
  flex-shrink: 0; margin-left: auto; padding: 3px 12px; font-size: 12px; font-weight: 600;
  color: var(--app-text-soft); background: var(--app-surface-3); border: 1px solid #ddd; border-radius: 999px; cursor: pointer;
  transition: all 0.15s ease;
}
.model-pick-btn:hover { background: var(--app-surface-3); }
.model-pick-btn.on { color: #fff; background: var(--app-accent); border-color: var(--app-accent); }
.api-config-card { border: 1px solid var(--app-border); border-radius: 10px; padding: 10px 12px; margin-bottom: 8px; background: var(--app-surface-2); }
.api-config-row { display: flex; align-items: center; gap: 8px; }
.api-config-name { font-size: 13px; font-weight: 600; color: var(--app-text); flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.api-config-default-badge { font-size: 10.5px; font-weight: 600; color: #12b76a; background: rgba(18, 183, 106, 0.12); padding: 2px 8px; border-radius: 999px; flex-shrink: 0; }
.api-config-actions { display: flex; gap: 4px; flex-shrink: 0; }
.api-config-action-btn { font-size: 11.5px; color: var(--app-text-soft); background: transparent; border: 1px solid var(--app-border); border-radius: 6px; padding: 3px 8px; cursor: pointer; }
.api-config-action-btn:hover { background: var(--app-surface-3); }
.api-config-action-btn.danger { color: #d94834; border-color: #f3c9c2; }
.api-config-action-btn.on { color: #fff; background: var(--app-accent); border-color: var(--app-accent); }
.api-config-meta { margin-top: 5px; font-size: 11px; color: var(--app-text-faint); font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.api-config-add-btn {
  display: flex; align-items: center; justify-content: center; gap: 6px;
  padding: 9px 0; border: 1px dashed #d4d4d4; border-radius: 10px;
  color: var(--app-text-soft); font-size: 12.5px; font-weight: 600; cursor: pointer;
}
.api-config-add-btn:hover { background: var(--app-surface-2); border-color: #c4c4c4; }

.api-config-form { border: 1px solid var(--app-border); border-radius: 10px; padding: 14px; background: var(--app-surface-2); }
.api-free-badge { font-size: 10px; font-weight: 700; color: #0d9488; background: var(--app-surface-2); border: 1px solid #99f6e4; border-radius: 999px; padding: 1px 7px; }
.api-config-card.free { background: var(--app-surface); }
.api-config-card.free:first-child { margin-top: 0; }

.vendor-group { margin-bottom: 10px; }
.vendor-group:last-child { margin-bottom: 0; }
.vendor-head {
  display: flex; align-items: center; flex-wrap: wrap; gap: 8px;
  min-height: 54px; box-sizing: border-box; padding: 9px 11px; background: var(--app-surface-2);
  border: 1px solid var(--app-border); border-radius: 12px; user-select: none;
  transition: border-color .16s ease, background .16s ease, transform .16s ease, box-shadow .16s ease;
}
.vendor-head:hover {
  background: var(--app-surface); border-color: color-mix(in srgb, var(--app-accent) 28%, var(--app-border));
  transform: translateY(-1px); box-shadow: 0 7px 20px rgba(0,0,0,.045);
}
.vendor-logo {
  width: 32px; height: 32px; display: grid; place-items: center; flex: none;
  color: var(--vendor-color); background: color-mix(in srgb, var(--vendor-color) 11%, var(--app-surface));
  border: 1px solid color-mix(in srgb, var(--vendor-color) 18%, var(--app-border)); border-radius: 9px;
}
.vendor-name { font-size: 13px; font-weight: 700; color: var(--app-text); flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.vendor-count { font-size: 10.5px; font-weight: 600; color: var(--app-text-soft); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 999px; padding: 1px 8px; flex-shrink: 0; }
.vendor-keystate { font-size: 10.5px; font-weight: 600; color: var(--app-text-faint); flex-shrink: 0; }
.vendor-keystate.on { color: #12b76a; }
.vendor-keystate.free { color: var(--app-accent); }
.vendor-key-btn { min-height: 28px; font-size: 11px; font-weight: 600; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 8px; padding: 4px 10px; cursor: pointer; flex-shrink: 0; }
.vendor-key-btn:hover { background: var(--app-surface-3); }
.vendor-key-link { color: var(--app-accent); text-decoration: none; }
.vendor-key-link:hover { text-decoration: underline; }
.settings-section-title-row { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.auto-sort-btn {
  display: inline-flex; align-items: center; gap: 4px;
  font-size: 11px; font-weight: 600; color: var(--app-accent);
  background: var(--app-accent-soft); border: 1px solid var(--app-accent);
  border-radius: 999px; padding: 3px 10px; cursor: pointer; flex-shrink: 0;
  transition: background 0.15s;
}
.auto-sort-btn:hover { background: var(--app-surface-3); }
.agg-api-card { background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 10px; padding: 10px 12px; margin-bottom: 12px; }
.agg-api-head { font-size: 12px; font-weight: 700; color: var(--app-text); margin-bottom: 8px; }
.agg-api-row { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.agg-api-row:last-of-type { margin-bottom: 0; }
.agg-api-label { font-size: 11px; color: var(--app-text-soft); width: 62px; flex: none; }
.agg-api-code { font-size: 11.5px; font-family: var(--app-mono-font, ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace); color: var(--app-accent); background: var(--app-surface-2); border: 1px solid var(--app-border-soft); border-radius: 6px; padding: 2px 8px; }
.agg-api-copy { font-size: 10.5px; color: var(--app-text-soft); background: var(--app-surface-2); border: 1px solid var(--app-border-soft); border-radius: 6px; padding: 2px 8px; cursor: pointer; flex: none; }
.agg-api-copy:hover { background: var(--app-surface-3); }
.agg-api-tip { font-size: 10.5px; color: var(--app-text-faint); margin-top: 6px; line-height: 1.5; }
/* 自定义 API 锁 + 解锁弹窗 */
.locked { opacity: 0.5; cursor: pointer; }
.locked:hover { opacity: 0.7; }
.mm-backdrop { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.45); display: flex; align-items: center; justify-content: center; z-index: 99999; }
.mm-card { width: 420px; max-width:90vw; max-height: 80vh; background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 14px; display: flex; flex-direction: column; box-shadow: 0 20px 60px rgba(0,0,0,0.25); overflow: hidden; padding:24px; }
.mm-input { width:100%; padding:8px 12px; border:1px solid var(--app-border); border-radius:8px; background:var(--app-surface); color:var(--app-text); font-size:13px; outline:none; box-sizing:border-box; }
.mm-input:focus { border-color:var(--app-accent); }
.mm-error { font-size:12px; color:#e74c3c; margin-bottom:8px; }
.mm-actions { display:flex; gap:8px; justify-content:center; }
.mm-btn { padding:6px 16px; border-radius:8px; font-size:12px; font-weight:600; cursor:pointer; border:1px solid var(--app-border); background:var(--app-surface); color:var(--app-text); }
.mm-btn-primary { background:var(--app-accent); color:#fff; border-color:var(--app-accent); }
.mm-btn-cancel { color:var(--app-text-soft); }
.mm-title { font-size:14px; font-weight:700; color:var(--app-text); }
.vendor-model-cards { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
.fm-card {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 4px 10px; border-radius: 8px;
  background: var(--app-surface); border: 1px solid var(--app-border-soft);
  font-size: 12px; color: var(--app-text); max-width: 100%;
}
.fm-signal { display: inline-flex; align-items: flex-end; gap: 1.5px; flex: none; }
.fm-signal i { width: 3px; border-radius: 1px; background: var(--app-surface-3); }
.fm-signal i:nth-child(1) { height: 4px; }
.fm-signal i:nth-child(2) { height: 6px; }
.fm-signal i:nth-child(3) { height: 8px; }
.fm-signal i:nth-child(4) { height: 10px; }
.fm-signal i.on { background: var(--app-accent); }
.fm-name { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fm-card.sig-0 .fm-name { color: var(--app-text-faint); }
.fm-tag { flex: none; font-size: 10px; line-height: 1.4; padding: 1px 5px; border-radius: 4px; background: var(--app-surface-3); color: var(--app-text-soft); }
.vendor-thanks {
  margin-top: 12px; padding-top: 10px;
  font-size: 11px; color: var(--app-text-faint);
  text-align: center;
  border-top: 1px solid var(--app-border-soft);
  user-select: none;
}
.vendor-key-inline { display: flex; align-items: center; gap: 8px; background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 10px; padding: 8px 10px; margin-bottom: 8px; }
.vendor-key-input { flex: 1; min-width: 0; font-size: 12.5px; color: var(--app-text); border: 1px solid var(--app-border); border-radius: 8px; padding: 6px 10px; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif; }
.vendor-key-input:focus { outline: none; border-color: #c0c0c0; }
.vendor-key-save { font-size: 12px; font-weight: 600; color: #fff; background: #1a1a1a; border: none; border-radius: 8px; padding: 6px 14px; cursor: pointer; flex-shrink: 0; }
.vendor-key-save:hover { background: #333; }
.firecrawl-key-status { font-size: 12px; color: #2e7d32; margin-top: 6px; display: inline-block; }
.vendor-key-cancel { font-size: 12px; font-weight: 600; color: var(--app-text-soft); background: var(--app-surface-3); border: 1px solid var(--app-border); border-radius: 8px; padding: 6px 12px; cursor: pointer; flex-shrink: 0; }
.vendor-key-cancel:hover { background: var(--app-surface-3); }
.api-preset-label { font-size: 11.5px; color: var(--app-text-faint); margin-right: 2px; }
.api-preset-btn { font-size: 11.5px; font-weight: 600; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 999px; padding: 4px 10px; cursor: pointer; }
.api-preset-btn:hover { background: var(--app-surface-3); }

.api-form-field { display: flex; flex-direction: column; gap: 4px; margin-bottom: 10px; }
.api-form-field span { font-size: 11.5px; color: var(--app-text-soft); font-weight: 600; }
.api-form-field input { font-size: 13px; padding: 7px 10px; border: 1px solid var(--app-border); border-radius: 8px; background: var(--app-surface); outline: none; font-family: inherit; }
.api-form-field input:focus { border-color: #c96442; }
.api-form-hint { font-size: 11.5px; line-height: 1.6; color: var(--app-text-soft); }

.api-form-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 4px; }
.api-form-btn { font-size: 12.5px; font-weight: 600; padding: 6px 14px; border-radius: 8px; cursor: pointer; border: none; }
.api-form-btn.cancel { background: transparent; border: 1px solid var(--app-border); color: var(--app-text-soft); }
.api-form-btn.cancel:hover { background: var(--app-surface-3); }
.api-form-btn.save { background: #1a1a1a; color: #fff; }
.api-form-btn.save:hover { background: #333333; }

/* 设置项基础控件 */
.param-row {
  display: flex; align-items: center; gap: 12px; min-height: 50px; box-sizing: border-box;
  padding: 7px 2px; border-bottom: 1px solid var(--app-border-soft);
}
.param-label { flex-shrink: 0; width: 96px; font-size: 12.5px; color: var(--app-text); }
.param-value { flex-shrink: 0; width: 72px; text-align: right; font-size: 12px; color: var(--app-text-soft); font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; }
.param-switch { position: relative; display: inline-block; margin-left: auto; cursor: pointer; }
.param-switch input { position: absolute; opacity: 0; width: 0; height: 0; }
.param-switch-track { display: block; width: 38px; height: 22px; border-radius: 999px; background: var(--app-border); transition: background 0.15s ease; position: relative; }
.param-switch-track::after { content: ''; position: absolute; top: 2px; left: 2px; width: 18px; height: 18px; border-radius: 50%; background: var(--app-surface); box-shadow: 0 1px 2px rgba(0,0,0,0.2); transition: transform 0.15s ease; }
.param-switch input:checked + .param-switch-track { background: var(--app-accent); }
.param-switch input:checked + .param-switch-track::after { transform: translateX(16px); }
/* 主题分段控件（仿图1 的 Small/Medium/Large 分段） */
.seg-control { margin-left: auto; display: inline-flex; background: var(--app-surface-3); border: 1px solid var(--app-border); border-radius: 8px; padding: 2px; }
.seg-btn { border: none; background: transparent; color: var(--app-text-soft); font-size: 12.5px; padding: 4px 14px; border-radius: 6px; cursor: pointer; transition: all 0.12s; }
.seg-btn.on { background: var(--app-surface); color: var(--app-text); box-shadow: 0 1px 2px rgba(0,0,0,0.08); font-weight: 600; }

/* 配色色卡网格 */
.theme-swatches { display: flex; flex-wrap: wrap; gap: 8px; margin-left: auto; max-width: 460px; }
.theme-swatch {
  display: inline-flex; align-items: center; gap: 7px;
  padding: 6px 12px 6px 8px; border-radius: 999px; cursor: pointer;
  background: var(--app-surface-2); border: 1.5px solid var(--app-border);
  color: var(--app-text-soft); font-size: 12.5px; transition: all 0.12s;
}
.theme-swatch:hover { border-color: var(--app-accent-soft); color: var(--app-text); }
.theme-swatch.on { border-color: var(--app-accent); color: var(--app-text); background: var(--app-accent-soft); font-weight: 600; }
.theme-swatch-dot { width: 16px; height: 16px; border-radius: 50%; box-shadow: inset 0 0 0 1px rgba(0,0,0,0.12); }
.theme-swatch-label { line-height: 1; }

/* MCP / Skills 实体卡片 */
.entity-card { border: 1px solid var(--app-border); border-radius: 10px; padding: 11px 13px; margin-bottom: 8px; background: var(--app-surface-2); }
.entity-head { display: flex; align-items: center; gap: 8px; color: var(--app-text); }
.entity-name { font-size: 13px; font-weight: 700; }
.entity-badge { font-size: 10.5px; font-weight: 600; color: var(--app-accent); background: var(--app-accent-soft); padding: 1px 8px; border-radius: 999px; }
.entity-meta { margin-top: 5px; font-size: 11.5px; color: var(--app-text-faint); font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; line-height: 1.5; word-break: break-all; }
.entity-tags { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 8px; }
.entity-tag { font-size: 10.5px; color: var(--app-text-soft); background: var(--app-surface-3); border: 1px solid var(--app-border); border-radius: 6px; padding: 2px 7px; font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; }
.mcp-state.is-connected { color: #047857; background: #d1fae5; }
.mcp-state.is-unavailable { color: #b45309; background: #fef3c7; }
.catalog-toolbar { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.catalog-search {
  flex: 1; min-width: 150px; height: 36px; box-sizing: border-box;
  display: flex; align-items: center; gap: 7px; padding: 0 10px;
  color: var(--app-text-faint); background: var(--app-surface-2);
  border: 1px solid var(--app-border); border-radius: 9px;
}
.catalog-search:focus-within { border-color: var(--app-accent); color: var(--app-accent); }
.catalog-search input {
  flex: 1; min-width: 0; border: none; outline: none; color: var(--app-text);
  background: transparent; font: inherit; font-size: 12.5px;
}
.catalog-source {
  height: 36px; max-width: 180px; padding: 0 28px 0 10px; color: var(--app-text);
  background: var(--app-surface-2); border: 1px solid var(--app-border); border-radius: 9px;
  font: inherit; font-size: 12px; outline: none;
}
.catalog-source:focus { border-color: var(--app-accent); }
.catalog-search-btn, .catalog-install-btn {
  min-height: 36px; padding: 0 14px; border: 1px solid var(--app-accent);
  border-radius: 9px; color: #fff; background: var(--app-accent);
  font: inherit; font-size: 12px; font-weight: 650; cursor: pointer;
  transition: opacity 0.15s ease, transform 0.15s ease, background 0.15s ease;
}
.catalog-search-btn:hover, .catalog-install-btn:not(:disabled):hover { transform: translateY(-1px); }
.catalog-install-btn:disabled { opacity: 0.62; cursor: default; }
.catalog-install-btn.installed { color: #047857; background: #d1fae5; border-color: #a7f3d0; }
.catalog-install-btn.installed.removable { color: var(--app-text-soft); background: var(--app-surface-3); border-color: var(--app-border); cursor: pointer; }
.catalog-card {
  display: flex; align-items: center; gap: 14px; padding: 12px 13px; margin-bottom: 8px;
  border: 1px solid var(--app-border); border-radius: 11px; background: var(--app-surface-2);
  transition: border-color 0.15s ease, transform 0.15s ease;
}
.catalog-card:hover { border-color: var(--app-accent-soft); transform: translateY(-1px); }
.catalog-card-main { flex: 1; min-width: 0; }
.catalog-id {
  margin-top: 4px; overflow: hidden; color: var(--app-text-faint); font-size: 10.5px;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; text-overflow: ellipsis; white-space: nowrap;
}
.catalog-desc { margin-top: 7px; color: var(--app-text-soft); font-size: 11.5px; line-height: 1.55; }
.skill-steps { margin: 8px 0 2px; padding-left: 20px; font-size: 12px; color: var(--app-text-soft); line-height: 1.7; }
/* 技能来源角标：自研走中性色，外部走强调色，一眼区分 */
.entity-badge.src-learned { color: var(--app-text-soft); background: var(--app-surface-3); }
.entity-badge.src-ext { color: var(--app-accent); background: var(--app-accent-soft); }
.skill-status.is-active { color: #047857; background: #d1fae5; }
.skill-status.is-archived { color: var(--app-text-faint); background: var(--app-surface-3); }
.skill-detail { margin: 8px 0; display: grid; gap: 4px; font-size: 11.5px; line-height: 1.55; color: var(--app-text-soft); }
.skill-detail b { color: var(--app-text); margin-right: 5px; }
.skill-actions { display: flex; gap: 6px; margin-top: 10px; }
.skill-actions button { border: 1px solid var(--app-border); background: var(--app-surface); color: var(--app-text-soft); border-radius: 6px; padding: 3px 9px; font-size: 11px; cursor: pointer; }
.skill-actions button:hover { color: #fff; background: var(--app-accent); border-color: var(--app-accent); }
.skill-actions button.danger:hover { background: #dc2626; border-color: #dc2626; }
/* 外部技能正文（SKILL.md markdown 原文，保留换行/缩进） */
.skill-body { margin: 8px 0 2px; padding: 10px 12px; font-size: 12px; line-height: 1.65; color: var(--app-text-soft); background: var(--app-surface-3); border-radius: 8px; white-space: pre-wrap; word-break: break-word; font-family: inherit; max-height: 320px; overflow-y: auto; }
@media (max-width: 720px) {
  .catalog-toolbar { align-items: stretch; flex-wrap: wrap; }
  .catalog-source { max-width: none; flex: 1 1 100%; }
  .catalog-card { align-items: flex-start; flex-direction: column; }
  .catalog-install-btn { width: 100%; }
}

/* Profile（仿图2） */
.profile-row { display: flex; align-items: center; gap: 16px; padding: 9px 0; border-bottom: 1px solid var(--app-border-soft); }
.profile-label { flex-shrink: 0; width: 180px; font-size: 13px; color: var(--app-text); }
.profile-avatar { width: 40px; height: 40px; border-radius: 50%; background: var(--app-accent); color: #fff; display: flex; align-items: center; justify-content: center; font-size: 16px; font-weight: 700; }
.profile-input { flex: 1; min-width: 0; font-size: 13px; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 8px; padding: 8px 12px; }
.profile-input:focus { outline: none; border-color: var(--app-accent); }
/* 账号 UID：灰色小字 */
.profile-uid { font-size: 13px; color: var(--app-text-soft); }
.profile-uid.faint { color: var(--app-text-faint); }
/* 亲密等级：爱心表示 */
.profile-intimacy { display: flex; align-items: center; gap: 8px; }
.intimacy-hearts { font-size: 15px; line-height: 1; letter-spacing: 2px; color: #ff5d7e; }
.intimacy-level { font-size: 13px; font-weight: 600; color: var(--app-accent); }
/* 亲密度进度粉条 */
.intimacy-progress { display: inline-flex; align-items: center; gap: 6px; margin-left: 2px; }
.intimacy-progress-bar { width: 76px; height: 6px; border-radius: 3px; background: rgba(255, 93, 126, 0.18); overflow: hidden; }
.intimacy-progress-fill { display: block; height: 100%; border-radius: 3px; background: linear-gradient(90deg, #ff8fa8, #ff5d7e); transition: width .3s ease; }
.intimacy-progress-text { font-size: 11px; font-weight: 600; color: #ff5d7e; min-width: 32px; text-align: right; }
.profile-instructions { width: 100%; box-sizing: border-box; font-size: 13px; line-height: 1.6; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 10px; padding: 10px 12px; resize: vertical; font-family: inherit; }
.profile-instructions:focus { outline: none; border-color: var(--app-accent); }
.profile-actions { display: flex; align-items: center; justify-content: flex-end; gap: 12px; margin-top: 14px; }
.profile-saved { font-size: 12px; color: #12b76a; }

/* 外观实时预览：只展示当前主题结果，不承担第二套选择交互。 */
.appearance-mode-title { margin-top: 24px; }
.appearance-preview-title { margin-top: 24px; }
.theme-live-preview {
  --preview-bg: var(--app-bg);
  --preview-surface: var(--app-surface);
  --preview-surface-2: var(--app-surface-2);
  --preview-border: var(--app-border);
  --preview-text: var(--app-text);
  --preview-muted: var(--app-text-faint);
  overflow: hidden;
  min-height: 245px;
  color: var(--preview-text);
  background: var(--preview-bg);
  border: 1px solid var(--preview-border);
  border-radius: 14px;
  box-shadow: 0 10px 28px rgba(0,0,0,.06);
  transition: background .18s ease, color .18s ease, border-color .18s ease;
}
.theme-live-topbar {
  height: 42px; box-sizing: border-box; display: flex; align-items: center; justify-content: space-between;
  padding: 0 14px; background: var(--preview-surface); border-bottom: 1px solid var(--preview-border);
}
.theme-live-brand { display: inline-flex; align-items: center; gap: 6px; color: var(--preview-text); font-size: 11.5px; font-weight: 750; }
.theme-live-brand :deep(svg) { color: var(--preview-accent); }
.theme-live-status { display: inline-flex; align-items: center; gap: 6px; color: var(--preview-muted); font-size: 10px; }
.theme-live-status i { width: 6px; height: 6px; border-radius: 50%; background: var(--preview-accent); box-shadow: 0 0 0 3px var(--preview-accent-soft); }
.theme-live-body { display: grid; grid-template-columns: 128px 1fr; min-height: 202px; }
.theme-live-sidebar {
  display: flex; flex-direction: column; gap: 5px; padding: 13px 9px;
  background: var(--preview-surface-2); border-right: 1px solid var(--preview-border);
}
.theme-live-sidebar span { display: flex; align-items: center; gap: 7px; padding: 7px 9px; color: var(--preview-muted); border-radius: 7px; font-size: 10.5px; }
.theme-live-sidebar span.on { color: var(--preview-accent); background: var(--preview-accent-soft); font-weight: 700; }
.theme-live-main { display: flex; flex-direction: column; padding: 22px 24px 16px; background: var(--preview-surface); }
.theme-live-heading { color: var(--preview-text); font-size: 14px; font-weight: 750; }
.theme-live-copy { margin-top: 5px; color: var(--preview-muted); font-size: 10.5px; }
.theme-live-message {
  align-self: flex-start; margin-top: 20px; padding: 8px 11px; color: var(--preview-text);
  background: var(--preview-surface-2); border: 1px solid var(--preview-border); border-radius: 5px 11px 11px 11px; font-size: 10.5px;
}
.theme-live-composer {
  min-height: 36px; box-sizing: border-box; display: flex; align-items: center; gap: 10px;
  margin-top: auto; padding: 5px 6px 5px 11px; color: var(--preview-muted);
  background: var(--preview-surface); border: 1px solid var(--preview-border); border-radius: 10px; font-size: 10px;
}
.theme-live-composer span { flex: 1; }
.theme-live-composer b { width: 25px; height: 25px; display: grid; place-items: center; color: #fff; background: var(--preview-accent); border-radius: 7px; }
.api-form-btn.save:disabled { opacity: 0.6; cursor: default; }

/* 记忆 tab：卡片化单块展示，移除不再使用的分段说明样式 */
.memory-stack { display: flex; flex-direction: column; gap: 10px; }
.memory-card { border: 1px solid var(--app-border); border-radius: 12px; padding: 14px 16px; background: var(--app-surface-2); }
.memory-card-head { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.memory-card-title { font-size: 13px; font-weight: 700; color: var(--app-text); }
.memory-seg { margin-top: 10px; }
.memory-seg-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text);
  margin-bottom: 4px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.mem-bucket {
  font-family: var(--app-mono-font, monospace);
  font-size: 11px;
  font-weight: 600;
  color: var(--app-accent);
  background: var(--app-surface-3);
  border: 1px solid var(--app-border);
  border-radius: 6px;
  padding: 1px 7px;
}
.mem-empty-tag { color: var(--app-text-faint); font-weight: 400; font-size: 12px; }
.memory-md {
  margin-top: 0;
  padding: 14px 16px;
  background: var(--app-surface-2);
  border: 1px solid var(--app-border);
  border-radius: 12px;
  max-height: 44vh;
  overflow-y: auto;
}
.mem-emphasis { color: var(--app-accent); font-weight: 600; }
.memory-empty {
  padding: 14px 16px;
  font-size: 12.5px;
  color: var(--app-text-faint);
  background: var(--app-surface-2);
  border: 1px dashed var(--app-border);
  border-radius: 12px;
}
.version-value { color: var(--app-text); font-size: 12.5px; font-weight: 600; }
.version-new { color: var(--app-accent); }
.update-notes {
  flex: 1;
  min-height: 0;
  overflow: auto;
  color: var(--app-text-soft);
  font-size: 12.5px;
  line-height: 1.65;
  word-break: break-word;
  max-height: 220px;
}
.update-notes :deep(h1),
.update-notes :deep(h2),
.update-notes :deep(h3) {
  margin: 12px 0 6px;
  font-size: 13.5px;
  color: var(--app-text);
}
.update-notes :deep(h1:first-child),
.update-notes :deep(h2:first-child),
.update-notes :deep(h3:first-child) { margin-top: 0; }
.update-notes :deep(p) { margin: 6px 0; }
.update-notes :deep(ul),
.update-notes :deep(ol) { margin: 6px 0; padding-left: 20px; }
.update-notes :deep(code) {
  padding: 1px 5px;
  border-radius: 5px;
  background: var(--app-code-bg);
  font-family: var(--app-font);
  font-size: 11.5px;
}
.update-notes :deep(pre) { padding: 10px 12px; border-radius: 8px; background: var(--app-code-bg); overflow: auto; }
.update-notes :deep(pre code) { padding: 0; background: none; }
.update-notes :deep(a) { color: var(--app-accent); }

@media (max-width: 900px) {
  .settings-modal-card { width: calc(100vw - 28px); height: calc(100vh - 28px); border-radius: 16px; }
  .settings-sidebar { width: 176px; padding-inline: 10px; }
  .settings-content { padding-inline: 22px; }
}

@media (max-width: 640px) {
  .settings-modal-card { width: calc(100vw - 16px); height: calc(100vh - 16px); border-radius: 14px; }
  .settings-modal-header { min-height: 62px; padding: 11px 12px; }
  .settings-privacy-badge { display: none; }
  .settings-sidebar { width: 136px; padding: 12px 7px 18px; }
  .settings-nav-label { margin-left: 9px; font-size: 9px; }
  .settings-tab { min-height: 38px; padding-inline: 9px; font-size: 12px; }
  .settings-subtabs { margin-left: 13px; padding-left: 8px; }
  .settings-subtab { padding-inline: 7px; font-size: 11.5px; }
  .settings-content { padding-inline: 16px; }
  .param-row { align-items: flex-start; flex-direction: column; gap: 7px; padding: 10px 0; }
  .param-label { width: auto; }
  .model-select, .seg-control, .theme-swatches { width: 100%; max-width: none; margin-left: 0; }
  .seg-btn { flex: 1; }
  .theme-live-body { grid-template-columns: 84px 1fr; }
  .theme-live-sidebar { padding-inline: 6px; }
  .theme-live-sidebar span { padding-inline: 7px; }
  .theme-live-main { padding: 18px 14px 14px; }
  .theme-live-copy { display: none; }
}

@media (prefers-reduced-motion: reduce) {
  .settings-modal-card, .settings-panel { animation: none; }
  .settings-tab, .vendor-head { transition: none; }
}
</style>
