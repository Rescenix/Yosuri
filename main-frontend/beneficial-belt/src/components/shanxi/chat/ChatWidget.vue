<template>
  <div class="chat-widget-root">
    <div class="chat-toggle-button" v-if="!isOpen" @click="toggleChat">
      <Icon icon="mdi:chat" width="28" color="white" />
    </div>

    <div class="chat-window" :class="{ expanded: isExpanded }" :style="{ display: isOpen ? 'flex' : 'none' }">

      <!-- ★ 主内容区 -->
      <div class="chat-main">

        <!-- 顶部横条已删除：会话切换在左侧 Gemini 风侧栏，工具组浮在聊天区右上角 -->
        <div class="chat-body-row">
        <!-- ★ Gemini 风侧栏：展开=平铺会话面板，折叠=竖向图标条（带会话横条） -->
        <aside v-if="isExpanded" class="gem-sidebar" :class="{ collapsed: !sidebarOpen }">
          <!-- 顶部：展开态 汉堡+折叠toggle；折叠态只有 toggle -->
          <div class="gem-top">
            <a href="/" class="gem-icon-btn gem-home" title="首页">
              <Icon icon="majesticons:shooting-star-line" width="18" />
            </a>
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
            @select-session="selectSession"
            @new-session="newSession"
            @rename-session="renameSession"
            @delete-session="deleteSession"
            @open-settings="showSettings = true"
          />

          <!-- 折叠态：竖向图标条 -->
          <template v-else>
            <button class="gem-icon-btn" @click="newSession" title="发起新对话">
              <Icon icon="mdi:pencil-plus-outline" width="18" />
            </button>
            <button class="gem-icon-btn" @click="toggleSidebar" title="搜索对话内容">
              <Icon icon="mdi:magnify" width="18" />
            </button>
            <button class="gem-icon-btn" @click="toggleSidebar" title="项目">
              <Icon icon="mdi:folder-outline" width="18" />
            </button>
            <button class="gem-icon-btn" @click="toggleSidebar" title="附件">
              <Icon icon="hugeicons:file-attachment" width="18" />
            </button>
            <!-- 会话横条：鼠标悬停立刻弹出可点击的会话卡片（不再用原生 title 提示）。
                 笔记本(置顶)与最近分区。 -->
            <div
              class="gem-rail-sessions"
              @mouseenter="openRailCard"
              @mouseleave="closeRailCardDelayed"
            >
              <template v-if="railPinned.length">
                <button
                  v-for="s in railPinned"
                  :key="s.id"
                  class="gem-rail-bar pinned"
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
              <div class="gem-rail-avatar" title="Prometheus · Pro">P</div>
            </div>

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
                <template v-if="railPinned.length">
                  <div class="rail-card-label">笔记本</div>
                  <button
                    v-for="s in railPinned"
                    :key="s.id"
                    class="rail-card-row"
                    :class="{ active: s.id === activeSession }"
                    @click="onRailCardSelect(s.id)"
                  >
                    <span class="rail-card-mark pinned" :class="{ running: s.id === runningSession }"></span>
                    <span class="rail-card-name">{{ s.name }}</span>
                  </button>
                </template>
                <div class="rail-card-label">最近</div>
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

        <!-- 侧栏折叠时:便签 + 看板娘独占一栏（三栏工作区的左栏）。
             以前它们是 position:absolute 的悬浮层，会直接压在聊天内容上；
             现在这一栏真实占位，中间聊天列靠 flex:1 让位，互不遮挡。
             栏内两个元素仍可各自拖动(位置记进 localStorage)，只是不再靠拖动来躲开聊天区。 -->
        <div v-if="isExpanded && !sidebarOpen" class="studio-side-col">
          <div
            class="side-drag"
            :class="{ dragging: stickyDrag.dragging.value, nudged: stickyDrag.offset.value.x || stickyDrag.offset.value.y }"
            :style="{ transform: `translate(${stickyDrag.offset.value.x}px, ${stickyDrag.offset.value.y}px)` }"
            title="拖动可移动，双击复位"
            @mousedown="stickyDrag.onDown"
            @dblclick="stickyDrag.reset"
          >
            <TaskTodoSticky :items="todoState.items" />
          </div>
          <div
            class="side-drag studio-side-live2d"
            :class="{ dragging: live2dDrag.dragging.value, nudged: live2dDrag.offset.value.x || live2dDrag.offset.value.y }"
            :style="{ transform: `translate(${live2dDrag.offset.value.x}px, ${live2dDrag.offset.value.y}px)` }"
            title="拖动可移动，双击复位"
            @mousedown="live2dDrag.onDown"
            @dblclick="live2dDrag.reset"
          >
            <Live2DWidget />
          </div>
        </div>

               <div class="chat-body studio">
          <!-- 共享聊天列 -->
          <div class="chat-content studio">

            <!-- 右上角工具组：顶部横条删除后浮在聊天区右上（Code 模式才有意义）。
                 工具窗口（终端/Diff/预览）打开后丝滑变成竖条贴靠面板边缘，DOM 顺序不变，
                 更多(三点)本来就排最后，变竖条后自然落在底部。 -->
            <div class="floating-tools" :class="{ vertical: dockPanels.length > 0 }" v-if="inputTopBarMode === 'git'">
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
                    <div v-if="item.type === 'image'" class="image-card">
                      <img :src="item.image" style="max-width: 240px; border-radius: 12px;" />
                    </div>
                    <div v-else-if="item.sender === 'user'" class="message-bubble user" :class="{ editing: editingMsgId === item.id }">
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
                      <div v-else-if="item.content">{{ item.content }}</div>
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
                      <button
                        v-else-if="item.content"
                        class="msg-edit-btn"
                        title="编辑并重新发送"
                        @click="editUserMessage(item)"
                      >
                        <Icon icon="mdi:pencil-outline" width="15" />
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
              <button v-show="showScrollButton" class="scroll-to-bottom-btn" @click="forceScrollToBottom" title="回到底部">
                <Icon icon="mdi:chevron-down" width="20" color="#555" />
              </button>

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
                <UserMessageRail :messages="messages" @jump="jumpToMessage" />

                <div class="input-toolbar-right">
                  <!-- Context window 用量：圆环 + 模型 pill + 模式 pill（紧凑版） -->
                  <div class="context-bar-widget" @click.stop="toggleTokenPanel" title="Context window 用量">
                    <span class="ctx-bar-text" style="display:none">{{ formatTok(ctxTotalUsed) }}/{{ formatTok(ctxWindow) }}</span>
                    <svg class="ctx-ring" viewBox="0 0 18 18" width="14" height="14" aria-hidden="true">
                      <circle class="ctx-ring-track" cx="9" cy="9" r="7" fill="none" />
                      <circle
                        class="ctx-ring-fill"
                        cx="9" cy="9" r="7" fill="none"
                        :stroke-dasharray="ctxRingC"
                        :stroke-dashoffset="ctxRingC * (1 - ctxPct / 100)"
                      />
                    </svg>
                    <span class="ctx-bar-pct" style="display:none">{{ ctxPct.toFixed(0) }}%</span>
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
                    <div v-if="showModelMenu" class="model-menu-dropdown" @click.stop>
                      <div v-if="!hasModels" class="model-menu-empty"></div>
                      <div
                        v-for="m in modelOptions"
                        :key="m.value"
                        class="model-menu-item"
                        :class="{ active: selectedModel === m.value }"
                        @click="selectModel(m.value)"
                      >{{ m.label }}</div>
                    </div>
                  </div>

                  <!-- 模式 pill（effort） -->
                  <div v-if="currentCapability.reasoning" ref="effortWidgetRef" class="effort-widget" @click.stop="showEffortPanel = !showEffortPanel">
                    <span class="effort-value">{{ effortLabel }}</span>
                  </div>
                  <Teleport to="body">
                    <div v-if="showEffortPanel" class="effort-panel" :style="effortPanelStyle" @click.stop>
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

      <!-- 工具审批已改成输入框上方的轻量条（见 .approval-bar），不再用打断式弹窗 -->
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
import { previewRequest } from '../composables/previewBus.js'
import UserMessageRail from './UserMessageRail.vue'
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
import TaskTodoSticky from './TaskTodoSticky.vue'
import Live2DWidget from './Live2DWidget.vue'
import PreviewBrowser from './PreviewBrowser.vue'
import NewSessionHome from './NewSessionHome.vue'
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
    // parentId/forkIndex 是侧栏拼分支树用的血缘（后端 SessionInfo 带下来，根会话为空）
    const real = (data || []).map(s => ({
      id: s.id, name: shortTitle(s.title),
      parentId: s.parent_id || '', forkIndex: s.fork_index || 0, updatedAt: s.updated_at
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
function newSession() {
  const id = 'sess_' + Date.now().toString(36)
  sessionList.value = [{ id, name: '新对话', parentId: '', forkIndex: 0 }, ...sessionList.value]
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
// 用量圆环的周长：半径 r=7（与模板里的 <circle r="7"> 对应），dasharray/offset 都按它算
const ctxRingC = 2 * Math.PI * 7

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

// 注：原来这里还有个 liveContextStats（input+output 口径）专供底部横条，
// 跟面板的分类之和是两套对不上的口径。现已统一到 ctxTotalUsed / ctxWindow / ctxPct，
// 该 computed 随之删除。

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

// agent 改了前端文件 → 后端推 preview_open → 这里把预览面板挂进 dock。
// 刻意不用 toggleDockPanel：那是"开关"语义，面板已经开着的时候会被它关掉，
// 正好跟"自动打开"相反。导航到具体地址由 PreviewBrowser 自己 watch 同一个源。
watch(() => previewRequest.seq, () => {
  if (!dockPanels.value.includes('preview')) {
    dockPanels.value = [...dockPanels.value, 'preview']
  }
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

// ==================== 便签/看板娘 独立拖动 ====================
// 每个元素一套独立的 translate 偏移,互不影响,分别记进 localStorage(刷新不丢)。
// 注意:现在它们待在 .studio-side-col 这一栏里,拖动只是微调位置,
// 不再承担"躲开聊天内容"的职责——那件事已经由栏本身占位解决了。
// 拖动范围限制在工作区（.chat-body-row）内：以前没有边界，一不小心拖到聊天区
// 后面或者屏幕外，元素就"消失"了，而且因为抓不到它，再也拖不回来。
const DRAG_BOUNDS_SELECTOR = '.chat-body-row'

const fitRange = (v, lo, hi) => (hi < lo ? lo : Math.min(Math.max(v, lo), hi))

// 算出 offset 的合法范围。appliedOff 必须是「此刻页面上真正生效的偏移」——
// getBoundingClientRect() 拿到的 rect 已经含了这个 transform，用候选偏移去反推
// 原始位置会算出完全错误的边界（这正是第一版没夹住的原因）。
function dragBoundsOf(el, appliedOff) {
  const box = el?.closest?.(DRAG_BOUNDS_SELECTOR)
  if (!el || !box) return null
  const r = el.getBoundingClientRect()
  const c = box.getBoundingClientRect()
  if (!r.width || !c.width) return null // 还没布局完
  const natLeft = r.left - appliedOff.x // 元素在 offset=0 时的位置
  const natTop = r.top - appliedOff.y
  return {
    minX: c.left - natLeft,
    maxX: c.right - r.width - natLeft,
    minY: c.top - natTop,
    maxY: c.bottom - r.height - natTop,
  }
}

function clampToBounds(off, b) {
  if (!b) return off
  return { x: fitRange(off.x, b.minX, b.maxX), y: fitRange(off.y, b.minY, b.maxY) }
}

function makeDrag(storageKey) {
  const offset = ref({ x: 0, y: 0 })
  const dragging = ref(false)
  let start = null
  let el = null
  let bounds = null
  try {
    const s = JSON.parse(localStorage.getItem(storageKey) || 'null')
    if (s && typeof s.x === 'number' && typeof s.y === 'number') offset.value = s
  } catch { /* 无历史位置 */ }
  function onMove(e) {
    if (!dragging.value || !start) return
    const raw = { x: start.ox + (e.clientX - start.mx), y: start.oy + (e.clientY - start.my) }
    // 边界在按下时算好，拖动过程中只做纯算术夹取：既不用每帧读 rect（Vue 异步
    // 渲染下 rect 会滞后于 offset，读出来是上一帧的），也省掉反复触发布局
    offset.value = clampToBounds(raw, bounds)
  }
  function onUp() {
    dragging.value = false
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
    try { localStorage.setItem(storageKey, JSON.stringify(offset.value)) } catch { /* 忽略 */ }
  }
  function onDown(e) {
    dragging.value = true
    el = e.currentTarget
    // 此刻 rect 与 offset.value 是一致的（都是当前已渲染状态），是唯一能正确
    // 反推出元素原始位置的时机
    bounds = dragBoundsOf(el, offset.value)
    start = { mx: e.clientX, my: e.clientY, ox: offset.value.x, oy: offset.value.y }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    e.preventDefault()
  }
  // 把已经存在 localStorage 里的越界位置拉回来。没有这一步的话，
  // 上一个版本拖丢的元素这次仍然在界外，用户永远够不着它。
  // 复位：清掉偏移回到栏内默认位置。夹回边界只能保证"看得见"，回不到原位——
  // 拖飞之后被按在工作区边缘的那个位置，既不是用户想要的也不是默认的。
  function reset() {
    offset.value = { x: 0, y: 0 }
    try { localStorage.removeItem(storageKey) } catch { /* 忽略 */ }
  }
  function rescue(selector) {
    const node = document.querySelector(selector)
    if (!node) return
    // 这里 offset.value 就是页面上生效的偏移，跟 rect 一致，可以直接算边界
    const fixed = clampToBounds(offset.value, dragBoundsOf(node, offset.value))
    if (fixed.x !== offset.value.x || fixed.y !== offset.value.y) {
      offset.value = fixed
      try { localStorage.setItem(storageKey, JSON.stringify(fixed)) } catch { /* 忽略 */ }
    }
  }
  return { offset, dragging, onDown, rescue, reset }
}
const stickyDrag = makeDrag('corner_sticky_offset')
const live2dDrag = makeDrag('corner_live2d_offset')

// 越界救回的 watch 放在 sidebarOpen 声明之后（见下方 rescueDragged），
// 不能放这里：watch 会立即求值 getter 建依赖，会撞上 sidebarOpen 的 TDZ。
function rescueDragged() {
  nextTick(() => {
    stickyDrag.rescue('.studio-side-col .side-drag:not(.studio-side-live2d)')
    live2dDrag.rescue('.studio-side-col .studio-side-live2d')
  })
}

// 光在挂载时夹一次不够：看板娘的 canvas 是异步加载的，它撑大之后这一列
// （justify-content:flex-end）会把便签整体往上顶，刚算好的边界当场失效。
// 用 ResizeObserver 盯着尺寸变化重新夹，顺带覆盖窗口缩放。
let sideColRO = null
function watchSideColResize() {
  sideColRO?.disconnect()
  const col = document.querySelector('.studio-side-col')
  if (!col || typeof ResizeObserver === 'undefined') return
  let pending = null
  sideColRO = new ResizeObserver(() => {
    clearTimeout(pending)              // 合并连续的尺寸抖动，别每帧都夹一次
    pending = setTimeout(rescueDragged, 120)
  })
  sideColRO.observe(col)
  for (const child of col.children) sideColRO.observe(child)
}
onUnmounted(() => sideColRO?.disconnect())

// 点导航轴上的圆点：滚到那条用户消息并高亮一下，否则跳过去了也不知道落在哪。
// 用 behavior:'auto' 而不是 'smooth'：平滑滚动是可中断的动画，会被聊天区
// 自动跟底的逻辑在半路打回原位（实测 smooth 跳完 scrollTop 原封不动，
// 瞬时跳则稳定生效）。跨两千像素找旧消息本来也不需要看动画。
function jumpToMessage(id) {
  const el = document.querySelector(`[data-msg-id="${id}"]`)
  if (!el) return
  el.scrollIntoView({ behavior: 'auto', block: 'center' })
  el.classList.add('msg-jump-flash')
  setTimeout(() => el.classList.remove('msg-jump-flash'), 1200)
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
  toggleChat, updateParams,
  groupedMessages, formatChatTime
} = useChatWidget(props, { renderMarkdown })

// 工作流跑完后重新拉一次会话列表：把分叉时乐观插入的分支名跟后端算出的标题对齐，
// 也顺带修掉"新会话标题要切走再切回才出现"的老毛病（以前只在挂载/切会话时拉）。
// 必须放在上面的解构之后：watch 的 getter 是立即求值的，写在解构之前会命中 TDZ
// （同一文件里 runningSession 那个 computed 能放在前面，只是因为 computed 是惰性的）。
watch(() => flowState.active, (now, was) => {
  if (was && !now) loadSessionList()
})

// ==================== 思考强度（Effort）：Faster(low) ↔ Smarter(high) ====================
// 注意：debugReasoning 来自上面的 useChatWidget 解构，本段必须放在解构之后，
// 否则 setup 阶段会命中暂时性死区（TDZ）报 "Cannot access before initialization"。
const EFFORT_LEVELS = ['low', 'medium', 'high']
const EFFORT_UI_LABELS = { low: 'Faster', medium: 'Balanced', high: 'Smarter' }
const showEffortPanel = ref(false)
const effortWidgetRef = ref(null)
const initialEffortIdx = EFFORT_LEVELS.indexOf(debugReasoning.value)
const effortLevel = ref(initialEffortIdx >= 0 ? initialEffortIdx : 1)
const effortLabel = computed(() => EFFORT_UI_LABELS[EFFORT_LEVELS[effortLevel.value]])
const effortPanelStyle = computed(() => {
  if (!effortWidgetRef.value) return {}
  const rect = effortWidgetRef.value.getBoundingClientRect()
  return {
    position: 'fixed',
    top: (rect.top - 8) + 'px',
    left: (rect.left + rect.width / 2) + 'px',
    transform: 'translate(-50%, -100%)',
    zIndex: 9999
  }
})
function onEffortChange() {
  debugReasoning.value = EFFORT_LEVELS[effortLevel.value]
  localStorage.setItem('debugReasoning', debugReasoning.value)
}
if (!debugReasoning.value) onEffortChange() // 首次没设置过时落一个默认值，跟滑块初始位置对齐

// ==================== UI 状态 ====================
const showParams = ref(false)
const showMoreMenu = ref(false)
const showBackgroundTasks = ref(false)

// ==================== 左侧 Gemini 风侧栏：展开 vs 折叠竖条 ====================
const sidebarOpen = ref(localStorage.getItem('sidebarOpen') !== '0')
function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value
  localStorage.setItem('sidebarOpen', sidebarOpen.value ? '1' : '0')
  if (!sidebarOpen.value) refreshPinnedRail() // 进折叠态时同步最新置顶分区
}

// 便签/看板娘这一列是 v-if 挂上去的，出现之后才量得到尺寸；此时把上个版本
// 拖到界外、已经够不着的位置夹回可视范围（见 clampOffset）。
watch(() => isExpanded.value && !sidebarOpen.value, (shown) => {
  if (!shown) { sideColRO?.disconnect(); return }
  rescueDragged()
  nextTick(watchSideColResize)
})
// 首次进入必须走 onMounted：上面这个 watch 如果带 immediate，会在 setup 阶段
// （DOM 还没挂载）就执行，querySelector 拿不到那一列，观察器根本建不起来。
onMounted(() => {
  if (isExpanded.value && !sidebarOpen.value) {
    rescueDragged()
    nextTick(watchSideColResize)
  }
})
// 折叠态会话横条：笔记本(置顶) 与 最近 分两区，悬浮 title 显示会话名。
// 置顶 id 由 SessionMenuContent 写在 localStorage('pinnedSessions')，这里读来分区，
// 折叠切换时刷新一次（置顶操作只发生在展开态，折叠时读到的就是最新的）。
const pinnedIdsRail = ref([])
function refreshPinnedRail() {
  try { pinnedIdsRail.value = JSON.parse(localStorage.getItem('pinnedSessions') || '[]') } catch { pinnedIdsRail.value = [] }
}
onMounted(refreshPinnedRail)
const railPinned = computed(() => sessionList.value.filter(s => pinnedIdsRail.value.includes(s.id)))
const railRecent = computed(() => sessionList.value.filter(s => !pinnedIdsRail.value.includes(s.id)).slice(0, 10))

// 悬停横条弹出的会话卡片：立刻打开（无延迟），移开留 160ms 缓冲，
// 让鼠标能从横条平移到卡片上而不闪断。卡片自身也挂同一对进入/离开处理。
const railCardOpen = ref(false)
const railCardStyle = ref({})
let railCardCloseTimer = null
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
  if (oid) dockPanelsBySession[oid] = [...dockPanels.value]
  dockPanels.value = [...(dockPanelsBySession[nid] || [])]
  showBackgroundTasks.value = false
  showMoreMenu.value = false
})

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
    showModelMenu.value = false; showTokenPanel.value = false; showMoreMenu.value = false
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
</style>