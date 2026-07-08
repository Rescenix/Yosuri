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

        <a href="/shanxi-hut" @click="menuOpen = false">
          <Icon icon="mdi:archive-outline" width="16" style="margin-right:8px" />
          项目库
        </a>
        <a href="/blog" @click="menuOpen = false">
          <Icon icon="mdi:book-open-outline" width="16" style="margin-right:8px" />
          研习录
        </a>
        <a href="/timeline" @click="menuOpen = false">
          <Icon icon="mdi:timeline-clock-outline" width="16" style="margin-right:8px" />
          生命线
        </a>
        <a href="https://github.com/3478556810" target="_blank" rel="noopener" @click="menuOpen = false">
          <Icon icon="mdi:github" width="16" style="margin-right:8px" />
          GitHub
        </a>

        <div class="drawer-divider"></div>

      
      </aside>

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
                <Icon icon="mdi:console" width="17" color="#6b6b6b" />
              </button>
              <button class="header-icon-btn" :class="{ active: dockPanels.includes('diff') }" @click="toggleDockPanel('diff')" title="Diff">
                <Icon icon="mdi:file-compare" width="17" color="#6b6b6b" />
              </button>
              <button class="header-icon-btn" :class="{ active: dockPanels.includes('preview') }" @click="toggleDockPanel('preview')" title="预览">
                <Icon icon="mdi:play-circle-outline" width="17" color="#6b6b6b" />
              </button>
              <button class="header-icon-btn" :class="{ active: showBackgroundTasks }" @click.stop="showBackgroundTasks = !showBackgroundTasks" title="后台任务">
                <Icon icon="mdi:clipboard-list-outline" width="17" color="#6b6b6b" />
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
              :active-chat-mode="activeChatMode"
              :workflow-active="workflowState.active"
              @select-session="onFloatingSelectSession"
              @new-session="onFloatingNewSession"
              @rename-session="renameSession"
              @delete-session="deleteSession"
              @trigger-chat="triggerChat"
              @trigger-code="triggerWorkflow()"
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
            :active-chat-mode="activeChatMode"
            :workflow-active="workflowState.active"
            @select-session="selectSession"
            @new-session="newSession"
            @rename-session="renameSession"
            @delete-session="deleteSession"
            @trigger-chat="triggerChat"
            @trigger-code="triggerWorkflow()"
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
                      {{ item.content }}
                    </div>
                    <MessageStepGroup
                      v-else-if="item.kind === 'group'"
                      :id="'group-' + item.id"
                      :group="item"
                      :ref="(el) => setGroupRef(item.id, el)"
                    />
                    <div v-else class="assistant-message">
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

                <!-- 全局工作流审计状态栏：只在有消息时显示 -->
                <div v-if="messages.length > 0" class="global-audit-bar">
                  <div class="audit-left">
                    <Icon icon="majesticons:shooting-star-line" width="16" class="audit-star-icon" :class="auroraState" />
                    <span class="audit-status-text">{{ dynamicStatusText }}</span>
                  </div>
                  <div class="audit-right">
                    <span class="audit-stats">{{ formatDuration(elapsedSeconds) }} · {{ formatTok(tokenStats.inputTokens + tokenStats.outputTokens) }} tokens</span>
                  </div>
                </div>
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

              <div v-else-if="inputTopBarMode === 'git'" class="input-git-bar">
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
                <div v-if="attachments.length" class="attach-chip-row">
                  <div v-for="att in attachments" :key="att.id" class="attach-chip" :class="[att.kind, att.status]">
                    <img v-if="att.kind === 'image'" :src="att.previewUrl" class="attach-chip-thumb" />
                    <div v-else class="attach-chip-icon">
                      <Icon v-if="att.kind === 'folder'" icon="mdi:folder-outline" width="20" color="#8a8378" />
                      <span v-else>{{ att.ext }}</span>
                    </div>
                    <div class="attach-chip-meta">
                      <span class="attach-chip-name" :title="att.name">{{ att.name }}</span>
                      <span v-if="att.status === 'analyzing'" class="attach-chip-status">分析中…</span>
                      <span v-else-if="att.status === 'error'" class="attach-chip-status error">{{ att.errorMsg }}</span>
                      <span v-else-if="att.kind === 'folder'" class="attach-chip-status">{{ att.fileCount }} 个文件</span>
                    </div>
                    <button class="attach-chip-remove" type="button" @click="removeAttachment(att.id)" title="移除">
                      <Icon icon="mdi:close" width="11" />
                    </button>
                  </div>
                </div>

                <div class="input-row">
                  <!-- 渐变动画的浮动占位符 -->
                  <transition name="fade-placeholder" mode="out-in">
                    <span v-if="!userInput.trim()" :key="randomPlaceholder" class="input-placeholder-text">
                      {{ randomPlaceholder }}
                    </span>
                  </transition>

                  <textarea ref="chatInputRef" class="chat-input" v-model="userInput" @keypress.enter="handleSend" @input="adjustInputHeight" @paste="handlePaste" rows="1"></textarea>

                  <!-- "+" 附加菜单用的两个隐藏原生选择器，不占布局，点菜单项时用 .click() 触发 -->
                  <input ref="attachFileInputRef" type="file" multiple style="display:none" @change="onAttachFilesSelected" @click.stop />
                  <input ref="attachFolderInputRef" type="file" webkitdirectory multiple style="display:none" @change="onAttachFolderSelected" @click.stop />

                  <button v-if="workflowState.active" class="input-inner-btn input-right-btn input-stop-btn" @click="stopWorkflow" title="停止工作流（已生成内容会保留）">
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
                  <!-- 模型切换器 -->
                  <div class="sch-model" @click.stop="showModelMenu = !showModelMenu">
                    <span>{{ modelOptions.find(m => m.value === selectedModel)?.label || '模型' }}</span>
                    <span class="sch-caret">▾</span>
                    <div v-if="showModelMenu" class="model-menu-dropdown" @click.stop>
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

                  <!-- 纯圆环 Token 进度 -->
                  <div class="token-ring-widget" @click.stop="toggleTokenPanel" title="Token 用量">
                    <svg class="ctx-ring" width="16" height="16" viewBox="0 0 36 36">
                      <path class="ring-bg" d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831" fill="none" stroke="#e5e5e5" stroke-width="4" />
                      <path class="ring-fg" d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831" fill="none" stroke="#c96442" stroke-width="4" stroke-linecap="round" :style="{ strokeDasharray: '100, 100', strokeDashoffset: 100 - (tokenStats.contextPct || 0) }" />
                    </svg>
                    <div v-if="showTokenPanel" class="token-usage-panel" @click.stop>
                      <div class="tup-row">
                        <span class="tup-label">Context window（估算）</span>
                        <span class="tup-value">{{ formatTok(tokenStats.inputTokens + tokenStats.outputTokens) }} / {{ formatTok(tokenStats.contextWindow) }} ({{ tokenStats.contextPct.toFixed(0) }}%)</span>
                      </div>
                      <div class="tup-bar"><div class="tup-bar-fill" :style="{ width: Math.min(tokenStats.contextPct, 100) + '%' }"></div></div>
                      <div class="tup-row">
                        <span class="tup-label">输入 Tokens</span>
                        <span class="tup-value">{{ formatTok(tokenStats.inputTokens) }}</span>
                      </div>
                      <div class="tup-row">
                        <span class="tup-label">输出 Tokens</span>
                        <span class="tup-value">{{ formatTok(tokenStats.outputTokens) }}</span>
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
                <div class="tool-dock-pane-header">
                  <span class="tool-dock-pane-title">{{ dockPanelLabel(panelKey) }}</span>
                  <span class="tool-dock-pane-meta">{{ { diff: diffTotals, terminal: 'node', preview: '' }[panelKey] }}</span>
                  <button class="tool-dock-pane-close" @click="closeDockPanel(panelKey)" title="关闭">
                    <Icon icon="mdi:close" width="14" color="#a3a3a3" />
                  </button>
                </div>
                <div class="tool-dock-pane-body">
                  <DiffPanel
                    v-if="panelKey === 'diff'"
                    :files="diffFiles"
                    :expanded-diffs="expandedDiffs"
                    @toggle-file="toggleDiffFile"
                  />
                  <Terminal v-else-if="panelKey === 'terminal'" class="tool-panel-terminal" :open="true" :embedded="true" />
                  <div v-else class="tool-panel-preview-empty">
                    <Icon icon="mdi:monitor-dashboard" width="28" color="#c4bcae" />
                    <span>预览功能开发中</span>
                  </div>
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
import DOMPurify from 'dompurify'
import 'katex/dist/katex.min.css'
import MarkdownIt from 'markdown-it'
import markdownItKatex from 'markdown-it-katex'
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
import NewSessionHome from './NewSessionHome.vue'

const props = defineProps({
  autoOpen: { type: Boolean, default: false },
  sessionId: { type: String, default: 'global_chat_session' }
})

// ==================== 会话与数据状态 ====================
const sessionList = ref([
  { id: 'aether', name: 'Aether', desc: '上下文注入层缓存优化', branch: 're0', status: 'running', time: '刚刚' },
  { id: 'prism', name: 'Prism', desc: '文件树懒加载与固定标签', branch: 'main', status: 'done', time: '2 小时前' },
  { id: 'nebula', name: 'Nebula', desc: '终端输出流式渲染', branch: 'dev', status: 'idle', time: '昨天' }
])
const activeSession = ref('aether')
const activeSessionObj = computed(
  () => sessionList.value.find(s => s.id === activeSession.value) || sessionList.value[0] || null
)
function selectSession(id) {
  activeSession.value = id
  switchSession(id)
}
function newSession() {
  const id = 'sess_' + Date.now().toString(36)
  sessionList.value = [{ id, name: '未命名会话', desc: '等待第一条指令…', branch: 'main', status: 'idle', time: '刚刚' }, ...sessionList.value]
  activeSession.value = id
  switchSession(id)
}
function renameSession({ id, name }) {
  const target = sessionList.value.find(s => s.id === id)
  if (target) target.name = name
}
function deleteSession(id) {
  const idx = sessionList.value.findIndex(s => s.id === id)
  if (idx === -1) return
  sessionList.value = sessionList.value.filter(s => s.id !== id)
  if (activeSession.value === id) {
    activeSession.value = sessionList.value[0]?.id || ''
  }
}
const toggleTokenPanel = () => {
  showTokenPanel.value = !showTokenPanel.value
}
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

const expandedDiffs = ref({})
function toggleDiffFile(path) {
  expandedDiffs.value = { ...expandedDiffs.value, [path]: !expandedDiffs.value[path] }
}
// Diff 面板的文件列表不再是硬编码假数据，从消息流里所有 write_file/edit_file
// 工具调用中提取真实的 before/after；同一路径出现多次只保留最后一次改动
// （不尝试重建整个会话期间的累计 diff，后端目前也没提供完整快照可供重建）
const diffFiles = computed(() => {
  const byPath = new Map()
  for (const msg of messages.value) {
    if (msg.kind !== 'group') continue
    for (const step of msg.steps) {
      for (const tc of (step.toolCalls || [])) {
        if (tc.name !== 'write_file' && tc.name !== 'edit_file') continue
        const args = parseToolArgs(tc.args)
        if (!args.path) continue
        if (tc.name === 'write_file') {
          byPath.set(args.path, { path: args.path, oldContent: '', newContent: args.content || '' })
        } else {
          byPath.set(args.path, { path: args.path, oldContent: args.old_string || '', newContent: args.new_string || '' })
        }
      }
    }
  }
  return [...byPath.values()]
})
const diffTotals = ''

// ==================== 工作目录切换：Recent + Open folder ====================
// "文件夹" 是当前 monorepo（GitRepoRoot）下的真实子目录，复用已有的 /api/file-tree
// 拿顶层目录列表，不新开接口；选择结果和最近列表存 localStorage 做持久化
const WORKDIR_STORAGE_KEY = 'aether_workdir_state_v1'
const WORKDIR_IGNORED = new Set(['node_modules', 'build', '__pycache__', 'dist', '.git'])
const currentWorkDir = ref({ name: 'main-frontend', path: 'main-frontend' })
const workDirRecents = ref([{ name: 'main-frontend', path: 'main-frontend' }])
const showWorkDirMenu = ref(false)
const workDirMenuView = ref('recent') // 'recent' | 'browse'
const workDirBrowseOptions = ref([])
const workDirBrowseLoading = ref(false)

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
function toggleWorkDirMenu() {
  showWorkDirMenu.value = !showWorkDirMenu.value
  if (showWorkDirMenu.value) workDirMenuView.value = 'recent'
}
function selectWorkDir(dir) {
  currentWorkDir.value = dir
  // 去重后塞到最前面，最多保留 6 条最近记录
  workDirRecents.value = [dir, ...workDirRecents.value.filter(d => d.path !== dir.path)].slice(0, 6)
  saveWorkDirState()
  showWorkDirMenu.value = false
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

// 输入框上方工具栏三态：首页/新会话(无消息)显示工作目录条；Code 模式(有消息)
// 显示 git 状态条；普通 Chat 模式(有消息)整个区域不显示
const inputTopBarMode = computed(() => {
  if (messages.value.length === 0) return 'dir'
  if (activeChatMode.value === 'code') return 'git'
  return 'none'
})

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
// "自定义" 是动态的第5个选项，标签取用户在设置面板里配的默认 API 配置名——
// 这里只做选择器展示，真正让 custom 这个 value 能发出消息是下一阶段的事
const customApiConfig = ref(null)
async function loadCustomApiConfig() {
  try {
    const res = await fetch('/api/models/config')
    if (!res.ok) return
    const data = await res.json()
    customApiConfig.value = (data.configs || []).find(c => c.is_default) || null
  } catch (e) {}
}
const modelOptions = computed(() => [
  { label: '本地 7B', value: 'local' },
  { label: 'Cloud 480B', value: 'cloud' },
  { label: 'DeepSeek', value: 'ds' },
  { label: 'DeepSeekProxy', value: 'ds_browser' },
  { label: customApiConfig.value ? (customApiConfig.value.name || '自定义') : '自定义 · 未配置', value: 'custom' }
])
const selectedModel = ref(localStorage.getItem('selectedModel') || 'ds_browser')
const showModelMenu = ref(false)
function selectModel(value) { selectedModel.value = value; localStorage.setItem('selectedModel', value); showModelMenu.value = false }

// ==================== 设置面板 ====================
const showSettings = ref(false)
function onSettingsClosed() {
  showSettings.value = false
  loadCustomApiConfig()
}

// ==================== 底部工具条：Auto 模式 + "+" 附加菜单 ====================
const autoModeOptions = ['Auto', 'Ask', 'Plan']
const autoMode = ref('Auto')
const showAutoMenu = ref(false)
const showAddMenu = ref(false)
function selectAutoMode(opt) { autoMode.value = opt; showAutoMenu.value = false }

// ==================== Markdown 渲染 ====================
const md = new MarkdownIt({ breaks: true, linkify: true, html: true })
md.use(markdownItKatex, { throwOnError: false, errorColor: '#ef4444', strict: false })
md.use(function(md) {
  md.core.ruler.before('normalize', 'math_bracket', function(state) {
    state.src = state.src.replace(/\[([\s\S]*?)\]/g, (match, inner) => {
      if (!/\\[a-zA-Z]+/.test(inner)) return match;
      if (/^\s*\${1,2}[\s\S]*\${1,2}\s*$/.test(inner)) return match;
      const trimmed = inner.trim();
      if (trimmed.includes('\n') || trimmed.length > 60 || /\\begin\{/.test(trimmed)) {
        return `$$\n${trimmed}\n$$`;
      }
      return `$${trimmed}$`;
    });
    return true;
  });
})
function renderMarkdown(text, skipSanitize = false) {
  if (!text) return ''
  text = text.replace(/[\u200B\u00A0\u200E\u200F]/g, '')
  text = text.replace(/\\dots/g, '\\ldots')
  text = text.replace(/(?<!\$)\\implies(?!\$)/g, ' $\\implies$ ')
  text = text.replace(/(?<!\$)(\\bbox\[[^\]]*\])(?!\$)/g, (match) => `$${match}$`)
  if (/\\bbox/.test(text)) text = '\\require{bbox}\n' + text
  const raw = md.render(text)
  return skipSanitize ? raw : DOMPurify.sanitize(raw)
}
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

// ==================== useChatWidget ====================
const {
  isOpen, isExpanded, userInput, messages, sessionId,
  isLoggedIn, debugTemp, debugTopP, debugReasoning, lastTokenUsage, lastLatency, debugMaxTokens, balance,
  currentStatus, statusDotColor,
  messagesContainer, chatInputRef, userScrolledUp,
  forceScrollToBottom, adjustInputHeight, switchSession,
  sendMessage, sendWorkflow, stopWorkflow, workflowState, tokenStats, chatState, backgroundTaskList, handleImageUpload, playVoice,
  toggleChat, updateParams,
  groupedMessages, formatChatTime
} = useChatWidget(props, { renderMarkdown })

// ==================== UI 状态 ====================
const menuOpen = ref(false)
const showParams = ref(false)
const showMoreMenu = ref(false)
const showBackgroundTasks = ref(false)

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

// ==================== 模式及悬浮菜单 ====================
const activeChatMode = ref(null)
function triggerChat() {
  if (workflowState.active) return
  activeChatMode.value = activeChatMode.value === 'chat' ? null : 'chat'
  chatInputRef.value?.focus()
}
function triggerWorkflow() {
  if (workflowState.active) return
  activeChatMode.value = activeChatMode.value === 'code' ? null : 'code'
  chatInputRef.value?.focus()
}
function handleSend() {
  if (hasPendingAttachments.value) return
  const combined = buildOutgoingMessage()
  if (!combined) return
  userInput.value = combined
  clearAttachments()
  if (activeChatMode.value === 'code') {
    sendWorkflow('code', userInput.value.trim())
  } else {
    sendMessage()
  }
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

// ==================== 全局工作流审计状态栏：实时计时 ====================
// Chat 模式（非工作流）之前完全没有接入这套计时/aurora 状态机，导致发普通消息
// 时审计栏一直显示"待机"——isBusy 把两条路径统一起来
const isBusy = computed(() => workflowState.active || chatState.active)
const elapsedSeconds = ref(0)
let elapsedTimer = null

// 监听忙碌状态，控制计时器的启动与停止
watch(isBusy, (isRunning) => {
  if (isRunning) {
    // 工作流开始，重置时间为0并开启计时
    elapsedSeconds.value = 0
    clearInterval(elapsedTimer)
    elapsedTimer = setInterval(() => {
      elapsedSeconds.value++
    }, 1000)
  } else {
    // 工作流结束（无论是完成、停止还是报错），立刻停止计时
    clearInterval(elapsedTimer)
    elapsedTimer = null
  }
})

function formatDuration(totalSeconds) {
  const m = Math.floor(totalSeconds / 60)
  const s = totalSeconds % 60
  if (m >= 60) return `${Math.floor(m / 60)}h ${m % 60}m`
  return `${m}m ${s}s`
}

// ==================== Aurora 状态图标：三种质感状态机 ====================
// idle：静止/极轻微呼吸；processing：工作流处理中；commit：任务完成的一次性回弹反馈
const auroraState = ref('待机')
let commitResetTimer = null

// ==================== 右侧动态状态标语：轮换文案 ====================
const statusMessages = ['幻想中...', '想到了绝妙的点子!', '几乎要完成了...', '尝试放弃思考...']
const statusMessageIndex = ref(0)
let statusRotateTimer = null
const dynamicStatusText = computed(() => isBusy.value ? statusMessages[statusMessageIndex.value] : '待机 ...')

// 记录这一次"忙碌"是工作流驱动的还是普通 Chat 驱动的——Chat 模式没有
// workflowState.status 那种 completed/failed/stopped 细分，结束时默认当作成功处理
let busyWasWorkflow = false
watch(isBusy, (isRunning) => {
  if (isRunning) {
    busyWasWorkflow = workflowState.active
    // 开始忙碌：核心星体进入 processing 态，状态标语从头轮换
    auroraState.value = 'processing'
    clearTimeout(commitResetTimer)

    statusMessageIndex.value = 0
    clearInterval(statusRotateTimer)
    statusRotateTimer = setInterval(() => {
      statusMessageIndex.value = (statusMessageIndex.value + 1) % statusMessages.length
    }, 2500)
  } else {
    // 忙碌结束：停止标语轮换；如果是正常完成，触发一次 150ms 的物理回弹反馈，
    // 反馈播完再回到 idle，避免和 idle 的呼吸动画打架
    clearInterval(statusRotateTimer)
    statusRotateTimer = null

    const success = busyWasWorkflow ? workflowState.status === 'completed' : true
    if (success) {
      auroraState.value = 'commit'
      clearTimeout(commitResetTimer)
      commitResetTimer = setTimeout(() => { auroraState.value = '待机' }, 200)
    } else {
      auroraState.value = '待机'
    }
  }
})

// 组件销毁时的兜底清理，防止内存泄漏
onUnmounted(() => {
  if (elapsedTimer) clearInterval(elapsedTimer)
  if (statusRotateTimer) clearInterval(statusRotateTimer)
  if (commitResetTimer) clearTimeout(commitResetTimer)
})

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
async function handlePaste(e) {
  const items = e.clipboardData?.items
  if (!items) return
  let imageFile = null
  for (const item of items) {
    if (item.type && item.type.startsWith('image/')) { imageFile = item.getAsFile(); break }
  }
  if (!imageFile) return
  e.preventDefault()
  if (workflowState.active) { showVisionError('工作流运行中，请稍后再粘贴图片'); return }
  clearTimeout(visionStatusTimer); visionStatus.value = 'analyzing'
  try {
    const base64 = await fileToBase64(imageFile)
    const res = await fetch('/api/aether/vision-preprocess', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ image_base64: base64, mime_type: imageFile.type || 'image/png' })
    })
    if (!res.ok) throw new Error(`请求失败 (${res.status})`)
    const data = await res.json()
    if (!data.text) throw new Error('未返回分析文本')
    visionStatus.value = ''
    const existing = userInput.value
    const assembledTask = `[用户上传了一张图片，Gemini分析结果如下]\n${data.text}\n基于以上信息，请完成以下任务：` + (existing ? `\n${existing}` : '')
    userInput.value = assembledTask
    sendWorkflow('code', assembledTask)
  } catch (err) { showVisionError('图片分析失败') }
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

watch(messages, () => { nextTick(() => { highlightAllCodeBlocks() }) }, { deep: true })
// 切进 git 状态条可见的 Code 模式时刷新一次，避免面板上的 +N/-N 停留在挂载时的旧快照
watch(inputTopBarMode, (mode) => { if (mode === 'git') fetchGitStatus() })

// ==================== 初始化 ====================
onMounted(() => {
  loadCustomApiConfig()
  fetchGitStatus()
  loadWorkDirState()
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



</style>

<style>
@import './chat-global.css';
</style>