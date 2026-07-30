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
                <div class="gem-search-section-label">近期对话</div>
                <div v-if="filteredSearchSessions.length === 0" class="gem-search-empty">
                  未找到匹配对话
                </div>
                <button
                  v-for="s in filteredSearchSessions"
                  :key="s.id"
                  class="gem-search-row"
                  :class="{ active: s.id === activeSession }"
                  @click="onSearchSelect(s.id)"
                >
                  <span class="gem-search-row-name">{{ s.name }}</span>
                  <span class="gem-search-row-date">{{ formatSearchDate(s.updatedAt) }}</span>
                </button>
              </div>
            </div>
          </Transition>

          <!-- 顶部：品牌标识 + 折叠toggle -->
                    <div class="gem-top">
                                <div class="gem-brand-wrap">
                                  <a href="/" class="gem-icon-btn gem-home" title="首页">
                                    <Icon icon="majesticons:shooting-star-line" width="20" />
                                  </a>
                                  <span v-if="sidebarOpen" class="gem-brand-text">Rescene</span>
                                </div>
                                <button class="gem-icon-btn gem-collapse" @click="toggleSidebar" :title="sidebarOpen ? '折叠边栏' : '打开边栏'">
                                  <Icon icon="lucide:sidebar" width="18" />
                                </button>
                              </div>

          <!-- 展开态：复用会话面板（新对话/搜索/置顶/最近/底部账号+设置） -->
          <SessionMenuContent
                      v-if="sidebarOpen"
                      fill
                      :sessions="sessionList"
                      :active-session="activeSession"
                      :running-session="runningSession"
                      :completed-sessions="completedSessions"
                      :question-session="questionSession"
            @select-session="selectSession"
            @new-session="newSession"
            @rename-session="renameSession"
            @delete-session="deleteSession"
            @open-settings="showSettings = true"
            @open-search="openSearchPanel"
                        @open-plugins="openPluginsMarket"
                        @open-scheduled-tasks="showScheduledTask = true"
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
                        <button class="gem-icon-btn" @click="showScheduledTask = true" title="定时任务">
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
                          <button class="gem-icon-btn" :class="{ active: showGitDeliveryAgent }" @click="openGitDeliveryAgent" title="Agent 工作台">
                <Icon icon="mdi:robot-outline" width="18" />
              </button>
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

            <!-- 右上角工具组：顶部横条删除后浮在聊天区右上（Code 模式才有意义）。
                 工具窗口（终端/Diff/预览）打开后丝滑变成竖条贴靠面板边缘，DOM 顺序不变，
                 更多(三点)本来就排最后，变竖条后自然落在底部。 -->
            <div
              v-if="inputTopBarMode === 'git'"
              class="floating-tools"
              :class="{ vertical: dockPanels.length > 0 && !dockHidden, 'collapsed-dock': dockPanels.length > 0 && dockHidden }"
            >
              <button
                v-if="dockPanels.length > 0 && dockHidden"
                class="header-icon-btn dock-restore-btn"
                type="button"
                title="展开工具坞"
                @click.stop="toggleDockHidden"
              >
                <Icon icon="mdi:chevron-left" width="19" color="#6b6b6b" />
              </button>
              <template v-else>
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
              </template>
            </div>

            <!-- 重构：将 Home 组件从 `chat-messages` 中剥离，作为 `chat-content` 的直接子节点。
                 当 `messages` 为空时，它独占整个 Flex 空间，把输入区推到最底部。 -->
            <div v-if="messages.length === 0" class="home-container-for-layout">
              <NewSessionHome />
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
                      <!-- 朗读/复制那一栏：以前只挂在纯文本 assistant 气泡上，而现在所有回复
                           都走四态机(agentflow)，等于这一栏彻底消失了。跑完再显示，跑的过程中
                           内容还在变，复制没意义。 -->
                      <div v-if="item.status === 'completed' && flowFinalText(item)" class="flow-tools">
                        <button class="tool-btn" @click="playVoice(flowFinalText(item))" title="朗读">
                          <Icon icon="mdi:volume-high" width="16" />
                        </button>
                        <button class="tool-btn" @click="copyText(flowFinalText(item))" title="复制">
                          <Icon icon="mdi:content-copy" width="16" />
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
              <!-- 底部边缘 blur：sticky 吸在滚动视口底边，正好压在输入区上沿 -->
              <div class="msg-edge-blur bottom"></div>
            </div>

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
                <div v-if="todoState.items.length" class="todo-bar">
                <div class="todo-bar-head">
                  <Icon icon="mdi:format-list-checks" width="14" class="todo-bar-icon" />
                  <span class="todo-bar-title">任务清单</span>
                  <span class="todo-bar-progress">{{ todoDoneCount }}/{{ todoState.items.length }}</span>
                </div>
                <ul class="todo-bar-list">
                  <li
                    v-for="(it, i) in todoState.items"
                    :key="i"
                    class="todo-bar-item"
                    :class="'todo-' + it.status"
                  >
                    <span class="todo-bar-mark">{{ it.status === 'done' ? '☑' : it.status === 'doing' ? '▶' : '☐' }}</span>
                    <span class="todo-bar-text">{{ it.text }}</span>
                  </li>
                </ul>
              </div>
              </Transition>

              <!-- Agent 提问：与审批条同层，直接贴在输入框上方，选单选项后立即提交。 -->
              <QuestionModal
                v-if="questionState.pending"
                :question="questionState.pending"
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

              <!-- ===== 断点续跑条 =====
                   后端每轮落盘检查点，重启/断线后这里显示上次没跑完的任务。
                   续跑复用原 workflow_id 和原模型，从断点那一轮接着问，
                   已经跑完的工具不会重跑。 -->
              <div v-if="resumeState.pending && !flowState.active" class="resume-bar">
                <Icon icon="mdi:history" width="15" class="resume-bar-icon" />
                <div class="resume-bar-main">
                  <div class="resume-bar-line">
                    <span class="resume-bar-label">上次任务未跑完</span>
                    <span class="resume-bar-round">第 {{ resumeState.pending.round }} 轮中断</span>
                  </div>
                  <div class="resume-bar-task" :title="resumeState.pending.task">{{ resumeState.pending.task }}</div>
                </div>
                <button class="resume-bar-btn ghost" @click="dismissResumable">放弃</button>
                <button class="resume-bar-btn primary" @click="resumeCodeWorkflow">续跑</button>
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
                      <div class="pr-menu-item" @click="openGitDeliveryAgent">
                        <Icon icon="mdi:robot-outline" width="15" />
                        <span>交付 Agent Git</span>
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
                    <button
                      class="toolbar-pill-btn mode-pill"
                      :class="{ 'mode-yolo': agentModeIsYolo, 'mode-idle': !agentModeIsYolo }"
                      @click.stop="showAutoMenu = !showAutoMenu"
                    >
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

                <!-- 用户消息导航轴：占据工具栏中间的弹性空间 -->
                <UserMessageRail :messages="messages" :active-id="activeUserMessageId" @jump="jumpToMessage" />

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

                  <!-- 模型名 pill -->
                  <div class="sch-model" @click.stop="showModelMenu = !showModelMenu">
                    <span>{{ modelOptions.find(m => m.value === selectedModel)?.label || (hasModels ? '模型' : '无可用模型') }}</span>
                    <Icon icon="mdi:chevron-down" width="14" class="sch-model-caret" />
                    <div v-if="showModelMenu" class="model-menu-dropdown" @click.stop>
                      <div class="model-menu-search">
                        <Icon icon="mdi:magnify" width="14" class="model-menu-search-icon" />
                        <input v-model="modelSearch" type="text" placeholder="搜索模型" class="model-menu-search-input" @click.stop />
                      </div>
                      <div class="model-menu-footer" @click.stop="showModelManager = true; showModelMenu = false">
                        <Icon icon="mdi:cog-outline" width="14" /> 编辑模型...
                      </div>
                      <div v-if="!hasModels" class="model-menu-empty">没有可用模型（去设置填 Key 或选免 Key 模型）</div>
                      <template v-for="grp in filteredGroupedOptions" :key="grp.vendor">
                        <div class="model-menu-group-title">{{ grp.vendor }}</div>
                        <div
                          v-for="m in grp.items"
                          :key="m.value"
                          class="model-menu-item"
                          :class="{ active: selectedModel === m.value }"
                          @click="selectModel(m.value)"
                        ><span class="model-menu-check" v-if="selectedModel === m.value">✓</span><span>{{ m.label }}</span></div>
                      </template>
                    </div>
                  </div>

                  <!-- 模式 pill（effort） -->
                  <div v-if="currentCapability.reasoning" ref="effortWidgetRef" class="effort-widget" @click.stop="showEffortPanel = !showEffortPanel">
                    <span class="effort-value">{{ effortLabel }}</span>
                  </div>
                  <!-- 热力图日历按钮 -->
                  <button class="toolbar-icon-pill-btn" @click.stop="showHeatmapPopup = !showHeatmapPopup" title="活动热力图">
                    <Icon icon="mdi:calendar-month-outline" width="15" />
                  </button>
                  <HeatmapPopup v-model:visible="showHeatmapPopup" />
                  <Teleport to="body">
                    <div v-if="showEffortPanel" class="effort-panel" :style="effortPanelPos" @click.stop>
                      <div class="effort-panel-title">
                        Effort <b>{{ modelOptions.find(m => m.value === selectedModel)?.label || '' }}</b>
                      </div>
                      <div class="effort-slider-row">
                        <span class="effort-end">Faster</span>
                        <input type="range" min="0" max="2" step="1" v-model.number="effortLevel" class="effort-slider" @click.stop @input="onEffortChange" />
                        <span class="effort-end">Smarter</span>
                      </div>
                    </div>
                  </Teleport>
                </div>
              </div>
            </div>
          </div>

                    <!-- ★ AIStudio 右：多面板停靠 -->
          <aside
            class="tool-panel tool-panel-tabbed"
            v-if="isExpanded && dockPanels.length"
            :class="{ hidden: dockHidden, expanded: dockExpanded }"
            :style="dockExpanded || dockHidden ? {} : { width: dockWidth + 'px' }"
          >
            <div class="tool-dock-resize-handle" @mousedown="startDockWidthDrag"></div>
            <div class="tool-dock-tabs">
              <button
                v-for="panelKey in dockPanels"
                :key="panelKey"
                class="tool-dock-tab"
                :class="{ active: activeDockPanel === panelKey }"
                @click="setActiveDockPanel(panelKey)"
                :title="dockPanelLabel(panelKey)"
              >
                <Icon :icon="dockPanelIcon(panelKey)" width="14" class="tool-dock-tab-icon" />
                <span class="tool-dock-tab-label">{{ dockPanelLabel(panelKey) }}</span>
                <span class="tool-dock-tab-close" @click.stop="closeDockPanel(panelKey)">
                  <Icon icon="mdi:close" width="12" />
                </span>
              </button>
              <div class="tool-dock-tab-actions">
                <button ref="dockAddBtnRef" class="tool-dock-tab-add" @click.stop="toggleDockAddMenu" title="新建工具标签页">
                  <Icon icon="mdi:plus" width="16" />
                </button>
              </div>
              <!-- Teleport 到 body：.tool-dock-tabs 设了 overflow-x:auto，按 CSS 规范
                   overflow-x/overflow-y 只要有一个不是 visible，另一个会被隐式当 auto——
                   等于整条标签栏纵向也裁切。之前挂在标签栏内部 position:absolute 的菜单，
                   状态确实翻转了，但会被这层隐式裁切吃掉，看起来就是"点了没反应"。
                   CodeEditor.vue 的右键菜单也是因为同样的坑才 Teleport 到 body 的。 -->
              <Teleport to="body">
                <div v-if="showDockAddMenu" class="tool-dock-add-menu" :style="dockAddMenuStyle" @click.stop>
                  <button
                    v-for="option in dockPanelOptions"
                    :key="option.key"
                    class="tool-dock-add-item"
                    @click="openDockPanel(option.key)"
                  >
                    <span class="tool-dock-add-item-left">
                      <Icon :icon="option.icon" width="15" />
                      <span>{{ option.label }}</span>
                    </span>
                    <span v-if="dockPanels.includes(option.key)" class="tool-dock-add-badge">已打开</span>
                    <span v-else-if="option.shortcut" class="tool-dock-add-shortcut">{{ option.shortcut }}</span>
                  </button>
                </div>
              </Teleport>
              <!-- 标签组最右侧：放大(占满工作区宽度) / 收起整块工具坞(不清空已开的标签，
                   随便点一个 终端/预览/… 就能叫回来) —— 独立于标签组本身，跟 Cursor
                   顶栏最右那两个图标一个位置。 -->
              <div class="tool-dock-global-actions">
                <button class="tool-dock-tab-add" @click.stop="toggleDockExpanded" :title="dockExpanded ? '还原宽度' : '放大'">
                  <Icon :icon="dockExpanded ? 'mdi:arrow-collapse' : 'mdi:arrow-expand'" width="15" />
                </button>
                <button class="tool-dock-tab-add" @click.stop="toggleDockHidden" title="隐藏工具坞">
                  <Icon icon="mdi:dock-right" width="15" />
                </button>
              </div>
            </div>
            <div class="tool-dock-pane tool-dock-pane-single">
              <div class="tool-dock-pane-body">
                <DiffPanel v-if="activeDockPanel === 'diff'" />
                <div v-else-if="activeDockPanel === 'terminal'" class="terminal-group">
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
                <PreviewBrowser v-else-if="activeDockPanel === 'preview'" />
                <FileToolPanel v-else-if="activeDockPanel === 'file'" :embedded="true" />
                <BackgroundTasksPanel v-else-if="activeDockPanel === 'tasks'" :embedded="true" :tasks="backgroundTaskList" @select-task="jumpToGroup" />
              </div>
            </div>
          </aside>
        </div>
      </div>
      </div>

      <SettingsModal v-if="showSettings" @close="onSettingsClosed" />
      <ScheduledTaskModal v-if="showScheduledTask" @close="showScheduledTask = false" @create="onCreateScheduledTask" />
      <ModelManagerModal
        v-if="showModelManager"
        :free-models="freeModelsFull"
        :loading="false"
        @close="showModelManager = false"
        @add-provider="showModelManager = false; showSettings = true"
      />
      <div v-if="gitActionMessage" class="git-action-toast">{{ gitActionMessage }}</div>
      <GitDeliveryAgent v-if="showGitDeliveryAgent" @close="showGitDeliveryAgent = false" />

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

  </div>
</template>

<script setup>
import { ref, watch, nextTick, computed, onMounted, onUnmounted } from 'vue'
import GitDeliveryAgent from './GitDeliveryAgent.vue'
import { Icon } from '@iconify/vue'
import hljs from 'highlight.js'
import 'highlight.js/styles/atom-one-dark.min.css'
import 'katex/dist/katex.min.css'
import { renderMarkdown } from './markdownRenderer.js'
import { streamFadeConfig } from '../composables/streamFadeConfig.js'
import { previewRequest } from '../composables/previewBus.js'
import UserMessageRail from './UserMessageRail.vue'
import { useChatWidget } from './useChatWidget.js'
import { useResizableWidth } from './useResizable.js'
import SessionList from './SessionList.vue'
import SessionMenuContent from './SessionMenuContent.vue'
import ScheduledTaskModal from './ScheduledTaskModal.vue'
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
import HeatmapPopup from './HeatmapPopup.vue'
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
  if (approvalState.pending.length > 0 || resumeState.pending || questionState.pending) {
    return activeSession.value
  }
  return ''
})

function shortTitle(title) {
  title = (title || '新对话').trim()
  return title.length > 24 ? title.slice(0, 24) + '…' : title
}

async function loadSessionList() {
  try {
    const res = await fetch('/api/sessions')
    const data = await res.json()
    // parentId/forkIndex 是侧栏拼分支树用的血缘（后端 SessionInfo 带下来，根会话为空）
    const real = (data || []).map(s => ({
      id: s.id, name: shortTitle(s.title),
      parentId: s.parent_id || '', forkIndex: s.fork_index || 0, updatedAt: s.updated_at,
      workdir: getSessionWorkdir(s.id)
    }))
    // 当前会话哪怕还一条消息都没有（刚新建/刚打开应用）也要出现在列表里，
    // 不然侧栏在"发第一条消息之前"会看不到自己正在哪个会话上。
    // 注意别把刚分叉出来的分支覆盖成无名根——confirmEdit 已经乐观插入过带血缘的条目了
    if (!real.some(s => s.id === sessionId.value)) {
      const optimistic = sessionList.value.find(s => s.id === sessionId.value)
      real.unshift(optimistic || { id: sessionId.value, name: '新对话', parentId: '', forkIndex: 0 })
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
// ==================== 工作目录 → 会话分组 ====================
const WD_MAP_KEY = 'shanxi_session_workdir'

function loadWorkdirMapping() {
  try { return JSON.parse(localStorage.getItem(WD_MAP_KEY) || '{}') } catch { return {} }
}
function saveWorkdirMapping(m) {
  try { localStorage.setItem(WD_MAP_KEY, JSON.stringify(m)) } catch {}
}
function getSessionWorkdir(sid) {
  const mapped = loadWorkdirMapping()[sid]
  if (mapped) return mapped
  const fallback = currentWorkDir.value?.name?.trim() || ''
  if (fallback) {
    const m = loadWorkdirMapping()
    m[sid] = fallback
    saveWorkdirMapping(m)
  }
  return fallback
}
function recordSessionWorkdir(sid) {
  const wd = currentWorkDir.value?.name?.trim()
  if (!wd) return
  const m = loadWorkdirMapping()
  m[sid] = wd
  saveWorkdirMapping(m)
}

function newSession() {
  const id = 'sess_' + Date.now().toString(36)
  sessionList.value = [{ id, name: '新对话', parentId: '', forkIndex: 0 }, ...sessionList.value]
  // 记录当前工作目录到会话映射
  recordSessionWorkdir(id)
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
  // 清理 workdir 映射
  const wm = loadWorkdirMapping()
  delete wm[id]
  saveWorkdirMapping(wm)
  // 重新拉而不是本地 filter：被删会话的分支在后端会被提升为根会话，
  // 本地 filter 的话那些分支还挂着指向已删父会话的 parentId，在树里会变成孤儿
  await loadSessionList()
  if (activeSession.value === id) {
    const next = sessionList.value.find(s => s.id !== id)?.id || ('sess_' + Date.now().toString(36))
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

// ==================== 右侧工具面板（多面板停靠） ====================
const dockPanels = ref([])
const activeDockPanel = ref('')
const dockWidth = ref(380)
const { startDrag: startDockWidthDrag } = useResizableWidth(dockWidth, { min: 300, max: 720, edge: 'left', persistKey: 'dockWidth' })
const showDockAddMenu = ref(false)
const dockAddBtnRef = ref(null)
const dockAddMenuStyle = ref({})

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
function ensureActiveDockPanel() {
  if (!dockPanels.value.length) {
    activeDockPanel.value = ''
    showDockAddMenu.value = false
    return
  }
  if (!dockPanels.value.includes(activeDockPanel.value)) {
    activeDockPanel.value = dockPanels.value[dockPanels.value.length - 1]
  }
}
function setActiveDockPanel(key) {
  activeDockPanel.value = key
}
// 放大：工具坞宽度撑到接近整个工作区（放弃 dockWidth 那个手动拖出来的值，
// 再点一次收回去）。隐藏：整块坞收进 0 宽，但 dockPanels/activeDockPanel 不清空——
// terminal 的会话、预览的页面导航状态都还在，随手点开任意一个面板按钮就原样回来。
const dockExpanded = ref(false)
const dockHidden = ref(false)
const showSnippet = ref(false)
const snippetInsertCmd = ref('')
function toggleDockExpanded() {
  dockExpanded.value = !dockExpanded.value
}
function toggleDockHidden() {
  const willHide = !dockHidden.value
  dockHidden.value = willHide
  if (willHide) dockExpanded.value = false
}
function onSnippetInsert(cmd) {
  // 先清空再设值，确保 watcher 即使值相同也会触发
  snippetInsertCmd.value = ''
  nextTick(() => { snippetInsertCmd.value = cmd })
}
function openDockPanel(key) {
  if (!dockPanels.value.includes(key)) dockPanels.value = [...dockPanels.value, key]
  activeDockPanel.value = key
  showDockAddMenu.value = false
  dockHidden.value = false // 打开面板这个动作本身就该让它可见，不然像是没反应
}
function toggleDockPanel(key) {
  if (activeDockPanel.value === key && dockPanels.value.includes(key)) {
    closeDockPanel(key)
    return
  }
  openDockPanel(key)
}
function closeDockPanel(key) {
  dockPanels.value = dockPanels.value.filter(k => k !== key)
  ensureActiveDockPanel()
  if (!dockPanels.value.length) {
    dockExpanded.value = false
    dockHidden.value = false
  }
}
// 菜单现在 Teleport 到 body 了，没法再靠 CSS position:absolute 相对按钮定位，
// 开菜单那一刻手动量一次按钮的屏幕坐标，换算成 fixed 定位（跟 CodeEditor.vue
// 右键菜单捕获 event.clientX/Y 是同一个思路，只是这里定位基准是按钮不是点击点）。
function toggleDockAddMenu() {
  showDockAddMenu.value = !showDockAddMenu.value
  if (showDockAddMenu.value && dockAddBtnRef.value) {
    const r = dockAddBtnRef.value.getBoundingClientRect()
    const menuW = 200
    let left = r.right - menuW
    if (left < 8) left = 8
    if (left + menuW > window.innerWidth - 8) left = window.innerWidth - menuW - 8
    dockAddMenuStyle.value = { top: `${r.bottom + 6}px`, left: `${left}px` }
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
const currentWorkDir = ref({ name: 'main-frontend', path: 'main-frontend' })
const workDirRecents = ref([{ name: 'main-frontend', path: 'main-frontend' }])
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

function openSystemWorkDirPicker() {
  showWorkDirMenu.value = false
  // 清空旧值后，同一个目录也能再次触发 change。
  if (workDirFolderInputRef.value) {
    workDirFolderInputRef.value.value = ''
    workDirFolderInputRef.value.click()
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
  // 普通浏览器出于安全原因只提供目录名，此时仍可选择仓库根目录下的子目录。
  let path = folderName
  const nativeFilePath = files[0].path
  if (nativeFilePath && relativeParts.length > 1) {
    const normalized = nativeFilePath.replace(/\\/g, '/')
    const relativeTail = relativeParts.slice(1).join('/')
    if (normalized.endsWith(`/${relativeTail}`)) {
      path = normalized.slice(0, -(relativeTail.length + 1))
    }
  }
  await selectWorkDir({ name: folderName, path })
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
    await refreshAgentFSTimeline()
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
const showGitDeliveryAgent = ref(false)
function openGitDeliveryAgent() {
  showPrMenu.value = false
  showGitDeliveryAgent.value = true
}
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
// 但最终答复必然是最后一条）。朗读/复制按钮拿它当内容。
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

// ==================== 模型选择 ====================
// 下拉展示后端内置目录与自定义提供方目录中「免 key 或已配 Key」的全部模型，按提供方分组；
// 不再有「选为可用」手动门控——填了 Key（或模型本身免 key）就自动出现。
// 图2「编辑模型」弹窗里的开关控制 hiddenModelIds（用户可隐藏不想见的模型），默认空=全显示。
// hiddenModelIds 由 composables/modelVisibility.js 统一管理（与图2 弹窗共享）。
const isModelVisible = (fm) => (fm.keyless || fm.api_key_set) && !hiddenModelIds.value.has(fm.id)

const selectedModel = ref(localStorage.getItem('selectedModel') || '')
// 列表为空时下拉无选项；选中项若不在真实可见列表里则定位到第一个。
watch([freeModelsFull, hiddenModelIds], () => {
  const ids = visibleModelIds.value
  if (ids.length === 0) return
  if (!ids.includes(selectedModel.value)) {
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
const modelSearch = ref('')
// 搜索过滤后的分组（图1 顶部搜索框）
const filteredGroupedOptions = computed(() => {
  const q = modelSearch.value.trim().toLowerCase()
  if (!q) return groupedModelOptions.value
  return groupedModelOptions.value
    .map(g => ({ vendor: g.vendor, items: g.items.filter(m => m.label.toLowerCase().includes(q)) }))
    .filter(g => g.items.length > 0)
})
function selectModel(value) { selectedModel.value = value; localStorage.setItem('selectedModel', value); showModelMenu.value = false }
// 图2「编辑模型」弹窗开关
const showModelManager = ref(false)

// ==================== 设置面板 ====================
const showSettings = ref(false)
const showScheduledTask = ref(false)
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
  }).catch(() => {
    console.log('定时任务数据:', data)
  })
}

// ==================== 底部工具条：Yolo 模式 + "+" 附加菜单 + Command 切换器 ====================
// 模式三态：Yolo（全自动批准）/ Ask（危险工具每步问）/ Plan（执行前必问）。
// 选了就写 localStorage('agentMode')，四态机发起工作流时透传给后端；
// 同时回显到 autoMode 变量驱动按钮文案与主题色动画。
const autoModeOptions = ['Yolo', 'Ask']
const autoMode = ref(localStorage.getItem('agentMode') === 'ask' ? 'Ask' : 'Yolo')
const showAutoMenu = ref(false)
const showAddMenu = ref(false)
const agentModeIsYolo = computed(() => autoMode.value === 'Yolo')
function selectAutoMode(opt) {
  autoMode.value = opt
  showAutoMenu.value = false
  // 只有两态：Yolo(全自动批准) / Ask(危险工具每步问)
  const mode = opt === 'Yolo' ? 'yolo' : 'ask'
  localStorage.setItem('agentMode', mode)
}

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
  isLoggedIn, debugTemp, debugTopP, debugReasoning, debugMaxTokens, balance,
  currentStatus, statusDotColor,
  messagesContainer, chatInputRef, userScrolledUp,
  forceScrollToBottom, adjustInputHeight, switchSession,
  backgroundTaskList, playVoice,
  flowState, startCodeWorkflow, stopCodeWorkflow, approvalState, respondApproval,
  resumeState, resumeCodeWorkflow, dismissResumable, todoState, sendSteerMessage,
  questionState, answerQuestion,
  toggleChat, updateParams,
  groupedMessages, formatChatTime
} = useChatWidget(props, { renderMarkdown })

// 任务清单完成数（输入框上方 todo-bar 用），仿 Hermes 勾选清单。
const todoDoneCount = computed(() =>
  (todoState.items || []).filter(it => it.status === 'done').length
)

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

// ==================== 思考强度（Effort）：Faster(low) ↔ Smarter(high) ====================
// 注意：debugReasoning 来自上面的 useChatWidget 解构，本段必须放在解构之后，
// 否则 setup 阶段会命中暂时性死区（TDZ）报 "Cannot access before initialization"。
const EFFORT_LEVELS = ['low', 'medium', 'high']
const EFFORT_UI_LABELS = { low: 'Faster', medium: 'Balanced', high: 'Smarter' }
const showEffortPanel = ref(false)
const showHeatmapPopup = ref(false)
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

function openSearchPanel() {
  sidebarOpen.value = true
  localStorage.setItem('sidebarOpen', '1')
  showSearchPanel.value = true
  nextTick(() => searchPanelInput.value?.focus())
}

function closeSearchPanel() {
  showSearchPanel.value = false
  searchQuery.value = ''
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
      active: activeDockPanel.value
    }
  }
  const saved = dockPanelsBySession[nid] || { panels: [], active: '' }
  dockPanels.value = [...saved.panels]
  activeDockPanel.value = saved.active
  ensureActiveDockPanel()
  showDockAddMenu.value = false
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
function handleSend() {
  if (hasPendingAttachments.value) return
  // 工作流跑着的时候，回车不再是"发一条新消息"（之前会在 startCodeWorkflow 里
  // 被 flowState.active 静默挡掉），而是把这句话当中途插话塞进正在跑的那个工作流。
  if (flowState.active) {
    const steerText = userInput.value.trim()
    if (!steerText) return
    userInput.value = ''
    nextTick(() => { if (chatInputRef.value) chatInputRef.value.style.height = 'auto' })
    sendSteerMessage(steerText)
    return
  }
  const combined = buildOutgoingMessage()
  if (!combined) return
  const displayText = userInput.value.trim()
  const displayAttachments = attachments.value.filter(a => a.status === 'ready').map(a => ({ ...a }))
  clearAttachments()
  userInput.value = ''
  // 发送后内容必空，直接把高度交回 CSS（min-height:40px 兜底成单行），
  // 不依赖 adjustInputHeight 的 scrollHeight 测量——它会在 v-model 未同步时量到旧高度而卡两行
  nextTick(() => { if (chatInputRef.value) chatInputRef.value.style.height = 'auto' })
  startCodeWorkflow(combined, { text: displayText, attachments: displayAttachments })
}
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
// 粘贴图片跟"+"菜单选图走同一条路：只当附件加进 attachments（复用 attachImageFile
// 的分析逻辑），发不发由用户自己按发送/回车决定。之前这里分析一完成就无条件
// handleSend()——分析本身是异步的（本地 llama.cpp 在小显存卡上可能要几十秒），
// 用户趁等待打字问问题时，分析一结束就把这句话连同图一起抢发出去，用户根本没
// 机会确认。跟"+"菜单选图（onAttachFilesSelected）保持一致："先附加，用户自己
// 决定何时发送"，不再有这个隐藏的自动发送时机。
function handlePaste(e) {
  const items = e.clipboardData?.items
  if (!items) return
  let imageFile = null
  for (const item of items) {
    if (item.type && item.type.startsWith('image/')) { imageFile = item.getAsFile(); break }
  }
  if (!imageFile) return
  e.preventDefault()
  if (flowState.active) { showVisionError('工作流运行中，请稍后再粘贴图片'); return }
  attachImageFile(imageFile)
}

// ==================== "+" 附加菜单：添加文件/照片、添加文件夹 ====================
// 跟粘贴图片（handlePaste）共用同一套 vision-preprocess 接口和状态提示，但这里是
// "先附加、用户自己决定何时发送"
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
    // 未配置时会默认使用首个可用的识图模型（本地 llama.cpp / GLM-5.2 / Kimi 等）。
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

// 发送那一刻才把附件序列化进正文：图片用 vision 分析结果、文件夹用清单、
// 文本文件只给文件名（不塞全文——agent 在后端工作目录里自己 read_file 读取，
// 把整份源码怼进消息既撑爆上下文又没必要）。顺序固定放在用户文字前面。
function buildOutgoingMessage() {
  const blocks = attachments.value
    .filter(a => a.status === 'ready')
    .map(a => {
      if (a.kind === 'image') return `[图片: ${a.name}]\n${a.analysisText || ''}`
      if (a.kind === 'folder') return `[文件夹: ${a.name}，共 ${a.fileCount} 个文件]\n${a.manifest}`
      // 文本/代码文件：只给文件名，让 agent 自行 read_file，不把内容塞进消息
      return `[文件: ${a.name}]`
    })
  const typed = userInput.value.trim()
  return [...blocks, typed].filter(Boolean).join('\n')
}

const showScrollButton = computed(() => { return isOpen.value && userScrolledUp.value })

watch(messages, () => { nextTick(() => { streamFadePass(); highlightAllCodeBlocks() }) }, { deep: true })
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
  refreshAgentFSTimeline()
  agentFSPollTimer = window.setInterval(() => {
    if (!sidebarOpen.value) refreshAgentFSTimeline()
  }, 4000)
  document.addEventListener('click', () => {
    showModelMenu.value = false; showTokenPanel.value = false
    showAutoMenu.value = false; showAddMenu.value = false; showPrMenu.value = false; showWorkDirMenu.value = false; showBranchMenu.value = false
    showDockAddMenu.value = false
    closeAgentFSDiff()
  })
  window.addEventListener('keydown', onGlobalDockShortcut)
  window.addEventListener('resize', syncAgentFSTreeViewport)
  nextTick(syncAgentFSTreeViewport)
})
onUnmounted(() => {
  window.removeEventListener('keydown', onGlobalDockShortcut)
  window.removeEventListener('resize', syncAgentFSTreeViewport)
  window.clearInterval(agentFSPollTimer)
})
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

/* 自适应占位符的绝对定位 */
.input-placeholder-text {
  position: absolute;
  left: 16px;             /* 和 input-wrapper 的 padding-left 保持一致 */
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
</style>
