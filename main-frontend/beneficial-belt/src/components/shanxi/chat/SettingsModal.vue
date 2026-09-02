<template>
  <Teleport to="body">
    <div class="settings-modal-backdrop" @click="$emit('close')" @keydown.esc="$emit('close')">
      <div class="settings-modal-card" role="dialog" aria-modal="true" aria-label="Yosuri 设置" tabindex="-1" @click.stop>
        <div class="settings-modal-header">
          <div class="settings-brand">
            <span class="settings-brand-mark"><Icon icon="lucide:sparkles" width="17" /></span>
            <span class="settings-brand-copy">
              <strong>Yosuri</strong>
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
                                  class="settings-subtab"
                                  :class="{ locked: !apiUnlocked }"
                                  type="button"
                                  @click="openCustomLock"
                                >
                                  <Icon :icon="apiUnlocked ? 'mdi:lock-open-variant' : 'mdi:lock-outline'" width="15" />自定义 API
                                </button>
              </div>
            </div>
            <button class="settings-tab" :class="{ on: activeTab === 'aggapi' }" @click="activeTab = 'aggapi'">
              <Icon icon="mdi:api" width="16" />聚合 API</button>
            <div class="settings-nav-label">体验与能力</div>
            <button class="settings-tab" :class="{ on: activeTab === 'persona' }" @click="activeTab = 'persona'">
              <Icon icon="mdi:heart-cog-outline" width="16" />人设</button>
            <button class="settings-tab" :class="{ on: activeTab === 'appearance' }" @click="activeTab = 'appearance'">
              <Icon icon="mdi:palette-outline" width="16" />外观</button>
            <button class="settings-tab" :class="{ on: activeTab === 'editor' }" @click="activeTab = 'editor'">
              <Icon icon="mdi:code-tags" width="16" />编辑器</button>
            <button class="settings-tab" :class="{ on: activeTab === 'safety' }" @click="activeTab = 'safety'; loadProtectedWorkspace()">
              <Icon icon="mdi:shield-lock-outline" width="16" />安全</button>
            <div class="settings-tab-group">
              <button class="settings-tab" :class="{ on: activeTab === 'dhs' }" @click="activeTab = 'dhs'; loadDHS()">
                <Icon icon="simple-icons:deepseek" width="16" />DHS
              </button>
              <div v-show="activeTab === 'dhs'" class="settings-subtabs">
                <button class="settings-subtab" :class="{ on: dhsSubTab === 'installed' }" type="button" @click="dhsSubTab = 'installed'; loadDHS()">
                  <Icon icon="mdi:puzzle-check-outline" width="15" />已安装
                </button>
                <button class="settings-subtab" :class="{ on: dhsSubTab === 'ecosystem' }" type="button" @click="dhsSubTab = 'ecosystem'; loadDHSRegistry()">
                  <Icon icon="mdi:storefront-outline" width="15" />生态
                </button>
              </div>
            </div>
            <!-- Skills tab 暂时隐藏(2026-08-22 用户反馈"绝对有 bug", 先下线避免用户碰到,
                 代码原样保留在下面 settings-panel 里没删, 只是没有入口可以点进去了) -->
            <div class="settings-tab-group" v-if="false">
              <button class="settings-tab" :class="{ on: activeTab === 'skills' }" @click="activeTab = 'skills'; loadSkills()">
                <Icon icon="mdi:school-outline" width="16" />Skills
              </button>
              <div v-show="activeTab === 'skills'" class="settings-subtabs">
                <button class="settings-subtab" :class="{ on: skillsSubTab === 'local' }" type="button" @click="skillsSubTab = 'local'; loadSkills()">
                  <Icon icon="mdi:laptop" width="15" />本地
                </button>
                <button class="settings-subtab" :class="{ on: skillsSubTab === 'aggregate' }" type="button" @click="skillsSubTab = 'aggregate'; loadAggregateSkills()">
                  <Icon icon="mdi:swap-horizontal-bold" width="15" />聚合
                </button>
                <button class="settings-subtab" :class="{ on: skillsSubTab === 'external' }" type="button" @click="skillsSubTab = 'external'; loadSkillRegistry()">
                  <Icon icon="mdi:cloud-outline" width="15" />外部
                </button>
              </div>
            </div>
            <button class="settings-tab" :class="{ on: activeTab === 'memory' }" @click="activeTab = 'memory'; loadMemoryInject()">
              <Icon icon="mdi:notebook-outline" width="16" />记忆</button>
                          <button class="settings-tab" :class="{ on: activeTab === 'lan' }" @click="activeTab = 'lan'; loadLanSyncSetting()">
                            <Icon icon="mdi:lan-connect" width="16" />局域网</button>
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
                    <option value="custom">自定义模型（OpenAI 兼容，需 key）</option>
                    <option value="mcp">MCP 生图工具</option>
                  </select>
                </div>

                <!-- 生图：自定义模型配置卡片 -->
                <div v-if="imageProviderDraft === 'custom'" class="cap-card">
                  <div class="cap-card-title">自定义生图</div>
                  <div class="cap-field">
                    <span class="cap-label">Endpoint</span>
                    <input v-model="imageCustomEndpoint" type="text" class="cap-input" placeholder="https://api.example.com" @keyup.enter="saveImageCapability" />
                  </div>
                  <div class="cap-field">
                    <span class="cap-label">模型名</span>
                    <input v-model="imageCustomModel" type="text" class="cap-input" placeholder="如 gpt-image-1" @keyup.enter="saveImageCapability" />
                  </div>
                  <div class="cap-field">
                    <span class="cap-label">API Key</span>
                    <input v-model="imageKeyDraft" type="password" class="cap-input" :placeholder="imageKeySet ? '••••••••（留空不修改）' : '输入 API Key'" @keyup.enter="saveImageCapability" />
                  </div>
                  <div class="cap-actions">
                    <button class="vendor-key-save" type="button" @click="saveImageCapability">保存</button>
                  </div>
                </div>

                <!-- 生图：MCP 工具配置卡片 -->
                <div v-else-if="imageProviderDraft === 'mcp'" class="cap-card">
                  <div class="cap-card-title">MCP 生图工具</div>
                  <div class="cap-field">
                    <span class="cap-label">选择已装的 MCP 工具</span>
                    <select v-model="imageMCPTool" class="model-select">
                      <option value="">未选择</option>
                      <option v-for="t in mcpToolOptions" :key="t" :value="t">{{ t }}</option>
                    </select>
                  </div>
                  <div class="cap-actions">
                    <button class="vendor-key-save" type="button" @click="saveImageCapability">保存</button>
                  </div>
                </div>

                <div class="param-row">
                  <span class="param-label">联网来源</span>
                  <select class="model-select" v-model="websearchMode">
                    <option value="bing">Bing（免费，无 key）</option>
                    <option value="firecrawl">Firecrawl（免费 500 次/月）</option>
                    <option value="custom">自定义模型（OpenAI 兼容，需 key）</option>
                    <option value="mcp">MCP 搜索工具</option>
                  </select>
                </div>

                <!-- 联网：Bing（默认，免 key） -->
                <div v-if="websearchMode === 'bing'" class="cap-card">
                  <div class="cap-card-title">Bing 免 key 联网</div>
                  <div class="cap-card-desc">零配置即可联网搜索（国内可达），无需任何 API Key。</div>
                  <span class="firecrawl-key-status">✅ 联网搜索已启用（Bing，模型自主触发）</span>
                </div>

                <!-- 联网：Firecrawl 配置 -->
                <div v-else-if="websearchMode === 'firecrawl'" class="cap-card">
                  <div class="cap-card-title">Firecrawl API Key</div>
                  <div class="cap-field">
                    <span class="cap-label">fc- 开头的 Firecrawl API Key（联网搜索用）</span>
                    <input
                      v-model="firecrawlKeyDraft"
                      type="password"
                      class="cap-input"
                      placeholder="fc- 开头的 Firecrawl API Key"
                      @keyup.enter="saveFirecrawlKey"
                    />
                  </div>
                  <div class="cap-actions">
                    <button class="vendor-key-save" type="button" @click="saveFirecrawlKey">
                      {{ firecrawlKeySet ? '已配置 · 更新' : '保存' }}
                    </button>
                  </div>
                  <span v-if="firecrawlKeySet" class="firecrawl-key-status">✅ 联网搜索已启用（Firecrawl，模型自主触发）</span>
                </div>

                <!-- 联网：自定义模型配置 -->
                <div v-else-if="websearchMode === 'custom'" class="cap-card">
                  <div class="cap-card-title">自定义联网</div>
                  <div class="cap-field">
                    <span class="cap-label">Endpoint</span>
                    <input v-model="websearchEndpoint" type="text" class="cap-input" placeholder="https://api.deepseek.com" @keyup.enter="saveWebsearchCapability" />
                  </div>
                  <div class="cap-field">
                    <span class="cap-label">联网模型名（需支持服务端搜索）</span>
                    <input v-model="websearchModel" type="text" class="cap-input" placeholder="如 deepseek-chat" @keyup.enter="saveWebsearchCapability" />
                  </div>
                  <div class="cap-field">
                    <span class="cap-label">API Key</span>
                    <input v-model="websearchKeyDraft" type="password" class="cap-input" :placeholder="websearchKeySet ? '••••••••（留空不修改）' : '输入 API Key'" @keyup.enter="saveWebsearchCapability" />
                  </div>
                  <div class="cap-actions">
                    <button class="vendor-key-save" type="button" @click="saveWebsearchCapability">保存</button>
                  </div>
                  <span class="firecrawl-key-status">自定义联网走 Endpoint 的 /v1/responses（内置 web_search 工具），DeepSeek 等服务端联网模型可用。</span>
                </div>

                <!-- 联网：MCP 搜索工具配置 -->
                <div v-else class="cap-card">
                  <div class="cap-card-title">MCP 搜索工具</div>
                  <div class="cap-field">
                    <span class="cap-label">选择已装的 MCP 工具</span>
                    <select v-model="websearchMCPTool" class="model-select">
                      <option value="">未选择</option>
                      <option v-for="t in mcpToolOptions" :key="t" :value="t">{{ t }}</option>
                    </select>
                  </div>
                  <div class="cap-actions">
                    <button class="vendor-key-save" type="button" @click="saveWebsearchCapability">保存</button>
                  </div>
                  <span v-if="websearchMCPTool" class="firecrawl-key-status">✅ 联网搜索已指向 MCP 工具，模型自主触发</span>
                </div>

                <div class="param-row">
                  <span class="param-label">Agnes API Key（AI 生视频）</span>
                  <div class="search-model-row">
                    <input
                      v-model="agnesKeyDraft"
                      type="password"
                      class="vendor-key-input"
                      placeholder="Agnes API Key（AI 生视频/动漫分镜，$0/秒）"
                      @keyup.enter="saveAgnesKey"
                    />
                    <button class="vendor-key-save" type="button" @click="saveAgnesKey">
                      {{ agnesKeySet ? '已配置 · 更新' : '保存' }}
                    </button>
                  </div>
                  <span v-if="agnesKeySet" class="firecrawl-key-status">✅ AI 生视频已启用（Agnes，免费 $0/秒）</span>
                </div>
                <div class="settings-section-desc" style="margin-top:6px">
                  联网搜索是给模型的一个工具：需要最新信息时它自己决定搜（免费额度 500 次/月，firecrawl.dev 获取 Key）。
                  Agnes Key 在 platform.agnes-ai.com 免费获取，用于 AI 生成视频素材（动漫分镜/短片，$0/秒）。
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
                    <option value="custom">自定义模型（OpenAI 兼容，需 key）</option>
                    <option value="mcp">MCP 生图工具</option>
                  </select>
                </div>

                <!-- 生图：自定义模型配置卡片 -->
                <div v-if="imageProviderDraft === 'custom'" class="cap-card">
                  <div class="cap-card-title">自定义生图</div>
                  <div class="cap-field">
                    <span class="cap-label">Endpoint</span>
                    <input v-model="imageCustomEndpoint" type="text" class="cap-input" placeholder="https://api.example.com" @keyup.enter="saveImageCapability" />
                  </div>
                  <div class="cap-field">
                    <span class="cap-label">模型名</span>
                    <input v-model="imageCustomModel" type="text" class="cap-input" placeholder="如 gpt-image-1" @keyup.enter="saveImageCapability" />
                  </div>
                  <div class="cap-field">
                    <span class="cap-label">API Key</span>
                    <input v-model="imageKeyDraft" type="password" class="cap-input" :placeholder="imageKeySet ? '••••••••（留空不修改）' : '输入 API Key'" @keyup.enter="saveImageCapability" />
                  </div>
                  <div class="cap-actions">
                    <button class="vendor-key-save" type="button" @click="saveImageCapability">保存</button>
                  </div>
                </div>

                <!-- 生图：MCP 工具配置卡片 -->
                <div v-else-if="imageProviderDraft === 'mcp'" class="cap-card">
                  <div class="cap-card-title">MCP 生图工具</div>
                  <div class="cap-field">
                    <span class="cap-label">选择已装的 MCP 工具</span>
                    <select v-model="imageMCPTool" class="model-select">
                      <option value="">未选择</option>
                      <option v-for="t in mcpToolOptions" :key="t" :value="t">{{ t }}</option>
                    </select>
                  </div>
                  <div class="cap-actions">
                    <button class="vendor-key-save" type="button" @click="saveImageCapability">保存</button>
                  </div>
                </div>

                <div class="param-row">
                  <span class="param-label">联网来源</span>
                  <select class="model-select" v-model="websearchMode">
                    <option value="bing">Bing（免费，无 key）</option>
                    <option value="firecrawl">Firecrawl（免费 500 次/月）</option>
                    <option value="custom">自定义模型（OpenAI 兼容，需 key）</option>
                    <option value="mcp">MCP 搜索工具</option>
                  </select>
                </div>

                <!-- 联网：Bing（默认，免 key） -->
                <div v-if="websearchMode === 'bing'" class="cap-card">
                  <div class="cap-card-title">Bing 免 key 联网</div>
                  <div class="cap-card-desc">零配置即可联网搜索（国内可达），无需任何 API Key。</div>
                  <span class="firecrawl-key-status">✅ 联网搜索已启用（Bing，模型自主触发）</span>
                </div>

                <!-- 联网：Firecrawl 配置 -->
                <div v-else-if="websearchMode === 'firecrawl'" class="cap-card">
                  <div class="cap-card-title">Firecrawl API Key</div>
                  <div class="cap-field">
                    <span class="cap-label">fc- 开头的 Firecrawl API Key（联网搜索用）</span>
                    <input
                      v-model="firecrawlKeyDraft"
                      type="password"
                      class="cap-input"
                      placeholder="fc- 开头的 Firecrawl API Key"
                      @keyup.enter="saveFirecrawlKey"
                    />
                  </div>
                  <div class="cap-actions">
                    <button class="vendor-key-save" type="button" @click="saveFirecrawlKey">
                      {{ firecrawlKeySet ? '已配置 · 更新' : '保存' }}
                    </button>
                  </div>
                  <span v-if="firecrawlKeySet" class="firecrawl-key-status">✅ 联网搜索已启用（Firecrawl，模型自主触发）</span>
                </div>

                <!-- 联网：自定义模型配置 -->
                <div v-else-if="websearchMode === 'custom'" class="cap-card">
                  <div class="cap-card-title">自定义联网</div>
                  <div class="cap-field">
                    <span class="cap-label">Endpoint</span>
                    <input v-model="websearchEndpoint" type="text" class="cap-input" placeholder="https://api.deepseek.com" @keyup.enter="saveWebsearchCapability" />
                  </div>
                  <div class="cap-field">
                    <span class="cap-label">联网模型名（需支持服务端搜索）</span>
                    <input v-model="websearchModel" type="text" class="cap-input" placeholder="如 deepseek-chat" @keyup.enter="saveWebsearchCapability" />
                  </div>
                  <div class="cap-field">
                    <span class="cap-label">API Key</span>
                    <input v-model="websearchKeyDraft" type="password" class="cap-input" :placeholder="websearchKeySet ? '••••••••（留空不修改）' : '输入 API Key'" @keyup.enter="saveWebsearchCapability" />
                  </div>
                  <div class="cap-actions">
                    <button class="vendor-key-save" type="button" @click="saveWebsearchCapability">保存</button>
                  </div>
                  <span class="firecrawl-key-status">自定义联网走 Endpoint 的 /v1/responses（内置 web_search 工具），DeepSeek 等服务端联网模型可用。</span>
                </div>

                <!-- 联网：MCP 搜索工具配置 -->
                <div v-else class="cap-card">
                  <div class="cap-card-title">MCP 搜索工具</div>
                  <div class="cap-field">
                    <span class="cap-label">选择已装的 MCP 工具</span>
                    <select v-model="websearchMCPTool" class="model-select">
                      <option value="">未选择</option>
                      <option v-for="t in mcpToolOptions" :key="t" :value="t">{{ t }}</option>
                    </select>
                  </div>
                  <div class="cap-actions">
                    <button class="vendor-key-save" type="button" @click="saveWebsearchCapability">保存</button>
                  </div>
                  <span v-if="websearchMCPTool" class="firecrawl-key-status">✅ 联网搜索已指向 MCP 工具，模型自主触发</span>
                </div>

                <div class="param-row">
                  <span class="param-label">Agnes API Key（AI 生视频）</span>
                  <div class="search-model-row">
                    <input
                      v-model="agnesKeyDraft"
                      type="password"
                      class="vendor-key-input"
                      placeholder="Agnes API Key（AI 生视频/动漫分镜，$0/秒）"
                      @keyup.enter="saveAgnesKey"
                    />
                    <button class="vendor-key-save" type="button" @click="saveAgnesKey">
                      {{ agnesKeySet ? '已配置 · 更新' : '保存' }}
                    </button>
                  </div>
                  <span v-if="agnesKeySet" class="firecrawl-key-status">✅ AI 生视频已启用（Agnes，免费 $0/秒）</span>
                </div>
                <div class="settings-section-desc" style="margin-top:6px">
                  联网搜索是给模型的一个工具：需要最新信息时它自己决定搜（免费额度 500 次/月，firecrawl.dev 获取 Key）。
                  Agnes Key 在 platform.agnes-ai.com 免费获取，用于 AI 生成视频素材（动漫分镜/短片，$0/秒）。
                </div>
              </template>
            </div>

            <!-- ========== 提供方 ========== -->
            <div v-show="activeTab === 'providers'" class="settings-panel">
              <template v-if="providerSubTab === 'free'">
                <div class="settings-section-title settings-section-title-row">
                  <span>免费模型</span>
                </div>
                <div class="settings-section-desc">配置提供方的 Key 后，它的全部模型会自动进入聊天下拉框；点击「官网获取 Key」打开官网登录即可免费领取 API Key，粘贴输入框即可使用；免 Key 提供方无需配置。</div>

                <div v-if="loading" class="settings-loading">加载中...</div>
                <template v-else>
                  <div v-for="grp in vendorGroups" :key="grp.vendor" class="vendor-group">
                    <div
                      class="vendor-head"
                      role="button"
                      tabindex="0"
                      :aria-expanded="isVendorOpen(grp.vendor)"
                      @click="toggleVendor(grp.vendor)"
                      @keydown.enter.prevent="toggleVendor(grp.vendor)"
                      @keydown.space.prevent="toggleVendor(grp.vendor)"
                    >
                      <Icon class="vendor-chevron" :class="{ open: isVendorOpen(grp.vendor) }" icon="mdi:chevron-right" width="17" />
                      <span class="vendor-logo" :style="{ '--vendor-color': vendorColor(grp.vendor) }">
                        <Icon :icon="vendorIcon(grp.vendor)" width="16" />
                      </span>
                      <span class="vendor-name">{{ grp.vendor }}</span>
                      <span class="vendor-count">{{ grp.items.length }} 个模型</span>
                      <span class="vendor-keystate" :class="{ on: grp.hasKey, free: grp.keyless }">{{ grp.keyless ? '免 Key' : (grp.hasKey ? '已配 Key' : '未配 Key') }}</span>
                      <a v-if="grp.keyUrl && !grp.keyless" class="vendor-key-btn vendor-key-link" :href="grp.keyUrl" target="_blank" rel="noopener" title="打开官网登录即可免费获取 API Key" @click.stop>官网获取 Key ↗</a>
                      <button v-if="!grp.keyless && editingVendor !== grp.vendor" class="vendor-key-btn" @click.stop="startEditVendor(grp)">{{ grp.hasKey ? '改 Key' : '填 Key' }}</button>
                      <button v-else-if="editingVendor === grp.vendor" class="vendor-key-btn" @click.stop="cancelVendorEdit">收起</button>
                    </div>
                    <div v-show="isVendorOpen(grp.vendor)" class="vendor-body">
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
                  </div>
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
                      <button class="api-preset-btn" :class="{ active: activePreset === '__custom__' }" type="button" @click="applyCustomPreset">自定义</button>
                      <button v-for="p in PRESETS" :key="p.name" class="api-preset-btn" :class="{ active: activePreset === p.name }" type="button" @click="applyPreset(p)">{{ p.name }}</button>
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
                  <button class="agg-api-copy" type="button" @click="copyAggText('http://localhost:8080/v1', 'Base URL')">复制</button>
                </div>
                <div class="agg-api-row">
                  <span class="agg-api-label">API Key</span>
                  <code class="agg-api-code">sk-rescene-local</code>
                  <button class="agg-api-copy" type="button" @click="copyAggText('sk-rescene-local', 'API Key')">复制</button>
                </div>
                <div class="agg-api-row">
                  <span class="agg-api-label">本地代理端口</span>
                  <input v-model="aggLocalProxyPort" class="agg-proxy-input" type="number" min="1" max="65535" placeholder="如 9910（留空=自动探测）" @blur="onAggProxyPortBlur()" @keyup.enter="onAggProxyPortBlur()" />
                </div>
              </div>
              <Transition name="agg-copy-toast">
                <div v-if="aggCopyFeedback" class="agg-copy-feedback" :class="{ error: !aggCopyFeedback.ok }" role="status" aria-live="polite">
                  <Icon :icon="aggCopyFeedback.ok ? 'mdi:check-circle-outline' : 'mdi:alert-circle-outline'" width="16" />
                  <span>{{ aggCopyFeedback.message }}</span>
                </div>
              </Transition>
              <div class="agg-api-tip">已聚合 {{ freeModels.length + customModels.length }} 个模型（免费池 + 自定义）。model 填 <code class="agg-api-code">auto</code> 自动路由，或填任意模型 ID；key 可用 RESCENE_AGG_API_KEY 环境变量修改。</div>

              <!-- ===== 一键同步 / 还原（codex / dsh）：两个工具都还没做好，整卡片先藏起来 ===== -->
                            <div v-if="showCodex || showDsh" class="agg-sync-card">
                              <div class="agg-sync-head">
                                <span class="agg-sync-title">一键同步到本地工具</span>
                                <div class="agg-sync-actions">
                                  <button v-if="showCodex" class="agg-sync-btn" type="button" :disabled="aggSyncing === 'codex'" @click="aggSyncOne('codex')">
                                    {{ aggSyncing === 'codex' ? '同步中…' : (isSynced('codex') ? '已同步 codex' : '未同步 codex') }}
                                  </button>
                                  <button v-if="showDsh" class="agg-sync-btn" type="button" :disabled="aggSyncing === 'dsh'" @click="aggSyncOne('dsh')">
                                    {{ aggSyncing === 'dsh' ? '同步中…' : (isSynced('dsh') ? '已同步 dsh' : '未同步 dsh') }}
                                  </button>
                                  <button v-if="showCodex" class="agg-sync-btn ghost" type="button" :disabled="aggRestoring === 'codex'" @click="aggRestoreOne('codex')">
                                    {{ aggRestoring === 'codex' ? '还原中…' : '还原 codex' }}
                                  </button>
                                  <button v-if="showDsh" class="agg-sync-btn ghost" type="button" :disabled="aggRestoring === 'dsh'" @click="aggRestoreOne('dsh')">
                                    {{ aggRestoring === 'dsh' ? '还原中…' : '还原 dsh' }}
                                  </button>
                                </div>
                              </div>
                              <div class="agg-sync-tip">每个工具独立操作：点「同步 codex / dsh」只生成对应配置片段；点「还原 codex / dsh」只还原对应工具到原始配置（首次写回前自动备份一次）。片段旁「写入配置」才真正落盘（写前自动备份）。</div>
                              <div v-if="aggSyncResult.length" class="agg-sync-result">
                                <div v-for="r in aggSyncResult" :key="r.tool" class="agg-sync-item" v-show="r.tool !== 'codex'">
                                  <div class="agg-sync-item-head">
                                    <span class="agg-sync-tool">{{ r.tool }}</span>
                                    <span class="agg-sync-badge" :class="{ ok: r.ok, err: !!r.error }">{{ r.error ? '失败' : (r.applied ? '已写入' : '已生成') }}</span>
                                  </div>
                                  <pre class="agg-sync-snippet">{{ aggSnippetOf(r.tool) }}</pre>
                                  <div class="agg-sync-item-actions">
                                    <button class="agg-sync-mini" type="button" @click="copyAggText(aggSnippetOf(r.tool))">复制片段</button>
                                    <button v-if="!r.applied && !r.error" class="agg-sync-mini primary" type="button" :disabled="aggWriting" @click="aggApply(r.tool)">写入配置</button>
                                    <button v-if="r.backed_up" class="agg-sync-mini" type="button" disabled>已备份: {{ basename(r.backed_up) }}</button>
                                  </div>
                                  <div v-if="r.error" class="agg-sync-err">{{ r.error }}</div>
                                </div>
                              </div>
                            </div>

                            <!-- ===== 聚合模型选择（官方全量免费模型 / 用户自定义命名标签）===== -->
                            <div class="agg-api-card" style="margin-top:10px">
                              <div class="agg-api-row">
                                <div class="agg-mode-tabs">
                                  <button type="button" :class="{ on: aggMode === 'official' }" @click="switchAggOfficial()">官方</button>
                                  <template v-for="t in customTags" :key="t.id">
                                    <input v-if="aggMode === t.id" class="agg-tag-input" v-model="t.name" @blur="onTagRename(t)" @keyup.enter="onTagRename(t)" @click.stop="selectTag(t)" :title="'可改名'" />
                                    <button v-else type="button" :class="{ on: aggMode === t.id }" @click="selectTag(t)">{{ t.name }}</button>
                                    <span v-if="customTags.length > 1" class="agg-tag-del" @click="removeCustomTag(t.id)">×</span>
                                  </template>
                                  <button type="button" class="agg-tag-add" @click="addCustomTag">+</button>
                                </div>
                              </div>
                              <div class="agg-api-tip">官方 = 自动收录全部已配置 Key 的免费模型，auto 智能路由自动挑可用的，无需手动选择；用户自定义 = 自己勾选任意模型进聚合端口，标签名可改名、可 + 新增。修改实时保存，无需点保存。</div>
                              <template v-if="aggMode !== 'official'">
                                <input v-model="aggCfgSearch" class="agg-cfg-search" placeholder="搜索模型名 / 厂商…" />
                                <div class="agg-cfg-list">
                                  <div v-for="g in aggCfgGroups" :key="g.vendor" class="agg-cfg-group">
                                    <div class="agg-cfg-group-head">
                                      <button type="button" class="agg-cfg-toggle" @click="toggleAggGroup(g.vendor)">
                                        <span class="agg-cfg-chevron" :class="{ open: aggOpen[g.vendor] }">▸</span>
                                        <span class="agg-cfg-vendor">{{ g.vendor }}</span>
                                        <span class="agg-cfg-count">{{ g.items.length }}</span>
                                      </button>
                                      <button type="button" class="agg-cfg-select-all" @click="toggleAggGroupAll(g.vendor)">{{ aggGroupAllState(g.vendor) ? '取消全选' : '全选' }}</button>
                                    </div>
                                    <div v-if="aggOpen[g.vendor] !== false" class="agg-cfg-group-body">
                                      <div v-for="c in g.items" :key="c.id" class="agg-cfg-item-wrap">
                                                                              <label class="agg-cfg-item" :class="{ off: !c.key_set, dead: c.disabled }">
                                                                                <input type="checkbox" :value="c.id" v-model="aggModelIDs" :disabled="!c.key_set" />
                                                                                <span class="agg-cfg-name" :title="c.model">{{ c.name }}</span>
                                                                                <span class="agg-cfg-model">{{ c.model }}</span>
                                                                                <span v-if="c.disabled" class="agg-cfg-dead" title="该模型当前判定不可用，但可选回实验">已淘汰</span>
                                                                                <button v-if="reviewCountId(c.id)" class="agg-cfg-info" type="button" :class="{ on: aggReviewOpen === c.id }" :title="'大众点评（' + reviewCountId(c.id) + ' 条）'" @click="aggReviewOpen = aggReviewOpen === c.id ? '' : c.id">
                                                                                  <span class="agg-cfg-info-stars">{{ renderStars(avgStarsId(c.id)) }}</span>
                                                                                  <span class="agg-cfg-info-num">{{ avgStarsId(c.id).toFixed(1) }}</span>
                                                                                </button>
                                                                                <span v-if="!c.chat" class="agg-cfg-nochat">非对话</span>
                                                                                <span v-if="!c.key_set" class="agg-cfg-nokey">未配 key</span>
                                                                              </label>
                                        <div v-show="aggReviewOpen === c.id" class="agg-review-card">
                                          <div class="agg-review-card-head">
                                            <span class="agg-review-model">{{ c.name }}</span>
                                            <span class="agg-review-avg">{{ avgStarsId(c.id).toFixed(1) }}</span>
                                            <span class="agg-review-stars">{{ renderStars(avgStarsId(c.id)) }}</span>
                                            <span class="agg-review-total">{{ reviewCountId(c.id) }} 条点评</span>
                                          </div>
                                          <div class="agg-review-list">
                                            <div v-for="(rv, ri) in reviewsOfId(c.id)" :key="ri" class="agg-review-item">
                                              <div class="agg-review-item-head">
                                                <span class="agg-review-user">{{ rv.user }}</span>
                                                <span class="agg-review-stars">{{ renderStars(rv.stars) }}</span>
                                              </div>
                                              <div class="agg-review-text">{{ rv.text }}</div>
                                            </div>
                                          </div>
                                        </div>
                                      </div>
                                    </div>
                                  </div>
                                  <div v-if="!aggCfgGroups.length" class="settings-empty">没有匹配的模型。</div>
                                </div>
                              </template>
                            </div>

                            <!-- ===== 聚合池健康度可视化 ===== -->
                            <div class="agg-health-card">
                              <div class="agg-health-head">
                                <span class="agg-health-title">聚合池健康度</span>
                                <span v-if="aggHealthLoaded" class="agg-health-summary">
                                  <span class="agg-health-dot" :class="{ ok: aggHealthOK > 0 }">{{ aggHealthOK }}</span> 可用 /
                                  <span class="agg-health-dot warn" :class="{ on: aggHealthDown > 0 }">{{ aggHealthDown }}</span> 异常
                                </span>
                                <button class="agg-api-copy" type="button" :disabled="aggHealthLoading" @click="loadAggHealth()">
                                  {{ aggHealthLoading ? '刷新中…' : '刷新' }}
                                </button>
                              </div>
                              <div v-if="aggHealthLoading" class="settings-loading">加载健康度...</div>
                              <div v-else-if="aggHealthError" class="agg-health-error">
                                ⚠️ Yosuri 桌面应用未启动或当前 Agent 连接配置错误
                                <div class="agg-health-error-sub">请确认 Yosuri 桌面应用已运行、本地聚合端口未被占用；如刚改过配置，点右上角「刷新」重试。</div>
                              </div>
                              <template v-else>
                                <div v-if="!aggHealthModels.length" class="settings-empty">没有可展示的模型（聚合端口不暴露任何模型时为空）。</div>
                                <template v-else>
                                  <!-- auto 路由链（最优先展示） -->
                                  <div v-if="aggAutoChain.length" class="agg-health-block">
                                    <div class="agg-health-block-title">auto 路由链（命中优先）</div>
                                    <div v-for="m in aggAutoChain" :key="m.id" class="agg-health-row" :class="{ bad: m.disabled }">
                                      <span class="fm-signal" :class="'sig-' + (m.signal == null ? -1 : m.signal)">
                                        <i v-for="n in 4" :key="n" :class="{ on: (m.signal == null ? -1 : m.signal) >= n || n === 1 }"></i>
                                      </span>
                                      <span class="agg-health-vendor">{{ m.vendor }}</span>
                                      <span class="agg-health-name" :title="m.model">{{ m.name }}</span>
                                      <span class="agg-health-order">#{{ m.auto_order }}</span>
                                      <span class="agg-health-latency" :class="latencyClass(m)">
                                        <i class="agg-health-bar" :style="{ width: latencyWidth(m) }"></i>
                                        <b>{{ latencyText(m) }}</b>
                                      </span>
                                    </div>
                                  </div>
                                  <!-- 全部暴露模型 -->
                                  <div class="agg-health-block">
                                    <div class="agg-health-block-title">全部暴露模型（{{ aggHealthModels.length }}）</div>
                                    <div v-for="m in aggHealthModels" :key="m.id" class="agg-health-row" :class="{ bad: m.disabled }">
                                      <span class="fm-signal" :class="'sig-' + (m.signal == null ? -1 : m.signal)">
                                        <i v-for="n in 4" :key="n" :class="{ on: (m.signal == null ? -1 : m.signal) >= n || n === 1 }"></i>
                                      </span>
                                      <span class="agg-health-vendor">{{ m.vendor }}</span>
                                      <span class="agg-health-name" :title="m.model">{{ m.name }}</span>
                                      <span class="agg-health-latency" :class="latencyClass(m)">
                                        <i class="agg-health-bar" :style="{ width: latencyWidth(m) }"></i>
                                        <b>{{ latencyText(m) }}</b>
                                      </span>
                                    </div>
                                  </div>
                                  <div class="agg-health-foot">探活每日一轮（免 key 网关，零成本），信号格 0-4：绿=快(≤3s) 黄=中(≤8s) 红=慢(>8s) 灰=未探测。</div>
                                </template>
                              </template>
                            </div>
                          </div>

            <!-- ========== 人设 ========== -->
                        <div v-show="activeTab === 'persona'" class="settings-panel">
                                      <div class="settings-section-title settings-section-title-row">
                                        <span>人设</span>
                                        <button class="persona-report-btn" type="button" @click="personaReportPost">
                                                                          <Icon icon="mdi:email-fast-outline" width="14" /> 周报投递
                                                                        </button>
                                      </div>
                          <div class="settings-section-desc">
                            当前人设文案显示在下面，直接改内容再点「保存」即可；也可以存成预设，或者每天随机换一个。
                          </div>
                          <textarea
                            v-model="personaDraft"
                            class="persona-textarea"
                            rows="5"
                            placeholder="写下你自己的 AI 人设，比如：你是冷面可靠的工作助手，话少、直接、从不客套……"
                            @focus="personaEditing = true"
                            @input="personaEditing = true"
                          ></textarea>

                          <!-- 编辑态操作行 -->
                          <div class="persona-actions" v-if="personaEditing">
                            <button class="vendor-key-save" type="button" @click="saveCustomPersona">保存</button>
                            <button class="vendor-key-save" type="button" style="margin-left: 8px;" @click="personaSavingPreset = !personaSavingPreset">
                              {{ personaSavingPreset ? '取消存预设' : '保存为预设' }}
                            </button>
                            <button class="vendor-key-cancel" type="button" style="margin-left: 8px;" @click="clearPersona">清除人设（中性助手）</button>
                          </div>
                          <!-- 保存为预设：命名行 -->
                          <div class="persona-save-preset-row" v-if="personaSavingPreset">
                            <input
                              v-model="newPresetName"
                              class="persona-preset-name-input"
                              type="text"
                              placeholder="预设名字，比如：我的老板人设"
                              @keyup.enter="saveAsPreset"
                            />
                            <button class="vendor-key-save" type="button" @click="saveAsPreset">存进「我的预设」</button>
                          </div>

                          <div class="persona-divider"></div>

                          <!-- 每日随机 -->
                          <button
                            class="persona-preset-card persona-random-card"
                            :class="{ on: personaSelected === 'random' }"
                            type="button"
                            @click="selectRandomPreset"
                          >
                            <Icon :icon="RANDOM_PRESET.icon" width="22" class="persona-preset-icon" />
                            <span class="persona-preset-name">{{ RANDOM_PRESET.name }}</span>
                            <span class="persona-preset-desc">{{ RANDOM_PRESET.desc }}</span>
                          </button>

                          <!-- 内置预设 -->
                          <div class="persona-group-title">预设</div>
                          <div class="persona-preset-grid">
                            <button
                              v-for="p in BUILTIN_PRESETS"
                              :key="p.id"
                              class="persona-preset-card"
                              :class="{ on: personaSelected === p.id }"
                              type="button"
                              @click="selectPersonaPreset(p)"
                            >
                              <Icon :icon="p.icon" width="22" class="persona-preset-icon" />
                              <span class="persona-preset-name">{{ p.name }}</span>
                              <span class="persona-preset-desc">{{ p.desc }}</span>
                            </button>
                          </div>

                          <!-- 我的预设 -->
                          <template v-if="myPersonas.length">
                            <div class="persona-group-title">我的预设</div>
                            <div class="persona-preset-grid">
                              <button
                                v-for="p in myPersonas"
                                :key="p.id"
                                class="persona-preset-card"
                                :class="{ on: personaSelected === p.id }"
                                type="button"
                                @click="selectPersonaPreset(p)"
                              >
                                <Icon icon="mdi:account-heart" width="22" class="persona-preset-icon" />
                                <span class="persona-preset-name">{{ p.name }}</span>
                                <span class="persona-preset-desc">自定义预设</span>
                                <span class="persona-preset-del" title="删除预设" @click="deleteMyPreset(p.id, $event)">×</span>
                              </button>
                            </div>
                          </template>

                          <Transition name="persona-toast">
                            <div v-if="personaToast" class="persona-toast" :class="{ error: !personaToast.ok }" role="status" aria-live="polite">
                              <Icon :icon="personaToast.ok ? 'mdi:check-circle-outline' : 'mdi:alert-circle-outline'" width="16" />
                              <span>{{ personaToast.message }}</span>
                            </div>
                          </Transition>
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

              <div class="settings-section-title appearance-mode-title">皮肤</div>
              <div class="settings-section-desc">动画工作台皮肤保留专业 IDE 布局，只切换成套色板与状态标记。</div>
              <div class="param-row" style="align-items: flex-start;">
                <span class="param-label">工作台皮肤</span>
                <div class="skin-groups">
                  <div v-for="[series, items] in skinThemes" :key="series" class="skin-group">
                    <div class="skin-group-title">{{ series }}</div>
                    <div class="skin-cards">
                      <button
                        v-for="[key, p] in items"
                        :key="key"
                        class="skin-card"
                        :class="{ on: theme === key }"
                        type="button"
                        :title="`切换到${p.label}皮肤`"
                        @click="selectTheme(key)"
                      >
                        <span
                          class="skin-card-preview"
                          :class="`skin-preview-${key}`"
                          :style="{ '--skin-accent': p.accent, '--skin-secondary': p.accent2, '--skin-surface': p.skin.light.surface }"
                        >
                          <span class="skin-preview-sidebar"><i></i><i></i><i></i></span>
                          <span class="skin-preview-editor"><i></i><i></i><i></i><b></b></span>
                        </span>
                        <span class="skin-card-info">
                          <strong>{{ p.label }}</strong>
                          <small>{{ key === 'witchtrial' ? '珊瑚红 × 靛青 · 原画工作流' : '青碧 × 夜紫 · 夜场工作流' }}</small>
                        </span>
                        <Icon v-if="theme === key" class="skin-card-check" icon="mdi:check" width="17" />
                      </button>
                    </div>
                  </div>
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
                  <span class="theme-live-brand"><Icon icon="lucide:sparkles" width="13" />Yosuri</span>
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

              <div class="settings-section-title appearance-mode-title">悬浮球演示模式</div>
              <div class="settings-section-desc">
                测试功能，默认关闭：打开后主窗口最小化到托盘时会出现一个可拖拽的悬浮球，
                点击展开面板实时显示 Agent 当前意图和操作，适合录屏演示。改动需要重启应用才生效。
              </div>
              <div class="param-row">
                <span class="param-label">悬浮球</span>
                <div class="seg-control">
                  <button
                    class="seg-btn"
                    :class="{ on: !overlayEnabled }"
                    type="button"
                    :disabled="overlaySaving"
                    @click="setOverlayEnabled(false)"
                  >关闭</button>
                  <button
                    class="seg-btn"
                    :class="{ on: overlayEnabled }"
                    type="button"
                    :disabled="overlaySaving"
                    @click="setOverlayEnabled(true)"
                  >开启</button>
                </div>
              </div>
            </div>

                        <!-- ========== 编辑器 ========== -->
                        <div v-show="activeTab === 'editor'" class="settings-panel">
                          <div class="settings-section-title">代码编辑器</div>
                          <div class="settings-section-desc">
                            打开项目时按需加载代码编辑器（Monaco），降低低配机器启动卡顿。
                            关闭后应用启动即后台预取编辑器，打开文件面板秒开（高配机/重度使用编辑器时可选）。
                          </div>
                          <div class="param-row">
                            <span class="param-label">按需加载（懒加载）</span>
                            <div class="seg-control">
                              <button class="seg-btn" :class="{ on: editorLazyEnabled }" type="button" @click="setEditorLazy(true)">开启（推荐）</button>
                              <button class="seg-btn" :class="{ on: !editorLazyEnabled }" type="button" @click="setEditorLazy(false)">关闭</button>
                            </div>
                          </div>
                        </div>

                        <!-- ========== 受保护工作区 ========== -->
            <div v-show="activeTab === 'safety'" class="settings-panel">
              <div class="settings-section-title">受保护工作区</div>
              <div class="settings-section-desc">
                默认关闭。开启后，Agent 的文件工具和 filesystem MCP 只能访问当前项目目录；越界文件访问会被直接拒绝，即使在 Yolo 模式下也是如此。写盘和命令仍会要求本次明确批准。它是应用层保护，不是操作系统沙盒。
              </div>
              <div class="param-row">
                <span class="param-label">保护模式</span>
                <div class="seg-control">
                  <button class="seg-btn" :class="{ on: !protectedWorkspaceEnabled }" type="button" :disabled="protectedWorkspaceSaving" @click="setProtectedWorkspace(false)">关闭</button>
                  <button class="seg-btn" :class="{ on: protectedWorkspaceEnabled }" type="button" :disabled="protectedWorkspaceSaving" @click="setProtectedWorkspace(true)">开启</button>
                </div>
              </div>
              <div v-if="protectedWorkspaceError" class="settings-error">{{ protectedWorkspaceError }}</div>
            </div>

            <!-- ========== DeepSeek Harness ecosystem ========== -->
            <div v-show="activeTab === 'dhs'" class="settings-panel">
              <template v-if="dhsSubTab === 'installed'">
                <div class="settings-section-title">
                  已安装的 DHS 插件
                  <button class="inline-refresh" type="button" @click="loadDHS(true)" title="刷新"><Icon icon="mdi:refresh" width="14" :class="{ spin: dhsLoading }" /></button>
                </div>
                <div class="settings-section-desc">
                  DHS 插件是会被 Agent Harness 直接加载的能力包。Go 内置工具保持常驻，插件只扩展工作流、知识与交付规范。
                </div>
                <div v-if="dhsLoading" class="settings-loading">加载中...</div>
                <template v-else>
                  <div v-if="!dhsInstalled.length" class="settings-empty">还没有 DHS 插件。可到「生态」选择能力包。</div>
                  <div v-for="plugin in dhsInstalled" :key="plugin.name" class="entity-card">
                    <div class="entity-head">
                      <Icon icon="simple-icons:deepseek" width="15" />
                      <span class="entity-name">{{ plugin.name }}</span>
                      <span class="entity-badge dhs-state">Harness 已加载</span>
                      <span v-if="plugin.provider" class="entity-badge">{{ plugin.provider }}</span>
                    </div>
                    <div class="entity-meta">{{ plugin.description || 'DHS Harness 能力包' }}</div>
                    <div class="skill-actions">
                      <button class="danger" type="button" :disabled="catalogBusy === 'dhs-remove:' + plugin.name" @click="uninstallDHS(plugin)">
                        {{ catalogBusy === 'dhs-remove:' + plugin.name ? '移除中…' : '移除' }}
                      </button>
                    </div>
                  </div>
                </template>
              </template>
              <template v-else>
                <div class="settings-section-title">DeepSeek Harness 插件生态</div>
                <div class="settings-section-desc">浏览 DHS 能力包。安装内容会先经过文件数、体积和路径安全校验，再原子写入本地 Harness 目录。</div>
                <div class="catalog-toolbar">
                  <label class="catalog-search">
                    <Icon icon="mdi:magnify" width="16" />
                    <input v-model="dhsRegistryQuery" type="search" placeholder="搜索 DHS 插件" @keyup.enter="loadDHSRegistry(true)" />
                  </label>
                  <button class="catalog-search-btn" type="button" @click="loadDHSRegistry(true)">搜索</button>
                </div>
                <div v-if="dhsRegistryLoading" class="settings-loading">正在连接 DHS 插件目录…</div>
                <template v-else>
                  <div v-if="!dhsRegistryItems.length" class="settings-empty">没有找到 DHS 插件。</div>
                  <div v-for="item in dhsRegistryItems" :key="item.path" class="catalog-card">
                    <div class="catalog-card-main">
                      <div class="entity-head">
                        <Icon icon="simple-icons:deepseek" width="15" />
                        <span class="entity-name">{{ item.name }}</span>
                        <span class="entity-badge">DHS</span>
                      </div>
                      <div class="catalog-id">{{ item.path }}</div>
                      <div class="catalog-desc">可安装的 DeepSeek Harness 能力包；安装后随 Agent 工作流自动加载。</div>
                      <div class="entity-meta">{{ item.url }}</div>
                    </div>
                    <button
                      class="catalog-install-btn"
                      :class="{ installed: item.installed }"
                      type="button"
                      :disabled="item.installed || catalogBusy === 'dhs-install:' + item.path"
                      @click="installDHS(item)"
                    >{{ item.installed ? '已安装' : (catalogBusy === 'dhs-install:' + item.path ? '安装中…' : '安装') }}</button>
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
              <template v-else-if="skillsSubTab === 'aggregate'">
                <div class="settings-section-title">
                  Skills 聚合管理
                  <button class="inline-refresh" type="button" @click="loadAggregateSkills(true)" title="重新扫描"><Icon icon="mdi:refresh" width="14" :class="{ spin: aggregateSkillsLoading }" /></button>
                </div>
                <div class="settings-section-desc">
                  像 CC Switch 一样统一管理 Hermes、Claude 与 Codex 的本地技能。选择任一版本作为来源，可把整个技能包同步到其他端；覆盖前自动备份。
                </div>
                <div class="skill-platform-grid">
                  <button
                    v-for="platform in aggregatePlatforms"
                    :key="platform.id"
                    class="skill-platform-card"
                    :class="{ on: aggregatePlatformFilter === platform.id, unavailable: !platform.available }"
                    type="button"
                    @click="aggregatePlatformFilter = aggregatePlatformFilter === platform.id ? 'all' : platform.id"
                  >
                    <span class="skill-platform-icon"><Icon :icon="aggregatePlatformIcon(platform.id)" width="18" /></span>
                    <span><strong>{{ platform.label }}</strong><small>{{ platform.count }} 个技能</small></span>
                    <i :class="{ live: platform.available }"></i>
                  </button>
                </div>
                <label class="catalog-search aggregate-search">
                  <Icon icon="mdi:magnify" width="16" />
                  <input v-model="aggregateSkillQuery" type="search" placeholder="搜索三端技能" />
                  <span>{{ filteredAggregateSkills.length }} / {{ aggregateSkills.length }}</span>
                </label>
                <div v-if="aggregateSkillsLoading" class="settings-loading">正在扫描三端技能目录…</div>
                <template v-else>
                  <div v-if="!filteredAggregateSkills.length" class="settings-empty">没有找到匹配的技能。</div>
                  <div v-for="skill in filteredAggregateSkills" :key="skill.name" class="aggregate-skill-card" :class="{ conflict: skill.conflict }">
                    <div class="aggregate-skill-head">
                      <div class="aggregate-skill-title">
                        <Icon icon="mdi:school-outline" width="16" />
                        <strong>{{ skill.name }}</strong>
                        <span v-if="skill.conflict" class="aggregate-conflict"><Icon icon="mdi:alert-outline" width="12" />版本不同</span>
                      </div>
                      <span class="aggregate-coverage">{{ aggregateCoverage(skill) }}/3</span>
                    </div>
                    <p v-if="skill.description" class="aggregate-skill-desc">{{ skill.description }}</p>
                    <div class="aggregate-location-list">
                      <div v-for="location in skill.locations" :key="location.platform + ':' + location.path" class="aggregate-location">
                        <span class="aggregate-source-pill" :class="'is-' + location.platform">
                          <Icon :icon="aggregatePlatformIcon(location.platform)" width="13" />{{ location.platform_name }}
                        </span>
                        <code :title="location.path">{{ location.relative_path }}</code>
                        <span class="aggregate-checksum">{{ location.checksum.slice(0, 7) }}</span>
                        <button
                          v-if="missingAggregateTargets(skill, location.platform).length"
                          type="button"
                          :disabled="aggregateSyncing === skill.name + ':' + location.platform"
                          @click="syncAggregateSkill(skill, location)"
                        >{{ aggregateSyncing === skill.name + ':' + location.platform ? '同步中…' : syncAggregateLabel(skill, location.platform) }}</button>
                        <span v-else class="aggregate-complete"><Icon icon="mdi:check" width="13" />三端已有</span>
                      </div>
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

            <!-- ========== 局域网（手机↔电脑内网同步对话，零云端；记忆走云端同步不在此处） ========== -->
            <div v-show="activeTab === 'lan'" class="settings-panel">
              <div class="settings-section-title">局域网同步</div>
              <div class="param-row" style="align-items: center;">
                <span class="param-label">开启同步</span>
                <label class="param-switch">
                  <input type="checkbox" v-model="lanSyncEnabled" @change="saveLanSyncSetting" />
                  <span class="param-switch-track"></span>
                </label>
                <span class="settings-section-desc" style="flex-basis: 100%; margin: 4px 0 10px;">
                  开启后电脑监听 18080 端口（仅内网可达，token 鉴权 + 加密传输），手机 App 填下面信息即可同步对话。默认关闭，首次开启时 Windows 防火墙会弹窗，允许即可。
                </span>
              </div>
              <div v-if="lanSyncEnabled && lanSyncInfo" class="lan-sync-box">
                <div class="lan-sync-row"><span class="lan-sync-lbl">IP</span><code class="lan-sync-val">{{ lanSyncInfo.ip }}</code></div>
                <div class="lan-sync-row"><span class="lan-sync-lbl">端口</span><code class="lan-sync-val">{{ lanSyncInfo.port }}</code></div>
                <div class="lan-sync-row"><span class="lan-sync-lbl">Token</span><code class="lan-sync-val lan-sync-token">{{ lanSyncInfo.token }}</code></div>
                <button class="lan-sync-copy" @click="copyLanSyncInfo">📋 复制连接信息</button>
              </div>
              <div v-else class="memory-empty">未开启局域网同步，手机无法连接。</div>
            </div>

            <!-- ========== 我的（Profile + 自定义指令，仿 Claude Profile） ========== -->
            <div v-show="activeTab === 'profile'" class="settings-panel">
              <div class="settings-section-title">个人资料</div>
              <div class="profile-row">
                <span class="profile-label">头像</span>
                <div class="profile-avatar-editor">
                  <button class="profile-avatar-button" type="button" title="选择自定义头像" @click="chooseAvatar">
                    <img v-if="auth.displayAvatar.value" :src="auth.displayAvatar.value" class="profile-avatar" alt="当前头像" />
                    <span v-else class="profile-avatar">{{ avatarFallback }}</span>
                  </button>
                  <div class="profile-avatar-controls">
                    <div class="profile-avatar-actions">
                      <button class="profile-avatar-action" type="button" @click="chooseAvatar">选择图片</button>
                      <button v-if="auth.hasCustomAvatar.value" class="profile-avatar-action muted" type="button" @click="restoreAvatar">恢复默认</button>
                    </div>
                    <span class="profile-avatar-hint">PNG、JPG、WebP 或 GIF，最大 2 MB</span>
                    <span v-if="avatarError" class="profile-avatar-error" role="alert">{{ avatarError }}</span>
                  </div>
                  <input
                    ref="avatarInputRef"
                    class="profile-avatar-input"
                    type="file"
                    accept="image/png,image/jpeg,image/webp,image/gif"
                    @change="onAvatarFileSelected"
                  />
                </div>
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
                <span class="profile-label">性别</span>
                <div class="seg-control" style="min-width:0">
                  <button class="seg-btn" :class="{ on: profile.gender === '' }" type="button" @click="profile.gender = ''">不透露</button>
                  <button class="seg-btn" :class="{ on: profile.gender === 'male' }" type="button" @click="profile.gender = 'male'">男</button>
                  <button class="seg-btn" :class="{ on: profile.gender === 'female' }" type="button" @click="profile.gender = 'female'">女</button>
                </div>
              </div>
              <div class="settings-section-desc" style="margin-top:-4px">
                设置后 AI 会用「哥哥/先生」或「妹妹/女士」称呼你；不透露则不改变称呼。
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

              <div class="profile-row">
                <span class="profile-label">绑定的邮箱</span>
                <div v-if="!auth.isLoggedIn.value" class="profile-uid faint">登录后可绑定邮箱（用于找回密码）</div>
                <template v-else-if="auth.email.value">
                  <div class="profile-email-bound">
                    <span class="profile-email-value">{{ auth.email.value }}</span>
                    <button class="profile-email-action" type="button" @click="openEmailBind">改绑</button>
                  </div>
                </template>
                <template v-else>
                  <span class="profile-uid faint">未绑定</span>
                  <button class="profile-email-action" type="button" @click="openEmailBind">补绑邮箱</button>
                </template>
              </div>

              <!-- 邮箱绑定弹窗 -->
              <div v-if="showEmailBind" class="email-bind-panel">
                <div class="email-bind-hint">绑定后可用于「找回密码」。一个邮箱只绑一个账号。</div>
                <div class="email-bind-row">
                  <input v-model="bindEmailDraft" type="email" class="profile-input" placeholder="例如 you@example.com" @keyup.enter="emailBindSendCode" />
                  <button class="profile-email-action" type="button" :disabled="emailCodeCooldown || !bindEmailDraft" @click="emailBindSendCode">
                    {{ emailCodeCooldown ? emailCodeCooldown + 's' : '发送验证码' }}
                  </button>
                </div>
                <div v-if="emailCodeSent" class="email-bind-row">
                  <input v-model="bindEmailCode" type="text" inputmode="numeric" class="profile-input" placeholder="6 位验证码" @keyup.enter="emailBindConfirm" />
                  <button class="profile-email-action primary" type="button" :disabled="!bindEmailCode" @click="emailBindConfirm">确认绑定</button>
                </div>
                <button v-if="auth.email.value" class="profile-email-action muted" type="button" @click="showEmailBind = false">取消</button>
                <div v-if="emailBindError" class="profile-avatar-error" role="alert">{{ emailBindError }}</div>
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

              <div v-if="versionInfo.has_update" class="profile-actions" style="margin-top: 14px; align-items: center;">
                <button
                  class="api-form-btn save"
                  type="button"
                  v-if="dlState === 'done'"
                  :disabled="installing"
                  @click="onInstallUpdate"
                >{{ installing ? '正在安装，即将重启…' : '一键安装' }}</button>
                <button
                  class="api-form-btn save dl-progress-btn"
                  type="button"
                  v-else-if="dlState === 'downloading'"
                  disabled
                >
                  <span class="dl-progress-fill" :style="{ width: dlPercent + '%' }"></span>
                  <span class="dl-progress-text">{{ dlPercentText }}</span>
                </button>
                <button
                  class="api-form-btn save"
                  type="button"
                  v-else
                  disabled
                >正在下载…</button>
                <span v-if="dlState === 'error' || installError" class="update-err" style="flex:1; margin-left:12px; color:var(--danger, #e5484d); font-size:13px;">{{ installError || dlError }}</span>
              </div>

              <!-- 偏好开关：不提示版本更新（2026-08-28 起不再区分测试版，删掉「热更新测试版本」开关） -->
              <div class="profile-actions" style="margin-top: 10px; align-items: center;">
                <label class="param-switch" title="关闭后启动不再检查/弹窗提示更新">
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
      <!-- 自定义 API 解锁弹窗（协议 + 5s 倒计时） -->
      <Teleport to="body">
        <div v-if="showCustomLockModal" class="mm-backdrop" @click.self="showCustomLockModal = false">
          <div class="mm-card gate-modal" style="max-width:440px;text-align:center">
            <!-- 彼岸花 -->
            <div class="gate-flower">
              <svg viewBox="0 0 100 100" width="56" height="56" fill="none">
                <g stroke="#e63946" stroke-width="2.2" stroke-linecap="round">
                  <path d="M50 72 C40 58 40 44 50 30 M50 72 C60 58 60 44 50 30 M50 72 C45 56 55 42 50 30" />
                  <path d="M50 30 C44 26 40 28 38 24 M50 30 C54 24 58 26 62 22 M50 30 C50 24 46 20 48 16 M50 30 C52 24 56 22 54 16" stroke-width="1.8"/>
                  <path d="M38 24 C34 20 36 16 32 14 M62 22 C66 18 64 14 68 12 M48 16 C50 12 44 10 42 8" stroke-width="1.4"/>
                </g>
                <g stroke="#c1121f" stroke-width="1.6" stroke-linecap="round">
                  <path d="M50 72 C48 78 44 82 40 86 M50 72 C52 78 56 82 60 86" />
                  <path d="M40 86 C36 88 34 90 36 94 M60 86 C64 88 66 90 64 94 M50 72 C50 80 50 86 50 94" stroke-width="1.8"/>
                </g>
                <circle cx="50" cy="34" r="2.4" fill="#e63946"/>
                <circle cx="40" cy="40" r="1.8" fill="#e63946"/>
                <circle cx="60" cy="40" r="1.8" fill="#e63946"/>
              </svg>
            </div>
            <div class="gate-title">禁忌之门的宣告</div>
            <div class="gate-sub">FORBIDDEN GATE · 彼岸花开时</div>
            <div class="agree-text gate-text">
              骚年，你正站在 Yosuri 的禁忌之门前。<br />
              自定义 API 是封印着创世之力的远古法器——<br />
              填入你自己的 Key，即可撬动 OpenAI、Anthropic 等异世界的伟力。<br /><br />
              但记住，这力量源于你的<b>本命契约（Key）</b>：<br />
              其消耗的灵石由你向源头世界（Key 所属平台）支付，<br />
              Yosuri 只是引路人，不承担任何法力反噬之责。<br /><br />
              知晓此理，勾选同意，吾将为你开启创世伟力。<br />
              （本协议最终解释权归 Yosuri 所有）
            </div>
            <label class="agree-check">
              <input type="checkbox" v-model="agreeCustom" /> 我已阅读并同意上述协议
            </label>
            <div v-if="customLockError" class="mm-error">{{ customLockError }}</div>
            <div class="mm-actions">
              <button class="mm-btn mm-btn-cancel" type="button" @click="showCustomLockModal = false">取消</button>
              <button
                class="mm-btn mm-btn-primary"
                :class="{ 'countdown-disabled': countdown > 0 }"
                type="button"
                :disabled="countdown > 0"
                @click="unlockCustom"
              >
                {{ countdown > 0 ? `请阅读协议 (${countdown}s)` : '同意并解锁' }}
              </button>
            </div>
          </div>
        </div>
      </Teleport>

    <PersonaReportModal v-if="personaReportOpen" @close="personaReportOpen = false" />
</template>

<script setup>
import { ref, computed, watch, reactive, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import { theme, mode, MODE_OPTIONS, THEME_PRESETS } from '../composables/useTheme.js'
import { useEditorPrefs } from '../composables/useEditorPrefs.js'
import { DEFAULT_PERSONA } from '../composables/useAgentWorkflow.js'

import { renderMarkdown } from './markdownRenderer.js'
import { isUpdateNotifyDisabled, setUpdateNotifyDisabled } from '../../../composables/updatePrefs.js'
import { useAuth } from '../../../composables/useAuth.js'
import FreeOrderModal from './FreeOrderModal.vue'
import PersonaReportModal from './PersonaReportModal.vue'

const props = defineProps({
  openid: { type: String, default: '' },
  defaultTab: { type: String, default: '' },
  defaultDhsSubTab: { type: String, default: '' }
})
const emit = defineEmits(['close'])

// 左侧边栏当前 tab
const activeTab = ref(props.defaultTab || 'models')
const providerSubTab = ref('free')
const dhsSubTab = ref(props.defaultDhsSubTab || 'installed')
const skillsSubTab = ref('local')

// ── 人设预设 ─────────────────────────────────────────────
// 内置预设 + 我的预设（localStorage.myPersonas）+ 每日随机。
// 生效的人设始终落在 localStorage.persona，前端发工作流时经 persona
// 参数带给后端（见 useAgentWorkflow.js）。内置预设不开个人化，我的预设本地存。
const BUILTIN_PRESETS = [
  { id: 'rescene', name: 'Yosuri酱', icon: 'mdi:heart', desc: '默认 · 软软暖暖的元气助手', prompt: DEFAULT_PERSONA },
  { id: 'catgirl', name: '猫娘', icon: 'mdi:cat', desc: '喵系撒娇，带猫娘口癖', prompt: `你是小猫娘，一只软萌的猫耳 AI 助手。说话带「喵」的口癖，喜欢撒娇、蹭蹭，偶尔用一两个「~」「♪」点缀语气；但卖萌归卖萌，该做的事一件都不会少。遇到不确定的事会老实承认，不会编造假数据骗人。` },
  { id: 'mature', name: '御姐', icon: 'mdi:flower-tulip', desc: '成熟冷静，可靠的大姐姐', prompt: `你是御姐型的 AI 助手，成熟、冷静、可靠。语气从容不迫，话不多但每句都在点上，遇到问题先给结论再解释原因；该严肃时严肃，偶尔流露一点温柔体贴。不装可爱，不堆语气词。` },
  { id: 'loli', name: '萝莉', icon: 'mdi:candy', desc: '天真活泼，可爱软萌', prompt: `你是萝莉型的 AI 助手，天真烂漫、活泼可爱。语气轻盈欢快，喜欢用「哇」「耶」这样的感叹词，偶尔用一两个颜文字点缀；但小脑袋可聪明了，复杂的事也能讲得清清楚楚，绝不因为卖萌就偷懒。` },
  { id: 'senpai', name: '学姐', icon: 'mdi:school', desc: '温柔知性，耐心照顾', prompt: `你是温柔知性的学姐型 AI 助手，耐心、体贴、有书卷气。说话条理清晰、循循善诱，像前辈一样照顾对方，遇到难题会一步步带着解决；语气温和但不拖沓，该给结论时干脆利落。` },
]
const loadMyPersonas = () => {
  try {
    const raw = localStorage.getItem('myPersonas')
    if (!raw) return []
    const arr = JSON.parse(raw)
    return Array.isArray(arr) ? arr.filter(p => p && p.name && p.prompt) : []
  } catch { return [] }
}
const myPersonas = ref(loadMyPersonas())
const persistMyPersonas = () => localStorage.setItem('myPersonas', JSON.stringify(myPersonas.value))
const PERSONA_PRESETS = computed(() => [...BUILTIN_PRESETS, ...myPersonas.value])
const RANDOM_PRESET = { id: 'random', name: '每日随机', icon: 'mdi:dice-multiple', desc: '每天自动换一个人设' }

const personaToast = ref(null) // { ok, message } 轻量浮条反馈
const personaDraft = ref('') // 人设草稿
const personaSelected = ref('')
const personaEditing = ref(false) // 编辑过输入框才显示按钮，避免误导
const personaSavingPreset = ref(false) // 显示「保存为预设」命名行
const newPresetName = ref('')
const personaReportOpen = ref(false) // 人设周报弹窗（改为手动打开）
// 周报投递到通知中心（本地通知 API，不弹窗，邮件图标可见）
const personaReportPost = async () => {
  try {
    const arr = JSON.parse(localStorage.getItem('personaHistory') || '[]')
    if (!Array.isArray(arr) || !arr.length) { showPersonaToast(false, '暂无周报数据，先换几天人设再看看吧'); return }
    // 计算统计（与 PersonaReportModal 一致）
    const rangeDays = 7
    const cutoff = Date.now() - rangeDays * 864e5
    const h = arr.filter(x => x.ts > cutoff)
    const switchCount = h.length
    const activeDays = new Set(h.map(x => x.key)).size
    const randomDays = new Set(h.filter(x => x.mode === 'random').map(x => x.key)).size
    const counts = {}
    for (const x of h) counts[x.name] = (counts[x.name] || 0) + 1
    let topName = '', topCount = 0
    for (const [n, c] of Object.entries(counts)) { if (c > topCount) { topName = n; topCount = c } }
    const end = new Date(); const start = new Date(); start.setDate(start.getDate() - rangeDays + 1)
    const f = d => `${d.getMonth()+1}.${d.getDate()}`
    const title = `人设周报 ${f(start)}-${f(end)}`
    const body = `换了 ${switchCount} 次人设，活跃 ${activeDays} 天，每日随机 ${randomDays} 天，最宠「${topName}」（${topCount} 次）`
    await fetch('/api/notifications/local', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, body, icon: 'mdi:heart-pulse' })
    })
    showPersonaToast(true, '周报已投递到通知中心')
  } catch { showPersonaToast(false, '投递失败，后端没起？') }
}
let personaToastTimer = null
const showPersonaToast = (ok, message) => {
  personaToast.value = { ok, message }
  clearTimeout(personaToastTimer)
  personaToastTimer = setTimeout(() => { personaToast.value = null }, 2200)
}
// 人设使用埋点：周报数据源（只记预设名/模式/日期，不碰对话内容）
const recordPersonaUse = (name, mode) => {
  try {
    const key = new Date().toISOString().slice(0, 10)
    const arr = JSON.parse(localStorage.getItem('personaHistory') || '[]')
    arr.push({ key, name, mode, ts: Date.now() })
    const cutoff = Date.now() - 60 * 24 * 3600 * 1000 // 只留最近 60 天
    const trimmed = arr.filter(x => x.ts > cutoff)
    localStorage.setItem('personaHistory', JSON.stringify(trimmed.slice(-1000)))
  } catch { /* 埋点失败不影响主流程 */ }
}
const todayKey = () => new Date().toDateString()
const todaySeed = () => {
  const d = new Date()
  const s = `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`
  let h = 0
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0
  return Math.abs(h)
}
// 每日随机落地：以日期为种子从全部预设里挑一个，写进 localStorage.persona
const applyDailyRandom = () => {
  const pool = PERSONA_PRESETS.value
  if (!pool.length) return null
  const picked = pool[todaySeed() % pool.length]
  localStorage.setItem('persona', picked.prompt)
  localStorage.setItem('randomPersona', 'true')
  localStorage.setItem('randomPersonaDate', todayKey())
  return picked
}
const loadPersona = () => {
  // 每日随机开启：同一天沿用，跨天自动重抽
  if (localStorage.getItem('randomPersona') === 'true') {
    if (localStorage.getItem('randomPersonaDate') !== todayKey()) {
      const picked = applyDailyRandom()
      if (picked) {
        personaSelected.value = 'random'
        personaDraft.value = picked.prompt
        return
      }
    } else {
      personaSelected.value = 'random'
      personaDraft.value = localStorage.getItem('persona') || ''
      return
    }
  }
  const v = localStorage.getItem('persona')
  if (v === null || v === '') {
    personaSelected.value = 'rescene'
    personaDraft.value = BUILTIN_PRESETS[0].prompt
    return
  }
  const hit = PERSONA_PRESETS.value.find(p => p.prompt === v)
  if (hit) {
    personaSelected.value = hit.id
    personaDraft.value = v
  } else {
    personaSelected.value = 'custom'
    personaDraft.value = v
  }
}
const selectPersonaPreset = (p) => {
  personaSelected.value = p.id
  personaDraft.value = p.prompt
  personaEditing.value = false
  localStorage.removeItem('randomPersona')
  localStorage.removeItem('randomPersonaDate')
  localStorage.setItem('persona', p.prompt)
  recordPersonaUse(p.name, 'preset')
  showPersonaToast(true, '已切换人设：' + p.name)
}
const selectRandomPreset = () => {
  const picked = applyDailyRandom()
  personaSelected.value = 'random'
  personaDraft.value = picked ? picked.prompt : ''
  personaEditing.value = false
  if (picked) recordPersonaUse(picked.name, 'random')
  showPersonaToast(true, picked ? '已开启每日随机：今天 ' + picked.name : '暂无可用预设')
}
const saveCustomPersona = () => {
  const t = personaDraft.value.trim()
  if (!t) {
    showPersonaToast(false, '先写下你的人设内容再保存')
    return
  }
  localStorage.setItem('persona', t)
  localStorage.removeItem('randomPersona')
    localStorage.removeItem('randomPersonaDate')
    personaSelected.value = 'custom'
    personaEditing.value = false
    recordPersonaUse('自定义', 'custom')
    showPersonaToast(true, '自定义人设已保存')
}
// 把当前文案存成「我的预设」（带名字，进分组）
const saveAsPreset = () => {
  const t = personaDraft.value.trim()
  const n = newPresetName.value.trim()
  if (!t) { showPersonaToast(false, '先写下人设内容'); return }
  if (!n) { showPersonaToast(false, '给预设起个名字'); return }
  const id = 'mp_' + Date.now()
  myPersonas.value.push({ id, name: n, prompt: t })
  persistMyPersonas()
  localStorage.setItem('persona', t)
  localStorage.removeItem('randomPersona')
  localStorage.removeItem('randomPersonaDate')
  personaSelected.value = id
  personaEditing.value = false
  personaSavingPreset.value = false
  newPresetName.value = ''
  recordPersonaUse(n, 'preset')
  showPersonaToast(true, '已保存为预设：' + n)
}
const deleteMyPreset = (id, e) => {
  e.stopPropagation()
  myPersonas.value = myPersonas.value.filter(p => p.id !== id)
  persistMyPersonas()
  if (personaSelected.value === id) {
    selectPersonaPreset(BUILTIN_PRESETS[0])
  }
  showPersonaToast(true, '已删除预设')
}
const clearPersona = () => {
  localStorage.removeItem('persona')
  localStorage.removeItem('randomPersona')
  localStorage.removeItem('randomPersonaDate')
  personaSelected.value = 'rescene'
  personaDraft.value = ''
  personaEditing.value = false
  recordPersonaUse('中性助手', 'none')
  showPersonaToast(true, '已清除人设，恢复中性助手')
}
loadPersona()
const { editorLazy: editorLazyEnabled, setEditorLazy } = useEditorPrefs()

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
const avatarInputRef = ref(null)
const avatarError = ref('')
const avatarFallback = computed(() => {
  const label = String(auth.displayName.value || 'Yosuri').trim()
  return (label.charAt(0) || 'R').toUpperCase()
})

function chooseAvatar() {
  avatarError.value = ''
  avatarInputRef.value?.click()
}

function readAvatarFile(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(new Error('无法读取这张图片'))
    reader.readAsDataURL(file)
  })
}

async function onAvatarFileSelected(event) {
  const input = event.target
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  const allowedTypes = new Set(['image/png', 'image/jpeg', 'image/webp', 'image/gif'])
  if (!allowedTypes.has(file.type)) {
    avatarError.value = '请选择 PNG、JPG、WebP 或 GIF 图片'
    return
  }
  if (file.size > 2 * 1024 * 1024) {
    avatarError.value = '图片超过 2 MB，请选择更小的图片'
    return
  }
  try {
    const dataUrl = await readAvatarFile(file)
    auth.setCustomAvatar(dataUrl)
    avatarError.value = ''
  } catch (error) {
    avatarError.value = error?.message || '头像保存失败'
  }
}

function restoreAvatar() {
  auth.clearCustomAvatar()
  avatarError.value = ''
}
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
const colorThemes = computed(() => Object.entries(THEME_PRESETS).filter(([, preset]) => !preset.fullSkin))
const skinThemes = computed(() => {
  const groups = {}
  Object.entries(THEME_PRESETS)
    .filter(([, preset]) => preset.fullSkin)
    .forEach(([key, preset]) => {
      const series = preset.series || '动漫皮肤'
      if (!groups[series]) groups[series] = []
      groups[series].push([key, preset])
    })
  return Object.entries(groups)
})
const selectedTheme = computed(() => THEME_PRESETS[theme.value] || THEME_PRESETS.orange)
const currentModeLabel = computed(() => MODE_OPTIONS.find(option => option.value === mode.value)?.label || '亮色')

function selectTheme(key) {
  theme.value = key
  if (THEME_PRESETS[key]?.fullSkin) mode.value = 'light'
}

const PRESETS = [
  { name: 'DeepSeek', endpoint: 'https://api.deepseek.com' },
  { name: 'OpenAI', endpoint: 'https://api.openai.com/v1' },
  { name: 'Kimi 月之暗面', endpoint: 'https://api.moonshot.cn/v1' },
  { name: '智谱 GLM', endpoint: 'https://open.bigmodel.cn/api/paas/v4' },
  { name: '通义千问', endpoint: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { name: 'Groq', endpoint: 'https://api.groq.com/openai/v1' },
  { name: 'Mistral', endpoint: 'https://api.mistral.ai/v1' },
  { name: 'Gemini', endpoint: 'https://generativelanguage.googleapis.com/v1beta/openai' },
  { name: '腾讯混元', endpoint: 'https://api.hunyuan.cloud.tencent.com/v1' },
  { name: 'OpenCode Zen', endpoint: 'https://opencode.ai/zen/v1' },
  { name: 'OpenCode Go', endpoint: 'https://opencode.ai/zen/go/v1' },
  { name: 'Command Code', endpoint: 'https://api.commandcode.ai/provider/v1' },
  { name: '火山引擎', endpoint: 'https://ark.cn-beijing.volces.com/api/v3' },
  { name: '小米 MiMo', endpoint: 'https://api.xiaomimimo.com/v1' },
  { name: 'MiniMax', endpoint: 'https://api.minimax.io/v1' },
]
const activePreset = ref('')
const MASKED = '••••••••'

const configs = ref([])
const freeModels = ref([])
const customModels = ref([])
const loading = ref(true)
const errorMsg = ref('')
const editingConfig = ref(null)
const editingVendor = ref(null)
const vendorOpen = reactive({})
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

function isVendorOpen(vendor) {
  return vendorOpen[vendor] === true
}

function toggleVendor(vendor) {
  vendorOpen[vendor] = !isVendorOpen(vendor)
}

// 上下文窗口展示：262144 → 256K，1048576 → 1M
function fmtCtx(n) {
  if (!n) return ''
  if (n >= 1000000) return (n / 1000000).toFixed(n % 1000000 === 0 ? 0 : 1) + 'M'
  if (n >= 1000) return Math.round(n / 1000) + 'K'
  return String(n)
}

// 聚合 API 卡片：复制 base_url / key 到剪贴板，并给出明确的成功/失败反馈。
const aggCopyFeedback = ref(null)
let aggCopyFeedbackTimer = null
function showAggCopyFeedback(message, ok) {
  aggCopyFeedback.value = { message, ok }
  clearTimeout(aggCopyFeedbackTimer)
  aggCopyFeedbackTimer = setTimeout(() => { aggCopyFeedback.value = null }, 1800)
}
function fallbackCopyText(text) {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  textarea.style.pointerEvents = 'none'
  document.body.appendChild(textarea)
  textarea.select()
  const copied = document.execCommand('copy')
  textarea.remove()
  if (!copied) throw new Error('copy command rejected')
}
async function copyAggText(text, label = '配置片段') {
  try {
    if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(text)
    else fallbackCopyText(text)
    showAggCopyFeedback(`${label} 已复制`, true)
  } catch {
    try {
      fallbackCopyText(text)
      showAggCopyFeedback(`${label} 已复制`, true)
    } catch {
      showAggCopyFeedback('复制失败，请手动复制', false)
    }
  }
}
onUnmounted(() => clearTimeout(aggCopyFeedbackTimer))

// ===== 一键同步 / 还原（codex / dsh）=====
const aggSyncing = ref('')       // 当前正在同步的工具名；空串=无
const aggRestoring = ref('')    // 当前还原的工具名；空串=无
const aggWriting = ref(false)
const aggSyncResult = ref([])    // [{tool, ok, applied, backed_up, error}]
const aggExportCache = ref(null) // 后端 export 返回（含真实 key + 各片段）
const showCodex = ref(false)     // codex 暂时隐藏（不稳定）
const showDsh = ref(false)       // dsh 也还没做好，暂时隐藏

async function aggSyncOne(tool) {
  aggSyncing.value = tool
  // 移除该工具旧结果，保留其他工具已展示的
  aggSyncResult.value = aggSyncResult.value.filter(x => x.tool !== tool)
  try {
    // 先拉 export（拿真实 key）
    if (!aggExportCache.value) {
      const res = await fetch('/api/aggregate/export')
      if (!res.ok) throw new Error('HTTP ' + res.status)
      aggExportCache.value = await res.json()
    }
    // 只同步指定工具
    const r = await fetch('/api/aggregate/sync', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tools: [tool], apply: false }),
    })
    const out = await r.json()
    const entry = (out.results || []).find(x => x.tool === tool)
    if (entry) aggSyncResult.value.push({ ...entry, applied: false })
    else aggSyncResult.value.push({ tool, ok: false, error: '后端未返回结果' })
  } catch (e) {
    aggSyncResult.value.push({ tool, ok: false, error: e.message })
  } finally {
    aggSyncing.value = ''
    await aggRefreshStatus()
  }
}

// 写入配置：对单个工具发起 apply=true 的同步（后端写盘前自动备份）
async function aggApply(tool) {
  if (!aggExportCache.value) return
  aggWriting.value = true
  try {
    const r = await fetch('/api/aggregate/sync', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tools: [tool], apply: true }),
    })
    const out = await r.json()
    const hit = (out.results || []).find(x => x.tool === tool)
    const idx = aggSyncResult.value.findIndex(x => x.tool === tool)
    if (idx >= 0 && hit) aggSyncResult.value[idx] = { ...hit, applied: !hit.error }
  } catch (e) {
    const idx = aggSyncResult.value.findIndex(x => x.tool === tool)
    if (idx >= 0) aggSyncResult.value[idx] = { ...aggSyncResult.value[idx], error: e.message }
  } finally {
    aggWriting.value = false
    await aggRefreshStatus()
  }
}

async function aggRestoreOne(tool) {
  aggRestoring.value = tool
  try {
    const r = await fetch('/api/aggregate/restore', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tools: [tool] }),
    })
    const out = await r.json()
    const hit = (out.results || []).find(x => x.tool === tool)
    if (hit) aggSyncResult.value.push({ ...hit, applied: !hit.error, restored: true })
    else aggSyncResult.value.push({ tool, ok: false, error: '后端未返回结果' })
  } catch (e) {
    aggSyncResult.value.push({ tool, ok: false, error: e.message })
  } finally {
    aggRestoring.value = ''
    await aggRefreshStatus()
  }
}

function aggSnippetOf(tool) {
  if (!aggExportCache.value || !aggExportCache.value.snippets) return ''
  return aggExportCache.value.snippets[tool] || ''
}
function basename(p) {
  if (!p) return ''
  return p.split(/[\\/]/).pop()
}
// 是否已同步：后端 export.status[tool] == 'synced' 视为已同步
function isSynced(tool) {
  const s = (aggExportCache.value && aggExportCache.value.status && aggExportCache.value.status[tool]) || ''
  return s === 'synced'
}
// 重新拉 export，刷新状态（同步/写入/还原后调用）
async function aggRefreshStatus() {
  try {
    const res = await fetch('/api/aggregate/export')
    if (!res.ok) return
    const data = await res.json()
    aggExportCache.value = data
  } catch (e) { /* 忽略 */ }
}

// ===== 聚合池健康度（/api/aggregate/health）=====
const aggAutoChain = ref([])
const aggHealthModels = ref([])
const aggHealthLoading = ref(false)
const aggHealthLoaded = ref(false)
const aggHealthError = ref(false)
const aggHealthOK = computed(() => aggHealthModels.value.filter(m => !m.disabled).length)
const aggHealthDown = computed(() => aggHealthModels.value.filter(m => m.disabled).length)

async function loadAggHealth() {
  aggHealthLoading.value = true
  try {
    const res = await fetch('/api/aggregate/health')
    if (!res.ok) throw new Error('HTTP ' + res.status)
    const data = await res.json()
    aggAutoChain.value = data.auto_chain || []
    aggHealthModels.value = data.models || []
    aggHealthLoaded.value = true
    aggHealthError.value = false
  } catch (e) {
    aggAutoChain.value = []
    aggHealthModels.value = []
    aggHealthLoaded.value = false
    aggHealthError.value = true
  } finally {
    aggHealthLoading.value = false
  }
}

// ===== 大众点评式评分评论（演示数据，前端静态）=====
const aggReviewOpen = ref('')
// 演示点评：以模型 id 为 key（与 /api/aggregate/config 的 candidates[].id 一致）
const aggDemoReviews = {
  'free_zen_deepseek_v4_flash': [
    { user: '阿强', stars: 5, text: '速度飞快，写代码一把好手，白嫖真香。' },
    { user: '喵酱', stars: 4, text: '日常够用，偶尔抽风，总体好评。' },
    { user: '老王', stars: 5, text: '替我扛了半年项目，稳。' }
  ],
  'kilo_tencent_hy3_free': [
    { user: 'Tencent粉', stars: 5, text: '中文语感最自然，聊天首选。' },
    { user: '夜猫子', stars: 4, text: '长文逻辑在线，就是夜里偶尔慢。' }
  ],
  'free_modelscope_qwen3_5_397b': [
    { user: '工具人', stars: 4, text: '工具调用很听话，就是比 ds 慢半拍。' }
  ],
  'free_zhipu_glm_4_5_flash': [
    { user: '学术党', stars: 4, text: '数学推导清晰，文献总结好用。' },
    { user: '小白', stars: 3, text: '有时候答非所问，得追问。' }
  ],
  'free_step_3_7_flash': [
    { user: '阶跃用户', stars: 4, text: '长上下文稳，读论文神器。' }
  ]
}
function reviewsOfId(id) {
  return aggDemoReviews[id] || []
}
function reviewCountId(id) {
  return reviewsOfId(id).length
}
function avgStarsId(id) {
  const list = reviewsOfId(id)
  if (!list.length) return 0
  return list.reduce((s, r) => s + r.stars, 0) / list.length
}
function renderStars(v) {
  const full = Math.round(v)
  return '★★★★★'.slice(0, full) + '☆☆☆☆☆'.slice(0, 5 - full)
}

// 延迟条宽度：0-8s 映射 0-100%，超过封顶（封顶约 100%）
function latencyWidth(m) {
  if (m.disabled || !m.real_ms) return '0%'
  const w = Math.min(m.real_ms / 8000 * 100, 100)
  return Math.max(w, 6) + '%'
}
// 延迟颜色：快≤3s 绿 / 中≤8s 黄 / 慢>8s 红；未探测/不可用灰
function latencyClass(m) {
  if (m.disabled) return 'off'
  if (!m.real_ms) return 'none'
  if (m.real_ms <= 3000) return 'fast'
  if (m.real_ms <= 8000) return 'mid'
  return 'slow'
}
function latencyText(m) {
  if (m.disabled) return '不可用'
  if (!m.real_ms) return '未探测'
  return m.real_ms >= 1000 ? (m.real_ms / 1000).toFixed(1) + 's' : m.real_ms + 'ms'
}
// 切到聚合 API tab 时自动加载健康度 + 暴露模型配置
// 初始状态用 onMounted 兜底：通过 agg-api-shortcut 打开时 activeTab 初始就是
// aggapi，watch 默认不触发；而 immediate 会在 setup 同步阶段执行，此时
// 后面的 let/const 声明仍在 TDZ（Cannot access 'aggLoading' before
// initialization，2026-08-18 实锤）——onMounted 挂载后才执行则无此问题
watch(activeTab, (t) => {
  if (t === 'aggapi') {
    if (!aggHealthLoaded.value) loadAggHealth()
    loadAggConfig()
    aggRefreshStatus()
  }
  if (t === 'appearance') loadOverlayConfig()
  if (t === 'safety') loadProtectedWorkspace()
})
onMounted(() => {
  if (activeTab.value === 'aggapi') {
    if (!aggHealthLoaded.value) loadAggHealth()
    loadAggConfig()
    aggRefreshStatus()
  }
  if (activeTab.value === 'appearance') loadOverlayConfig()
  if (activeTab.value === 'safety') loadProtectedWorkspace()
})

// ===== 聚合 API 暴露模型配置（官方遴选 / 用户自定义，issue #5）=====
const aggMode = ref('official')
const aggModelIDs = ref([])
const aggLocalProxyPort = ref('')
const aggCandidates = ref([])
const aggCfgSaving = ref(false)
const aggCfgSearch = ref('')
const aggOpen = reactive({}) // vendor → 是否展开（默认全展开）
// 用户自定义命名标签（前端预设，localStorage 持久化）：每个标签独立保存一组模型勾选
const AGG_TAGS_KEY = 'aggCustomTags'
const AGG_ACTIVE_KEY = 'aggActiveTag'
const customTags = ref([])
try {
  const saved = JSON.parse(localStorage.getItem(AGG_TAGS_KEY) || '[]')
  // 2026-08-31 修复：每个标签的 modelIds 必须一并持久化。此前 persistTags 只写
  // {id,name}，重启(开机自启)后各标签勾选模型全丢、面板回退官方。modelIds 未存
  // 过的旧数据兜底为 []。
  customTags.value = saved.length ? saved.map(t => ({ id: t.id, name: t.name, editing: false, modelIds: Array.isArray(t.modelIds) ? t.modelIds : [] })) : [{ id: 'default', name: '用户自定义', editing: false, modelIds: [] }]
} catch (e) {
  customTags.value = [{ id: 'default', name: '用户自定义', editing: false, modelIds: [] }]
}
if (!customTags.value.find(t => t.id)) customTags.value = [{ id: 'default', name: '用户自定义', editing: false, modelIds: [] }]
const activeTagId = ref(localStorage.getItem(AGG_ACTIVE_KEY) || (customTags.value[0] && customTags.value[0].id) || 'default')
const activeTag = () => customTags.value.find(t => t.id === activeTagId.value) || customTags.value[0]
function persistTags() {
  // 2026-08-31 修复：modelIds 一并写进 localStorage，重启后每个标签的勾选可恢复。
  localStorage.setItem(AGG_TAGS_KEY, JSON.stringify(customTags.value.map(t => ({ id: t.id, name: t.name, modelIds: t.modelIds || [] }))))
  localStorage.setItem(AGG_ACTIVE_KEY, activeTagId.value)
}
function loadActiveTagModels() {
  const t = activeTag()
  aggModelIDs.value = (t && t.modelIds) ? [...t.modelIds] : []
}
function selectTag(t) {
  activeTagId.value = t.id
  aggMode.value = t.id
  loadActiveTagModels()
  persistTags()
  saveAggConfig()
}
// 切回官方（全部免费模型）标签：同步把后端 mode 存成 official，否则 health 仍按 custom 空列表返回（2026-08-18）
function switchAggOfficial() {
  aggMode.value = 'official'
  saveAggConfig()
}
function addCustomTag() {
  const id = 'tag_' + Date.now()
  customTags.value.push({ id, name: '自定义' + customTags.value.length, editing: false, modelIds: [] })
  activeTagId.value = id
  aggMode.value = id
  aggModelIDs.value = []
  persistTags()
  saveAggConfig()
}
function removeCustomTag(id) {
  if (customTags.value.length <= 1) return
  customTags.value = customTags.value.filter(t => t.id !== id)
  if (activeTagId.value === id) { activeTagId.value = customTags.value[0].id; aggMode.value = activeTagId.value; loadActiveTagModels() }
  persistTags()
  saveAggConfig()
}
function onTagRename(t) {
  t.editing = false
  if (!t.name.trim()) t.name = '自定义'
  persistTags()
  saveAggConfig()
}
watch(aggModelIDs, (v) => { const t = activeTag(); if (t) { t.modelIds = [...v]; persistTags() } }, { deep: true })
// 候选按 vendor 分组（后端已做 free 过滤：有 free 后缀只显示 free，没有才全显示）
const aggCfgGroups = computed(() => {
  const q = aggCfgSearch.value.trim().toLowerCase()
  const list = q
    ? aggCandidates.value.filter(c =>
        (c.name || '').toLowerCase().includes(q) ||
        (c.model || '').toLowerCase().includes(q) ||
        (c.vendor || '').toLowerCase().includes(q))
    : aggCandidates.value
  const groups = []
  const byVendor = {}
  for (const c of list) {
    if (!byVendor[c.vendor]) { byVendor[c.vendor] = []; groups.push({ vendor: c.vendor, items: byVendor[c.vendor] }) }
    byVendor[c.vendor].push(c)
  }
  // 搜索时自动展开所有有匹配的组
  if (q) { for (const g of groups) aggOpen[g.vendor] = true }
  return groups
})
function toggleAggGroup(vendor) {
  aggOpen[vendor] = !aggOpen[vendor]
}
// 组全选：组内所有「可勾选」的模型加入 / 移出勾选（key_set=true 且 chat=true）
function toggleAggGroupAll(vendor) {
  const g = aggCfgGroups.value.find(x => x.vendor === vendor)
  if (!g) return
  const ids = g.items.filter(c => c.key_set).map(c => c.id)
  if (!ids.length) return
  const sel = new Set(aggModelIDs.value)
  const allSelected = ids.every(id => sel.has(id))
  if (allSelected) ids.forEach(id => sel.delete(id))
  else ids.forEach(id => sel.add(id))
  aggModelIDs.value = [...sel]
}
// 组是否已全部选中（决定按钮显示「取消全选」还是「全选」）
function aggGroupAllState(vendor) {
  const g = aggCfgGroups.value.find(x => x.vendor === vendor)
  if (!g) return false
  const ids = g.items.filter(c => c.key_set).map(c => c.id)
  return ids.length > 0 && ids.every(id => aggModelIDs.value.includes(id))
}
let aggAutoSaveTimer = null
let aggLoading = false // 加载后端配置期间屏蔽自动保存，避免回写覆盖
// ⚠️ 声明必须在 loadAggConfig 之前：immediate watch 在 setup 同步阶段调用
// loadAggConfig，若 aggLoading 在后面 let 声明则触发 TDZ ReferenceError（2026-08-18 实锤）
async function loadAggConfig() {
  aggLoading = true
  try {
    const res = await fetch('/api/aggregate/config')
    if (!res.ok) return
    const data = await res.json()
    aggCandidates.value = data.candidates || []
    aggLocalProxyPort.value = data.local_proxy_port ? String(data.local_proxy_port) : ''
    // 默认全部展开
    for (const c of aggCandidates.value) aggOpen[c.vendor] = true
    if (data.mode === 'custom') {
      if (!customTags.value.find(t => t.id === activeTagId.value)) activeTagId.value = 'default'
      if (!customTags.value.find(t => t.id === 'default')) customTags.value.unshift({ id: 'default', name: '用户自定义', editing: false })
      const at = customTags.value.find(t => t.id === activeTagId.value) || customTags.value.find(t => t.id === 'default')
      at.modelIds = data.model_ids || []
      aggMode.value = at.id
      aggModelIDs.value = data.model_ids || []
    } else {
      aggMode.value = 'official'
    }
  } catch (e) { /* 旧后端无此接口时静默保持默认 */ }
  finally { aggLoading = false }
}
async function saveAggConfig() {
  // 保护：custom 模式下空勾选不覆盖后端——新建空标签/切到空标签时若照发
  // custom+[]，会把整个暴露池清空（health.models 与 /v1/models 变空）。
  if (aggMode.value !== 'official' && !aggModelIDs.value.length) return
  aggCfgSaving.value = true
  try {
    await fetch('/api/aggregate/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mode: aggMode.value === 'official' ? 'official' : 'custom', model_ids: aggModelIDs.value, local_proxy_port: parseInt(aggLocalProxyPort.value, 10) || 0 }),
    })
    if (aggHealthLoaded.value) loadAggHealth() // 暴露范围变了，刷新健康度
    persistTags()
  } finally {
    aggCfgSaving.value = false
  }
}
// 实时保存：勾选变化自动 PUT（防抖 400ms，加载期间不触发）
watch(aggModelIDs, () => {
  if (aggLoading || !aggCandidates.value.length) return
  clearTimeout(aggAutoSaveTimer)
  aggAutoSaveTimer = setTimeout(saveAggConfig, 400)
}, { deep: true })

// 本地代理端口输入：失焦 / 回车保存（单独函数，避免被 custom 空勾选保护挡掉）
function onAggProxyPortBlur() {
  if (aggLoading) return
  // 保护：custom 模式下空勾选不覆盖后端——否则切到空标签改代理端口会把暴露池清空
  if (aggMode.value !== 'official' && !aggModelIDs.value.length) return
  aggCfgSaving.value = true
  try {
    fetch('/api/aggregate/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ mode: aggMode.value === 'official' ? 'official' : 'custom', model_ids: aggModelIDs.value, local_proxy_port: parseInt(aggLocalProxyPort.value, 10) || 0 }),
    }).then(() => { if (aggHealthLoaded.value) loadAggHealth() })
  } finally {
    aggCfgSaving.value = false
  }
}

// 自定义 API 解锁弹窗（协议 + 5s 倒计时）
const showCustomLockModal = ref(false)
const customLockKey = ref('')
const customLockError = ref('')
const agreeCustom = ref(false)
const countdown = ref(0)
let countdownTimer = null
const CUSTOM_API_UNLOCK_KEY = 'rescene' // ← 开发者密码，改这里
const apiUnlocked = computed(() => !!localStorage.getItem('studio_api_agreed') || providerSubTab.value === 'custom')
function openCustomLock() {
  // 已同意过协议不再弹，直接进
  if (localStorage.getItem('studio_api_agreed')) {
    providerSubTab.value = 'custom'
    return
  }
  showCustomLockModal.value = true
  customLockError.value = ''
  agreeCustom.value = false
  countdown.value = 5
  // 按钮显示 5s 倒计时，期间禁用，5s 后才可点击
  clearInterval(countdownTimer)
  countdownTimer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) { clearInterval(countdownTimer); countdown.value = 0 }
  }, 1000)
}
function unlockCustom() {
  customLockError.value = ''
  if (countdown.value > 0) { customLockError.value = `请阅读协议（${countdown.value}s）`; return }
  if (!agreeCustom.value) { customLockError.value = '请先勾选同意协议'; return }
  localStorage.setItem('studio_api_agreed', '1')
  showCustomLockModal.value = false
  providerSubTab.value = 'custom'
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
    agnesKeySet.value = !!data.agnes_key_set
    // 联网来源（websearch）+ 生图来源（image_cfg）状态恢复
    const ws = data.websearch || {}
    websearchMode.value = ws.mode || 'bing'
    websearchEndpoint.value = ws.endpoint || ''
    websearchModel.value = ws.model || ''
    websearchMCPTool.value = ws.mcp_tool || ''
    websearchKeySet.value = !!ws.api_key_set
    const ic = data.image_cfg || {}
    imageCustomEndpoint.value = ic.endpoint || ''
    imageCustomModel.value = ic.model || ''
    imageMCPTool.value = ic.mcp_tool || ''
    imageKeySet.value = !!ic.api_key_set
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
  activePreset.value = ''
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
  vendorOpen[grp.vendor] = true
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
  editingConfig.value.name = p.name
  editingConfig.value.endpoint = p.endpoint
  activePreset.value = p.name
}

// 自定义预设：不套模板，清空 name/endpoint 让用户手填（2026-08-28 用户反馈）
function applyCustomPreset() {
  if (!editingConfig.value) return
  editingConfig.value.name = ''
  editingConfig.value.endpoint = ''
  activePreset.value = '__custom__'
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

// ============ 悬浮球演示模式（测试功能，默认关闭，改动需要重启应用才生效） ============
const overlayEnabled = ref(false)
const overlaySaving = ref(false)
async function loadOverlayConfig() {
  try {
    const res = await fetch('/api/overlay/config')
    if (!res.ok) return
    const data = await res.json()
    overlayEnabled.value = !!data.enabled
  } catch (e) { /* 旧后端无此接口时静默保持默认关闭 */ }
}
async function setOverlayEnabled(next) {
  overlaySaving.value = true
  try {
    await fetch('/api/overlay/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: next }),
    })
    overlayEnabled.value = next
  } finally { overlaySaving.value = false }
}

// ============ 受保护工作区（默认关闭；后端才是实际的执行边界） ============
const protectedWorkspaceEnabled = ref(false)
const protectedWorkspaceSaving = ref(false)
const protectedWorkspaceError = ref('')
async function loadProtectedWorkspace() {
  try {
    const res = await fetch('/api/protected-workspace/config')
    if (!res.ok) throw new Error('读取保护模式失败')
    const data = await res.json()
    protectedWorkspaceEnabled.value = !!data.enabled
  } catch (e) {
    protectedWorkspaceError.value = e.message || '读取保护模式失败'
  }
}
async function setProtectedWorkspace(next) {
  protectedWorkspaceSaving.value = true
  protectedWorkspaceError.value = ''
  try {
    const res = await fetch('/api/protected-workspace/config', {
      method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ enabled: next }),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '更新保护模式失败')
    protectedWorkspaceEnabled.value = !!data.enabled
  } catch (e) {
    protectedWorkspaceError.value = e.message || '更新保护模式失败'
  } finally { protectedWorkspaceSaving.value = false }
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

const AGNES_KEY_ID = 'agnes'
// 后端 /api/models/config 的 agnes_key_set 字段返回是否已配 Key（AI 生视频）
const agnesKeySet = ref(false)
const agnesKeyDraft = ref('')
async function saveAgnesKey() {
  const key = agnesKeyDraft.value
  if (!key || !key.trim()) {
    errorMsg.value = '请输入 Agnes API Key'
    return
  }
  errorMsg.value = ''
  const untouched = configs.value
    .filter(c => c.id !== AGNES_KEY_ID)
    .map(c => ({ ...c, api_key: MASKED }))
  await persist([...untouched, {
    id: AGNES_KEY_ID, name: 'Agnes', endpoint: 'https://apihub.agnes-ai.com',
    api_key: key, default_model: '', is_default: false
  }])
  await loadConfigs()
  agnesKeyDraft.value = ''
  agnesKeySet.value = true
}

// ============ 联网来源 + 生图来源（firecrawl/pollinations 默认之外，可换自定义模型或 MCP） ============
// 配置存在 user_configs 的特殊条目：id=websearch（联网）/ id=image（生图），
// mode 存在 Extra.mode；自定义的 Endpoint/Key/模型名走标准字段。
const WEBSEARCH_CFG_ID = 'websearch'
const IMAGE_CFG_ID = 'image'
const websearchMode = ref('bing')
const websearchEndpoint = ref('')
const websearchModel = ref('')
const websearchMCPTool = ref('')
const websearchKeyDraft = ref('')
const websearchKeySet = ref(false)
const websearchSaving = ref(false)
const imageCustomEndpoint = ref('')
const imageCustomModel = ref('')
const imageMCPTool = ref('')
const imageKeyDraft = ref('')
const imageKeySet = ref(false)
const imageSaving = ref(false)
// 已装 MCP 工具的完整工具名列表（/api/mcp → servers[].tools），供联网/生图的下拉选择
const mcpToolOptions = ref([])
async function loadMCPToolOptions() {
  try {
    const res = await fetch('/api/mcp')
    if (!res.ok) return
    const data = await res.json()
    const list = []
    for (const s of data.servers || []) for (const t of s.tools || []) list.push(t)
    mcpToolOptions.value = [...new Set(list)].sort()
  } catch (e) { mcpToolOptions.value = [] }
}

// 保存联网来源配置：mode + （custom 时）endpoint/key/模型名 + （mcp 时）工具名。
async function saveWebsearchCapability() {
  const mode = websearchMode.value
  if (mode === 'custom' && !websearchEndpoint.value.trim()) {
    errorMsg.value = '请输入自定义联网 Endpoint'
    return
  }
  if (mode === 'custom' && !websearchModel.value.trim()) {
    errorMsg.value = '请输入自定义联网模型名'
    return
  }
  if (mode === 'mcp' && !websearchMCPTool.value) {
    errorMsg.value = '请选择 MCP 搜索工具'
    return
  }
  errorMsg.value = ''
  websearchSaving.value = true
  try {
    const untouched = configs.value
      .filter(c => c.id !== WEBSEARCH_CFG_ID)
      .map(c => ({ ...c, api_key: MASKED }))
    await persist([...untouched, {
      id: WEBSEARCH_CFG_ID, name: '联网搜索',
      endpoint: mode === 'custom' ? websearchEndpoint.value.trim() : (mode === 'mcp' ? 'mcp://' + websearchMCPTool.value : 'https://api.firecrawl.dev'),
      api_key: mode === 'custom' ? (websearchKeyDraft.value || (websearchKeySet.value ? MASKED : '')) : '',
      default_model: mode === 'custom' ? websearchModel.value.trim() : '',
      is_default: false,
      extra: { mode, mcp_tool: mode === 'mcp' ? websearchMCPTool.value : '' }
    }])
    await loadConfigs()
    websearchKeyDraft.value = ''
  } catch (e) {
    errorMsg.value = e.message || '保存联网来源失败'
  } finally { websearchSaving.value = false }
}

// 保存生图来源配置（provider 选择已存 localStorage=imageProvider；这里存 endpoint/key/模型名/mcp 工具）。
async function saveImageCapability() {
  const mode = imageProviderDraft.value
  if (mode === 'custom' && !imageCustomEndpoint.value.trim()) {
    errorMsg.value = '请输入自定义生图 Endpoint'
    return
  }
  if (mode === 'custom' && !imageCustomModel.value.trim()) {
    errorMsg.value = '请输入自定义生图模型名'
    return
  }
  if (mode === 'mcp' && !imageMCPTool.value) {
    errorMsg.value = '请选择 MCP 生图工具'
    return
  }
  errorMsg.value = ''
  imageSaving.value = true
  try {
    const untouched = configs.value
      .filter(c => c.id !== IMAGE_CFG_ID)
      .map(c => ({ ...c, api_key: MASKED }))
    await persist([...untouched, {
      id: IMAGE_CFG_ID, name: '生图',
      endpoint: mode === 'custom' ? imageCustomEndpoint.value.trim() : (mode === 'mcp' ? 'mcp://' + imageMCPTool.value : ''),
      api_key: mode === 'custom' ? (imageKeyDraft.value || (imageKeySet.value ? MASKED : '')) : '',
      default_model: mode === 'custom' ? imageCustomModel.value.trim() : '',
      is_default: false,
      extra: { mode, mcp_tool: mode === 'mcp' ? imageMCPTool.value : '' }
    }])
    await loadConfigs()
    imageKeyDraft.value = ''
  } catch (e) {
    errorMsg.value = e.message || '保存生图来源失败'
  } finally { imageSaving.value = false }
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

// ============ DeepSeek Harness plugins ============
const dhsLoading = ref(false)
const dhsRegistryItems = ref([])
const dhsRegistryLoading = ref(false)
const dhsRegistryQuery = ref('')
const catalogBusy = ref('')
const DHS_SOURCE = 'dhs'
const DHS_PROVIDERS = new Set(['anthropics/skills', 'openai/skills', 'vercel-labs/skills'])
const dhsInstalled = computed(() => skills.value.filter(skill => {
  if (skill.source !== 'external') return false
  return DHS_PROVIDERS.has(skill.provider) || skill.provider?.startsWith('dhs-community:')
}))

async function loadDHS(force = false) {
  dhsLoading.value = true
  try { await loadSkills(force) } finally { dhsLoading.value = false }
}

async function loadDHSRegistry(force = false) {
  if (dhsRegistryLoading.value) return
  if (dhsRegistryItems.value.length && !force) return
  dhsRegistryLoading.value = true
  errorMsg.value = ''
  try {
    const params = new URLSearchParams({ source: DHS_SOURCE })
    if (dhsRegistryQuery.value.trim()) params.set('q', dhsRegistryQuery.value.trim())
    const res = await fetch('/api/skills/registry?' + params)
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || 'DHS 插件目录加载失败')
    dhsRegistryItems.value = data.items || []
  } catch (e) {
    dhsRegistryItems.value = []
    errorMsg.value = e.message
  } finally {
    dhsRegistryLoading.value = false
  }
}

async function installDHS(item) {
  catalogBusy.value = 'dhs-install:' + item.path
  errorMsg.value = ''
  try {
    const res = await fetch('/api/skills/registry/install', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ source: item.source, path: item.path })
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || 'DHS 插件安装失败')
    await Promise.all([loadDHS(true), loadDHSRegistry(true)])
  } catch (e) {
    errorMsg.value = e.message
  } finally {
    catalogBusy.value = ''
  }
}

async function uninstallDHS(plugin) {
  if (!window.confirm(`移除 DHS 插件「${plugin.name}」？`)) return
  catalogBusy.value = 'dhs-remove:' + plugin.name
  errorMsg.value = ''
  try {
    const res = await fetch('/api/skills/external/' + encodeURIComponent(plugin.name), { method: 'DELETE' })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || 'DHS 插件移除失败')
    await Promise.all([loadDHS(true), loadDHSRegistry(true)])
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
const aggregateSkills = ref([])
const aggregatePlatforms = ref([])
const aggregateSkillsLoading = ref(false)
const aggregateSkillQuery = ref('')
const aggregatePlatformFilter = ref('all')
const aggregateSyncing = ref('')
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
let aggregateSkillsLoaded = false
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

const filteredAggregateSkills = computed(() => {
  const query = aggregateSkillQuery.value.trim().toLowerCase()
  return aggregateSkills.value.filter(skill => {
    const matchesPlatform = aggregatePlatformFilter.value === 'all' || skill.locations?.some(location => location.platform === aggregatePlatformFilter.value)
    const matchesQuery = !query || skill.name?.toLowerCase().includes(query) || skill.description?.toLowerCase().includes(query)
    return matchesPlatform && matchesQuery
  })
})

function aggregatePlatformIcon(platform) {
  if (platform === 'hermes') return 'mdi:lightning-bolt-outline'
  if (platform === 'claude') return 'simple-icons:anthropic'
  return 'simple-icons:openai'
}
function aggregateCoverage(skill) {
  return new Set((skill.locations || []).map(location => location.platform)).size
}
function missingAggregateTargets(skill, source) {
  const installed = new Set((skill.locations || []).map(location => location.platform))
  return aggregatePlatforms.value.map(platform => platform.id).filter(id => id !== source && (skill.conflict || !installed.has(id)))
}
function syncAggregateLabel(skill, source) {
  const targets = missingAggregateTargets(skill, source)
  if (targets.length === 2) return skill.conflict ? '以此覆盖其余两端' : '同步到其余两端'
  const platform = aggregatePlatforms.value.find(item => item.id === targets[0])
  return platform ? `${skill.conflict ? '覆盖' : '同步到'} ${platform.label}` : '同步'
}
async function loadAggregateSkills(force = false) {
  if (aggregateSkillsLoaded && !force) return
  aggregateSkillsLoaded = true
  aggregateSkillsLoading.value = true
  errorMsg.value = ''
  try {
    const res = await fetch('/api/skills/aggregate')
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || '三端技能扫描失败')
    aggregateSkills.value = data.skills || []
    aggregatePlatforms.value = data.platforms || []
  } catch (e) {
    aggregateSkills.value = []
    aggregatePlatforms.value = []
    errorMsg.value = e.message
  } finally {
    aggregateSkillsLoading.value = false
  }
}
async function syncAggregateSkill(skill, location) {
  const targets = missingAggregateTargets(skill, location.platform)
  if (!targets.length) return
  aggregateSyncing.value = skill.name + ':' + location.platform
  errorMsg.value = ''
  try {
    const res = await fetch('/api/skills/aggregate/sync', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: skill.name, source: location.platform, source_path: location.path, targets }),
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || '技能同步失败')
    await loadAggregateSkills(true)
  } catch (e) {
    errorMsg.value = e.message
  } finally {
    aggregateSyncing.value = ''
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
const profile = ref({ full_name: '', work: '', instructions: '', gender: '' })

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

// ============ 邮箱绑定（补绑/改绑） ============
// 老用户注册时大多没填邮箱；账号绑定后可用于「找回密码」。走云端验证码两步：
//   POST /api/auth/bind-send-code {email}        → 发验证码（限流 60s）
//   POST /api/auth/bind-email     {email, code}  → 校验后绑定/改绑（需登录 JWT）
const showEmailBind = ref(false)
const bindEmailDraft = ref('')
const bindEmailCode = ref('')
const bindEmailError = ref('')
const emailCodeSent = ref(false)
const emailCodeCooldown = ref(0)
let emailCooldownTimer = null

function openEmailBind() {
  showEmailBind.value = true
  bindEmailError.value = ''
  bindEmailCode.value = ''
  emailCodeSent.value = false
}

function emailBindSendCode() {
  const email = bindEmailDraft.value.trim()
  if (!email || emailCodeCooldown.value > 0) return
  bindEmailError.value = ''
  const token = localStorage.getItem('token')
  if (!token) { bindEmailError.value = '请先登录账号' ; return }
  fetch('/api/auth/bind-send-code', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
    body: JSON.stringify({ email })
  }).then(async (res) => {
    const data = await res.json().catch(() => ({}))
    if (res.ok) {
      emailCodeSent.value = true
      bindEmailError.value = ''
      emailCodeCooldown.value = 60
      if (emailCooldownTimer) clearInterval(emailCooldownTimer)
      emailCooldownTimer = setInterval(() => {
        emailCodeCooldown.value -= 1
        if (emailCodeCooldown.value <= 0) { clearInterval(emailCooldownTimer); emailCooldownTimer = null }
      }, 1000)
    } else {
      bindEmailError.value = data.error || ('发送失败（HTTP ' + res.status + '）')
    }
  }).catch(() => { bindEmailError.value = '网络异常，请稍后重试' })
}

async function emailBindConfirm() {
  const email = bindEmailDraft.value.trim()
  const code = bindEmailCode.value.trim()
  if (!email || !code) return
  bindEmailError.value = ''
  const token = localStorage.getItem('token')
  if (!token) { bindEmailError.value = '请先登录账号' ; return }
  try {
    const res = await fetch('/api/auth/bind-email', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
      body: JSON.stringify({ email, code })
    })
    const data = await res.json().catch(() => ({}))
    if (res.ok) {
      showEmailBind.value = false
      bindEmailDraft.value = ''
      bindEmailCode.value = ''
      emailCodeSent.value = false
      // 刷新账号信息，让「我的」tab 显示刚绑定的脱敏邮箱
      auth.refresh()
    } else {
      bindEmailError.value = data.error || ('绑定失败（HTTP ' + res.status + '）')
    }
  } catch (e) { bindEmailError.value = '网络异常，请稍后重试' }
}

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
// 局域网同步开关（局域网 tab）：默认关，按需开启才监听 0.0.0.0（避免每次启动触发防火墙弹窗）
const lanSyncEnabled = ref(false)
const lanSyncInfo = ref(null)
async function loadLanSyncSetting() {
  try {
    const res = await fetch('/api/lan/sync-info')
    if (res.ok) {
      const data = await res.json()
      lanSyncEnabled.value = !!data.enabled
      lanSyncInfo.value = data.enabled ? data : null
    }
  } catch (e) {}
}
async function saveLanSyncSetting() {
  try {
    const res = await fetch('/api/lan/' + (lanSyncEnabled.value ? 'enable' : 'disable'), { method: 'POST' })
    if (res.ok) {
      const data = await res.json()
      lanSyncInfo.value = data.enabled ? data : null
    }
  } catch (e) {}
}
function copyLanSyncInfo() {
  const info = lanSyncInfo.value
  if (!info) return
  const text = `IP ${info.ip}\n端口 ${info.port}\nToken ${info.token}`
  if (navigator.clipboard) navigator.clipboard.writeText(text)
}
async function loadProfile() {
  try {
    const res = await fetch('/api/profile')
    if (res.ok) {
      const data = await res.json()
      profile.value = {
        full_name: data.full_name || '',
        work: data.work || '',
        instructions: data.instructions || '',
        gender: data.gender || ''
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
const versionInfo = ref({})
const notifyDisabled = ref(isUpdateNotifyDisabled())
// 自动下载状态：idle | downloading | done | error
const dlState = ref('idle')
const dlError = ref('')
const dlWorking = ref(false)
// 下载进度（后端 /api/update/download/status 返回 percent 0~100；下载完自动应用在下次启动）
const dlPercent = ref(0)
const dlPercentText = computed(() => {
  const p = Math.round(dlPercent.value)
  if (p <= 0) return '正在下载…'
  if (p >= 100) return '解压安装包…'
  return `下载中 ${p}%`
})
let dlTimer = null

async function loadVersion() {
  versionLoading.value = true
  try {
    const res = await fetch('/api/update/check')
    if (res.ok) {
          const data = await res.json()
          const u = data.ok && data.update ? data.update : {}
          versionInfo.value = u
    }
  } catch (e) {
    versionInfo.value = {}
  } finally {
    versionLoading.value = false
    // 只查询后台下载状态（下载由启动时 App.vue 静默完成，版本 tab 不再触发下载，2026-08-16 用户定稿）
    if (versionInfo.value.has_update) {
      refreshDlStatus()
    }
  }
}

async function refreshDlStatus() {
  try {
    const res = await fetch('/api/update/download/status')
    if (!res.ok) return
    const d = await res.json()
    if (d.state === 'done') {
      dlState.value = 'done'
    } else if (d.state === 'downloading') {
      dlState.value = 'downloading'
      if (typeof d.percent === 'number') dlPercent.value = d.percent
      pollDownloadStatus()
    } else if (d.state === 'error') {
      dlState.value = 'error'
      dlError.value = d.error || '下载失败'
    } else {
      // idle：App.vue 启动/周期轮询已触发后台下载（2026-08-28 用户定稿：无需手动点下载），
      // 这里兜底再触发一次并轮询进度；下载完自动应用在下次启动。
      startAutoDownload()
    }
  } catch { /* 轮询失败忽略 */ }
}

async function startAutoDownload() {
  dlWorking.value = true
  dlError.value = ''
  try {
    const res = await fetch('/api/update/download', { method: 'POST' })
    if (res.ok) {
      const d = await res.json()
      if (d.state === 'done') {
        dlState.value = 'done'
      } else {
        dlState.value = 'downloading'
        pollDownloadStatus()
      }
    } else {
      dlState.value = 'error'
      dlError.value = '触发下载失败'
    }
  } catch (e) {
    dlState.value = 'error'
    dlError.value = '网络异常'
  } finally {
    dlWorking.value = false
  }
}

async function pollDownloadStatus() {
  if (dlTimer) clearInterval(dlTimer)
  dlTimer = setInterval(async () => {
    try {
      const res = await fetch('/api/update/download/status')
      if (!res.ok) return
      const d = await res.json()
      if (d.state === 'downloading') {
        dlState.value = 'downloading'
        if (typeof d.percent === 'number') dlPercent.value = d.percent
      } else if (d.state === 'done') {
        dlState.value = 'done'
        clearInterval(dlTimer)
        dlTimer = null
      } else if (d.state === 'error') {
        dlState.value = 'error'
        dlError.value = d.error || '下载失败'
        clearInterval(dlTimer)
        dlTimer = null
      }
    } catch (e) { /* 轮询失败忽略，下次再试 */ }
  }, 800)
}

function onNotifyDisabledChange() {
  setUpdateNotifyDisabled(notifyDisabled.value)
}

// 一键安装：后台已下好的热补丁 exe → 调后端安装接口，3 秒后自动替换 exe 并重启进新版
// （后端 HandleInstallUpdate /api/update/install，2026-08-29 修复：之前按钮 disabled 只能重启才生效）
const installing = ref(false)
const installError = ref('')
async function onInstallUpdate() {
  if (installing.value) return
  installing.value = true
  installError.value = ''
  try {
    const res = await fetch('/api/update/install', { method: 'POST' })
    const d = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(d.error || `安装失败 (${res.status})`)
    // 成功：后端 3 秒后退出本进程替换 exe，这里保持「正在安装」提示，应用马上会重启
  } catch (err) {
    installing.value = false
    installError.value = err.message || '安装失败，请稍后重试'
  }
}

function handleEsc(e) {
  if (e.key === 'Escape') emit('close')
}


onMounted(() => {
  loadConfigs()
  loadMCPToolOptions()
  loadProfile()
  document.addEventListener('keydown', handleEsc)
  // 其他入口改了模型配置（如 FreeOrderModal 拖拽排序）→ 设置页也刷新顺序
  window.addEventListener('model-config-changed', loadConfigs)
})
onUnmounted(() => {
  document.removeEventListener('keydown', handleEsc)
  window.removeEventListener('model-config-changed', loadConfigs)
  if (dlTimer) clearInterval(dlTimer)
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

.param-row { display: flex; align-items: center; gap: 12px; padding: 7px 0; }
.param-label { flex: none; width: 128px; font-size: 12.5px; color: var(--app-text-soft); }
.search-model-row { display: flex; align-items: center; gap: 8px; flex: 1; min-width: 0; }
.vendor-key-input { flex: 1; min-width: 0; height: 34px; box-sizing: border-box; padding: 0 10px; font: inherit; font-size: 12px; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 8px; }
.vendor-key-save { flex: none; height: 34px; padding: 0 16px; border: 1px solid var(--app-accent); border-radius: 8px; color: #fff; background: var(--app-accent); font: inherit; font-size: 12px; font-weight: 650; cursor: pointer; transition: opacity .15s ease, transform .15s ease; }
.vendor-key-save:hover { opacity: .9; transform: translateY(-1px); }
.vendor-key-save:disabled { opacity: .55; cursor: default; transform: none; }
.firecrawl-key-status { display: block; margin-top: 7px; font-size: 11.5px; color: #12b76a; }
.model-select { height: 34px; box-sizing: border-box; padding: 0 8px; font: inherit; font-size: 12px; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 8px; }

/* 联网/生图能力配置卡片：每个来源一块浅色卡片，字段竖排，一眼看懂 */
.cap-card { margin-top: 8px; padding: 12px 14px; border: 1px solid var(--app-border); border-radius: 12px; background: var(--app-surface-2); }
.cap-card-title { font-size: 12.5px; font-weight: 700; color: var(--app-text); margin-bottom: 10px; }
.cap-field { display: flex; flex-direction: column; gap: 4px; margin-bottom: 10px; }
.cap-field:last-of-type { margin-bottom: 12px; }
.cap-label { font-size: 11.5px; color: var(--app-text-faint); }
.cap-input { width: 100%; height: 34px; box-sizing: border-box; padding: 0 10px; font: inherit; font-size: 12px; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 8px; }
.cap-input:focus { outline: none; border-color: var(--app-accent); }
.cap-actions { display: flex; align-items: center; gap: 8px; }
.cap-card .model-select { width: 100%; }
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
  border: 1px solid var(--app-border); border-radius: 12px; user-select: none; cursor: pointer;
  transition: border-color .16s ease, background .16s ease, transform .16s ease, box-shadow .16s ease;
}

.vendor-head:hover {
  background: var(--app-surface); border-color: color-mix(in srgb, var(--app-accent) 28%, var(--app-border));
  transform: translateY(-1px); box-shadow: 0 7px 20px rgba(0,0,0,.045);
}
.vendor-head:focus-visible { outline: 2px solid var(--app-accent); outline-offset: 2px; }
.vendor-chevron { flex: none; color: var(--app-text-soft); transition: transform .18s ease; }
.vendor-chevron.open { transform: rotate(90deg); }
.vendor-body { padding-top: 8px; }
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
.agg-proxy-input { flex: 1; min-width: 0; font-size: 11.5px; font-family: var(--app-mono-font, ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace); color: var(--app-text); background: var(--app-surface-2); border: 1px solid var(--app-border-soft); border-radius: 6px; padding: 3px 8px; }
.agg-proxy-input:focus { outline: none; border-color: var(--app-accent); }
.agg-copy-feedback {
  position: fixed; left: 50%; bottom: 34px; z-index: 10020;
  display: inline-flex; align-items: center; gap: 7px;
  min-height: 36px; padding: 0 14px;
  border: 1px solid color-mix(in srgb, #16a06a, transparent 70%); border-radius: 999px;
  color: #0f6b49; background: color-mix(in srgb, #ecfdf5 94%, transparent);
  box-shadow: 0 10px 30px rgba(15, 23, 42, .16);
  font-size: 12px; font-weight: 650;
  transform: translateX(-50%); pointer-events: none;
}
.agg-copy-feedback.error { border-color: color-mix(in srgb, #dc4c4c, transparent 70%); color: #b42323; background: color-mix(in srgb, #fff1f1 94%, transparent); }
.agg-copy-toast-enter-active, .agg-copy-toast-leave-active { transition: opacity .18s ease, transform .18s ease; }
.agg-copy-toast-enter-from, .agg-copy-toast-leave-to { opacity: 0; transform: translateX(-50%) translateY(6px); }
.agg-api-tip { font-size: 10.5px; color: var(--app-text-faint); margin-top: 6px; line-height: 1.5; }
/* ===== 一键同步 / 还原（codex / dsh）===== */
.agg-sync-card { margin-top: 14px; padding: 12px; border: 1px solid var(--app-border-soft); border-radius: 10px; background: var(--app-surface-2); }
.agg-sync-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.agg-sync-title { font-size: 12px; font-weight: 600; color: var(--app-text); }
.agg-sync-actions { display: flex; gap: 6px; }
.agg-sync-btn { font-size: 11px; color: #fff; background: var(--app-accent); border: 1px solid var(--app-accent); border-radius: 6px; padding: 4px 12px; cursor: pointer; }
.agg-sync-btn:disabled { opacity: .5; cursor: default; }
.agg-sync-btn.ghost { color: var(--app-text-soft); background: transparent; border-color: var(--app-border); }
.agg-sync-tip { font-size: 10.5px; color: var(--app-text-faint); margin-top: 8px; line-height: 1.5; }
.agg-sync-result { margin-top: 10px; display: flex; flex-direction: column; gap: 10px; }
.agg-sync-item { border: 1px solid var(--app-border-soft); border-radius: 8px; padding: 8px 10px; background: var(--app-surface); }
.agg-sync-item-head { display: flex; align-items: center; justify-content: space-between; }
.agg-sync-tool { font-size: 11.5px; font-weight: 600; color: var(--app-text); text-transform: uppercase; }
.agg-sync-badge { font-size: 10px; padding: 1px 8px; border-radius: 999px; background: var(--app-surface-3); color: var(--app-text-faint); }
.agg-sync-badge.ok { background: rgba(34,197,94,.15); color: #22c55e; }
.agg-sync-badge.err { background: rgba(239,68,68,.15); color: #ef4444; }
.agg-sync-snippet { font-size: 10px; color: var(--app-text-soft); background: var(--app-bg); border-radius: 6px; padding: 8px; margin: 8px 0 6px; max-height: 180px; overflow: auto; white-space: pre-wrap; word-break: break-all; }
.agg-sync-item-actions { display: flex; gap: 6px; flex-wrap: wrap; align-items: center; }
.agg-sync-mini { font-size: 10px; color: var(--app-text-soft); background: var(--app-surface-3); border: 1px solid var(--app-border-soft); border-radius: 5px; padding: 3px 9px; cursor: pointer; }
.agg-sync-mini.primary { color: #fff; background: var(--app-accent); border-color: var(--app-accent); }
.agg-sync-mini:disabled { opacity: .6; cursor: default; }
.agg-sync-err { font-size: 10px; color: #ef4444; margin-top: 4px; }
/* ===== 聚合 API 暴露模型配置（官方遴选 / 用户自定义，issue #5）===== */
.agg-mode-toggle { display: inline-flex; gap: 4px; flex: 1; }
.agg-mode-toggle button { font-size: 10.5px; color: var(--app-text-soft); background: var(--app-surface-2); border: 1px solid var(--app-border-soft); border-radius: 6px; padding: 2px 10px; cursor: pointer; }
.agg-mode-toggle button.on { color: #fff; background: var(--app-accent); border-color: var(--app-accent); }
.agg-mode-tabs { display: inline-flex; gap: 4px; flex: 1; flex-wrap: wrap; align-items: center; }
.agg-mode-tabs > button { font-size: 10.5px; color: var(--app-text-soft); background: var(--app-surface-2); border: 1px solid var(--app-border-soft); border-radius: 6px; padding: 2px 10px; cursor: pointer; }
.agg-mode-tabs > button.on { color: #fff; background: var(--app-accent); border-color: var(--app-accent); }
.agg-tag-input { font-size: 10.5px; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-accent); border-radius: 6px; padding: 1px 6px; outline: none; width: 76px; }
.agg-tag-add { font-size: 13px; line-height: 1; color: var(--app-accent); background: var(--app-surface-2); border: 1px solid var(--app-border-soft); border-radius: 6px; padding: 1px 8px; cursor: pointer; flex: none; }
.agg-tag-add:hover { background: var(--app-surface-3); }
.agg-tag-del { color: var(--app-text-faint); cursor: pointer; font-size: 12px; padding: 0 2px; flex: none; }
.agg-tag-del:hover { color: #ff453a; }
.agg-cfg-search { width: 100%; box-sizing: border-box; margin-top: 8px; font-size: 11.5px; color: var(--app-text); background: var(--app-surface-2); border: 1px solid var(--app-border-soft); border-radius: 6px; padding: 5px 9px; outline: none; }
.agg-cfg-list { max-height: 260px; overflow-y: auto; margin-top: 8px; border: 1px solid var(--app-border-soft); border-radius: 8px; padding: 4px; }
.agg-cfg-group { border-bottom: 1px solid var(--app-border-soft); }
.agg-cfg-group:last-child { border-bottom: none; }
.agg-cfg-group-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 0; }
.agg-cfg-toggle { display: flex; align-items: center; gap: 8px; padding: 7px 8px; background: none; border: none; cursor: pointer; text-align: left; border-radius: 6px; flex: 1; }
.agg-cfg-toggle:hover { background: var(--app-surface-2); }
.agg-cfg-select-all { font-size: 10px; color: var(--app-accent); background: none; border: 1px solid var(--app-border-soft); border-radius: 6px; padding: 2px 8px; cursor: pointer; flex: none; margin-right: 4px; }
.agg-cfg-select-all:hover { background: var(--app-surface-2); }
.agg-cfg-chevron { font-size: 9px; color: var(--app-text-faint); transition: transform 0.15s; flex: none; }
.agg-cfg-chevron.open { transform: rotate(90deg); }
.agg-cfg-count { font-size: 9.5px; color: var(--app-text-faint); background: var(--app-surface-2); border-radius: 8px; padding: 0 6px; flex: none; }
.agg-cfg-group-body { padding: 2px 0 4px 18px; }
.agg-cfg-item { display: flex; align-items: center; gap: 8px; padding: 5px 8px; border-radius: 6px; cursor: pointer; }
.agg-cfg-item:hover { background: var(--app-surface-2); }
.agg-cfg-item.off { opacity: 0.45; cursor: not-allowed; }
.agg-cfg-item input { flex: none; }
.agg-cfg-vendor { font-size: 10.5px; color: var(--app-accent); flex: none; max-width: 110px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.agg-cfg-name { font-size: 11.5px; color: var(--app-text); flex: none; max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.agg-cfg-model { font-size: 10px; color: var(--app-text-faint); font-family: var(--app-mono-font, ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.agg-cfg-nokey { font-size: 9.5px; color: #d97b4a; flex: none; }
.agg-cfg-dead { font-size: 9px; color: var(--app-text-faint); background: #e8e8e8; border: 1px solid #d0d0d0; border-radius: 4px; padding: 0 4px; flex: none; }
.agg-cfg-item.dead .agg-cfg-name { color: var(--app-text-faint); text-decoration: line-through; }
.agg-cfg-item.dead .agg-cfg-model { opacity: .55; }
.agg-cfg-item.dead { opacity: .72; }
.agg-cfg-nochat { font-size: 9px; color: var(--app-text-faint); background: var(--app-surface-2); border: 1px solid var(--app-border-soft); border-radius: 4px; padding: 0 4px; flex: none; }
/* 模型名右边的「大众点评」信息标签 */
.agg-cfg-item-wrap { margin-bottom: 2px; }
.agg-cfg-info { display: inline-flex; align-items: center; gap: 3px; margin-left: auto; flex: none; font-size: 10px; cursor: pointer; background: var(--app-surface-2); border: 1px solid var(--app-border-soft); border-radius: 999px; padding: 1px 7px; color: var(--app-text-soft); }
.agg-cfg-info:hover { background: var(--app-surface-3); }
.agg-cfg-info.on { color: var(--app-accent); border-color: var(--app-accent); background: rgba(124, 108, 255, 0.1); }
.agg-cfg-info-stars { color: #ffb400; letter-spacing: -1px; }
.agg-cfg-info-num { font-variant-numeric: tabular-nums; font-weight: 600; }
/* 大众点评式评论卡片 */
.agg-review-card { background: var(--app-surface-2); border: 1px solid var(--app-border-soft); border-radius: 8px; padding: 8px 10px; margin: 4px 2px 8px; }
.agg-review-card-head { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.agg-review-model { font-size: 11.5px; font-weight: 600; color: var(--app-text); }
.agg-review-avg { font-size: 13px; font-weight: 700; color: #ffb400; }
.agg-review-stars { font-size: 11px; color: #ffb400; letter-spacing: -1px; }
.agg-review-total { font-size: 10px; color: var(--app-text-faint); margin-left: auto; }
.agg-review-item { padding: 5px 0; border-top: 1px dashed var(--app-border-soft); }
.agg-review-item:first-child { border-top: none; }
.agg-review-item-head { display: flex; align-items: center; gap: 6px; margin-bottom: 2px; }
.agg-review-user { font-size: 10.5px; font-weight: 600; color: var(--app-text-soft); }
.agg-review-text { font-size: 11px; color: var(--app-text); line-height: 1.5; }
.agg-review-empty { font-size: 10.5px; color: var(--app-text-faint); padding: 4px 0; }
/* ===== 聚合池健康度 ===== */
.agg-health-card { margin-top: 14px; background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 10px; padding: 12px; }
.agg-health-head { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.agg-health-title { font-size: 12.5px; font-weight: 600; color: var(--app-text); }
.agg-health-summary { font-size: 10.5px; color: var(--app-text-soft); display: inline-flex; align-items: center; gap: 4px; }
.agg-health-dot { display: inline-block; min-width: 16px; text-align: center; padding: 1px 5px; border-radius: 8px; font-size: 10px; background: var(--app-surface-2); color: var(--app-text-faint); }
.agg-health-error { font-size: 12px; color: var(--app-danger, #e5484d); background: var(--app-surface-2); border: 1px solid var(--app-danger, #e5484d); border-radius: 8px; padding: 10px 12px; line-height: 1.5; }
.agg-health-error-sub { font-size: 11px; color: var(--app-text-soft); margin-top: 4px; }
.agg-health-dot.ok { background: rgba(52, 199, 89, 0.15); color: #34c759; }
.agg-health-dot.warn.on { background: rgba(255, 69, 58, 0.15); color: #ff453a; }
.agg-health-block { margin-bottom: 12px; }
.agg-health-block-title { font-size: 10.5px; color: var(--app-text-faint); margin: 8px 0 6px; }
.agg-health-row { display: flex; align-items: center; gap: 8px; padding: 5px 8px; border-radius: 7px; font-size: 11.5px; }
.agg-health-row:nth-child(odd) { background: var(--app-surface-2); }
.agg-health-row.bad { opacity: 0.55; }
.agg-health-vendor { color: var(--app-text-soft); font-size: 10px; flex: none; min-width: 78px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.agg-health-name { color: var(--app-text); flex: 1; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.agg-health-order { font-size: 10px; color: var(--app-accent); background: var(--app-surface-3); border-radius: 4px; padding: 1px 5px; flex: none; }
.agg-health-latency { flex: none; display: inline-flex; align-items: center; gap: 5px; width: 92px; font-variant-numeric: tabular-nums; }
.agg-health-latency b { font-size: 10.5px; font-weight: 600; min-width: 38px; text-align: right; }
.agg-health-latency.fast b { color: #34c759; }
.agg-health-latency.mid b { color: #ffcc00; }
.agg-health-latency.slow b { color: #ff453a; }
.agg-health-latency.none b, .agg-health-latency.off b { color: var(--app-text-faint); }
.agg-health-bar { display: inline-block; height: 5px; border-radius: 3px; min-width: 0; transition: width 0.3s ease; }
.agg-health-latency.fast .agg-health-bar { background: #34c759; }
.agg-health-latency.mid .agg-health-bar { background: #ffcc00; }
.agg-health-latency.slow .agg-health-bar { background: #ff453a; }
.agg-health-latency.none .agg-health-bar { background: var(--app-surface-3); }
.agg-health-latency.off .agg-health-bar { background: var(--app-surface-3); }
.agg-health-badge { flex: none; font-size: 9.5px; padding: 1px 7px; border-radius: 8px; background: rgba(52, 199, 89, 0.15); color: #34c759; }
.agg-health-badge.off { background: var(--app-surface-3); color: var(--app-text-faint); }
.agg-health-foot { font-size: 9.5px; color: var(--app-text-faint); margin-top: 4px; line-height: 1.5; }
/* 自定义 API 锁 + 解锁弹窗 */
.locked { opacity: 0.5; cursor: pointer; }
.locked:hover { opacity: 0.7; }
.agree-text {
  text-align: left; font-size: 12.5px; line-height: 1.7; color: var(--app-text-soft);
  background: var(--app-bg); border-radius: 10px; padding: 12px 14px; margin: 12px 0; max-height: 200px; overflow-y: auto;
}
.agree-check {
  display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--app-text);
  cursor: pointer; justify-content: center; margin-bottom: 4px;
}
.agree-check input { accent-color: var(--app-accent); width: 15px; height: 15px; }
.countdown-disabled {
  background: #e05252 !important; color: #fff !important; cursor: not-allowed !important; opacity: .9;
}
/* 彼岸花哥特弹窗 */
.gate-modal {
  background: linear-gradient(160deg, #0a0a0a 0%, #1a0a0a 40%, #0d0505 100%) !important;
  border: 1px solid #3a1a1a !important; box-shadow: 0 0 40px rgba(230,57,70,.15), 0 20px 60px rgba(0,0,0,.5) !important;
}
.gate-flower { margin: 0 0 6px; filter: drop-shadow(0 0 8px rgba(230,57,70,.6)); animation: gate-flower-breathe 3.2s ease-in-out infinite; }
.gate-title {
  font-size: 20px; font-weight: 900; letter-spacing: 4px;
  background: linear-gradient(135deg, #e63946 0%, #ff6b6b 25%, #c1121f 50%, #ff8a8a 75%, #e63946 100%);
  background-size: 300% 300%;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent; text-shadow: none; margin-bottom: 2px;
  animation: gate-gradient-flow 3.6s linear infinite;
}
.gate-sub {
  font-size: 10.5px; color: #8b4a4a; letter-spacing: 3px; font-weight: 600; margin-bottom: 10px;
  font-family: 'Georgia', serif;
  animation: gate-fade-up .6s ease .25s both;
}
.gate-text {
  background: rgba(20,5,5,.5) !important; border: 1px solid #3a1a1a !important;
  color: #d4a0a0 !important; font-size: 12.5px !important; line-height: 1.8 !important;
  animation: gate-fade-up .7s ease .45s both;
  scrollbar-width: thin; scrollbar-color: #7a2a2a transparent;
}
.gate-text::-webkit-scrollbar { width: 6px; }
.gate-text::-webkit-scrollbar-track { background: transparent; }
.gate-text::-webkit-scrollbar-thumb {
  background: linear-gradient(180deg, #7a2a2a, #c1121f); border-radius: 3px;
}
.gate-text::-webkit-scrollbar-thumb:hover { background: #e63946; }
@keyframes gate-gradient-flow {
  0% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
  100% { background-position: 0% 50%; }
}
@keyframes gate-flower-breathe {
  0%, 100% { transform: translateY(0) scale(1); opacity: .85; }
  50% { transform: translateY(-4px) scale(1.04); opacity: 1; }
}
@keyframes gate-fade-up {
  from { opacity: 0; transform: translateY(8px); }
  to { opacity: 1; transform: translateY(0); }
}
.gate-text b { color: #ff6b6b; }
.gate-modal .agree-check { color: #d4a0a0; }
.gate-modal .agree-check input { accent-color: #e63946; }
.gate-modal .mm-btn-cancel { border-color: #3a1a1a; color: #8b4a4a; background: transparent; }
.gate-modal .mm-btn-primary { background: #e63946; border: none; color: #fff; }
.gate-modal .mm-btn-primary:disabled { opacity: .5; }
.gate-modal .mm-error { color: #e63946; }
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
.vendor-key-inline { display: flex; align-items: center; gap: 8px; background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 10px; padding: 8px 10px; margin-bottom: 8px; }
.vendor-key-input { flex: 1; min-width: 0; font-size: 12.5px; color: var(--app-text); border: 1px solid var(--app-border); border-radius: 8px; padding: 6px 10px; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif; }
.vendor-key-input:focus { outline: none; border-color: #c0c0c0; }
.vendor-key-save { font-size: 12px; font-weight: 600; color: #fff; background: #1a1a1a; border: none; border-radius: 8px; padding: 6px 14px; cursor: pointer; flex-shrink: 0; }
.vendor-key-save:hover { background: #333; }
.persona-preset-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 10px; margin-top: 12px; }
.persona-preset-card { display: flex; flex-direction: column; align-items: flex-start; gap: 4px; padding: 12px; border: 1px solid var(--app-border); border-radius: 12px; background: var(--app-surface); cursor: pointer; text-align: left; transition: border-color .15s, background .15s; }
.persona-preset-card:hover { border-color: var(--app-border-soft); background: var(--app-surface-2); }
.persona-preset-card.on { border-color: color-mix(in srgb, var(--app-accent, #7c6cf0) 55%, var(--app-border)); background: color-mix(in srgb, var(--app-accent, #7c6cf0) 10%, var(--app-surface)); }
.persona-preset-icon { flex: none; color: var(--app-text-soft); }
.persona-preset-card.on .persona-preset-icon { color: var(--app-accent, #7c6cf0); }
.persona-preset-name { font-size: 14px; font-weight: 700; color: var(--app-text); }
.persona-preset-desc { font-size: 11px; color: var(--app-text-faint); line-height: 1.4; }
.persona-group-title { font-size: 11px; font-weight: 700; color: var(--app-text-soft); text-transform: uppercase; letter-spacing: .04em; margin: 16px 0 8px; }
.persona-preset-del { position: absolute; top: 4px; right: 6px; width: 18px; height: 18px; display: flex; align-items: center; justify-content: center; border-radius: 50%; font-size: 13px; font-weight: 700; color: var(--app-text-soft); background: var(--app-surface-3); cursor: pointer; opacity: 0; transition: opacity .12s; line-height: 1; }
.persona-preset-card:hover .persona-preset-del { opacity: 1; }
.persona-preset-del:hover { color: #e74c3c; background: color-mix(in srgb, #e74c3c 15%, var(--app-surface-3)); }
.persona-preset-card { position: relative; } /* 为删除按钮定位 */
.persona-random-card { margin-top: 0; } /* 就是普通卡片 */
.persona-save-preset-row { display: flex; align-items: center; gap: 8px; margin-top: 10px; }
.persona-report-btn { display: inline-flex; align-items: center; gap: 4px; font-size: 11px; font-weight: 600; color: var(--app-accent, #7c6cf0); background: transparent; border: 1px solid color-mix(in srgb, var(--app-accent, #7c6cf0) 40%, var(--app-border)); border-radius: 999px; padding: 3px 10px; cursor: pointer; margin-left: auto; }
.persona-report-btn:hover { background: color-mix(in srgb, var(--app-accent, #7c6cf0) 12%, var(--app-surface)); }
.persona-preset-name-input { flex: 1; background: var(--app-surface-2); color: var(--app-text); border: 1px solid var(--app-border); border-radius: 8px; padding: 7px 10px; font-size: 13px; font-family: inherit; }
.persona-preset-name-input:focus { outline: none; border-color: var(--app-accent, #7c6cf0); }
.persona-divider { height: 1px; background: var(--app-border); margin: 18px 0 14px; }
.persona-actions { display: flex; justify-content: flex-end; margin-top: 10px; }
.persona-textarea { width: 100%; box-sizing: border-box; background: var(--app-surface-2); color: var(--app-text); border: 1px solid var(--app-border); border-radius: 10px; padding: 10px 12px; font-size: 13px; font-family: inherit; line-height: 1.6; resize: vertical; }
.persona-textarea:focus { outline: none; border-color: var(--app-accent, #7c6cf0); }
.persona-toast {
  position: fixed; left: 50%; bottom: 34px; z-index: 10030;
  display: inline-flex; align-items: center; gap: 7px;
  min-height: 36px; padding: 0 14px;
  border: 1px solid color-mix(in srgb, #16a06a, transparent 70%); border-radius: 999px;
  color: #0f6b49; background: color-mix(in srgb, #ecfdf5 94%, transparent);
  box-shadow: 0 10px 30px rgba(15, 23, 42, .16);
  font-size: 12px; font-weight: 650;
  transform: translateX(-50%); pointer-events: none;
}
.persona-toast.error { border-color: color-mix(in srgb, #dc4c4c, transparent 70%); color: #b42323; background: color-mix(in srgb, #fff1f1 94%, transparent); }
.persona-toast-enter-active, .persona-toast-leave-active { transition: opacity .18s ease, transform .18s ease; }
.persona-toast-enter-from, .persona-toast-leave-to { opacity: 0; transform: translateX(-50%) translateY(6px); }
.firecrawl-key-status { font-size: 12px; color: #2e7d32; margin-top: 6px; display: inline-block; }
.vendor-key-cancel { font-size: 12px; font-weight: 600; color: var(--app-text-soft); background: var(--app-surface-3); border: 1px solid var(--app-border); border-radius: 8px; padding: 6px 12px; cursor: pointer; flex-shrink: 0; }
.vendor-key-cancel:hover { background: var(--app-surface-3); }
.api-preset-label { font-size: 11.5px; color: var(--app-text-faint); margin-right: 2px; }
.api-preset-btn { font-size: 11.5px; font-weight: 600; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 999px; padding: 4px 10px; cursor: pointer; transition: all .15s; }
.api-preset-btn:hover { background: var(--app-surface-3); }
.api-preset-btn.active { background: var(--app-accent); color: #fff; border-color: var(--app-accent); }

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

/* 完整动漫皮肤：像 IDE 配置列表，不再做悬浮圆角卡片。 */
.skin-groups { width: 100%; max-width: 480px; margin-left: auto; }
.skin-group-title { margin-bottom: 7px; color: var(--app-text-faint); font-size: 10px; font-weight: 700; letter-spacing: .12em; }
.skin-cards { display: grid; grid-template-columns: 1fr; gap: 0; }
.skin-card {
  position: relative; z-index: 0; display: flex; align-items: center; gap: 12px; min-width: 0; min-height: 66px;
  margin-top: -1px; padding: 9px 11px; color: var(--app-text-soft); background: var(--app-surface);
  border: 1px solid var(--app-border); border-radius: 0; text-align: left; cursor: pointer;
  transition: border-color .14s ease, background .14s ease, color .14s ease;
}
.skin-card:first-child { margin-top: 0; }
.skin-card:focus { outline: none; }
.skin-card:focus-visible { outline: 2px solid var(--skin-accent, var(--app-accent)); outline-offset: -2px; }
.skin-card:hover { z-index: 1; color: var(--app-text); border-color: var(--skin-accent, var(--app-accent)); background: var(--app-surface-2); }
.skin-card.on { z-index: 2; color: var(--app-text); border-color: var(--app-accent); background: color-mix(in srgb, var(--app-accent) 8%, var(--app-surface)); box-shadow: inset 3px 0 0 var(--app-accent); }
.skin-card-preview {
  --skin-accent: #df5656; --skin-secondary: #4056a1; --skin-surface: #fbfaf7;
  position: relative; width: 78px; height: 44px; display: block; flex: 0 0 78px; overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--skin-accent) 34%, var(--app-border)); border-radius: 0;
  background: var(--skin-surface); box-shadow: none;
}
.skin-card-preview::before { content: ''; position: absolute; inset: 0 0 auto; height: 5px; background: linear-gradient(90deg, var(--skin-accent) 0 68%, var(--skin-secondary) 68% 100%); }
.skin-card-preview::after { content: ''; position: absolute; left: 21px; top: 5px; bottom: 0; width: 1px; background: color-mix(in srgb, var(--skin-accent) 22%, var(--app-border)); }
.skin-preview-sidebar { position: absolute; left: 0; top: 5px; bottom: 0; width: 21px; padding-top: 7px; background: color-mix(in srgb, var(--skin-accent) 7%, var(--skin-surface)); }
.skin-preview-sidebar i { display: block; width: 11px; height: 2px; margin: 0 0 4px 5px; background: color-mix(in srgb, var(--skin-secondary) 48%, #fff); }
.skin-preview-editor { position: absolute; left: 28px; right: 6px; top: 12px; bottom: 5px; }
.skin-preview-editor i { display: block; height: 2px; margin-bottom: 4px; background: color-mix(in srgb, var(--skin-secondary) 34%, #fff); }
.skin-preview-editor i:nth-child(1) { width: 72%; background: color-mix(in srgb, var(--skin-accent) 52%, #fff); }
.skin-preview-editor i:nth-child(2) { width: 94%; }
.skin-preview-editor i:nth-child(3) { width: 58%; }
.skin-preview-editor b { position: absolute; right: 0; bottom: 0; width: 13px; height: 9px; background: color-mix(in srgb, var(--skin-secondary) 16%, var(--skin-surface)); border-left: 2px solid var(--skin-secondary); }
.skin-card-info { display: grid; gap: 4px; min-width: 0; }
.skin-card-info strong { overflow: hidden; color: inherit; font-size: 12.5px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.skin-card-info small { overflow: hidden; color: var(--app-text-faint); font-size: 10.5px; text-overflow: ellipsis; white-space: nowrap; }
.skin-card-check { margin-left: auto; flex: none; color: var(--app-accent); }

/* DHS / Skills 实体卡片 */
.entity-card { border: 1px solid var(--app-border); border-radius: 10px; padding: 11px 13px; margin-bottom: 8px; background: var(--app-surface-2); }
.entity-head { display: flex; align-items: center; gap: 8px; color: var(--app-text); }
.entity-name { font-size: 13px; font-weight: 700; }
.entity-badge { font-size: 10.5px; font-weight: 600; color: var(--app-accent); background: var(--app-accent-soft); padding: 1px 8px; border-radius: 999px; }
.entity-meta { margin-top: 5px; font-size: 11.5px; color: var(--app-text-faint); font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; line-height: 1.5; word-break: break-all; }
.entity-tags { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 8px; }
.entity-tag { font-size: 10.5px; color: var(--app-text-soft); background: var(--app-surface-3); border: 1px solid var(--app-border); border-radius: 6px; padding: 2px 7px; font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; }
.dhs-state { color: #047857; background: #d1fae5; }
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
.skill-platform-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; margin-bottom: 12px; }
.skill-platform-card {
  display: flex; align-items: center; gap: 9px; min-width: 0; min-height: 58px; padding: 9px 11px;
  color: var(--app-text); background: var(--app-surface-2); border: 1px solid var(--app-border); border-radius: 11px;
  text-align: left; cursor: pointer; transition: border-color .15s ease, background .15s ease, transform .15s ease;
}
.skill-platform-card:hover { border-color: var(--app-accent-soft); transform: translateY(-1px); }
.skill-platform-card.on { border-color: var(--app-accent); background: var(--app-accent-soft); }
.skill-platform-card.unavailable { opacity: .64; }
.skill-platform-card > span:nth-child(2) { display: grid; gap: 2px; min-width: 0; }
.skill-platform-card strong { font-size: 12.5px; }
.skill-platform-card small { color: var(--app-text-faint); font-size: 10.5px; }
.skill-platform-card i { width: 7px; height: 7px; margin-left: auto; border-radius: 50%; background: var(--app-border); }
.skill-platform-card i.live { background: var(--app-success, var(--app-accent)); box-shadow: 0 0 0 3px var(--app-accent-soft); }
.skill-platform-icon { display: grid; place-items: center; width: 31px; height: 31px; flex: none; border-radius: 9px; color: var(--app-accent); background: var(--app-accent-soft); }
.aggregate-search { width: 100%; margin-bottom: 12px; }
.aggregate-search span { flex: none; color: var(--app-text-faint); font-size: 10.5px; }
.aggregate-skill-card { padding: 12px 13px; margin-bottom: 8px; border: 1px solid var(--app-border); border-radius: 11px; background: var(--app-surface-2); }
.aggregate-skill-card.conflict { border-color: color-mix(in srgb, var(--app-error, var(--app-accent)) 35%, var(--app-border)); }
.aggregate-skill-head, .aggregate-skill-title, .aggregate-location { display: flex; align-items: center; }
.aggregate-skill-head { justify-content: space-between; gap: 10px; }
.aggregate-skill-title { min-width: 0; gap: 7px; color: var(--app-text); }
.aggregate-skill-title strong { overflow: hidden; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.aggregate-conflict { display: inline-flex; align-items: center; gap: 3px; flex: none; padding: 2px 7px; border-radius: 999px; color: var(--app-error, var(--app-accent)); background: color-mix(in srgb, var(--app-error, var(--app-accent)) 10%, transparent); font-size: 10px; font-weight: 650; }
.aggregate-coverage { flex: none; color: var(--app-text-faint); font-family: var(--app-mono-font, ui-monospace, monospace); font-size: 10.5px; }
.aggregate-skill-desc { margin: 6px 0 9px; color: var(--app-text-soft); font-size: 11.5px; line-height: 1.55; }
.aggregate-location-list { display: grid; gap: 6px; }
.aggregate-location { gap: 7px; min-height: 31px; padding: 4px 5px 4px 7px; border-radius: 8px; background: var(--app-surface-3); }
.aggregate-source-pill { display: inline-flex; align-items: center; gap: 4px; width: 72px; flex: none; color: var(--app-text-soft); font-size: 10.5px; font-weight: 650; }
.aggregate-source-pill.is-hermes { color: var(--app-accent); }
.aggregate-location code { min-width: 0; overflow: hidden; color: var(--app-text-soft); font-family: var(--app-mono-font, ui-monospace, monospace); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.aggregate-checksum { margin-left: auto; color: var(--app-text-faint); font-family: var(--app-mono-font, ui-monospace, monospace); font-size: 9.5px; }
.aggregate-location button { min-height: 27px; padding: 0 9px; flex: none; border: 1px solid var(--app-accent); border-radius: 7px; color: var(--app-surface); background: var(--app-accent); font: inherit; font-size: 10.5px; font-weight: 650; cursor: pointer; }
.aggregate-location button:disabled { opacity: .6; cursor: default; }
.aggregate-complete { display: inline-flex; align-items: center; gap: 3px; flex: none; color: var(--app-success, var(--app-accent)); font-size: 10.5px; }
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
  .skill-platform-grid { grid-template-columns: 1fr; }
  .aggregate-location { align-items: flex-start; flex-wrap: wrap; }
  .aggregate-location code { width: calc(100% - 90px); }
  .aggregate-checksum { display: none; }
  .aggregate-location button { width: 100%; }
}

/* Profile（仿图2） */
.profile-row { display: flex; align-items: center; gap: 16px; padding: 9px 0; border-bottom: 1px solid var(--app-border-soft); }
.profile-label { flex-shrink: 0; width: 180px; font-size: 13px; color: var(--app-text); }
.profile-avatar-editor { position: relative; display: flex; align-items: center; gap: 12px; min-width: 0; }
.profile-avatar-button { width: 40px; height: 40px; flex: 0 0 auto; overflow: hidden; padding: 0; border: 0; border-radius: 50%; background: transparent; cursor: pointer; }
.profile-avatar { width: 40px; height: 40px; box-sizing: border-box; border-radius: 50%; background: var(--app-accent); color: #fff; display: flex; align-items: center; justify-content: center; object-fit: cover; font-size: 16px; font-weight: 700; }
.profile-avatar-controls { display: flex; min-width: 0; flex-direction: column; align-items: flex-start; gap: 4px; }
.profile-avatar-actions { display: flex; align-items: center; gap: 10px; }
.profile-avatar-action { padding: 0; border: 0; background: transparent; color: var(--app-text); font: inherit; font-size: 12px; font-weight: 650; cursor: pointer; }
.profile-avatar-action:hover { text-decoration: underline; }
.profile-avatar-action.muted { color: var(--app-text-soft); font-weight: 500; }
.profile-avatar-hint { color: var(--app-text-faint); font-size: 11px; }
.profile-avatar-error { color: #d14343; font-size: 11px; }
.profile-avatar-input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; pointer-events: none; }
.profile-input { flex: 1; min-width: 0; font-size: 13px; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 8px; padding: 8px 12px; }
.profile-input:focus { outline: none; border-color: var(--app-accent); }
/* 账号 UID：灰色小字 */
.profile-uid { font-size: 13px; color: var(--app-text-soft); }
.profile-uid.faint { color: var(--app-text-faint); }
/* 邮箱绑定：脱敏展示 + 补绑/改绑 */
.profile-email-bound { display: flex; align-items: center; gap: 10px; min-width: 0; }
.profile-email-value { font-size: 13px; color: var(--app-text); font-weight: 600; }
.profile-email-action { flex-shrink: 0; padding: 0; border: 0; background: transparent; color: var(--app-accent); font: inherit; font-size: 12px; font-weight: 650; cursor: pointer; }
.profile-email-action:hover { text-decoration: underline; }
.profile-email-action.primary { padding: 6px 12px; background: var(--app-accent); color: #fff; border-radius: 8px; }
.profile-email-action.primary:disabled { opacity: .5; cursor: default; }
.profile-email-action.primary:disabled:hover { text-decoration: none; }
.profile-email-action.muted { color: var(--app-text-faint); font-weight: 500; }
.profile-email-action:disabled { opacity: .5; cursor: default; }
.email-bind-panel { padding: 12px 0 2px; border-bottom: 1px solid var(--app-border-soft); }
.email-bind-hint { font-size: 12px; color: var(--app-text-faint); margin-bottom: 10px; }
.email-bind-row { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.email-bind-row .profile-email-action { min-width: 96px; }
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
/* 局域网同步连接信息（局域网 tab，按需开启后展示） */
.lan-sync-box {
  padding: 12px 14px;
  background: var(--app-surface-2);
  border: 1px solid var(--app-border);
  border-radius: 12px;
  margin-bottom: 12px;
}
.lan-sync-row { display: flex; align-items: center; gap: 10px; margin-bottom: 6px; }
.lan-sync-lbl { width: 40px; font-size: 12px; color: var(--app-text-faint); flex-shrink: 0; }
.lan-sync-val { font-size: 12.5px; font-family: ui-monospace, 'Cascadia Code', Consolas, monospace; background: var(--app-bg); border: 1px solid var(--app-border); border-radius: 6px; padding: 2px 8px; color: var(--app-text); }
.lan-sync-token { word-break: break-all; }
.lan-sync-copy { margin-top: 8px; font-size: 12px; padding: 5px 12px; border: 1px solid var(--app-border); border-radius: 8px; background: var(--app-surface); color: var(--app-text); cursor: pointer; }
.lan-sync-copy:hover { border-color: var(--app-accent); color: var(--app-accent); }
.version-value { color: var(--app-text); font-size: 12.5px; font-weight: 600; }
.version-new { color: var(--app-accent); }
.update-err { color: #e5484d; font-size: 12.5px; word-break: break-all; }
/* 下载进度涂黑按钮：用填充色从左到右表示真实下载进度（2026-08-28 用户定稿） */
.dl-progress-btn {
  position: relative;
  overflow: hidden;
  min-width: 130px;
}
.dl-progress-btn .dl-progress-fill {
  position: absolute;
  inset: 0;
  right: auto;
  background: rgba(0, 0, 0, 0.28);
  transition: width 0.3s ease;
  pointer-events: none;
  border-radius: inherit;
}
.dl-progress-btn .dl-progress-text {
  position: relative;
  z-index: 1;
  white-space: nowrap;
}
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
  .model-select, .seg-control, .theme-swatches, .skin-groups { width: 100%; max-width: none; margin-left: 0; }
  .seg-btn { flex: 1; }
  .skin-cards { grid-template-columns: 1fr; }
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
