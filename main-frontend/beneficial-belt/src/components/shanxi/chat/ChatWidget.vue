<template>
  <div class="chat-widget-root">
    <div class="chat-toggle-button" v-if="!isOpen" @click="toggleChat">
      <Icon icon="mdi:chat" width="28" color="white" />
    </div>

    <div class="chat-window" :class="{ expanded: isExpanded }" :style="{ display: isOpen ? 'flex' : 'none' }">

      <!-- ★ 主内容区 -->
      <div class="chat-main">

        <!-- 顶部横条已删除：会话切换在左侧 Gemini 风侧栏，工具组浮在聊天区右上角 -->
        <div class="chat-body-row" :class="{ 'tool-expanded': dockExpanded }">
        <!-- ★ Gemini 风侧栏：展开=平铺会话面板，折叠=竖向图标条（带会话横条） -->
        <aside
          v-if="isExpanded"
          class="gem-sidebar"
          :class="{ collapsed: !sidebarOpen }"
          :style="sidebarOpen ? { width: sidebarWidth + 'px' } : {}"
        >
          <!-- 展开态：右侧拖拽条，用于自定义侧栏宽度（仿右侧面板） -->
          <div v-if="sidebarOpen" class="gem-sidebar-resize-handle" @mousedown="startSidebarWidthDrag"></div>

          <!-- ★ Gemini 风格对话搜索面板：覆盖整个侧栏，与 Gemini 一致 -->
          <Transition name="search-panel">
            <div v-if="showSearchPanel" class="gem-search-panel" @click.stop>
              <div class="gem-search-head">
                <button class="gem-icon-btn" @click="closeSearchPanel" title="返回">
                  <Icon icon="mdi:arrow-left" width="18" />
                </button>
                <div class="gem-search-input-wrap">
                  <Icon icon="mdi:magnify" width="16" color="#9a9a9a" />
                  <input
                    ref="searchPanelInput"
                    v-model="searchQuery"
                    type="text"
                    class="gem-search-input"
                    placeholder="搜索对话内容"
                    @keydown.esc="closeSearchPanel"
                  />
                  <button v-if="searchQuery" class="gem-search-clear" @click="searchQuery = ''">
                    <Icon icon="mdi:close" width="14" />
                  </button>
                </div>
              </div>
              <div class="gem-search-body">
                <div class="gem-search-section-label">{{ searchQuery.trim() ? '命中会话' : '近期对话' }}</div>
                <div v-if="searchLoading" class="gem-search-empty">搜索中…</div>
                <div v-else-if="searchResults.length === 0" class="gem-search-empty">
                  {{ searchQuery.trim() ? '未找到匹配会话' : '还没有会话' }}
                </div>
                <button
                  v-for="s in searchResults"
                  :key="s.id"
                  class="gem-search-row"
                  :class="{ active: s.id === activeSession }"
                  @click="onSearchSelect(s.id)"
                >
                  <span class="gem-search-row-name">{{ s.name }}</span>
                  <span class="gem-search-row-date">{{ formatSearchDate(s.updatedAt) }}</span>
                  <span v-if="s.hits && s.hits.length" class="gem-search-row-snippet" v-html="highlightHits(s.hits)"></span>
                </button>
              </div>
            </div>
          </Transition>

          <!-- 顶部：品牌标识；搜索与定时任务收进右上角，和 Codex 侧栏保持一致 -->
                    <div class="gem-top">
                                <div class="gem-brand-wrap">
                                  <span v-if="sidebarOpen" class="gem-brand-text">Yosuri</span>
                                </div>
                                <div v-if="sidebarOpen" class="gem-top-actions">
                                                                  <button class="gem-icon-btn" @click="openSearchPanel" title="搜索对话" aria-label="搜索对话">
                                                                    <Icon icon="mdi:magnify" width="18" />
                                                                  </button>
                                                                  <router-link class="gem-icon-btn" to="/sites" title="发布并分享 Agent 写好的网页" aria-label="网站">
                                                                    <Icon icon="mdi:web" width="18" />
                                                                  </router-link>
                                                                  <button class="gem-icon-btn gem-collapse" @click="toggleSidebar" :title="sidebarOpen ? '折叠边栏' : '打开边栏'" aria-label="折叠边栏">
                                    <Icon icon="lucide:sidebar" width="18" />
                                  </button>
                                </div>
                                <button v-else class="gem-icon-btn gem-collapse" @click="toggleSidebar" title="打开边栏" aria-label="打开边栏">
                                  <Icon icon="lucide:sidebar" width="18" />
                                </button>
                              </div>

          <!-- 展开态：复用会话面板（新对话/搜索/置顶/最近/底部账号+设置） -->
          <SessionMenuContent
                                v-if="sidebarOpen"
                                fill
                                :sessions="sessionList"
                                :projects="projects"
                                :recent-projects="workDirRecents"
                                :active-session="activeSession"
                                :running-session="runningSession"
                                :completed-sessions="completedSessions"
                                :question-session="questionSession"
                                :notif-count="notifCount"
                                :current-workdir="currentProjectName"
                      @select-session="selectSession"
            @new-session="newSession"
            @rename-session="renameSession"
            @delete-session="deleteSession"
            @delete-sessions="deleteSessions"
            @delete-project="deleteProject"
            @open-settings="showSettings = true"
            @open-search="openSearchPanel"
                        @open-plugins="openPluginsMarket"
                        @open-scheduled-tasks="showScheduledTaskManager = true"
                                                @create-project="createProject"
                        @open-mail="showMailPanel = true"
                                                @reorder-sessions="reorderSessionList"
                      />

          <!-- 折叠态：竖向图标条（项目就是会话横条本身） -->
          <template v-else>
            <button class="gem-icon-btn" @click="newSession" title="新建任务">
              <Icon icon="mdi:plus-circle-outline" width="18" />
            </button>
            <button class="gem-icon-btn" @click="openSearchPanel" title="搜索对话">
              <Icon icon="mdi:magnify" width="18" />
            </button>
            <button class="gem-icon-btn" @click="openPluginsMarket" title="插件市场">
                          <Icon icon="mdi:puzzle-outline" width="18" />
                        </button>
                        <button class="gem-icon-btn" @click="showScheduledTaskManager = true" title="定时任务">
                          <Icon icon="mdi:clock-outline" width="18" />
                        </button>
            <!-- 会话横条：与 AgentFS 图谱完全分离，保留快速会话跳转 -->
            <div class="gem-rail-sessions" @mouseenter="openRailCard" @mouseleave="closeRailCardDelayed">
              <template v-if="railProject.length">
                <button
                  v-for="s in railProject"
                  :key="s.id"
                  class="gem-rail-bar"
                  :class="{ active: s.id === activeSession, running: s.id === runningSession }"
                  @click="selectSession(s.id)"
                ></button>
                <div class="gem-rail-divider"></div>
              </template>
              <button
                v-for="s in railRecent"
                :key="s.id"
                class="gem-rail-bar"
                :class="{ active: s.id === activeSession, running: s.id === runningSession }"
                @click="selectSession(s.id)"
              ></button>
            </div>
            <div class="gem-rail-bottom">
              <button class="gem-icon-btn" @click="showSettings = true" title="设置">
                <Icon icon="mdi:cog-outline" width="18" />
              </button>
              <img v-if="railAuth.displayAvatar.value" :src="railAuth.displayAvatar.value" class="gem-rail-avatar" :title="railAuth.displayName.value" />
              <div v-else class="gem-rail-avatar" :title="railAuth.displayName.value">{{ (railAuth.displayName.value || '?').charAt(0).toUpperCase() }}</div>
            </div>

            <Teleport to="body">
              <Transition name="agentfs-card">
                <aside
                  v-if="railUtilityPreview"
                  class="rail-utility-preview"
                  :class="`is-${railUtilityPreview}`"
                  :style="railUtilityPreviewStyle"
                  @mouseenter="cancelRailUtilityPreviewClose"
                  @mouseleave="closeRailUtilityPreviewDelayed"
                >
                  <div v-if="railUtilityPreview === 'agentfs'" class="rail-agentfs-preview">
                    <div v-if="gitGraph.commits.length" class="agentfs-tree">
                      <svg class="agentfs-tree-links" :width="agentFSGraphWidth" :height="agentFSGraphHeight" :viewBox="`0 0 ${agentFSGraphWidth} ${agentFSGraphHeight}`" aria-hidden="true">
                        <defs>
                          <linearGradient id="agentfs-preview-link-gradient" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="0%" stop-color="var(--app-accent)" stop-opacity=".72" />
                            <stop offset="100%" stop-color="var(--app-accent)" stop-opacity=".16" />
                          </linearGradient>
                        </defs>
                        <path
                          v-for="link in gitGraphLinks"
                          :key="link.key"
                          :d="link.d"
                          class="agentfs-tree-link"
                          :class="link.trunk ? 'trunk-link' : 'branch-link'"
                          :style="link.trunk ? {} : { '--link-hue': link.hue }"
                        />
                      </svg>
                      <button
                        v-for="node in gitGraphNodes"
                        :key="`preview-${node.commit.hash}`"
                        class="agentfs-node git-graph-node"
                        :class="{ current: node.commit.current }"
                        :style="{ left: node.x + 'px', top: node.y + 'px', '--node-hue': node.hue, '--node-order': node.index }"
                        type="button"
                        :title="`${node.commit.hash.slice(0, 8)} · ${node.commit.subject}`"
                      >
                        <span class="agentfs-node-dot"></span>
                        <span class="agentfs-node-label">{{ node.label }}</span>
                      </button>
                    </div>
                    <div v-else class="rail-utility-empty">当前仓库还没有提交记录</div>
                  </div>
                                  </aside>
              </Transition>
            </Teleport>

            <Teleport to="body">
              <Transition name="agentfs-card">
                <aside
                  v-if="selectedAgentFSSnapshot"
                  class="agentfs-diff-card"
                  :style="agentFSDiffCardStyle"
                  @click.stop
                >
                  <div class="agentfs-card-meta">
                    <Icon icon="mdi:file-code-outline" width="15" class="agentfs-meta-icon" />
                    <strong :title="selectedAgentFSSnapshot.rel_path">{{ selectedAgentFSSnapshot.rel_path }}</strong>
                    <code>{{ selectedAgentFSSnapshot.commit }}</code>
                    <span class="agentfs-diff-stats">
                      <b>+{{ agentFSDiffStats.added }}</b>
                      <em>−{{ agentFSDiffStats.removed }}</em>
                    </span>
                    <span class="agentfs-meta-time">{{ selectedAgentFSSnapshot.op === 'edit' ? '编辑' : '写入' }} · {{ formatAgentFSTime(selectedAgentFSSnapshot.ts) }}</span>
                    <button type="button" class="agentfs-card-close" title="关闭" @click="closeAgentFSDiff">
                      <Icon icon="mdi:close" width="15" />
                    </button>
                  </div>
                  <div class="agentfs-diff-body">
                    <div v-if="agentFSDiffLoading" class="agentfs-diff-state">
                      <Icon icon="mdi:loading" width="20" class="agentfs-spin" /> 正在读取快照…
                    </div>
                    <div v-else-if="agentFSDiffError" class="agentfs-diff-state error">{{ agentFSDiffError }}</div>
                    <div v-else-if="agentFSDiffLines.length" class="agentfs-code">
                      <div
                        v-for="(line, index) in agentFSDiffLines"
                        :key="index"
                        class="agentfs-code-line"
                        :class="line.kind"
                      >
                        <span class="agentfs-line-no">{{ line.number }}</span>
                        <code>{{ line.text || ' ' }}</code>
                      </div>
                    </div>
                    <div v-else class="agentfs-diff-state">该快照没有可显示的文本差异</div>
                  </div>
                </aside>
              </Transition>
            </Teleport>

            <!-- 悬停会话卡片：贴着折叠栏右侧弹出，整行可点击切换会话。
                 Teleport 到 body，避免被侧栏的 overflow/宽度裁切。 -->
            <Teleport to="body">
              <div
                v-if="railCardOpen"
                class="rail-card"
                :style="railCardStyle"
                @mouseenter="openRailCard"
                @mouseleave="closeRailCardDelayed"
              >
                <div class="rail-card-label">{{ currentWorkDir.name || '最近' }}</div>
                <button
                  v-for="s in railProject"
                  :key="s.id"
                  class="rail-card-row"
                  :class="{ active: s.id === activeSession }"
                  @click="onRailCardSelect(s.id)"
                >
                  <span class="rail-card-mark" :class="{ running: s.id === runningSession }"></span>
                  <span class="rail-card-name">{{ s.name }}</span>
                </button>
                <div v-if="railRecent.length" class="rail-card-divider"></div>
                <div class="rail-card-label">其他</div>
                <button
                  v-for="s in railRecent"
                  :key="s.id"
                  class="rail-card-row"
                  :class="{ active: s.id === activeSession }"
                  @click="onRailCardSelect(s.id)"
                >
                  <span class="rail-card-mark" :class="{ running: s.id === runningSession }"></span>
                  <span class="rail-card-name">{{ s.name }}</span>
                </button>
              </div>
            </Teleport>
          </template>
        </aside>

                <div class="chat-body studio">
          <!-- 共享聊天列 -->
          <div class="chat-content studio">

            <!-- 会话标题：消息列上方，靠左，背景透明 -->
            <div v-if="messages.length && activeSessionObj?.name" class="chat-session-titlebar">
              <span class="chat-session-title-text">{{ activeSessionObj.name }}</span>
            </div>

            <!-- 没有工具窗口时才显示横向入口；一旦打开工具窗就完全隐藏，避免遮挡内容。 -->
            <div v-if="inputTopBarMode === 'git' && !hasVisibleDockPanels" class="floating-tools">
              <button class="header-icon-btn" :class="{ active: dockPanels.includes('terminal') }" @click="toggleDockPanel('terminal')" title="终端">
                <Icon icon="ri:terminal-line" width="17" color="#6b6b6b" />
              </button>
              <button class="header-icon-btn" :class="{ active: dockPanels.includes('diff') }" @click="toggleDockPanel('diff')" title="Diff">
                <Icon icon="proicons:diff" width="17" color="#6b6b6b" />
              </button>
              <button class="header-icon-btn" :class="{ active: dockPanels.includes('preview') }" @click="toggleDockPanel('preview')" title="预览">
                <Icon icon="mage:preview" width="17" color="#6b6b6b" />
              </button>
              <button class="header-icon-btn" :class="{ active: dockPanels.includes('tasks') }" @click.stop="toggleDockPanel('tasks')" title="后台任务">
                <Icon icon="mdi:task-minus" width="17" color="#6b6b6b" />
              </button>
              <button class="header-icon-btn" :class="{ active: dockPanels.includes('file') }" @click.stop="toggleDockPanel('file')" title="文件">
                <Icon icon="mdi:file-code-outline" width="17" color="#6b6b6b" />
              </button>
            </div>

            <!-- 重构：将 Home 组件从 `chat-messages` 中剥离，作为 `chat-content` 的直接子节点。
                 当 `messages` 为空时，它独占整个 Flex 空间，把输入区推到最底部。 -->
            <div v-if="messages.length === 0" class="home-container-for-layout">
              <NewSessionHome
                :show-content="showHeatmapPopup"
                @send-followup="onFollowUpClick"
              />
            </div>

            <!-- 普通聊天/工作流模式：当有消息时，滚动容器才接管整个区域 -->
                        <div v-else class="chat-messages" ref="messagesContainer">
                          <!-- 顶部边缘 blur（仿 Gemini：内容从模糊里滑入/滑出）。
                   必须跟底部那条一样待在 .chat-messages 里：原来它是 .chat-content 的
                   绝对定位子节点，而 .chat-content 有右侧补偿 padding，绝对定位按
                   padding box 算，blur 就整体右移、正文左侧压根没被盖住。 -->
              <div class="msg-edge-blur top"></div>
              <div class="chat-messages-inner">
                <template v-for="item in groupedMessages">
                  <div v-if="item.type === 'time'" :key="`time-${item.timestamp}`" class="chat-time">
                    {{ formatChatTime(item.timestamp) }}
                  </div>
                  <div v-else-if="item.type === 'message'" :key="item.id" class="message-row" :class="item.sender" :data-msg-id="item.id">
                    <article v-if="item.kind === 'screenshot'" class="agent-screenshot-card">
                      <div class="agent-screenshot-head">
                        <span><Icon icon="mdi:camera-outline" width="15" /> Agent 已发布截图</span>
                        <small>{{ item.sourceUrl || '当前预览' }}</small>
                      </div>
                      <img :src="item.image" :alt="item.content || 'Agent 交付截图'" class="agent-screenshot-image" />
                      <p v-if="item.content" class="agent-screenshot-note">{{ item.content }}</p>
                    </article>
                    <div v-else-if="item.sender === 'user'" class="message-bubble user" :class="{ editing: editingMsgId === item.id, active: String(activeUserMessageId) === String(item.id) }">
                      <AttachmentChipRow v-if="item.attachments?.length" :attachments="item.attachments" />
                      <!-- 编辑态：消息框本身变成输入框，就地改，不去下面的输入框 -->
                      <textarea
                        v-if="editingMsgId === item.id"
                        class="msg-edit-input"
                        v-model="editDraft"
                        rows="1"
                        @keydown.enter.exact.prevent="confirmEdit(item)"
                        @keydown.esc.prevent="cancelEdit"
                        @input="autoGrowEdit"
                        @blur="onEditBlur"
                      ></textarea>
                      <!-- 点击消息文本本身就进入编辑态（不再依赖右下角铅笔按钮）。
                           文本 div 只在「非编辑态」渲染，与上面的 textarea 互斥，
                           所以点击它进入编辑不会重复触发。 -->
                      <div
                        v-else-if="item.content"
                        class="msg-user-text"
                        @click="editUserMessage(item)"
                      >{{ item.content }}</div>
                      <!-- 编辑态右下角按钮变「发送」，否则是悬浮出现的「编辑」。
                           mousedown.prevent：点发送时不让 textarea 失焦，否则会先触发 @blur 复原、
                           把编辑态撤掉导致这一下点了个寂寞。 -->
                      <button
                        v-if="editingMsgId === item.id"
                        class="msg-edit-btn confirm"
                        title="发送（Enter），Esc 取消"
                        @mousedown.prevent
                        @click="confirmEdit(item)"
                      >
                        <Icon icon="mdi:arrow-up" width="16" />
                      </button>
                    </div>
                    <MessageStepGroup
                      v-else-if="item.kind === 'group'"
                      :id="'group-' + item.id"
                      :group="item"
                      :ref="(el) => setGroupRef(item.id, el)"
                    />
                    <!-- 必须包一层竖向容器：.message-row 是 flex-direction:row，
                         面板和工具栏平铺进去的话工具栏会变成"面板右边被拉满高的一竖条" -->
                    <div v-else-if="item.kind === 'agentflow'" class="agentflow-wrap">
                      <AgentWorkflowPanel :id="'group-' + item.id" :flow="item" />
                      <!-- 复制栏：以前只挂在纯文本 assistant 气泡上，而现在所有回复
                           都走四态机(agentflow)，等于这一栏彻底消失了。跑完再显示，跑的过程中
                           内容还在变，复制没意义。 -->
                      <div v-if="item.status === 'completed' && flowFinalText(item)" class="flow-tools">
                        <button class="tool-btn" @click="copyText(flowFinalText(item))" title="复制">
                          <Icon icon="mdi:content-copy" width="16" />
                        </button>
                        <span class="tools-spacer"></span>
                        <button class="tool-btn speak-btn" @click="speakMessage(item, flowFinalText(item))" :title="speakState(item.id)" :class="{ 'speaking': speakingId === item.id, 'loading': loadingId === item.id }">
                          <Icon :icon="speakIcon(item.id)" width="18" :class="{ 'spin': loadingId === item.id }" />
                        </button>
                      </div>
                    </div>
                    <div v-else class="assistant-message" :class="{ streaming: item.isStreaming }">
                      <div v-if="item.reasoning" class="reasoning-stream">
                        <div class="reasoning-label">
                          正在思考
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
                        <button class="tool-btn" @click="copyText(item.content)" title="复制">
                          <Icon icon="mdi:content-copy" width="16" />
                        </button>
                        <span class="tools-spacer"></span>
                        <button class="tool-btn speak-btn" @click="speakMessage(item, item.content)" :title="speakState(item.id)" :class="{ 'speaking': speakingId === item.id, 'loading': loadingId === item.id }">
                          <Icon :icon="speakIcon(item.id)" width="18" :class="{ 'spin': loadingId === item.id }" />
                        </button>
                      </div>
                    </div>
                  </div>
                </template>
              </div>
              <!-- 底部边缘 blur：sticky 吸在滚动视口底边，正好压在输入区上沿 -->
              <div class="msg-edge-blur bottom"></div>
            </div>

            <!-- 用户消息导航：以完整聊天内容列为定位基准，避免随消息滚动区高度塌缩到底部。 -->
            <UserMessageRail :messages="messages" :active-id="activeUserMessageId" @jump="jumpToMessage" />

            <div v-if="copiedVisible" class="copy-toast">✓ 已复制</div>

            <div class="chat-input-area">
              <!-- 回到底部：紧贴在输入框卡片正上方 -->
              <button v-show="showScrollButton" class="scroll-to-bottom-btn" :class="{ 'wf-running': flowState.active }" @click="forceScrollToBottom" title="回到底部">
                <Icon icon="mdi:chevron-down" width="20" color="#555" />
              </button>

              <!-- ===== 任务清单条（TODO）=====
                   仿 Hermes：贴在输入框正上方，agent 调 update_todo 时实时勾选。
                   后端推 todo 事件 → todoState.items 全量覆盖。全部完成后延迟 3.5s 淡出，
                   让用户先看到 N/N 再整条消失；中途出现新/未完成项则取消淡出。 -->
              <Transition name="todo-fade">
                              <div v-if="todoState.items.length" class="todo-bar" :style="inputBarFadeStyle">
                                <div class="todo-bar-head" :class="{ collapsed: todoCollapsed }" @click="todoCollapsed = !todoCollapsed">
                                  <Icon :icon="todoCollapsed ? 'mdi:chevron-up' : 'mdi:chevron-down'" width="14" class="todo-bar-chevron" />
                                  <Icon icon="mdi:format-list-checks" width="14" class="todo-bar-icon" />
                                  <span class="todo-bar-title">任务</span>
                                  <span class="todo-bar-progress">{{ todoDoneCount }}/{{ todoState.items.length }}</span>
                                </div>
                                <ul v-show="!todoCollapsed" class="todo-bar-list">
                                  <li
                                    v-for="(it, i) in todoState.items"
                                    :key="i"
                                    class="todo-bar-item"
                                    :class="'todo-' + it.status"
                                  >
                                    <Icon
                                      :icon="it.status === 'done' ? 'mdi:check-circle' : it.status === 'doing' ? 'mdi:dots-vertical' : 'mdi:circle-outline'"
                                      width="14"
                                      class="todo-bar-mark"
                                    />
                                    <span class="todo-bar-text">{{ it.text }}</span>
                                  </li>
                                </ul>
                              </div>
                            </Transition>

              <!-- Agent 提问：与审批条同层，直接贴在输入框上方，选单选项后立即提交。 -->
              <QuestionModal
                v-if="questionState.pending"
                :question="questionState.pending"
                :style="inputBarFadeStyle"
                @answer="answerQuestion"
              />

              <!-- ===== 工具审批轻量条（Ask 模式）=====
                   贴在输入框正上方，不打断视线；每条 60s 倒计时，归零自动同意
                   （后端另有 65s 兜底，防前端整个挂掉时工作流永久阻塞）。 -->
              <div
                v-for="item in approvalState.pending"
                :key="item.id"
                class="approval-bar"
              >
                <span class="approval-bar-countdown" :title="item.remain + ' 秒后自动同意'">{{ item.remain }}</span>
                <div class="approval-bar-main">
                  <div class="approval-bar-line">
                    <span class="approval-bar-tool">{{ item.tool }}</span>
                    <!-- 越界访问单独标一下：不然只看工具名会以为是普通的写盘确认 -->
                    <span
                      v-if="item.reason === 'path_outside_workdir'"
                      class="approval-bar-badge"
                      :title="'工作目录：' + item.workdir"
                    >工作目录之外</span>
                    <span class="approval-bar-args">{{ approvalArgsPreview(item.args) }}</span>
                  </div>
                  <div class="approval-bar-progress">
                    <div class="approval-bar-progress-fill" :style="{ width: (item.remain / item.total * 100) + '%' }"></div>
                  </div>
                </div>
                <label
                  class="approval-bar-remember"
                  :title="item.reason === 'path_outside_workdir'
                    ? '本次会话内不再询问该目录下的操作'
                    : '本次会话内不再询问此工具'"
                >
                  <input type="checkbox" v-model="item.remember" />
                  <span>不再问</span>
                </label>
                <button class="approval-bar-btn deny" @click="respondApproval(item, false)">拒绝</button>
                <button class="approval-bar-btn allow" @click="respondApproval(item, true)">允许</button>
              </div>

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
                    <span
                      class="input-dir-item input-dir-clickable"
                      :class="{ active: showBranchMenu }"
                      @click.stop="toggleBranchMenu"
                    >
                      <Icon icon="mdi:source-branch" width="13" color="#6b6b6b" />
                      {{ gitStatus.branch || 'main' }}
                    </span>
                    <button class="input-dir-add-btn" type="button" title="从系统中选择工作目录" @click.stop="openSystemWorkDirPicker">
                      <Icon icon="mdi:plus" width="15" />
                    </button>
                    <input
                      ref="workDirFolderInputRef"
                      type="file"
                      webkitdirectory
                      multiple
                      class="workdir-folder-input"
                      @change="onSystemWorkDirSelected"
                      @click.stop
                    />
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
                      <div class="workdir-menu-item" @click="openSystemWorkDirPicker">
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
                  <div v-if="showBranchMenu" class="branch-menu-dropdown" @click.stop>
                    <label class="branch-search">
                      <Icon icon="mdi:magnify" width="17" />
                      <input
                        ref="branchSearchInput"
                        v-model="branchSearch"
                        placeholder="搜索分支"
                        @keydown.esc="showBranchMenu = false"
                      />
                    </label>
                    <div class="branch-menu-label">分支</div>
                    <div v-if="branchesLoading" class="branch-menu-empty">正在读取分支…</div>
                    <button
                      v-for="branch in filteredGitBranches"
                      :key="branch"
                      type="button"
                      class="branch-menu-item"
                      :disabled="branchSwitching"
                      @click="checkoutGitBranch(branch)"
                    >
                      <Icon icon="mdi:source-branch" width="17" />
                      <span>{{ branch }}</span>
                      <Icon
                        v-if="branch === gitStatus.branch"
                        class="branch-menu-check"
                        icon="mdi:check"
                        width="18"
                      />
                    </button>
                    <div v-if="!branchesLoading && !filteredGitBranches.length" class="branch-menu-empty">
                      没有匹配的分支
                    </div>
                    <div class="branch-menu-divider"></div>
                    <button type="button" class="branch-menu-create" @click="createGitBranch">
                      <Icon icon="mdi:plus" width="20" />
                      <span>创建并检出新分支…</span>
                    </button>
                  </div>
                </div>

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
                    </div>
                  </div>
                </div>
              </div>

              <!-- 输入框容器：外层改列布局，附件预览条占一整行浮在文字行上方，
                                 原来的横向内容（占位符/textarea/按钮）收进 .input-row 保持不变 -->
                            <div ref="inputWrapperRef" class="input-wrapper" style="position: relative;">
                                                        <!-- 后台任务/子代理悬浮条：有进行中的任务时显示，紧贴输入框上沿 -->
                                                        <div v-if="runningTaskCount > 0" class="input-running-tasks-bar" @click.stop="toggleDockPanel('tasks')">
                                                          <Icon icon="mdi:loading" width="13" class="rt-spin" />
                                                          <template v-if="runningSubagentCount > 0"><span>{{ runningSubagentCount }} 个子代理进行中</span></template>
                                                          <template v-if="runningBgTaskCount > 0"><span>{{ runningBgTaskCount }} 个后台任务进行中</span></template>
                                                          <template v-if="runningSubagentCount === 0 && runningBgTaskCount === 0"><span>{{ runningTaskCount }} 个任务进行中</span></template>
                                                          <Icon icon="mdi:chevron-up" width="14" class="rt-caret" />
                                                        </div>
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
                    <span v-if="!userInput.trim() && attachments.length === 0 && !historySwapping" :key="randomPlaceholder" class="input-placeholder-text">
                      {{ randomPlaceholder }}
                    </span>
                  </transition>

                  <div class="toolbar-dropdown-wrap">
                    <button class="input-left-plus" @click.stop="showAddMenu = !showAddMenu" title="添加">
                      <Icon icon="mdi:plus" width="16" />
                    </button>
                    <div v-if="showAddMenu" class="add-menu-dropdown" @click.stop>
                      <div class="add-menu-item" @click="triggerAttachFiles">
                        <Icon icon="mdi:paperclip" width="14" color="#6b6b6b" />
                        <span>添加文件或照片</span>
                      </div>
                      <div class="add-menu-item" @click="triggerAttachVideo">
                        <Icon icon="mdi:video-outline" width="14" color="#6b6b6b" />
                        <span>添加视频</span>
                      </div>
                      <div class="add-menu-item" @click="triggerAttachFolder">
                        <Icon icon="mdi:folder-outline" width="14" color="#6b6b6b" />
                        <span>添加文件夹</span>
                      </div>
                    </div>
                  </div>

                  <textarea ref="chatInputRef" class="chat-input" v-model="userInput" @keydown.enter.prevent="handleSend" @keydown.up="onChatInputKeydown" @keydown.down="onChatInputKeydown" @input="adjustInputHeight" @paste="handlePaste" rows="1"></textarea>

                  <!-- 模型名 pill：点击向上弹模型菜单（悬浮在输入框上方） -->
                  <div class="sch-model" ref="modelPillRef" @click.stop="toggleModelMenu">
                    <span>{{ selectedModelLabel }}</span>
                    <Icon icon="mdi:chevron-down" width="14" class="sch-model-caret" />
                  </div>

                  <!-- 语音输入：按住说话（Edge 原生语音识别） -->
                  <button class="voice-btn" :class="{ listening: voiceListening }"
                    @mousedown.prevent="voiceStart" @mouseup.prevent="voiceStop" @mouseleave="voiceStop"
                    :title="voiceListening ? '松开结束并发送' : '按住说话'">
                    <Icon :icon="voiceListening ? 'mdi:microphone' : 'mdi:microphone-outline'" width="18" />
                  </button>

                  <!-- 语音输出开关：喇叭静音 = 关闭 AI 自动朗读 -->
                  <button class="input-inner-btn speak-toggle-btn" :class="{ muted: !ttsOn }" @click="toggleTts"
                    :title="ttsOn ? '关闭语音朗读' : '开启语音朗读（AI 回复自动朗读）'">
                    <Icon :icon="ttsOn ? 'mdi:volume-high' : 'mdi:volume-mute'" width="18" :color="ttsOn ? '#171717' : undefined" />
                  </button>

                  <Teleport to="body">
                                      <div v-if="showModelMenu" class="model-menu-dropdown" :style="modelMenuStyle" @click.stop>
                                        <div class="model-menu-head">
                                          <div class="model-menu-search">
                                            <Icon icon="mdi:magnify" width="14" class="model-menu-search-icon" />
                                            <input v-model="modelSearch" type="text" placeholder="搜索模型" class="model-menu-search-input" @click.stop />
                                          </div>
                                          <button class="model-menu-manage" @click.stop="showModelManager = true; showModelMenu = false" title="管理模型">
                                            <Icon icon="mdi:cog-outline" width="14" />
                                            <span>管理模型</span>
                                          </button>
                                        </div>
                                        <div v-if="!hasModels" class="model-menu-empty">没有可用模型（去设置填 Key 或选免 Key 模型）</div>
                                        <div v-if="hasModels" class="model-menu-list">
                                          <div
                                            class="model-menu-item model-menu-auto"
                                            :class="{ active: selectedModel === 'auto' }"
                                            @click="selectModel('auto')"
                                          >
                                            <span class="model-menu-check" v-if="selectedModel === 'auto'">✓</span>
                                            <span>Auto 智能路由</span>
                                          </div>
                                          <div class="model-menu-divider"></div>
                                          <template v-for="grp in filteredGroupedOptions" :key="grp.vendor">
                                            <div class="model-menu-group-title">{{ grp.vendor }}</div>
                                            <div
                                              v-for="m in grp.items"
                                              :key="m.value"
                                              class="model-menu-item"
                                              :class="{ active: selectedModel === m.value, 'effort-hover': hoveredModel === m.value }"
                                              @click="selectModel(m.value)"
                                              @mouseenter="onItemHover(m.value, $event)"
                                              @mouseleave="onItemLeave"
                                            ><span class="model-menu-check" v-if="selectedModel === m.value">✓</span><span class="model-menu-item-label">{{ m.label }}</span><span class="model-menu-effort-tag">{{ effortLabel }}</span><span v-if="sharedPoolModelIds.has(m.value)" class="model-menu-tag-free">公益免费</span></div>
                                          </template>
                                        </div>
                                      </div>
                                      <!-- hover 思考强度选择卡片 -->
                                      <div v-if="hoveredModel && showModelMenu" class="model-menu-effort-popup" :style="hoverEffortStyle" @click.stop @mouseenter="hoverPopupEnter" @mouseleave="hoverPopupLeave">
                                        <div class="mep-header">
                                          <span class="mep-title">思考</span>
                                          <span class="mep-toggle" :class="{ on: debugReasoning }" @click="toggleThinking">{{ debugReasoning ? '开' : '关' }}</span>
                                        </div>
                                        <div class="mep-divider"></div>
                                        <div class="mep-section-label">推理强度</div>
                                        <div
                                          v-for="lv in EFFORT_LEVELS"
                                          :key="lv"
                                          class="mep-option"
                                          :class="{ active: effortLevel === EFFORT_LEVELS.indexOf(lv) }"
                                          @click="selectEffort(lv)"
                                        ><span class="mep-opt-label">{{ EFFORT_UI_LABELS[lv] }}</span><span class="mep-opt-check" v-if="effortLevel === EFFORT_LEVELS.indexOf(lv)">✓</span></div>
                                      </div>
                                    </Teleport>

                  <!-- "追加"菜单用的两个隐藏原生选择器，不占布局，点菜单项时用 .click() 触发 -->
                  <input ref="attachFileInputRef" type="file" multiple style="display:none" @change="onAttachFilesSelected" @click.stop />
                  <input ref="attachFolderInputRef" type="file" webkitdirectory multiple style="display:none" @change="onAttachFolderSelected" @click.stop />
                  <input ref="attachVideoInputRef" type="file" accept="video/*" multiple style="display:none" @change="onAttachVideoSelected" @click.stop />

                  <button v-if="flowState.active && !userInput.trim() && attachments.length === 0" class="input-inner-btn input-right-btn input-stop-btn" @click="stopCodeWorkflow()" title="停止工作流（已生成内容会保留）">
                    <Icon icon="mdi:stop" width="16" color="#fff" />
                  </button>
                  <!-- 发送按钮常驻占位：即使无输入也显示（灰色向上箭头），保证模型 pill 旁边永远有它，
                       不因空输入让 pill 沦为最右元素。有输入时高亮为可用态。 -->
                  <button v-else class="input-inner-btn input-right-btn" :class="flowState.active ? 'input-steer-btn' : (userInput.trim() || attachments.length ? 'input-send-btn' : 'input-send-btn input-send-idle')" @click="handleSend" :title="flowState.active ? '发送插话给工作流（不打断，模型下一轮处理）' : '发送消息'">
                    <Icon :icon="flowState.active ? 'mdi:send' : 'mdi:arrow-up'" width="18" color="#fff" />
                  </button>
                </div>
              </div>

              <!-- ========== 底部工具条（本次漏掉的部分已精准补全） ========== -->
              <div class="input-bottom-toolbar">
                  <div class="input-toolbar-left">
                  <!-- git 工具栏开关：点亮时上方展开分支/PR 状态条，只在有会话时出现 -->
                  <button
                    v-if="inputTopBarMode === 'git'"
                    class="toolbar-icon-pill-btn"
                    @click.stop="toggleGitBar"
                    title="Git 工具栏"
                  >
                    <Icon icon="mdi:source-branch" width="15" />
                  </button>
                </div>

                                <div class="input-toolbar-right">
                                  <!-- Context window 用量：圆环 + 模型 pill + 模式 pill（紧凑版） -->
                  <div v-if="messages.length > 0 && ctxTotalUsed > 0" class="context-bar-widget" @click.stop="toggleTokenPanel" title="Context window 用量">
                    <svg class="ctx-reservoir" viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
                      <defs>
                        <clipPath id="ctxResClip">
                          <circle cx="12" cy="12" r="10" />
                        </clipPath>
                      </defs>
                      <circle cx="12" cy="12" r="10" fill="none" stroke="var(--app-accent)" stroke-width="1.2" opacity="0.5" />
                      <g clip-path="url(#ctxResClip)">
                        <rect class="ctx-water-body" x="0" :y="ctxWaterY" width="24" :height="ctxWaterH" fill="var(--app-accent)" opacity="0.85" />
                        <path class="ctx-wave" :d="ctxWavePath" fill="var(--app-accent)" opacity="0.85" />
                      </g>
                    </svg>
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

                  <Teleport to="body">
                    <div v-if="showEffortPanel" class="effort-panel" :style="effortPanelPos" @click.stop>
                      <div class="effort-panel-title">
                        Effort <b>{{ modelOptions.find(m => m.value === selectedModel)?.label || '' }}</b>
                      </div>
                      <div class="effort-slider-row">
                        <span class="effort-end">快</span>
                                                <input type="range" min="0" max="2" step="1" v-model.number="effortLevel" class="effort-slider" @click.stop @input="onEffortChange" />
                                                <span class="effort-end">深</span>
                      </div>
                    </div>
                  </Teleport>
                </div>
              </div>
            </div>
          </div>

          <!-- ★ Windows / VS Code 风格双停靠工作区：标签可在右侧与底部之间拖放。 -->
          <aside
            v-for="dockLocation in visibleDockLocations"
            :key="dockLocation"
            class="tool-panel tool-panel-tabbed"
            :class="[
              `dock-${dockLocation}`,
              {
                hidden: dockIsHidden(dockLocation),
                expanded: dockIsExpanded(dockLocation),
                'drop-active': dockDropTarget === dockLocation
              }
            ]"
            :style="dockPanelStyle(dockLocation)"
            @dragover.prevent.stop="onDockRegionDragOver(dockLocation)"
            @drop.prevent.stop="onDockPanelDrop(dockLocation)"
          >
            <div
              class="tool-dock-resize-handle"
              :class="`is-${dockLocation}`"
              @mousedown="dockLocation === 'right' ? startDockWidthDrag($event) : startDockHeightDrag($event)"
            ></div>
            <div class="tool-dock-tabs">
              <div class="tool-dock-location-mark" :title="dockLocation === 'right' ? '右侧工具区' : '底部工具区'">
                <Icon :icon="dockLocation === 'right' ? 'mdi:dock-right' : 'mdi:dock-bottom'" width="14" />
              </div>
              <button
                v-for="panelKey in dockPanelsFor(dockLocation)"
                :key="panelKey"
                class="tool-dock-tab"
                :class="{
                  active: activeDockPanelFor(dockLocation) === panelKey,
                  dragging: draggedDockPanel === panelKey
                }"
                draggable="true"
                @click="setActiveDockPanel(panelKey, dockLocation)"
                @dblclick="moveDockPanel(panelKey, dockLocation === 'right' ? 'bottom' : 'right')"
                @dragstart="onDockTabDragStart($event, panelKey, dockLocation)"
                @dragend="onDockTabDragEnd"
                @dragover.prevent.stop="onDockTabDragOver(dockLocation)"
                @drop.prevent.stop="onDockPanelDrop(dockLocation, panelKey)"
                :title="`${dockPanelLabel(panelKey)} · 拖动可停靠，双击快速切换位置`"
              >
                <Icon icon="mdi:dots-grid" width="12" class="tool-dock-tab-grip" />
                <Icon :icon="dockPanelIcon(panelKey)" width="14" class="tool-dock-tab-icon" />
                <span class="tool-dock-tab-label">{{ dockPanelLabel(panelKey) }}</span>
                <span class="tool-dock-tab-close" @click.stop="closeDockPanel(panelKey)">
                  <Icon icon="mdi:close" width="12" />
                </span>
              </button>
              <div class="tool-dock-tab-actions">
                <button class="tool-dock-tab-add" @click.stop="toggleDockAddMenu($event, dockLocation)" title="添加工具标签页">
                  <Icon icon="mdi:plus" width="16" />
                </button>
              </div>
              <div class="tool-dock-global-actions">
                <span class="tool-dock-drag-tip">拖动标签可重新布局</span>
                <button class="tool-dock-tab-add" @click.stop="toggleDockExpanded(dockLocation)" :title="dockIsExpanded(dockLocation) ? '还原' : '最大化工具区'">
                  <Icon :icon="dockIsExpanded(dockLocation) ? 'mdi:arrow-collapse' : 'mdi:arrow-expand'" width="15" />
                </button>
                <button class="tool-dock-tab-add" @click.stop="toggleDockHidden(dockLocation)" :title="dockLocation === 'right' ? '隐藏右侧工具区' : '隐藏底部工具区'">
                  <Icon :icon="dockLocation === 'right' ? 'mdi:dock-right' : 'mdi:dock-bottom'" width="15" />
                </button>
              </div>
            </div>
            <div class="tool-dock-pane tool-dock-pane-single">
              <div class="tool-dock-pane-body">
                <DiffPanel v-if="activeDockPanelFor(dockLocation) === 'diff'" />
                <div v-else-if="activeDockPanelFor(dockLocation) === 'terminal'" class="terminal-group">
                  <div class="terminal-tabs-bar">
                    <button
                      v-for="tab in terminalTabs"
                      :key="tab.id"
                      class="terminal-tab-item"
                      :class="{ active: tab.id === activeTerminalId }"
                      @click="activeTerminalId = tab.id"
                    >
                      <Icon icon="ri:terminal-line" width="12" />
                      <span>{{ tab.name }}</span>
                      <span v-if="terminalTabs.length > 1" class="terminal-tab-close" @click.stop="closeTerminalTab(tab.id)">
                        <Icon icon="mdi:close" width="11" />
                      </span>
                    </button>
                    <button class="terminal-tab-add" @click="addTerminalTab" title="新建终端">
                      <Icon icon="mdi:plus" width="14" />
                    </button>
                    <div class="terminal-tabs-spacer"></div>
                    <button class="terminal-tab-snippet-btn" :class="{ active: showSnippet }" @click="showSnippet = !showSnippet" title="脚本片段">
                      <Icon icon="mdi:code-braces" width="15" />
                    </button>
                  </div>
                  <div class="terminal-with-snippet">
                    <Terminal
                      v-for="tab in terminalTabs"
                      :key="tab.id"
                      v-show="tab.id === activeTerminalId"
                      class="tool-panel-terminal"
                      :open="true"
                      :embedded="true"
                      :terminal-id="tab.id"
                      :snippet-visible="showSnippet && tab.id === activeTerminalId"
                      :snippet-insert-cmd="tab.id === activeTerminalId ? snippetInsertCmd : ''"
                      @toggle-snippet="showSnippet = !showSnippet"
                    />
                    <SnippetPanel v-if="showSnippet" @insert="onSnippetInsert" />
                  </div>
                </div>
                <PreviewBrowser v-else-if="activeDockPanelFor(dockLocation) === 'preview'" />
                <FileToolPanel v-else-if="activeDockPanelFor(dockLocation) === 'file'" :embedded="true" :workdir-path="currentWorkDir.path" :workdir-name="currentWorkDir.name" @run-command="runEditorCommand" />
                <BackgroundTasksPanel v-else-if="activeDockPanelFor(dockLocation) === 'tasks'" :embedded="true" :tasks="backgroundTaskList" @select-task="jumpToGroup" />
              </div>
            </div>
          </aside>

          <!-- 菜单 Teleport 到 body，避免被可横向滚动的标签栏裁掉。 -->
          <Teleport to="body">
            <div ref="dockAddMenuRef" v-if="showDockAddMenu" class="tool-dock-add-menu" :style="dockAddMenuStyle" @click.stop>
              <div class="tool-dock-add-heading">打开工具 · 自动停靠</div>
              <button
                v-for="option in dockPanelOptions"
                :key="option.key"
                class="tool-dock-add-item"
                @click="openDockPanelFromAddMenu(option.key)"
              >
                <span class="tool-dock-add-item-left">
                  <Icon :icon="option.icon" width="15" />
                  <span>{{ option.label }}</span>
                </span>
                <span class="tool-dock-add-meta">
                  <span class="tool-dock-add-badge">{{ dockPanelAddTargetLabel(option.key) }}</span>
                  <span v-if="option.shortcut" class="tool-dock-add-shortcut">{{ option.shortcut }}</span>
                </span>
              </button>
            </div>
          </Teleport>

          <!-- 拖动时才出现的 Windows 式停靠预览；目的地始终可见，即使该区域尚未创建。 -->
          <div v-if="draggedDockPanel" class="tool-dock-drop-overlay" aria-hidden="true">
            <div
              class="tool-dock-drop-target is-right"
              :class="{ active: dockDropTarget === 'right' }"
              @dragover.prevent.stop="onDockRegionDragOver('right')"
              @drop.prevent.stop="onDockPanelDrop('right')"
            >
              <Icon icon="mdi:dock-right" width="24" />
              <span>停靠到右侧</span>
            </div>
            <div
              class="tool-dock-drop-target is-bottom"
              :class="{ active: dockDropTarget === 'bottom' }"
              @dragover.prevent.stop="onDockRegionDragOver('bottom')"
              @drop.prevent.stop="onDockPanelDrop('bottom')"
            >
              <Icon icon="mdi:dock-bottom" width="24" />
              <span>停靠到底部</span>
            </div>
          </div>
        </div>
      </div>
      </div>

      <SettingsModal v-if="showSettings" @close="onSettingsClosed" />
      <ScheduledTaskManager
              v-if="showScheduledTaskManager"
              @close="showScheduledTaskManager = false"
              @create="openScheduledTaskCreate"
              @toast="showGitToast"
            />
            <!-- 通知面板 -->
                        <Teleport to="body">
                          <div v-if="showMailPanel" class="mail-panel-backdrop" @click.self="showMailPanel = false">
                            <div class="mail-panel" @click="markNotifRead">
                              <div class="mail-panel-head">
                                <h2><Icon icon="mdi:email-outline" width="20" /> 通知</h2>
                                <div class="mail-panel-actions">
                                  <button class="mail-panel-act-btn" type="button" @click.stop="markNotifRead" title="全部标为已读">
                                    <Icon icon="mdi:email-check-outline" width="16" />
                                  </button>
                                  <button class="mail-panel-close" type="button" @click="showMailPanel = false">
                                    <Icon icon="mdi:close" width="20" />
                                  </button>
                                </div>
                              </div>
                              <div class="mail-panel-body">
                                <div v-if="notifications.length === 0" class="mail-panel-empty">暂无通知</div>
                                <div v-for="n in notifications" :key="n.id" class="mail-notif-row" :class="{ unread: !n.is_read }">
                                  <div class="mail-notif-dot" v-if="!n.is_read"></div>
                                  <div class="mail-notif-content">
                                    <div class="mail-notif-title">{{ n.title }}</div>
                                    <div class="mail-notif-body" v-if="n.body">{{ n.body }}</div>
                                    <div class="mail-notif-meta">
                                      <span class="mail-notif-type">{{ notifTypeLabel(n.type) }}</span>
                                      <span class="mail-notif-time">{{ n.created_at }}</span>
                                    </div>
                                  </div>
                                </div>
                              </div>
                            </div>
                          </div>
                        </Teleport>
      <ScheduledTaskModal v-if="showScheduledTask" @close="closeScheduledTask" @create="onCreateScheduledTask" />
      <ModelManagerModal
              v-if="showModelManager"
              :free-models="freeModelsFull"
              :loading="false"
              @close="showModelManager = false"
              @add-provider="showModelManager = false; showSettings = true"
              @delete-key="onDeleteModelKey"
            />
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

      <!-- 工具审批已改成输入框上方的轻量条（见 .approval-bar），不再用打断式弹窗 -->
    </div>

    <!-- 插件市场浮层（占位入口）：独立组件 + Teleport，与 SettingsModal 保持一致 -->
    <PluginsMarketModal v-if="showPluginsPanel" @close="closePluginsPanel" />

    <!-- 彩虹 toast（技能习得 + 全局 showRainbowToast 触发；样式升级版） -->
    <RainbowToast />

  </div>
</template>

<script setup>
import { ref, watch, nextTick, computed, onMounted, onUnmounted, reactive } from 'vue'
import { Icon } from '@iconify/vue'
import hljs from 'highlight.js'
import 'highlight.js/styles/atom-one-dark.min.css'
import 'katex/dist/katex.min.css'
import { renderMarkdown, renderMermaidIn } from './markdownRenderer.js'
import { computeHardwareFingerprint } from '../../../utils/hardwareFingerprint.js'
import { streamFadeConfig } from '../composables/streamFadeConfig.js'
import { previewRequest } from '../composables/previewBus.js'
import UserMessageRail from './UserMessageRail.vue'
import { useChatWidget } from './useChatWidget.js'
import { useResizableHeight, useResizableWidth } from './useResizable.js'
import SessionList from './SessionList.vue'
import SessionMenuContent from './SessionMenuContent.vue'
import RainbowToast from './RainbowToast.vue'
import ScheduledTaskModal from './ScheduledTaskModal.vue'
import ScheduledTaskManager from './ScheduledTaskManager.vue'
import SettingsModal from './SettingsModal.vue'
import ModelManagerModal from './ModelManagerModal.vue'
import PluginsMarketModal from './PluginsMarketModal.vue'
import QuestionModal from './QuestionModal.vue'
import FileToolPanel from './FileToolPanel.vue'
import DiffPanel from './DiffPanel.vue'
import { parseToolArgs } from './toolArgs.js'
import Terminal from './Terminal.vue'
import { useAuth } from '../../../composables/useAuth.js'
import SnippetPanel from './SnippetPanel.vue'
import BackgroundTasksPanel from './BackgroundTasksPanel.vue'
import ResceneStatusIcon from './ResceneStatusIcon.vue'
import MessageStepGroup from './MessageStepGroup.vue'
import AgentWorkflowPanel from './AgentWorkflowPanel.vue'
import AttachmentChipRow from './AttachmentChipRow.vue'
import PreviewBrowser from './PreviewBrowser.vue'
import NewSessionHome from './NewSessionHome.vue'
import { hiddenModelIds, toggleHidden, syncHidden } from '../composables/modelVisibility.js'
import { contextBreakdown, loadContextBreakdown, setConversationTokens } from '../composables/contextBreakdown.js'
import { sessionTokenStats, loadSessionTokenStats } from '../composables/sessionTokenStats.js'

const props = defineProps({
  autoOpen: { type: Boolean, default: false },
  sessionId: { type: String, default: 'global_chat_session' }
})
const railAuth = useAuth()

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

// 已完成的工作流会话：工作流从活跃 → 停止时把当时活跃的会话记入集合
const completedSessions = ref(new Set())
// 有未决问题的会话：approval/resume/question 任一 pending 即认为该会话在提问
const questionSession = computed(() => {
  if (approvalState.pending.length > 0 || questionState.pending) {
    return activeSession.value
  }
  return ''
})

function shortTitle(title) {
  title = (title || '新对话').trim()
  return title.length > 24 ? title.slice(0, 24) + '…' : title
}

// AI 标题生成中的会话集合：/api/title/generate 请求已发出但还没回来期间，
// 列表刷新时保持当前显示名（「新对话」），别让后端派生的用户原文标题（「你好」）
// 抢先刷出来再被 AI 标题替换——那是标题跳动的元凶。请求结算（成功/失败/超时）即移除。
const pendingTitleSessions = new Set()

async function loadSessionList() {
  try {
    const res = await fetch('/api/sessions')
    const data = await res.json()
    // parentId/forkIndex 是侧栏拼分支树用的血缘（后端 SessionInfo 带下来，根会话为空）
    const real = (data || []).map(s => {
      const old = sessionList.value.find(x => x.id === s.id)
      let name = shortTitle(s.title)
      // AI 标题生成中：保持当前显示名，等 AI 标题到达由 updateSessionTitle 一次性替换
      if (pendingTitleSessions.has(s.id) && old) name = old.name
      return {
              id: s.id, name,
              parentId: s.parent_id || '', forkIndex: s.fork_index || 0, updatedAt: s.updated_at,
              workdir: getSessionWorkdir(s.id),
              titleExplicit: !!s.title_explicit
            }
    })
    // 当前会话哪怕还一条消息都没有（刚新建/刚打开应用）也要出现在列表里，
    // 不然侧栏在"发第一条消息之前"会看不到自己正在哪个会话上。
    // 注意别把刚分叉出来的分支覆盖成无名根——confirmEdit 已经乐观插入过带血缘的条目了
    if (!real.some(s => s.id === sessionId.value)) {
      const optimistic = sessionList.value.find(s => s.id === sessionId.value)
      real.unshift(optimistic || { id: sessionId.value, name: '新对话', parentId: '', forkIndex: 0, workdir: getSessionWorkdir(sessionId.value) })
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
  // 切换会话时工作目录跟随该会话所属项目：会话有明确归属且不在当前项目时才切，
  // recordSession:false 只切 cwd 不动会话归属（会话本来就属于目标项目）
  followSessionWorkdir(id)
}
// 会话行拖拽排序：本地乐观重排 sessionList（后端按 updatedAt 派生，刷新后恢复时间序；纯前端交互）
function reorderSessionList({ from, to }) {
  const list = [...sessionList.value]
  const fromIdx = list.findIndex(s => s.id === from)
  const toIdx = list.findIndex(s => s.id === to)
  if (fromIdx < 0 || toIdx < 0 || fromIdx === toIdx) return
  const [moved] = list.splice(fromIdx, 1)
  list.splice(toIdx, 0, moved)
  sessionList.value = list
}
async function followSessionWorkdir(id) {
  const target = sessionList.value.find(s => s.id === id)
  const wdName = target?.workdir || ''
  // 未分组会话：没有归属项目，清空当前工作目录（文件树随之清空/显示项目占位），
  // 不再沿用上一个项目——否则切到未分组会话，文件编辑器还停在旧项目（re0）上。
  if (!wdName) {
    if (currentWorkDir.value?.path) {
      currentWorkDir.value = { name: '', path: '' }
      saveWorkDirState()
    }
    return
  }
  if (currentWorkDir.value?.name === wdName) return
  const project = projects.value.find(p => p.name === wdName)
  if (!project) return
  await selectWorkDir({ name: project.name, path: project.path }, { recordSession: false })
}
// ==================== 工作目录 → 会话分组 ====================
const WD_MAP_KEY = 'shanxi_session_workdir'
const PROJECTS_KEY = 'shanxi_projects_v1'
const LEGACY_AUTO_PROJECTS = new Set(['re0', 'main-frontend'])

function normalizeProjectPath(path) {
  return String(path || '').replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase()
}
function readProjects() {
  try {
    const value = JSON.parse(localStorage.getItem(PROJECTS_KEY) || '[]')
    return Array.isArray(value)
      ? value.filter(p => p?.name?.trim() && p?.path).map(p => ({ name: p.name.trim(), path: p.path }))
      : []
  } catch { return [] }
}
const projects = ref(readProjects())
function saveProjects() {
  try { localStorage.setItem(PROJECTS_KEY, JSON.stringify(projects.value)) } catch {}
}
function rememberProject(project) {
  const name = project?.name?.trim()
  const path = project?.path
  if (!name || !path) return
  const normalized = normalizeProjectPath(path)
  // 项目身份只认路径，不认名字：两个不同路径哪怕撞名也各自独立存在，
  // 不然按名字去重会把旧项目的条目顶掉，导致它名下的历史会话被"过继"给新项目。
  projects.value = [
    { name, path },
    ...projects.value.filter(p => normalizeProjectPath(p.path) !== normalized)
  ].slice(0, 30)
  saveProjects()
}
function findProjectByPath(path) {
  const normalized = normalizeProjectPath(path)
  return projects.value.find(p => normalizeProjectPath(p.path) === normalized) || null
}

function loadWorkdirMapping() {
  try { return JSON.parse(localStorage.getItem(WD_MAP_KEY) || '{}') } catch { return {} }
}
function saveWorkdirMapping(m) {
  try { localStorage.setItem(WD_MAP_KEY, JSON.stringify(m)) } catch {}
}
function getSessionWorkdir(sid) {
  return loadWorkdirMapping()[sid] || ''
}
function recordSessionWorkdir(sid) {
  if (!sid) return
  // 与 newSession 同口径：只认显式创建过的项目，未选择不自动归组
  const wd = findProjectByPath(currentWorkDir.value?.path)?.name || ''
  if (!wd) return
  const m = loadWorkdirMapping()
  m[sid] = wd
  saveWorkdirMapping(m)
  sessionList.value = sessionList.value.map(s => s.id === sid ? { ...s, workdir: wd } : s)
}

// 旧版把开发机默认值 main-frontend 和仓库根名 re0 自动写进每个会话。
// 只清理由旧自动逻辑产生、且用户没有显式创建过的同名项目。
function migrateLegacyWorkdirMapping() {
  const explicitNames = new Set(projects.value.map(p => p.name))
  const m = loadWorkdirMapping()
  let changed = false
  for (const [sid, name] of Object.entries(m)) {
    if (LEGACY_AUTO_PROJECTS.has(name) && !explicitNames.has(name)) {
      delete m[sid]
      changed = true
    }
  }
  if (changed) saveWorkdirMapping(m)
}
migrateLegacyWorkdirMapping()

async function newSession(project) {
  // 从"选择项目"弹窗里显式选中了一个项目：先把工作目录切过去，再建会话，
  // 不依赖 currentWorkDir 是否已经指向它（未选中项目时才会走到这个分支）
  if (project?.path && normalizeProjectPath(project.path) !== normalizeProjectPath(currentWorkDir.value?.path)) {
    await selectWorkDir(project, { recordSession: false })
  }
  const id = 'sess_' + Date.now().toString(36)
  // 只有显式创建过的项目才归组；后端启动目录等未选目录不自动挂名（避免凭空冒出 re0 之类），
  // 新建对话保持未分组（home 空态），用户显式选择目录后由 selectWorkDir 归入项目
  const workdir = findProjectByPath(currentWorkDir.value?.path)?.name || ''
  sessionList.value = [{ id, name: '新对话', parentId: '', forkIndex: 0, workdir }, ...sessionList.value]
  recordSessionWorkdir(id)
  switchSession(id)
}


function updateSessionTitle(title, fallback, sid) {
  // 标题请求已结算（成功/失败/超时），解除「AI 标题生成中保持显示名」的保护
  if (sid) pendingTitleSessions.delete(sid)
  if (!title) return
  // 精确作用到发起标题生成的会话：用户可能在生成期间切到别的会话，标题不能安错家
  const targetId = sid || sessionId.value
  const current = sessionList.value.find(s => s.id === targetId)
  // 只覆盖「未显式设置过标题」的会话：新对话、或后端派生标题（用户首条消息原文）。
  // 2026-09-03 修：原来用 name === shortTitle(fallback) 比对「当前标题是否等于用户消息最后
  // 一行」来判断是否是默认标题——多行/长消息时侧栏标题是全文截断，永远对不上，AI 标题
  // 生成了也写不进去（用户实测：短消息如「你好」能生效，其他全原样）。改为后端下发的
  // title_explicit 标记：用户手动改名过 / AI 标题已写入才算显式，其余一律允许覆盖。
  const isDefault = !current || !current.titleExplicit
  if (!isDefault) return
  const trimmed = (title || '').trim()
  if (!trimmed) return
  const safe = shortTitle(trimmed)
  const prev = current?.name || ''
  if (prev === safe) return
  sessionList.value = sessionList.value.map(s => s.id === targetId ? { ...s, name: safe } : s)
  fetch(`/api/sessions/${encodeURIComponent(targetId)}/title`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title: trimmed })
  }).catch(() => {})
}
function renameSession({ id, name }) {
  const target = sessionList.value.find(s => s.id === id)
  if (target) {
    target.name = shortTitle(name)
    // 用户手动改名 = 显式标题：之后 AI 标题生成不得再覆盖它
    target.titleExplicit = true
  }
  fetch(`/api/sessions/${encodeURIComponent(id)}/title`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title: name })
  }).catch(() => {})
}
async function deleteSession(id) {
  try {
    await fetch(`/api/sessions/${id}`, { method: 'DELETE' })
  } catch (e) {
    console.warn('删除会话失败', e)
  }
  // 清理 workdir 映射
  const wm = loadWorkdirMapping()
  delete wm[id]
  saveWorkdirMapping(wm)
  // 重新拉而不是本地 filter：被删会话的分支在后端会被提升为根会话，
  // 本地 filter 的话那些分支还挂着指向已删父会话的 parentId，在树里会变成孤儿。
  // 先切走当前会话再拉列表：loadSessionList 有"当前会话必须出现在列表里"的
  // 保护逻辑，若删的是当前会话而没先切换，它会把已删会话又插回列表顶部
  if (activeSession.value === id) {
    const next = sessionList.value.find(s => s.id !== id)?.id || ('sess_' + Date.now().toString(36))
    switchSession(next)
  }
  await loadSessionList()
}
async function deleteSessions(ids) {
  for (const id of ids) {
    try {
      await fetch(`/api/sessions/${id}`, { method: 'DELETE' })
    } catch (e) {
      console.warn('删除会话失败', id, e)
    }
    const wm = loadWorkdirMapping()
    delete wm[id]
    saveWorkdirMapping(wm)
  }
  // 先切走当前会话再拉列表：loadSessionList 有"当前会话必须出现在列表里"的
  // 保护逻辑，若删的是当前会话而没先切换，它会把已删会话又插回列表顶部
  if (ids.includes(activeSession.value)) {
    const next = sessionList.value.find(s => !ids.includes(s.id))?.id || ('sess_' + Date.now().toString(36))
    switchSession(next)
  }
  await loadSessionList()
}
// 删除整个项目：其下所有会话 + 项目实体 + workdir 归属映射 + 置顶记录
async function deleteProject(name) {
  const ids = sessionList.value.filter(s => s.workdir === name).map(s => s.id)
  if (ids.length) await deleteSessions(ids)
  // 项目实体：从 projects 移除（workdirMap 靠它撑起空项目分组，不删的话侧栏会留下空壳）
  projects.value = projects.value.filter(p => p.name !== name)
  saveProjects()
  // 归属映射：清掉所有指向该项目的会话记录
  const m = loadWorkdirMapping()
  let changed = false
  for (const [sid, wd] of Object.entries(m)) {
    if (wd === name) { delete m[sid]; changed = true }
  }
  if (changed) saveWorkdirMapping(m)
  // 置顶记录
  try {
    const pinned = JSON.parse(localStorage.getItem('shanxi_pinned_projects') || '[]')
    const next = pinned.filter(p => p !== name)
    if (next.length !== pinned.length) localStorage.setItem('shanxi_pinned_projects', JSON.stringify(next))
  } catch {}
  // 若删除的正是当前工作目录，重置为空（避免工作目录条指着已删除项目）
  if (currentWorkDir.value?.name === name) {
    currentWorkDir.value = { name: '', path: '' }
    saveWorkDirState()
  }
  await loadSessionList()
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
// 底部横条与展开面板共用这一个口径（分类之和 ≈ 真实 prompt_tokens）。
// 之前横条用的是 input+output、面板用分类之和，两套口径必然对不上。
const ctxTotalUsed = computed(() => {
  const sum = ctxRows.value.reduce((s, r) => s + r.tokens, 0)
  if (sum > 0) return sum
  // 没有分类明细的老会话（早于 context_breakdown 上线）：退回持久化的会话级 token，
  // 否则横条会从"有数"变成 0/0
  const p = sessionTokenStats.value
  return (p?.inputTokens || 0) + (p?.outputTokens || 0)
})
const ctxWindow = computed(() => contextBreakdown.value.contextWindow || sessionTokenStats.value?.contextWindow || currentCapability.value.context_window || 0)
const ctxPct = computed(() => ctxWindow.value > 0 ? Math.min((ctxTotalUsed.value / ctxWindow.value) * 100, 100) : 0)
// 水位：<=35% 固定 35%，>35% 弹性映射
const MIN_LEVEL = 35
const ctxLevel = computed(() => ctxPct.value <= MIN_LEVEL ? MIN_LEVEL : ctxPct.value)
const ctxWaterY = computed(() => 24 - ctxLevel.value * 0.2)
const ctxWaterH = computed(() => ctxLevel.value * 0.2 + 2)
// 随机水波：三层不同频率/振幅的正弦叠加，用 t 驱动相位偏移模拟流动
const waveT = ref(0)
let waveRaf = 0
function animateWave() {
  waveT.value = Date.now() * 0.001
  waveRaf = requestAnimationFrame(animateWave)
}
onMounted(() => animateWave())
onUnmounted(() => cancelAnimationFrame(waveRaf))

const ctxWavePath = computed(() => {
  const wy = ctxWaterY.value
  const t = waveT.value
  // 三层极浅涟漪，amplitude 极小使水面近乎水平
  const layers = [
    { amp: 0.15, freq: 1.4, speed: 0.6 },
    { amp: 0.1,  freq: 2.3, speed: -0.9 },
    { amp: 0.05, freq: 3.5, speed: 1.3 },
  ]
  let d = `M -2 ${wy}`
  for (let x = -2; x <= 26; x += 0.4) {
    let y = wy
    for (const l of layers) {
      y += Math.sin(x * l.freq + t * l.speed) * l.amp
    }
    d += ` L ${x} ${y}`
  }
  d += ` L 26 26 L -2 26 Z`
  return d
})

// ==================== 模型能力（识图 / 上下文窗口 / 是否支持思考强度） ====================
// 免费池模型的能力元数据是静态已知的（后端 freeModelCatalog），开工前就能查到；
// DeepSeekProxy / Cloud / 自定义配置这些没有静态元数据，退回最近一次真实工作流
// 的 model_info 回填——第一次用之前不知道，用过一次就记住了。
const modelCapabilities = ref({}) // { [modelId]: {vision, context_window, reasoning} }
// 后端 catalog 的 id→显示名映射，供下拉框在只有裸 id（持久化的选择）时复原标签
const modelLabels = ref({}) // { [modelId]: 显示名 }
// 全量免费模型目录（含 vendor 字段），用于聊天下拉按提供方分组渲染
const freeModelsFull = ref([]) // [{ id, vendor, name, ... }]
const sharedPoolModelIds = ref(new Set()) // 共享池模型的 ID 集合
const sharedPoolQuota = ref(null) // { used, limit, remaining } 从云端返回
async function loadModelCapabilities() {
  try {
    const res = await fetch('/api/models/config')
    const data = await res.json()
    const map = {}
    const labels = {}
    freeModelsFull.value = [...(data.free_models || []), ...(data.custom_models || [])]
    for (const fm of freeModelsFull.value) {
      map[fm.id] = { vision: fm.vision, context_window: fm.context_window, reasoning: fm.reasoning }
      labels[fm.id] = fm.name
    }
    modelCapabilities.value = map
    modelLabels.value = labels
  } catch (e) {
    console.warn('加载模型能力失败', e)
  }
  // 同时拉取共享池模型（免费试用）
  try {
    const spRes = await fetch('/api/models/shared-pool')
    if (spRes.ok) {
      const spData = await spRes.json()
      const spModels = spData.free_models || []
      const spIds = new Set()
      for (const sp of spModels) {
          const idx = freeModelsFull.value.findIndex(m => m.id === sp.id)
          const localModel = idx >= 0 ? freeModelsFull.value[idx] : null
          // 本地优先：用户已配自己 key 或免 key（如 OpenCode Zen）→ 不覆盖，不入共享池
          if (localModel && (localModel.api_key_set || localModel.keyless)) {
            continue
          }
          spIds.add(sp.id)
          const modelWithFlag = { ...sp, sharedPool: true, api_key_set: true }
          if (idx >= 0) {
            freeModelsFull.value[idx] = modelWithFlag
          } else {
            freeModelsFull.value.push(modelWithFlag)
          }
          modelCapabilities.value[sp.id] = { vision: sp.vision, context_window: sp.context_window, reasoning: sp.reasoning }
          modelLabels.value[sp.id] = sp.name
        }
        sharedPoolModelIds.value = spIds
        sharedPoolQuota.value = spData.quota || null
      }
    } catch (e) {
      // 共享池不可用时不阻塞（用户可能没登录）
      console.warn('共享池不可用', e)
    }
}
onMounted(() => {
  loadModelCapabilities()
  window.addEventListener('model-config-changed', loadModelCapabilities)
})
onUnmounted(() => window.removeEventListener('model-config-changed', loadModelCapabilities))
// 刷新时从 localStorage 恢复当前会话的上下文分类占用（不丢）
loadContextBreakdown(localStorage.getItem('prism_session_id') || '')
// 刷新时恢复当前会话的真实 input/output token（不丢，横条靠它显示实际值）
sessionTokenStats.value = loadSessionTokenStats(localStorage.getItem('prism_session_id') || '')

const lastUserMessageId = computed(() => {
  for (let i = messages.value.length - 1; i >= 0; i--) {
    const m = messages.value[i]
    if (m.sender === 'user' && (m.content || '').trim()) return m.id
  }
  return ''
})
const currentCapability = computed(() => {
  return modelCapabilities.value[selectedModel.value]
    || { vision: false, context_window: 0, reasoning: false }
})

// 注：原来这里还有个 liveContextStats（input+output 口径）专供底部横条，
// 跟面板的分类之和是两套对不上的口径。现已统一到 ctxTotalUsed / ctxWindow / ctxPct，
// 该 computed 随之删除。

// ==================== 工具工作区：右侧 + 底部双停靠 ====================
// 只开放两个稳定的停靠方向：文件/预览适合右侧，终端/任务适合底部。
// 这是 VS Code/Windows 工作台里最适合教学录屏的一组布局，不做会遮挡代码的任意浮窗。
// v4 修正底部 Dock 的“＋”把所有工具都污染成底部位置的问题；升级一次存储键，
// 让被旧逻辑挤到底部的文件/预览/Diff 回到适合编码的课堂默认值。
const DOCK_LAYOUT_STORAGE_KEY = 'rescene_tool_dock_layout_v4'
const DEFAULT_DOCK_PLACEMENT = {
  diff: 'right',
  terminal: 'bottom',
  preview: 'right',
  file: 'right',
  tasks: 'bottom'
}
function loadDockPlacement() {
  try {
    const parsed = JSON.parse(localStorage.getItem(DOCK_LAYOUT_STORAGE_KEY) || '{}')
    const placement = parsed?.placement || {}
    return Object.fromEntries(
      Object.keys(DEFAULT_DOCK_PLACEMENT).map(key => [
        key,
        placement[key] === 'bottom' ? 'bottom' : (placement[key] === 'right' ? 'right' : DEFAULT_DOCK_PLACEMENT[key])
      ])
    )
  } catch {
    return { ...DEFAULT_DOCK_PLACEMENT }
  }
}
const dockPanels = ref([])
const dockPlacement = reactive(loadDockPlacement())
const activeDockPanels = reactive({ right: '', bottom: '' })
const dockWidth = ref(420)
const dockHeight = ref(240)
// 右侧尽量可拉宽，但仍给侧栏和聊天区留出可操作空间。
const dockMaxWidth = () => Math.max(720, window.innerWidth - 420)
const dockMaxHeight = () => Math.max(220, Math.min(620, window.innerHeight - 260))
const { startDrag: startDockWidthDrag } = useResizableWidth(dockWidth, {
  min: 320,
  max: dockMaxWidth,
  edge: 'left',
  persistKey: 'dockWidth'
})
const { startDrag: startDockHeightDrag } = useResizableHeight(dockHeight, {
  min: 160,
  max: dockMaxHeight,
  edge: 'top',
  persistKey: 'dockHeightV2'
})
const rightDockPanels = computed(() => dockPanels.value.filter(key => dockPlacement[key] !== 'bottom'))
const bottomDockPanels = computed(() => dockPanels.value.filter(key => dockPlacement[key] === 'bottom'))
const visibleDockLocations = computed(() => {
  if (!isExpanded.value) return []
  const locations = []
  if (rightDockPanels.value.length && !dockHidden.right) locations.push('right')
  if (bottomDockPanels.value.length && !dockHidden.bottom) locations.push('bottom')
  return locations
})
const dockHidden = reactive({ right: false, bottom: false })
const dockExpandedLocation = ref('')
const dockExpanded = computed(() => Boolean(dockExpandedLocation.value))
const hasVisibleDockPanels = computed(() => (
  (rightDockPanels.value.length > 0 && !dockHidden.right)
  || (bottomDockPanels.value.length > 0 && !dockHidden.bottom)
))
const draggedDockPanel = ref('')
const draggedDockSource = ref('')
const dockDropTarget = ref('')
const showDockAddMenu = ref(false)
const dockAddLocation = ref('right')
const dockAddMenuRef = ref(null)
const dockAddMenuStyle = ref({})

function dockPanelsFor(location) {
  return location === 'bottom' ? bottomDockPanels.value : rightDockPanels.value
}
function activeDockPanelFor(location) {
  return activeDockPanels[location] || ''
}
function dockIsHidden(location) {
  return Boolean(dockHidden[location])
}
function dockIsExpanded(location) {
  return dockExpandedLocation.value === location
}
function dockPanelStyle(location) {
  if (dockIsHidden(location) || dockIsExpanded(location)) return {}
  return location === 'bottom'
    ? { height: dockHeight.value + 'px' }
    : { width: dockWidth.value + 'px' }
}
function persistDockPlacement() {
  try {
    localStorage.setItem(DOCK_LAYOUT_STORAGE_KEY, JSON.stringify({ placement: { ...dockPlacement } }))
  } catch {}
}

// ==================== 终端标签组 ====================
let terminalSeq = 0
const terminalTabs = ref([{ id: 'term_' + Date.now().toString(36), name: '终端 1' }])
const activeTerminalId = ref(terminalTabs.value[0].id)
function addTerminalTab() {
  terminalSeq++
  const id = 'term_' + Date.now().toString(36) + terminalSeq
  const tab = { id, name: '终端 ' + (terminalTabs.value.length + 1) }
  terminalTabs.value = [...terminalTabs.value, tab]
  activeTerminalId.value = tab.id
}
function closeTerminalTab(id) {
  if (terminalTabs.value.length <= 1) return
  const idx = terminalTabs.value.findIndex(t => t.id === id)
  terminalTabs.value = terminalTabs.value.filter(t => t.id !== id)
  if (activeTerminalId.value === id) {
    activeTerminalId.value = terminalTabs.value[Math.min(idx, terminalTabs.value.length - 1)].id
  }
}
// shortcut 只标真的注册了全局快捷键的（见下面 onGlobalDockShortcut）——不给
// Diff/任务挂一个中看不中用的提示文字，那是纯粹的视觉谎言。
const DOCK_PANEL_META = {
  diff: { label: 'Diff', icon: 'proicons:diff' },
  terminal: { label: '终端', icon: 'ri:terminal-line', shortcut: 'Ctrl+J' },
  preview: { label: '预览', icon: 'mage:preview', shortcut: 'Ctrl+Shift+B' },
  file: { label: '文件', icon: 'mdi:file-code-outline', shortcut: 'Ctrl+G' },
  tasks: { label: '任务', icon: 'mdi:task-minus' }
}
const dockPanelOptions = Object.entries(DOCK_PANEL_META).map(([key, meta]) => ({ key, ...meta }))
function dockPanelLabel(key) { return DOCK_PANEL_META[key]?.label || key }
function dockPanelIcon(key) { return DOCK_PANEL_META[key]?.icon || 'mdi:application-outline' }

// 真·全局快捷键，跟菜单上标的提示一一对应——Ctrl+G/Ctrl+J 在 Monaco 编辑器里
// 有原生含义（跳转行/…），焦点在输入框或编辑器里时让开，不抢它们的按键。
function onGlobalDockShortcut(e) {
  if (!(e.ctrlKey || e.metaKey)) return
  const t = e.target
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable || t.closest?.('.monaco-editor'))) return
  const key = e.key.toLowerCase()
  if (key === 'g' && !e.shiftKey) { e.preventDefault(); openDockPanel('file') }
  else if (key === 'j' && !e.shiftKey) { e.preventDefault(); openDockPanel('terminal') }
  else if (key === 'b' && e.shiftKey) { e.preventDefault(); openDockPanel('preview') }
}
function ensureActiveDockPanels() {
  for (const location of ['right', 'bottom']) {
    const panels = dockPanelsFor(location)
    if (!panels.includes(activeDockPanels[location])) {
      activeDockPanels[location] = panels[panels.length - 1] || ''
    }
    if (!panels.length) dockHidden[location] = false
  }
  if (!dockPanels.value.length) showDockAddMenu.value = false
}
function setActiveDockPanel(key, location = dockPlacement[key] || DEFAULT_DOCK_PLACEMENT[key] || 'right') {
  activeDockPanels[location] = key
  dockHidden[location] = false
}
const showSnippet = ref(false)
const snippetInsertCmd = ref('')
function toggleDockExpanded(location) {
  dockExpandedLocation.value = dockExpandedLocation.value === location ? '' : location
}
function toggleDockHidden(location) {
  dockHidden[location] = !dockHidden[location]
  if (dockHidden[location] && dockExpandedLocation.value === location) dockExpandedLocation.value = ''
}
function onSnippetInsert(cmd) {
  // 先清空再设值，确保 watcher 即使值相同也会触发
  snippetInsertCmd.value = ''
  nextTick(() => { snippetInsertCmd.value = cmd })
}
function runEditorCommand(cmd) {
  // 文件面板的一键运行复用真实常驻终端，而不是再造一个只显示假日志的“运行窗口”。
  openDockPanel('terminal')
  onSnippetInsert(cmd)
}
function moveDockPanel(key, location, beforeKey = '') {
  const previousLocation = dockPlacement[key] || DEFAULT_DOCK_PLACEMENT[key] || 'right'
  let ordered = dockPanels.value.filter(panelKey => panelKey !== key)
  const beforeIndex = beforeKey && beforeKey !== key ? ordered.indexOf(beforeKey) : -1
  if (beforeIndex >= 0) ordered.splice(beforeIndex, 0, key)
  else ordered.push(key)
  dockPanels.value = ordered
  dockPlacement[key] = location
  activeDockPanels[location] = key
  dockHidden[location] = false
  if (previousLocation !== location) ensureActiveDockPanels()
  persistDockPlacement()
}
function openDockPanel(key, location = dockPlacement[key] || DEFAULT_DOCK_PLACEMENT[key] || 'right') {
  moveDockPanel(key, location)
  showDockAddMenu.value = false
}
// Dock 内的“＋”是全局工具入口，不是“添加到当前 Dock”。无论从底部还是右侧打开，
// 都按工具类型进入推荐位置，避免终端打开后文件/预览/Diff 被迫挤在底部。
function openDockPanelFromAddMenu(key) {
  openDockPanel(key, DEFAULT_DOCK_PLACEMENT[key] || 'right')
}
function toggleDockPanel(key) {
  const location = dockPlacement[key] || DEFAULT_DOCK_PLACEMENT[key] || 'right'
  if (dockHidden[location] && dockPanels.value.includes(key)) {
    setActiveDockPanel(key, location)
    return
  }
  if (activeDockPanels[location] === key && dockPanels.value.includes(key)) {
    closeDockPanel(key)
    return
  }
  openDockPanel(key, location)
}
function closeDockPanel(key) {
  const location = dockPlacement[key] || DEFAULT_DOCK_PLACEMENT[key] || 'right'
  dockPanels.value = dockPanels.value.filter(k => k !== key)
  ensureActiveDockPanels()
  if (!dockPanels.value.length) {
    dockExpandedLocation.value = ''
    dockHidden.right = false
    dockHidden.bottom = false
  } else if (!dockPanelsFor(location).length && dockExpandedLocation.value === location) {
    dockExpandedLocation.value = ''
  }
}
function dockPanelPlacementLabel(key) {
  return dockPlacement[key] === 'bottom' ? '底部' : '右侧'
}
function dockPanelAddTargetLabel(key) {
  const target = DEFAULT_DOCK_PLACEMENT[key] || 'right'
  const targetLabel = target === 'bottom' ? '底部' : '右侧'
  if (!dockPanels.value.includes(key)) return targetLabel
  const current = dockPlacement[key] || target
  return current === target ? `已在${targetLabel}` : `移到${targetLabel}`
}
function onDockTabDragStart(event, key, location) {
  draggedDockPanel.value = key
  draggedDockSource.value = location
  dockDropTarget.value = location
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', key)
  }
}
function onDockTabDragOver(location) {
  dockDropTarget.value = location
}
function onDockRegionDragOver(location) {
  if (draggedDockPanel.value) dockDropTarget.value = location
}
function clearDockDragState() {
  draggedDockPanel.value = ''
  draggedDockSource.value = ''
  dockDropTarget.value = ''
}
function onDockPanelDrop(location, beforeKey = '') {
  const key = draggedDockPanel.value
  if (!key) return
  if (key === beforeKey && draggedDockSource.value === location) {
    setActiveDockPanel(key, location)
  } else {
    moveDockPanel(key, location, beforeKey)
  }
  clearDockDragState()
}
function onDockTabDragEnd() {
  clearDockDragState()
}
// 菜单 Teleport 到 body 后使用视口 fixed 坐标。必须等它真正渲染出来再读取高度：
// 优先放在按钮下方，放不下就翻到上方；两边都不够时选空间更大的一侧，并把菜单
// 限制在剩余空间内滚动，避免低窗口/底部 Dock 场景越过屏幕边缘。
function toggleDockAddMenu(event, location) {
  const shouldOpen = !showDockAddMenu.value || dockAddLocation.value !== location
  showDockAddMenu.value = shouldOpen
  dockAddLocation.value = location
  const button = event?.currentTarget
  if (shouldOpen && button) {
    const anchor = button.getBoundingClientRect()
    const safe = 8
    const gap = 6
    const viewportW = window.innerWidth
    const viewportH = window.innerHeight
    const menuW = Math.min(200, Math.max(1, viewportW - safe * 2))

    // 先隐身测量，避免菜单用默认坐标闪到屏幕外一帧。
    dockAddMenuStyle.value = {
      top: `${safe}px`,
      left: `${safe}px`,
      width: `${menuW}px`,
      maxHeight: `${Math.max(1, viewportH - safe * 2)}px`,
      visibility: 'hidden'
    }
    nextTick(() => {
      if (!showDockAddMenu.value || !dockAddMenuRef.value) return
      const naturalHeight = dockAddMenuRef.value.getBoundingClientRect().height
      const belowTop = anchor.bottom + gap
      const aboveBottom = anchor.top - gap
      const belowSpace = Math.max(0, viewportH - safe - belowTop)
      const aboveSpace = Math.max(0, aboveBottom - safe)
      const placeBelow = naturalHeight <= belowSpace || (naturalHeight > aboveSpace && belowSpace >= aboveSpace)
      const availableHeight = placeBelow ? belowSpace : aboveSpace
      const menuH = Math.min(naturalHeight, Math.max(1, availableHeight))
      const top = placeBelow ? belowTop : Math.max(safe, aboveBottom - menuH)
      const unclampedLeft = anchor.right - menuW
      const maxLeft = Math.max(safe, viewportW - safe - menuW)
      const left = Math.min(Math.max(safe, unclampedLeft), maxLeft)

      dockAddMenuStyle.value = {
        top: `${top}px`,
        left: `${left}px`,
        width: `${menuW}px`,
        maxHeight: `${menuH}px`,
        visibility: 'visible'
      }
    })
  }
}

// agent 改了前端文件 → 后端推 preview_open → 这里把预览面板挂进 dock。
// 刻意不用 toggleDockPanel：那是"开关"语义，面板已经开着的时候会被它关掉，
// 正好跟"自动打开"相反。导航到具体地址由 PreviewBrowser 自己 watch 同一个源。
watch(() => previewRequest.seq, () => {
  openDockPanel('preview')
})

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
const currentWorkDir = ref({ name: '', path: '' })
// 后端启动目录/未选目录不算"已选中项目"——只有用户显式创建过的项目才数，
// 用来给"新建会话"决定要不要强制弹项目选择
const currentProjectName = computed(() => findProjectByPath(currentWorkDir.value?.path)?.name || '')
const workDirRecents = ref([])
const showWorkDirMenu = ref(false)
const workDirMenuView = ref('recent') // 'recent' | 'browse'
const workDirBrowseOptions = ref([])
const workDirBrowseLoading = ref(false)
const workDirSwitching = ref(false)
const workDirFolderInputRef = ref(null)

// ==================== AgentFS 会话快照树 ====================
const agentFSTimeline = ref([])
const agentFSLoading = ref(false)
const selectedAgentFSSnapshot = ref(null)
const agentFSDiffRaw = ref('')
const agentFSDiffLoading = ref(false)
const agentFSDiffError = ref('')
const agentFSDiffCardStyle = ref({ top: '120px', left: '76px' })
const agentFSTreeRef = ref(null)
const agentFSViewportHeight = ref(620)
let agentFSPollTimer = null
let boundAgentFSKey = ''
const agentFSGraphWidth = 260
const gitGraph = ref({ commits: [], current_branch: '' })

function syncAgentFSTreeViewport() {
  const height = agentFSTreeRef.value?.clientHeight
  if (height) agentFSViewportHeight.value = Math.max(180, Math.floor(height))
}

// AgentFS 审计记录携带真实 branch / parent_commit。旧记录没有这两个字段时，
// 按历史顺序兼容成 main 单主干，不伪造额外分叉。
// Git 图的画布按提交数自然增高，外层树容器负责滚动，避免滚轮落到聊天区。
const agentFSGraphHeight = computed(() => Math.max(180, agentFSViewportHeight.value - 8, gitGraph.value.commits.length * 42 + 60))
const agentFSGraphNodes = computed(() => {
  const ordered = [...agentFSTimeline.value].sort((a, b) => (b.seq || 0) - (a.seq || 0))
  const spacing = Math.min(48, Math.max(30, (agentFSGraphHeight.value - 70) / Math.max(ordered.length + 1, 2)))
  const branchNames = [...new Set(ordered.map(item => item.branch || 'main'))]
  const laneGap = 30
  const firstLaneX = Math.round(agentFSGraphWidth / 2 - ((branchNames.length - 1) * laneGap) / 2)
  const latestByBranch = new Set()
  return ordered.map((snapshot, index) => {
    const name = (snapshot.rel_path || '').split('/').pop() || snapshot.rel_path
    const branch = snapshot.branch || 'main'
    const lane = Math.max(0, branchNames.indexOf(branch))
    const centerX = firstLaneX + lane * laneGap
    const isLeaf = !latestByBranch.has(branch)
    latestByBranch.add(branch)
    return {
      snapshot,
      index,
      branch,
      isLeaf,
      isBranch: branch !== 'main' || branchNames.length > 1,
      hue: (330 + lane * 67) % 360,
      centerX,
      x: centerX - 12,
      y: 28 + index * spacing,
      shortName: name.length > 15 ? `${name.slice(0, 12)}…` : name
    }
  })
})
const agentFSGraphLinks = computed(() => {
  const nodes = agentFSGraphNodes.value
  if (!nodes.length) return []
  const byCommit = new Map(nodes.map(node => [node.snapshot.commit, node]))
  return nodes.flatMap((node, index) => {
    let parent = node.snapshot.parent_commit ? byCommit.get(node.snapshot.parent_commit) : null
    // 旧数据没有 parent_commit：退化连接到下一条更旧记录。
    if (!parent && !node.snapshot.parent_commit) parent = nodes[index + 1]
    if (!parent) return []
    const x1 = node.centerX
    const y1 = node.y + 12
    const x2 = parent.centerX
    const y2 = parent.y + 12
    const midY = Math.round((y1 + y2) / 2)
    return [{
      key: `${node.snapshot.commit}-${parent.snapshot.commit}`,
      d: x1 === x2
        ? `M ${x1} ${y1} L ${x2} ${y2}`
        : `M ${x1} ${y1} C ${x1} ${midY}, ${x2} ${midY}, ${x2} ${y2}`,
      trunk: node.branch === 'main' && parent.branch === 'main',
      hue: node.hue
    }]
  })
})

const gitGraphNodes = computed(() => {
  const lanes = []
  return gitGraph.value.commits.map((commit, index) => {
    let lane = lanes.indexOf(commit.hash)
    if (lane < 0) { lane = 0; lanes.unshift(commit.hash) }
    const parents = commit.parents || []
    if (parents.length) lanes[lane] = parents[0]; else lanes.splice(lane, 1)
    for (const parent of parents.slice(1)) if (!lanes.includes(parent)) lanes.push(parent)
    const centerX = Math.round(agentFSGraphWidth / 2 - ((Math.max(lanes.length, lane + 1) - 1) * 28) / 2 + lane * 28)
    return { commit, index, centerX, x: centerX - 12, y: 24 + index * 42, hue: (210 + lane * 62) % 360,
      label: (commit.branches || []).join(', ') || `${commit.hash.slice(0, 7)} · ${commit.subject}` }
  })
})
const gitGraphLinks = computed(() => {
  const nodes = gitGraphNodes.value, byHash = new Map(nodes.map(n => [n.commit.hash, n]))
  return nodes.flatMap(node => (node.commit.parents || []).flatMap(hash => {
    const parent = byHash.get(hash); if (!parent) return []
    const y1 = node.y + 12, y2 = parent.y + 12, mid = Math.round((y1 + y2) / 2)
    return [{ key: `${node.commit.hash}-${hash}`, trunk: node.centerX === parent.centerX, hue: node.hue, d: `M ${node.centerX} ${y1} C ${node.centerX} ${mid}, ${parent.centerX} ${mid}, ${parent.centerX} ${y2}` }]
  }))
})

const agentFSDiffLines = computed(() => {
  let oldLine = 0
  let newLine = 0
  const result = []
  for (const text of agentFSDiffRaw.value.split('\n')) {
    const hunk = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/.exec(text)
    if (hunk) {
      oldLine = Number(hunk[1])
      newLine = Number(hunk[2])
      result.push({ text: `区块 · 原行 ${oldLine} → 新行 ${newLine}`, number: '··', kind: 'hunk' })
      continue
    }
    if (text.startsWith('+++') || text.startsWith('---') || text.startsWith('diff ') ||
        text.startsWith('index ') || text.startsWith('commit ') || text.startsWith('Author:') ||
        text.startsWith('Date:') || (!oldLine && !newLine)) continue
    if (text.startsWith('+')) {
      result.push({ text: text.slice(1), number: newLine++, kind: 'added' })
    } else if (text.startsWith('-')) {
      result.push({ text: text.slice(1), number: oldLine++, kind: 'removed' })
    } else {
      result.push({ text: text.startsWith(' ') ? text.slice(1) : text, number: newLine || oldLine, kind: 'context' })
      oldLine++
      newLine++
    }
  }
  return result
})

const agentFSDiffStats = computed(() => ({
  added: agentFSDiffLines.value.filter(line => line.kind === 'added').length,
  removed: agentFSDiffLines.value.filter(line => line.kind === 'removed').length
}))

function formatAgentFSTime(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const today = new Date()
  const sameDay = date.toDateString() === today.toDateString()
  return sameDay
    ? date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    : date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

async function bindAgentFSSession() {
  if (!activeSession.value || !currentWorkDir.value?.path) return false
  const key = `${activeSession.value}\u0000${currentWorkDir.value.path}`
  if (boundAgentFSKey === key) return true
  try {
    const res = await fetch('/api/agentfs/open', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        project: currentWorkDir.value.name,
        workdir: currentWorkDir.value.path,
        session_id: activeSession.value
      })
    })
    if (res.ok) boundAgentFSKey = key
    return res.ok
  } catch {
    return false
  }
}

async function refreshAgentFSTimeline() {
  if (!activeSession.value) return
  agentFSLoading.value = true
  try {
    await bindAgentFSSession()
    const params = new URLSearchParams({
      project: currentWorkDir.value.name,
      session_id: activeSession.value
    })
    const res = await fetch(`/api/agentfs/log?${params}`)
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || `读取失败 (${res.status})`)
    agentFSTimeline.value = (Array.isArray(data.log) ? data.log : []).slice().reverse().slice(0, 18)
    if (selectedAgentFSSnapshot.value &&
        !agentFSTimeline.value.some(item => item.commit === selectedAgentFSSnapshot.value.commit)) {
      closeAgentFSDiff()
    }
  } catch (err) {
    console.warn('读取 AgentFS 会话时间线失败', err)
    agentFSTimeline.value = []
  } finally {
    agentFSLoading.value = false
  }
}

async function openAgentFSSnapshot(snapshot, event) {
  selectedAgentFSSnapshot.value = snapshot
  agentFSDiffRaw.value = ''
  agentFSDiffError.value = ''
  const rect = event.currentTarget.getBoundingClientRect()
  const cardWidth = Math.min(620, window.innerWidth - 100)
  const cardHeight = Math.min(560, window.innerHeight - 32)
  agentFSDiffCardStyle.value = {
    left: `${Math.min(rect.right + 18, window.innerWidth - cardWidth - 16)}px`,
    top: `${Math.max(16, Math.min(rect.top - 88, window.innerHeight - cardHeight - 16))}px`,
    width: `${cardWidth}px`,
    maxHeight: `${cardHeight}px`
  }
  agentFSDiffLoading.value = true
  try {
    const res = await fetch('/api/agentfs/diff', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ project: currentWorkDir.value.name, seq: snapshot.seq })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || `Diff 读取失败 (${res.status})`)
    agentFSDiffRaw.value = data.diff || ''
  } catch (err) {
    agentFSDiffError.value = err.message || '无法读取该快照'
  } finally {
    agentFSDiffLoading.value = false
  }
}

function closeAgentFSDiff() {
  selectedAgentFSSnapshot.value = null
  agentFSDiffRaw.value = ''
  agentFSDiffError.value = ''
}

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
    const project = findProjectByPath(data.path)
    const dir = { name: project?.name || data.name || data.path, path: data.path }
    currentWorkDir.value = dir
    // 后端启动目录只是运行上下文，不等于用户创建的项目。只有显式项目才进入
    // Recent，避免再次把开发仓库名 re0 暴露到真实用户界面。
    if (project) {
      workDirRecents.value = [dir, ...workDirRecents.value.filter(d => normalizeProjectPath(d.path) !== normalizeProjectPath(dir.path))].slice(0, 6)
    }
    saveWorkDirState()
  } catch (e) {}
}
function toggleWorkDirMenu() {
  showWorkDirMenu.value = !showWorkDirMenu.value
  if (showWorkDirMenu.value) workDirMenuView.value = 'recent'
}

function openSystemWorkDirPicker() {
  showWorkDirMenu.value = false
  // 系统原生文件夹选择器（后端 PowerShell IFileOpenDialog）：
  // 浏览器拿不到本地绝对路径，file input(webkitdirectory) 只能给目录名，
  // 选仓库根目录时会被后端 Join(GitRepoRoot, name) 拼错路径而失败（400 目录不存在）。
  // 原生选择器一次拿到绝对路径，这才是正路（workdir_handler.go 注释原话）。
  systemPickWorkdir()
}
async function systemPickWorkdir() {
  try {
    const res = await fetch('/api/workdir/pick', { method: 'POST' })
    const data = await res.json()
    if (!data || data.cancelled) return
    if (!data.path) {
      showGitToast('无法识别所选目录')
      return
    }
    await selectWorkDir({ name: data.name || data.path, path: data.path })
  } catch (e) {
    showGitToast(e.message || '选择目录失败')
  }
}
async function onSystemWorkDirSelected(event) {
  const files = Array.from(event.target.files || [])
  if (!files.length) return

  const relativeParts = (files[0].webkitRelativePath || '').split('/').filter(Boolean)
  const folderName = relativeParts[0]
  if (!folderName) {
    showGitToast('无法识别所选目录')
    return
  }

  // Electron/WebView 会暴露 File.path，可保留系统选择器返回的完整路径；
  // 普通浏览器出于安全原因只提供目录名——此时目录名不可信（后端会按
  // GitRepoRoot 相对路径解析），必须改走系统原生选择器拿绝对路径。
  const nativeFilePath = files[0].path
  if (!nativeFilePath) {
    showGitToast('浏览器拿不到本地路径，改用系统选择器')
    await systemPickWorkdir()
    return
  }
  let path = folderName
  if (nativeFilePath && relativeParts.length > 1) {
    const normalized = nativeFilePath.replace(/\\/g, '/')
    const relativeTail = relativeParts.slice(1).join('/')
    if (normalized.endsWith(`/${relativeTail}`)) {
      path = normalized.slice(0, -(relativeTail.length + 1))
    }
  }
  await selectWorkDir({ name: folderName, path })
}
async function selectWorkDir(dir, opts = {}) {
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
    const resolved = { name: dir.name || data.name || dir.path, path: data.path || dir.path }
    rememberProject(resolved)
    currentWorkDir.value = resolved
    // 去重后塞到最前面，最多保留 6 条最近记录
    workDirRecents.value = [resolved, ...workDirRecents.value.filter(d => normalizeProjectPath(d.path) !== normalizeProjectPath(resolved.path))].slice(0, 6)
    // 切换目录就是把当前会话归入该项目；旧实现只切了 agent cwd，侧栏不会更新。
    // 创建项目（opts.recordSession=false）只切 cwd 不动会话归属——当前对话仍留在原项目，
    // 否则会话会被凭空塞进刚建的空项目。
    if (opts.recordSession !== false) recordSessionWorkdir(activeSession.value)
    saveWorkDirState()
    showWorkDirMenu.value = false
    showGitToast(`已切换工作目录: ${resolved.name}`)
    await refreshAgentFSTimeline()
  } catch (e) {
    showGitToast(e.message || '切换工作目录失败')
  } finally {
    workDirSwitching.value = false
  }
}

async function createProject({ name, sourceFolder, thenNewSession }) {
  const projectName = name?.trim()
  if (!projectName || !sourceFolder?.path) {
    showGitToast('项目名称或源文件夹无效')
    return
  }
  // 创建项目 = 新建实体 + 切 cwd，但不动当前会话归属（recordSession: false）：
  // 会话属于它原来所在的项目，等用户在新项目里新建对话才归入新项目
  await selectWorkDir({ name: projectName, path: sourceFolder.path }, { recordSession: false })
  // 从"新建会话时没有项目可选"这条路径过来的创建，建完项目要接着把会话建上
  if (thenNewSession) await newSession(currentWorkDir.value)
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
const showBranchMenu = ref(false)
const gitBranches = ref([])
const branchSearch = ref('')
const branchSearchInput = ref(null)
const branchesLoading = ref(false)
const branchSwitching = ref(false)
const filteredGitBranches = computed(() => {
  const query = branchSearch.value.trim().toLocaleLowerCase()
  return query
    ? gitBranches.value.filter(branch => branch.toLocaleLowerCase().includes(query))
    : gitBranches.value
})

async function readGitResponse(res) {
  const text = await res.text()
  if (!text) return {}
  try {
    return JSON.parse(text)
  } catch {
    if (!res.ok) {
      throw new Error(res.status === 404
        ? '分支接口尚未加载，请重启后端服务'
        : `Git 接口返回异常 (${res.status})`)
    }
    throw new Error('Git 接口返回了无法识别的数据')
  }
}

async function fetchGitBranches() {
  branchesLoading.value = true
  try {
    const res = await fetch('/api/git/branches')
    const data = await readGitResponse(res)
    if (!res.ok) throw new Error(data.error || `读取失败 (${res.status})`)
    gitBranches.value = Array.isArray(data.branches) ? data.branches : []
  } catch (err) {
    showGitToast(err.message || '读取分支失败')
  } finally {
    branchesLoading.value = false
  }
}

async function toggleBranchMenu() {
  showBranchMenu.value = !showBranchMenu.value
  showWorkDirMenu.value = false
  if (!showBranchMenu.value) return
  branchSearch.value = ''
  await fetchGitBranches()
  nextTick(() => branchSearchInput.value?.focus())
}

async function checkoutGitBranch(branch) {
  if (branch === gitStatus.value.branch) {
    showBranchMenu.value = false
    return
  }
  branchSwitching.value = true
  try {
    const res = await fetch('/api/git/checkout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ branch })
    })
    const data = await readGitResponse(res)
    if (!res.ok) throw new Error(data.error || `切换失败 (${res.status})`)
    showBranchMenu.value = false
    await fetchGitStatus()
    showGitToast(`已切换到 ${branch}`)
  } catch (err) {
    showGitToast(err.message || '切换分支失败')
  } finally {
    branchSwitching.value = false
  }
}

async function createGitBranch() {
  const suggested = branchSearch.value.trim()
  const branch = window.prompt('输入新分支名称', suggested)
  if (!branch?.trim()) return
  branchSwitching.value = true
  try {
    const res = await fetch('/api/git/branches', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ branch: branch.trim() })
    })
    const data = await readGitResponse(res)
    if (!res.ok) throw new Error(data.error || `创建失败 (${res.status})`)
    showBranchMenu.value = false
    await Promise.all([fetchGitStatus(), fetchGitBranches()])
    showGitToast(`已创建并切换到 ${data.branch}`)
  } catch (err) {
    showGitToast(err.message || '创建分支失败')
  } finally {
    branchSwitching.value = false
  }
}
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
  "今天我们要创造什么？"

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



// 点导航轴上的圆点：滚到那条用户消息并高亮一下，否则跳过去了也不知道落在哪。
// 用 behavior:'auto' 而不是 'smooth'：平滑滚动是可中断的动画，会被聊天区
// 自动跟底的逻辑在半路打回原位（实测 smooth 跳完 scrollTop 原封不动，
// 瞬时跳则稳定生效）。跨两千像素找旧消息本来也不需要看动画。
function jumpToMessage(id) {
  manualActiveId.value = id
  const el = document.querySelector(`[data-msg-id="${id}"]`)
  if (!el) return
  el.scrollIntoView({ behavior: 'auto', block: 'center' })
  // 标记用户已主动滚动，防止自动跟底逻辑把刚跳过来的气泡又拉走
  userScrolledUp.value = true
}

// ==================== 工具函数 ====================
function cleanContent(content) { return content ? content.replace(/\[(action|emotion):[^\]]*\]/g, '') : '' }

// 一次工作流的「最终回答」= 最后一个 intent 块（工具调用之间的叙述也是 intent，
// 但最终答复必然是最后一条）。复制按钮拿它当内容。
function flowFinalText(flow) {
  const blocks = flow?.blocks || []
  for (let i = blocks.length - 1; i >= 0; i--) {
    if (blocks[i].type === 'intent' && (blocks[i].text || '').trim()) return blocks[i].text
  }
  return ''
}

// ==================== 就地编辑 + 替换式重发 ====================
// 点编辑：用户消息框本身变成输入框（不是去下面那个输入框）；右上角编辑按钮变发送按钮。
// 确认后是「替换式」：把这条消息及其之后的对话全部截掉（前端 + 后端会话存档），
// 再用新文本从这个点重新发起工作流——等价于 ChatGPT 的编辑消息=从这里重来。
const editingMsgId = ref(null)
const editDraft = ref('')

// 编辑中的 textarea 直接按 class 取——它在 v-for 里，用模板 ref 会被 Vue 收集成数组，
// .value.focus() 打空；而全场同一时刻只有一个 .msg-edit-input（v-if 保证），querySelector 稳。
function editTextareaEl() { return document.querySelector('.msg-edit-input') }

function editUserMessage(item) {
  if (flowState.active) return // 工作流进行中不打断
  editingMsgId.value = item.id
  editDraft.value = item.content || ''
  nextTick(() => {
    const el = editTextareaEl()
    if (!el) return
    el.focus()
    const len = el.value.length
    el.setSelectionRange(len, len) // 光标移末尾
    autoGrowEdit()
  })
}

function cancelEdit() {
  editingMsgId.value = null
  editDraft.value = ''
}

// 失焦即复原到普通无按钮态（丢弃这次编辑）。点发送按钮不会走到这里——
// 发送按钮用了 @mousedown.prevent 保住 textarea 焦点，@blur 不触发。
function onEditBlur() {
  cancelEdit()
}

function autoGrowEdit() {
  const el = editTextareaEl()
  if (!el) return
  el.style.height = 'auto'
  el.style.height = el.scrollHeight + 'px'
}

// 编辑重发 = 开新分支，不再是截断。原来那条线索完整保留在侧栏里，
// 用户可以在"原来那版"和"改过的这版"之间来回切。
async function confirmEdit(item) {
  if (flowState.active) return
  const text = editDraft.value.trim()
  if (!text) return
  const i = messages.value.findIndex(m => m.id === item.id)
  if (i < 0) { cancelEdit(); return }

  // 后端会话存档只在"往返完成"时按 user+assistant 成对落盘（失败的不落）。
  // 所以要保留的条数 = 被编辑消息之前「已完成的 agentflow 数」× 2，这样能正确跳过
  // 中途失败、没进存档的往返，不会算多。
  let completed = 0
  for (let k = 0; k < i; k++) {
    const m = messages.value[k]
    if (m.kind === 'agentflow' && m.status === 'completed') completed++
  }
  const keep = completed * 2
  const sid = sessionId.value || localStorage.getItem('prism_session_id') || ''

  // 原地重发兜底：分叉失败绝不能把用户刚打的字吃掉。注意它现在是非破坏性的
  // （不再调 truncate），最坏只是多出一条重复的尾巴。
  const resendInPlace = () => {
    messages.value.splice(i)
    cancelEdit()
    startCodeWorkflow(text)
  }
  if (!sid) { resendInPlace(); return }

  let newId = ''
  try {
    const res = await fetch(`/api/sessions/${encodeURIComponent(sid)}/fork`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ keep })
    })
    if (!res.ok) throw new Error(`fork 返回 ${res.status}`)
    newId = (await res.json()).session_id || ''
  } catch (err) {
    console.warn('分叉会话失败，退回原地重发', err)
    resendInPlace()
    return
  }
  if (!newId || newId === sid) { resendInPlace(); return }

  // 乐观插入：分支立刻带着血缘出现在侧栏。keep===0 时后端那条分支还是空会话，
  // List() 会跳过它，所以这一步也是那种情况下分支唯一的可见来源。
  // 名字用用户刚打的字，正好等于后端稍后算出的标题，不会有可见的改名。
  sessionList.value = [
    { id: newId, name: shortTitle(text), parentId: sid, forkIndex: keep, justForked: true },
    ...sessionList.value
  ]
  // 高亮一下就撤，让用户一眼看到新分支落在树的哪个位置
  setTimeout(() => {
    const n = sessionList.value.find(s => s.id === newId)
    if (n) n.justForked = false
  }, 1300)

  // 必须 await：switchSession 内部会 await loadAllHistory()，而后者整体替换
  // messages.value。放在 startCodeWorkflow 之后的话，刚推的用户气泡和 flow 对象
  // 会被冲掉，但 useAgentWorkflow 里的 currentFlow 仍持有引用——SSE 继续往一个
  // 已脱离的对象里流，表现为消息凭空消失、工作流永远转圈。
  await switchSession(newId)

  // 这里不再 splice：loadAllHistory 已经把服务端权威的前缀加载出来了。
  // 行为变化：旧 splice 会保留 index 之前未落盘的消息（失败/中断的轮次），
  // 现在分支只由已落盘状态构建，那些会消失——这是对的，它们本来刷新一下也留不住。
  cancelEdit()
  startCodeWorkflow(text)
}

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

// ==================== 消息朗读（Edge TTS 晓依，前端直连，零成本） ====================
// 走 wss://speech.platform.bing.com WebSocket，浏览器直连无跨域问题（2026-09-04 实测）。
// 三态反馈：空闲=volume-low，加载中=spinner 旋转+「正在合成语音」，播放中=volume-high+脉冲动画。
// 音频实时拉流播放，不落盘；同一时间只播一条，点另一条自动切；再点当前条停止。
import { EdgeTTSBrowser } from 'edge-tts-universal/browser'
const speakingId = ref(null)
const loadingId = ref(null)
let speakAudio = null
let speakToken = 0
// 音色由设置面板「语音音色」选择（localStorage: rescene_tts_voice），朗读时读取最新值
function ttsVoiceNow() {
  return localStorage.getItem('rescene_tts_voice') || 'zh-CN-XiaoyiNeural'
}

function speakIcon(id) {
  if (loadingId.value === id) return 'mdi:loading'
  if (speakingId.value === id) return 'mdi:volume-high'
  return 'mdi:volume-low'
}
function speakState(id) {
  if (loadingId.value === id) return '正在合成语音…'
  if (speakingId.value === id) return '停止朗读'
  return '朗读'
}

function stripMarkdownForTTS(md) {
  return String(md || '')
    .replace(/```[\s\S]*?```/g, '，代码块已省略，')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/[*_~>|]/g, '')
    .replace(/\n{2,}/g, '。')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 600)
}

async function speakMessage(item, rawText) {
  const text = stripMarkdownForTTS(rawText)
  if (!text) return
  // 再点当前这条：加载中=取消，播放中=停止
  if (loadingId.value === item.id || speakingId.value === item.id) { stopSpeaking(); return }
  stopSpeaking()
  const myToken = ++speakToken
  loadingId.value = item.id
  try {
    const tts = new EdgeTTSBrowser(text, ttsVoiceNow())
    const res = await tts.synthesize()
    if (myToken !== speakToken) return // 期间用户已切走/取消
    loadingId.value = null
    speakingId.value = item.id
    const url = URL.createObjectURL(res.audio)
    speakAudio = new Audio(url)
    speakAudio.onended = () => { if (myToken === speakToken) { speakingId.value = null; URL.revokeObjectURL(url) } }
    speakAudio.onerror = () => { if (myToken === speakToken) { speakingId.value = null; URL.revokeObjectURL(url) } }
    await speakAudio.play()
  } catch (e) {
    if (myToken === speakToken) { loadingId.value = null; speakingId.value = null }
  }
}

function stopSpeaking() {
  speakToken++
  if (speakAudio) { speakAudio.pause(); speakAudio = null }
  speakingId.value = null
  loadingId.value = null
}

// ==================== 语音输入（Edge 原生 Web Speech，按住说话） ====================
const voiceListening = ref(false)
let voiceRec = null
function voiceStart() {
  const SR = window.SpeechRecognition || window.webkitSpeechRecognition
  if (!SR) { alert('当前浏览器不支持语音输入，请用 Edge/Chrome'); return }
  if (voiceListening.value) return
  try {
    const rec = new SR()
    voiceRec = rec
    rec.lang = 'zh-CN'
    rec.interimResults = true
    rec.continuous = false
    rec.onresult = (e) => {
      let t = ''
      for (let i = 0; i < e.results.length; i++) t += e.results[i][0].transcript
      userInput.value = t
      adjustInputHeight()
    }
    rec.onend = () => { voiceListening.value = false }
    rec.onerror = () => { voiceListening.value = false }
    rec.start()
    voiceListening.value = true
  } catch (e) { console.warn('语音识别启动失败', e); voiceListening.value = false }
}
function voiceStop() {
  if (!voiceListening.value) return
  try { voiceRec?.stop() } catch (e) { /* 忽略 */ }
  voiceListening.value = false
  const t = (userInput.value || '').trim()
  if (t) handleSend()
}
onUnmounted(stopSpeaking)

// ==================== 模型选择 ====================
// 下拉展示后端内置目录与自定义提供方目录中「免 key 或已配 Key」的全部模型，按提供方分组；
// 不再有「选为可用」手动门控——填了 Key（或模型本身免 key）就自动出现。
// 图2「编辑模型」弹窗里的开关控制 hiddenModelIds（用户可隐藏不想见的模型），默认空=全显示。
// hiddenModelIds 由 composables/modelVisibility.js 统一管理（与图2 弹窗共享）。
const isModelVisible = (fm) => (fm.keyless || fm.api_key_set) && !fm.disabled && !hiddenModelIds.value.has(fm.id)

const selectedModel = ref(localStorage.getItem('selectedModel') || '')
// 模型 pill 的显示名：Auto 模式固定文案；其他取下拉里的 label，找不到回退占位。
const selectedModelLabel = computed(() => {
  if (selectedModel.value === 'auto') return 'Auto 智能路由'
  return modelOptions.value.find(m => m.value === selectedModel.value)?.label || (hasModels.value ? '模型' : '无可用模型')
})
// 列表为空时下拉无选项；选中项若不在真实可见列表里则定位到第一个。
// Auto 模式是虚拟选项（不在可见列表里），永不强制切走。
watch([freeModelsFull, hiddenModelIds], () => {
  const ids = visibleModelIds.value
  if (ids.length === 0) return
  if (selectedModel.value !== 'auto' && !ids.includes(selectedModel.value)) {
    selectedModel.value = ids[0]
    localStorage.setItem('selectedModel', ids[0])
  }
}, { deep: true })

const visibleModelIds = computed(() =>
  freeModelsFull.value.filter(isModelVisible).map(fm => fm.id)
)
// 按提供方（vendor）分组后的下拉数据：[{ vendor, items: [{ label, value }] }]
// 仅展示免 key/已配 key 且未被隐藏的模型；同名模型由后端生成的复合 ID 精确区分。
const groupedModelOptions = computed(() => {
  const groups = new Map()
  for (const fm of freeModelsFull.value) {
    if (!isModelVisible(fm)) continue
    const vendor = fm.vendor || '其他'
    if (!groups.has(vendor)) groups.set(vendor, { vendor, items: [] })
    groups.get(vendor).items.push({
      label: modelLabels.value[fm.id] || fm.name || fm.id,
      value: fm.id
    })
  }
  return Array.from(groups.values())
})
// 保持旧 modelOptions 引用（模板/逻辑其他地方可能直接读），指向展平列表
const modelOptions = computed(() => groupedModelOptions.value.flatMap(g => g.items))
const hasModels = computed(() => modelOptions.value.length > 0)
const showModelMenu = ref(false)
const modelPillRef = ref(null)
const inputWrapperRef = ref(null)
const modelMenuStyle = ref({})
// 菜单打开时测量位置：底缘贴输入框上沿（不是贴 pill），右缘对齐 pill，不悬空不顶屏
function measureModelMenu() {
  const pill = modelPillRef.value
  const anchor = inputWrapperRef.value || pill
  if (!anchor) { modelMenuStyle.value = {}; return }
  const anchorRect = anchor.getBoundingClientRect()
  const pillRect = pill ? pill.getBoundingClientRect() : anchorRect
  const menuW = Math.min(360, window.innerWidth - 32)
  const gap = 4
  // 右缘对齐 pill 右缘（下拉菜单惯例）
  let left = pillRect.right - menuW
  if (left < 8) left = 8
  if (left + menuW > window.innerWidth - 8) left = window.innerWidth - menuW - 8
  // 上方空间：输入框顶缘到视口顶（菜单最高 400px，超出按空间收缩）
  const spaceAbove = anchorRect.top - gap
  const maxH = Math.min(400, spaceAbove)
  if (spaceAbove < 200) {
    // 上方空间不够，翻到输入框下方
    const top = anchorRect.bottom + gap
    modelMenuStyle.value = { position: 'fixed', top: top + 'px', left: left + 'px', width: menuW + 'px', maxHeight: (window.innerHeight - top - 8) + 'px', zIndex: 9999 }
  } else {
    // 向上弹出：bottom 固定，菜单底缘始终紧贴输入框上沿
    modelMenuStyle.value = { position: 'fixed', bottom: (window.innerHeight - anchorRect.top + gap) + 'px', left: left + 'px', width: menuW + 'px', maxHeight: maxH + 'px', zIndex: 9999 }
  }
}

// 打开/关闭模型菜单（打开时定位到当前选中模型，避免 124 个模型列表打开在顶部还要手动拉）
function toggleModelMenu() {
  showModelMenu.value = !showModelMenu.value
  if (showModelMenu.value) {
    nextTick(() => {
      measureModelMenu()
      const list = document.querySelector('.model-menu-list')
      const active = list?.querySelector('.model-menu-item.active')
      if (list && active) {
        // 滚动到选中项：优先居中显示（长列表），列表太短滚不动就自动贴近
        list.scrollTop = Math.max(0, active.offsetTop - list.clientHeight / 2 + active.clientHeight / 2)
      }
    })
  }
}
const modelSearch = ref('')
// 搜索过滤后的分组（图1 顶部搜索框）
const filteredGroupedOptions = computed(() => {
  const q = modelSearch.value.trim().toLowerCase()
  return groupedModelOptions.value
    .map(g => ({
      vendor: g.vendor,
      items: g.items.filter(m => {
        if (q && !m.label.toLowerCase().includes(q)) return false
        return true
      })
    }))
    .filter(g => g.items.length > 0)
})

function selectModel(value) { selectedModel.value = value; localStorage.setItem('selectedModel', value); showModelMenu.value = false }
// 图2「编辑模型」弹窗开关
const showModelManager = ref(false)
// 删除模型密钥
async function onDeleteModelKey(modelId) {
  try {
    const res = await fetch('/api/models/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        configs: [{ id: modelId, api_key: '' }]
      })
    })
    if (!res.ok) throw new Error('HTTP ' + res.status)
    // 重新加载模型列表
    loadModelCapabilities()
  } catch (e) {
    console.warn('删除密钥失败', e)
  }
}

// ==================== 设置面板 ====================
const showSettings = ref(false)
const showScheduledTask = ref(false)
const showScheduledTaskManager = ref(false)
const showMailPanel = ref(false)
const notifications = ref([])
const notifCount = ref(0)
let notifPollTimer = null
function getAuthToken() {
  try { return localStorage.getItem('token') || '' } catch { return '' }
}
async function pollNotifications() {
  try {
    // 云端通知
    const res = await fetch('/api/notifications?limit=50&unread_only=false')
    let cloudList = []
    let cloudUnread = 0
    if (res.ok) {
      const data = await res.json()
      cloudList = data.notifications || []
      cloudUnread = data.unread_count || 0
    }
    // 本地通知（脚本/人设周报等，不依赖云端）
    let localList = []
    let localUnread = 0
    try {
      const lres = await fetch('/api/notifications/local')
      if (lres.ok) {
        const ldata = await lres.json()
        localList = ldata.notifications || []
        localUnread = ldata.unread_count || 0
      }
    } catch {}
    notifications.value = [...localList, ...cloudList]
    notifCount.value = localUnread + cloudUnread
  } catch { /* 后端没起就静默 */ }
}
function startNotifPoll() {
  pollNotifications()
  notifPollTimer = setInterval(pollNotifications, 30000)
}
function stopNotifPoll() {
  if (notifPollTimer) { clearInterval(notifPollTimer); notifPollTimer = null }
}
async function markNotifRead() {
  const token = getAuthToken()
  try {
    await fetch('/api/notifications/read', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
      body: '{}'
    })
  } catch {}
  // 本地通知也标已读
  try {
    await fetch('/api/notifications/read/local', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{}'
    })
  } catch {}
  // ⚠️ 只清 count 不够——行内红点读的是 notifications[].is_read，必须同步更新数组（后端挂了也本地清）
  notifications.value = (notifications.value || []).map(n => ({ ...n, is_read: true }))
  notifCount.value = 0
}
function notifTypeLabel(type) {
  const labels = { system: '系统', invite: '邀请码', cron: '定时任务', vip: '会员' }
  return labels[type] || type
}
// 新建弹窗是否从管理面板进入（取消/关闭时回到管理面板）
const scheduledTaskFromManager = ref(false)
function openScheduledTaskCreate() {
  // 管理面板点「新建」→ 关管理面板，打开新建弹窗
  scheduledTaskFromManager.value = true
  showScheduledTaskManager.value = false
  showScheduledTask.value = true
}
function closeScheduledTask() {
  showScheduledTask.value = false
  // 从管理面板进来时，取消/关闭后回到管理面板（v-if 重新挂载自动刷新列表）
  if (scheduledTaskFromManager.value) {
    scheduledTaskFromManager.value = false
    showScheduledTaskManager.value = true
  }
}
function onSettingsClosed() {
  showSettings.value = false
  loadModelCapabilities()
  // 设置面板「模型」页的统一模式直接写 localStorage('selectedModel')（跟这里同一个 key），
  // 不经过本组件的 selectModel()，所以关闭时要主动拉回来，否则顶部下拉还显示旧值。
  const persisted = localStorage.getItem('selectedModel')
  if (persisted && persisted !== selectedModel.value) selectedModel.value = persisted
}

function onCreateScheduledTask(data) {
  showScheduledTask.value = false
  fetch('/api/cron/create', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data)
  })
    .then(r => r.ok ? r.json() : Promise.reject(new Error('HTTP ' + r.status)))
    .then(() => {
      showGitToast('✅ 定时任务已创建，到点会弹系统通知')
      // 创建完回到管理面板（v-if 重新挂载 → onMounted 自动刷新列表）
      showScheduledTaskManager.value = true
    })
    .catch(e => {
      console.log('定时任务数据:', data, e)
      showGitToast('❌ 定时任务创建失败：' + (e.message || '网络错误'))
    })
}

// ==================== 底部工具条："+" 附加菜单 + Command 切换器 ====================
// 模式已删除（自研沙盒，agentfs 回退兜底），默认全自动 yolo，不再有 Ask/Plan 审批。
const showAddMenu = ref(false)

// 审批弹窗里把工具参数 JSON 美化显示；解析失败就原样展示字符串。
// 审批条是单行轻量展示，不能像原来的弹窗那样摊开整段 JSON。
// 优先挑出最能说明"要动什么"的字段（路径/命令），否则压成一行并截断。
function approvalArgsPreview(args) {
  if (!args) return ''
  let obj = args
  if (typeof args === 'string') {
    try { obj = JSON.parse(args) } catch { return truncateOneLine(String(args), 90) }
  }
  if (obj && typeof obj === 'object') {
    const key = ['command', 'path', 'file_path', 'source', 'destination'].find(k => obj[k])
    if (key) return truncateOneLine(String(obj[key]), 90)
    return truncateOneLine(JSON.stringify(obj), 90)
  }
  return truncateOneLine(String(obj), 90)
}
function truncateOneLine(s, max) {
  s = s.replace(/\s+/g, ' ').trim()
  return s.length > max ? s.slice(0, max) + '…' : s
}

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

// ==================== mermaid 图表渲染 ====================
// 聊天正文 / agent 意图块里出现 ```mermaid 时，renderMarkdown 已把围栏转成
// .mermaid-diagram 占位 div。这里用 MutationObserver 盯着聊天容器（覆盖流式
// 追加 + agent flow 意图块两处），出现未渲染的占位就懒加载 mermaid 转成 SVG。
// 节流：图渲染是异步重型，多个连续节点用 promise 串行，避免并发卡顿。
let mermaidObserver = null
let mermaidRenderQueued = false
function ensureMermaidObserver() {
  if (mermaidObserver) return
  mermaidObserver = new MutationObserver(() => {
    if (mermaidRenderQueued) return
    mermaidRenderQueued = true
    setTimeout(async () => {
      mermaidRenderQueued = false
      const root = document.querySelector('.chat-window')
      if (root) {
        try { await renderMermaidIn(root) } catch (e) { /* 单次失败不阻塞后续 */ }
      }
    }, 80)
  })
  const container = document.querySelector('.chat-window')
  if (container) mermaidObserver.observe(container, { childList: true, subtree: true })
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
  const t0 = performance.now()
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
  const t1 = performance.now()
  if (t1 - t0 > 16) console.warn('[perf] streamFadePass', (t1 - t0).toFixed(1), 'ms')
}

// ==================== useChatWidget ====================
const {
  isOpen, isExpanded, userInput, messages, sessionId,
  isLoggedIn, debugTemp, debugTopP, debugReasoning, debugMaxTokens, balance,
  currentStatus, statusDotColor,
  messagesContainer, chatInputRef, userScrolledUp, inputBarFade,
  forceScrollToBottom, adjustInputHeight, switchSession,
  onStreamUpdate,
  backgroundTaskList,
  runningTaskCount,
  runningSubagentCount,
  runningBgTaskCount,
  flowState, startCodeWorkflow, stopCodeWorkflow, approvalState, respondApproval,
  todoState, sendSteerMessage,
  questionState, answerQuestion,
  toggleChat, updateParams,
  groupedMessages, formatChatTime
} = useChatWidget(props, { renderMarkdown })

// ==================== 语音输出开关（喇叭按钮，Hermes 布局） ====================
// ⚠️ 必须放在 useChatWidget 解构之后：watch(messages) 在 setup 同步执行，解构前引用会 TDZ 白屏
const ttsOn = ref(localStorage.getItem('rescene_tts_on') === '1')
function toggleTts() {
  ttsOn.value = !ttsOn.value
  localStorage.setItem('rescene_tts_on', ttsOn.value ? '1' : '0')
  if (!ttsOn.value) stopSpeaking()
}
let autoSpeakToken = 0
async function autoSpeak(text) {
  if (!ttsOn.value || !text) return
  const t = stripMarkdownForTTS(text)
  if (!t) return
  stopSpeaking()
  const myToken = ++autoSpeakToken
  try {
    const tts = new EdgeTTSBrowser(t, ttsVoiceNow())
    const res = await tts.synthesize()
    if (myToken !== autoSpeakToken) return
    const url = URL.createObjectURL(res.audio)
    const a = new Audio(url)
    a.onended = () => { if (myToken === autoSpeakToken) URL.revokeObjectURL(url) }
    a.onerror = () => { if (myToken === autoSpeakToken) URL.revokeObjectURL(url) }
    await a.play()
  } catch (e) { /* 自动朗读失败不打扰用户 */ }
}
// AI 回复完成后自动朗读（仅新回复）
let lastSpeakSig = ''
watch(messages, (list) => {
  if (!ttsOn.value || !list.length) return
  const last = list[list.length - 1]
  if (last && last.sender === 'assistant' && !last.isStreaming && last.content) {
    const sig = (last.id || '') + ':' + String(last.content).length
    if (sig !== lastSpeakSig) {
      lastSpeakSig = sig
      autoSpeak(String(last.content))
    }
  }
}, { deep: true })

// 任务清单完成数（输入框上方 todo-bar 用），仿 Hermes 勾选清单。
const todoDoneCount = computed(() =>
  (todoState.items || []).filter(it => it.status === 'done').length
)
// todo 条折叠：点击头部切换，折叠后只留一行标题，展开才显示清单明细。
const todoCollapsed = ref(false)

// 上滑时 todo/askuser 随滚动淡出（仿 Hermes）：透明度来自 useChatWidget 的 inputBarFade。
// 在底部（≈1）时不写 opacity，避免内联样式压住 todo 全部完成时的 todo-fade 淡出动画。
// ⚠️ 不再用 pointer-events:none——那会让淡出后的条点击不到、也悬浮不到（Hermes 是 hover 即恢复不透明）。
// 悬浮恢复不透明由 .todo-bar:hover / .question-bar:hover 的 CSS 处理（见 chat-window.css / QuestionModal.vue）。
const inputBarFadeStyle = computed(() => {
  const s = {}
  // 用 CSS 变量传淡出值：元素 opacity: var(--bar-fade)，:hover 时 CSS 能强制拉回 1
  // 在底部（≈1）时不设变量（让默认 opacity:1 生效），避免压住 todo-fade 淡出动画
  if (inputBarFade.value < 0.999) s['--bar-fade'] = String(inputBarFade.value)
  return s
})

// 全部完成后延迟淡出：先让用户看到 N/N，再整条消失（仿 Hermes 收尾）。
// agent 每次 update_todo 会全量覆盖 items，所以中途插入新/未完成项要取消定时。
let todoClearTimer = null
watch(
  () => todoState.items.length > 0 && todoDoneCount.value === todoState.items.length,
  (allDone) => {
    if (todoClearTimer) { clearTimeout(todoClearTimer); todoClearTimer = null }
    if (allDone) {
      todoClearTimer = setTimeout(() => { todoState.items = [] }, 3500)
    }
  }
)
onUnmounted(() => { if (todoClearTimer) clearTimeout(todoClearTimer) })


// 导航轴当前高亮的用户消息 id：必须放在 useChatWidget 解构之后，避免 setup 阶段命中 TDZ。
// 默认跟随最后一条用户消息；点击节点后切换到对应消息，新消息进来再重置回最新。
// 注意：消息 id 可能是数字 0，用 ?? 而不是 ||，避免 0 被当成空值。
const manualActiveId = ref(null)
const activeUserMessageId = computed(() => manualActiveId.value ?? lastUserMessageId.value)
watch(lastUserMessageId, () => { manualActiveId.value = null })

// 工作流跑完后重新拉一次会话列表：把分叉时乐观插入的分支名跟后端算出的标题对齐，
// 也顺带修掉"新会话标题要切走再切回才出现"的老毛病（以前只在挂载/切会话时拉）。
// 必须放在上面的解构之后：watch 的 getter 是立即求值的，写在解构之前会命中 TDZ
// （同一文件里 runningSession 那个 computed 能放在前面，只是因为 computed 是惰性的）。
watch(() => flowState.active, (now, was) => {
  if (was && !now) {
    completedSessions.value.add(activeSession.value)
    completedSessions.value = new Set(completedSessions.value)
    loadSessionList()
  }
})

function onSessionTitleUpdate(e) {
  const d = e?.detail
  if (!d || !d.title) return
  updateSessionTitle(d.title, d.fallback, d.sid)
}

function onSessionTitlePending(e) {
  const sid = e?.detail?.sid
  if (sid) pendingTitleSessions.add(sid)
}

// ==================== 思考强度（Effort）：Faster(low) ↔ Smarter(high) ====================
// 注意：debugReasoning 来自上面的 useChatWidget 解构，本段必须放在解构之后，
// 否则 setup 阶段会命中暂时性死区（TDZ）报 "Cannot access before initialization"。
const EFFORT_LEVELS = ['low', 'medium', 'high']
const EFFORT_UI_LABELS = { low: '低', medium: '中', high: '高' }
const showEffortPanel = ref(false)
const showHeatmapPopup = ref(true)
const effortWidgetRef = ref(null)
// 面板定位：打开时 nextTick 重新测量 pill 位置并做视口边界 clamp，
// 避免 computed 惰性求值在 ref 未就绪时拿到错误坐标（表现为弹层掉到下方）。
const effortPanelPos = ref({})
function measureEffortPanel() {
  const el = effortWidgetRef.value
  if (!el) { effortPanelPos.value = {}; return }
  const rect = el.getBoundingClientRect()
  const panelW = 220
  const panelH = 88 // 估算高度（标题+滑块），用于底部边界判断
  // 默认浮在 pill 正上方、水平居中对齐
  let top = rect.top - 8
  let left = rect.left + rect.width / 2
  let transform = 'translate(-50%, -100%)'
  // 顶部空间不足则翻到 pill 下方
  if (top - panelH < 4) {
    top = rect.bottom + 8
    transform = 'translate(-50%, 0)'
  }
  // 水平超出右边界则右对齐收边
  if (left + panelW / 2 > window.innerWidth - 4) {
    left = window.innerWidth - panelW / 2 - 4
  } else if (left - panelW / 2 < 4) {
    left = panelW / 2 + 4
  }
  effortPanelPos.value = {
    position: 'fixed',
    top: top + 'px',
    left: left + 'px',
    transform,
    zIndex: 9999
  }
}
watch(showEffortPanel, (open) => {
  if (open) nextTick(measureEffortPanel)
})
const initialEffortIdx = EFFORT_LEVELS.indexOf(debugReasoning.value)
const effortLevel = ref(initialEffortIdx >= 0 ? initialEffortIdx : 1)
const effortLabel = computed(() => EFFORT_UI_LABELS[EFFORT_LEVELS[effortLevel.value]])
function onEffortChange() {
  debugReasoning.value = EFFORT_LEVELS[effortLevel.value]
  localStorage.setItem('debugReasoning', debugReasoning.value)
}
if (!debugReasoning.value) onEffortChange() // 首次没设置过时落一个默认值，跟滑块初始位置对齐

// ==================== 模型菜单 hover 思考强度卡片 ====================
const hoveredModel = ref('')
const hoverEffortStyle = ref({})
let hoverLeaveTimer = null
// 悬停某模型行：把卡片定位到该行右侧（菜单本身是 fixed，取行的视口坐标即可）
function onItemHover(value, ev) {
  hoveredModel.value = value
  clearTimeout(hoverLeaveTimer)
  const item = ev?.currentTarget
  const rect = item ? item.getBoundingClientRect() : null
  if (rect) {
    const popW = 168
    // 默认贴行右侧；右缘超视口则翻到行左侧
    let left = rect.right + 6
    if (left + popW > window.innerWidth - 8) left = rect.left - popW - 6
    if (left < 8) left = 8
    hoverEffortStyle.value = {
      position: 'fixed',
      top: Math.max(8, rect.top - 4) + 'px',
      left: left + 'px',
      width: popW + 'px',
      zIndex: 10000
    }
  }
}
function onItemLeave() {
  // 延迟关闭，给鼠标从行滑向卡片留时间
  clearTimeout(hoverLeaveTimer)
  hoverLeaveTimer = setTimeout(() => { hoveredModel.value = '' }, 120)
}
function hoverPopupEnter() { clearTimeout(hoverLeaveTimer) }
function hoverPopupLeave() { hoveredModel.value = '' }
// 思考开关：当前有 reasoning 值则关闭（置空），否则落回当前档位
function toggleThinking() {
  if (debugReasoning.value) {
    debugReasoning.value = ''
    localStorage.setItem('debugReasoning', '')
  } else {
    onEffortChange()
  }
}
function selectEffort(lv) {
  const idx = EFFORT_LEVELS.indexOf(lv)
  if (idx < 0) return
  effortLevel.value = idx
  onEffortChange()
}

// ==================== UI 状态 ====================
const showParams = ref(false)

// ==================== 左侧 Gemini 风侧栏：展开 vs 折叠竖条 ====================
const sidebarOpen = ref(localStorage.getItem('sidebarOpen') !== '0')
const sidebarWidth = ref(220)
const { startDrag: startSidebarWidthDrag } = useResizableWidth(sidebarWidth, {
  min: 180,
  max: 400,
  edge: 'right',
  persistKey: 'gemSidebarWidth'
})
function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value
  localStorage.setItem('sidebarOpen', sidebarOpen.value ? '1' : '0')
}

// ==================== Gemini 风格对话搜索面板 ====================
const showSearchPanel = ref(false)
const searchQuery = ref('')
const searchPanelInput = ref(null)

const filteredSearchSessions = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const list = [...sessionList.value].sort((a, b) => {
    const ta = a.updatedAt ? new Date(a.updatedAt).getTime() : 0
    const tb = b.updatedAt ? new Date(b.updatedAt).getTime() : 0
    return tb - ta
  })
  if (!q) return list
  return list.filter(s => (s.name || '').toLowerCase().includes(q))
})

// recall 真实搜索：空查询=近期对话（本地列表），非空=防抖调 /api/sessions-search
// 后端按会话聚合返回命中会话 + 片段，替代原来只按标题过滤本地列表的假搜索。
const searchResults = ref([])
const searchLoading = ref(false)
let searchTimer = null

function highlightHits(hits) {
  if (!hits || !hits.length) return ''
  const q = searchQuery.value.trim()
  let text = (hits[0].content || '').replace(/[<>&]/g, ch => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;' }[ch]))
  if (q) {
    try {
      const esc = q.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
      text = text.replace(new RegExp('(' + esc + ')', 'gi'), '<mark>$1</mark>')
    } catch (e) { /* 非法正则保持原样 */ }
  }
  return text
}

async function runSessionSearch(q) {
  if (!q) {
    searchResults.value = filteredSearchSessions.value.map(s => ({ id: s.id, name: s.name, updatedAt: s.updatedAt, hits: [] }))
    return
  }
  searchLoading.value = true
  try {
    const res = await fetch('/api/sessions-search?q=' + encodeURIComponent(q) + '&limit=20')
    const data = await res.json()
    searchResults.value = (data.sessions || []).map(s => ({
      id: s.session_id,
      name: s.title || '新对话',
      updatedAt: s.updated_at,
      hits: s.hits || []
    }))
  } catch (e) {
    searchResults.value = []
  } finally {
    searchLoading.value = false
  }
}

watch(searchQuery, (q) => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => runSessionSearch((q || '').trim()), 250)
})

function openSearchPanel() {
  sidebarOpen.value = true
  localStorage.setItem('sidebarOpen', '1')
  showSearchPanel.value = true
  runSessionSearch((searchQuery.value || '').trim())
  nextTick(() => searchPanelInput.value?.focus())
}

function closeSearchPanel() {
  showSearchPanel.value = false
  searchQuery.value = ''
  clearTimeout(searchTimer)
}

function onSearchSelect(id) {
  selectSession(id)
  closeSearchPanel()
}

// 插件市场：当前为占位入口，后续可接入真实插件商店接口
const showPluginsPanel = ref(false)
function openPluginsMarket() {
  showPluginsPanel.value = true
}
function closePluginsPanel() {
  showPluginsPanel.value = false
}

function formatSearchDate(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  const now = new Date()
  const isYesterday = d.getDate() === now.getDate() - 1 && d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear()
  if (isYesterday) return '昨天'
  if (d.toDateString() === now.toDateString()) return '今天'
  return `${d.getMonth() + 1}月${d.getDate()}日`
}

// 折叠态会话横条：当前项目会话在上，其他在下。
// workdir 由 localStorage('shanxi_session_workdir') 映射管理。
function getCurrentWorkdirName() {
  return currentWorkDir.value?.name?.trim() || ''
}
const currentWorkdirName = computed(getCurrentWorkdirName)
const railProject = computed(() => {
  const wd = getCurrentWorkdirName()
  if (!wd) return []
  return sessionList.value.filter(s => s.workdir === wd).slice(0, 10)
})
const railRecent = computed(() => {
  const wd = getCurrentWorkdirName()
  if (!wd) return sessionList.value.slice(0, 10)
  return sessionList.value.filter(s => s.workdir !== wd).slice(0, 10)
})

// 悬停横条弹出的会话卡片：立刻打开（无延迟），移开留 160ms 缓冲，
// 让鼠标能从横条平移到卡片上而不闪断。卡片自身也挂同一对进入/离开处理。
const railCardOpen = ref(false)
const railCardStyle = ref({})
let railCardCloseTimer = null
const railUtilityPreview = ref('')
const railUtilityPreviewStyle = ref({})
let railUtilityPreviewCloseTimer = null

function cancelRailUtilityPreviewClose() {
  clearTimeout(railUtilityPreviewCloseTimer)
}
function openRailUtilityPreview(type, event) {
  cancelRailUtilityPreviewClose()
  railCardOpen.value = false
  const rect = event.currentTarget.getBoundingClientRect()
  const cardHeight = type === 'agentfs'
    ? Math.min(500, window.innerHeight - 32)
    : Math.min(500, window.innerHeight - 32)
  const top = Math.max(16, Math.min(rect.bottom - cardHeight, window.innerHeight - cardHeight - 16))
  railUtilityPreviewStyle.value = {
    left: `${rect.right + 10}px`,
    top: `${top}px`,
    height: `${cardHeight}px`
  }
  railUtilityPreview.value = type
  if (type === 'agentfs') refreshAgentFSTimeline()
}
function closeRailUtilityPreviewDelayed() {
  cancelRailUtilityPreviewClose()
  railUtilityPreviewCloseTimer = setTimeout(() => {
    railUtilityPreview.value = ''
  }, 160)
}
function openAgentFSFromRailPreview(snapshot, event) {
  cancelRailUtilityPreviewClose()
  railUtilityPreview.value = ''
  openAgentFSSnapshot(snapshot, event)
}
function openRailCard(e) {
  clearTimeout(railCardCloseTimer)
  // 只在从横条区进入时重算位置；从卡片自身进入时保持原位
  const railEl = e?.currentTarget?.classList?.contains('gem-rail-sessions') ? e.currentTarget : null
  if (railEl) {
    const r = railEl.getBoundingClientRect()
    const maxH = Math.min(420, window.innerHeight - 32)
    // 竖直方向以横条区顶部为锚，超出视口下沿时上推
    let top = r.top
    if (top + maxH > window.innerHeight - 16) top = Math.max(16, window.innerHeight - 16 - maxH)
    railCardStyle.value = { left: (r.right + 10) + 'px', top: top + 'px', maxHeight: maxH + 'px' }
  }
  railCardOpen.value = true
}
function closeRailCardDelayed() {
  clearTimeout(railCardCloseTimer)
  railCardCloseTimer = setTimeout(() => { railCardOpen.value = false }, 160)
}
function onRailCardSelect(id) {
  clearTimeout(railCardCloseTimer)
  railCardOpen.value = false
  selectSession(id)
}

// ==================== 工具面板状态绑定会话 ====================
// dockPanels（终端/Diff/预览）是会话的工作现场：切会话/新会话时各自恢复各自的，
// 修掉"新会话回到首页还挂着上个会话工具弹窗"的 bug。仅内存级（刷新清零）。
const dockPanelsBySession = {}
watch(() => sessionId.value, (nid, oid) => {
  if (oid) {
    dockPanelsBySession[oid] = {
      panels: [...dockPanels.value],
      active: { ...activeDockPanels },
      hidden: { ...dockHidden },
      expanded: dockExpandedLocation.value
    }
  }
  const saved = dockPanelsBySession[nid] || {
    panels: [],
    active: { right: '', bottom: '' },
    hidden: { right: false, bottom: false },
    expanded: ''
  }
  dockPanels.value = [...saved.panels]
  activeDockPanels.right = saved.active?.right || ''
  activeDockPanels.bottom = saved.active?.bottom || ''
  dockHidden.right = Boolean(saved.hidden?.right)
  dockHidden.bottom = Boolean(saved.hidden?.bottom)
  dockExpandedLocation.value = saved.expanded || ''
  ensureActiveDockPanels()
  showDockAddMenu.value = false
  clearDockDragState()
  // 切会话时重载上下文数据，否则继续显示上个会话的数值
  loadContextBreakdown(nid || '')
  sessionTokenStats.value = loadSessionTokenStats(nid || '')
})

// 消息流里每个 kind:'group' 的组件实例，供后台任务清单点击跳转+展开用
const groupRefs = {}
function setGroupRef(id, el) {
  if (el) groupRefs[id] = el
}
function jumpToGroup(id) {
  nextTick(() => {
    groupRefs[id]?.expand()
    document.getElementById('group-' + id)?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  })
}

// ==================== 发送：聊天/代码只有一条链路了 ====================
// 之前 Chat/Code 是两个模式两条路——Chat 走轻量流式，Code 走四态机能调工具。
// 合并成一条：永远走四态机（startCodeWorkflow），模型自己判断要不要调工具，
// 不需要工具时就是普通对话回复，agent 两件事都能干，用户不用先选模式。
// 共享池模型（免费试用）走简单聊天/POST 流式，无 agent 工作流。
// ==================== 输入历史（↑/↓ 浏览最近发送） ====================
// Claude Code 式：输入框为空时 ↑ 回退上一条，↓ 前进；发送成功后入栈。
// 仅在「已进入浏览」或「输入为空」时拦截箭头，不干扰多行编辑的光标移动。
const chatHistory = ref([])
const historySwapping = ref(false)
let swapTimer = null
const chatHistoryIndex = ref(-1) // -1 = 未在浏览（编辑新内容）
const CHAT_HISTORY_KEY = 'rescene_chat_history'
const CHAT_HISTORY_MAX = 50

function loadChatHistory() {
  try {
    const raw = localStorage.getItem(CHAT_HISTORY_KEY)
    chatHistory.value = raw ? JSON.parse(raw) : []
  } catch { chatHistory.value = [] }
}
function saveChatHistory() {
  try { localStorage.setItem(CHAT_HISTORY_KEY, JSON.stringify(chatHistory.value)) } catch {}
}
function pushChatHistory(text) {
  const t = (text || '').trim()
  if (!t) return
  chatHistory.value = [t, ...chatHistory.value.filter(x => x !== t)].slice(0, CHAT_HISTORY_MAX)
  chatHistoryIndex.value = -1
  saveChatHistory()
}
function onChatInputKeydown(e) {
  if (e.key === 'ArrowUp') {
    // 未浏览且输入非空 → 让位给光标移动（多行编辑）
    if (chatHistoryIndex.value === -1 && userInput.value.trim()) return
    if (!chatHistory.value.length) return
    e.preventDefault()
    if (chatHistoryIndex.value === -1) chatHistoryIndex.value = 0
    else chatHistoryIndex.value = Math.min(chatHistoryIndex.value + 1, chatHistory.value.length - 1)
    applyHistoryText(e.target, chatHistory.value[chatHistoryIndex.value])
  } else if (e.key === 'ArrowDown') {
    if (chatHistoryIndex.value === -1) return
    e.preventDefault()
    chatHistoryIndex.value -= 1
    const text = chatHistoryIndex.value < 0 ? '' : chatHistory.value[chatHistoryIndex.value]
    if (chatHistoryIndex.value < 0) chatHistoryIndex.value = -1
    applyHistoryText(e.target, text)
  }
}

// 历史切换：替换文本 + 重算高度 + 纯 opacity 淡入。
// 不做位移动画——textarea 是 flex:1，位移只挪文本区不挪旁边按钮，底边会跳。
function applyHistoryText(el, text) {
  userInput.value = text
  el.value = text
  historySwapping.value = true
  if (swapTimer) clearTimeout(swapTimer)
  swapTimer = setTimeout(() => { historySwapping.value = false }, 150)
  // 摘类 → 强制 reflow → 加回，让 opacity 动画每次切换都重触发（同帧内完成，无闪烁）
  el.classList.remove('chat-input-swap')
  void el.offsetWidth
  el.classList.add('chat-input-swap')
  nextTick(adjustInputHeight)
}

// ★ 直连流式模式下的思考区域「瀑布滚动」：思考 text 流式追加时滚到底，避免置顶不动。
// ⚠️ 性能修复：原版 watch(messages, { deep: true }) 深度监听整个数组，思考流式每 token
// 都触发一次 → nextTick → 全 DOM querySelectorAll + 滚动，逐 token 全量重查 = 巨卡。
// 改为监听「流式思考消息对象」this.reasoning 单字段 + 节流滚动，只在该消息思考变长时滚一次。
function scrollStreamedReasoning() {
  const list = document.querySelectorAll('.assistant-message.streaming .reasoning-text')
  if (!list.length) return
  const el = list[list.length - 1]
  el.scrollTop = el.scrollHeight
}
// 节流：思考每 ~80ms 最多滚一次，避免每 token 全 DOM 重查
let reasoningScrollTimer = null
watch(
  () =>
    messages.value
      .map((m) =>
        m.kind !== 'agentflow' && m.sender === 'assistant' && m.isStreaming ? m.reasoning || '' : ''
      )
      .join('|'),
  () => {
    if (reasoningScrollTimer) return
    reasoningScrollTimer = setTimeout(() => {
      reasoningScrollTimer = null
      scrollStreamedReasoning()
    }, 80)
  }
)

// 点击 follow-up 卡片 → 填入输入框并发送
function onFollowUpClick(card) {
  userInput.value = card.label
  nextTick(() => adjustInputHeight())
  handleSend()
}

function handleSend() {
  if (hasPendingAttachments.value) return
  // 亲密度 +1（fire-and-forget，失败静默不阻断发送）
  railAuth.incIntimacy()
  // 工作流跑着的时候，回车不再是"发一条新消息"
  if (flowState.active) {
      const steerText = userInput.value.trim()
      if (!steerText) return
      userInput.value = ''
      nextTick(() => { if (chatInputRef.value) chatInputRef.value.style.height = 'auto' })
      const ok = sendSteerMessage(steerText)
      if (ok) pushChatHistory(steerText)
      if (!ok) {
        messages.value.push({
          id: `steer-fail-${Date.now()}`,
          kind: 'text',
          sender: 'user',
          content: steerText,
          status: 'steered-fail',
          timestamp: new Date()
        })
        onStreamUpdate?.()
      }
      return
    }
  const combined = buildOutgoingMessage()
  if (!combined) return
  const displayText = userInput.value.trim()
  pushChatHistory(displayText)
  const displayAttachments = attachments.value.filter(a => a.status === 'ready').map(a => ({ ...a }))
  clearAttachments()
  userInput.value = ''
  nextTick(() => { if (chatInputRef.value) chatInputRef.value.style.height = 'auto' })
  // 公益免费模型：发消息瞬间就把「用户气泡 + bot 正在思考框」都建出来，
    // 鉴权/配额在后台并行——绝不让首屏等云往返（否则气泡、思考框都要卡到配额回来才显示）
    if (sharedPoolModelIds.value.has(selectedModel.value)) {
      const userMsg = {
        id: `sp_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`,
        sender: 'user',
        content: displayText,
        attachments: displayAttachments || [],
        timestamp: new Date()
      }
      messages.value.push(userMsg)
      // bot flow 先即时建出（running + 空 blocks）→「正在思考」扫描线立刻出现
      const flow = reactive({
        id: `sp_flow_${Date.now()}`,
        kind: 'agentflow',
        sender: 'bot',
        status: 'running',
        task: combined,
        blocks: [],
        startTime: Date.now(),
        endTime: null,
        modelInfo: null,
        timestamp: new Date()
      })
      messages.value.push(flow)
            onStreamUpdate?.()
            // 游客：先确保本机已拿到云端分发的游客 UID（首次发消息没有则现补，
            //   保证「点开就用」的公益免费体验；登录用户已有账号身份，无需此步）
            ensureGuestUid()
            // 配额/鉴权放后台：通过则拉流填充；不足则在这个已显示的框里补失败
            checkSharedPoolQuota().then(ok => {
        if (ok) {
          sendSharedPoolStream(flow, combined, selectedModel.value)
        } else {
          flow.status = 'failed'
          const q = sharedPoolQuota.value
          flow.blocks.push({
            type: 'intent',
            text: q?.limit != null
              ? `😅 公益免费额度已用完（今日 ${q.used ?? q.limit}/${q.limit} 次）\n\n填自己的 Key 继续使用，无限制～`
              : `😅 公益免费额度已用完（今日 50/50 次）\n\n填自己的 Key 继续使用，无限制～`
          })
          flow.endTime = Date.now()
          onStreamUpdate?.()
        }
      }).catch(() => {
        // 配额检查异常：不阻塞，直接代理到云端
        sendSharedPoolStream(flow, combined, selectedModel.value)
      })
      return
    }
    // opts.model = 下拉框当前选中的模型（响应式 ref，watch 保证非空）
    startCodeWorkflow(combined, { text: displayText, attachments: displayAttachments }, { model: selectedModel.value })
  }

  // 确保本机已有云端游客 UID（未登录时公益免费的身份依据）。
    // 同一设备首次调用用设备指纹向 /api/auth/uid 换 UID 并缓存（幂等，之后恒定）。
    let ensureGuestUidPromise = null
    async function ensureGuestUid() {
      const cached = localStorage.getItem('aurora_uid')
      if (cached) return cached
      if (ensureGuestUidPromise) return ensureGuestUidPromise
      ensureGuestUidPromise = (async () => {
        try {
          let deviceId = localStorage.getItem('aurora_device_id')
          if (!deviceId) {
            deviceId = (crypto.randomUUID && crypto.randomUUID())
              || ('d-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 10))
            localStorage.setItem('aurora_device_id', deviceId)
          }
          const res = await fetch('/api/auth/uid', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ device_id: deviceId, fingerprint: computeHardwareFingerprint() })
          })
          if (res.ok) {
            const data = await res.json()
            if (data.uid) {
              localStorage.setItem('aurora_uid', String(data.uid))
              return String(data.uid)
            }
          }
        } catch {
          // 云端不可达：本次放弃游客身份，云端 401 会给出明确引导
        }
        return null
      })()
      return ensureGuestUidPromise
    }

    // 公益免费请求的统一身份头：登录带 JWT，未登录带游客 UID（X-Guest-Uid），
    // 游客凭本地 UID「点开就用」，免去先登录才能体验公益免费的障碍。
    function sharedPoolAuthHeaders() {
      const h = { 'Content-Type': 'application/json' }
      const token = localStorage.getItem('token')
      if (token) h['Authorization'] = 'Bearer ' + token
      const guest = localStorage.getItem('aurora_uid') || (railAuth?.uid?.value)
      if (guest != null && guest !== '' && guest !== 'undefined') h['X-Guest-Uid'] = String(guest)
      return h
    }

    // 公益免费配额检查（带 25s 缓存：连续对话不再每次发送都打一次云端配额往返；
    // 配额的权威值仍由云端 429 兜底，缓存过期后自动回源）。
    // 重要：只有云端明确返回 remaining=0 才算「已用完」；401（无身份）/网络异常
    // 一律当作「可尝试」放行——真正被限流时云端 429 会给准确提示，绝不误报已用完。
    let quotaCacheOk = false
    let quotaCacheUntil = 0
    async function checkSharedPoolQuota() {
      const now = Date.now()
      if (quotaCacheUntil > now) return quotaCacheOk
      // 先确保游客 UID 已拿到并缓存（否则请求没带 X-Guest-Uid，云端会 401）
      await ensureGuestUid()
      try {
        const res = await fetch('/api/shared-pool/quota', {
          headers: sharedPoolAuthHeaders()
        })
        if (res.status === 401 || res.status === 403) {
          // 身份没被识别：不阻塞，交给真正的请求（429 会兜底真实额度）
          quotaCacheOk = true; quotaCacheUntil = now + 10000; return true
        }
        if (!res.ok) {
          quotaCacheOk = false; quotaCacheUntil = now + 10000; return false
        }
        const data = await res.json()
        quotaCacheOk = (data.quota?.remaining || 0) > 0
        quotaCacheUntil = now + 25000
        return quotaCacheOk
      } catch {
        // 网络异常：不阻塞（云端 429 兜底），避免连不上配额服务就误报已用完
        quotaCacheOk = true; quotaCacheUntil = 0
        return true
      }
    }

  // 公益免费流式：把上游 SSE 流进「handleSend 已即时建好并显示『正在思考』」的 flow。
    // 创建/回显已在 handleSend 完成，这里只装配请求 + 组装 blocks，避免重复创建。
    // 断连自动重试（08-28）：流式中断/网络错误时重新发起请求，最多 MAX_STREAM_RETRY 次，
    // 每次重试在 flow 里插一条「重新尝试连接神经网络 (n/m)」的可见提示（颜文字友好），
    // 已累积的回复文本保留不清空；全部撞完才判失败。
    const MAX_STREAM_RETRY = 3
    async function sendSharedPoolStream(flow, combined, model) {
      // 先确保游客 UID 已拿到并缓存，请求才带得上 X-Guest-Uid（否则云端 401）
      await ensureGuestUid()
      const retryMsg = (n, total) => `重新尝试连接神经网络 (${n}/${total})～`
      let accumulated = '' // 已流式累积的回复文本（重试时保留）
      let retryCount = 0
      for (; ; ) {
        const attempt = retryCount + 1
        const ok = await new Promise(resolve => {
          fetch('/api/chat/shared-pool', {
            method: 'POST',
            headers: sharedPoolAuthHeaders(),
            body: JSON.stringify({
              model: model,
              messages: [{ role: 'user', content: combined }],
              stream: true
            })
          }).then(async res => {
            if (res.status === 429) {
              const err = await res.json().catch(() => ({}))
              const used = err.quota?.used, limit = err.quota?.limit
              flow.status = 'failed'
              flow.blocks.push({
                type: 'intent',
                text: used != null
                  ? `😅 公益免费额度已用完（今日 ${used}/${limit} 次）\n\n填自己的 Key 继续使用，无限制～`
                  : `😅 上游免费模型暂时繁忙（429），稍等几分钟再试试～`
              })
              flow.endTime = Date.now()
              onStreamUpdate?.()
              resolve(false)
              return
            }
            if (res.status === 401 || res.status === 403) {
              // 云端没识别游客身份（极端情况：游客 UID 服务不可达）。
              // 公益免费不需要登录 —— 这是网络/云端问题，给平实提示并引导重试。
              flow.status = 'failed'
              flow.blocks.push({ type: 'intent', text: `😅 暂时连不上公益免费服务，请检查网络后重试～` })
              flow.endTime = Date.now()
              onStreamUpdate?.()
              resolve(false)
              return
            }
            if (!res.ok) {
              const err = await res.json().catch(() => ({ error: '请求失败' }))
              flow.status = 'failed'
              flow.blocks.push({ type: 'intent', text: `共享池错误：${err.error || '未知错误'}` })
              flow.endTime = Date.now()
              onStreamUpdate?.()
              resolve(false)
              return
            }
            // 流式读取 SSE
            const reader = res.body.getReader()
            const decoder = new TextDecoder()
            let buffer = ''
            const streamOk = await (async () => {
              try {
                while (true) {
                  const { done, value } = await reader.read()
                  if (done) break
                  buffer += decoder.decode(value, { stream: true })
                  const lines = buffer.split('\n')
                  buffer = lines.pop() || ''
                  for (const line of lines) {
                    if (!line.startsWith('data: ')) continue
                    const data = line.slice(6)
                    if (data === '[DONE]') { flow.status = 'completed'; return true }
                    try {
                      const parsed = JSON.parse(data)
                      const content = parsed.choices?.[0]?.delta?.content || ''
                      if (content) {
                        accumulated += content
                        const last = flow.blocks[flow.blocks.length - 1]
                        // 上一条是「重新尝试连接」提示块：用真实回复替换它，别把提示和正文黏在一起
                        if (last?.type === 'intent' && last.retryNote) {
                          flow.blocks[flow.blocks.length - 1] = { type: 'intent', text: content }
                        } else if (last?.type === 'intent') last.text += content
                        else flow.blocks.push({ type: 'intent', text: content })
                        onStreamUpdate?.()
                      }
                    } catch { }
                  }
                }
                flow.status = 'completed'
                flow.endTime = Date.now()
                onStreamUpdate?.()
                return true
              } catch (e) {
                // 流式中断（网络抖动/上游断流）：返回 false 走外层重试
                console.warn('shared-pool stream interrupted:', e)
                return false
              }
            })()
            resolve(streamOk)
          }).catch(err => {
            // fetch 层网络错误（连接失败）：也走外层重试
            console.warn('shared-pool fetch error:', err)
            resolve(false)
          })
        })
        if (ok) return
        // 到这里 = 本次尝试失败，判断还能不能重试
        retryCount++
        if (retryCount >= MAX_STREAM_RETRY) {
          flow.status = 'failed'
          flow.blocks.push({ type: 'intent', text: `😅 连接一直不稳定，试了 ${MAX_STREAM_RETRY} 次都没连上，稍等片刻再发一次试试～` })
          flow.endTime = Date.now()
          onStreamUpdate?.()
          return
        }
        // 重试前清理：若上次尝试已流出半截回复（最后一个 intent 块），删掉它，
        // 避免重试生成的完整回复和半截内容拼在一起重复；保留提示块等重试成功后替换
        const lastBlock = flow.blocks[flow.blocks.length - 1]
        if (lastBlock?.type === 'intent' && !lastBlock.retryNote) {
          flow.blocks.pop()
        }
        // 重试提示：插一条可见的友好提示（保留已累积回复），短暂停顿让用户看得见
        flow.blocks.push({ type: 'intent', text: retryMsg(attempt, MAX_STREAM_RETRY), retryNote: true })
        flow.status = 'running'
        onStreamUpdate?.()
        await new Promise(r => setTimeout(r, 900))
      }
    }

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
// 粘贴图片跟"+"菜单选图走同一条路：只当附件加进 attachments（复用 attachImageFile
// 的分析逻辑），发不发由用户自己按发送/回车决定。之前这里分析一完成就无条件
// handleSend()——分析本身是异步的（云端识图模型可能要几十秒），
// 用户趁等待打字问问题时，分析一结束就把这句话连同图一起抢发出去，用户根本没
// 机会确认。跟"+"菜单选图（onAttachFilesSelected）保持一致："先附加，用户自己
// 决定何时发送"，不再有这个隐藏的自动发送时机。
function handlePaste(e) {
  const items = e.clipboardData?.items
  if (!items) return
  let imageFile = null
  let videoFile = null
  for (const item of items) {
    if (item.type && item.type.startsWith('image/')) { imageFile = item.getAsFile(); break }
    if (item.type && item.type.startsWith('video/')) { videoFile = item.getAsFile(); break }
  }
  const file = imageFile || videoFile
  if (!file) return
  e.preventDefault()
  if (flowState.active) { showVisionError('工作流运行中，请稍后再粘贴图片/视频'); return }
  if (file.type.startsWith('video/')) attachVideoFile(file)
  else attachImageFile(file)
}

// ==================== "+" 附加菜单：添加文件/照片、添加文件夹 ====================
// 跟粘贴图片（handlePaste）共用同一套 vision-preprocess 接口和状态提示，但这里是
// "先附加、用户自己决定何时发送"
const attachFileInputRef = ref(null)
const attachFolderInputRef = ref(null)
const attachVideoInputRef = ref(null)

function triggerAttachFiles() { showAddMenu.value = false; attachFileInputRef.value?.click() }
function triggerAttachVideo() { showAddMenu.value = false; attachVideoInputRef.value?.click() }
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
// 只清空输入框上方的待发送 chip 列表，不 revoke blob URL——
// 唯一调用点在 handleSend()，此时这些 URL 已经被浅拷贝进 displayAttachments 塞进了
// 消息气泡（见下方 handleSend），气泡要一直能显示缩略图。之前这里连 URL 一起撤销，
// 气泡里的图片发出去那一刻就变裂图标（浅拷贝复制的是同一个 blob URL 字符串，撤销
// 是全局生效的，不是"给这个数组用的"就不影响别处）。真正要撤销 URL 的场景是用户
// 发送前点掉某个附件（removeAttachment），那时确实再没人会用到这张图了。
function clearAttachments() {
  attachments.value = []
}
const hasPendingAttachments = computed(() => attachments.value.some(a => a.status === 'analyzing'))

async function attachImageFile(file) {
  const id = ++attachmentSeq
  const previewUrl = URL.createObjectURL(file)
  attachments.value.push({ id, kind: 'image', name: file.name, status: 'analyzing', previewUrl })
  try {
    const base64 = await fileToBase64(file)
    // 设置面板「模型」页：统一模式下主模型兼管识图，分开模式下用单独配的识图模型；
    // 未配置时会自动默认选中第一个可用模型，用户可自行更换。
    const modelMode = localStorage.getItem('modelMode') === 'split' ? 'split' : 'unified'
    const visionModel = modelMode === 'split'
      ? (localStorage.getItem('visionModel') || '')
      : (localStorage.getItem('selectedModel') || '')
    const res = await fetch('/api/aether/vision-preprocess', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ image_base64: base64, mime_type: file.type || 'image/png', model: visionModel })
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || `识图请求失败 (${res.status})`)
    if (!data.text) throw new Error('未返回分析文本')
    const item = attachments.value.find(a => a.id === id)
    if (item) { item.status = 'ready'; item.analysisText = data.text }
  } catch (err) {
    const item = attachments.value.find(a => a.id === id)
    if (item) { item.status = 'error'; item.errorMsg = err?.message || '识图失败' }
  }
}

// 视频附件：与图片共用同一套「先附加、分析完成标 ready、用户自己决定发送」的流程，
// 只是走后端 /api/video/analyze（multipart 上传 → ffmpeg 抽帧 → 视觉模型链理解）。
async function attachVideoFile(file) {
  const id = ++attachmentSeq
  const previewUrl = URL.createObjectURL(file)
  attachments.value.push({ id, kind: 'video', name: file.name, status: 'analyzing', previewUrl })
  try {
    const modelMode = localStorage.getItem('modelMode') === 'split' ? 'split' : 'unified'
    const visionModel = modelMode === 'split'
      ? (localStorage.getItem('visionModel') || '')
      : (localStorage.getItem('selectedModel') || '')
    const fd = new FormData()
    fd.append('file', file)
    if (visionModel) fd.append('model', visionModel)
    const res = await fetch('/api/video/analyze', { method: 'POST', body: fd })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || `视频理解请求失败 (${res.status})`)
    if (!data.text) throw new Error('未返回分析文本')
    const item = attachments.value.find(a => a.id === id)
    if (item) { item.status = 'ready'; item.analysisText = data.text }
  } catch (err) {
    const item = attachments.value.find(a => a.id === id)
    if (item) { item.status = 'error'; item.errorMsg = err?.message || '视频理解失败' }
  }
}

async function attachTextFile(file) {
  const id = ++attachmentSeq
  // 只登记文件名，不读全文——发送时只把文件名带进消息，agent 在工作目录里自己 read_file。
  // 浏览器安全沙箱也拿不到真实磁盘路径，塞全文既撑爆上下文又无意义。
  attachments.value.push({ id, kind: 'file', name: file.name, ext: extOf(file.name), status: 'ready' })
}

function onAttachFilesSelected(e) {
  const files = Array.from(e.target.files || [])
  e.target.value = ''
  for (const file of files) {
    if (file.type && file.type.startsWith('image/')) attachImageFile(file)
    else if (file.type && file.type.startsWith('video/')) attachVideoFile(file)
    else attachTextFile(file)
  }
}

// 「添加视频」专用入口：菜单项触发 attachVideoInputRef（accept="video/*"）后走这里。
function onAttachVideoSelected(e) {
  const files = Array.from(e.target.files || [])
  e.target.value = ''
  for (const file of files) {
    attachVideoFile(file)
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

// 发送那一刻才把附件序列化进正文：图片用 vision 分析结果、文件夹用清单、
// 文本文件只给文件名（不塞全文——agent 在后端工作目录里自己 read_file 读取，
// 把整份源码怼进消息既撑爆上下文又没必要）。顺序固定放在用户文字前面。
function buildOutgoingMessage() {
  const blocks = attachments.value
    .filter(a => a.status === 'ready')
    .map(a => {
      if (a.kind === 'image') return `[图片: ${a.name}]\n${a.analysisText || ''}`
      if (a.kind === 'video') return `[视频: ${a.name}]\n${a.analysisText || ''}`
      if (a.kind === 'folder') return `[文件夹: ${a.name}，共 ${a.fileCount} 个文件]\n${a.manifest}`
      // 文本/代码文件：只给文件名，让 agent 自行 read_file，不把内容塞进消息
      return `[文件: ${a.name}]`
    })
  const typed = userInput.value.trim()
  return [...blocks, typed].filter(Boolean).join('\n')
}

const showScrollButton = computed(() => { return isOpen.value && userScrolledUp.value })

// 流式时给新到的助手消息做瀑布渐变 + 高亮代码块。
// ⚠️ 性能修复：原版 watch(messages, { deep: true }) 每 token 触发 → nextTick → streamFadePass()
// （重包 span）+ highlightAllCodeBlocks()（全 DOM 高亮）双重全量重做 = 思考流式巨卡。
// 改为节流合并（~120ms 最多一次）+ 只对「流式中的消息」做，避免逐 token 全量重查。
// 2026-09-02 修：原 watch 只依赖 messages.value.length，但 useAgentWorkflow 的 appendText
// 是原地 mutate blocks[].text，不改变数组长度 → 渐变永远不触发。补上 blocks 文本总长度检测。
// 2026-09-03 修：原来写的 flowState.blocks 是 undefined（flowState 只有 active 字段），
// 导致依赖恒为 0，watcher 仍只被 length 驱动——工作流流式期间 highlightAllCodeBlocks 永不
// 触发，代码块没有复制按钮/高亮，直到下一条消息才全量补跑。改为直接统计 messages 里
// agentflow 消息的 blocks 文本总长度，streaming 期间每个增量都会触发。
let streamPassTimer = null
watch(
  () => messages.value.length + '|' + messages.value.reduce((s, m) => {
    if (m?.kind !== 'agentflow' || !Array.isArray(m.blocks)) return s
    return s + m.blocks.reduce((x, b) => x + (b.text ? b.text.length : 0), 0)
  }, 0),
  () => {
    if (streamPassTimer) return
    streamPassTimer = setTimeout(() => {
      streamPassTimer = null
      nextTick(() => {
        streamFadePass()
        highlightAllCodeBlocks()
      })
    }, 120)
  }
)
watch(
  [activeSession, () => currentWorkDir.value.path],
  () => {
    boundAgentFSKey = ''
    closeAgentFSDiff()
    refreshAgentFSTimeline()
  }
)
watch(sidebarOpen, open => {
  if (!open) refreshAgentFSTimeline()
  else closeAgentFSDiff()
})
watch(dockPanels, panels => {
  if (panels.length) closeAgentFSDiff()
  else if (!sidebarOpen.value) refreshAgentFSTimeline()
})
watch(agentFSTimeline, () => nextTick(syncAgentFSTreeViewport), { flush: 'post' })
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
  refreshGitGraph()
  loadWorkDirState()
  syncWorkDirFromBackend()
  loadChatHistory()
  refreshAgentFSTimeline()
  ensureMermaidObserver()
  agentFSPollTimer = window.setInterval(() => {
    if (!sidebarOpen.value) refreshAgentFSTimeline()
  }, 4000)
  document.addEventListener('click', () => {
    showModelMenu.value = false; showTokenPanel.value = false
    showAddMenu.value = false; showPrMenu.value = false; showWorkDirMenu.value = false; showBranchMenu.value = false
    showDockAddMenu.value = false
    closeAgentFSDiff()
  })
  window.addEventListener('keydown', onGlobalDockShortcut)
  window.addEventListener('resize', syncAgentFSTreeViewport)
  nextTick(syncAgentFSTreeViewport)
  // 监听会话标题更新事件（来自 useAgentWorkflow 的 onTitleUpdate）
  window.addEventListener('session-title-update', onSessionTitleUpdate)
  // 监听 AI 标题生成中事件：列表刷新时保持「新对话」，避免后端派生的原文标题抢先替换
    window.addEventListener('session-title-pending', onSessionTitlePending)
    startNotifPoll()
    // 右键菜单「粘贴」如果剪贴板是图片，DesktopFloatingMenu 会 dispatch paste-image
      window.addEventListener('paste-image', onPasteImageFromMenu)
      // 工作流建议按钮：AgentWorkflowPanel 点 follow-up 建议时抛事件，这里填入输入框并发送
      window.addEventListener('workflow-suggestion', onWorkflowSuggestion)
    })
    onUnmounted(() => {
      window.removeEventListener('keydown', onGlobalDockShortcut)
      window.removeEventListener('resize', syncAgentFSTreeViewport)
      window.clearInterval(agentFSPollTimer)
      window.removeEventListener('session-title-update', onSessionTitleUpdate)
      window.removeEventListener('session-title-pending', onSessionTitlePending)
      window.removeEventListener('paste-image', onPasteImageFromMenu)
      window.removeEventListener('workflow-suggestion', onWorkflowSuggestion)
      stopNotifPoll()
    })
    // 工作流建议按钮点击：把建议文本填进输入框并发送，用户不用打字。
    function onWorkflowSuggestion(e) {
    const text = e.detail && e.detail.text
    if (!text || !text.trim()) return
    userInput.value = text.trim()
    nextTick(() => {
      if (chatInputRef.value) {
        chatInputRef.value.style.height = 'auto'
        adjustInputHeight()
      }
      handleSend()
    })
    }
// 右键菜单粘贴图片：与 Ctrl+V（handlePaste）走同一条附件路
function onPasteImageFromMenu(e) {
  const file = e.detail && e.detail.file
  if (!file) return
  if (flowState.active) { showVisionError('工作流运行中，请稍后再粘贴图片'); return }
  attachImageFile(file)
}
async function refreshGitGraph() {
  try {
    const res = await fetch('/api/git/graph')
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '读取 Git 图失败')
    gitGraph.value = { commits: Array.isArray(data.commits) ? data.commits : [], current_branch: data.current_branch || '' }
  } catch (err) {
    console.warn('读取 Git 分支图失败', err)
    gitGraph.value = { commits: [], current_branch: '' }
  }
}
</script>

<style scoped>
@import '../../../styles/shanxi/chat-window.css';

/* 加固：.chat-window 本身是 position:fixed，理论上不会撑高这个根节点，
   但显式约束一下成本很低，避免任何万一 */
.chat-widget-root { height: 100%; overflow: hidden; }

/* 后台任务/子代理悬浮条：输入框上方，有进行中任务时显示，紧贴输入框上沿 */
.input-wrapper > .input-running-tasks-bar {
  position: absolute;
  /* 全宽 + 居中 + max-width 封顶，跟输入框卡片 / todo-bar 同宽同中线 */
  left: 0;
  right: 0;
  width: 100%;
  max-width: 720px;
  margin: 0 auto;
  box-sizing: border-box;
  bottom: 100%;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 7px 12px;
  font-size: 12.5px;
  color: var(--app-accent);
  background: color-mix(in srgb, var(--app-accent) 10%, var(--app-surface));
  border: 1px solid color-mix(in srgb, var(--app-accent) 35%, transparent);
  border-bottom: none;
  border-radius: 0;
  cursor: pointer;
  transition: background 0.2s ease;
}
.input-wrapper > .input-running-tasks-bar:hover { background: color-mix(in srgb, var(--app-accent) 16%, var(--app-surface)); }
.rt-spin { animation: rt-spin 1s linear infinite; }
@keyframes rt-spin { to { transform: rotate(360deg); } }
.rt-sub { color: var(--app-text-faint); font-size: 11px; }
.rt-caret { margin-left: auto; opacity: 0.6; }

/* 切换历史时强制隐藏 placeholder（不经过 transition 延时） */
.input-row.input-swapping .input-placeholder-text {
  opacity: 0 !important;
  transition: none !important;
}
/* 自适应占位符的绝对定位 */
.input-placeholder-text {
  position: absolute;
  left: 44px;             /* 16px 容器内边距 + 22px 左侧加号按钮宽度 + 6px 间距，避开加号不重叠 */
  top: 10px;              /* 和 textarea 的 padding-top 保持一致 */
  pointer-events: none;   /* 确保鼠标点击能直接穿透进 textarea */
  color: var(--app-text-faint);
  font-size: 15px;
  font-family: inherit;
  z-index: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 80%;         /* 防止占位符太长时覆盖到右侧的按钮 */
}

/* 输入栏左侧加号：裸图标，无圆框无背景（Hermes 布局） */
.input-left-plus {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 26px;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--app-text-soft);
  cursor: pointer;
  flex-shrink: 0;
  border-radius: 0;
  transition: color .15s ease;
}
.input-left-plus:hover { color: var(--app-text); }

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

/* ==================== 魔女审判 · 聊天组件氛围覆盖 ==================== */

/* 用户消息：火印边框 + 内发光 */
[data-skin="witchtrial"] .message-bubble.user {
  position: relative;
  background: linear-gradient(145deg, rgba(40, 20, 24, 0.95), rgba(28, 14, 18, 0.98));
  border: 1.5px solid rgba(199, 62, 62, 0.55);
  box-shadow:
    0 0 0 1px rgba(0, 0, 0, 0.45),
    0 4px 16px rgba(0, 0, 0, 0.35),
    inset 0 0 18px rgba(199, 62, 62, 0.08);
  color: #f5e6e0;
  border-radius: 14px 14px 4px 14px;
}
[data-skin="witchtrial"] .message-bubble.user::before {
  content: '';
  position: absolute;
  inset: -2px;
  border-radius: 15px 15px 5px 15px;
  background: linear-gradient(135deg, rgba(199, 62, 62, 0.45), transparent 50%, rgba(199, 62, 62, 0.2));
  z-index: -1;
  filter: blur(4px);
  opacity: 0.8;
}

/* AI 消息：羊皮纸/古籍质感 */
[data-skin="witchtrial"] .assistant-message {
  background: linear-gradient(180deg, rgba(30, 24, 28, 0.96), rgba(24, 18, 22, 0.98));
  border: 1px solid rgba(139, 110, 90, 0.25);
  border-radius: 14px;
  box-shadow: inset 0 0 30px rgba(139, 110, 90, 0.05), 0 4px 16px rgba(0, 0, 0, 0.3);
}
[data-skin="witchtrial"] .assistant-message .markdown-body {
  color: #e8ddd0;
}
[data-skin="witchtrial"] .assistant-message .markdown-body code {
  background: rgba(0, 0, 0, 0.35);
  color: #e8a89a;
}

/* 工具调用条：暗红印章感 */
[data-skin="witchtrial"] .tool-call-indicator {
  background: rgba(199, 62, 62, 0.12);
  border: 1px solid rgba(199, 62, 62, 0.25);
  color: #e8a8a0;
}

/* 正在思考：火焰脉冲 */
[data-skin="witchtrial"] .reasoning-stream .reasoning-label {
  color: #d98a7a;
  text-shadow: 0 0 8px rgba(199, 62, 62, 0.35);
}
[data-skin="witchtrial"] .reasoning-stream {
  border-left: 2px solid rgba(199, 62, 62, 0.4);
  background: rgba(199, 62, 62, 0.05);
}

/* 工具/步骤卡片：铁链暗框 */
[data-skin="witchtrial"] .step-card,
[data-skin="witchtrial"] .agent-step {
  border-color: rgba(199, 62, 62, 0.2);
  background: rgba(20, 14, 18, 0.7);
}

/* ==================== 二阶堂希罗 · 红黑洛丽塔 ==================== */

/* 用户消息：粉丝绒圆润气泡 + 蝴蝶结 */
[data-skin="witchtrial_hiiro"] .message-bubble.user {
  position: relative;
  background: linear-gradient(145deg, rgba(90, 22, 48, 0.95), rgba(48, 14, 28, 0.98));
  border: 1.5px solid rgba(233, 30, 99, 0.6);
  border-radius: 22px 22px 4px 22px;
  box-shadow:
    0 0 0 1px rgba(0, 0, 0, 0.4),
    0 4px 18px rgba(0, 0, 0, 0.3),
    0 0 22px rgba(233, 30, 99, 0.18);
  color: #fff5f8;
}

/* 用户消息被导航轴选中：只加亮边框，去掉发光变化，避免跳转后"一闪" */
[data-skin="witchtrial_hiiro"] .message-bubble.user.active {
  border-color: rgba(255, 160, 180, 0.95);
}

/* 右上角蝴蝶结 */
[data-skin="witchtrial_hiiro"] .message-bubble.user::before {
  content: '🎀';
  position: absolute;
  top: -10px;
  right: -8px;
  font-size: 22px;
  line-height: 1;
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.45));
  pointer-events: none;
  z-index: 2;
  animation: hiiro-bow-sway 2.8s ease-in-out infinite;
  transform-origin: 50% 80%;
}

@keyframes hiiro-bow-sway {
  0%, 100% { transform: rotate(-6deg) scale(1); }
  50% { transform: rotate(6deg) scale(1.05); }
}

/* AI 消息：暗粉蕾丝羊皮纸 */
[data-skin="witchtrial_hiiro"] .assistant-message {
  background: linear-gradient(180deg, rgba(55, 26, 38, 0.96), rgba(38, 18, 26, 0.98));
  border: 1px solid rgba(240, 98, 146, 0.22);
  border-radius: 20px;
  box-shadow: inset 0 0 32px rgba(233, 30, 99, 0.06), 0 4px 16px rgba(0, 0, 0, 0.28);
}
[data-skin="witchtrial_hiiro"] .assistant-message .markdown-body {
  color: #ffeef4;
}
[data-skin="witchtrial_hiiro"] .assistant-message .markdown-body code {
  background: rgba(0, 0, 0, 0.32);
  color: #ff9ec4;
}

/* 工具调用条：蕾丝印章 */
[data-skin="witchtrial_hiiro"] .tool-call-indicator {
  background: rgba(233, 30, 99, 0.14);
  border: 1px solid rgba(240, 98, 146, 0.25);
  color: #ff9ec4;
  border-radius: 12px;
}

/* 正在思考：粉柔光 */
[data-skin="witchtrial_hiiro"] .reasoning-stream .reasoning-label {
  color: #ff9ec4;
}
[data-skin="witchtrial_hiiro"] .reasoning-stream {
  border-left: 2px solid rgba(233, 30, 99, 0.45);
  background: rgba(233, 30, 99, 0.06);
  border-radius: 0 12px 12px 0;
}

/* 工具/步骤卡片：洛丽塔暗框 */
[data-skin="witchtrial_hiiro"] .step-card,
[data-skin="witchtrial_hiiro"] .agent-step {
  border-color: rgba(233, 30, 99, 0.22);
  background: rgba(38, 16, 26, 0.74);
  border-radius: 14px;
}

/* 用户消息文本：点击即编辑，给出明确可点反馈 */
.msg-user-text {
  cursor: pointer;
  user-select: text;
}
.msg-user-text:hover {
  filter: brightness(1.08);
}

/* 字体设置必须同时作用于机器人消息及其 Markdown 内容。
 * 这里放在组件样式最后，覆盖 chat-window.css 中遗留的 16/17px 固定值。 */
.chat-window .chat-messages .assistant-message,
.chat-window .chat-messages .assistant-message .markdown-body {
  font-size: 16px !important;
}
.chat-window .chat-messages .assistant-message .markdown-body p,
.chat-window .chat-messages .assistant-message .markdown-body li,
.chat-window .chat-messages .assistant-message .markdown-body blockquote,
.chat-window .chat-messages .assistant-message .markdown-body table,
.chat-window .chat-messages .assistant-message .markdown-body pre {
  font-size: inherit !important;
}
.chat-window .chat-input-area textarea.chat-input {
  font-size: 16px !important;
  line-height: 1.6 !important;
}

/* ===== 通知面板 ===== */
.mail-panel-backdrop {
  position: fixed;
  inset: 0;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(4px);
}
.mail-panel {
  width: 480px;
  max-width: 92vw;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  background: var(--app-surface, #1e293b);
  border: 1px solid var(--app-border, #334155);
  border-radius: 16px;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.35);
}
.mail-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--app-border, #334155);
}
.mail-panel-head h2 {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 700;
  color: var(--app-text, #f1f5f9);
  margin: 0;
}
.mail-panel-close {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--app-text-soft, #94a3b8);
  cursor: pointer;
  transition: background .15s ease, color .15s ease;
}
.mail-panel-close:hover {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 92%);
  color: var(--app-text, #f1f5f9);
}
.mail-panel-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
.mail-panel-act-btn {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--app-text-soft, #94a3b8);
  cursor: pointer;
  transition: background .15s ease, color .15s ease;
}
.mail-panel-act-btn:hover {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 92%);
  color: var(--app-text, #f1f5f9);
}
.mail-panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
  min-height: 200px;
}
.mail-panel-empty {
  text-align: center;
  color: var(--app-text-faint, #64748b);
  font-size: 14px;
  padding: 48px 0;
}
/* 通知行 */
.mail-notif-row {
  display: flex;
  gap: 10px;
  padding: 12px 20px;
  border-bottom: 1px solid color-mix(in srgb, var(--app-border, #334155), transparent 60%);
  transition: background .15s ease;
}
.mail-notif-row:hover {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 96%);
}
.mail-notif-row.unread {
  background: color-mix(in srgb, var(--app-accent), transparent 94%);
}
.mail-notif-row.unread:hover {
  background: color-mix(in srgb, var(--app-accent), transparent 90%);
}
.mail-notif-dot {
  width: 8px;
  height: 8px;
  min-width: 8px;
  margin-top: 5px;
  border-radius: 50%;
  background: var(--app-accent, #6366f1);
}
.mail-notif-content {
  flex: 1;
  min-width: 0;
}
.mail-notif-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text, #f1f5f9);
  margin-bottom: 2px;
}
.mail-notif-body {
  font-size: 13px;
  color: var(--app-text-2, #94a3b8);
  line-height: 1.5;
  margin-bottom: 4px;
}
.mail-notif-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--app-text-faint, #64748b);
}
.mail-notif-type {
  padding: 1px 6px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--app-text, #202124), transparent 92%);
}

/* ★ 历史切换淡入动画（chat-input-swap 类被 JS flashInputSwap 使用，但从未定义过 CSS → 补上） */
.chat-input-swap {
  animation: chatInputSwap 0.22s ease-out;
}
@keyframes chatInputSwap {
  from { opacity: 0.35; transform: translateY(-3px); }
  to   { opacity: 1; transform: translateY(0); }
}
</style>
