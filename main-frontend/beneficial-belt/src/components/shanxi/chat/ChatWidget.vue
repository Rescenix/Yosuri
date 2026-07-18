<template>
  <div class="chat-widget-root">
    <div class="chat-toggle-button" v-if="!isOpen" @click="toggleChat">
      <Icon icon="mdi:chat" width="28" color="white" />
    </div>

    <div class="chat-window" :class="{ expanded: isExpanded }" :style="{ display: isOpen ? 'flex' : 'none' }">

      <!-- ★ 左侧折叠菜单（侧边抽屉） —— 项目任务列表 -->
      <div v-if="menuOpen" class="drawer-backdrop" @click="menuOpen = false"></div>
      <aside class="drawer-panel" :class="{ open: menuOpen }">
        <h4 class="site-icon">
          <Icon icon="majesticons:shooting-star-line" width="20" />
        </h4>

        <a href="/blog" @click="menuOpen = false">
          <Icon icon="mdi:book-open-outline" width="16" style="margin-right:8px" />
          博客
        </a>
        <a href="/timeline" @click="menuOpen = false">
          <Icon icon="mdi:timeline-clock-outline" width="16" style="margin-right:8px" />
          更新日志
        </a>
        <a href="#" @click.prevent="openReader">
          <Icon icon="ic:outline-book" width="16" style="margin-right:8px" />
          阅读器
        </a>
        <a href="https://github.com/3478556810" target="_blank" rel="noopener" @click="menuOpen = false">
          <Icon icon="iconoir:github" width="16" style="margin-right:8px" />
          GitHub
        </a>

        <div class="drawer-divider"></div>

      </aside>

      <!-- ★ 阅读器全屏浮层：把阅读小屋嵌进聊天窗，替代独立的 /reading-hut 项目库页 -->
      <Teleport to="body">
        <div v-if="showReader" class="reader-overlay">
          <button v-if="!readerBookId" class="reader-overlay-close" @click="closeReader" aria-label="关闭阅读器">
            <Icon icon="mdi:close" width="20" color="#6b6b6b" />
          </button>
          <div class="reader-overlay-body">
            <BookShelf v-if="!readerBookId" @open="openReaderBook" />
            <ReaderView v-else :book-id="readerBookId" @close="closeReaderBook" />
          </div>
        </div>
      </Teleport>

      <!-- ★ 主内容区（随菜单推开） -->
      <div class="chat-main" :class="{ shifted: menuOpen }">

        <!-- ★★★ 合并后的一体化顶部栏 ★★★ -->
        <div class="chat-header">
          <div class="header-left">
            <button class="header-icon-btn" @click="menuOpen = !menuOpen" title="展开导航" aria-label="展开导航">
              <Icon icon="mdi:menu" width="18" color="#6b6b6b" />
            </button>
            <button
              class="header-icon-btn"
              :class="{ active: menuPinned }"
              @click.stop="toggleMenuPinned"
              @mouseenter="onSessionMenuEnter"
              @mouseleave="onSessionMenuLeave"
              title="会话与模式"
              aria-label="会话与模式"
            >
              <Icon icon="lucide:sidebar" width="18" color="#6b6b6b" />
            </button>
            <span class="header-session-name">{{ activeSessionObj?.name || '未命名会话' }}</span>
            <span class="sch-branch">{{ activeSessionObj?.branch || 'main' }}</span>
           
          </div>

          <!-- 右侧功能区（工具图标；模型切换器搬去了输入区底部工具条）
               这一整组只在 Code 模式下才有意义（终端/Diff/预览/后台任务都是代码工作流的产物），
               Chat 模式和首页状态直接不渲染，不是隐藏 -->
          <div class="header-right">
            <div class="header-tools-group" v-if="inputTopBarMode === 'git'">
              <button class="header-icon-btn" :class="{ active: dockPanels.includes('terminal') }" @click="toggleDockPanel('terminal')" title="终端">
                <Icon icon="ri:terminal-line" width="17" color="#6b6b6b" />
              </button>
              <button class="header-icon-btn" :class="{ active: dockPanels.includes('diff') }" @click="toggleDockPanel('diff')" title="Diff">
                <Icon icon="proicons:diff" width="17" color="#6b6b6b" />
              </button>
              <button class="header-icon-btn" :class="{ active: dockPanels.includes('preview') }" @click="toggleDockPanel('preview')" title="预览">
                <Icon icon="mage:preview" width="17" color="#6b6b6b" />
              </button>
              <button class="header-icon-btn" :class="{ active: showBackgroundTasks }" @click.stop="showBackgroundTasks = !showBackgroundTasks" title="后台任务">
                <Icon icon="mdi:task-minus" width="17" color="#6b6b6b" />
              </button>
              <div class="header-more-wrap">
                <button class="header-icon-btn" @click.stop="showMoreMenu = !showMoreMenu" title="更多">
                  <Icon icon="mdi:dots-vertical" width="18" color="#6b6b6b" />
                </button>
                <div v-if="showMoreMenu" class="more-menu-dropdown" @click.stop>
                  <div class="more-menu-item disabled">更多功能开发中</div>
                </div>
              </div>
            </div>
          </div>

          <!-- 悬停轻量预览：不钉住时鼠标停留在侧边栏图标上短暂展示，不占布局，
               鼠标移开即消失（不用 backdrop 拦截点击，纯浮层） -->
          <div
            v-if="menuHovering && !menuPinned"
            class="floating-menu-panel"
            @click.stop
            @mouseenter="onSessionMenuEnter"
            @mouseleave="onSessionMenuLeave"
          >
            <SessionMenuContent
              :sessions="sessionList"
              :active-session="activeSession"
              :running-session="runningSession"
              @select-session="onFloatingSelectSession"
              @new-session="onFloatingNewSession"
              @rename-session="renameSession"
              @delete-session="deleteSession"
              @open-settings="showSettings = true"
            />
          </div>
        </div>

        <!-- ★ 左侧钉住面板：点击图标钉住后是 chat-body.studio 的真实 flex 兄弟节点（占真实布局宽度，
             在顶部栏下方），跟上面 drawer-panel 那种 transform 覆盖层是两套独立机制，互不干扰 -->
        <div class="chat-body-row">
        <aside v-if="menuPinned" class="session-pin-panel" :style="{ width: sessionPinWidth + 'px' }">
          <div class="session-pin-resize-handle" @mousedown="startSessionPinDrag"></div>
          <SessionMenuContent
            fill
            :sessions="sessionList"
            :active-session="activeSession"
            :running-session="runningSession"
            @select-session="selectSession"
            @new-session="newSession"
            @rename-session="renameSession"
            @delete-session="deleteSession"
            @open-settings="showSettings = true"
          />
        </aside>

               <div class="chat-body studio">
          <!-- 共享聊天列 -->
          <div class="chat-content studio">

            <!-- 重构：将 Home 组件从 `chat-messages` 中剥离，作为 `chat-content` 的直接子节点。
                 当 `messages` 为空时，它独占整个 Flex 空间，把输入区推到最底部。 -->
            <div v-if="messages.length === 0" class="home-container-for-layout">
              <NewSessionHome />
            </div>

            <!-- 普通聊天/工作流模式：当有消息时，滚动容器才接管整个区域 -->
            <div v-else class="chat-messages" ref="messagesContainer">
              <div class="chat-messages-inner">
                <template v-for="item in groupedMessages">
                  <div v-if="item.type === 'time'" :key="`time-${item.timestamp}`" class="chat-time">
                    {{ formatChatTime(item.timestamp) }}
                  </div>
                  <div v-else-if="item.type === 'message'" :key="item.id" class="message-row" :class="item.sender">
                    <div v-if="item.type === 'image'" class="image-card">
                      <img :src="item.image" style="max-width: 240px; border-radius: 12px;" />
                    </div>
                    <div v-else-if="item.sender === 'user'" class="message-bubble user">
                      <AttachmentChipRow v-if="item.attachments?.length" :attachments="item.attachments" />
                      <div v-if="item.content">{{ item.content }}</div>
                    </div>
                    <MessageStepGroup
                      v-else-if="item.kind === 'group'"
                      :id="'group-' + item.id"
                      :group="item"
                      :ref="(el) => setGroupRef(item.id, el)"
                    />
                    <AgentWorkflowPanel
                      v-else-if="item.kind === 'agentflow'"
                      :id="'group-' + item.id"
                      :flow="item"
                    />
                    <div v-else class="assistant-message" :class="{ streaming: item.isStreaming }">
                      <div v-if="item.reasoning" class="reasoning-stream">
                        <div class="reasoning-label">
                          <Icon icon="la:atom" width="14" color="#6b7280" />
                          思考中...
                        </div>
                        <div class="reasoning-text" v-html="renderMarkdown(item.reasoning, true)"></div>
                      </div>
                      <div v-if="item.toolCallName" class="tool-call-indicator">
                        <Icon icon="mdi:cog-sync" width="14" color="#6b7280" />
                        <span>正在调用工具：{{ item.toolCallName }}</span>
                        <span v-if="item.toolCallDetail" class="tool-call-detail">{{ item.toolCallDetail }}</span>
                      </div>
                      <div class="markdown-body" v-html="renderMarkdown(item.content, true)"></div>
                      <div class="assistant-tools">
                        <button class="tool-btn" @click="playVoice(item.content)" title="朗读">
                          <Icon icon="mdi:volume-high" width="16" />
                        </button>
                        <button class="tool-btn" @click="copyText(item.content)" title="复制">
                          <Icon icon="mdi:content-copy" width="16" />
                        </button>
                      </div>
                    </div>
                  </div>
                </template>
              </div>
            </div>

            <div v-if="copiedVisible" class="copy-toast">✓ 已复制</div>

            <div class="chat-input-area">
              <!-- 回到底部：紧贴在输入框卡片正上方 -->
              <button v-show="showScrollButton" class="scroll-to-bottom-btn" @click="forceScrollToBottom" title="回到底部">
                <Icon icon="mdi:chevron-down" width="20" color="#555" />
              </button>

              <!-- 输入框上方工具栏三态切换 -->
              <div v-if="inputTopBarMode === 'dir'" class="input-dir-bar">
                <div class="toolbar-dropdown-wrap input-dir-menu-wrap">
                  <div class="input-dir-left">
                    <span class="input-dir-item">
                      <Icon icon="mdi:laptop" width="13" color="#6b6b6b" />
                      Local
                    </span>
                    <span class="input-dir-divider"></span>
                    <span class="input-dir-item input-dir-clickable" @click.stop="toggleWorkDirMenu">
                      <Icon icon="mdi:folder-outline" width="13" color="#6b6b6b" />
                      {{ currentWorkDir.name }}
                    </span>
                    <span class="input-dir-divider"></span>
                    <span class="input-dir-item">
                      <Icon icon="mdi:source-branch" width="13" color="#6b6b6b" />
                      {{ activeSessionObj?.branch || 'main' }}
                    </span>
                    <span class="input-dir-divider"></span>
                    <span class="input-dir-item input-dir-worktree">worktree</span>
                  </div>

                  <div v-if="showWorkDirMenu" class="workdir-menu-dropdown" @click.stop>
                    <template v-if="workDirMenuView === 'recent'">
                      <div class="workdir-menu-label">Recent</div>
                      <div
                        v-for="dir in workDirRecents"
                        :key="dir.path"
                        class="workdir-menu-item"
                        @click="selectWorkDir(dir)"
                      >
                        <span>{{ dir.name }}</span>
                        <Icon v-if="dir.path === currentWorkDir.path" icon="mdi:check" width="14" color="#1a1a1a" />
                      </div>
                      <div class="workdir-menu-divider"></div>
                      <div class="workdir-menu-item" @click="openFolderBrowser">
                        <span>Open folder...</span>
                      </div>
                    </template>
                    <template v-else>
                      <div class="workdir-menu-label workdir-menu-back" @click="workDirMenuView = 'recent'">
                        <Icon icon="mdi:chevron-left" width="14" /> Recent
                      </div>
                      <div v-if="workDirBrowseLoading" class="workdir-menu-item disabled">加载中…</div>
                      <template v-else-if="workDirBrowseOptions.length">
                        <div
                          v-for="dir in workDirBrowseOptions"
                          :key="dir.path"
                          class="workdir-menu-item"
                          @click="selectWorkDir(dir)"
                        >
                          <Icon icon="mdi:folder-outline" width="13" color="#6b6b6b" />
                          <span>{{ dir.name }}</span>
                        </div>
                      </template>
                      <div v-else class="workdir-menu-item disabled">未找到可选目录</div>
                    </template>
                  </div>
                </div>

                <button class="input-dir-add-btn" type="button" title="添加工作目录" @click.stop="toggleWorkDirMenu">
                  <Icon icon="mdi:plus" width="15" />
                </button>
              </div>

              <div v-else-if="inputTopBarMode === 'git' && showGitBar" class="input-git-bar">
                <div class="input-git-left">
                  <Icon icon="mdi:source-branch" width="13" color="#6b6b6b" />
                  <span class="input-git-branch">{{ gitStatus.branch || activeSessionObj?.branch || 'main' }}</span>
                </div>
                <div class="input-git-right">
                  <span class="input-git-diff-badge">
                    <span class="input-git-add">+{{ gitStatus.added }}</span>
                    <span class="input-git-remove">−{{ gitStatus.removed }}</span>
                  </span>
                  <div class="toolbar-dropdown-wrap">
                    <button class="input-git-pr-btn" type="button" @click.stop="showPrMenu = !showPrMenu">
                      Create PR
                      <span class="sch-caret">▾</span>
                    </button>
                    <div v-if="showPrMenu" class="pr-menu-dropdown" @click.stop>
                      <div class="pr-menu-item" @click="runGitAdd">
                        <Icon icon="mdi:plus-box-outline" width="14" />
                        <span>Git Add .</span>
                      </div>
                      <div class="pr-menu-item" @click="openCommitModal">
                        <Icon icon="mdi:content-save-edit-outline" width="14" />
                        <span>Git Commit</span>
                      </div>
                      <div class="pr-menu-item" @click="runGitPush">
                        <Icon icon="mdi:cloud-upload-outline" width="14" />
                        <span>Git Push</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 输入框容器：外层改列布局，附件预览条占一整行浮在文字行上方，
                   原来的横向内容（占位符/textarea/按钮）收进 .input-row 保持不变 -->
              <div class="input-wrapper" style="position: relative;">
                <!-- 粘贴图片提示 -->
                <div v-if="visionStatus" class="vision-status-toast" :class="{ error: visionStatus === 'error' }">
                  <template v-if="visionStatus === 'analyzing'">
                    <Icon icon="mdi:image-outline" width="14" color="#6b7280" /> 图片分析中...
                  </template>
                  <template v-else>
                    <Icon icon="mdi:alert-circle-outline" width="14" color="#d94834" /> {{ visionStatusMessage }}
                  </template>
                </div>

                <div v-if="showParams" class="params-panel">
                  <div class="param-row">
                    <span class="param-label">T</span>
                    <input type="range" min="0" max="2" step="0.1" v-model.number="debugTemp" @change="updateParams" />
                    <span class="param-value">{{ debugTemp }}</span>
                  </div>
                  <div class="param-row">
                    <span class="param-label">TopP</span>
                    <input type="range" min="0" max="1" step="0.05" v-model.number="debugTopP" @change="updateParams" />
                    <span class="param-value">{{ debugTopP }}</span>
                  </div>
                  <div class="param-row">
                    <span class="param-label">Tokens</span>
                    <input type="number" v-model.number="debugMaxTokens" min="100" max="8192" step="100" @change="updateParams" />
                  </div>
                  <div class="param-row">
                    <span class="param-label">思考</span>
                    <select v-model="debugReasoning" @change="updateParams">
                      <option value="">关闭</option>
                      <option value="high">开启（高）</option>
                      <option value="max">开启（最强）</option>
                    </select>
                  </div>
                </div>

                <!-- 附件预览：图片缩略图 / 文件与文件夹占位卡，横向排在输入文字上方，
                     真正的文字内容只在发送那一刻才拼进正文（buildOutgoingMessage） -->
                <AttachmentChipRow :attachments="attachments" removable @remove="removeAttachment" />

                <div class="input-row">
                  <!-- 渐变动画的浮动占位符 -->
                  <transition name="fade-placeholder" mode="out-in">
                    <span v-if="!userInput.trim() && attachments.length === 0" :key="randomPlaceholder" class="input-placeholder-text">
                      {{ randomPlaceholder }}
                    </span>
                  </transition>

                  <textarea ref="chatInputRef" class="chat-input" v-model="userInput" @keydown.enter.prevent="handleSend" @input="adjustInputHeight" @paste="handlePaste" rows="1"></textarea>

                  <!-- "+" 附加菜单用的两个隐藏原生选择器，不占布局，点菜单项时用 .click() 触发 -->
                  <input ref="attachFileInputRef" type="file" multiple style="display:none" @change="onAttachFilesSelected" @click.stop />
                  <input ref="attachFolderInputRef" type="file" webkitdirectory multiple style="display:none" @change="onAttachFolderSelected" @click.stop />

                  <button v-if="flowState.active" class="input-inner-btn input-right-btn input-stop-btn" @click="stopCodeWorkflow()" title="停止工作流（已生成内容会保留）">
                    <Icon icon="mdi:stop" width="16" color="#fff" />
                  </button>
                  <button v-else-if="(userInput.trim() || attachments.length) && !hasPendingAttachments" class="input-inner-btn input-right-btn input-send-btn" @click="handleSend">
                    <Icon icon="fluent-mdl2:up" width="18" color="#fff" />
                  </button>
                </div>
              </div>

              <!-- ========== 底部工具条（本次漏掉的部分已精准补全） ========== -->
              <div class="input-bottom-toolbar">
                <div class="input-toolbar-left">
                  <div class="toolbar-dropdown-wrap">
                    <button class="toolbar-pill-btn" @click.stop="showAutoMenu = !showAutoMenu">
                      <Icon icon="mdi:creation" width="13" />
                      <span>{{ autoMode }}</span>
                      <span class="sch-caret">▾</span>
                    </button>
                    <div v-if="showAutoMenu" class="auto-menu-dropdown" @click.stop>
                      <div
                        v-for="opt in autoModeOptions"
                        :key="opt"
                        class="auto-menu-item"
                        :class="{ active: autoMode === opt }"
                        @click="selectAutoMode(opt)"
                      >{{ opt }}</div>
                    </div>
                  </div>
                  <!-- git 工具栏开关：点亮时上方展开分支/PR 状态条，只在有会话时出现 -->
                  <button
                    v-if="inputTopBarMode === 'git'"
                    class="toolbar-icon-pill-btn"
                    @click.stop="toggleGitBar"
                    title="Git 工具栏"
                  >
                    <Icon icon="mdi:source-branch" width="15" />
                  </button>
                  <div class="toolbar-dropdown-wrap">
                    <button class="toolbar-icon-pill-btn" @click.stop="showAddMenu = !showAddMenu" title="添加">
                      <Icon icon="mdi:plus" width="16" />
                    </button>
                    <div v-if="showAddMenu" class="add-menu-dropdown" @click.stop>
                      <div class="add-menu-item" @click="triggerAttachFiles">
                        <Icon icon="mdi:paperclip" width="14" color="#6b6b6b" />
                        <span>添加文件或照片</span>
                      </div>
                      <div class="add-menu-item" @click="triggerAttachFolder">
                        <Icon icon="mdi:folder-outline" width="14" color="#6b6b6b" />
                        <span>添加文件夹</span>
                      </div>
                    </div>
                  </div>
                </div>
                <div class="input-toolbar-right">
                  <span v-if="demoMode.enabled" class="demo-badge" title="演示模式：发消息只本地渲染，不花 token">演示</span>
                  <!-- Context window 用量：常驻横条（放在模型左边，一眼可见） -->
                  <div class="context-bar-widget" @click.stop="toggleTokenPanel" title="Context window 用量">
                    <span class="ctx-bar-text">{{ formatTok(liveContextStats.used) }}/{{ formatTok(liveContextStats.contextWindow) }}</span>
                    <div class="ctx-bar-track"><div class="ctx-bar-fill" :style="{ width: liveContextStats.pct + '%' }"></div></div>
                    <span class="ctx-bar-pct">{{ liveContextStats.pct.toFixed(0) }}%</span>
                    <div v-if="showTokenPanel" class="token-usage-panel" @click.stop>
                      <div class="tup-header">
                        <span class="tup-title">上下文用量</span>
                        <span class="tup-total">~{{ formatTok(ctxTotalUsed) }}/{{ formatTok(ctxWindow) }} Tokens</span>
                      </div>
                      <div class="tup-pct">{{ ctxPct.toFixed(0) }}%</div>
                      <div class="tup-bar">
                        <div
                          v-for="r in ctxRows"
                          :key="r.key"
                          class="tup-bar-seg"
                          :style="{ width: (ctxWindow > 0 ? (r.tokens / ctxWindow) * 100 : 0) + '%', background: r.color }"
                        ></div>
                      </div>
                      <div class="tup-list">
                        <div v-for="r in ctxRows" :key="r.key" class="tup-item">
                          <span class="tup-dot" :style="{ background: r.color }"></span>
                          <span class="tup-label">{{ r.label }}</span>
                          <span class="tup-value">{{ formatTok(r.tokens) }}</span>
                        </div>
                      </div>
                    </div>
                  </div>

                  <!-- 模型切换器：不加粗、不带折叠箭头——整块本来就可点开 -->
                  <div class="sch-model" @click.stop="showModelMenu = !showModelMenu">
                    <span>{{ modelOptions.find(m => m.value === selectedModel)?.label || (hasModels ? '模型' : '无可用模型') }}</span>
                    <div v-if="showModelMenu" class="model-menu-dropdown" @click.stop>
                      <div v-if="!hasModels" class="model-menu-empty"></div>
                      <div
                        v-for="m in modelOptions"
                        :key="m.value"
                        class="model-menu-item"
                        :class="{ active: selectedModel === m.value }"
                        @click="selectModel(m.value)"
                      >
                        {{ m.label }}
                      </div>
                    </div>
                  </div>

                  <!-- 思考强度：只有当前模型确认支持 reasoning 时才出现（放在模型右边） -->
                  <div v-if="currentCapability.reasoning" class="effort-widget" @click.stop="showEffortPanel = !showEffortPanel">
                    <span class="effort-label">Effort</span>
                    <span class="effort-value">{{ effortLabel }}</span>
                    <div v-if="showEffortPanel" class="effort-panel" @click.stop>
                      <div class="effort-panel-title">
                        Effort <b>{{ modelOptions.find(m => m.value === selectedModel)?.label || '' }}</b>
                      </div>
                      <div class="effort-slider-row">
                        <span class="effort-end">Faster</span>
                        <input type="range" min="0" max="2" step="1" v-model.number="effortLevel" class="effort-slider" @click.stop @input="onEffortChange" />
                        <span class="effort-end">Smarter</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- ★ AIStudio 右：多面板停靠 -->
          <aside class="tool-panel" v-if="isExpanded && dockPanels.length" :style="{ width: dockWidth + 'px' }">
            <div class="tool-dock-resize-handle" @mousedown="startDockWidthDrag"></div>
            <template v-for="(panelKey, panelIdx) in dockPanels" :key="panelKey">
              <div class="tool-dock-pane" :style="{ flex: (dockRatios[panelKey] || 0) + ' 1 0%' }">
                <button class="tool-dock-pane-close" @click="closeDockPanel(panelKey)" title="关闭">
                  <Icon icon="mdi:close" width="14" color="#a3a3a3" />
                </button>
                <div class="tool-dock-pane-body">
                  <DiffPanel v-if="panelKey === 'diff'" />
                  <Terminal v-else-if="panelKey === 'terminal'" class="tool-panel-terminal" :open="true" :embedded="true" />
                  <PreviewBrowser v-else-if="panelKey === 'preview'" />
                </div>
              </div>
              <div
                v-if="panelIdx < dockPanels.length - 1"
                class="tool-dock-split-handle"
                @mousedown="(e) => startDockSplitDrag(panelIdx, panelIdx + 1, e)"
              ></div>
            </template>
          </aside>
        </div>
      </div>
      </div>

      <!-- 后台任务悬浮面板：作为 chat-window 的直接子元素挂载，避开 chat-main 的
           transform 造成的包含块，保证 position:fixed/absolute 能相对整个窗口定位 -->
      <BackgroundTasksPanel
        v-if="showBackgroundTasks"
        :tasks="backgroundTaskList"
        @close="showBackgroundTasks = false"
        @select-task="jumpToGroup"
      />

      <SettingsModal v-if="showSettings" @close="onSettingsClosed" />

      <div v-if="gitActionMessage" class="git-action-toast">{{ gitActionMessage }}</div>

      <!-- Git Commit 的毛玻璃浮层：居中悬浮，跟侧边栏抽屉一样挂在 chat-window 根下
           避免被内部 transform 影响定位 -->
      <div v-if="showCommitModal" class="commit-modal-backdrop" @click.self="closeCommitModal">
        <div class="commit-modal-glass">
          <div class="commit-modal-title">Commit message</div>
          <textarea
            ref="commitTextareaRef"
            v-model="commitMessage"
            class="commit-modal-textarea"
            placeholder="输入提交信息，支持多行…"
            rows="1"
            @input="adjustCommitTextareaHeight"
            @keydown.esc="closeCommitModal"
          ></textarea>
          <div class="commit-modal-actions">
            <button class="commit-modal-btn commit-modal-cancel" @click="closeCommitModal">取消</button>
            <button
              class="commit-modal-btn commit-modal-confirm"
              :disabled="!commitMessage.trim() || committing"
              @click="runGitCommit"
            >{{ committing ? '提交中…' : '确认提交' }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, computed, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import hljs from 'highlight.js'
import 'highlight.js/styles/atom-one-dark.min.css'
import 'katex/dist/katex.min.css'
import { renderMarkdown } from './markdownRenderer.js'
import { streamFadeConfig } from '../composables/streamFadeConfig.js'
import { useChatWidget } from './useChatWidget.js'
import { useResizableWidth, useResizableSplit } from './useResizable.js'
import SessionList from './SessionList.vue'
import SessionMenuContent from './SessionMenuContent.vue'
import SettingsModal from './SettingsModal.vue'
import DiffPanel from './DiffPanel.vue'
import { parseToolArgs } from './toolArgs.js'
import Terminal from './Terminal.vue'
import BackgroundTasksPanel from './BackgroundTasksPanel.vue'
import AuroraStatusIcon from './AuroraStatusIcon.vue'
import MessageStepGroup from './MessageStepGroup.vue'
import AgentWorkflowPanel from './AgentWorkflowPanel.vue'
import AttachmentChipRow from './AttachmentChipRow.vue'
import PreviewBrowser from './PreviewBrowser.vue'
import NewSessionHome from './NewSessionHome.vue'
import BookShelf from '../../reading/BookShelf.vue'
import ReaderView from '../../reading/ReaderView.vue'
import { chatModelList } from '../composables/chatModelList.js'
import { contextBreakdown, loadContextBreakdown } from '../composables/contextBreakdown.js'
import { sessionTokenStats, loadSessionTokenStats } from '../composables/sessionTokenStats.js'

const props = defineProps({
  autoOpen: { type: Boolean, default: false },
  sessionId: { type: String, default: 'global_chat_session' }
})

// ==================== 会话与数据状态 ====================
// 会话列表曾经是硬编码假数据（Aether/Prism/Nebula 占位），新增/删除/重命名只改
// 这个本地假列表，从不碰后端——聊天内容其实一直在存（sessionStore），但侧栏
// 完全反映不出来，重启后除了"当前"那一个会话，其它全部无从找起。
// 现在改成真从 /api/sessions 拉取，activeSession 直接绑定真实 sessionId（不再是
// 独立的、可能和实际加载的会话对不上的本地状态）。
const sessionList = ref([])
const activeSession = computed(() => sessionId.value)
const activeSessionObj = computed(
  () => sessionList.value.find(s => s.id === activeSession.value) || sessionList.value[0] || null
)
// 当前正在跑 agent 的会话 id：工作流活跃时就是当前选中的会话，否则为空。
// 会话列表据此在对应会话左侧点亮蓝色指示灯。
const runningSession = computed(() => (flowState.active ? activeSession.value : ''))

function shortTitle(title) {
  title = (title || '新对话').trim()
  return title.length > 24 ? title.slice(0, 24) + '…' : title
}

async function loadSessionList() {
  try {
    const res = await fetch('/api/sessions')
    const data = await res.json()
    const real = (data || []).map(s => ({ id: s.id, name: shortTitle(s.title) }))
    // 当前会话哪怕还一条消息都没有（刚新建/刚打开应用）也要出现在列表里，
    // 不然侧栏在"发第一条消息之前"会看不到自己正在哪个会话上
    if (!real.some(s => s.id === sessionId.value)) {
      real.unshift({ id: sessionId.value, name: '新对话' })
    }
    sessionList.value = real
  } catch (e) {
    console.warn('加载会话列表失败', e)
  }
}
onMounted(loadSessionList)

function selectSession(id) {
  switchSession(id)
  loadSessionList()
}
function newSession() {
  const id = 'sess_' + Date.now().toString(36)
  sessionList.value = [{ id, name: '新对话' }, ...sessionList.value]
  switchSession(id)
}
// 重命名目前只改侧栏显示，不持久化到后端——SessionStore 的标题是从首条用户消息
// 派生的，没有独立的标题字段可写；要做到重启后记得住改过的名字，需要后端加一个
// 显式的标题存储字段，这轮先不做（这轮只被明确问到"新增/删除/聊天记录"）。
function renameSession({ id, name }) {
  const target = sessionList.value.find(s => s.id === id)
  if (target) target.name = name
}
async function deleteSession(id) {
  try {
    await fetch(`/api/sessions/${id}`, { method: 'DELETE' })
  } catch (e) {
    console.warn('删除会话失败', e)
  }
  sessionList.value = sessionList.value.filter(s => s.id !== id)
  if (activeSession.value === id) {
    const next = sessionList.value[0]?.id || ('sess_' + Date.now().toString(36))
    switchSession(next)
  }
}
const toggleTokenPanel = () => {
  showTokenPanel.value = !showTokenPanel.value
}
// 仿图：上下文分类明细（6 类）。数据来自后端真实估算（字符/4，与四态机口径一致），
// 从 contextBreakdown store 读，刷新不丢。
const CTX_CATEGORIES = [
  { key: 'system',    label: '系统提示词', color: '#98a2b3' },
  { key: 'tools',     label: '工具定义',   color: '#a78bfa' },
  { key: 'skill',     label: '技能',       color: '#d97706' },
  { key: 'subagent',  label: '子代理定义', color: '#3b82f6' },
  { key: 'memory',    label: '记忆',       color: '#fb923c' },
  { key: 'conversation', label: '对话',    color: '#0f766e' },
]
const ctxRows = computed(() => {
  const cb = contextBreakdown.value
  return CTX_CATEGORIES.map(c => ({ ...c, tokens: cb[c.key] || 0 }))
})
const ctxTotalUsed = computed(() => ctxRows.value.reduce((s, r) => s + r.tokens, 0))
const ctxWindow = computed(() => contextBreakdown.value.contextWindow || 0)
const ctxPct = computed(() => ctxWindow.value > 0 ? Math.min((ctxTotalUsed.value / ctxWindow.value) * 100, 100) : 0)

// ==================== 模型能力（识图 / 上下文窗口 / 是否支持思考强度） ====================
// 免费池模型的能力元数据是静态已知的（后端 freeModelCatalog），开工前就能查到；
// DeepSeekProxy / Cloud / 自定义配置这些没有静态元数据，退回最近一次真实工作流
// 的 model_info 回填——第一次用之前不知道，用过一次就记住了。
const modelCapabilities = ref({}) // { [modelId]: {vision, context_window, reasoning} }
// 后端 catalog 的 id→显示名映射，供下拉框在只有裸 id（持久化的选择）时复原标签
const modelLabels = ref({}) // { [modelId]: 显示名 }
async function loadModelCapabilities() {
  try {
    const res = await fetch('/api/models/config')
    const data = await res.json()
    const map = {}
    const labels = {}
    for (const fm of (data.free_models || [])) {
      map[fm.id] = { vision: fm.vision, context_window: fm.context_window, reasoning: fm.reasoning }
      labels[fm.id] = fm.name
    }
    for (const c of (data.configs || [])) {
      labels[c.id] = c.name || '自定义配置'
    }
    modelCapabilities.value = map
    modelLabels.value = labels
  } catch (e) {
    console.warn('加载模型能力失败', e)
  }
}
onMounted(loadModelCapabilities)
// 刷新时从 localStorage 恢复当前会话的上下文分类占用（不丢）
loadContextBreakdown(localStorage.getItem('prism_session_id') || '')
// 刷新时恢复当前会话的真实 input/output token（不丢，横条靠它显示实际值）
sessionTokenStats.value = loadSessionTokenStats(localStorage.getItem('prism_session_id') || '')

const lastAgentFlow = computed(() => {
  for (let i = messages.value.length - 1; i >= 0; i--) {
    if (messages.value[i].kind === 'agentflow') return messages.value[i]
  }
  return null
})
const currentCapability = computed(() => {
  return modelCapabilities.value[selectedModel.value]
    || lastAgentFlow.value?.modelInfo
    || { vision: false, context_window: 0, reasoning: false }
})

// ==================== Context window 用量：优先用当前/最近一次 code 工作流的真实数据 ====================
// 旧的 tokenStats 只从 /api/chat/stream 与 /api/workflow/run 两条老路径回填，
// 四态机（Code 模式真正在用的那条）从来没接过——纯聊天/旧工作流之外，这里都是 0。
// 有 agentflow 就用它的真实 modelInfo.context_window + 本轮 token 数；没有则退回持久化的
// 会话级真实 token（sessionTokenStats，刷新不丢）；都没有才退回旧的内存 tokenStats。
const liveContextStats = computed(() => {
  const flow = lastAgentFlow.value
  if (flow && flow.modelInfo) {
    const inputTokens = flow.inputTokens || 0
    const outputTokens = flow.outputTokens || 0
    const used = inputTokens + outputTokens
    const contextWindow = flow.modelInfo.context_window || 0
    return {
      used, contextWindow, inputTokens, outputTokens,
      pct: contextWindow > 0 ? Math.min((used / contextWindow) * 100, 100) : 0
    }
  }
  // 刷新后 agentflow 不在内存：用持久化的会话 token（绑定会话、刷新不丢），
  // 不回退到恒为 0 的内存 tokenStats。
  const persisted = sessionTokenStats.value
  if (persisted && (persisted.inputTokens || persisted.outputTokens || persisted.contextWindow)) {
    const used = (persisted.inputTokens || 0) + (persisted.outputTokens || 0)
    const contextWindow = persisted.contextWindow || 0
    return {
      used, contextWindow,
      inputTokens: persisted.inputTokens || 0,
      outputTokens: persisted.outputTokens || 0,
      pct: contextWindow > 0 ? Math.min((used / contextWindow) * 100, 100) : (persisted.contextPct || 0)
    }
  }
  return {
    used: tokenStats.inputTokens + tokenStats.outputTokens,
    contextWindow: tokenStats.contextWindow,
    inputTokens: tokenStats.inputTokens,
    outputTokens: tokenStats.outputTokens,
    pct: Math.min(tokenStats.contextPct || 0, 100)
  }
})

// ==================== 右侧工具面板（多面板停靠） ====================
const dockPanels = ref([])
const dockWidth = ref(380)
const { startDrag: startDockWidthDrag } = useResizableWidth(dockWidth, { min: 300, max: 720, edge: 'left', persistKey: 'dockWidth' })
const { ratios: dockRatios, startDragBetween: startDockSplitDrag } = useResizableSplit(dockPanels, { min: 0.15 })

const DOCK_PANEL_LABELS = { diff: 'Diff', terminal: '终端', preview: '预览' }
function dockPanelLabel(key) { return DOCK_PANEL_LABELS[key] || key }
function toggleDockPanel(key) {
  const idx = dockPanels.value.indexOf(key)
  dockPanels.value = idx === -1 ? [...dockPanels.value, key] : dockPanels.value.filter(k => k !== key)
}
function closeDockPanel(key) {
  dockPanels.value = dockPanels.value.filter(k => k !== key)
}

// Diff 面板已改为 git 工作树全量 diff（DiffPanel 自己拉 /api/git/working-diff），
// 不再从会话工具调用里拼 before/after

// ==================== 工作目录切换：Recent + Open folder ====================
// "文件夹" 是当前 monorepo（GitRepoRoot）下的真实子目录，复用已有的 /api/file-tree
// 拿顶层目录列表，不新开接口。localStorage 只是 UI 层的即时展示缓存——真正的持久化
// 和"agent 记不记得"由后端 /api/workdir 负责（落盘到 ~/shanxi_data/workdir.txt，
// 所有 read_file/write_file/edit_file/execute_command 立刻切到新目录），
// 不调这个接口的话，选目录就只是好看，agent 该读哪还是读哪，等于没切
const WORKDIR_STORAGE_KEY = 'aether_workdir_state_v1'
const WORKDIR_IGNORED = new Set(['node_modules', 'build', '__pycache__', 'dist', '.git'])
const currentWorkDir = ref({ name: 'main-frontend', path: 'main-frontend' })
const workDirRecents = ref([{ name: 'main-frontend', path: 'main-frontend' }])
const showWorkDirMenu = ref(false)
const workDirMenuView = ref('recent') // 'recent' | 'browse'
const workDirBrowseOptions = ref([])
const workDirBrowseLoading = ref(false)
const workDirSwitching = ref(false)

function loadWorkDirState() {
  try {
    const raw = localStorage.getItem(WORKDIR_STORAGE_KEY)
    if (!raw) return
    const data = JSON.parse(raw)
    if (data.current?.name) currentWorkDir.value = data.current
    if (Array.isArray(data.recents) && data.recents.length) workDirRecents.value = data.recents
  } catch (e) {}
}
function saveWorkDirState() {
  try {
    localStorage.setItem(WORKDIR_STORAGE_KEY, JSON.stringify({ current: currentWorkDir.value, recents: workDirRecents.value }))
  } catch (e) {}
}
// 挂载时用后端真实值校准——localStorage 只是缓存，后端 workdir.txt 才是权威来源
// （比如换了台机器、或者上次没走前端直接调了接口，localStorage 会跟真实值不一致）
async function syncWorkDirFromBackend() {
  try {
    const res = await fetch('/api/workdir')
    if (!res.ok) return
    const data = await res.json()
    if (!data.path) return
    const dir = { name: data.name || data.path, path: data.path }
    currentWorkDir.value = dir
    workDirRecents.value = [dir, ...workDirRecents.value.filter(d => d.path !== dir.path)].slice(0, 6)
    saveWorkDirState()
  } catch (e) {}
}
function toggleWorkDirMenu() {
  showWorkDirMenu.value = !showWorkDirMenu.value
  if (showWorkDirMenu.value) workDirMenuView.value = 'recent'
}
async function selectWorkDir(dir) {
  if (workDirSwitching.value) return
  workDirSwitching.value = true
  try {
    const res = await fetch('/api/workdir', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: dir.path })
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || `切换失败 (${res.status})`)
    }
    const data = await res.json()
    const resolved = { name: data.name || dir.name, path: data.path || dir.path }
    currentWorkDir.value = resolved
    // 去重后塞到最前面，最多保留 6 条最近记录
    workDirRecents.value = [resolved, ...workDirRecents.value.filter(d => d.path !== resolved.path)].slice(0, 6)
    saveWorkDirState()
    showWorkDirMenu.value = false
    showGitToast(`已切换工作目录: ${resolved.name}`)
  } catch (e) {
    showGitToast(e.message || '切换工作目录失败')
  } finally {
    workDirSwitching.value = false
  }
}
async function openFolderBrowser() {
  workDirMenuView.value = 'browse'
  workDirBrowseLoading.value = true
  try {
    const res = await fetch('/api/file-tree')
    if (!res.ok) throw new Error('拉取目录失败')
    const tree = await res.json()
    workDirBrowseOptions.value = (tree || [])
      .filter(n => n.type === 'folder' && !n.name.startsWith('.') && !WORKDIR_IGNORED.has(n.name))
      .map(n => ({ name: n.name, path: n.path || n.name }))
  } catch (e) {
    workDirBrowseOptions.value = []
  } finally {
    workDirBrowseLoading.value = false
  }
}

// ==================== Git 状态条 + PR 面板（Add/Commit/Push） ====================
// 复用后端已有的 /api/git-status、/api/git/add-all、/api/git/commit、/api/git/push，
// 不新增接口——面板上的分支名、+N/-N 都是这里拉回来的真实数据，不再是写死的假值
const gitStatus = ref({ branch: '', added: 0, removed: 0 })
async function fetchGitStatus() {
  try {
    const res = await fetch('/api/git-status')
    if (!res.ok) return
    const data = await res.json()
    // 兼容后端还没重启、旧二进制不返回 added/removed 字段的情况，避免面板上显示 "+undefined"
    gitStatus.value = { branch: '', added: 0, removed: 0, ...data }
  } catch (e) {}
}

const showPrMenu = ref(false)
const gitActionMessage = ref('')
let gitToastTimer = null
function showGitToast(msg) {
  gitActionMessage.value = msg
  clearTimeout(gitToastTimer)
  gitToastTimer = setTimeout(() => { gitActionMessage.value = '' }, 2500)
}

async function runGitAdd() {
  showPrMenu.value = false
  try {
    const res = await fetch('/api/git/add-all', { method: 'POST' })
    if (!res.ok) throw new Error(await res.text())
    showGitToast('已执行 git add .')
    await fetchGitStatus()
  } catch (e) { showGitToast('git add 失败') }
}

async function runGitPush() {
  showPrMenu.value = false
  showGitToast('推送中…')
  try {
    const res = await fetch('/api/git/push', { method: 'POST' })
    const text = await res.text()
    if (!res.ok) throw new Error(text)
    showGitToast('推送成功')
  } catch (e) { showGitToast('推送失败，详见控制台'); console.error(e) }
}

const showCommitModal = ref(false)
const commitMessage = ref('')
const committing = ref(false)
const commitTextareaRef = ref(null)

function openCommitModal() {
  showPrMenu.value = false
  commitMessage.value = ''
  showCommitModal.value = true
  nextTick(() => { commitTextareaRef.value?.focus(); adjustCommitTextareaHeight() })
}
function closeCommitModal() {
  if (committing.value) return
  showCommitModal.value = false
}
// 跟主输入框的 adjustInputHeight 同一套手法：先回弹 auto 量出真实内容高度，
// 再赋值成 scrollHeight，撑开容器而不是在 textarea 内部出滚动条
function adjustCommitTextareaHeight() {
  const el = commitTextareaRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = el.scrollHeight + 'px'
}
async function runGitCommit() {
  if (!commitMessage.value.trim() || committing.value) return
  committing.value = true
  try {
    const res = await fetch('/api/git/commit', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: commitMessage.value })
    })
    if (!res.ok) throw new Error(await res.text())
    showCommitModal.value = false
    commitMessage.value = ''
    showGitToast('提交成功')
    await fetchGitStatus()
  } catch (e) {
    showGitToast('提交失败，详见控制台')
    console.error(e)
  } finally {
    committing.value = false
  }
}

// 输入框上方工具栏两态：首页/新会话(无消息)显示工作目录条；一旦开始对话
// 就进入 git 态——但 git 状态条不再默认铺开，改成由底部工具栏的 git 按钮
// 手动开关（showGitBar），默认收起，需要看分支/提交时才点开
const inputTopBarMode = computed(() => {
  if (messages.value.length === 0) return 'dir'
  return 'git'
})
// git 状态条的显隐开关，默认收起
const showGitBar = ref(false)
function toggleGitBar() {
  showGitBar.value = !showGitBar.value
  if (showGitBar.value) fetchGitStatus()
}

// ==================== 项目数据 ====================
// 定义占位符池子（老王主题风格）
const placeholders = [
  "今天我们要创造什么？",
  "请告诉我你的想法，我会帮你实现",
  "有什么问题需要解答吗？",
  "我可以帮你写代码、写文章、做计划……",

]

const randomPlaceholder = ref("输入你的问题...")

onMounted(() => {
  // 从数组中随机取一条
  const randomIndex = Math.floor(Math.random() * placeholders.length)
  randomPlaceholder.value = placeholders[randomIndex]
  // 2. 每隔 6 秒自动轮换一次
  setInterval(() => {
    const nextIndex = Math.floor(Math.random() * placeholders.length)
    randomPlaceholder.value = placeholders[nextIndex]
  }, 6000) // 60000毫秒 = 60秒
})

// ==================== 工具函数 ====================
function cleanContent(content) { return content ? content.replace(/\[(action|emotion):[^\]]*\]/g, '') : '' }

const copiedVisible = ref(false)
async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text)
    copiedVisible.value = true; setTimeout(() => { copiedVisible.value = false }, 2000)
  } catch (err) {
    const textarea = document.createElement('textarea')
    textarea.value = text; textarea.style.position = 'fixed'; textarea.style.opacity = '0'
    document.body.appendChild(textarea); textarea.select(); document.execCommand('copy'); document.body.removeChild(textarea)
    copiedVisible.value = true; setTimeout(() => { copiedVisible.value = false }, 2000)
  }
}

// ==================== 模型选择 ====================
// 下拉只显示用户在设置面板「选为可用」加入的模型——读共享的 chatModelList store，
// 这是 SettingsModal.toggleVendorModels 写入的权威列表。设置里没勾选 → 这里就是空。
// modelLabels 仅用于把持久化的裸 id 还原成显示名（后端返回 name）。
const selectedModel = ref(localStorage.getItem('selectedModel') || '')
// 列表为空（用户设置里没选任何模型）时，下拉无选项；选中项若不在真实列表里则定位到第一个。
// 只用后端真实存在的 id 定位，跳过 localStorage 残留的幽灵 id（如 'cloud'/480B）。
watch(chatModelList, (list) => {
  const ids = (list || []).filter(m => m.value in modelLabels.value).map(m => m.value)
  if (ids.length === 0) return
  if (!ids.includes(selectedModel.value)) {
    selectedModel.value = ids[0]
    localStorage.setItem('selectedModel', ids[0])
  }
}, { deep: true })
const modelOptions = computed(() => {
  // 仅展示用户在设置面板选为可用的模型，且必须是后端真实存在的（modelLabels 有记录）。
  // 过滤掉 localStorage 残留的幽灵 id（如过期的 'cloud'/480B），它们不在后端 freeModelCatalog 里。
  return chatModelList.value
    .filter(m => m.value in modelLabels.value)
    .map(m => ({
      label: modelLabels.value[m.value] || m.label || m.value,
      value: m.value
    }))
})
const hasModels = computed(() => modelOptions.value.length > 0)
const showModelMenu = ref(false)
function selectModel(value) { selectedModel.value = value; localStorage.setItem('selectedModel', value); showModelMenu.value = false }

// ==================== 设置面板 ====================
const showSettings = ref(false)
function onSettingsClosed() {
  showSettings.value = false
  // 设置面板改动（填 key/加配置）后重载后端模型列表，下拉立即反映真实模型，无需刷新页面。
  loadModelCapabilities()
}

// ==================== 底部工具条：Auto 模式 + "+" 附加菜单 ====================
const autoModeOptions = ['Auto', 'Ask', 'Plan']
const autoMode = ref('Auto')
const showAutoMenu = ref(false)
const showAddMenu = ref(false)
function selectAutoMode(opt) { autoMode.value = opt; showAutoMenu.value = false }

// ==================== Markdown 渲染 ====================
// renderMarkdown 挪进了 markdownRenderer.js，跟 MessageStepGroup 共用同一套
// markdown-it + katex 管线——之前 code 模式的 step 卡片没走这条管线，公式/代码块/
// markdown 语法全部裸奔成纯文本
function highlightAllCodeBlocks() {
  requestAnimationFrame(() => {
    document.querySelectorAll('.chat-messages .markdown-body pre').forEach(pre => {
      const code = pre.querySelector('code')
      if (!code) return
      const classList = [...code.classList]
      const langClass = classList.find(c => c.startsWith('language-'))
      const lang = langClass ? langClass.replace('language-', '') : 'text'
      pre.setAttribute('data-lang', lang)
      hljs.highlightElement(code)
      if (!pre.querySelector('.code-btn-group')) {
        const btnGroup = document.createElement('div')
        btnGroup.className = 'code-btn-group'
        const copyBtn = document.createElement('button')
        copyBtn.className = 'copy-code-btn'
        copyBtn.textContent = '复制'
        copyBtn.onclick = async () => {
          const success = await copyText(code.textContent || '')
          if (success) { copyBtn.textContent = '已复制'; setTimeout(() => { copyBtn.textContent = '复制' }, 2000) }
        }
        btnGroup.appendChild(copyBtn)
        pre.appendChild(btnGroup)
      }
    })
  })
}

// ==================== AI 消息流式瀑布渐变 ====================
// 仿主流 AI（ChatGPT/Gemini）流式输出：新到的字符按先后顺序级联淡入
// （透明度 0→1 + 轻微 blur 消散），形成"瀑布"式的渐变尾巴。
// 难点：正文是 v-html 整段重渲染的，每个 chunk 都会把上一轮包的 span 冲掉。
// 解法：为每个消息元素记录"已见文本长度 + 各批次到达时间"，每次重渲染后
// 重新包 span，并用负的 animation-delay 恢复各字符已播进度，视觉上无缝。
// 参数集中在 ../composables/streamFadeConfig.js（reactive + localStorage 持久化），
// 设置面板直接读写 streamFadeConfig 即可。
const STREAM_SEG_CHARS = 2 // 每个 span 包几个字符（性能/细腻度折中）
const streamFadeState = new WeakMap() // el -> { len, pending: [{start, bornAt}] }

function collectStreamTextNodes(root) {
  const nodes = []
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode(n) {
      const p = n.parentElement
      // 代码块/公式/表格由 hljs/katex/markdown 接管 DOM，不要往里插 span——
      // 否则表格边吐边重排列宽会抖、代码块随每次 chunk 整段重渲染会变慢。
      // 这类整块直接跳出现（仿 ChatGPT），只让思考块与普通正文段落保留瀑布级联。
      if (p && p.closest('pre, code, table, .katex, .code-btn-group')) return NodeFilter.FILTER_REJECT
      return NodeFilter.FILTER_ACCEPT
    }
  })
  let n
  while ((n = walker.nextNode())) nodes.push(n)
  return nodes
}

function applyStreamFade(el) {
  const { fadeMs, staggerMs, maxSweepMs, blurPx } = streamFadeConfig
  let st = streamFadeState.get(el)
  if (!st) { st = { len: 0, pending: [] }; streamFadeState.set(el, st) }
  const now = performance.now()
  const nodes = collectStreamTextNodes(el)
  const total = nodes.reduce((s, n) => s + n.nodeValue.length, 0)
  if (total < st.len) { st.len = total; st.pending = []; return } // 切会话/markdown 回缩，重置
  if (total > st.len) { st.pending.push({ start: st.len, bornAt: now }); st.len = total }
  // 每批的实际级联间隔：字符太多时压缩，保证 MAX_SWEEP 内铺完
  let ranges = st.pending.map((r, i) => ({ ...r, end: st.pending[i + 1]?.start ?? st.len }))
  ranges = ranges.filter(r => {
    const stag = Math.min(staggerMs, maxSweepMs / Math.max(1, r.end - r.start))
    return now - r.bornAt < (r.end - r.start) * stag + fadeMs
  })
  st.pending = ranges.map(r => ({ start: r.start, bornAt: r.bornAt }))
  if (!ranges.length) return
  const fadeFrom = ranges[0].start
  let offset = 0
  for (const node of nodes) {
    const nodeStart = offset
    const text = node.nodeValue
    offset += text.length
    if (offset <= fadeFrom) continue
    // 上一轮已包好的 span，动画还在跑，别动它
    if (node.parentElement && node.parentElement.closest('.stream-fade-seg')) continue
    const frag = document.createDocumentFragment()
    const plainEnd = Math.max(0, fadeFrom - nodeStart)
    if (plainEnd > 0) frag.appendChild(document.createTextNode(text.slice(0, plainEnd)))
    for (let i = plainEnd; i < text.length; i += STREAM_SEG_CHARS) {
      const seg = text.slice(i, i + STREAM_SEG_CHARS)
      const pos = nodeStart + i
      let range = ranges[0]
      for (const r of ranges) { if (r.start <= pos) range = r; else break }
      const stag = Math.min(staggerMs, maxSweepMs / Math.max(1, range.end - range.start))
      const delay = (pos - range.start) * stag - (now - range.bornAt)
      if (delay <= -fadeMs) { frag.appendChild(document.createTextNode(seg)); continue }
      const span = document.createElement('span')
      span.className = 'stream-fade-seg'
      span.style.animationDuration = fadeMs + 'ms'
      span.style.animationDelay = delay.toFixed(1) + 'ms'
      span.style.setProperty('--sf-blur', blurPx + 'px')
      span.textContent = seg
      // 动画一跑完就把 span 拆回纯文本节点：否则成千上万个带 will-change 的 span
      // 会永久堆在已完成的消息里，滚动时全量重合成 → 果冻抖动。拆回后零图层零开销。
      span.addEventListener('animationend', () => {
        const p = span.parentNode
        if (p) p.replaceChild(document.createTextNode(span.textContent), span)
      }, { once: true })
      frag.appendChild(span)
    }
    node.parentNode.replaceChild(frag, node)
  }
}

function streamFadePass() {
  if (!streamFadeConfig.enabled) return
  // 只处理带 .streaming 的助手消息（isStreaming=true，即正在 SSE 输出的那条）。
  // 历史消息（切会话加载、刷新恢复）一律不做渐变：既没必要，还会因为整段包 span
  // 让含表格的消息反复触发列宽重算——就是"切会话时表格抖动"的来源。
  // 主聊天现已走四态机 agentflow（/api/code/workflow），回答渲染在
  // AgentWorkflowPanel 的 .agent-flow 里：意图块 .flow-intent.markdown-body、
  // 思考块 .flow-thinking-text。它们没有 .assistant-message.streaming 外层，
  // 故原选择器命中不了——补充命中，并用 .agent-flow.streaming（running 时挂）
  // 作为"正在流式"的标识，让瀑布渐变接到主链路。
  document.querySelectorAll(
    '.chat-messages .assistant-message.streaming .markdown-body, ' +
    '.chat-messages .assistant-message.streaming .reasoning-text, ' +
    '.agent-flow.streaming .flow-intent.markdown-body, ' +
    '.agent-flow.streaming .flow-thinking-text'
  ).forEach(applyStreamFade)
}

// ==================== useChatWidget ====================
const {
  isOpen, isExpanded, userInput, messages, sessionId,
  isLoggedIn, debugTemp, debugTopP, debugReasoning, lastTokenUsage, lastLatency, debugMaxTokens, balance,
  currentStatus, statusDotColor,
  messagesContainer, chatInputRef, userScrolledUp,
  forceScrollToBottom, adjustInputHeight, switchSession,
  sendMessage, sendWorkflow, stopWorkflow, workflowState, tokenStats, chatState, backgroundTaskList, handleImageUpload, playVoice,
  flowState, startCodeWorkflow, stopCodeWorkflow,
  demoMode, startDemoFlow,
  toggleChat, updateParams,
  groupedMessages, formatChatTime
} = useChatWidget(props, { renderMarkdown })

// ==================== 思考强度（Effort）：Faster(low) ↔ Smarter(high) ====================
// 注意：debugReasoning 来自上面的 useChatWidget 解构，本段必须放在解构之后，
// 否则 setup 阶段会命中暂时性死区（TDZ）报 "Cannot access before initialization"。
const EFFORT_LEVELS = ['low', 'medium', 'high']
const EFFORT_UI_LABELS = { low: 'Faster', medium: 'Balanced', high: 'Smarter' }
const showEffortPanel = ref(false)
const initialEffortIdx = EFFORT_LEVELS.indexOf(debugReasoning.value)
const effortLevel = ref(initialEffortIdx >= 0 ? initialEffortIdx : 1)
const effortLabel = computed(() => EFFORT_UI_LABELS[EFFORT_LEVELS[effortLevel.value]])
function onEffortChange() {
  debugReasoning.value = EFFORT_LEVELS[effortLevel.value]
  localStorage.setItem('debugReasoning', debugReasoning.value)
}
if (!debugReasoning.value) onEffortChange() // 首次没设置过时落一个默认值，跟滑块初始位置对齐

// ==================== UI 状态 ====================
const menuOpen = ref(false)
const showParams = ref(false)
const showMoreMenu = ref(false)
const showBackgroundTasks = ref(false)

// ==================== 阅读器全屏浮层 ====================
// 把阅读小屋（BookShelf + ReaderView）嵌进聊天窗，替代独立的 /reading-hut 项目库页。
// 组件自包含（自己拉书架/书本），零 props 改写即可内嵌。
const showReader = ref(false)
const readerBookId = ref('')
function openReader() {
  showReader.value = true
  readerBookId.value = ''
  menuOpen.value = false
}
function closeReader() {
  showReader.value = false
  readerBookId.value = ''
}
function openReaderBook(book) {
  readerBookId.value = book.id
}
function closeReaderBook() {
  readerBookId.value = ''
}

// ==================== 左侧会话/模式面板：钉住 vs 悬停轻量预览 ====================
const menuPinned = ref(false)
const menuHovering = ref(false)
let menuHoverCloseTimer = null
function onSessionMenuEnter() {
  if (menuPinned.value) return
  clearTimeout(menuHoverCloseTimer)
  menuHovering.value = true
}
function onSessionMenuLeave() {
  if (menuPinned.value) return
  // 留一点缓冲时间，避免鼠标从图标移到预览面板中途正好穿过图标外的空隙时闪烁关闭
  menuHoverCloseTimer = setTimeout(() => { menuHovering.value = false }, 150)
}
function toggleMenuPinned() {
  menuPinned.value = !menuPinned.value
  if (menuPinned.value) menuHovering.value = false
}
const sessionPinWidth = ref(300)
const { startDrag: startSessionPinDrag } = useResizableWidth(sessionPinWidth, { min: 240, max: 480, edge: 'right', persistKey: 'sessionPinWidth' })

// 消息流里每个 kind:'group' 的组件实例，供后台任务清单点击跳转+展开用
const groupRefs = {}
function setGroupRef(id, el) {
  if (el) groupRefs[id] = el
}
function jumpToGroup(id) {
  showBackgroundTasks.value = false
  nextTick(() => {
    groupRefs[id]?.expand()
    document.getElementById('group-' + id)?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  })
}

// ==================== 发送：聊天/代码只有一条链路了 ====================
// 之前 Chat/Code 是两个模式两条路——Chat 走轻量流式，Code 走四态机能调工具。
// 合并成一条：永远走四态机（startCodeWorkflow），模型自己判断要不要调工具，
// 不需要工具时就是普通对话回复，agent 两件事都能干，用户不用先选模式。
function handleSend() {
  if (hasPendingAttachments.value) return
  const combined = buildOutgoingMessage()
  if (!combined) return
  const displayText = userInput.value.trim()
  // 演示模式：零 token 本地沙盒，直接渲染预置长对话验收瀑布渐变，不触网
  if (demoMode.enabled) {
    userInput.value = ''
    nextTick(() => { if (chatInputRef.value) chatInputRef.value.style.height = 'auto' })
    startDemoFlow(displayText)
    return
  }
  const displayAttachments = attachments.value.filter(a => a.status === 'ready').map(a => ({ ...a }))
  clearAttachments()
  userInput.value = ''
  // 发送后内容必空，直接把高度交回 CSS（min-height:40px 兜底成单行），
  // 不依赖 adjustInputHeight 的 scrollHeight 测量——它会在 v-model 未同步时量到旧高度而卡两行
  nextTick(() => { if (chatInputRef.value) chatInputRef.value.style.height = 'auto' })
  startCodeWorkflow(combined, { text: displayText, attachments: displayAttachments })
}
function onFloatingSelectSession(id) { selectSession(id); menuHovering.value = false }
function onFloatingNewSession() { newSession(); menuHovering.value = false }

// Token 环状进度条
const showTokenPanel = ref(false)
function formatTok(n) {
  n = n || 0
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k'
  return String(n)
}

// ==================== 图片粘贴 ====================
const visionStatus = ref('')
const visionStatusMessage = ref('')
let visionStatusTimer = null
function showVisionError(msg) {
  visionStatus.value = 'error'; visionStatusMessage.value = msg
  clearTimeout(visionStatusTimer); visionStatusTimer = setTimeout(() => { visionStatus.value = '' }, 3000)
}
function fileToBase64(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => { const result = String(reader.result || ''); resolve(result.split(',')[1] || '') }
    reader.onerror = () => reject(new Error('图片读取失败'))
    reader.readAsDataURL(file)
  })
}
// 粘贴图片跟"+"菜单选图走同一条路：先当附件加进 attachments（复用 attachImageFile
// 的分析逻辑），分析完直接自动发送——保留"粘贴即发"的原有习惯，但气泡里现在是
// 一张缩略图 chip + 用户自己敲的话，不再是整段 Gemini 分析原文糊在消息正文里
async function handlePaste(e) {
  const items = e.clipboardData?.items
  if (!items) return
  let imageFile = null
  for (const item of items) {
    if (item.type && item.type.startsWith('image/')) { imageFile = item.getAsFile(); break }
  }
  if (!imageFile) return
  e.preventDefault()
  if (flowState.active) { showVisionError('工作流运行中，请稍后再粘贴图片'); return }
  await attachImageFile(imageFile)
  handleSend()
}

// ==================== "+" 附加菜单：添加文件/照片、添加文件夹 ====================
// 跟粘贴图片（handlePaste）共用同一套 vision-preprocess 接口和状态提示，但这里是
// "先附加、用户自己决定何时发送"，不像粘贴那样识别完直接 sendWorkflow
const attachFileInputRef = ref(null)
const attachFolderInputRef = ref(null)

function triggerAttachFiles() { showAddMenu.value = false; attachFileInputRef.value?.click() }
function triggerAttachFolder() { showAddMenu.value = false; attachFolderInputRef.value?.click() }

// 附件不再直接怼进输入框文字里——改成跟 ChatGPT/Claude 一样，在输入框上方
// 显示一排预览 chip（图片缩略图 / 文件占位卡），真正的文字内容只在发送那一刻
// 才拼进消息正文，见 buildOutgoingMessage()
const attachments = ref([])
let attachmentSeq = 0

function extOf(name) {
  const m = /\.([a-zA-Z0-9]+)$/.exec(name || '')
  return m ? m[1].toUpperCase() : 'FILE'
}
function removeAttachment(id) {
  const idx = attachments.value.findIndex(a => a.id === id)
  if (idx === -1) return
  const [removed] = attachments.value.splice(idx, 1)
  if (removed.previewUrl) URL.revokeObjectURL(removed.previewUrl)
}
function clearAttachments() {
  for (const a of attachments.value) { if (a.previewUrl) URL.revokeObjectURL(a.previewUrl) }
  attachments.value = []
}
const hasPendingAttachments = computed(() => attachments.value.some(a => a.status === 'analyzing'))

async function attachImageFile(file) {
  const id = ++attachmentSeq
  const previewUrl = URL.createObjectURL(file)
  attachments.value.push({ id, kind: 'image', name: file.name, status: 'analyzing', previewUrl })
  try {
    const base64 = await fileToBase64(file)
    const res = await fetch('/api/aether/vision-preprocess', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ image_base64: base64, mime_type: file.type || 'image/png' })
    })
    if (!res.ok) throw new Error(`请求失败 (${res.status})`)
    const data = await res.json()
    if (!data.text) throw new Error('未返回分析文本')
    const item = attachments.value.find(a => a.id === id)
    if (item) { item.status = 'ready'; item.analysisText = data.text }
  } catch (err) {
    const item = attachments.value.find(a => a.id === id)
    if (item) { item.status = 'error'; item.errorMsg = '分析失败' }
  }
}

async function attachTextFile(file) {
  const id = ++attachmentSeq
  attachments.value.push({ id, kind: 'file', name: file.name, ext: extOf(file.name), status: 'analyzing' })
  try {
    const text = await file.text()
    const truncated = text.length > 4000 ? text.slice(0, 4000) + '\n…（已截断）' : text
    const item = attachments.value.find(a => a.id === id)
    if (item) { item.status = 'ready'; item.content = truncated }
  } catch (err) {
    const item = attachments.value.find(a => a.id === id)
    if (item) { item.status = 'error'; item.errorMsg = '读取失败' }
  }
}

function onAttachFilesSelected(e) {
  const files = Array.from(e.target.files || [])
  e.target.value = ''
  for (const file of files) {
    if (file.type && file.type.startsWith('image/')) attachImageFile(file)
    else attachTextFile(file)
  }
}

// 文件夹选择拿到的是扁平文件列表（每个文件带 webkitRelativePath），浏览器不允许
// 直接读目录结构——先给个清单让模型知道有哪些文件，需要看内容再走 read_file 工具
function onAttachFolderSelected(e) {
  const files = Array.from(e.target.files || [])
  e.target.value = ''
  if (files.length === 0) return
  const folderName = files[0].webkitRelativePath?.split('/')[0] || '未命名文件夹'
  const list = files.slice(0, 200).map(f => f.webkitRelativePath).join('\n')
  const more = files.length > 200 ? `\n…（共 ${files.length} 个文件，已截断显示）` : ''
  attachments.value.push({
    id: ++attachmentSeq, kind: 'folder', name: folderName, status: 'ready',
    fileCount: files.length, manifest: list + more
  })
}

// 发送那一刻才把附件序列化进正文：图片用 vision 分析结果、文本文件用代码块、
// 文件夹用清单——顺序固定放在用户自己敲的文字前面，读起来像"这是材料，这是我的问题"
function buildOutgoingMessage() {
  const blocks = attachments.value
    .filter(a => a.status === 'ready')
    .map(a => {
      if (a.kind === 'image') return `[图片: ${a.name}]\n${a.analysisText || ''}`
      if (a.kind === 'folder') return `[文件夹: ${a.name}，共 ${a.fileCount} 个文件]\n${a.manifest}`
      return `[文件: ${a.name}]\n\`\`\`\n${a.content || ''}\n\`\`\``
    })
  const typed = userInput.value.trim()
  return [...blocks, typed].filter(Boolean).join('\n')
}

const showScrollButton = computed(() => { return isOpen.value && userScrolledUp.value })

watch(messages, () => { nextTick(() => { streamFadePass(); highlightAllCodeBlocks() }) }, { deep: true })
// 切进 git 状态条可见的 Code 模式时刷新一次，避免面板上的 +N/-N 停留在挂载时的旧快照
watch(inputTopBarMode, (mode) => { if (mode === 'git') fetchGitStatus() })
// 工作流（四态机）结束时，停止按钮消失，立刻把输入框高度塌回单行——
// 直接交回 CSS（auto + min-height 兜底），不靠 scrollHeight 测量
watch(() => flowState.active, (active, wasActive) => {
  if (wasActive && !active) nextTick(() => { if (chatInputRef.value) chatInputRef.value.style.height = 'auto' })
})

// ==================== 初始化 ====================
onMounted(() => {
  fetchGitStatus()
  loadWorkDirState()
  syncWorkDirFromBackend()
  document.addEventListener('click', () => {
    showModelMenu.value = false; showTokenPanel.value = false; menuHovering.value = false; showMoreMenu.value = false
    showAutoMenu.value = false; showAddMenu.value = false; showPrMenu.value = false; showWorkDirMenu.value = false
  })

})
</script>

<style scoped>
@import '../../../styles/shanxi/chat-window.css';

/* 加固：.chat-window 本身是 position:fixed，理论上不会撑高这个根节点，
   但显式约束一下成本很低，避免任何万一 */
.chat-widget-root { height: 100%; overflow: hidden; }

/* 自适应占位符的绝对定位 */
.input-placeholder-text {
  position: absolute;
  left: 16px;             /* 和 input-wrapper 的 padding-left 保持一致 */
  top: 10px;              /* 和 textarea 的 padding-top 保持一致 */
  pointer-events: none;   /* 确保鼠标点击能直接穿透进 textarea */
  color: #a9a9a9;
  font-size: 15px;
  font-family: inherit;
  z-index: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 80%;         /* 防止占位符太长时覆盖到右侧的按钮 */
}

/* 过渡动画的核心 */
.fade-placeholder-enter-active,
.fade-placeholder-leave-active {
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.fade-placeholder-enter-from,
.fade-placeholder-leave-to {
  opacity: 0;
  transform: translateY(-6px); /* 旧文字向上淡出，新文字向下淡入 */
}

.fade-placeholder-enter-to,
.fade-placeholder-leave-from {
  opacity: 1;
  transform: translateY(0);
}

/* 演示模式角标：发送按钮旁的零 token 沙盒提示 */
.demo-badge {
  flex-shrink: 0;
  align-self: center;
  margin-left: 4px;
  padding: 2px 7px;
  font-size: 11px;
  font-weight: 700;
  line-height: 1.4;
  color: #fff;
  background: linear-gradient(135deg, #f59e0b, #ef4444);
  border-radius: 6px;
  cursor: default;
  user-select: none;
}



</style>

<style>
@import './chat-global.css';

/* ==================== AI 流式瀑布渐变 ==================== */
/* 时长/间隔的权威值在 streamFadeConfig（JS 会内联覆盖这里的 .5s 与 --sf-blur） */
@keyframes om-stream-fade {
  from { opacity: 0; filter: blur(var(--sf-blur, 2px)); }
  to   { opacity: 1; filter: blur(0); }
}
.stream-fade-seg {
  animation: om-stream-fade .5s ease-out both;
  will-change: opacity, filter;
}
</style>