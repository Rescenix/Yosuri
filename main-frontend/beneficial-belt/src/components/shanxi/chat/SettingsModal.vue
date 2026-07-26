<template>
  <Teleport to="body">
    <div class="settings-modal-backdrop" @click="$emit('close')" @keydown.esc="$emit('close')">
      <div class="settings-modal-card" @click.stop>
        <div class="settings-modal-header">
          <span class="settings-modal-title">设置</span>
          <button class="settings-modal-close" @click="$emit('close')" title="关闭">
            <Icon icon="mdi:close" width="18" />
          </button>
        </div>
        <div class="settings-modal-body">
          <!-- 左侧边栏 -->
          <div class="settings-sidebar">
            <button class="settings-tab" :class="{ on: activeTab === 'models' }" @click="activeTab = 'models'">
              <Icon icon="mdi:brain" width="16" />模型</button>
            <button class="settings-tab" :class="{ on: activeTab === 'providers' }" @click="activeTab = 'providers'">
              <Icon icon="mdi:server-network-outline" width="16" />提供方</button>
            <button class="settings-tab" :class="{ on: activeTab === 'appearance' }" @click="activeTab = 'appearance'">
              <Icon icon="mdi:palette-outline" width="16" />外观</button>
            <button class="settings-tab" :class="{ on: activeTab === 'mcp' }" @click="activeTab = 'mcp'; loadMCP()">
              <Icon icon="mdi:connection" width="16" />MCP</button>
            <button class="settings-tab" :class="{ on: activeTab === 'kanban' }" @click="activeTab = 'kanban'">
              <Icon icon="mdi:human-greeting-variant" width="16" />看板</button>
            <button class="settings-tab" :class="{ on: activeTab === 'skills' }" @click="activeTab = 'skills'; loadSkills()">
              <Icon icon="mdi:school-outline" width="16" />Skills</button>
            <button class="settings-tab" :class="{ on: activeTab === 'profile' }" @click="activeTab = 'profile'">
              <Icon icon="mdi:account-circle-outline" width="16" />我的</button>
          </div>

          <!-- 右侧内容区 -->
          <div class="settings-content">
            <!-- ========== 模型 ========== -->
            <div v-show="activeTab === 'models'" class="settings-panel">
              <div class="settings-section-title">模型配置</div>
              <div class="settings-section-desc">
                统一模型：一个模型同时处理对话与识图。分开配置：文字对话、识图分析各用各的模型
                （比如聊天走云端大模型、识图走本地 llama.cpp）。候选来自「提供方」里选为可用的模型。
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
                  标注“识图”的模型支持视觉分析；未标注的模型不处理图片。
                </div>
              </template>

              <template v-else>
                <div class="param-row">
                  <span class="param-label">文字模型</span>
                  <select class="model-select" v-model="textModelDraft" @change="setTextModel(textModelDraft)">
                    <option v-if="!chatList.length" value="">先去「提供方」选至少一个可用模型</option>
                    <option v-for="m in chatList" :key="m.value" :value="m.value">{{ m.label }}</option>
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
                  未检测到可用识图模型。本地模型需确保 llama-server 已启动且模型文件存在；云端模型请到「提供方」选择。
                </div>
              </template>
            </div>

            <!-- ========== 提供方 ========== -->
            <div v-show="activeTab === 'providers'" class="settings-panel">
              <div class="settings-section-title">免费模型提供商</div>
              <div class="settings-section-desc">点一个提供商，它的全部模型会直接进聊天下拉框（再点一次移除）。填了 Key 的提供商才会真正进调用链。</div>

              <div v-if="loading" class="settings-loading">加载中...</div>
              <template v-else>
                <div v-for="grp in vendorGroups" :key="grp.vendor" class="vendor-group">
                  <div class="vendor-head">
                    <span class="vendor-name">{{ grp.vendor }}</span>
                    <span class="vendor-count">{{ grp.items.length }} 个模型</span>
                    <span class="vendor-keystate" :class="{ on: grp.hasKey }">{{ grp.hasKey ? '已配 Key' : '未配 Key' }}</span>
                    <button
                      class="model-pick-btn"
                      :class="{ on: isVendorSelected(grp.vendor) }"
                      type="button"
                      @click.stop="toggleVendorModels(grp)"
                    >{{ isVendorSelected(grp.vendor) ? '已选' : '选为可用' }}</button>
                    <button v-if="editingVendor !== grp.vendor" class="vendor-key-btn" @click.stop="startEditVendor(grp)">{{ grp.hasKey ? '改 Key' : '填 Key' }}</button>
                    <button v-else class="vendor-key-btn" @click.stop="cancelVendorEdit">收起</button>
                  </div>
                  <div v-if="editingVendor === grp.vendor" class="vendor-key-inline">
                    <input
                      v-model="vendorKeyDraft"
                      type="password"
                      class="vendor-key-input"
                      :placeholder="grp.hasKey ? '••••••••（留空则不修改）' : '输入 ' + grp.vendor + ' 的 API Key'"
                      @keyup.enter="saveVendorKey(grp)"
                    />
                    <button class="vendor-key-save" type="button" @click="saveVendorKey(grp)">保存</button>
                    <button class="vendor-key-cancel" type="button" @click="cancelVendorEdit">取消</button>
                  </div>
                  <div class="vendor-model-hint">点击「选为可用」将加入：{{ grp.items.map(m => m.name).join('、') }}</div>
                </div>
              </template>

              <div class="settings-section-title" style="margin-top: 18px;">自定义 API 配置</div>
              <div class="settings-section-desc">配置自己的模型接入方式；设为默认的配置会排在调用链最前面。</div>

              <template v-if="!loading">
                <div v-for="cfg in configs" :key="cfg.id" class="api-config-card">
                  <div class="api-config-row">
                    <span class="api-config-name">{{ cfg.name || '未命名配置' }}</span>
                    <span v-if="cfg.is_default" class="api-config-default-badge">默认</span>
                    <div class="api-config-actions">
                      <button
                        class="api-config-action-btn"
                        :class="{ on: isInChatList(cfg.id) }"
                        @click="toggleConfigModel(cfg)"
                      >{{ isInChatList(cfg.id) ? '已选' : '选为可用' }}</button>
                      <button v-if="!cfg.is_default" class="api-config-action-btn" @click="setDefault(cfg.id)">设为默认</button>
                      <button class="api-config-action-btn" @click="startEdit(cfg)">编辑</button>
                      <button class="api-config-action-btn danger" @click="removeConfig(cfg.id)">删除</button>
                    </div>
                  </div>
                  <div class="api-config-meta">
                    {{ cfg.endpoint }} · {{ cfg.default_model || '未指定模型' }} · {{ cfg.api_key_set ? '已设置 Key' : '未设置 Key' }}
                  </div>
                </div>

                <div v-if="!editingConfig" class="api-config-add-btn" @click="startAdd">
                  <Icon icon="mdi:plus" width="15" /> 新增 API 配置
                </div>

                <div v-else class="api-config-form">
                  <div class="api-preset-row">
                    <span class="api-preset-label">预设模板：</span>
                    <button v-for="p in PRESETS" :key="p.name" class="api-preset-btn" type="button" @click="applyPreset(p)">{{ p.name }}</button>
                  </div>
                  <label class="api-form-field">
                    <span>API 名称</span>
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
                  <label class="api-form-field">
                    <span>默认模型名</span>
                    <input v-model="editingConfig.default_model" type="text" placeholder="比如 deepseek-chat" autocomplete="off" />
                  </label>
                  <div class="api-form-actions">
                    <button class="api-form-btn cancel" type="button" @click="cancelEdit">取消</button>
                    <button class="api-form-btn save" type="button" @click="saveConfig">保存</button>
                  </div>
                </div>
              </template>
            </div>

            <!-- ========== 外观 ========== -->
            <div v-show="activeTab === 'appearance'" class="settings-panel">
              <div class="settings-section-title">主题</div>
              <div class="settings-section-desc">选择配色皮肤，切换即时生效并自动保存。</div>
              <div class="param-row" style="align-items: flex-start;">
                <span class="param-label">配色</span>
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

              <div class="settings-section-title" style="margin-top: 10px;">皮肤</div>
              <div class="settings-section-desc">完整氛围皮肤；部分皮肤为会员专享。</div>
              <div class="param-row" style="align-items: flex-start;">
                <span class="param-label">限定皮肤</span>
                <div class="skin-groups">
                  <div v-for="[series, items] in skinThemes" :key="series" class="skin-group">
                    <div class="skin-group-title">{{ series }}</div>
                    <div class="skin-cards">
                      <button
                        v-for="[key, p] in items"
                        :key="key"
                        class="skin-card"
                        :class="{ on: theme === key, locked: p.vip && !isVip }"
                        type="button"
                        :title="p.label"
                        @click="selectTheme(key)"
                      >
                        <span class="skin-card-preview" :style="{ background: p.skin?.dark?.surface || p.accent, borderColor: p.accent }">
                          <span class="skin-card-flame" :style="{ background: p.accent }"></span>
                        </span>
                        <span class="skin-card-info">
                          <span class="skin-card-label">{{ p.label }}</span>
                          <span v-if="p.vip" class="skin-card-badge">
                            <Icon icon="mdi:crown-outline" width="11" />
                            会员
                          </span>
                        </span>
                        <span v-if="p.vip && !isVip" class="skin-card-lock">
                          <Icon icon="mdi:lock-outline" width="14" />
                        </span>
                      </button>
                    </div>
                  </div>
                </div>
              </div>

              <!-- VIP 解锁提示 -->
              <div v-if="vipPromptSkin" class="vip-prompt">
                <Icon icon="mdi:crown-outline" width="16" />
                <span>「{{ THEME_PRESETS[vipPromptSkin]?.label }}」是会员专属皮肤，使用 GitHub 登录解锁后即可使用。</span>
                <button class="vip-prompt-btn" type="button" @click="unlockVipByLogin">GitHub 登录解锁</button>
                <button class="vip-prompt-close" type="button" @click="vipPromptSkin = ''">
                  <Icon icon="mdi:close" width="14" />
                </button>
              </div>

              <div class="param-row">
                <span class="param-label">亮度</span>
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

              <div class="settings-section-title" style="margin-top: 18px;">流式渐变</div>
              <div class="settings-section-desc">AI 回复逐字级联淡入的"瀑布"效果（仿 ChatGPT/Gemini）。改动即时生效并自动保存。</div>

              <div class="param-row">
                <span class="param-label">流式渐变</span>
                <label class="param-switch">
                  <input type="checkbox" v-model="streamFadeConfig.enabled" />
                  <span class="param-switch-track"></span>
                </label>
              </div>

              <template v-if="streamFadeConfig.enabled">
                <div class="param-row">
                  <span class="param-label">淡入时长</span>
                  <input class="param-range" type="range" min="150" max="1500" step="50" v-model.number="streamFadeConfig.fadeMs" />
                  <span class="param-value">{{ streamFadeConfig.fadeMs }} ms</span>
                </div>
                <div class="param-row">
                  <span class="param-label">级联间隔</span>
                  <input class="param-range" type="range" min="0" max="40" step="2" v-model.number="streamFadeConfig.staggerMs" />
                  <span class="param-value">{{ streamFadeConfig.staggerMs }} ms/字</span>
                </div>
                <div class="param-row">
                  <span class="param-label">模糊强度</span>
                  <input class="param-range" type="range" min="0" max="6" step="0.5" v-model.number="streamFadeConfig.blurPx" />
                  <span class="param-value">{{ streamFadeConfig.blurPx }} px</span>
                </div>
                <div class="param-row">
                  <span class="param-label">大块扫过上限</span>
                  <input class="param-range" type="range" min="100" max="1000" step="50" v-model.number="streamFadeConfig.maxSweepMs" />
                  <span class="param-value">{{ streamFadeConfig.maxSweepMs }} ms</span>
                </div>
              </template>
              <div class="param-reset-row">
                <button class="param-reset-btn" type="button" @click="resetStreamFadeConfig">恢复默认</button>
              </div>

              <!-- 精简预览：专注 Markdown / 公式 / 表格，跟随主题，无滚动条 -->
              <div class="settings-section-title" style="margin-top: 18px;">实时预览</div>
              <div class="settings-section-desc">按当前主题与渐变配置循环重播，专注排版 / 公式 / 表格。</div>
              <div class="preview-stage">
                <div class="preview-bubble markdown-body" ref="previewBubble"></div>
              </div>
            </div>

            <!-- ========== MCP ========== -->
            <div v-show="activeTab === 'mcp'" class="settings-panel">
              <div class="settings-section-title">
                MCP 生态
                <button class="inline-refresh" type="button" @click="loadMCP" title="刷新"><Icon icon="mdi:refresh" width="14" :class="{ spin: mcpLoading }" /></button>
              </div>
              <div class="settings-section-desc">项目已接入的 MCP server（读自 <code>{{ mcpConfigPath || 'mcp.json' }}</code>）与运行时注册的工具。</div>
              <div v-if="mcpLoading" class="settings-loading">加载中...</div>
              <template v-else>
                <div v-if="!mcpServers.length" class="settings-empty">未配置 MCP server（项目根目录放 mcp.json 即可接入）。</div>
                <div v-for="s in mcpServers" :key="s.name" class="entity-card">
                  <div class="entity-head">
                    <Icon icon="mdi:connection" width="15" />
                    <span class="entity-name">{{ s.name }}</span>
                    <span class="entity-badge">{{ s.tools.length }} 工具</span>
                  </div>
                  <div class="entity-meta">{{ s.command }} {{ (s.args || []).join(' ') }}</div>
                  <div v-if="s.tools.length" class="entity-tags">
                    <span v-for="t in s.tools" :key="t" class="entity-tag">{{ t.replace('mcp__' + s.name + '__', '') }}</span>
                  </div>
                </div>
              </template>
            </div>

            <!-- ========== Skills ========== -->
            <div v-show="activeTab === 'skills'" class="settings-panel">
              <div class="settings-section-title">
                技能库
                <button class="inline-refresh" type="button" @click="loadSkills" title="刷新"><Icon icon="mdi:refresh" width="14" :class="{ spin: skillsLoading }" /></button>
              </div>
              <div class="settings-section-desc">
                <strong>自研</strong>：工作流成功后自动沉淀（<code>{{ skillsDir || './skills' }}</code>）。
                <strong>外部</strong>：把 Anthropic/Claude 风格的 <code>SKILL.md</code> 文件夹丢进 <code>{{ skillsExtDir || './skills-ext' }}</code> 即被加载。两类都会注入到 agent 的技能库索引。
              </div>
              <div v-if="skillsLoading" class="settings-loading">加载中...</div>
              <template v-else>
                <div v-if="!skills.length" class="settings-empty">技能库为空。跑几次 Code 工作流会自动沉淀自研技能，或往 <code>./skills-ext</code> 丢外部 SKILL.md。</div>
                <div v-for="sk in skills" :key="(sk.source || '') + ':' + sk.name" class="entity-card">
                  <div class="entity-head" @click="toggleSkill(sk.name)" style="cursor:pointer">
                    <Icon :icon="sk.source === 'external' ? 'mdi:puzzle-outline' : 'mdi:school-outline'" width="15" />
                    <span class="entity-name">{{ sk.name }}</span>
                    <span class="entity-badge" :class="sk.source === 'external' ? 'src-ext' : 'src-learned'">
                      {{ sk.source === 'external' ? '外部' : '自研' }}
                    </span>
                    <span v-if="sk.source !== 'external'" class="entity-badge">{{ (sk.steps || []).length }} 步</span>
                    <Icon :icon="expandedSkill === sk.name ? 'mdi:chevron-up' : 'mdi:chevron-down'" width="16" style="margin-left:auto" />
                  </div>
                  <div class="entity-meta">{{ sk.description }}</div>
                  <ol v-if="expandedSkill === sk.name && sk.steps && sk.steps.length" class="skill-steps">
                    <li v-for="(st, i) in sk.steps" :key="i">{{ st }}</li>
                  </ol>
                  <pre v-else-if="expandedSkill === sk.name && sk.body" class="skill-body">{{ sk.body }}</pre>
                </div>
              </template>
            </div>

            <!-- ========== 看板娘 ========== -->
            <div v-show="activeTab === 'kanban'" class="settings-panel">
              <div class="settings-section-title">桌面看板娘</div>
              <div class="settings-section-desc">控制聊天窗口折叠时显示的桌面伴侣组件。开关即时生效并自动保存。</div>

              <div class="param-row">
                <span class="param-label">看板娘</span>
                <label class="param-switch">
                  <input type="checkbox" v-model="kanbanLive2D" />
                  <span class="param-switch-track"></span>
                </label>
              </div>
              <div class="settings-section-desc" style="margin-top:-8px; margin-bottom:14px;">桌面左下角的 Live2D 动态角色（需在 public/live2d/ 下放置模型资源）。</div>

              <div class="param-row">
                <span class="param-label">任务卡片</span>
                <label class="param-switch">
                  <input type="checkbox" v-model="kanbanTaskCard" />
                  <span class="param-switch-track"></span>
                </label>
              </div>
              <div class="settings-section-desc" style="margin-top:-8px; margin-bottom:14px;">桌面左下角的便签/待办任务卡片，可拖动、双击复位。</div>
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

              <div class="settings-section-title" style="margin-top: 18px;">给 AI 的自定义指令</div>
              <div class="settings-section-desc">这些会跨对话注入系统提示词，影响 AI 的语气与行为。</div>
              <textarea class="profile-instructions" v-model="profile.instructions" rows="6" placeholder="例如：用温柔、清晰的语气；理性稳重，不要过度共情。"></textarea>

              <div class="profile-actions">
                <span v-if="profileSaved" class="profile-saved">已保存</span>
                <button class="api-form-btn save" type="button" @click="saveProfile" :disabled="profileSaving">{{ profileSaving ? '保存中…' : '保存' }}</button>
              </div>
            </div>
          </div>

          <div v-if="errorMsg" class="settings-error">{{ errorMsg }}</div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import { chatModelList, setChatModelList, syncChatModelList } from '../composables/chatModelList.js'
import { streamFadeConfig, resetStreamFadeConfig } from '../composables/streamFadeConfig.js'
import { theme, mode, MODE_OPTIONS, THEME_PRESETS, initTheme } from '../composables/useTheme.js'
import { useAuth } from '../../../composables/useAuth.js'
import { renderMarkdown } from './markdownRenderer.js'

// 精简预览样本：专注 Markdown 排版 / 行内+块级公式 / 表格，去掉冗长解说。
const PREVIEW_MD =
`## 排版预览

正文段落、**加粗**、行内代码 \`renderMarkdown\`、行内公式 $E = mc^2$。

$$
x = \\frac{-b \\pm \\sqrt{b^2 - 4ac}}{2a}
$$

| 参数 | 含义 | 建议值 |
| --- | --- | --- |
| fadeMs | 单字淡入 | 500ms |
| staggerMs | 字间级联 | 14ms |
| blurPx | 起始模糊 | 2px |`

const props = defineProps({
  openid: { type: String, default: '' }
})
const emit = defineEmits(['close'])

// 左侧边栏当前 tab
const activeTab = ref('models')

// ============ 会员与皮肤 ============
// isVip 以服务端鉴权为准（useAuth 从 /api/auth/me 的 JWT.is_vip 读取），
// 游客恒为 false，且不再信任 localStorage（杜绝改本地存储白嫖会员皮肤）。
const { isVip } = useAuth()
const vipPromptSkin = ref('')
const colorThemes = computed(() => Object.entries(THEME_PRESETS).filter(([, p]) => !p.fullSkin))
const skinThemes = computed(() => {
  const groups = {}
  Object.entries(THEME_PRESETS)
    .filter(([, p]) => p.fullSkin)
    .forEach(([key, p]) => {
      const series = p.series || '限定皮肤'
      if (!groups[series]) groups[series] = []
      groups[series].push([key, p])
    })
  return Object.entries(groups)
})

function canUseTheme(key) {
  const p = THEME_PRESETS[key]
  return !p?.vip || isVip.value
}

function selectTheme(key) {
  if (!canUseTheme(key)) {
    vipPromptSkin.value = key
    return
  }
  vipPromptSkin.value = ''
  theme.value = key
  // 切到完整皮肤时强制暗色，避免亮色下氛围不对
  const p = THEME_PRESETS[key]
  if (p?.fullSkin && mode.value === 'light') {
    mode.value = 'dark'
  }
}

// 会员皮肤点不开时：已登录但非会员（理论上不会发生，GitHub/管理员登录即会员）、
// 或未登录游客。统一引导走 GitHub 登录解锁，不再提供本地伪造入口。
function unlockVipByLogin() {
  window.location.href = '/api/auth/github'
}

// ============ 流式渐变无限循环预览（纯前端，不花 token） ============
const previewBubble = ref(null)
const previewHtml = ref('')
let previewTimer = null
let previewOffset = 0

// 用真实 renderMarkdown 把样本渲染成 HTML（含 katex/表格/代码），按当前配置
// 逐字包 span.stream-fade-seg 触发全局 om-stream-fade 动画；到尾部后从头循环。
function renderPreviewFrame() {
  const el = previewBubble.value
  if (!el) return
  const { fadeMs, staggerMs, blurPx } = streamFadeConfig
  const html = renderMarkdown(PREVIEW_MD, true)
  const text = PREVIEW_MD
  // 用一段“打字机”窗口：每帧多露几个字 + 给新露出的字加淡入动画
  const spanAll = (fullHtml) => {
    // 简单地整段插 span 会破坏标签，这里对纯文本快照逐字动画不合适；
    // 改为：整段 v-html 渲染，再对当前可见文本节点逐字包 span 做一次性淡入。
    el.innerHTML = fullHtml
    const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT, {
      acceptNode(n) {
        const p = n.parentElement
        if (p && p.closest('pre, code, table, .katex, .code-btn-group')) return NodeFilter.FILTER_REJECT
        return NodeFilter.FILTER_ACCEPT
      }
    })
    const nodes = []
    let n
    while ((n = walker.nextNode())) nodes.push(n)
    let i = 0
    for (const node of nodes) {
      const text = node.nodeValue
      const frag = document.createDocumentFragment()
      for (const ch of text) {
        const span = document.createElement('span')
        span.className = 'stream-fade-seg'
        span.textContent = ch
        span.style.animationDuration = fadeMs + 'ms'
        span.style.animationDelay = (i * staggerMs) + 'ms'
        span.style.setProperty('--sf-blur', blurPx + 'px')
        frag.appendChild(span)
        i++
      }
      node.parentNode.replaceChild(frag, node)
    }
  }
  spanAll(html)
}

function startPreviewLoop() {
  stopPreviewLoop()
  renderPreviewFrame()
  // 整段淡入播完（按字符数估算时长）后从头循环
  const total = PREVIEW_MD.length
  const { fadeMs, staggerMs } = streamFadeConfig
  const oneRound = total * staggerMs + fadeMs + 800
  previewTimer = setInterval(() => {
    if (activeTab.value === 'appearance') renderPreviewFrame()
  }, Math.max(2500, oneRound))
}
function stopPreviewLoop() {
  if (previewTimer) { clearInterval(previewTimer); previewTimer = null }
}

const PRESETS = [
  { name: 'DeepSeek', endpoint: 'https://api.deepseek.com' }
]
const MASKED = '••••••••'

const configs = ref([])
const freeModels = ref([])
const loading = ref(true)
const errorMsg = ref('')
const editingConfig = ref(null)
const editingVendor = ref(null)
const vendorKeyDraft = ref('')
const isNew = ref(false)

const vendorGroups = computed(() => {
  const map = new Map()
  for (const fm of freeModels.value) {
    // 本地模型（llama.cpp / Local=true）不放在提供方里手动勾选，
    // 而是自动进入识图模型候选，做到"下载即用"。
    if (fm.local) continue
    const v = fm.vendor || '其他'
    if (!map.has(v)) map.set(v, { vendor: v, items: [], hasKey: false })
    const g = map.get(v)
    g.items.push(fm)
    if (fm.api_key_set) g.hasKey = true
  }
  return Array.from(map.values())
})

function configUrl() {
  return `/api/models/config${props.openid ? '?openid=' + encodeURIComponent(props.openid) : ''}`
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
}

function startAdd() {
  isNew.value = true
  errorMsg.value = ''
  editingConfig.value = {
    id: 'cfg_' + Date.now().toString(36),
    name: '', endpoint: '', api_key: '', api_key_set: false,
    default_model: '', is_default: configs.value.length === 0
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
}
function cancelVendorEdit() {
  editingVendor.value = null
  vendorKeyDraft.value = ''
}
async function saveVendorKey(grp) {
  const key = vendorKeyDraft.value
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

async function saveConfig() {
  if (!editingConfig.value.endpoint.trim()) {
    errorMsg.value = 'Endpoint 不能为空'
    return
  }
  errorMsg.value = ''
  const entry = {
    id: editingConfig.value.id,
    name: editingConfig.value.name,
    endpoint: editingConfig.value.endpoint,
    api_key: editingConfig.value.api_key || MASKED,
    default_model: editingConfig.value.default_model,
    is_default: editingConfig.value.is_default
  }
  let next = isNew.value
    ? [...configs.value, entry]
    : configs.value.map(c => (c.id === entry.id ? entry : c))
  if (entry.is_default) {
    next = next.map(c => ({ ...c, api_key: c.id === entry.id ? entry.api_key : MASKED, is_default: c.id === entry.id }))
  }
  try {
    await persist(next)
    await loadConfigs()
    editingConfig.value = null
  } catch (e) {
    errorMsg.value = e.message
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

const chatList = chatModelList
function loadChatList() {
  syncChatModelList()
}

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

// id → 是否支持识图，合并免费池 + 自定义配置两个来源（/api/models/config 都带 vision 字段）
const visionByID = computed(() => {
  const map = {}
  for (const fm of freeModels.value) map[fm.id] = !!fm.vision
  for (const c of configs.value) map[c.id] = !!c.vision
  return map
})
// 识图模型候选：用户手动在提供方里选的识图模型 + 本地模型（自动加入，无需勾选）。
const visionCapableChatList = computed(() => {
  const manual = chatList.value.filter(m => visionByID.value[m.value])
  const locals = freeModels.value
    .filter(fm => fm.local && fm.vision)
    .map(fm => ({ label: fm.name, value: fm.id }))
  return [...manual, ...locals]
})

// 当用户没有显式选过识图模型，且当前有可用识图模型时，自动默认选中第一个。
// 本地模型优先级最高：用户下载了本地模型，就默认用它识图。
watch(visionCapableChatList, (list) => {
  if (!visionModelDraft.value && list.length) {
    // 优先本地模型；没有本地模型时取第一个可用识图模型。
    const local = list.find(m => {
      const fm = freeModels.value.find(f => f.id === m.value)
      return fm && fm.local
    })
    const pick = local || list[0]
    visionModelDraft.value = pick.value
    localStorage.setItem(VISION_MODEL_KEY, pick.value)
  }
}, { immediate: true })

function isInChatList(value) {
  return chatList.value.some(m => m.value === value)
}
function isVendorSelected(vendor) {
  const items = vendorGroups.value.find(g => g.vendor === vendor)?.items || []
  if (items.length === 0) return false
  return items.every(it => isInChatList(it.id))
}
function toggleVendorModels(grp) {
  const entries = grp.items.map(fm => ({ label: fm.name, value: fm.id }))
  const allIn = entries.every(e => isInChatList(e.value))
  let next
  if (allIn) {
    const drop = new Set(entries.map(e => e.value))
    next = chatList.value.filter(m => !drop.has(m.value))
  } else {
    const have = new Set(chatList.value.map(m => m.value))
    next = [...chatList.value]
    for (const e of entries) if (!have.has(e.value)) next.push(e)
  }
  setChatModelList(next)
}
// 自定义 API 配置（DS 等）的「选为可用」：把它作为一个模型条目写进共享的 chatModelList，
// 聊天下拉 modelOptions 只读 chatModelList，没进这里就永远不显示。value 用配置 id（后端
// resolveExact 按 id 精确路由到用户自定义配置），label 用配置名。
function toggleConfigModel(cfg) {
  const value = cfg.id
  let next
  if (isInChatList(value)) {
    next = chatList.value.filter(m => m.value !== value)
  } else {
    next = [...chatList.value, { label: cfg.name || '未命名配置', value }]
  }
  setChatModelList(next)
}

// ============ MCP ============
const mcpServers = ref([])
const mcpLoading = ref(false)
const mcpConfigPath = ref('')
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

// ============ Skills ============
const skills = ref([])
const skillsLoading = ref(false)
const skillsDir = ref('')
const skillsExtDir = ref('')
const expandedSkill = ref(null)
let skillsLoaded = false
function toggleSkill(name) { expandedSkill.value = expandedSkill.value === name ? null : name }
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
// ============ Profile ============
const profile = ref({ full_name: '', work: '', instructions: '' })
const profileSaving = ref(false)
const profileSaved = ref(false)
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

function handleEsc(e) {
  if (e.key === 'Escape') emit('close')
}

// ============ 看板娘开关 ============
const kanbanLive2D = ref(localStorage.getItem('kanban_live2d') !== '0')
const kanbanTaskCard = ref(localStorage.getItem('kanban_taskcard') !== '0')
watch(kanbanLive2D, (v) => {
  localStorage.setItem('kanban_live2d', v ? '1' : '0')
  window.dispatchEvent(new CustomEvent('kanban-toggle', { detail: { key: 'kanban_live2d', value: v } }))
})
watch(kanbanTaskCard, (v) => {
  localStorage.setItem('kanban_taskcard', v ? '1' : '0')
  window.dispatchEvent(new CustomEvent('kanban-toggle', { detail: { key: 'kanban_taskcard', value: v } }))
})

onMounted(() => {
  loadConfigs()
  loadChatList()
  loadProfile()
  document.addEventListener('keydown', handleEsc)
  nextTick(startPreviewLoop)
})
onUnmounted(() => {
  document.removeEventListener('keydown', handleEsc)
  stopPreviewLoop()
})
</script>

<style scoped>
.settings-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(20, 18, 15, 0.35);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  z-index: 20000;
  display: flex;
  align-items: center;
  justify-content: center;
}
.settings-modal-card {
  width: 1020px;
  height: 600px;
  background: var(--app-surface);
  border-radius: 16px;
  box-shadow: var(--app-shadow);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.settings-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--app-border);
  flex-shrink: 0;
}
.settings-modal-title { font-size: 15px; font-weight: 700; color: var(--app-text); }
.settings-modal-close {
  display: flex; align-items: center; justify-content: center;
  width: 28px; height: 28px; border-radius: 6px; border: none;
  background: transparent; cursor: pointer; color: var(--app-text-soft);
}
.settings-modal-close:hover { background: var(--app-surface-3); }
.settings-modal-body {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
/* 左侧边栏 */
.settings-sidebar {
  width: 168px;
  flex-shrink: 0;
  border-right: 1px solid var(--app-border-soft);
  padding: 16px 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: var(--app-surface-2);
  overflow-y: auto;
}
.settings-tab {
  display: flex;
  align-items: center;
  gap: 8px;
  text-align: left;
  padding: 9px 14px;
  font-size: 13.5px;
  font-weight: 600;
  color: var(--app-text-soft);
  background: transparent;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.settings-tab:hover { background: var(--app-surface-3); }
.settings-tab.on { color: #fff; background: var(--app-accent); }
/* 右侧内容 */
.settings-content {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: 18px 22px 22px;
  background: var(--app-surface);
  color: var(--app-text);
}
.settings-panel { display: block; }

.settings-section-title { font-size: 13.5px; font-weight: 700; color: var(--app-text); margin-bottom: 4px; display: flex; align-items: center; gap: 6px; }
.settings-section-desc { font-size: 12px; color: var(--app-text-faint); margin-bottom: 14px; line-height: 1.5; }
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
  border: 1px solid var(--app-border); border-radius: 7px; padding: 6px 10px;
  cursor: pointer;
}
.model-select:focus { outline: none; border-color: var(--app-accent); }

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

.vendor-group { margin-bottom: 8px; }
.vendor-group:last-child { margin-bottom: 0; }
.vendor-head {
  display: flex; align-items: center; flex-wrap: wrap; gap: 8px;
  padding: 9px 12px; background: var(--app-surface-3); border: 1px solid var(--app-border); border-radius: 10px; user-select: none;
}
.vendor-head:hover { background: var(--app-surface-3); }
.vendor-name { font-size: 13px; font-weight: 700; color: var(--app-text); flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.vendor-count { font-size: 10.5px; font-weight: 600; color: var(--app-text-soft); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 999px; padding: 1px 8px; flex-shrink: 0; }
.vendor-keystate { font-size: 10.5px; font-weight: 600; color: var(--app-text-faint); flex-shrink: 0; }
.vendor-keystate.on { color: #12b76a; }
.vendor-key-btn { font-size: 11px; font-weight: 600; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 999px; padding: 3px 10px; cursor: pointer; flex-shrink: 0; }
.vendor-key-btn:hover { background: var(--app-surface-3); }
.vendor-model-hint { margin-top: 6px; font-size: 11px; color: var(--app-text-faint); line-height: 1.5; padding-left: 2px; }
.vendor-key-inline { display: flex; align-items: center; gap: 8px; background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 10px; padding: 8px 10px; margin-bottom: 8px; }
.vendor-key-input { flex: 1; min-width: 0; font-size: 12.5px; color: var(--app-text); border: 1px solid var(--app-border); border-radius: 8px; padding: 6px 10px; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif; }
.vendor-key-input:focus { outline: none; border-color: #c0c0c0; }
.vendor-key-save { font-size: 12px; font-weight: 600; color: #fff; background: #1a1a1a; border: none; border-radius: 8px; padding: 6px 14px; cursor: pointer; flex-shrink: 0; }
.vendor-key-save:hover { background: #333; }
.vendor-key-cancel { font-size: 12px; font-weight: 600; color: var(--app-text-soft); background: var(--app-surface-3); border: 1px solid var(--app-border); border-radius: 8px; padding: 6px 12px; cursor: pointer; flex-shrink: 0; }
.vendor-key-cancel:hover { background: var(--app-surface-3); }
.api-preset-label { font-size: 11.5px; color: var(--app-text-faint); margin-right: 2px; }
.api-preset-btn { font-size: 11.5px; font-weight: 600; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 999px; padding: 4px 10px; cursor: pointer; }
.api-preset-btn:hover { background: var(--app-surface-3); }

.api-form-field { display: flex; flex-direction: column; gap: 4px; margin-bottom: 10px; }
.api-form-field span { font-size: 11.5px; color: var(--app-text-soft); font-weight: 600; }
.api-form-field input { font-size: 13px; padding: 7px 10px; border: 1px solid var(--app-border); border-radius: 8px; background: var(--app-surface); outline: none; font-family: inherit; }
.api-form-field input:focus { border-color: #c96442; }

.api-form-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 4px; }
.api-form-btn { font-size: 12.5px; font-weight: 600; padding: 6px 14px; border-radius: 8px; cursor: pointer; border: none; }
.api-form-btn.cancel { background: transparent; border: 1px solid var(--app-border); color: var(--app-text-soft); }
.api-form-btn.cancel:hover { background: var(--app-surface-3); }
.api-form-btn.save { background: #1a1a1a; color: #fff; }
.api-form-btn.save:hover { background: #333333; }

/* 流式渐变参数控件 */
.param-row { display: flex; align-items: center; gap: 12px; padding: 7px 0; }
.param-label { flex-shrink: 0; width: 96px; font-size: 12.5px; color: var(--app-text); }
.param-range { flex: 1; min-width: 0; height: 4px; accent-color: var(--app-accent); cursor: pointer; }
.param-value { flex-shrink: 0; width: 72px; text-align: right; font-size: 12px; color: var(--app-text-soft); font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; }
.param-switch { position: relative; display: inline-block; margin-left: auto; cursor: pointer; }
.param-switch input { position: absolute; opacity: 0; width: 0; height: 0; }
.param-switch-track { display: block; width: 38px; height: 22px; border-radius: 999px; background: var(--app-border); transition: background 0.15s ease; position: relative; }
.param-switch-track::after { content: ''; position: absolute; top: 2px; left: 2px; width: 18px; height: 18px; border-radius: 50%; background: var(--app-surface); box-shadow: 0 1px 2px rgba(0,0,0,0.2); transition: transform 0.15s ease; }
.param-switch input:checked + .param-switch-track { background: var(--app-accent); }
.param-switch input:checked + .param-switch-track::after { transform: translateX(16px); }
.param-reset-row { display: flex; justify-content: flex-end; margin-top: 6px; }
.param-reset-btn { padding: 4px 14px; font-size: 12px; color: var(--app-text-soft); background: var(--app-surface-3); border: 1px solid var(--app-border); border-radius: 999px; cursor: pointer; transition: background 0.15s ease; }
.param-reset-btn:hover { background: var(--app-border); }

/* 主题分段控件（仿图1 的 Small/Medium/Large 分段） */
.seg-control { margin-left: auto; display: inline-flex; background: var(--app-surface-3); border: 1px solid var(--app-border); border-radius: 8px; padding: 2px; }
.seg-btn { border: none; background: transparent; color: var(--app-text-soft); font-size: 12.5px; padding: 4px 14px; border-radius: 6px; cursor: pointer; transition: all 0.12s; }
.seg-btn.on { background: var(--app-surface); color: var(--app-text); box-shadow: 0 1px 2px rgba(0,0,0,0.08); font-weight: 600; }

/* 配色皮肤色卡网格 */
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

/* 限定皮肤卡片（按系列分组） */
.skin-groups { display: flex; flex-direction: column; gap: 12px; margin-left: auto; max-width: 460px; width: 100%; }
.skin-group-title { font-size: 11.5px; color: var(--app-text-faint); margin-bottom: 6px; letter-spacing: 0.04em; }
.skin-cards { display: flex; flex-wrap: wrap; gap: 10px; }
.skin-card {
  position: relative;
  display: inline-flex; align-items: center; gap: 10px;
  padding: 8px 12px 8px 8px; border-radius: 12px; cursor: pointer;
  background: var(--app-surface-2); border: 1.5px solid var(--app-border);
  color: var(--app-text-soft); font-size: 12.5px; transition: all 0.15s ease;
  min-width: 130px;
}
.skin-card:hover { border-color: var(--app-accent-soft); color: var(--app-text); }
.skin-card.on { border-color: var(--app-accent); color: var(--app-text); background: var(--app-accent-soft); font-weight: 600; }
.skin-card.locked { opacity: 0.72; cursor: not-allowed; }
.skin-card.locked:hover { border-color: var(--app-border); }
.skin-card-preview {
  position: relative; width: 34px; height: 34px; border-radius: 8px; overflow: hidden;
  border: 1.5px solid var(--app-border); flex-shrink: 0;
}
.skin-card-flame {
  position: absolute; bottom: -4px; left: 50%; transform: translateX(-50%);
  width: 22px; height: 22px; border-radius: 50% 50% 50% 0;
  filter: blur(6px); opacity: 0.8;
}
.skin-card-info { display: flex; flex-direction: column; align-items: flex-start; gap: 3px; }
.skin-card-label { line-height: 1; }
.skin-card-badge {
  display: inline-flex; align-items: center; gap: 3px;
  font-size: 10px; font-weight: 700; color: #d4a017;
  background: rgba(212,160,23,0.12); padding: 1px 5px; border-radius: 999px;
}
.skin-card-lock {
  position: absolute; top: 6px; right: 6px;
  color: var(--app-text-faint);
}

/* VIP 解锁提示条 */
.vip-prompt {
  display: flex; align-items: center; gap: 10px;
  margin: 10px 0 4px 108px; padding: 10px 12px;
  background: var(--app-accent-soft); border: 1px solid var(--app-accent-soft);
  border-radius: 10px; font-size: 12px; color: var(--app-text);
  max-width: 460px;
}
.vip-prompt > span { flex: 1; line-height: 1.5; }
.vip-prompt-btn {
  flex-shrink: 0; padding: 4px 10px; font-size: 11.5px; font-weight: 600;
  color: #fff; background: var(--app-accent); border: none; border-radius: 6px; cursor: pointer;
}
.vip-prompt-btn:hover { background: var(--app-accent-hover); }
.vip-prompt-close {
  flex-shrink: 0; display: flex; align-items: center; justify-content: center;
  width: 22px; height: 22px; border-radius: 5px; border: none; background: transparent;
  color: var(--app-text-soft); cursor: pointer;
}
.vip-prompt-close:hover { background: var(--app-surface-3); }

/* MCP / Skills 实体卡片 */
.entity-card { border: 1px solid var(--app-border); border-radius: 10px; padding: 11px 13px; margin-bottom: 8px; background: var(--app-surface-2); }
.entity-head { display: flex; align-items: center; gap: 8px; color: var(--app-text); }
.entity-name { font-size: 13px; font-weight: 700; }
.entity-badge { font-size: 10.5px; font-weight: 600; color: var(--app-accent); background: var(--app-accent-soft); padding: 1px 8px; border-radius: 999px; }
.entity-meta { margin-top: 5px; font-size: 11.5px; color: var(--app-text-faint); font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; line-height: 1.5; word-break: break-all; }
.entity-tags { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 8px; }
.entity-tag { font-size: 10.5px; color: var(--app-text-soft); background: var(--app-surface-3); border: 1px solid var(--app-border); border-radius: 6px; padding: 2px 7px; font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; }
.skill-steps { margin: 8px 0 2px; padding-left: 20px; font-size: 12px; color: var(--app-text-soft); line-height: 1.7; }
/* 技能来源角标：自研走中性色，外部走强调色，一眼区分 */
.entity-badge.src-learned { color: var(--app-text-soft); background: var(--app-surface-3); }
.entity-badge.src-ext { color: var(--app-accent); background: var(--app-accent-soft); }
/* 外部技能正文（SKILL.md markdown 原文，保留换行/缩进） */
.skill-body { margin: 8px 0 2px; padding: 10px 12px; font-size: 12px; line-height: 1.65; color: var(--app-text-soft); background: var(--app-surface-3); border-radius: 8px; white-space: pre-wrap; word-break: break-word; font-family: inherit; max-height: 320px; overflow-y: auto; }

/* Profile（仿图2） */
.profile-row { display: flex; align-items: center; gap: 16px; padding: 9px 0; border-bottom: 1px solid var(--app-border-soft); }
.profile-label { flex-shrink: 0; width: 180px; font-size: 13px; color: var(--app-text); }
.profile-avatar { width: 40px; height: 40px; border-radius: 50%; background: var(--app-accent); color: #fff; display: flex; align-items: center; justify-content: center; font-size: 16px; font-weight: 700; }
.profile-input { flex: 1; min-width: 0; font-size: 13px; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 8px; padding: 8px 12px; }
.profile-input:focus { outline: none; border-color: var(--app-accent); }
.profile-instructions { width: 100%; box-sizing: border-box; font-size: 13px; line-height: 1.6; color: var(--app-text); background: var(--app-surface); border: 1px solid var(--app-border); border-radius: 10px; padding: 10px 12px; resize: vertical; font-family: inherit; }
.profile-instructions:focus { outline: none; border-color: var(--app-accent); }
.profile-actions { display: flex; align-items: center; justify-content: flex-end; gap: 12px; margin-top: 14px; }
.profile-saved { font-size: 12px; color: #12b76a; }

/* 实时预览（受主题影响，无内部滚动条：内容已精简到一屏内） */
.preview-stage {
  background: var(--app-bg);
  border: 1px solid var(--app-border);
  border-radius: 10px;
  padding: 14px 16px;
  overflow: hidden;
  font-size: 14px;
  line-height: 1.75;
  color: var(--app-text);
}
.preview-bubble { word-break: break-word; }
.preview-bubble :deep(h2) { color: var(--app-text); font-size: 1.15rem; margin: 0 0 0.5em; }
.preview-bubble :deep(p) { color: var(--app-text); margin: 0.4em 0; }
.preview-bubble :deep(code) { background: var(--app-code-bg); color: var(--app-accent); padding: 1px 5px; border-radius: 4px; }
.preview-bubble :deep(table) { border-collapse: collapse; margin: 0.6em 0; font-size: 0.9em; }
.preview-bubble :deep(th), .preview-bubble :deep(td) { border: 1px solid var(--app-border); padding: 4px 10px; color: var(--app-text); }
.preview-bubble :deep(th) { background: var(--app-surface-3); }
.preview-bubble :deep(.katex) { color: var(--app-text); }
.api-form-btn.save:disabled { opacity: 0.6; cursor: default; }
</style>
