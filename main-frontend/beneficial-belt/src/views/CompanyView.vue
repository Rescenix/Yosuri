<template>
  <main class="company-view">
    <header class="command-hero">
      <div class="command-topline">
        <span class="runtime-label"><i></i> LOCAL MULTI-AGENT RUNTIME</span>
        <span class="runtime-version">{{ osStats.version || 'RESCENE OS' }}</span>
      </div>
      <div class="command-copy">
        <div>
          <p class="command-kicker">LESS CHAT. MORE AUTOMATIC.</p>
          <h1><span>{{ osStats.totalAgents || 0 }}</span> 个 Agent，<br>一家公司正在自己运转。</h1>
          <p class="command-subtitle">不是群聊，不是角色扮演。每一次分工、接力、工具调用和交付都留下真实证据。</p>
        </div>
        <div class="zero-cost-seal">
          <span>本次模型账单</span>
          <strong>￥0<span>.00</span></strong>
          <small>FREE MODEL ROUTER</small>
        </div>
      </div>
      <div class="command-metrics">
        <div class="command-metric hot"><span>正在执行</span><strong>{{ osStats.workingCount || 0 }}</strong><small>AGENTS RUNNING</small></div>
        <div class="command-metric"><span>真实交付</span><strong>{{ totalOutputs }}</strong><small>FILES ON DISK</small></div>
        <div class="command-metric"><span>协作接力</span><strong>{{ totalHandoffs }}</strong><small>AGENT HANDOFFS</small></div>
        <div class="command-metric"><span>等待人类</span><strong>{{ approvalProjects.length }}</strong><small>PROJECT GATES</small></div>
      </div>
      <div class="integration-strip">
        <span class="integration-title">CONNECTED SYSTEMS</span>
        <span :class="{ ready: integrations.microsoft?.configured }"><i></i> Microsoft 365 · {{ integrations.microsoft?.configured ? 'Graph 已连接' : '待配置 Entra' }}</span>
        <span :class="{ ready: integrations.database?.configured }"><i></i> Database · {{ integrations.database?.configured ? '已连接' : '待配置连接串' }}</span>
        <span :class="{ ready: integrations.navicat?.installed }"><i></i> Navicat · {{ integrations.navicat?.installed ? '客户端就绪' : '未检测到' }}</span>
      </div>
    </header>

    <!-- 竖标签：审批台 / 实时运行 / 会议室 -->
    <div class="company-layout">
      <nav class="section-tabs">
        <button class="section-tab" :class="{ active: activeSection === 'tags' }" @click="activeSection = 'tags'; loadTags(); loadIterate(); loadDirective()">
          <Icon icon="mdi:auto-fix" width="18" />
          <span>预设</span>
          <span class="st-badge">{{ tagsList.length + iteratePlans.length }}</span>
        </button>
        <button class="section-tab" :class="{ active: activeSection === 'meeting' }" @click="activeSection = 'meeting'">
          <Icon icon="mdi:account-group-outline" width="18" />
          <span>会议室</span>
          <span class="st-badge">{{ meetings.length }}</span>
        </button>
        <button class="section-tab" :class="{ active: activeSection === 'live' }" @click="activeSection = 'live'">
          <Icon icon="mdi:chart-timeline-variant" width="18" />
          <span>实时运行</span>
          <span class="st-badge">{{ osStats.totalAgents || 0 }}</span>
        </button>
        <button class="section-tab" :class="{ active: activeSection === 'office' }" @click="activeSection = 'office'">
          <Icon icon="mdi:office-building" width="18" />
          <span>办公室</span>
          <span class="st-badge">{{ officePeople.length || agents.length }}</span>
        </button>
        <button class="section-tab" :class="{ active: activeSection === 'market' }" @click="activeSection = 'market'; loadMarket()">
          <Icon icon="mdi:storefront-outline" width="18" />
          <span>应用大厅</span>
          <span class="st-badge">{{ marketGames.length }}</span>
        </button>
        <button class="section-tab" :class="{ active: activeSection === 'approval' }" @click="activeSection = 'approval'">
          <Icon icon="mdi:clipboard-check-outline" width="18" />
          <span>审批台</span>
          <span class="st-badge">{{ approvalProjects.length }}</span>
        </button>
      </nav>
      <div class="section-content">

    <!-- 会议场景：展示已经生成的真实会议包，不用动画冒充实时会议。 -->
    <section v-show="activeSection === 'meeting'" class="meeting-room">
      <div class="section-heading">
        <div>
          <p class="section-kicker">0 · 会议室</p>
          <h2>{{ latestMeeting?.topic || '还没有可验证的会议' }}</h2>
        </div>
        <span v-if="latestMeeting?.kind === 'ai_reconstruction'" class="meeting-live reconstructed"><span class="live-pulse"></span> AI 重建回放</span>
        <span v-else-if="latestMeeting?.kind === 'project_review'" class="meeting-live reconstructed"><span class="live-pulse"></span> 项目交付评审</span>
        <span v-else class="meeting-live muted">仅会议纪要</span>
      </div>

      <template v-if="latestMeeting">
        <div v-if="latestMeeting.kind !== 'minutes_only'" class="meeting-proof-banner">
          <Icon icon="mdi:shield-check-outline" width="18" />
          <span>{{ latestMeeting.disclaimer }}</span>
          <strong>{{ latestMeeting.speeches?.length || 0 }} 位发言 · {{ latestMeeting.time }}</strong>
        </div>

        <div class="meeting-media-grid">
          <div class="meeting-replay">
            <video v-if="latestMeeting.replayFile" controls preload="metadata" :src="meetingFileUrl(latestMeeting, latestMeeting.replayFile)"></video>
            <div v-else class="meeting-media-missing">
              <Icon icon="mdi:video-off-outline" width="30" />
              <strong>回放尚未渲染</strong>
              <span>{{ latestMeeting.replayError || '这份历史会议只有文字纪要。新会议才会生成 AI 重建 MP4。' }}</span>
            </div>
          </div>
          <div class="meeting-assets">
            <p>{{ latestMeeting.kind === 'project_review' ? 'DELIVERY REVIEW PACKAGE' : 'MEETING PACKAGE' }}</p>
            <button v-if="latestMeeting.pptFile" type="button" @click="previewFile(latestMeeting.agent, latestMeeting.pptFile)"><Icon icon="mdi:microsoft-powerpoint" width="19" /><span><strong>{{ latestMeeting.kind === 'project_review' ? '项目路演 PPT' : '会议 PPT' }}</strong><small>站内逐页预览</small></span></button>
            <div v-else class="meeting-asset-disabled"><Icon icon="mdi:microsoft-powerpoint" width="19" /><span><strong>PPT 未生成</strong><small>{{ latestMeeting.pptError || '历史会议无演示稿' }}</small></span></div>
            <button type="button" @click="previewFile(latestMeeting.agent, latestMeeting.file)"><Icon icon="mdi:file-document-check-outline" width="19" /><span><strong>{{ latestMeeting.kind === 'project_review' ? '评审决议' : '会议纪要' }}</strong><small>查看完整决策记录</small></span></button>
            <a v-if="latestMeeting.transcriptFile" :href="meetingFileUrl(latestMeeting, latestMeeting.transcriptFile)" target="_blank" rel="noopener"><Icon icon="mdi:subtitles-outline" width="19" /><span><strong>VTT 时间轴</strong><small>逐席发言与证据</small></span></a>
          </div>
        </div>

        <div v-if="latestMeeting.speeches?.length" class="meeting-speeches">
          <div class="meeting-speeches-head"><span>{{ latestMeeting.kind === 'project_review' ? '部门证据汇报' : '部门逐席发言' }}</span><small>每句话都绑定磁盘证据</small></div>
          <article v-for="speech in latestMeeting.speeches" :key="speech.agent + speech.order" class="meeting-speech">
            <div class="speech-order">{{ String(speech.order).padStart(2, '0') }}</div>
            <div class="speech-copy">
              <header><strong>{{ speech.department }}</strong><span>{{ speech.agent }}</span><time>{{ speech.start }} → {{ speech.end }}</time></header>
              <p>{{ speech.text }}</p>
              <button type="button" @click="previewFile(speech.agent, speech.source)"><Icon icon="mdi:paperclip" width="14" /> 证据 · {{ speech.source }}</button>
            </div>
          </article>
        </div>

        <details class="meeting-minutes" :open="latestMeeting.kind === 'minutes_only'">
          <summary>{{ latestMeeting.kind === 'project_review' ? '完整评审纪要' : '完整会议纪要' }}</summary>
          <div class="meeting-note-body" v-html="renderMarkdown(latestMeeting.content)"></div>
        </details>

        <div v-if="meetings.length > 1" class="meeting-history">
          <span>历史会议</span>
          <div v-for="m in meetings.slice(1, 4)" :key="m.agent + m.file" class="meeting-history-row">
            <div><strong>{{ m.topic || m.file }}</strong><small>{{ m.time }} · {{ m.kind === 'project_review' ? `${m.speeches?.length || 0} 个部门评审` : (m.kind === 'ai_reconstruction' ? `${m.speeches?.length || 0} 位发言` : '仅纪要') }}</small></div>
            <button type="button" @click="previewFile(m.agent, m.file)">查看纪要</button>
          </div>
        </div>
      </template>
      <div v-else class="meeting-empty">没有真实部门产物，因此系统没有生成空壳会议。先完成调研、设计、编码等交付，CEO 才能召集可验证会议。</div>
    </section>

    <!-- 项目级审批工作台 -->
    <section v-show="activeSection === 'approval'" class="approval-desk">
      <div class="approval-hero">
        <div>
          <p class="section-kicker">HUMAN APPROVAL GATE</p>
          <h2>你审批项目，不审批文件。</h2>
          <p>Agent 的过程文件自动归档；只有完整项目抵达这里，才需要人类做价值判断。</p>
        </div>
        <div class="approval-total"><strong>{{ approvalProjects.length }}</strong><span>个团队项目验收中</span></div>
      </div>
      <div class="approval-policy">
        <span><Icon icon="mdi:account-multiple-check-outline" width="17" /> {{ approvalProjects.length }} 个团队项目进入验收</span>
        <span><Icon icon="mdi:account-arrow-right-outline" width="17" /> {{ soloProductionCount }} 个单 Agent 交付一并进入验收</span>
        <span><Icon icon="mdi:archive-outline" width="17" /> {{ standalonePendingCount }} 份零散产出自动归档</span>
        <span><Icon icon="mdi:shield-check-outline" width="17" /> 批准或退回将作用于整个项目</span>
      </div>
      <div v-if="approvalProjects.length" class="project-approval-grid">
        <article v-for="(project, index) in approvalProjects" :key="project.key" class="project-approval-card">
          <header>
            <span class="project-number">PROJECT {{ String(index + 1).padStart(2, '0') }}</span>
            <span class="project-score" :class="{ blocked: !project.ready }"><i></i> {{ project.ready ? 'READY' : 'IN PRODUCTION' }} · {{ project.completedStageCount }}/{{ projectStages.length }}</span>
          </header>
          <h3>{{ project.title }}</h3>
          <div class="project-meta">
            <span><Icon icon="mdi:robot-outline" width="15" /> {{ project.agents.length }} 个 Agent</span>
            <span><Icon icon="mdi:package-variant-closed" width="15" /> {{ project.artifacts.length }} 份产物</span>
            <span><Icon icon="mdi:source-branch" width="15" /> {{ project.roles.length }} 个部门接力</span>
          </div>

          <div class="project-preview delivery-theater" :class="`preview-${selectedShowcaseArtifact(project)?.kind || 'empty'}`">
            <div class="preview-head">
              <span><Icon icon="mdi:monitor-eye" width="15" /> VERIFIED DELIVERY THEATER</span>
              <button v-if="selectedShowcaseArtifact(project)" type="button" @click="previewProjectArtifact(project, selectedShowcaseArtifact(project))">沉浸预览 ↗</button>
            </div>
            <iframe v-if="selectedShowcaseArtifact(project)?.kind === 'html'" sandbox="allow-forms allow-modals allow-scripts" :src="artifactRawUrl(selectedShowcaseArtifact(project))" title="可运行交付物预览"></iframe>
            <video v-else-if="selectedShowcaseArtifact(project)?.kind === 'video'" :src="artifactRawUrl(selectedShowcaseArtifact(project))" controls playsinline preload="metadata"></video>
            <img v-else-if="selectedShowcaseArtifact(project)?.kind === 'image'" :src="artifactRawUrl(selectedShowcaseArtifact(project))" alt="项目交付物预览" />
            <button v-else-if="selectedShowcaseArtifact(project)?.kind === 'text'" class="artifact-hero report-hero" type="button" @click="previewProjectArtifact(project, selectedShowcaseArtifact(project))"><Icon icon="mdi:file-document-multiple" width="54" /><span><b>{{ textHeroLabel(selectedShowcaseArtifact(project).stage).en }}</b><strong>{{ textHeroLabel(selectedShowcaseArtifact(project).stage).cn }}</strong><small>{{ selectedShowcaseArtifact(project).name }}</small></span></button>
            <button v-else-if="selectedShowcaseArtifact(project)?.kind === 'spreadsheet'" class="artifact-hero excel-hero" type="button" @click="previewProjectArtifact(project, selectedShowcaseArtifact(project))"><Icon icon="mdi:microsoft-excel" width="54" /><span><b>RESEARCH DATA</b><strong>打开可复算 Excel</strong><small>真实磁盘数据 · 筛选 · 数值类型 · 冻结表头</small></span></button>
            <button v-else-if="selectedShowcaseArtifact(project)?.kind === 'pptx'" class="artifact-hero ppt-hero" type="button" @click="previewProjectArtifact(project, selectedShowcaseArtifact(project))"><Icon icon="mdi:microsoft-powerpoint" width="54" /><span><b>INVESTOR DECK</b><strong>逐页播放项目路演</strong><small>真实 PPTX · 16:9 · 站内解析</small></span></button>
            <button v-else-if="selectedShowcaseArtifact(project)" class="artifact-hero receipt-hero" type="button" @click="previewProjectArtifact(project, selectedShowcaseArtifact(project))"><Icon icon="mdi:certificate-outline" width="54" /><span><b>MACHINE RECEIPT</b><strong>查看机器发布回执</strong><small>渠道 · 时间 · 入口 · SHA-256</small></span></button>
            <div v-else class="preview-missing"><Icon icon="mdi:package-variant-remove" width="24" /><span>尚未生成可预览的最终产物</span></div>
            <div v-if="selectedShowcaseArtifact(project)" class="artifact-proof-strip">
              <span><i></i> {{ roleLabel(selectedShowcaseArtifact(project).producerRole) }}</span>
              <strong>{{ selectedShowcaseArtifact(project).verification || '真实文件已落盘并通过哈希校验' }}</strong>
              <code>SHA {{ (selectedShowcaseArtifact(project).sha256 || '').slice(0, 12) }}</code>
            </div>
          </div>

          <div class="delivery-switcher">
            <button v-for="artifact in projectShowcase(project)" :key="artifact.agent + artifact.path" type="button"
              :class="{ active: selectedShowcaseArtifact(project)?.path === artifact.path }" @click="selectShowcaseArtifact(project, artifact)">
              <span class="delivery-kind"><Icon :icon="artifactShowcaseIcon(artifact)" width="21" /></span>
              <span><b>{{ stageLabel(artifact.stage) }}</b><small>{{ artifact.name }}</small></span>
              <i class="verified-dot"></i>
            </button>
          </div>

          <div class="project-stage-track">
            <button v-for="stage in projectStages" :key="stage.key" type="button" class="stage-node"
              :class="{ done: project.stageEvidence[stage.key]?.length, missing: !project.stageEvidence[stage.key]?.length }"
              :title="stageTitle(project, stage)" @click="openStageEvidence(project, stage.key)">
              <span><Icon :icon="stage.icon" width="15" /></span><small>{{ stage.label }}</small>
            </button>
          </div>
          <p v-if="!project.ready" class="project-gap"><Icon icon="mdi:alert-circle-outline" width="15" /> 还缺 {{ project.missingStages.map(stageLabel).join('、') }}，未达到项目审批条件。</p>

          <div class="project-agents">
            <span v-for="agent in project.agents.slice(0, 6)" :key="agent">{{ agent }}</span>
            <span v-if="project.agents.length > 6">+{{ project.agents.length - 6 }}</span>
          </div>
          <details class="project-deliveries">
            <summary>查看项目交付清单 <b>{{ project.artifacts.length }}</b></summary>
            <button v-for="artifact in project.artifacts" :key="artifact.agent + artifact.path" type="button" @click="previewProjectArtifact(project, artifact)">
              <span>{{ artifact.agent }}</span><code>{{ artifact.name }}</code><em>{{ stageLabel(artifact.stage) }}</em>
            </button>
          </details>
          <div class="project-actions">
            <button class="reject-project" :disabled="decidingProject === project.key" @click="approveProject(project, 'reject')"><Icon icon="mdi:backup-restore" width="16" /> 整体退回</button>
            <button class="approve-project" :disabled="decidingProject === project.key || !project.ready" :title="project.ready ? '批准整个项目' : '阶段未齐全，不能批准'" @click="approveProject(project, 'approve')"><Icon icon="mdi:check-bold" width="16" /> {{ decidingProject === project.key ? '处理中…' : (project.ready ? '批准整个项目' : '等待完整交付') }}</button>
          </div>
        </article>
      </div>
      <div v-else class="approval-empty">没有由多个 Agent 完成的团队项目。单 Agent 试验不会再伪装成可审批项目。</div>
    </section>

    <!-- 多 Agent 作战室 -->
    <div v-show="activeSection === 'live'" class="war-room">
      <section class="orchestration-strip">
        <div class="production-overview">
          <div class="production-project">
            <span class="production-eyebrow"><i></i> CURRENT PROJECT</span>
            <h2>{{ currentProductionProject?.title || activeGoal?.objective || '等待项目进入生产线' }}</h2>
            <div class="production-tags" aria-label="项目标签">
              <span v-for="tag in currentProjectTags" :key="tag">{{ tag }}</span>
            </div>
          </div>
          <div class="production-score" :class="{ complete: productionProgressPercent === 100 }">
            <strong>{{ productionProgressPercent }}</strong><span>%</span>
            <small>{{ productionProgressLabel }}</small>
          </div>
        </div>
        <div class="department-progress" role="progressbar" aria-label="项目部门进度"
          aria-valuemin="0" aria-valuemax="100" :aria-valuenow="productionProgressPercent">
          <div v-for="(phase, i) in departmentProgress" :key="phase.role" class="department-phase"
            :class="{ done: phase.done, current: phase.current, pending: !phase.done && !phase.current }"
            :style="{ '--dept-color': phase.color, '--dept-rgb': phase.rgb }">
            <div class="phase-rail"><span :style="{ width: (phase.fill ?? 100) + '%' }"></span><i></i></div>
            <div class="phase-copy">
              <span class="phase-icon"><Icon :icon="phase.icon" width="17" /></span>
              <span><b>{{ phase.name }}</b><small>{{ phase.status }}</small></span>
              <em>0{{ i + 1 }}</em>
            </div>
          </div>
        </div>
      </section>

      <div class="war-grid">
        <section class="graph-panel">
          <div class="panel-heading">
            <div><span>HANDOFF GRAPH</span><h2>Agent 如何真正接力</h2></div>
            <div class="graph-head-actions">
              <div class="graph-view-switch" aria-label="协作图密度">
                <button type="button" :class="{ active: graphMode === 'focus' }" @click="setGraphMode('focus')">主干</button>
                <button type="button" :class="{ active: graphMode === 'all' }" @click="setGraphMode('all')">全部</button>
              </div>
              <div class="panel-stat"><strong>{{ displayGraphLinks.length }}</strong><small>/ {{ graphState.links.length }} 引用链</small></div>
            </div>
          </div>
          <div class="graph-stage" v-if="graphState.nodes.length">
            <div class="graph-tools" aria-label="协作图控制">
              <button type="button" title="放大" @click="zoomGraph(1.18)">＋</button>
              <button type="button" title="缩小" @click="zoomGraph(0.85)">－</button>
              <button type="button" title="适应画布" @click="fitGraph"><Icon icon="mdi:fit-to-screen-outline" width="15" /></button>
              <button type="button" title="恢复初始布局" @click="resetGraph"><Icon icon="mdi:restore" width="15" /></button>
              <span>{{ Math.round(graphZoom * 100) }}%</span>
            </div>
            <svg ref="graphSvg" :viewBox="graphState.viewBox" class="collab-svg" :class="{ moving: graphGesture }"
              @pointerdown="onCanvasPointerDown" @pointermove="onGraphPointerMove" @pointerup="onGraphPointerUp"
              @pointercancel="onGraphPointerUp" @wheel.prevent="onGraphWheel">
              <g :transform="graphTransform">
                <g v-for="d in graphState.deptLabels" :key="'lane' + d.key" class="cm-lane">
                  <line :x1="d.x" y1="62" :x2="d.x" y2="468" />
                  <text :x="d.x" :y="d.y" text-anchor="middle" class="cm-dept-label">{{ d.name }}</text>
                </g>
                <g v-for="lk in displayGraphLinks" :key="lk.key" class="cm-link">
                  <line class="cm-link-glow" :x1="lk.x1" :y1="lk.y1" :x2="lk.x2" :y2="lk.y2" :stroke="lk.color"
                    :stroke-width="lk.w + 5" :stroke-opacity="linkOpacity(lk) * lk.intensity * .18" vector-effect="non-scaling-stroke" />
                  <line class="cm-link-base" :x1="lk.x1" :y1="lk.y1" :x2="lk.x2" :y2="lk.y2" :stroke="lk.color"
                    :stroke-width="lk.w" :stroke-opacity="linkOpacity(lk) * lk.intensity * .72" vector-effect="non-scaling-stroke" />
                  <line class="cm-link-pulse" :x1="lk.x1" :y1="lk.y1" :x2="lk.x2" :y2="lk.y2" :stroke="lk.color"
                    :stroke-width="lk.w + 1.4" :stroke-opacity="linkOpacity(lk) * lk.intensity"
                    :style="{ animationDelay: lk.pulseDelay, animationDuration: lk.pulseDuration }" vector-effect="non-scaling-stroke" />
                </g>
                <g v-for="n in displayGraphNodes" :key="n.name" class="cm-node"
                  :class="{ hub: n.hub, sel: selectedNode && selectedNode.name === n.name, dragging: draggingNodeName === n.name, relay: n.relaying }"
                  @pointerdown.stop.prevent="onNodePointerDown($event, n)" @click="selectNode(n)">
                  <circle :cx="n.x" :cy="n.y" :r="n.r" :fill="n.hub ? '#22c55e' : (n.relaying ? '#f59e0b' : (activeDepts.has(n.role) ? n.stroke : '#111c2e'))" :stroke="n.hub ? '#86efac' : (n.relaying ? '#fbbf24' : (activeDepts.has(n.role) ? n.stroke : '#64748b'))" stroke-width="2"><title>{{ n.title }}</title></circle>
                  <text :x="n.x" :y="n.y" text-anchor="middle" dominant-baseline="central" fill="#f8fafc" class="cm-node-id">{{ n.short }}</text>
                  <text :x="n.x" :y="n.y + n.r + 12" text-anchor="middle" class="cm-node-name">{{ n.name }}</text>
                  <text v-if="n.relaying && n.relayStage" :x="n.x" :y="n.y - n.r - 8" text-anchor="middle" class="cm-node-relay">{{ n.relayStage }}</text>
                </g>
              </g>
            </svg>
            <div class="graph-caption"><span><i></i> {{ graphMode === 'focus' ? '主干中枢' : '全量中枢' }} {{ selectedNode?.name || graphState.hub }}</span><span>默认降噪 · 点击节点切换主干 · 可拖拽/缩放</span></div>
          </div>
          <div v-if="selectedNode" class="node-inspector">
            <div><span>{{ roleLabel(selectedNode.role) }}</span><strong>{{ selectedNode.name }}</strong></div>
            <p><b>输入</b>{{ selectedNodeOut }}</p><p><b>下游</b>{{ selectedNodeIn }}</p>
            <button @click="selectedNode = null">关闭</button>
          </div>
        </section>

        <section class="trace-panel">
          <div class="panel-heading"><div><span>LIVE TRACE</span><h2>此刻正在发生</h2></div><span class="trace-live"><i></i> LIVE</span></div>
          <div class="trace-list">
            <div v-for="(e, i) in recentEvents" :key="i" class="trace-row">
              <span class="trace-time">{{ e.timeText || formatTime(e.createdAt) }}</span>
              <span class="trace-icon"><Icon :icon="roleIcon(e.role || '')" width="15" /></span>
              <div><strong>{{ e.name || e.actor }}</strong><p>{{ e.message }}</p></div>
              <span class="trace-ok">✓</span>
            </div>
            <div v-if="!recentEvents.length" class="trace-empty">等待执行事件…</div>
          </div>
        </section>
      </div>

      <section class="agent-console">
        <div class="panel-heading">
          <div><span>EXECUTION NODES</span><h2>{{ selectedDepartment.name }} · 执行现场</h2></div>
          <div class="dept-switcher">
            <button v-for="dept in departments" :key="dept.key" :class="{ active: activeDept === dept.key }" @click="activeDept = dept.key">{{ dept.name }} <b>{{ dept.agents.length }}</b></button>
          </div>
        </div>
        <div class="agent-rows">
          <article v-for="agent in selectedDepartment.agents" :key="agent.name" class="agent-row" :class="{ busy: isBusy(agent), dead: isDead(agent) }">
            <span class="agent-avatar"><Icon :icon="roleIcon(agent.role)" width="19" /></span>
            <div class="agent-id"><strong>{{ agent.name }}</strong><small :class="statusClass(agent)">{{ statusText(agent) }}</small></div>
            <p class="agent-doing">{{ agent.task || doingText(agent) }}</p>
            <span v-if="agent.lastActive" class="agent-last" :title="'最后活动：' + agent.lastActive">{{ agent.lastActive }}</span>
            <div class="agent-handoff"><Icon icon="mdi:source-branch" width="15" /><span>{{ (agent.collabRefs || []).length }} handoffs</span></div>
            <div class="agent-artifacts">
              <button v-for="f in (agent.files || []).slice(0, 2)" :key="f" @click="previewFile(agent, f)">{{ fileType(f) }}</button>
              <span v-if="!(agent.files || []).length">暂无交付</span>
            </div>
            <strong class="agent-output">{{ agent.outputs || 0 }}</strong>
          </article>
        </div>
      </section>
    </div>

      <!-- 开罗式办公室场景：俯视公司，每个员工走动 + 头顶飘字（游戏发展国风格） -->
      <section v-show="activeSection === 'office'" class="office-scene">
        <div class="section-heading">
          <div>
            <p class="section-kicker">GAME DEV STORY · 公司总部</p>
            <h2>开发公司办公室</h2>
          </div>
          <span class="office-money"><b>💰 ¥{{ financeBalance.toLocaleString() }}</b><small>{{ financeTotalIn.toLocaleString() }} 累计销量收入</small></span>
        </div>
        <div class="office-map">
          <!-- 摆件 -->
          <span class="office-prop prop-water">🪴</span>
          <span class="office-prop prop-desk1">🖥️</span>
          <span class="office-prop prop-desk2">☕</span>
          <span class="office-prop prop-desk3">📦</span>
          <!-- CEO 办公室 -->
          <div class="office-dept dept-ceo" :style="deptStyle('ceo')">
            <div class="dept-sign">👑 CEO 办公室 <b>{{ deptPeople('ceo').length }}</b></div>
            <div v-for="p in deptPeople('ceo')" :key="p.name" class="office-person" :class="p.cls" :style="personStyle(p)">
              <span class="op-bubble"><i :class="p.dot"></i>{{ p.task }}</span>
              <span class="op-face">{{ p.emoji }}</span>
              <small class="op-name">{{ p.short }}</small>
            </div>
          </div>
          <!-- 6 个部门 -->
          <div v-for="d in officeDepts" :key="d.key" class="office-dept" :style="deptStyle(d.key)">
            <div class="dept-sign">{{ d.icon }} {{ d.name }} <b>{{ deptPeople(d.key).length }}</b></div>
            <div v-for="p in deptPeople(d.key)" :key="p.name" class="office-person" :class="p.cls" :style="personStyle(p)">
              <span class="op-bubble"><i :class="p.dot"></i>{{ p.task }}</span>
              <span class="op-face">{{ p.emoji }}</span>
              <small class="op-name">{{ p.short }}</small>
            </div>
          </div>
        </div>
      </section>

      <!-- 应用大厅：B站式信息流，真实评分排序 + 花钱推广置顶 -->
      <section v-show="activeSection === 'market'" class="market-room">
        <div class="section-heading">
          <div>
            <p class="section-kicker">APP STORE · 算法推流</p>
            <h2>应用大厅</h2>
          </div>
          <span class="reviews-total">{{ marketGames.length }} 款应用 · 推荐分 = 真实评分×人数 + 推广加权</span>
        </div>
        <div class="market-grid">
          <article v-for="g in marketGames" :key="g.agent + g.project" class="market-card" :class="{ promoted: g.promoted, unreviewed: !g.has_review }">
            <div class="mc-cover">
              <span class="mc-icon">{{ g.promoted ? '⭐' : (g.award ? awardEmoji(g.award.title) : '📦') }}</span>
              <span v-if="g.promoted" class="mc-promo">推广</span>
              <span v-if="g.award" class="mc-award" :class="awardClass(g.award.title)">{{ g.award.title }}</span>
            </div>
            <div class="mc-body">
              <h3>{{ g.project }}</h3>
              <p class="mc-agent">{{ g.agent }} 出品</p>
              <div class="mc-meta">
                <template v-if="g.has_review">
                  <span class="mc-score" :class="{ good: g.avg_score >= 7, mid: g.avg_score >= 5 && g.avg_score < 7 }"><b>{{ g.avg_score.toFixed(1) }}</b>/10</span>
                  <span class="mc-count">{{ g.count }} 位真实用户评分</span>
                </template>
                <span v-else class="mc-wait">⏳ 等待真实用户评测</span>
              </div>
              <p v-if="g.promo_desc" class="mc-promo-desc">{{ g.promo_desc }}</p>
            </div>
          </article>
        </div>
        <div v-if="!marketGames.length" class="reviews-empty">
          <Icon icon="mdi:storefront-outline" width="30" />
          <p>还没有应用上架。等 coder 完成交付后，应用大厅自动展示。</p>
        </div>
      </section>

      <section v-show="activeSection === 'tags'" class="tags-room">
      <div class="section-heading">
        <div>
          <p class="section-kicker">0 · 预设</p>
          <h2>标签管理 · 前沿技术迭代</h2>
        </div>
      </div>

      <div class="tags-input-row">
              <input v-model="tagInput" class="tags-input" placeholder="输入标签，如 AI Agent、多模态、融资…" @keyup.enter="addTag" />
              <button class="tags-add-btn" :disabled="!tagInput.trim() || tagAdding" @click="addTag">{{ tagAdding ? '添加中…' : '添加标签' }}</button>
            </div>

            <!-- 下达指令：用户自定义考题/项目目标，立项最高优先级（1vs100 考题通道） -->
            <div class="directive-zone">
              <div class="directive-head">
                <h3>📢 下达指令（考题）</h3>
                <span class="directive-tip">指令优先于标签/热点，公司立项必须围绕它执行</span>
              </div>
              <div class="directive-input-row">
                        <input v-model="directiveInput" class="directive-input" placeholder="例：做一个番茄钟+待办小工具，要能运行，限 30 分钟" @input="directiveInputDirty = true" @keyup.enter="saveDirective" />
                        <select v-model="directiveModel" class="directive-model-select" title="指定公司用哪个模型跑（留空=自动轮换）" @change="directiveModelTouched = true">
                          <option value="">模型：自动轮换</option>
                          <option v-for="m in directiveModelOptions" :key="m.id" :value="m.id">{{ m.name }}</option>
                        </select>
                        <button class="directive-save-btn" :disabled="directiveSaving" @click="saveDirective">{{ directiveSaving ? '下达中…' : '下达指令' }}</button>
                        <button v-if="directiveText" class="directive-clear-btn" :disabled="directiveSaving" @click="clearDirective">清除</button>
                      </div>
                      <div class="directive-output-row">
                        <label>产出输出 token 数</label>
                        <input v-model.number="modelMaxTokens" type="number" min="512" max="65536" step="512" class="directive-output-input" @change="saveModelMaxTokens" />
                        <button class="directive-output-save" :disabled="tokenSaving" @click="saveModelMaxTokens">{{ tokenSaving ? '保存中…' : '保存' }}</button>
                        <span class="directive-output-hint">较少的 token 预留会导致代码截断、产品无样式或互动；更多 token 让 Agents 更保证产品质量，但耗时更多（范围 512~65536）</span>
                      </div>
                      <div v-if="directiveError" class="directive-error">{{ directiveError }}</div>
                      <div v-if="directiveText" class="directive-current">
                        <Icon icon="mdi:bullhorn" width="16" />
                        <span>当前指令：<strong>{{ directiveText }}</strong><template v-if="directiveModel"><em> · 模型：{{ directiveModel }}</em></template></span>
                      </div>
            </div>

      <div class="tags-hot" v-if="hotTags.length">
        <h3>🔥 热门标签</h3>
        <div class="tags-shell">
          <span v-for="t in hotTags" :key="t.id" class="tag-pill hot" @click="tagInput = t.name; addTag()">
            {{ t.name }}
            <small>{{ t.usedCount }}</small>
          </span>
        </div>
      </div>

      <div class="tags-list" v-if="tagsList.length">
        <h3>我的标签（{{ tagsList.length }}）</h3>
        <div class="tags-shell">
          <span v-for="t in tagsList" :key="t.id" class="tag-pill">
            {{ t.name }}
            <small>×{{ t.usedCount }}</small>
            <button class="tag-del" @click="deleteTag(t.id)" title="删除标签">×</button>
          </span>
        </div>
      </div>
      <div v-else-if="!loadingTags" class="tags-empty">
        <Icon icon="mdi:tag-plus-outline" width="32" />
        <p>还没有标签，输入一个开始记录调研方向吧</p>
      </div>

      <!-- 迭代区：从已审批项目选，每天调研前沿技术迭代产品 -->
      <div class="iterate-zone">
        <h3 class="iterate-title">⚡ 前沿技术迭代</h3>
        <p class="iterate-desc">从已审批的项目里选一个，让 agents 每天调研最前沿技术来迭代该产品。</p>

        <div v-if="iteratePlans.length" class="iterate-plans">
          <h4>迭代中（{{ iteratePlans.length }}）</h4>
          <div v-for="p in iteratePlans" :key="p.project" class="iterate-plan-card">
            <div class="iterate-plan-head">
              <strong>{{ p.name }}</strong>
              <div class="iterate-plan-actions">
                <span class="iterate-badge">LIVE</span>
                <button class="iterate-stop-btn" @click="stopIterate(p)">停止</button>
              </div>
            </div>
            <p class="iterate-summary">{{ p.lastReport || '报告生成中…' }}</p>
            <small class="iterate-meta">开始于 {{ formatTime(p.startedAt) }}</small>
          </div>
        </div>

        <div v-if="iterateCandidates.length && !iteratePlans.length" class="iterate-candidates">
          <h4>候选项目（已审批，可开始迭代）</h4>
          <div v-for="cd in iterateCandidates" :key="cd.project" class="iterate-candidate-row">
            <span class="iterate-cand-name">{{ cd.name }}</span>
            <button class="iterate-start-btn" :disabled="iterateStarting" @click="startIterate(cd)">{{ iterateStarting ? '启动中…' : '开始迭代' }}</button>
          </div>
        </div>
        <div v-else class="iterate-empty">
          <Icon icon="mdi:rocket-launch-outline" width="30" />
          <p>还没有可迭代的已审批项目。去审批台通过一个项目后，就能在这里让它每天自我迭代。</p>
        </div>
      </div>
    </section>

      <!-- 真实评分评奖：每一个评分都来自真实用户 -->
      <section class="reviews-room">
        <div class="section-heading">
          <div>
            <p class="section-kicker">LIVE · 真实评分评奖</p>
            <h2>真实用户打分 · 评奖</h2>
          </div>
          <span v-if="reviewsList.length" class="reviews-total">{{ reviewsList.length }} 个产品已获真实评分</span>
        </div>
        <div v-if="reviewsList.length" class="reviews-list">
          <article v-for="rv in reviewsList" :key="rv.agent + rv.project" class="review-card">
            <header class="review-head">
              <div class="review-title">
                <strong>《{{ rv.project }}》</strong>
                <span class="review-agent">{{ rv.agent }}</span>
                <span v-if="rv.award" class="review-award" :class="awardClass(rv.award.title)">{{ rv.award.title }}</span>
              </div>
              <div class="review-score" :class="{ good: rv.avg_score >= 7, mid: rv.avg_score >= 5 && rv.avg_score < 7 }">
                <strong>{{ rv.avg_score.toFixed(1) }}</strong><small>/10 · {{ rv.count }} 位真实用户</small>
              </div>
            </header>
            <div class="review-users">
              <div v-for="u in rv.users" :key="u.nickname + u.created_at" class="review-user">
                <span class="ru-avatar">👤</span>
                <div class="ru-body">
                  <header>
                    <strong>{{ u.nickname }}</strong>
                    <span class="ru-stars">{{ '★'.repeat(Math.max(1, Math.round(u.score / 2))) }}<i>{{ u.score }}</i></span>
                    <em v-if="u.created_at" class="ru-model">{{ u.created_at }}</em>
                  </header>
                  <p>{{ u.comment }}</p>
                </div>
              </div>
            </div>
            <form class="review-form" @submit.prevent="submitReview(rv)">
              <input v-model="rv._nick" :placeholder="reviewNickname || '你的昵称'" class="rf-nick" maxlength="12" />
              <div class="rf-stars">
                <button v-for="s in 5" :key="s" type="button" class="rf-star" :class="{ on: rv._stars >= s }" @click="rv._stars = s">★</button>
                <span v-if="rv._stars" class="rf-score-num">{{ rv._stars * 2 }} 分</span>
              </div>
              <input v-model="rv._comment" placeholder="说说你的真实体验（≤120字）" class="rf-comment" maxlength="120" />
              <button type="submit" class="rf-submit" :disabled="rv._submitting">{{ rv._submitting ? '提交中…' : '提交评分' }}</button>
              <span v-if="rv._msg" class="rf-msg">{{ rv._msg }}</span>
            </form>
          </article>
        </div>
        <div v-else class="reviews-empty">
          <Icon icon="mdi:star-outline" width="30" />
          <p>还没有产品获得真实评分。等 coder 完成交付后，你——真实用户，来给第一个产品打分。</p>
        </div>
      </section>

      </div><!-- /section-content -->
    </div><!-- /company-layout -->

    <!-- 技能习得气泡（实机：agent 提炼技能时弹出） -->
    <Teleport to="body">
      <div class="skill-toast-layer">
        <TransitionGroup name="skill-pop">
          <div v-for="t in skillToasts" :key="t.id" class="skill-toast">
            <div class="st-icon"><Icon icon="mdi:lightbulb-on-outline" width="18" /></div>
            <div class="st-body">
              <span class="st-tag">技能习得 · {{ t.agent }}</span>
              <strong>{{ t.name }}</strong>
            </div>
            <span class="st-xp">+{{ t.xp }} XP</span>
          </div>
        </TransitionGroup>
      </div>
    </Teleport>

    <div v-if="artifactModal.open" class="modal-backdrop" @click.self="artifactModal.open = false">
      <section class="artifact-modal" role="dialog" aria-modal="true" aria-label="交付物内容">
        <header>
          <div><p class="section-kicker">{{ artifactModal.agent }}</p><h2>{{ artifactModal.file }}</h2></div>
          <div class="artifact-actions">
            <span v-if="artifactModal.kind === 'pptx' && pptDeck.slides.length" class="slide-counter">{{ slideIndex + 1 }} / {{ pptDeck.slides.length }}</span>
            <a v-if="artifactModal.rawUrl" :href="artifactModal.rawUrl" :download="artifactModal.file">下载原始产物</a>
            <button type="button" aria-label="关闭" @click="artifactModal.open = false">×</button>
          </div>
        </header>
        <div v-if="artifactModal.loading" class="artifact-loading">正在解析真实产物…</div>
        <div v-else-if="artifactModal.kind === 'video'" class="media-preview video-preview">
          <video :src="artifactModal.rawUrl" controls autoplay playsinline preload="metadata"></video>
        </div>
        <div v-else-if="artifactModal.kind === 'html'" class="media-preview app-preview">
          <iframe :srcdoc="artifactModal.content" sandbox="allow-forms allow-modals allow-scripts" title="可运行程序预览"></iframe>
        </div>
        <div v-else-if="artifactModal.kind === 'image'" class="media-preview image-preview">
          <img :src="artifactModal.rawUrl" :alt="artifactModal.file" />
        </div>
        <div v-else-if="artifactModal.kind === 'pptx' && pptDeck.slides.length" class="slide-view pptx-view">
          <div class="pptx-slide" :style="{ aspectRatio: `${pptDeck.size.w} / ${pptDeck.size.h}` }">
            <template v-for="(element, idx) in pptDeck.slides[slideIndex].elements" :key="idx">
              <div v-if="element.type === 'text'" class="pptx-element pptx-text" :style="pptxElementStyle(element, pptDeck.size)">{{ element.text }}</div>
              <img v-else class="pptx-element pptx-image" :src="element.src" :style="pptxElementStyle(element, pptDeck.size)" alt="" />
            </template>
          </div>
          <div class="slide-nav">
            <button :disabled="slideIndex === 0" @click="slideIndex--">‹ 上一页</button>
            <button :disabled="slideIndex === pptDeck.slides.length - 1" @click="slideIndex++">下一页 ›</button>
          </div>
        </div>
        <div v-else-if="artifactModal.kind === 'spreadsheet' && spreadsheetRows.length" class="spreadsheet-preview">
          <table><tbody><tr v-for="(row, r) in spreadsheetRows" :key="r"><th>{{ r + 1 }}</th><td v-for="(cell, col) in row" :key="col">{{ cell }}</td></tr></tbody></table>
          <span v-if="artifactModal.truncated">仅预览前 40 行，下载文件查看完整工作簿</span>
        </div>
        <div v-else-if="artifactModal.kind === 'spreadsheet'" class="artifact-empty">工作簿没有可显示的单元格，请下载后检查。</div>
        <div v-else-if="artifactModal.kind === 'pptx'" class="artifact-empty">PPTX 已生成，但文件结构无法解析；请下载后用 PowerPoint 打开。</div>
        <div v-else-if="isOutlineOnly" class="outline-warning"><strong>这不是成品 PPT / PV</strong><span>当前文件只是 Markdown 大纲或视频脚本，已从多媒体交付统计中剔除。</span></div>
        <div v-else-if="artifactModal.kind === 'binary'" class="artifact-empty">该二进制产物暂不支持内嵌预览，请下载原始文件。</div>
        <div v-else class="artifact-body" v-html="renderMarkdown(artifactModal.content)"></div>
      </section>
    </div>
  </main>
</template>

<script setup>
import { ref, computed, reactive, watch, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import { API_BASE_URL } from '../config.js'
import { parsePptxPreview, pptxElementStyle } from '../utils/pptxPreview.js'
import { parseXlsxPreview, parseDelimitedPreview } from '../utils/xlsxPreview.js'
const goals = ref([])
const agents = ref([])
const osStats = ref({ totalAgents: 0, workingCount: 0 })
const pendingApprovals = ref([])
const decidingProject = ref('')
const hiddenApprovalProjects = ref(new Set())
const meetings = ref([]) // 会议纪要（会议室 tab 展示真实开会内容）
// ===== 标签系统（调研方向 + 历史记录 + 热门标签）=====
const tagsList = ref([])
const hotTags = ref([])
const tagInput = ref('')
const tagAdding = ref(false)
const loadingTags = ref(true)
// ===== 下达指令（用户自定义考题/项目目标，立项最高优先级）=====
const directiveText = ref('')
const directiveInput = ref('')
const directiveModel = ref('')
const directiveModelTouched = ref(false)
const directiveInputDirty = ref(false)
const directiveSaving = ref(false)
const directiveModelOptions = ref([])
const directiveRun = ref({ status: 'idle', project: '', error: '', updatedAt: '' })
const directiveError = ref('')
// ===== 公司生产设置：产出输出 token 数（用户可填，存后端 config）=====
const modelMaxTokens = ref(16384)
const tokenSaving = ref(false)
async function loadModelMaxTokens() {
  try {
    const d = await api('/api/company/model-config')
    if (d && d.max_tokens) modelMaxTokens.value = Number(d.max_tokens)
  } catch (e) { /* 后端没起静默 */ }
}
async function saveModelMaxTokens() {
  tokenSaving.value = true
  try {
    const v = Math.max(512, Math.min(65536, Number(modelMaxTokens.value) || 16384))
    modelMaxTokens.value = v
    await api('/api/company/model-config', { method: 'PUT', body: JSON.stringify({ max_tokens: v }), headers: { 'Content-Type': 'application/json' } })
  } catch (e) { /* 静默 */ } finally { tokenSaving.value = false }
}
const iteratePlans = ref([])
const iterateCandidates = ref([])
const iterateStarting = ref(false)
const reviewsList = ref([])
async function loadIterate() {
  try {
    const d = await api('/api/company/iterate')
    iteratePlans.value = d.plans || []
    iterateCandidates.value = d.candidates || []
  } catch (e) {}
}
async function loadReviews() {
  try {
    const d = await api('/api/company/reviews')
    if (d && Array.isArray(d.reviews)) {
      reviewsList.value = d.reviews.map(r => ({ ...r, _stars: 0, _comment: '', _nick: '', _submitting: false, _msg: '' }))
    }
  } catch (e) { /* 后端没起静默 */ }
}
// 真实用户提交评分（昵称记忆在 localStorage，同昵称同项目覆盖旧评分）
const reviewNickname = ref(localStorage.getItem('rescene_reviewer') || '')
async function submitReview(rv) {
  const nick = (rv._nick || reviewNickname.value || '').trim()
  if (!nick) { rv._msg = '先填个昵称吧～'; return }
  const score = (rv._stars || 0) * 2
  if (score < 2) { rv._msg = '先点星星打分'; return }
  if (!(rv._comment || '').trim()) { rv._msg = '写一句真实感受再提交'; return }
  rv._submitting = true
  rv._msg = ''
  try {
    localStorage.setItem('rescene_reviewer', nick)
    const d = await api('/api/company/reviews', {
      method: 'POST',
      body: JSON.stringify({ project: rv.project, nickname: nick, score, comment: rv._comment.trim() }),
      headers: { 'Content-Type': 'application/json' },
    })
    rv._msg = d.award ? `提交成功！获得【${d.award.title}】` : '提交成功！'
    await loadReviews()
  } catch (e) {
    rv._msg = '提交失败：' + e.message
  } finally {
    rv._submitting = false
  }
}
function awardClass(title) {
  if (title.includes('神作')) return 'award-god'
  if (title.includes('好评')) return 'award-good'
  return 'award-new'
}
async function startIterate(cd) {
  iterateStarting.value = true
  try {
    const plan = await api('/api/company/iterate', { method: 'POST', body: JSON.stringify({ project: cd.project, name: cd.name }), headers: { 'Content-Type': 'application/json' } })
    iteratePlans.value = [plan, ...iteratePlans.value.filter(item => item.project !== plan.project)]
    await loadIterate()
    activeSection.value = 'live'
  } catch (e) { console.error(e) } finally { iterateStarting.value = false }
}
async function stopIterate(p) {
  if (!p || !p.project) return
  try {
    const r = await api('/api/company/iterate/stop', { method: 'POST', body: JSON.stringify({ project: p.project }), headers: { 'Content-Type': 'application/json' } })
    if (r && r.ok) {
      iteratePlans.value = iteratePlans.value.filter(item => item.project !== p.project)
      await loadIterate()
    }
  } catch (e) { console.error('stopIterate', e) }
}
async function loadTags() {
  try {
    loadingTags.value = true
    const [mine, hot] = await Promise.allSettled([
      api('/api/company/tags'),
      api('/api/company/tags/hot'),
    ])
    if (mine.status === 'fulfilled') tagsList.value = mine.value.tags || []
    if (hot.status === 'fulfilled') hotTags.value = hot.value.hot || []
  } catch (e) { /* 后端没起静默 */ } finally { loadingTags.value = false }
}
async function loadDirective() {
  try {
    const [d, cfg] = await Promise.allSettled([
      directiveApi('/api/company/directive'),
      api('/api/models/config'),
    ])
    if (d.status === 'fulfilled' && d.value) {
      directiveText.value = d.value.directive || ''
      // 用户正在输入时，轮询不能覆盖输入框里的草稿（修复 #10：等待几秒自动清空）。
      if (!directiveInputDirty.value) directiveInput.value = d.value.directive || ''
      // 轮询只同步已经持久化的值，不能覆盖用户刚在下拉框里做出的选择。
      if (!directiveModelTouched.value && !directiveSaving.value) directiveModel.value = d.value.model || ''
      directiveRun.value = d.value.run || { status: 'idle', project: '', error: '', updatedAt: '' }
      directiveError.value = ''
    } else if (d.status === 'rejected') {
      directiveError.value = d.reason?.message || '指令服务未启动'
    }
    if (cfg.status === 'fulfilled' && cfg.value) {
          // 用户已经接入的自定义模型优先；再补免费池。旧逻辑只截前 60 个
          // free_models，导致 DeepSeek 自定义 API 明明已配置却不在下拉框里。
          const providers = [
            ...(cfg.value.custom_models || []),
            ...(cfg.value.free_models || []),
            ...(cfg.value.configs || []),
          ]
          const seen = new Set()
          directiveModelOptions.value = providers
            .filter(p => p && p.id && !seen.has(p.id) && seen.add(p.id))
            .map(p => ({
              id: p.id,
              name: p.vendor ? `${p.vendor} · ${p.name || p.default_model || p.id}` : (p.name || p.default_model || p.id),
            }))
        }
  } catch (e) { /* 后端没起静默 */ }
}
async function saveDirective() {
  const d = directiveInput.value.trim()
  if (directiveSaving.value) return
  const requestedModel = directiveModel.value
  directiveSaving.value = true
  directiveError.value = ''
  try {
    const r = await directiveApi('/api/company/directive', { method: 'PUT', body: JSON.stringify({ directive: d, model: requestedModel }), headers: { 'Content-Type': 'application/json' } })
    if (r && r.ok) {
      directiveText.value = r.directive || ''
      directiveInput.value = r.directive || ''
      directiveInputDirty.value = false
      directiveModel.value = r.model || ''
      directiveModelTouched.value = false
      directiveRun.value = r.run || { status: 'queued', project: '', error: '', updatedAt: '' }
      activeSection.value = 'live'
      await Promise.allSettled([loadData({ quiet: true }), loadApprovals(), loadProductionAudit()])
    }
  } catch (e) {
    directiveError.value = `下达失败：${e?.message || '交付服务不可用'}`
    console.error('saveDirective', e)
  } finally { directiveSaving.value = false }
}
async function clearDirective() {
  directiveInput.value = ''
  await saveDirective()
}
async function addTag() {
  const name = tagInput.value.trim()
  if (!name || tagAdding.value) return
  tagAdding.value = true
  try {
    const r = await api('/api/company/tags', { method: 'POST', body: JSON.stringify({ name }) })
    if (r.ok) {
      await loadTags()
      tagInput.value = ''
    }
  } catch (e) { console.error('addTag', e) } finally { tagAdding.value = false }
}
async function deleteTag(id) {
  try {
    await api('/api/company/tags/' + id, { method: 'DELETE' })
    await loadTags()
  } catch (e) { console.error('deleteTag', e) }
}
const integrations = ref({ microsoft: {}, database: {}, navicat: {} })
const productionAudit = ref({ counts: {}, departments: [], passed: 0, total: 6 })
const activeSection = ref('live') // 竖标签：approval | live | meeting
const meetingNotes = ref([])
const activeDept = ref('')
const selectedGoalId = ref('')
const loading = ref(true)
const creating = ref(false)
const deciding = ref(false)
const feedback = ref('')
const error = ref('')
const form = reactive({ objective: '', successMetric: '' })
const artifactModal = reactive({ open: false, loading: false, content: '', agent: '', file: '', kind: '', mime: '', rawUrl: '', truncated: false })
const slideIndex = ref(0)
const pptDeck = reactive({ size: { w: 16, h: 9 }, slides: [] })
const spreadsheetRows = ref([])
const showcaseSelection = ref({})
const isOutlineOnly = computed(() => artifactModal.kind === 'text' && /^(PPT|PV)-/i.test(artifactModal.file || ''))
const latestMeeting = computed(() => meetings.value[0] || null)

const activeGoal = computed(() => goals.value.find(goal => goal.id === selectedGoalId.value) || goals.value[0] || null)
const productionTasks = computed(() => (activeGoal.value?.tasks || []).filter(task => task.stage !== 'approval'))
const approvedCount = computed(() => productionTasks.value.filter(task => task.status === 'approved').length)
const progressPercent = computed(() => productionTasks.value.length ? Math.round((approvedCount.value / productionTasks.value.length) * 100) : 0)
const recentEvents = computed(() => {
  const events = []
  for (const a of agents.value) {
    if (!a.recentLog) continue
    for (const rawLine of a.recentLog.split('\n')) {
      const line = rawLine.trim()
      if (!line) continue
      const m = line.match(/^\[([^\]]+)\]\s*(.*)$/)
      let timeText = m ? m[1] : ''
      // 短时间戳 [23:00] 无日期（live.log 旧行的日级短格式）。若不补日期，它会被字符串排序
      // 当成当天最晚排到最前、显示也缺日期，导致「时间不对」。用该 agent 的 lastActive 日期补全。
      if (timeText && timeText.indexOf('-') === -1 && /^\d{1,2}:\d{2}$/.test(timeText)) {
        const datePart = (a.lastActive || '').split(' ')[0]
        if (datePart) timeText = datePart + ' ' + timeText
      }
      let message = (m ? m[2] : line).slice(0, 90)
      events.push({ role: a.role, name: a.name, message, timeText })
    }
  }
  // 实时接力事件（/api/company/relays）：真实交接，插到 trace 流最新
  for (const r of relays.value) {
    const stage = r.stage || r.artifact || '交接'
    events.push({
      role: String(r.to || r.from || '').split('-')[0],
      name: `${r.from} → ${r.to}`,
      message: `接力 · ${stage}（${r.status === 'running' ? '进行中' : '已交接'}）`,
      timeText: formatRelayTime(r.doneAt),
      relay: true,
    })
  }
  events.sort((x, y) => (x.timeText < y.timeText ? 1 : -1))
  return events.slice(0, 10)
})
const totalOutputs = computed(() => agents.value.reduce((sum, agent) => sum + (agent.outputs || 0), 0))
const totalHandoffs = computed(() => agents.value.reduce((sum, agent) => sum + (agent.collabRefs || []).length, 0))
const standalonePendingCount = computed(() => pendingApprovals.value.filter(item => item.kind !== 'project').length)
const projectApprovalItemCount = computed(() => pendingApprovals.value.filter(item => item.kind === 'project').length)
const projectStages = [
  { key: 'meeting', label: '开会', icon: 'mdi:account-group-outline' },
  { key: 'research', label: '调研', icon: 'mdi:microscope' },
  { key: 'data', label: 'Excel', icon: 'mdi:microsoft-excel' },
  { key: 'requirements', label: '需求', icon: 'mdi:clipboard-text-outline' },
  { key: 'ui', label: 'UI', icon: 'mdi:palette-outline' },
  { key: 'docs', label: '文档', icon: 'mdi:file-document-outline' },
  { key: 'code', label: '编码', icon: 'mdi:code-tags' },
  { key: 'runnable', label: '程序', icon: 'mdi:play-box-outline' },
  { key: 'ppt', label: 'PPT', icon: 'mdi:presentation' },
  { key: 'pv', label: 'PV', icon: 'mdi:movie-open-play-outline' },
  { key: 'promotion', label: '宣传', icon: 'mdi:bullhorn-outline' },
]
const allApprovalProjects = computed(() => {
  const grouped = new Map()
  for (const item of pendingApprovals.value) {
    if (item.kind !== 'project') continue
    const rawName = String(item.file || '').replace(/^project\//, '')
    const title = rawName.replace(/^\d+[-_]/, '') || rawName || '未命名项目'
    const key = title.toLocaleLowerCase()
    if (!grouped.has(key)) grouped.set(key, { key, title, items: [], agents: [], roles: [], score: 0, sourceCount: 0, artifacts: [], stageEvidence: {}, preview: '', previewKind: '', previewFile: null })
    const project = grouped.get(key)
    project.items.push(item)
    if (!project.agents.includes(item.agent)) project.agents.push(item.agent)
    const role = String(item.agent || '').split('-')[0]
    if (role && !project.roles.includes(role)) project.roles.push(role)
    project.score = Math.max(project.score, Number(item.score) || 0)
    if (item.source) project.sourceCount++
    const artifacts = (item.artifacts || []).map(artifact => ({ ...artifact, agent: item.agent, path: `${item.file}/${artifact.name}` }))
    project.artifacts.push(...artifacts)
    for (const stage of (item.stages || [])) {
      if (!project.stageEvidence[stage]) project.stageEvidence[stage] = []
      const evidence = artifacts.filter(artifact => artifact.stage === stage || (stage === 'runnable' && artifact.name === item.source))
      // 只接受可点击的真实文件；项目目录名、角色名和日志话术不能作为完成证据。
      project.stageEvidence[stage].push(...evidence)
    }
    const previewRank = { video: 7, pptx: 6, spreadsheet: 5, html: 4, image: 3, code: 2, text: 1 }
    if (item.previewFile && (previewRank[item.previewKind] || 0) > (previewRank[project.previewKind] || 0)) {
      project.preview = item.preview
      project.previewKind = item.previewKind
      project.previewFile = { agent: item.agent, name: item.previewFile, path: `${item.file}/${item.previewFile}` }
    }
  }
  return [...grouped.values()].map(project => {
    const missingStages = projectStages.filter(stage => !(project.stageEvidence[stage.key] || []).length).map(stage => stage.key)
    return { ...project, completedStageCount: projectStages.length - missingStages.length, missingStages, ready: missingStages.length === 0 }
  }).sort((a, b) => b.agents.length - a.agents.length || b.completedStageCount - a.completedStageCount || a.title.localeCompare(b.title, 'zh-CN'))
})
const approvalProjects = computed(() => allApprovalProjects.value.filter(project => !hiddenApprovalProjects.value.has(project.key)))
const soloProductionCount = computed(() => allApprovalProjects.value.filter(project => project.agents.length < 2).length)

const productionDepartmentPlan = [
  { role: 'researcher', name: '研究部', icon: 'mdi:microscope', stages: ['research', 'data'], color: '#8b5cf6', rgb: '139, 92, 246' },
  { role: 'writer', name: '作者部', icon: 'ph:pen-nib-bold', stages: ['meeting', 'requirements', 'docs'], color: '#f59e0b', rgb: '245, 158, 11' },
  { role: 'designer', name: '设计部', icon: 'mdi:palette', stages: ['ui'], color: '#ec4899', rgb: '236, 72, 153' },
  { role: 'coder', name: '程序部', icon: 'mdi:code-tags', stages: ['code', 'runnable'], color: '#2563eb', rgb: '37, 99, 235' },
  { role: 'promoter', name: '宣传部', icon: 'mdi:megaphone', stages: ['ppt', 'pv'], color: '#14b8a6', rgb: '20, 184, 166' },
  { role: 'publisher', name: '发布部', icon: 'mdi:bullhorn', stages: ['promotion'], color: '#ef4444', rgb: '239, 68, 68' },
]
const latestReviewedProject = computed(() => {
  const meeting = meetings.value.find(item => item.kind === 'project_review')
  if (!meeting) return null
  return {
    title: String(meeting.topic || '').replace(/\s*·\s*完整交付评审\s*$/, '') || '最近完整交付项目',
    agents: meeting.agent ? [meeting.agent] : [],
    completedStageCount: projectStages.length,
    ready: true,
    stageEvidence: null,
    reviewed: true,
  }
})
const currentIterationProject = computed(() => {
  const plan = [...iteratePlans.value].sort((a, b) => String(b.startedAt || '').localeCompare(String(a.startedAt || '')))[0]
  if (!plan) return null
  const researchDelivered = Boolean(plan.reportFile) && !String(plan.lastReport || '').startsWith('首次调研失败')
  return {
    title: `${plan.name || plan.project || '未命名项目'} · 迭代`,
    iteration: true,
    researchDelivered,
    startedAt: plan.startedAt,
    agents: [],
    completedStageCount: researchDelivered ? 1 : 0,
    ready: false,
  }
})
const directiveProductionProject = computed(() => {
  if (!directiveText.value) return null
  const run = directiveRun.value || {}
  const delivered = run.project
    ? allApprovalProjects.value.find(project => project.items.some(item => String(item.file || '').replace(/^project\//, '') === run.project))
    : null
  if (delivered) return { ...delivered, directive: true, runStatus: 'completed' }
  const total = run.totalStages || projectStages.length
  const done = Math.min(Math.max(run.stagesDone || 0, 0), total)
  return {
    title: directiveText.value,
    directive: true,
    runStatus: run.status || 'queued',
    runError: run.error || '',
    agents: [], completedStageCount: done, totalStages: total, ready: false, stageEvidence: {},
  }
})
const currentProductionProject = computed(() => directiveProductionProject.value || currentIterationProject.value || approvalProjects.value[0] || allApprovalProjects.value[0] || latestReviewedProject.value || null)
const currentProjectTags = computed(() => {
  const project = currentProductionProject.value
  if (!project) return ['尚未排产', '等待真实交付']
  if (project.directive && !project.ready) return ['用户指令', project.runStatus === 'failed' ? '生产失败' : project.runStatus === 'completed' ? '已交付 · 等待人类审批' : '多 Agent 生产中', '等待真实交付']
  if (project.iteration) return ['迭代中', '前沿技术调研', project.researchDelivered ? '本轮报告已落盘' : '首轮调研中']
  if (project.reviewed) return [`${project.completedStageCount}/${projectStages.length} 阶段`, '完整交付', '评审留痕']
  return [
    `${project.agents.length} AGENTS`,
    `${project.completedStageCount}/${projectStages.length} 阶段`,
    project.ready ? '完整交付' : '生产中',
  ]
})
const productionProgressPercent = computed(() => {
  const project = currentProductionProject.value
  if (project?.iteration) return project.researchDelivered ? Math.round(100 / productionDepartmentPlan.length) : 0
  if (project) return Math.round((project.completedStageCount / projectStages.length) * 100)
  const total = Number(productionAudit.value.total) || productionDepartmentPlan.length
  return Math.round(((Number(productionAudit.value.passed) || 0) / total) * 100)
})
const productionProgressLabel = computed(() => {
  if (currentProductionProject.value?.directive && !currentProductionProject.value.ready) {
    if (currentProductionProject.value.runStatus === 'failed') return `生产失败 · ${currentProductionProject.value.runError || '请检查交付引擎'}`
    if (currentProductionProject.value.runStatus === 'completed') return '已交付 · 等待人类审批'
    return currentProductionProject.value.runStatus === 'queued' ? '已立项 · 正在唤醒多 Agent' : '多 Agent 正在生成完整交付'
  }
  if (currentProductionProject.value?.iteration) return currentProductionProject.value.researchDelivered ? '本轮调研已交付 · 迭代继续' : '研究部正在执行首轮调研'
  if (productionProgressPercent.value === 100) return '已完成 · 等待人类审批'
  if (productionProgressPercent.value > 0) return '多 Agent 正在接力'
  return currentProductionProject.value ? '项目已排产' : '等待生产证据'
})
const departmentProgress = computed(() => {
  const project = currentProductionProject.value
  // directive 生产中：用 stagesDone/totalStages 比例驱动部门进度，进度条随顶部数字平滑推进
  if (project?.directive && project.totalStages && project.runStatus !== 'completed') {
    const total = project.totalStages || projectStages.length
    const doneRatio = (project.completedStageCount || 0) / total
    const n = productionDepartmentPlan.length
    const deptFrac = 1 / n
    return productionDepartmentPlan.map((phase, index) => {
      const done = doneRatio >= (index + 1) * deptFrac
      const current = !done && doneRatio > index * deptFrac
      let fill = 0
      if (done) fill = 100
      else if (current) fill = Math.round(((doneRatio - index * deptFrac) / deptFrac) * 100)
      return {
        ...phase, done, current, fill,
        status: done ? '已交付' : (current ? '当前阶段' : '等待接力'),
      }
    })
  }
  const audits = new Map(departmentAudits.value.map(dept => [dept.role, dept]))
  if (project?.iteration) {
    return productionDepartmentPlan.map((phase, index) => ({
      ...phase,
      done: index === 0 && project.researchDelivered,
      current: index === 0,
      status: index === 0 ? (project.researchDelivered ? '本轮已交付' : '正在调研') : '等待真实接力',
    }))
  }
  const phases = productionDepartmentPlan.map(phase => {
    const done = project?.stageEvidence
      ? phase.stages.every(stage => (project.stageEvidence[stage] || []).length > 0)
      : Boolean(audits.get(phase.role)?.passed)
    return { ...phase, done }
  })
  const firstPending = phases.findIndex(phase => !phase.done)
  const currentIndex = firstPending === -1 ? phases.length - 1 : firstPending
  return phases.map((phase, index) => ({
    ...phase,
    current: index === currentIndex,
    status: phase.done ? (index === currentIndex ? '交付完成' : '已交付') : (index === currentIndex ? '当前阶段' : '等待接力'),
  }))
})

const showcaseStageOrder = ['research', 'pv', 'runnable', 'ui', 'data', 'docs', 'ppt', 'promotion']
// text 类产物（md/json）的卡片文案按阶段区分，不能都叫「调研报告」。
function textHeroLabel(stage) {
  switch (stage) {
    case 'research': return { en: 'RESEARCH REPORT', cn: '查看调研报告' }
    case 'docs': return { en: 'SOFTWARE DOCS', cn: '查看软件文档' }
    case 'requirements': return { en: 'REQUIREMENTS', cn: '查看需求计划' }
    case 'meeting': return { en: 'MEETING MINUTES', cn: '查看会议纪要' }
    default: return { en: 'TEXT ARTIFACT', cn: '查看文档' }
  }
}
function projectShowcase(project) {
  const artifacts = project?.artifacts || []
  return showcaseStageOrder.map(stage => artifacts.find(artifact => artifact.stage === stage && artifact.previewable)).filter(Boolean)
}
function selectedShowcaseArtifact(project) {
  const choices = projectShowcase(project)
  const selectedPath = showcaseSelection.value[project?.key]
  return choices.find(artifact => artifact.path === selectedPath) || choices[0] || project?.previewFile || null
}
function selectShowcaseArtifact(project, artifact) {
  showcaseSelection.value = { ...showcaseSelection.value, [project.key]: artifact.path }
}
function artifactShowcaseIcon(artifact) {
  return ({ video: 'mdi:movie-open-play-outline', html: 'mdi:application-braces-outline', spreadsheet: 'mdi:microsoft-excel', pptx: 'mdi:microsoft-powerpoint', image: 'mdi:image-outline', text: 'mdi:certificate-outline' })[artifact?.kind] || 'mdi:file-check-outline'
}

const deptMeta = {
  writer: { key: 'writer', name: '作者部', icon: 'ph:pen-nib-bold' },
  researcher: { key: 'researcher', name: '研究部', icon: 'mdi:microscope' },
  coder: { key: 'coder', name: '程序部', icon: 'mdi:code-tags' },
  designer: { key: 'designer', name: '设计部', icon: 'mdi:palette' },
  publisher: { key: 'publisher', name: '发布部', icon: 'mdi:bullhorn' },
  promoter: { key: 'promoter', name: '宣传部', icon: 'mdi:megaphone' },
}

const departments = computed(() => {
  try {
    const map = {}
    for (const k in deptMeta) map[k] = { ...deptMeta[k], agents: [], working: false, outputs: 0 }
    for (const a of agents.value) {
      const d = deptMeta[a.role]
      if (d && map[a.role]) {
        map[a.role].agents.push(a)
        map[a.role].outputs += a.outputs || 0
        if (isBusy(a)) map[a.role].working = true
      }
    }
    return Object.values(map)
  } catch (e) {
    window.__deptErr = (e && e.message) || String(e)
    return []
  }
})
const departmentAudits = computed(() => productionAudit.value.departments?.length ? productionAudit.value.departments : Object.values(deptMeta).map(dept => ({ role: dept.key, name: dept.name, responsibility: '等待磁盘审计', expected: [], evidence: [], passed: false, issue: '审计接口未就绪' })))
const selectedAudit = computed(() => departmentAudits.value.find(dept => dept.role === activeDept.value) || departmentAudits.value[0] || null)
const selectedDepartment = computed(() => departments.value.find(dept => dept.key === activeDept.value) || departments.value[0] || { name: '执行节点', agents: [] })

// 被引用（入）聚合：name -> {count, agents} —— 卡片显示「被 N 人接力」（2026-08-09：被引用的 agent 不该显示独立工作）
const collabIn = computed(() => {
  try {
    const m = {}
    for (const a of agents.value) {
      for (const r of (a.collabRefs || [])) {
        const t = r.agent
        if (!t) continue
        if (!m[t]) m[t] = { count: 0, agents: [] }
        m[t].count++
        if (!m[t].agents.includes(a.name)) m[t].agents.push(a.name)
      }
    }
    return m
  } catch (e) {
    window.__collabInErr = (e && e.message) || String(e)
    return {}
  }
})

// 协作网络图：部门泳道布局，避免同部门节点挤在同一条射线上。
const collabGraph = computed(() => {
  try {
    return computeCollabGraph()
  } catch (e) {
    window.__graphErr = (e && e.message) || String(e) // 诊断
    return { nodes: [], links: [], deptLabels: [], viewBox: '0 0 760 520', cx: 380, cy: 270, ring: 150, hub: '—' }
  }
})
function computeCollabGraph() {
  const list = agents.value
  if (!list.length) return { nodes: [], links: [], deptLabels: [], viewBox: '0 0 760 520', cx: 380, cy: 270, ring: 150, hub: '—' }
  // 引用关系：出（我引用谁）+ 入（谁引用我）
  const outRefs = {}
  const inRefs = {}
  for (const a of list) {
    const rs = (a.collabRefs || []).map(r => r.agent).filter(Boolean)
    if (rs.length) outRefs[a.name] = rs
    for (const r of rs) inRefs[r] = (inRefs[r] || 0) + 1
  }
  const active = new Set([...Object.keys(outRefs), ...Object.keys(inRefs)])
  const byName = {}
  for (const a of list) byName[a.name] = a
  const nodes = [...active].map(n => byName[n]).filter(Boolean)
  if (!nodes.length) return { nodes: [], links: [], deptLabels: [], viewBox: '0 0 760 520', cx: 380, cy: 270, ring: 150, hub: '—' }

  const laneX = { researcher: 78, writer: 198, designer: 318, coder: 442, promoter: 562, publisher: 682 }
  const laneOrder = ['researcher', 'writer', 'designer', 'coder', 'promoter', 'publisher']
  const colors = { writer: '#f59e0b', researcher: '#8b5cf6', coder: '#2563eb', designer: '#ec4899', publisher: '#ef4444', promoter: '#14b8a6', ceo: '#111827' }
  const CX = 380, CY = 260
  // 协作枢纽 = 被引用最多
  let hub = '', hubMax = 0
  for (const [n, c] of Object.entries(inRefs)) { if (c > hubMax) { hubMax = c; hub = n } }

  const perDept = {}
  for (const n of nodes) {
    const role = laneX[n.role] ? n.role : 'writer'
    if (!perDept[role]) perDept[role] = []
    perDept[role].push(n)
  }
  for (const role of Object.keys(perDept)) {
    perDept[role].sort((a, b) => (inRefs[b.name] || 0) - (inRefs[a.name] || 0) || a.name.localeCompare(b.name))
  }
  const idx = {}
  const nodeList = nodes.map(n => {
    const role = laneX[n.role] ? n.role : 'writer'
    const i = idx[role] = (idx[role] || 0) + 1
    const total = perDept[role]?.length || 1
    const isHub = n.name === hub
    const r = isHub ? 23 : Math.max(10, 10 + Math.min(inRefs[n.name] || 0, 5) * 1.6)
    const x = laneX[role]
    const y = total === 1 ? CY : 105 + (i - 1) * (330 / Math.max(total - 1, 1))
    return {
      name: n.name,
      short: String(n.name.split('-')[1] || n.name),
      role, x, y, r, hub: isHub,
      fill: isHub ? colors[role] : '#ffffff',
      stroke: colors[role],
      title: `${n.name}（${(deptMeta[role] || {}).name || role}）\n引用: ${(outRefs[n.name] || []).join('、') || '无'}\n被引用: ${inRefs[n.name] || 0} 次`,
    }
  })
  // 连线（同对合并加权）
  const linkMap = {}
  for (const [src, dsts] of Object.entries(outRefs)) {
    for (const dst of dsts) {
      const key = src + '|' + dst
      linkMap[key] = (linkMap[key] || 0) + 1
    }
  }
  const pos = {}
  for (const n of nodeList) pos[n.name] = n
  const links = []
  for (const [key, cnt] of Object.entries(linkMap)) {
    const [src, dst] = key.split('|')
    const s = pos[src], t = pos[dst]
    if (!s || !t) continue
    const pulseIndex = links.length
    links.push({
      key, source: src, target: dst,
      // 部门决定色相，接力次数只改变同一色相的明暗。
      w: 1.8,
      color: colors[s.role] || colors.writer,
      intensity: 0.55 + Math.min(cnt, 4) * 0.1,
      pulseDelay: `${-(pulseIndex % 7) * .19}s`,
      pulseDuration: `${1.15 + (pulseIndex % 4) * .18}s`,
    })
  }
  // 实时接力事件层：把 relays 的 from/to 接入 nodes 标记「正在接力」+ 叠加为强边
  const relayFrom = new Set()
  const relayTo = new Set()
  const relayStage = new Map()
  for (const r of relays.value) {
    if (r.from) relayFrom.add(r.from)
    if (r.to) relayTo.add(r.to)
    if (r.stage) relayStage.set(`${r.from}->${r.to}`, r.stage)
  }
  // 给 relay 相关节点打实时标记（未在 nodeList 里的接力方也补进节点）
  const relayNames = new Set([...relayFrom, ...relayTo])
  for (const rn of relayNames) {
    if (!byName[rn]) {
      const role = String(rn).split('-')[0]
      const lane = laneX[role] ? role : 'writer'
      const i = (idx[lane] = (idx[lane] || 0) + 1)
      const y = 105 + (i - 1) * 90
      const n = { name: rn, short: String(rn.split('-')[1] || rn), role: lane, x: laneX[lane], y, r: 11, hub: false, fill: colors[lane] || '#fff', stroke: colors[lane] || colors.writer, title: `${rn}（接力方）`, relaying: true, relayStage: relayStage.get(rn) || '' }
      nodeList.push(n)
      pos[rn] = n
      if (!pos[n.name]) pos[n.name] = n
    }
  }
  for (const n of nodeList) {
    if (relayFrom.has(n.name) || relayTo.has(n.name)) {
      n.relaying = true
      n.relayStage = relayStage.get(`${n.name}->`) || n.relayStage || ''
      // 找该 agent 最近的接力 stage
      for (const r of relays.value) {
        if (r.from === n.name || r.to === n.name) { n.relayStage = r.stage || n.relayStage; break }
      }
    }
  }
  // 每条 relay 事件叠加成边（来源端→目标端），并在同一对已有边时加粗
  for (const r of relays.value) {
    const s = pos[r.from], t = pos[r.to]
    if (!s || !t) continue
    const key = r.from + '|' + r.to
    const existing = links.find(lk => lk.key === key)
    if (existing) {
      existing.relay = true
      existing.w = Math.max(existing.w, 3)
      existing.stage = r.stage || existing.stage
    } else {
      links.push({
        key, source: r.from, target: r.to, w: 3, relay: true, stage: r.stage || '',
        color: colors[s.role] || colors.writer, intensity: 0.9, pulseDelay: '0s', pulseDuration: '.9s',
      })
    }
  }
  const deptLabels = laneOrder.map(k => ({ key: k, name: (deptMeta[k] || {}).name || k, x: laneX[k], y: 38 }))
  return { nodes: nodeList, links, deptLabels, viewBox: '0 0 760 500', cx: CX, cy: CY, ring: 0, hub: hub || '—' }
}

// —— 协作图交互：拖节点、平移、缩放，并在轮询刷新时保留用户布局 ——
const graphState = ref({ nodes: [], links: [], deptLabels: [], viewBox: '0 0 760 520', cx: 380, cy: 270, ring: 150, hub: '—' })
const relays = ref([]) // 实时接力事件流（后端 /api/company/relays）
const graphSvg = ref(null)
const graphMode = ref('focus')
const graphZoom = ref(1)
const graphPan = reactive({ x: 0, y: 0 })
const savedGraphPositions = new Map()
const draggingNodeName = ref('')
const graphGesture = ref(null)
let suppressNodeClick = false

watch(collabGraph, g => {  const previous = new Map(graphState.value.nodes.map(n => [n.name, { x: n.x, y: n.y }]))
  graphState.value = {
    ...g,
    nodes: g.nodes.map(n => ({ ...n, ...(savedGraphPositions.get(n.name) || previous.get(n.name) || {}) })),
  }
}, { immediate: true })

const graphTransform = computed(() => `translate(${graphPan.x} ${graphPan.y}) scale(${graphZoom.value})`)
const liveGraphLinks = computed(() => {
  const positions = new Map(graphState.value.nodes.map(n => [n.name, n]))
  return graphState.value.links.map(lk => {
    const s = positions.get(lk.source), t = positions.get(lk.target)
    if (!s || !t) return { ...lk, x1: 0, y1: 0, x2: 0, y2: 0 }
    const dx = t.x - s.x, dy = t.y - s.y
    const dist = Math.sqrt(dx * dx + dy * dy) || 1
    return {
      ...lk,
      x1: s.x + dx / dist * (s.r + 3), y1: s.y + dy / dist * (s.r + 3),
      x2: t.x - dx / dist * (t.r + 5), y2: t.y - dy / dist * (t.r + 5),
    }
  })
})
const selectedNode = ref(null) // 点击选中的 agent 详情
const displayGraphLinks = computed(() => {
  if (graphMode.value === 'all') return liveGraphLinks.value
  const focus = selectedNode.value?.name || graphState.value.hub
  const degree = new Map()
  for (const link of graphState.value.links) {
    degree.set(link.source, (degree.get(link.source) || 0) + 1)
    degree.set(link.target, (degree.get(link.target) || 0) + 1)
  }
  return liveGraphLinks.value
    .filter(link => link.source === focus || link.target === focus)
    .sort((a, b) => {
      const aOther = a.source === focus ? a.target : a.source
      const bOther = b.source === focus ? b.target : b.source
      return (degree.get(bOther) || 0) - (degree.get(aOther) || 0) || b.w - a.w
    })
    .slice(0, 10)
})
const displayGraphNodes = computed(() => {
  if (graphMode.value === 'all') return graphState.value.nodes
  const names = new Set([selectedNode.value?.name || graphState.value.hub])
  for (const link of displayGraphLinks.value) { names.add(link.source); names.add(link.target) }
  return graphState.value.nodes.filter(node => names.has(node.name))
})

function setGraphMode(mode) {
  graphMode.value = mode
  requestAnimationFrame(() => fitGraph())
}

function svgPoint(ev) {
  const svg = graphSvg.value
  if (!svg) return { x: 0, y: 0 }
  const pt = svg.createSVGPoint()
  pt.x = ev.clientX
  pt.y = ev.clientY
  return pt.matrixTransform(svg.getScreenCTM().inverse())
}
function graphPoint(ev) {
  const sp = svgPoint(ev)
  return { x: (sp.x - graphPan.x) / graphZoom.value, y: (sp.y - graphPan.y) / graphZoom.value }
}
function onNodePointerDown(ev, n) {
  ev.currentTarget.setPointerCapture?.(ev.pointerId)
  const p = graphPoint(ev)
  graphGesture.value = { type: 'node', pointerId: ev.pointerId, node: n, offsetX: n.x - p.x, offsetY: n.y - p.y, startX: ev.clientX, startY: ev.clientY }
  draggingNodeName.value = n.name
  suppressNodeClick = false
}
function onCanvasPointerDown(ev) {
  if (ev.target !== ev.currentTarget) return
  ev.currentTarget.setPointerCapture?.(ev.pointerId)
  const sp = svgPoint(ev)
  graphGesture.value = { type: 'pan', pointerId: ev.pointerId, startX: sp.x, startY: sp.y, panX: graphPan.x, panY: graphPan.y }
}
function onGraphPointerMove(ev) {
  const gesture = graphGesture.value
  if (!gesture || gesture.pointerId !== ev.pointerId) return
  if (gesture.type === 'node') {
    const p = graphPoint(ev)
    gesture.node.x = p.x + gesture.offsetX
    gesture.node.y = p.y + gesture.offsetY
    savedGraphPositions.set(gesture.node.name, { x: gesture.node.x, y: gesture.node.y })
    if (Math.hypot(ev.clientX - gesture.startX, ev.clientY - gesture.startY) > 4) suppressNodeClick = true
    return
  }
  const sp = svgPoint(ev)
  graphPan.x = gesture.panX + sp.x - gesture.startX
  graphPan.y = gesture.panY + sp.y - gesture.startY
}
function onGraphPointerUp(ev) {
  if (!graphGesture.value || graphGesture.value.pointerId !== ev.pointerId) return
  graphGesture.value = null
  draggingNodeName.value = ''
}
function setGraphZoom(next, anchor = { x: 380, y: 260 }) {
  const old = graphZoom.value
  const value = Math.min(2.5, Math.max(0.48, next))
  const worldX = (anchor.x - graphPan.x) / old
  const worldY = (anchor.y - graphPan.y) / old
  graphPan.x = anchor.x - worldX * value
  graphPan.y = anchor.y - worldY * value
  graphZoom.value = value
}
function zoomGraph(factor) { setGraphZoom(graphZoom.value * factor) }
function onGraphWheel(ev) { setGraphZoom(graphZoom.value * (ev.deltaY < 0 ? 1.1 : 0.9), svgPoint(ev)) }
function fitGraph() {
  const nodes = displayGraphNodes.value
  if (!nodes.length) return
  const xs = nodes.map(n => n.x), ys = nodes.map(n => n.y)
  const minX = Math.min(...xs) - 54, maxX = Math.max(...xs) + 54
  const minY = Math.min(...ys) - 54, maxY = Math.max(...ys) + 54
  const scale = Math.min(2.2, Math.max(0.48, Math.min(720 / (maxX - minX), 480 / (maxY - minY))))
  graphZoom.value = scale
  graphPan.x = 380 - ((minX + maxX) / 2) * scale
  graphPan.y = 260 - ((minY + maxY) / 2) * scale
}
function resetGraph() {
  savedGraphPositions.clear()
  graphState.value = computeCollabGraph()
  graphZoom.value = 1
  graphPan.x = 0
  graphPan.y = 0
  selectedNode.value = null
}
function selectNode(n) {
  if (suppressNodeClick) { suppressNodeClick = false; return }
  selectedNode.value = byNameOf(n.name) || null
}
// 手图「按部门高亮」：当前接力部门 = relays 最近一条 from/to 所在的部门，其余部门整体降噪。
const activeDepts = computed(() => {
  const r = [...relays.value].sort((a, b) => String(b.doneAt || '').localeCompare(String(a.doneAt || '')))[0]
  const set = new Set()
  if (r && r.from) set.add(String(r.from).split('-')[0])
  if (r && r.to) set.add(String(r.to).split('-')[0])
  return set
})
function linkOpacity(lk) {
  const name = selectedNode.value?.name
  if (!name && activeDepts.value.size) {
    // 未手动选节点时：当前接力部门相关的线高亮，其余部门降噪（不是全亮）。
    const sDept = String(lk.source || '').split('-')[0]
    const tDept = String(lk.target || '').split('-')[0]
    return (activeDepts.value.has(sDept) || activeDepts.value.has(tDept)) ? 0.9 : 0.06
  }
  return !name ? 0.48 : (lk.source === name || lk.target === name ? 0.92 : 0.08)
}
function byNameOf(name) {
  return agents.value.find(a => a.name === name) || null
}
const selectedNodeOut = computed(() => (selectedNode.value?.collabRefs || []).map(r => r.agent).join('、') || '无')
const selectedNodeIn = computed(() => {
  const m = collabIn.value[selectedNode.value?.name]
  return m ? `${m.agents.join('、')}（${m.count} 人）` : '无'
})

function statusLabel(status) {
  return { active: '执行中', awaiting_approval: '等待终审', completed: '已完成', failed: '已停止' }[status] || status
}

function taskStatusLabel(status) {
  return { blocked: '等待上游', ready: '可领取', running: '执行中', rework: '返工中', approved: '已通过', waiting_human: '等你审批', failed: '失败' }[status] || status
}

function roleLabel(role) {
  return { researcher: '研究部', writer: '作者部', promoter: '宣传部', editor: '编辑部', manager: '管理部', coder: '程序部', designer: '设计部', publisher: '发布部' }[role] || role
}

function roleIcon(role) {
  return { writer: 'ph:pen-nib-bold', researcher: 'mdi:microscope', coder: 'mdi:code-tags', designer: 'mdi:palette', publisher: 'mdi:bullhorn', promoter: 'mdi:megaphone' }[role] || 'mdi:robot'
}

function isBusy(agent) {
  if (agent.status === '工作中') return true
  const log = agent.recentLog || ''
  return /🧠|✍️|🔬|💻|🎨|📡|📣|⚙️|调研|学习|写|精读|项目|任务|宣传/.test(log) && !/失败|未完成|未成功|熔断|429/.test(log)
}

function isDead(agent) {
  return (agent.status || '').startsWith('停摆')
}

function statusClass(agent) {
  if (agent.status === '工作中') return 'st-working'
  if (agent.status === '空闲中') return 'st-idle'
  if (isDead(agent)) return 'st-dead'
  return 'st-unknown'
}

function statusText(agent) {
  if (agent.status === '工作中') return '🟢 工作中'
  if (agent.status === '空闲中') return '🟡 空闲中'
  if (isDead(agent)) return '🔴 ' + agent.status
  return '⚪ 未知'
}

function doingText(agent) {
  const log = agent.recentLog || ''
  const lines = log.split('\n').filter(Boolean)
  const last = lines[lines.length - 1] || ''
  if (/失败|熔断|429|未成功|未完成|限流/.test(last)) return '⚡ 充电中…'
  return last.replace(/^\[[^\]]*\]\s*/, '').replace(/·[^·]*$/, '').trim() || '待命中'
}

function fileType(f) {
  if (f.startsWith('学习')) return '📖 学习'
  if (f.startsWith('调研')) return '🔍 调研'
  if (f.startsWith('文章')) return '✍️ 文章'
  if (f.startsWith('任务')) return '⚙️ 任务'
  if (f.startsWith('今日目标')) return '🎯 目标'
  if (f.startsWith('计划')) return '📋 计划'
  if (f.startsWith('需求')) return '📐 需求'
  if (f.startsWith('设计')) return '🎨 设计'
  if (/\.(xlsx|xls|csv|tsv)$/i.test(f) || f.startsWith('Excel')) return '📊 Excel'
  if (f.startsWith('PPT')) return '📽️ PPT'
  return '📄'
}

function calcTools(agent) {
  const log = agent.recentLog || ''
  const tools = (log.match(/🧠|⚙️|🔬|💻|📡|🎨/g) || []).length
  return Math.min(tools, 99)
}

async function previewFile(agent, f, cacheKey = '') {
  try {
    const agentName = agent.name || agent
    const fileName = String(f || '')
    const versionQuery = cacheKey ? '&v=' + encodeURIComponent(cacheKey) : ''
    const query = 'agent=' + encodeURIComponent(agentName) + '&name=' + encodeURIComponent(fileName) + versionQuery
    artifactModal.open = true
    artifactModal.loading = true
    artifactModal.content = ''
    artifactModal.agent = agentName
    artifactModal.file = fileName.split('/').pop()
    artifactModal.kind = ''
    artifactModal.mime = ''
    artifactModal.truncated = false
    artifactModal.rawUrl = '/api/company/file?' + query + '&raw=1'
    pptDeck.slides = []
    spreadsheetRows.value = []
    slideIndex.value = 0
    const r = await fetch('/api/company/file?' + query)
    const d = await r.json()
    if (!r.ok) throw new Error(d.error || '读取失败')
    artifactModal.kind = d.kind || 'text'
    artifactModal.mime = d.mime || ''
    artifactModal.content = d.content || ''
    if (artifactModal.kind === 'pptx') {
      const binary = await fetch(artifactModal.rawUrl)
      if (!binary.ok) throw new Error('PPTX 文件读取失败')
      const parsed = await parsePptxPreview(await binary.arrayBuffer())
      pptDeck.size = parsed.size
      pptDeck.slides = parsed.slides
    } else if (artifactModal.kind === 'spreadsheet') {
      const binary = await fetch(artifactModal.rawUrl)
      if (!binary.ok) throw new Error('电子表格读取失败')
      const lower = artifactModal.file.toLowerCase()
      const parsed = lower.endsWith('.xlsx')
        ? await parseXlsxPreview(await binary.arrayBuffer())
        : parseDelimitedPreview(await binary.text(), lower.endsWith('.tsv') ? '\t' : ',')
      spreadsheetRows.value = parsed.rows
      artifactModal.truncated = parsed.truncated
    }
  } catch (e) {
    artifactModal.content = e?.message || '读取失败'
    if (!artifactModal.kind) artifactModal.kind = 'text'
  } finally {
    artifactModal.loading = false
  }
}

function stageLabel(key) {
  return projectStages.find(stage => stage.key === key)?.label || '过程'
}
function stageTitle(project, stage) {
  const evidence = project.stageEvidence[stage.key] || []
  return evidence.length ? `${stage.label}：${evidence.map(item => `${item.agent} / ${item.name}`).join('；')}` : `${stage.label}：缺失`
}
function previewProjectArtifact(project, artifact) {
  if (!artifact?.agent || !artifact?.path || artifact.path.split('/').length < 3) return
  previewFile({ name: artifact.agent }, artifact.path, artifact.sha256 || '')
}
function artifactRawUrl(artifact) {
  if (!artifact?.agent || !artifact?.path) return ''
  const versionQuery = artifact.sha256 ? '&v=' + encodeURIComponent(artifact.sha256) : ''
  return '/api/company/file?agent=' + encodeURIComponent(artifact.agent) + '&name=' + encodeURIComponent(artifact.path) + '&raw=1' + versionQuery
}
function openStageEvidence(project, stage) {
  const evidence = project.stageEvidence[stage] || []
  const artifact = evidence.find(item => item.path?.split('/').length >= 3)
  if (artifact) previewProjectArtifact(project, artifact)
}

// 审批相关
async function loadApprovals() {
  try {
    const d = await api('/api/company/approvals')
    pendingApprovals.value = d.pending || []
  } catch (e) { /* 静默 */ }
}

// 会议室 tab：拉最近会议纪要（真实开会内容）
async function loadMeetings() {
  try {
    const d = await api('/api/company/meetings')
    meetings.value = d.meetings || []
  } catch (e) { /* 静默 */ }
}

function meetingFileUrl(meeting, file) {
  if (!meeting?.agent || !file) return ''
  return '/api/company/file?agent=' + encodeURIComponent(meeting.agent) + '&name=' + encodeURIComponent(file) + '&raw=1&v=' + encodeURIComponent(meeting.id || meeting.time || '')
}

async function loadIntegrations() {
  try {
    integrations.value = await api('/api/company/integrations')
  } catch (e) { /* 后端未就绪时保持待连接状态 */ }
}

async function loadProductionAudit() {
  try {
    productionAudit.value = await api('/api/company/production-audit')
    if (!activeDept.value && productionAudit.value.departments?.length) activeDept.value = productionAudit.value.departments[0].role
  } catch (e) { /* 审计失败即保持全红，不伪造通过状态 */ }
}

async function approve(item, decision) {
  try {
    const r = await fetch('/api/company/approve', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ agent: item.agent, file: item.file, decision })
    })
    const d = await r.json()
    if (d.ok) {
      pendingApprovals.value = pendingApprovals.value.filter(x => !(x.agent === item.agent && x.file === item.file))
    }
  } catch (e) { /* 静默 */ }
}

async function approveProject(project, decision) {
  if (!project?.items?.length || decidingProject.value) return
  decidingProject.value = project.key
  hiddenApprovalProjects.value = new Set([...hiddenApprovalProjects.value, project.key])
  error.value = ''
  try {
    await Promise.all(project.items.map(item => api('/api/company/approve', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ agent: item.agent, file: item.file, project: project.key, decision })
    })))
    const decided = new Set(project.items.map(item => item.agent + '|' + item.file))
    pendingApprovals.value = pendingApprovals.value.filter(item => !decided.has(item.agent + '|' + item.file))
  } catch (err) {
    const visible = new Set(hiddenApprovalProjects.value)
    visible.delete(project.key)
    hiddenApprovalProjects.value = visible
    error.value = `项目审批失败：${err.message}`
  } finally {
    decidingProject.value = ''
  }
}

// 点击文件名预览（审批专用，兼容 previewFile）
function previewFile2(agent, file) {
  previewFile({ name: agent }, file)
}

// 工作流协作链：研究部→作者部→设计部→程序部→宣传部→发布部
const workflowChain = ['researcher', 'writer', 'designer', 'coder', 'promoter', 'publisher']
function interactWith(agent) {
  const idx = workflowChain.indexOf(agent.role)
  if (idx < 0) return '部门成员'
  // 同部门：找另一个 agent
  const same = departments.value?.find(d => d.key === agent.role)
  if (same && same.agents.length > 1) {
    const other = same.agents.find(a => a.name !== agent.name)
    if (other) return other.name
  }
  // 下一个部门：上游→下游
  if (idx < workflowChain.length - 1) {
    const nextDept = departments.value?.find(d => d.key === workflowChain[idx + 1])
    if (nextDept && nextDept.agents.length) return nextDept.agents[0].name + ' 等'
  }
  // 上一个部门：上游输入
  if (idx > 0) {
    const prevDept = departments.value?.find(d => d.key === workflowChain[idx - 1])
    if (prevDept && prevDept.agents.length) return prevDept.agents[0].name + ' 等'
  }
  return '部门成员'
}

// 简单 Markdown 渲染器（支持标题/加粗/列表/代码块/引用）
function renderMarkdown(text) {
  if (!text) return ''
  return text
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/^### (.+)$/gm, '<h3>$1</h3>')
    .replace(/^## (.+)$/gm, '<h2>$1</h2>')
    .replace(/^# (.+)$/gm, '<h1>$1</h1>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/^\- (.+)$/gm, '<li>$1</li>')
    .replace(/(<li>.*<\/li>\n?)+/g, '<ul>$&</ul>')
    .replace(/^> (.+)$/gm, '<blockquote>$1</blockquote>')
    .replace(/```(\w*)\n?([\s\S]*?)```/g, '<pre class="code-block"><code>$2</code></pre>')
    .replace(/\n\n/g, '</p><p>')
    .replace(/\n/g, '<br>')
    .replace(/^(.+)$/gm, function(m) {
      if (m.startsWith('<')) return m
      return m
    })
    .replace(/<p><\/p>/g, '')
}

function formatTime(value) {
  if (!value) return '刚刚'
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}

function formatRelayTime(value) {
  if (!value) return '刚刚'
  const d = new Date(value)
  if (isNaN(d.getTime())) return '刚刚'
  return new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit' }).format(d)
}

async function api(url, options) {
  // 2026-08-09 修复：fetch 无 timeout 会因后端重启间隙永久挂起 → Promise.all 永不 resolve → agents 不赋值（页面全 0）
  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), 8000)
  try {
    const response = await fetch(url, { ...options, signal: ctrl.signal })
    const text = await response.text()
    let data
    try {
      data = JSON.parse(text)
    } catch {
      throw new Error('公司工作流后端尚未启动或返回了无效响应')
    }
    if (!response.ok) throw new Error(data.error || `请求失败 (${response.status})`)
    return data
  } finally {
    clearTimeout(timer)
  }
}

async function directiveApi(url, options) {
  const bases = API_BASE_URL ? ['', API_BASE_URL] : ['']
  let lastError
  for (const base of bases) {
    const ctrl = new AbortController()
    const timer = setTimeout(() => ctrl.abort(), 8000)
    try {
      const response = await fetch(`${base}${url}`, { ...options, signal: ctrl.signal })
      const text = await response.text()
      let data
      try { data = JSON.parse(text) } catch { throw new Error('指令服务返回了无效响应') }
      if (!response.ok) throw new Error(data.error || `指令服务请求失败 (${response.status})`)
      return data
    } catch (err) {
      lastError = err?.name === 'AbortError' ? new Error('指令服务连接超时') : err
    } finally {
      clearTimeout(timer)
    }
  }
  throw lastError || new Error('指令服务不可用')
}

async function loadData({ quiet = false } = {}) {
  try {
    const [goalResult, agentResult, osResult, relayResult] = await Promise.allSettled([
      api('/api/company/goals'),
      api('/api/company/agents'),
      api('/api/company/os-stats'),
      api('/api/company/relays')
    ])
    if (goalResult.status === 'fulfilled') goals.value = goalResult.value.goals || []
    if (agentResult.status === 'fulfilled') agents.value = agentResult.value.agents || []
    if (osResult.status === 'fulfilled') osStats.value = osResult.value || osStats.value
    if (relayResult.status === 'fulfilled') relays.value = relayResult.value.relays || []
    if (!quiet && [goalResult, agentResult, osResult].every(result => result.status === 'rejected')) {
      throw goalResult.reason || agentResult.reason || osResult.reason
    }
    if (!activeDept.value && agents.value.length) {
      // 默认选第一个有人的部门
      const roles = [...new Set(agents.value.map(a => a.role))]
      activeDept.value = roles[0] || 'writer'
    }
    if (!selectedGoalId.value && goals.value.length) selectedGoalId.value = goals.value[0].id
    if (selectedGoalId.value && !goals.value.some(goal => goal.id === selectedGoalId.value)) selectedGoalId.value = goals.value[0]?.id || ''
    if (!quiet) error.value = ''
  } catch (err) {
    if (!quiet) error.value = err.message
  } finally {
    loading.value = false
  }
}

async function createGoal() {
  creating.value = true
  error.value = ''
  try {
    const goal = await api('/api/company/goals', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(form)
    })
    selectedGoalId.value = goal.id
    form.objective = ''
    form.successMetric = ''
    await loadData({ quiet: true })
  } catch (err) {
    error.value = err.message
  } finally {
    creating.value = false
  }
}

function selectGoal(id) {
  selectedGoalId.value = id
  feedback.value = ''
}

async function decide(decision) {
  deciding.value = true
  error.value = ''
  try {
    await api(`/api/company/goals/${encodeURIComponent(activeGoal.value.id)}/decision`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ decision, feedback: feedback.value })
    })
    feedback.value = ''
    await loadData({ quiet: true })
  } catch (err) {
    error.value = err.message
  } finally {
    deciding.value = false
  }
}

async function rerunGoal() {
  try {
    await api(`/api/company/goals/${encodeURIComponent(activeGoal.value.id)}/run`, { method: 'POST' })
    await loadData({ quiet: true })
  } catch (err) {
    error.value = err.message
  }
}

async function openArtifact(artifactId) {
  artifactModal.open = true
  artifactModal.loading = true
  artifactModal.content = ''
  try {
    const data = await api(`/api/company/goals/${encodeURIComponent(activeGoal.value.id)}/artifacts/${encodeURIComponent(artifactId)}`)
    artifactModal.content = data.content
  } catch (err) {
    artifactModal.content = err.message
  } finally {
    artifactModal.loading = false
  }
}

let timer
let polling = false

// ===== 开罗式办公室场景（游戏发展国风格：俯视公司，员工走动+头顶飘字）=====
const officeDepts = [
  { key: 'writer', name: '作者部', icon: '✍️' },
  { key: 'researcher', name: '研究部', icon: '🔬' },
  { key: 'coder', name: '程序部', icon: '💻' },
  { key: 'designer', name: '设计部', icon: '🎨' },
  { key: 'promoter', name: '宣传部', icon: '📣' },
  { key: 'publisher', name: '发布部', icon: '📢' },
]
const officeArea = {
  ceo:        { left: '36%', top: '1.5%', width: '28%', height: '24%' },
  writer:     { left: '1.5%', top: '29%', width: '31%', height: '31%' },
  researcher: { left: '34.5%', top: '29%', width: '31%', height: '31%' },
  coder:      { left: '67.5%', top: '29%', width: '31%', height: '31%' },
  designer:   { left: '1.5%', top: '64%', width: '31%', height: '32%' },
  promoter:   { left: '34.5%', top: '64%', width: '31%', height: '32%' },
  publisher:  { left: '67.5%', top: '64%', width: '31%', height: '32%' },
}
const officeEmoji = { writer: '✍️', researcher: '🔬', coder: '💻', designer: '🎨', promoter: '📣', publisher: '📢', ceo: '👑' }
const officePeople = ref([])
let officeWalkTimer = null

// 公司资金（开罗经济：评分→销量→入账）
const financeBalance = ref(0)
const financeTotalIn = ref(0)
async function loadFinance() {
  try {
    const d = await api('/api/company/finance')
    financeBalance.value = d.balance || 0
    financeTotalIn.value = d.total_in || 0
  } catch (e) { /* 后端没起静默 */ }
}

// 应用大厅（B站式信息流：真实评分排序 + 推广置顶）
const marketGames = ref([])
async function loadMarket() {
  try {
    const d = await api('/api/company/market')
    if (d && Array.isArray(d.games)) marketGames.value = d.games
  } catch (e) { /* 后端没起静默 */ }
}
function awardEmoji(title) {
  if (title.includes('神作')) return '🏆'
  if (title.includes('好评')) return '⭐'
  return '🌱'
}

function rebuildOfficePeople() {
  officePeople.value = agents.value.map(a => {
    const role = a.role === 'ceo' ? 'ceo' : a.role
    const dept = role === 'ceo' ? 'ceo' : (officeDepts.some(d => d.key === role) ? role : 'writer')
    const isDead = (a.status || '').startsWith('停摆')
    return {
      name: a.name,
      dept,
      emoji: officeEmoji[dept] || '🤖',
      short: String(a.name).split('-')[1] || a.name,
      task: (a.task || '待命中').slice(0, 12),
      dot: a.status === '工作中' ? 'dot-green' : (isDead ? 'dot-red' : 'dot-yellow'),
      cls: isDead ? 'op-dead' : (a.status === '工作中' ? 'op-work' : 'op-idle'),
      x: 8 + Math.random() * 78,
      y: 14 + Math.random() * 62,
    }
  })
}
// 走动：工作中/空闲中的员工每 3 秒换一个目标位置（CSS transition 平滑走过去）
function startOfficeWalk() {
  stopOfficeWalk()
  officeWalkTimer = setInterval(() => {
    for (const p of officePeople.value) {
      const a = agents.value.find(x => x.name === p.name)
      if (a && (a.status === '工作中' || a.status === '空闲中')) {
        p.x = 8 + Math.random() * 78
        p.y = 14 + Math.random() * 62
      }
    }
  }, 3000)
}
function stopOfficeWalk() {
  if (officeWalkTimer) { clearInterval(officeWalkTimer); officeWalkTimer = null }
}
function deptPeople(key) { return officePeople.value.filter(p => p.dept === key) }
function deptStyle(key) { return officeArea[key] || {} }
function personStyle(p) {
  return { left: p.x + '%', top: p.y + '%', transition: 'left 2.7s ease-in-out, top 2.7s ease-in-out' }
}

watch(agents, () => { rebuildOfficePeople() }, { deep: true })

onMounted(() => {
  document.title = '杉汐 | 公司目标'
  loadData()
  loadApprovals()
  loadMeetings()
  loadIntegrations()
  loadProductionAudit()
  loadIterate()
  loadDirective()
  loadModelMaxTokens()
  loadReviews()
  loadFinance()
  loadMarket()
  rebuildOfficePeople()
  startOfficeWalk()
  // 2026-08-09 修复：防重入——后端重启间隙请求会挂 8s（api 超时），3s 轮询若叠加上一轮挂起请求会耗尽连接
  timer = setInterval(() => {
    if (polling) return
    polling = true
    Promise.allSettled([loadData({ quiet: true }), loadApprovals(), loadMeetings(), loadIterate(), loadDirective(), loadReviews(), loadFinance(), loadMarket()]).finally(() => { polling = false })
  }, 3000)
})
onUnmounted(() => { clearInterval(timer); stopOfficeWalk() })
</script>

<style scoped>
.company-view {
  --ink: #17201c;
  --muted: #647069;
  --line: #dce3de;
  --soft: #f4f7f4;
  --accent: #176b47;
  --accent-soft: #e7f3ec;
  --warning: #a85d10;
  --danger: #b33a3a;
  max-width: 1060px;
  width: 100%;
  box-sizing: border-box;
  margin: 0 auto;
  padding: 24px 20px 60px;
  color: var(--ink);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', sans-serif;
}

/* 竖标签布局：左 tab 栏 + 右内容区 */
.company-layout { display: flex; align-items: stretch; gap: 16px; margin-top: 20px; }
.section-tabs { display: flex; flex-direction: column; gap: 8px; width: 104px; flex-shrink: 0; }
.section-tab {
  display: flex; align-items: center; gap: 6px; padding: 12px 10px;
  background: #fff; border: 1px solid var(--line); border-radius: 12px;
  font-size: 13px; font-weight: 700; color: #4b5563; cursor: pointer;
  transition: all .15s; text-align: left;
}
.section-tab:hover { border-color: #93c5fd; color: #1d4ed8; }
.section-tab.active { background: #eff6ff; border-color: #2563eb; color: #1d4ed8; box-shadow: 0 2px 8px rgba(37,99,235,.12); }
.st-badge {
  margin-left: auto; min-width: 20px; height: 20px; padding: 0 6px;
  display: inline-flex; align-items: center; justify-content: center;
  background: #e5e7eb; color: #374151; border-radius: 10px; font-size: 11px; font-weight: 700;
}
.section-tab.active .st-badge { background: #2563eb; color: #fff; }
.section-content { flex: 1; width: 100%; min-width: 0; box-sizing: border-box; }

/* 会议产物包：回放、PPT、VTT、逐席证据同屏验收。 */
.meeting-live.muted { background: #f1f5f9; color: #64748b; }
.meeting-live.reconstructed { background: #ecfdf5; color: #047857; }
.meeting-proof-banner { margin: 16px 0; display: flex; align-items: center; gap: 9px; padding: 11px 14px; border: 1px solid #a7f3d0; border-radius: 12px; background: #ecfdf5; color: #065f46; font-size: 12px; }
.meeting-proof-banner strong { margin-left: auto; white-space: nowrap; color: #064e3b; }
.meeting-media-grid { display: grid; grid-template-columns: minmax(0, 1fr) 260px; gap: 14px; }
.meeting-replay { min-height: 300px; overflow: hidden; border-radius: 16px; background: #0f172a; border: 1px solid #1e293b; }
.meeting-replay video { display: block; width: 100%; height: 100%; min-height: 300px; max-height: 470px; object-fit: contain; background: #020617; }
.meeting-media-missing { min-height: 300px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 9px; padding: 28px; color: #94a3b8; text-align: center; }
.meeting-media-missing strong { color: #e2e8f0; font-size: 16px; }
.meeting-media-missing span { max-width: 420px; font-size: 12px; line-height: 1.6; }
.meeting-assets { display: flex; flex-direction: column; gap: 9px; padding: 14px; border: 1px solid var(--line); border-radius: 16px; background: #fff; }
.meeting-assets > p { margin: 0 0 3px; color: #94a3b8; font-size: 10px; font-weight: 800; letter-spacing: .13em; }
.meeting-assets button, .meeting-assets a, .meeting-asset-disabled { width: 100%; min-height: 62px; display: flex; align-items: center; gap: 10px; padding: 11px; border: 1px solid #e2e8f0; border-radius: 11px; background: #f8fafc; color: #334155; text-align: left; text-decoration: none; }
.meeting-assets button, .meeting-assets a { cursor: pointer; }
.meeting-assets button:hover, .meeting-assets a:hover { border-color: #60a5fa; background: #eff6ff; }
.meeting-assets span { display: flex; flex-direction: column; min-width: 0; }
.meeting-assets strong { font-size: 13px; }
.meeting-assets small { margin-top: 3px; color: #94a3b8; font-size: 10px; overflow: hidden; text-overflow: ellipsis; }
.meeting-asset-disabled { opacity: .58; }
.meeting-speeches { margin-top: 16px; overflow: hidden; border: 1px solid var(--line); border-radius: 16px; background: #fff; }
.meeting-speeches-head { display: flex; justify-content: space-between; padding: 13px 16px; background: #f8fafc; border-bottom: 1px solid var(--line); }
.meeting-speeches-head span { font-size: 13px; font-weight: 800; color: #17201c; }
.meeting-speeches-head small { color: #64748b; }
.meeting-speech { display: grid; grid-template-columns: 45px minmax(0, 1fr); padding: 15px 16px; border-bottom: 1px solid #eef2f7; }
.meeting-speech:last-child { border-bottom: none; }
.speech-order { padding-top: 2px; color: #10b981; font-size: 12px; font-weight: 900; letter-spacing: .08em; }
.speech-copy header { display: flex; align-items: baseline; gap: 8px; }
.speech-copy header strong { color: #0f172a; font-size: 14px; }
.speech-copy header span { color: #64748b; font-size: 11px; }
.speech-copy header time { margin-left: auto; color: #94a3b8; font-size: 10px; }
.speech-copy p { margin: 7px 0 9px; color: #334155; font-size: 13px; line-height: 1.65; }
.speech-copy button { max-width: 100%; display: inline-flex; align-items: center; gap: 5px; padding: 5px 8px; border: 1px solid #dbeafe; border-radius: 7px; background: #eff6ff; color: #2563eb; font-size: 10px; cursor: pointer; overflow: hidden; text-overflow: ellipsis; }
.meeting-minutes { margin-top: 14px; border: 1px solid var(--line); border-radius: 12px; background: #fff; }
.meeting-minutes summary { padding: 12px 14px; color: #334155; font-size: 12px; font-weight: 800; cursor: pointer; }
.meeting-minutes .meeting-note-body { padding: 0 16px 16px; }
.meeting-note-body { font-size: 12.5px; color: #374151; line-height: 1.7; max-height: 360px; overflow: auto; }
.meeting-note-body h1, .meeting-note-body h2, .meeting-note-body h3 { color: #17201c; margin: 10px 0 5px; }
.meeting-note-body h1, .meeting-note-body h2 { font-size: 14px; }
.meeting-note-body h3 { font-size: 13px; }
.meeting-note-body li { margin-left: 16px; }
.meeting-history { margin-top: 14px; display: flex; flex-direction: column; gap: 7px; }
.meeting-history > span { color: #64748b; font-size: 11px; font-weight: 800; }
.meeting-history-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 10px 12px; border: 1px solid var(--line); border-radius: 10px; background: #fff; }
.meeting-history-row div { min-width: 0; display: flex; flex-direction: column; }
.meeting-history-row strong { overflow: hidden; color: #334155; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.meeting-history-row small { margin-top: 2px; color: #94a3b8; font-size: 10px; }
.meeting-history-row button { border: none; background: transparent; color: #2563eb; font-size: 11px; cursor: pointer; white-space: nowrap; }
.meeting-empty { margin-top: 18px; padding: 24px; text-align: center; color: #94a3b8; font-size: 13px; background: #fff; border: 1px dashed var(--line); border-radius: 14px; }

@media (max-width: 900px) {
  .meeting-media-grid { grid-template-columns: 1fr; }
  .meeting-assets { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .meeting-assets > p { grid-column: 1 / -1; }
  .meeting-proof-banner { align-items: flex-start; flex-wrap: wrap; }
  .meeting-proof-banner strong { margin-left: 0; }
}

/* 协作网络图（2026-08-09：真实接力可视化） */
.collab-map { margin-bottom: 18px; background: #fff; border: 1px solid var(--line); border-radius: 16px; padding: 14px 16px 12px; box-shadow: 0 2px 10px rgba(15,23,42,.04); }
.collab-map-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; margin-bottom: 8px; }
.cm-title { display: inline-flex; align-items: center; gap: 6px; font-size: 13px; font-weight: 800; color: #17201c; }
.cm-stats { font-size: 12px; color: #64748b; }
.cm-stats strong { color: #1d4ed8; }

/* ===== 标签系统（调研方向 + 热门标签）===== */
.tags-room { display: flex; flex-direction: column; gap: 18px; }
.tags-input-row { display: flex; gap: 10px; }
/* 迭代区（预设：已审批项目 → 每天前沿技术调研迭代） */
.iterate-zone { margin-top: 8px; padding-top: 22px; border-top: 1px solid #e2e8f0; }
.iterate-title { margin: 0 0 4px; font-size: 18px; color: #1e293b; }
.iterate-desc { margin: 0 0 16px; color: #64748b; font-size: 13px; }
.iterate-plans, .iterate-candidates { margin-bottom: 18px; }
.iterate-plans h4, .iterate-candidates h4 { margin: 0 0 10px; font-size: 13px; color: #64748b; }
.iterate-plan-card { padding: 14px 16px; border: 1px solid #dfe6f0; border-radius: 12px; background: #fff; margin-bottom: 10px; }
.iterate-plan-head { display: flex; align-items: center; justify-content: space-between; }
.iterate-plan-head strong { font-size: 15px; color: #1e293b; }
.iterate-badge { padding: 3px 8px; border-radius: 6px; background: #eef2ff; color: #4338ca; font-size: 10px; font-weight: 800; letter-spacing: .06em; }
.iterate-summary { margin: 8px 0 4px; color: #334155; font-size: 13px; line-height: 1.55; }
.iterate-meta { color: #94a3b8; font-size: 11px; }
.iterate-candidate-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 4px; border-bottom: 1px solid #eef2f7; }
.iterate-cand-name { font-weight: 700; color: #1e293b; }
.iterate-start-btn { padding: 7px 16px; border: 0; border-radius: 8px; background: #6366f1; color: #fff; font-size: 13px; font-weight: 700; cursor: pointer; transition: opacity .15s; flex: 0 0 auto; }
.iterate-start-btn:hover:not(:disabled) { opacity: .85; }
.iterate-start-btn:disabled { opacity: .5; cursor: default; }
.iterate-stop-btn { padding: 5px 12px; border: 1px solid #fecaca; border-radius: 8px; background: #fef2f2; color: #dc2626; font-size: 12px; font-weight: 700; cursor: pointer; transition: all .15s; flex: 0 0 auto; }
.iterate-stop-btn:hover { background: #dc2626; border-color: #dc2626; color: #fff; }
.iterate-empty { display: flex; align-items: center; gap: 12px; padding: 20px; border: 1px dashed #dbe3ef; border-radius: 12px; color: #94a3b8; font-size: 13px; }
.tags-input { flex: 1; min-width: 0; height: 42px; padding: 0 14px; border: 1.5px solid #dfe4df; border-radius: 12px; font-size: 14px; color: #17201c; background: #fff; outline: none; }
.tags-input:focus { border-color: #4f7cff; }
.tags-add-btn { height: 42px; padding: 0 18px; border: none; border-radius: 12px; background: #4f7cff; color: #fff; font-size: 14px; font-weight: 600; cursor: pointer; }
.tags-add-btn:disabled { opacity: 0.45; cursor: not-allowed; }
.tags-hot h3, .tags-list h3 { margin: 0 0 10px; font-size: 13px; font-weight: 800; color: #17201c; }
.tags-shell { display: flex; flex-wrap: wrap; gap: 8px; }
.tag-pill { display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px; border-radius: 999px; background: #eef4ff; color: #2f54a0; font-size: 13px; font-weight: 600; cursor: pointer; border: 1px solid #d6e4ff; }
.tag-pill:hover { background: #e0ecff; }
.tag-pill.hot { background: #fff7e6; border-color: #ffd591; color: #ad6800; }
.tag-pill small { font-size: 11px; opacity: 0.7; font-weight: 700; }
.tag-del { border: none; background: transparent; color: #94a3b8; font-size: 14px; cursor: pointer; padding: 0 2px; line-height: 1; }
.tag-del:hover { color: #d94834; }
.tags-empty { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 36px 0; color: #94a3b8; font-size: 13px; }
.collab-svg { width: 100%; height: auto; display: block; }
.cm-dept-label { font-size: 12px; font-weight: 800; fill: #94a3b8; }
.cm-node { cursor: pointer; }
.cm-node circle { transition: r .15s, filter .15s; }
.cm-node:hover circle { filter: brightness(1.1); }
.cm-node.hub circle { filter: drop-shadow(0 0 8px rgba(236,72,153,.55)); }
.cm-node.sel circle { stroke-width: 4; filter: drop-shadow(0 0 10px rgba(37,99,235,.45)); }
.cm-node.relay circle { filter: drop-shadow(0 0 12px rgba(245,158,11,.65)); animation: cmRelayPulse 1.2s ease-in-out infinite; }
.cm-node-relay { font-size: 9px; font-weight: 800; fill: #f59e0b; text-transform: uppercase; letter-spacing: .06em; }
.cm-link-relay-pulse { animation: cmRelayDash 1.1s linear infinite; stroke-dasharray: 6 4; }
@keyframes cmRelayPulse { 0%,100% { filter: drop-shadow(0 0 6px rgba(245,158,11,.4)); } 50% { filter: drop-shadow(0 0 14px rgba(245,158,11,.85)); } }
@keyframes cmRelayDash { to { stroke-dashoffset: -10; } }
.cm-node-id { font-size: 10px; font-weight: 800; }
.cm-node-name { font-size: 10px; fill: #475569; }
/* 点击节点详情卡 */
.cm-detail { margin-top: 10px; padding: 12px 14px; background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 12px; }
.cm-detail-head { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.cm-detail-role { display: inline-flex; align-items: center; gap: 4px; font-size: 11px; color: #2563eb; font-weight: 700; }
.cm-detail-head strong { font-size: 15px; color: #17201c; }
.cm-detail-close { margin-left: auto; border: none; background: #e2e8f0; color: #475569; width: 22px; height: 22px; border-radius: 6px; cursor: pointer; font-size: 14px; line-height: 1; }
.cm-detail-close:hover { background: #cbd5e1; }
.cm-detail-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 8px; }
.cm-detail-grid > div { display: flex; flex-direction: column; gap: 2px; }
.cm-detail-lbl { font-size: 10px; color: #94a3b8; font-weight: 700; }
.cm-detail-val { font-size: 12px; color: #1e293b; line-height: 1.5; }
.cm-detail-files { display: flex; flex-wrap: wrap; gap: 4px; }
.collab-legend { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-top: 8px; padding-top: 10px; border-top: 1px dashed var(--line); font-size: 11px; color: #64748b; }
.lg-dot { display: inline-block; width: 9px; height: 9px; border-radius: 50%; margin-right: 4px; vertical-align: middle; }
.lg-hint { margin-left: auto; color: #94a3b8; }

.hero { display: flex; align-items: flex-start; justify-content: space-between; gap: 48px; margin-bottom: 32px; }
.eyebrow, .section-kicker { margin: 0 0 8px; color: var(--accent); font-size: 11px; font-weight: 800; letter-spacing: .14em; }
.hero h1 { max-width: 780px; margin: 0; font-size: clamp(30px, 4vw, 52px); line-height: 1.13; letter-spacing: -.035em; }
.hero-copy { max-width: 720px; margin: 18px 0 0; color: var(--muted); font-size: 16px; line-height: 1.7; }
.hero-status { display: flex; align-items: center; flex: 0 0 auto; gap: 8px; margin-top: 8px; padding: 9px 14px; border: 1px solid var(--line); border-radius: 999px; color: var(--muted); font-size: 13px; font-weight: 700; background: white; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; background: #9ca69f; }
.hero-status.active .status-dot { background: #1d9a64; box-shadow: 0 0 0 5px rgba(29, 154, 100, .12); }

/* 王者归来 Hero 徽章 */
.hero-badge { display: inline-flex; align-items: center; gap: 8px; margin-bottom: 14px; padding: 7px 14px 7px 10px; border: 1px solid #c7d8ce; border-radius: 999px; background: #fff; color: #1a2e26; font-size: 13px; font-weight: 750; box-shadow: 0 3px 10px rgba(20,45,31,.06); }
.hero-badge .badge-dot { width: 6px; height: 6px; border-radius: 50%; background: #1d9a64; box-shadow: 0 0 0 4px rgba(29,154,100,.15); animation: pulse 1.4s infinite; }
@keyframes pulse { 0%,100% { opacity: 1 } 50% { opacity: .35 } }

.panel { border: 1px solid var(--line); border-radius: 16px; background: #fff; box-shadow: 0 10px 35px rgba(20, 45, 31, .045); }
.goal-composer { padding: 28px; }
.section-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; margin-bottom: 22px; }
.section-heading.compact { align-items: center; margin-bottom: 16px; }
.section-heading h2 { margin: 0; font-size: 20px; line-height: 1.25; letter-spacing: -.015em; }
.workflow-badge { padding: 7px 10px; border-radius: 8px; color: var(--accent); background: var(--accent-soft); font-size: 12px; font-weight: 700; }
.goal-form { display: grid; grid-template-columns: 1.5fr 1fr; gap: 16px; }
.goal-form label { display: flex; flex-direction: column; gap: 8px; color: #39433e; font-size: 13px; font-weight: 700; }
.goal-form label:first-child { grid-row: span 2; }
.goal-form small { color: #8a958e; font-weight: 500; }
textarea, input { box-sizing: border-box; width: 100%; min-height: 46px; border: 1px solid #cfd8d2; border-radius: 10px; outline: none; background: #fbfcfb; color: var(--ink); font: inherit; font-size: 15px; line-height: 1.55; padding: 12px 14px; transition: border-color .2s ease, box-shadow .2s ease; }
textarea { resize: vertical; }
textarea:focus, input:focus { border-color: var(--accent); box-shadow: 0 0 0 3px rgba(23, 107, 71, .12); }
.form-actions { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; }
.form-actions p { max-width: 420px; margin: 0; color: var(--muted); font-size: 12px; line-height: 1.5; }
.primary-button, .secondary-button { min-height: 44px; border-radius: 10px; padding: 0 18px; font: inherit; font-size: 14px; font-weight: 750; cursor: pointer; transition: transform .18s ease, background .18s ease, opacity .18s ease; }
.primary-button { border: 1px solid var(--accent); background: var(--accent); color: white; }
.secondary-button { border: 1px solid #bcc8c0; background: white; color: var(--ink); }
.primary-button:hover:not(:disabled), .secondary-button:hover:not(:disabled) { transform: translateY(-1px); }
button:disabled { cursor: not-allowed; opacity: .48; }
.error-message { margin: 16px 0 0; color: var(--danger); font-size: 13px; }

.workspace-grid { display: grid; grid-template-columns: 270px minmax(0, 1fr); gap: 20px; margin-top: 20px; align-items: start; }
.goal-list { position: sticky; top: 20px; padding: 18px; }
.goal-list-item { display: flex; align-items: flex-start; gap: 10px; width: 100%; min-height: 64px; margin-top: 6px; padding: 11px; border: 1px solid transparent; border-radius: 10px; background: transparent; color: inherit; text-align: left; cursor: pointer; transition: background .18s ease, border-color .18s ease; }
.goal-list-item:hover { background: var(--soft); }
.goal-list-item.selected { border-color: #c7d8ce; background: var(--accent-soft); }
.goal-list-status { flex: 0 0 auto; width: 8px; height: 8px; margin-top: 6px; border-radius: 50%; background: #a4ada7; }
.goal-list-status.active { background: #21875b; }
.goal-list-status.awaiting_approval { background: #d48125; }
.goal-list-status.completed { background: #3f7899; }
.goal-list-status.failed { background: var(--danger); }
.goal-list-copy { min-width: 0; }
.goal-list-copy strong { display: -webkit-box; overflow: hidden; font-size: 13px; line-height: 1.45; -webkit-line-clamp: 2; -webkit-box-orient: vertical; }
.goal-list-copy small { display: block; margin-top: 4px; color: var(--muted); font-size: 11px; }

.workflow-panel { padding: 28px; }
.goal-summary { display: flex; align-items: flex-start; justify-content: space-between; gap: 32px; }
.goal-summary h2 { max-width: 680px; margin: 0; font-size: 25px; line-height: 1.28; }
.goal-summary p { margin: 10px 0 0; color: var(--muted); font-size: 14px; line-height: 1.6; }
.goal-progress { flex: 0 0 auto; text-align: right; }
.goal-progress strong { display: block; color: var(--accent); font-size: 30px; line-height: 1; }
.goal-progress span { color: var(--muted); font-size: 11px; }
.progress-track { height: 5px; margin: 24px 0 30px; overflow: hidden; border-radius: 999px; background: #e7ece8; }
.progress-track span { display: block; height: 100%; border-radius: inherit; background: var(--accent); transition: width .4s ease; }

.task-pipeline { display: grid; gap: 16px; }
.task-card { position: relative; padding: 20px; border: 1px solid var(--line); border-radius: 12px; background: #fff; animation: enter .25s ease both; }
.task-card.running, .task-card.rework { border-color: #e3bb8e; background: #fffbf6; }
.task-card.approved { border-color: #bcd8c8; background: #f7fbf8; }
.task-card.waiting_human { border-color: #dfb471; background: #fffaf1; }
.task-card.blocked { opacity: .72; }
.dependency-line { position: absolute; top: -17px; left: 35px; height: 16px; border-left: 2px solid #cad4cd; }
.task-card header { display: flex; align-items: center; gap: 12px; }
.task-index { display: grid; place-items: center; width: 34px; height: 34px; border-radius: 9px; color: var(--accent); background: var(--accent-soft); font-size: 11px; font-weight: 800; }
.task-card header div { min-width: 0; }
.task-card h3 { margin: 0; font-size: 16px; }
.task-card header p { margin: 3px 0 0; color: var(--muted); font-size: 11px; }
.task-status { margin-left: auto; padding: 5px 8px; border-radius: 6px; color: var(--muted); background: var(--soft); font-size: 11px; font-weight: 750; }
.task-card.running .task-status, .task-card.rework .task-status { color: var(--warning); background: #f9ead9; }
.task-card.approved .task-status { color: var(--accent); background: var(--accent-soft); }
.task-objective { margin: 15px 0 12px; color: #3f4b44; font-size: 13px; line-height: 1.6; }
.criteria { display: flex; flex-wrap: wrap; gap: 6px; }
.criteria span { padding: 5px 8px; border: 1px solid #e2e7e3; border-radius: 6px; color: var(--muted); background: #fafbfa; font-size: 11px; }
.task-footer { display: flex; justify-content: space-between; align-items: center; margin-top: 14px; color: #849087; font-size: 11px; }
.text-button { border: 0; padding: 6px 0; background: transparent; color: var(--accent); font: inherit; font-weight: 700; cursor: pointer; }
.task-issue { margin: 12px 0 0; padding: 10px; border-radius: 8px; color: #8a4810; background: #fff1e2; font-size: 12px; line-height: 1.5; }

.approval-gate, .failure-gate { margin-top: 22px; padding: 22px; border: 1px solid #dfb471; border-radius: 12px; background: #fffaf1; }
.approval-gate h3, .failure-gate strong { margin: 0; font-size: 18px; }
.approval-gate p, .failure-gate p { margin: 7px 0 16px; color: var(--muted); font-size: 13px; line-height: 1.55; }
.approval-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 12px; }
.failure-gate { display: flex; align-items: center; justify-content: space-between; gap: 20px; border-color: #e0b1b1; background: #fff7f7; }
.failure-gate p { margin-bottom: 0; }

.event-ledger { margin-top: 30px; padding-top: 24px; border-top: 1px solid var(--line); }
.event-ledger ol { margin: 0; padding: 0; list-style: none; }
.event-ledger li { display: grid; grid-template-columns: 92px 72px 1fr; gap: 12px; padding: 10px 0; border-bottom: 1px solid #edf0ed; font-size: 12px; }
.event-ledger time { color: #879088; }
.event-actor { color: var(--accent); font-weight: 700; }
.event-ledger p { margin: 0; color: #3e4942; }

.empty-state { margin-top: 20px; padding: 54px 24px; text-align: center; }
.empty-state > span { color: var(--accent); font-size: 32px; }
.empty-state h2 { margin: 12px 0 8px; }
.empty-state p { margin: 0; color: var(--muted); }
.legacy-agents { margin-top: 20px; padding: 22px; }
.legacy-agents .section-heading > span { color: var(--muted); font-size: 12px; }
.agent-strip { display: flex; gap: 10px; overflow-x: auto; padding-bottom: 2px; }
.agent-chip { display: flex; align-items: center; flex: 0 0 auto; gap: 10px; min-width: 145px; padding: 10px 12px; border: 1px solid #e3e8e4; border-radius: 10px; background: var(--soft); }
.agent-chip > span { color: var(--accent); font-size: 18px; }
.agent-chip strong, .agent-chip small { display: block; }
.agent-chip strong { font-size: 12px; }
.agent-chip small { margin-top: 2px; color: var(--muted); font-size: 10px; }

.modal-backdrop { position: fixed; z-index: 1000; inset: 0; display: grid; place-items: center; padding: 20px; background: rgba(13, 23, 17, .52); }
.artifact-modal { width: min(880px, 100%); max-height: 86vh; display: flex; flex-direction: column; border-radius: 16px; background: white; box-shadow: 0 24px 80px rgba(0, 0, 0, .24); overflow: hidden; }
.artifact-modal header { display: flex; justify-content: space-between; align-items: center; padding: 16px 24px; border-bottom: 1px solid var(--line); flex-shrink: 0; }
.artifact-modal h2 { margin: 0; font-size: 18px; }
.artifact-modal header button { width: 44px; height: 44px; border: 0; border-radius: 50%; background: var(--soft); color: var(--ink); font-size: 25px; cursor: pointer; }
.artifact-body { flex: 1; overflow: auto; padding: 24px; color: #28332c; font: 15px/1.75 -apple-system,'PingFang SC','Microsoft YaHei',sans-serif; }
.artifact-body h1 { font-size: 22px; margin: 0 0 12px; }
.artifact-body h2 { font-size: 18px; margin: 18px 0 10px; }
.artifact-body h3 { font-size: 16px; margin: 14px 0 8px; }
.artifact-body ul { padding-left: 20px; }
.artifact-body li { margin: 4px 0; }
.artifact-body strong { font-weight: 700; }
.artifact-body blockquote { margin: 12px 0; padding: 8px 14px; border-left: 3px solid #2563eb; background: #f8fafc; color: #475569; }
.artifact-body .code-block { background: #1e293b; color: #e2e8f0; padding: 16px; border-radius: 10px; overflow-x: auto; font-size: 13px; line-height: 1.6; }
.artifact-actions { display: flex; align-items: center; gap: 12px; }
.artifact-actions a { padding: 8px 12px; border-radius: 9px; color: #fff; background: #176b47; font-size: 11px; font-weight: 800; text-decoration: none; }
.artifact-loading, .artifact-empty { display: grid; min-height: 360px; place-items: center; padding: 32px; color: #64748b; text-align: center; }
.media-preview { display: grid; min-height: 420px; place-items: center; overflow: hidden; background: #0b111b; }
.video-preview video { width: 100%; max-height: 70vh; background: #000; }
.app-preview iframe { width: 100%; height: min(68vh, 720px); border: 0; background: #fff; }
.image-preview img { display: block; max-width: 100%; max-height: 70vh; object-fit: contain; }
.pptx-view { padding: 20px; background: #111827; }
.pptx-slide { position: relative; width: 100%; overflow: hidden; background: #fff; box-shadow: 0 18px 48px rgba(0,0,0,.3); }
.pptx-element { position: absolute; }
.pptx-text { overflow: hidden; white-space: pre-wrap; word-break: break-word; overflow-wrap: anywhere; line-height: 1.25; }
.pptx-image { object-fit: contain; }
.outline-warning { display: grid; gap: 8px; margin: 24px; padding: 22px; border: 1px solid #fdba74; border-radius: 12px; color: #9a3412; background: #fff7ed; }
.outline-warning strong { font-size: 17px; }
.outline-warning span { font-size: 13px; }
.spreadsheet-preview { flex: 1; overflow: auto; padding: 18px; background: #f6faf7; }
.spreadsheet-preview table { min-width: 100%; border-collapse: separate; border-spacing: 0; color: #1f2937; background: #fff; box-shadow: 0 8px 24px rgba(15,23,42,.08); }
.spreadsheet-preview th, .spreadsheet-preview td { max-width: 280px; padding: 8px 10px; overflow: hidden; border-right: 1px solid #e5e7eb; border-bottom: 1px solid #e5e7eb; text-overflow: ellipsis; white-space: nowrap; font: 11px/1.4 ui-monospace,Consolas,monospace; }
.spreadsheet-preview th { position: sticky; left: 0; color: #64748b; background: #ecfdf5; }
.spreadsheet-preview tr:first-child td { position: sticky; top: 0; color: #fff; background: #166534; font-weight: 800; }
.spreadsheet-preview > span { display: block; margin-top: 10px; color: #64748b; font-size: 10px; }
.spreadsheet-launch { color: #22c55e; }
.slide-counter { font-size: 13px; color: #64748b; font-weight: 600; }
.slide-view { flex: 1; overflow: auto; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 16px; padding: 24px; }
.slide-card { width: min(740px, 100%); aspect-ratio: 16/9; background: linear-gradient(135deg, #f8fafc 0%, #eff6ff 100%); border: 1px solid #e5e7eb; border-radius: 16px; padding: 40px 48px; box-shadow: 0 8px 30px rgba(0,0,0,.08); display: flex; flex-direction: column; justify-content: center; overflow: auto; }
.slide-card h2 { font-size: 28px; color: #1e293b; margin: 0 0 16px; line-height: 1.3; }
.slide-card ul { margin: 0; padding-left: 24px; }
.slide-card li { font-size: 18px; color: #334155; margin: 10px 0; line-height: 1.5; }
.slide-card strong { color: #2563eb; }
.slide-nav { display: flex; gap: 12px; }
.slide-nav button { padding: 8px 20px; border: 1px solid #c7d8ce; border-radius: 999px; background: #fff; color: #166534; font-weight: 600; cursor: pointer; }
.slide-nav button:disabled { opacity: .4; cursor: not-allowed; }

/* 会议桌动画 */
.meeting-table { position: relative; display: flex; align-items: center; justify-content: center; gap: 16px; padding: 24px 0; flex-wrap: wrap; }
.meeting-ceo { position: relative; display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 18px; }
.ceo-icon { color: #2563eb; filter: drop-shadow(0 0 8px rgba(37,99,235,.3)); animation: ceoPulse 2s ease-in-out infinite; }
@keyframes ceoPulse { 0%,100% { transform: scale(1); } 50% { transform: scale(1.08); } }
.ceo-name { font-size: 13px; font-weight: 700; color: #1e293b; }
.ceo-ring { position: absolute; inset: -4px; border-radius: 50%; border: 2px solid rgba(37,99,235,.15); animation: ringPulse 2.5s ease-out infinite; }
@keyframes ringPulse { 0% { transform: scale(1); opacity: 1; } 100% { transform: scale(1.5); opacity: 0; } }
.meeting-seat { display: flex; flex-direction: column; align-items: center; gap: 4px; padding: 12px; border-radius: 14px; background: #fff; border: 1px solid #e5e7eb; min-width: 70px; animation: fadeUp .4s both; animation-delay: calc(var(--i) * 0.08s); }
.seat-icon { color: #64748b; }
.seat-icon.talking { color: #2563eb; animation: talkPulse 1.5s ease-in-out infinite; }
@keyframes talkPulse { 0%,100% { transform: scale(1); } 50% { transform: scale(1.12); } }
.seat-name { font-size: 11px; color: #334155; font-weight: 600; }
.seat-dot { width: 7px; height: 7px; border-radius: 50%; background: #d1d5db; }
.seat-dot.on { background: #22c55e; animation: pulse 1.2s infinite; }
.meeting-notice { text-align: center; font-size: 13px; color: #64748b; padding: 8px 0 0; }

@keyframes enter { from { opacity: 0; transform: translateY(5px); } to { opacity: 1; transform: translateY(0); } }

@media (max-width: 860px) {
  .hero { display: block; }
  .hero-status { width: fit-content; margin-top: 18px; }
  .goal-form, .workspace-grid { grid-template-columns: 1fr; }
  .goal-form label:first-child { grid-row: auto; }
  .goal-list { position: static; }
  .goal-summary { display: block; }
  .goal-progress { margin-top: 16px; text-align: left; }
}

@media (max-width: 560px) {
  .company-view { box-sizing: border-box; padding: 24px 92px 48px 14px; }
  .hero h1 { font-size: 32px; }
  .goal-composer, .workflow-panel { padding: 20px; }
  .form-actions, .approval-actions, .failure-gate { align-items: stretch; flex-direction: column; }
  .primary-button, .secondary-button { width: 100%; }
  .event-ledger li { grid-template-columns: 74px 54px 1fr; gap: 8px; }
  .task-card header { flex-wrap: wrap; }
  .task-status { margin-left: 46px; }
}

/* 100 人公司仪表盘 */
.os-dashboard { margin: 24px 0; }
.os-dash-live { display: inline-flex; align-items: center; gap: 6px; padding: 7px 14px; border: 1px solid #c7d8ce; border-radius: 999px; font-size: 12px; font-weight: 700; color: #1a2e26; background: #fff; white-space: nowrap; }
.live-pulse { width: 8px; height: 8px; border-radius: 50%; background: #1d9a64; box-shadow: 0 0 0 4px rgba(29,154,100,.15); animation: pulse 1.4s infinite; }
.os-tabs { display: flex; gap: 6px; margin-bottom: 20px; flex-wrap: wrap; }
.os-tab { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border: 1px solid #e5e7eb; border-radius: 999px; background: #fff; color: #374151; font-size: 13px; font-weight: 600; cursor: pointer; transition: all .15s; white-space: nowrap; }
.os-tab:hover { border-color: #c7d8ce; background: #f9fafb; }
.os-tab.active { border-color: #1d9a64; background: #f0fdf4; color: #166534; }
.os-tab-count { font-size: 11px; color: #6b7280; background: #f3f4f6; padding: 0 7px; border-radius: 999px; line-height: 18px; }
.os-tab.active .os-tab-count { background: #dcfce7; color: #16a34a; }
.os-tab-dot { width: 7px; height: 7px; border-radius: 50%; background: #22c55e; animation: pulse 1.2s infinite; }
.os-dept { margin-bottom: 24px; }
.os-dept-head { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.os-dept-ico { font-size: 18px; }
.os-dept-name { font-size: 16px; font-weight: 700; color: #1a1a2e; }
.os-dept-count { font-size: 12px; color: #6b7280; background: #eef2f7; padding: 2px 10px; border-radius: 999px; }
.os-dept-badge { font-size: 11px; color: #16a34a; background: #dcfce7; padding: 2px 10px; border-radius: 999px; }
.os-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 12px; }
.os-card { background: #fff; border: 1px solid #e5e7eb; border-radius: 14px; padding: 14px; transition: all .2s; box-shadow: 0 1px 3px rgba(0,0,0,.04); }
.os-card.busy { border-color: #86efac; box-shadow: 0 0 12px rgba(34,197,94,.1); }
.os-card-head { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.os-role-icon { font-size: 20px; }
.os-card-info { flex: 1; min-width: 0; }
.os-card-info strong { display: block; font-size: 13px; color: #1a1a2e; }
.os-card-role { font-size: 11px; color: #6b7280; }
.os-card-childhood { font-size: 11px; color: #8a958e; background: #f8fafc; border-radius: 8px; padding: 6px 8px; margin-bottom: 8px; line-height: 1.5; min-height: 30px; }
.os-status-dot { width: 9px; height: 9px; border-radius: 50%; background: #d1d5db; flex-shrink: 0; }
.os-status-dot.on { background: #22c55e; animation: pulse 1.2s infinite; }
.os-card-doing { font-size: 12px; color: #4b5563; background: #f9fafb; border: 1px solid #f3f4f6; border-radius: 8px; padding: 6px 8px; margin-bottom: 6px; min-height: 28px; line-height: 1.5; }
.os-card-interact { display: flex; align-items: center; gap: 4px; font-size: 11px; color: #64748b; margin-bottom: 6px; }
.os-card-collab { display: flex; flex-direction: column; gap: 3px; margin-bottom: 6px; }
.os-collab-item { display: flex; align-items: center; gap: 4px; font-size: 11px; color: #047857; background: #ecfdf5; border: 1px solid #d1fae5; border-radius: 6px; padding: 3px 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; cursor: default; }
.os-collab-in { display: flex; align-items: center; gap: 4px; font-size: 11px; color: #1d4ed8; background: #eff6ff; border: 1px solid #bfdbfe; border-radius: 6px; padding: 3px 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; cursor: default; }
.os-card-files { display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 6px; }
.os-file-tag { font-size: 10px; color: #1d4ed8; background: #eff6ff; border: 1px solid #bfdbfe; padding: 1px 7px; border-radius: 6px; cursor: pointer; transition: all .12s; }
.os-file-tag:hover { background: #dbeafe; }
.os-file-more { font-size: 10px; color: #6b7280; }
.os-card-metrics { display: flex; gap: 6px; }
.os-metric { flex: 1; text-align: center; background: #f8fafc; border-radius: 8px; padding: 6px 4px; }
.os-metric-num { display: block; font-size: 20px; font-weight: 800; color: #1a1a2e; line-height: 1.1; }
.os-card.busy .os-metric-num { color: #16a34a; }
.os-metric-lbl { display: block; font-size: 10px; color: #6b7280; margin-top: 1px; }
/* 活动流 */
.os-activity { margin: 24px 0; }
.os-events { display: flex; flex-direction: column; gap: 0; }
.os-event { display: flex; gap: 12px; }
.os-ev-line { display: flex; flex-direction: column; align-items: center; width: 20px; flex-shrink: 0; }
.os-ev-role { font-size: 14px; }
.os-ev-conn { flex: 1; width: 2px; background: #e5e7eb; }
.os-ev-card { flex: 1; background: #fff; border: 1px solid #e5e7eb; border-radius: 12px; padding: 10px 12px; margin-bottom: 10px; box-shadow: 0 1px 3px rgba(0,0,0,.03); }
.os-ev-name { font-weight: 600; font-size: 13px; color: #1a1a2e; }
.os-ev-time { font-size: 11px; color: #9ca3af; float: right; }
.os-ev-card p { margin: 4px 0 0; font-size: 12px; color: #4b5563; line-height: 1.5; }
.os-ev-empty { color: #9ca3af; font-size: 14px; text-align: center; padding: 30px 0; }

/* 2026 多 Agent 作战室：可观测、可接力、可审批 */
.company-view { width: 100%; max-width: 1380px; box-sizing: border-box; padding: 18px 84px 72px 24px; background: #f3f5f2; }
.command-hero { position: relative; overflow: hidden; padding: 24px 30px 0; border: 1px solid #26354a; border-radius: 24px; color: #f8fafc; background: #101827; box-shadow: 0 24px 70px rgba(15,23,42,.18); }
.command-hero::after { content: ''; position: absolute; right: -90px; top: -130px; width: 420px; height: 420px; border: 1px solid rgba(74,222,128,.2); border-radius: 50%; box-shadow: 0 0 0 70px rgba(74,222,128,.025), 0 0 0 140px rgba(74,222,128,.018); pointer-events: none; }
.command-topline { position: relative; z-index: 1; display: flex; justify-content: space-between; align-items: center; padding-bottom: 21px; border-bottom: 1px solid rgba(148,163,184,.2); color: #94a3b8; font-size: 10px; font-weight: 800; letter-spacing: .16em; }
.runtime-label { display: inline-flex; align-items: center; gap: 8px; color: #dbe7df; }
.runtime-label i, .trace-live i, .supervisor-pill i { width: 7px; height: 7px; border-radius: 50%; background: #4ade80; box-shadow: 0 0 0 5px rgba(74,222,128,.12); animation: pulse 1.5s infinite; }
.command-copy { position: relative; z-index: 1; display: grid; grid-template-columns: minmax(0,1fr) 230px; align-items: end; gap: 48px; padding: 42px 0 38px; }
.command-kicker { margin: 0 0 14px; color: #4ade80; font-size: 11px; font-weight: 900; letter-spacing: .22em; }
.command-copy h1 { margin: 0; max-width: 800px; font-size: clamp(38px,5vw,72px); line-height: .98; letter-spacing: -.055em; }
.command-copy h1 span { color: #4ade80; }
.command-subtitle { max-width: 650px; margin: 22px 0 0; color: #9eacbe; font-size: 14px; line-height: 1.7; }
.zero-cost-seal { position: relative; padding: 22px; border: 1px solid rgba(74,222,128,.38); border-radius: 18px; background: rgba(15,23,42,.72); box-shadow: inset 0 0 40px rgba(34,197,94,.05); }
.zero-cost-seal > span { display: block; color: #9eacbe; font-size: 11px; }
.zero-cost-seal strong { display: block; margin: 5px 0; color: #fff; font-size: 48px; line-height: 1; letter-spacing: -.06em; }
.zero-cost-seal strong span { color: #4ade80; font-size: 24px; }
.zero-cost-seal small { color: #4ade80; font-size: 9px; font-weight: 900; letter-spacing: .15em; }
.command-metrics { position: relative; z-index: 1; display: grid; grid-template-columns: repeat(4,1fr); margin: 0 -30px; border-top: 1px solid rgba(148,163,184,.2); }
.integration-strip { position: relative; z-index: 1; display: flex; align-items: center; gap: 18px; margin: 0 -30px; padding: 10px 30px; border-top: 1px solid rgba(148,163,184,.15); color: #94a3b8; background: rgba(3,7,18,.24); font-size: 9px; font-weight: 800; letter-spacing: .04em; }
.integration-strip span { display: inline-flex; align-items: center; gap: 6px; white-space: nowrap; }
.integration-strip i { width: 7px; height: 7px; border-radius: 50%; background: #64748b; }
.integration-strip span.ready { color: #d1fae5; }
.integration-strip span.ready i { background: #4ade80; box-shadow: 0 0 0 4px rgba(74,222,128,.1); }
.integration-title { margin-right: auto; color: #64748b; }
.command-metric { position: relative; padding: 18px 30px 20px; border-right: 1px solid rgba(148,163,184,.18); }
.command-metric:last-child { border-right: 0; }
.command-metric > span { display: block; color: #9eacbe; font-size: 11px; }
.command-metric strong { display: block; margin: 6px 0 2px; color: #f8fafc; font-size: 30px; line-height: 1; }
.command-metric small { color: #64748b; font-size: 8px; font-weight: 800; letter-spacing: .14em; }
.command-metric.hot strong { color: #4ade80; }

.company-layout { display: block; margin-top: 18px; }
.section-tabs { width: auto; flex-direction: row; gap: 5px; padding: 5px; border: 1px solid #dfe4df; border-radius: 14px; background: #fff; box-shadow: 0 8px 24px rgba(15,23,42,.04); }
.section-tab { flex: 1; justify-content: center; min-height: 46px; border: 0; border-radius: 10px; background: transparent; }
.section-tab:hover { border: 0; background: #f1f5f2; color: #101827; }
.section-tab.active { border: 0; color: #fff; background: #101827; box-shadow: 0 5px 16px rgba(15,23,42,.16); }
.section-tab.active .st-badge { color: #102017; background: #4ade80; }
.section-content { width: 100%; min-width: 0; box-sizing: border-box; margin-top: 18px; }
.section-content > .tags-room,
.section-content > .meeting-room,
.section-content > .war-room,
.section-content > .approval-desk { width: 100%; min-width: 0; box-sizing: border-box; }

.war-room { display: flex; flex-direction: column; gap: 18px; }
.orchestration-strip, .graph-panel, .trace-panel, .agent-console { border: 1px solid #dfe4df; border-radius: 18px; background: #fff; box-shadow: 0 10px 30px rgba(15,23,42,.045); }
.orchestration-strip { overflow: hidden; padding: 24px 26px 26px; }
.production-overview { display: flex; align-items: flex-start; justify-content: space-between; gap: 28px; margin-bottom: 26px; }
.production-project { min-width: 0; }
.production-eyebrow { display: inline-flex; align-items: center; gap: 7px; color: #718078; font-size: 9px; font-weight: 900; letter-spacing: .16em; }
.production-eyebrow i { width: 7px; height: 7px; border-radius: 50%; background: #22c55e; box-shadow: 0 0 0 4px rgba(34,197,94,.12); }
.production-project h2 { overflow: hidden; margin: 7px 0 10px; color: #101827; font-size: clamp(20px,2vw,27px); line-height: 1.15; letter-spacing: -.035em; text-overflow: ellipsis; white-space: nowrap; }
.production-tags { display: flex; flex-wrap: wrap; gap: 6px; }
.production-tags span { padding: 5px 9px; border: 1px solid #e1e7e3; border-radius: 999px; color: #5f6e66; background: #f7f9f7; font-size: 9px; font-weight: 800; letter-spacing: .035em; }
.production-score { flex: 0 0 auto; display: grid; grid-template-columns: auto auto; align-items: end; min-width: 142px; padding-left: 24px; border-left: 1px solid #e5e9e6; color: #64748b; }
.production-score strong { color: #142019; font-size: 38px; line-height: .9; letter-spacing: -.06em; }
.production-score > span { margin-left: 3px; color: #8a968f; font-size: 16px; font-weight: 800; }
.production-score small { grid-column: 1/-1; margin-top: 8px; color: #7b8880; font-size: 9px; white-space: nowrap; }
.production-score.complete strong, .production-score.complete > span { color: #16824f; }
.department-progress { display: grid; grid-template-columns: repeat(6,minmax(0,1fr)); gap: 7px; }
.department-phase { min-width: 0; }
.phase-rail { position: relative; height: 7px; margin: 0 1px 12px; border-radius: 999px; background: #e9eeeb; }
.phase-rail span { position: absolute; left: 0; top: 0; height: 100%; width: 100%; border-radius: inherit; background: #e9eeeb; transition: width .45s ease, background .3s ease, box-shadow .3s ease; }
.phase-rail i { position: absolute; z-index: 2; top: 50%; right: 0; width: 11px; height: 11px; border: 3px solid #fff; border-radius: 50%; background: #cbd5ce; box-shadow: 0 0 0 1px #d9e0db; transform: translate(2px,-50%); }
.department-phase.done .phase-rail span { background: var(--dept-color); box-shadow: 0 3px 10px rgba(var(--dept-rgb),.18); }
.department-phase.done .phase-rail i, .department-phase.current .phase-rail i { background: var(--dept-color); box-shadow: 0 0 0 1px rgba(var(--dept-rgb),.28); }
.department-phase.current .phase-rail span { background: linear-gradient(90deg, rgba(var(--dept-rgb),.28), var(--dept-color), rgba(var(--dept-rgb),.38)); background-size: 180% 100%; box-shadow: 0 0 14px rgba(var(--dept-rgb),.38); animation: department-flow 1.45s linear infinite; }
.department-phase.current .phase-rail i { animation: department-pulse 1.45s ease-out infinite; }
.phase-copy { display: grid; grid-template-columns: 34px minmax(0,1fr) auto; align-items: center; gap: 9px; min-width: 0; padding: 8px 8px 8px 6px; border-radius: 11px; transition: background .2s ease, transform .2s ease; }
.department-phase.current .phase-copy { background: rgba(var(--dept-rgb),.07); transform: translateY(-1px); }
.phase-icon { display: grid; place-items: center; width: 32px; height: 32px; border-radius: 9px; color: #89958e; background: #eef2ef; }
.department-phase.done .phase-icon, .department-phase.current .phase-icon { color: var(--dept-color); background: rgba(var(--dept-rgb),.11); }
.phase-copy > span:nth-child(2) { min-width: 0; }
.phase-copy b, .phase-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.phase-copy b { color: #26332c; font-size: 11px; }
.phase-copy small { margin-top: 3px; color: #9aa49e; font-size: 8px; }
.department-phase.done .phase-copy small { color: var(--dept-color); }
.phase-copy em { align-self: start; color: #b4beb8; font: 800 8px ui-monospace,monospace; font-style: normal; }
@keyframes department-flow { from { background-position: 180% 0; } to { background-position: -80% 0; } }
@keyframes department-pulse { 0% { box-shadow: 0 0 0 0 rgba(var(--dept-rgb),.5); } 70%,100% { box-shadow: 0 0 0 9px rgba(var(--dept-rgb),0); } }
.strip-heading, .panel-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; margin-bottom: 18px; }
.strip-heading span, .panel-heading > div > span { color: #718078; font-size: 9px; font-weight: 900; letter-spacing: .16em; }
.strip-heading h2, .panel-heading h2 { margin: 4px 0 0; color: #101827; font-size: 19px; letter-spacing: -.025em; }
.supervisor-pill { display: inline-flex; align-items: center; gap: 8px; padding: 7px 11px; border: 1px solid #b8e8c7; border-radius: 999px; color: #176b47 !important; background: #effcf3; letter-spacing: 0 !important; }
.supervisor-pill.danger { border-color: #fecaca; color: #b91c1c !important; background: #fff1f2; }
.pipeline { display: grid; grid-template-columns: repeat(6,minmax(0,1fr)); gap: 8px; }
.pipeline-node { position: relative; display: grid; grid-template-columns: 32px 1fr; grid-template-rows: auto auto; gap: 2px 9px; min-width: 0; min-height: 84px; padding: 12px; text-align: left; border: 1px solid #e4e9e5; border-radius: 13px; color: #334155; background: #f8faf8; cursor: pointer; transition: transform .18s,border-color .18s,background .18s; }
.pipeline-node:hover { transform: translateY(-2px); border-color: #9ad6ae; }
.pipeline-node.active { color: #eafcef; border-color: #263c30; background: #15241c; box-shadow: 0 10px 22px rgba(21,36,28,.15); }
.pipeline-index { position: absolute; right: 9px; top: 8px; color: #adb7b0; font: 800 9px ui-monospace,monospace; }
.pipeline-icon { grid-row: 1/3; align-self: center; display: grid; place-items: center; width: 32px; height: 32px; border-radius: 9px; color: #176b47; background: #e2f6e9; }
.pipeline-node.active .pipeline-icon { color: #102017; background: #4ade80; }
.pipeline-copy { min-width: 0; align-self: end; }
.pipeline-copy strong, .pipeline-copy small { display: block; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.pipeline-copy strong { font-size: 12px; }
.pipeline-copy small { margin-top: 3px; color: #849089; font: 500 8px ui-monospace,monospace; }
.pipeline-state { align-self: start; color: #86918b; font: 800 8px ui-monospace,monospace; }
.pipeline-node.running .pipeline-state { color: #16a34a; }
.pipeline-node.verified { border-color: #86efac; background: #f0fdf4; }
.pipeline-node.failed { border-color: #fecaca; background: #fff7f7; }
.pipeline-node.verified .pipeline-state { color: #15803d; }
.pipeline-node.failed .pipeline-state { color: #b91c1c; }
.audit-detail { display: grid; gap: 5px; margin-top: 12px; padding: 12px 14px; border: 1px solid #e2e8f0; border-radius: 11px; color: #64748b; background: #f8fafc; font-size: 10px; }
.audit-detail strong { color: #17201c; font-size: 12px; }
.pipeline-arrow { position: absolute; z-index: 3; right: -14px; top: 34px; color: #9ca9a0; pointer-events: none; }

.war-grid { display: grid; grid-template-columns: minmax(0,1.55fr) minmax(340px,.8fr); align-items: start; gap: 18px; }
.graph-panel, .trace-panel { padding: 22px; }
.graph-panel { position: relative; overflow: hidden; color: #e2e8f0; border-color: #253249; background: #111a2b; }
.graph-panel .panel-heading h2 { color: #f8fafc; }
.graph-panel .panel-heading > div > span { color: #4ade80; }
.panel-stat { display: flex; align-items: baseline; gap: 6px; text-align: right; }
.panel-stat strong { color: #4ade80; font-size: 28px; }
.panel-stat small { color: #8290a4; font-size: 10px; }
.graph-head-actions { display: flex; align-items: center; gap: 12px; }
.graph-view-switch { display: flex; padding: 3px; border: 1px solid #334258; border-radius: 8px; background: #0c1422; }
.graph-view-switch button { min-height: 27px; padding: 0 10px; border: 0; border-radius: 6px; color: #718098; background: transparent; font-size: 9px; font-weight: 800; cursor: pointer; }
.graph-view-switch button.active { color: #07120d; background: #4ade80; }
.graph-stage { position: relative; overflow: hidden; border: 1px solid #253249; border-radius: 14px; background: radial-gradient(circle at 50% 46%, rgba(39,54,76,.35), transparent 56%); }
.graph-stage .collab-svg { width: 100%; height: 420px; touch-action: none; user-select: none; cursor: grab; }
.graph-stage .collab-svg.moving { cursor: grabbing; }
.graph-stage .cm-dept-label { fill: #718098; font-size: 10px; font-weight: 800; }
.graph-stage .cm-lane line { stroke: #243248; stroke-width: 1; stroke-dasharray: 3 8; vector-effect: non-scaling-stroke; }
.graph-stage .cm-node-name { fill: #94a3b8; font-size: 9px; }
.graph-stage .cm-node { cursor: grab; }
.graph-stage .cm-node.dragging { cursor: grabbing; }
.graph-stage .cm-node.dragging circle { stroke: #f8fafc; filter: drop-shadow(0 0 12px rgba(74,222,128,.65)); }
.graph-stage line { transition: stroke-opacity .18s; }
.graph-stage .cm-link { pointer-events: none; }
.graph-stage .cm-link-glow,
.graph-stage .cm-link-base,
.graph-stage .cm-link-pulse { stroke-linecap: round; }
.graph-stage .cm-link-glow { filter: blur(2px); }
.graph-stage .cm-link-pulse {
  fill: none;
  stroke-dasharray: 2 16;
  filter: drop-shadow(0 0 4px rgba(255,255,255,.34));
  animation: cm-link-flow 1.35s linear infinite;
}
@keyframes cm-link-flow {
  from { stroke-dashoffset: 18; }
  to { stroke-dashoffset: 0; }
}
@media (prefers-reduced-motion: reduce) {
  .graph-stage .cm-link-pulse, .department-phase.current .phase-rail span, .department-phase.current .phase-rail i { animation: none; }
  .graph-stage .cm-link-pulse { stroke-dasharray: 3 13; }
}
.graph-tools { position: absolute; z-index: 4; top: 10px; left: 10px; display: flex; align-items: center; gap: 4px; padding: 4px; border: 1px solid #334258; border-radius: 9px; background: rgba(10,17,29,.88); backdrop-filter: blur(8px); }
.graph-tools button { display: grid; place-items: center; width: 28px; height: 28px; padding: 0; border: 0; border-radius: 6px; color: #cbd5e1; background: transparent; font-size: 17px; cursor: pointer; }
.graph-tools button:hover { color: #07120d; background: #4ade80; }
.graph-tools span { min-width: 38px; padding: 0 5px; color: #8290a4; font: 700 9px ui-monospace,monospace; text-align: right; }
.graph-caption { display: flex; justify-content: space-between; padding: 10px 12px; border-top: 1px solid #26344a; color: #718098; background: rgba(12,20,34,.82); font-size: 10px; }
.graph-caption span:first-child { display: flex; align-items: center; gap: 7px; color: #b9c7d9; }
.graph-caption i { width: 7px; height: 7px; border-radius: 50%; background: #4ade80; }
.node-inspector { position: absolute; z-index: 5; right: 18px; bottom: 48px; width: 230px; padding: 14px; border: 1px solid #3c4d65; border-radius: 12px; background: rgba(15,23,42,.96); box-shadow: 0 14px 40px rgba(0,0,0,.32); }
.node-inspector > div { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.node-inspector span, .node-inspector b { color: #4ade80; font-size: 9px; }
.node-inspector strong { font-size: 13px; }
.node-inspector p { margin: 5px 0; color: #9ba9bc; font-size: 10px; line-height: 1.45; }
.node-inspector b { display: inline-block; width: 34px; }
.node-inspector button { margin-top: 8px; border: 0; color: #94a3b8; background: transparent; font-size: 10px; cursor: pointer; }
.trace-panel { box-sizing: border-box; height: 570px; overflow: hidden; }
.trace-live { display: inline-flex; align-items: center; gap: 7px; color: #16a34a; font: 900 9px ui-monospace,monospace; }
.trace-list { display: flex; flex-direction: column; max-height: 480px; padding-right: 5px; overflow-y: auto; scrollbar-width: thin; }
.trace-row { display: grid; grid-template-columns: 54px 28px minmax(0,1fr) 18px; align-items: start; gap: 8px; padding: 12px 0; border-top: 1px solid #edf0ed; }
.trace-time { padding-top: 3px; color: #94a3b8; font: 600 9px ui-monospace,monospace; overflow: hidden; }
.trace-icon { display: grid; place-items: center; width: 26px; height: 26px; border-radius: 8px; color: #176b47; background: #e9f7ee; }
.trace-row strong { display: block; color: #18221d; font-size: 11px; }
.trace-row p { display: -webkit-box; margin: 3px 0 0; overflow: hidden; color: #68736d; font-size: 10px; line-height: 1.4; -webkit-line-clamp: 2; -webkit-box-orient: vertical; }
.trace-ok { padding-top: 4px; color: #22c55e; font-size: 11px; }
.trace-empty { padding: 80px 0; text-align: center; color: #94a3b8; }

.agent-console { padding: 22px; }
.dept-switcher { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 4px; }
.dept-switcher button { padding: 6px 9px; border: 1px solid #e1e6e2; border-radius: 8px; color: #66716a; background: #fafbfa; font-size: 10px; cursor: pointer; }
.dept-switcher button.active { border-color: #1e3327; color: #fff; background: #17291f; }
.dept-switcher b { margin-left: 3px; font-size: 9px; opacity: .6; }
.agent-rows { border-top: 1px solid #e7ebe8; }
.agent-row { display: grid; grid-template-columns: 34px 110px minmax(180px,1fr) 88px 95px 150px 32px; align-items: center; gap: 10px; min-height: 58px; border-bottom: 1px solid #edf0ed; }
.agent-row:hover { background: #fafcfb; }
.agent-avatar { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 9px; color: #66716a; background: #eef1ef; }
.agent-row.busy .agent-avatar { color: #102017; background: #4ade80; box-shadow: 0 0 0 4px rgba(74,222,128,.12); }
.agent-id strong, .agent-id small { display: block; }
.agent-id strong { color: #18221d; font-size: 11px; }
.agent-id small { margin-top: 3px; color: #94a3b8; font: 800 7px ui-monospace,monospace; letter-spacing: .1em; }
.agent-id small.st-working { color: #16a34a; }
.agent-id small.st-idle { color: #b45309; }
.agent-id small.st-dead { color: #b42318; }
.agent-id small.st-unknown { color: #94a3b8; }
.agent-row.busy .agent-id small { color: #16a34a; }
.agent-row.dead { opacity: .6; }
.agent-last { font-size: 10px; color: #8a938c; white-space: nowrap; background: #f1f5f2; padding: 2px 7px; border-radius: 999px; font-weight: 700; }
.agent-doing { margin: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: #647069; font-size: 10px; }
.agent-handoff { display: flex; align-items: center; gap: 5px; color: #64748b; font-size: 9px; }
.agent-artifacts { display: flex; gap: 4px; overflow: hidden; }
.agent-artifacts button { white-space: nowrap; padding: 4px 6px; border: 1px solid #d8e7dd; border-radius: 6px; color: #176b47; background: #f3fbf5; font-size: 8px; cursor: pointer; }
.agent-artifacts span { color: #a1aaa5; font-size: 9px; }
.agent-output { text-align: right; color: #17291f; font-size: 16px; }

.approval-desk { }
.approval-hero { display: flex; align-items: flex-end; justify-content: space-between; gap: 40px; padding-bottom: 24px; border-bottom: 1px solid #e5e9e6; }
.approval-hero h2 { margin: 0; color: #101827; font-size: clamp(28px,3vw,44px); letter-spacing: -.045em; }
.approval-hero p:not(.section-kicker) { max-width: 680px; margin: 10px 0 0; color: #748078; font-size: 13px; line-height: 1.6; }
.approval-total { flex: 0 0 auto; text-align: right; }
.approval-total strong { display: block; color: #176b47; font-size: 48px; line-height: 1; }
.approval-total span { display: block; margin-top: 5px; color: #7d8981; font-size: 11px; }
.approval-policy { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; padding: 16px 0 20px; }
.approval-policy span { display: inline-flex; align-items: center; gap: 6px; padding: 7px 10px; border: 1px solid #e1e6e2; border-radius: 999px; color: #627068; background: #fafcfa; font-size: 10px; }
.project-approval-grid { display: grid; grid-template-columns: minmax(0,1fr); gap: 12px; }
.project-approval-card { display: flex; flex-direction: column; min-width: 0; padding: 18px; border: 1px solid #e1e6e2; border-radius: 15px; background: #fbfcfb; transition: transform .18s,border-color .18s,box-shadow .18s; }
.project-approval-card:hover { transform: translateY(-2px); border-color: #b6d6c0; box-shadow: 0 12px 25px rgba(15,23,42,.07); }
.project-approval-card header { display: flex; align-items: center; justify-content: space-between; }
.project-number { color: #8d9891; font: 800 8px ui-monospace,monospace; letter-spacing: .15em; }
.project-score { display: inline-flex; align-items: center; gap: 5px; color: #16824f; font: 800 8px ui-monospace,monospace; }
.project-score i { width: 6px; height: 6px; border-radius: 50%; background: #4ade80; }
.project-score.blocked { color: #b76b12; }
.project-score.blocked i { background: #f59e0b; }
.project-approval-card h3 { margin: 16px 0 11px; color: #142019; font-size: 20px; letter-spacing: -.03em; }
.project-meta { display: flex; align-items: center; gap: 13px; flex-wrap: wrap; color: #748078; font-size: 9px; }
.project-meta span { display: inline-flex; align-items: center; gap: 4px; }
.project-preview { margin: 16px 0 14px; overflow: hidden; border: 1px solid #dfe5e1; border-radius: 12px; background: #101827; }
.preview-head { display: flex; align-items: center; justify-content: space-between; min-height: 38px; padding: 0 11px; border-bottom: 1px solid #293548; color: #d8e3dc; font-size: 9px; font-weight: 800; }
.preview-head span { display: inline-flex; align-items: center; gap: 6px; }
.preview-head button { border: 0; color: #4ade80; background: transparent; font-size: 9px; cursor: pointer; }
.project-preview iframe { display: block; width: 100%; height: clamp(360px,52vw,620px); border: 0; background: #fff; }
.project-preview video, .project-preview img { display: block; width: 100%; height: clamp(360px,52vw,620px); object-fit: contain; background: #030807; }
.pptx-launch { display: grid; width: 100%; height: 230px; place-items: center; align-content: center; gap: 10px; border: 0; color: #f97316; background: #111827; cursor: pointer; }
.pptx-launch span { color: #e5e7eb; font-size: 11px; font-weight: 800; }
.project-preview pre { height: 210px; margin: 0; padding: 14px; overflow: auto; color: #c5d2c9; background: #101827; font: 10px/1.6 ui-monospace,SFMono-Regular,Consolas,monospace; white-space: pre-wrap; }
.preview-missing { display: grid; place-items: center; align-content: center; gap: 8px; height: 170px; color: #7f8da1; font-size: 10px; }
.delivery-theater { position: relative; margin-bottom: 0; border-color: #193b32; border-radius: 18px 18px 0 0; background: #030807; box-shadow: 0 24px 70px rgba(6,78,59,.16); }
.delivery-theater .preview-head { min-height: 48px; padding: 0 16px; border-color: #ffffff16; color: #99f6e4; letter-spacing: .12em; }
.artifact-hero { position: relative; width: 100%; min-height: 430px; display: flex; align-items: center; justify-content: center; gap: 28px; border: 0; color: #fff; cursor: pointer; overflow: hidden; }
.artifact-hero::before { position: absolute; inset: -40%; content: ''; filter: blur(40px); opacity: .65; }
.artifact-hero > * { position: relative; }
.artifact-hero span { display: flex; align-items: flex-start; flex-direction: column; text-align: left; }
.artifact-hero b { margin-bottom: 9px; font: 800 10px ui-monospace,monospace; letter-spacing: .2em; }
.artifact-hero strong { font-size: clamp(24px,4vw,48px); letter-spacing: -.05em; }
.artifact-hero small { margin-top: 11px; color: #ffffff99; font-size: 12px; }
.excel-hero { background: #052e2b; }
.excel-hero::before { background: radial-gradient(circle,#34d399 0,transparent 55%); }
.ppt-hero { background: #2a1209; }
.ppt-hero::before { background: radial-gradient(circle,#fb923c 0,transparent 55%); }
.receipt-hero { background: #0f172a; }
.receipt-hero::before { background: radial-gradient(circle,#38bdf8 0,transparent 55%); }
.artifact-proof-strip { min-height: 42px; display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 12px; padding: 0 15px; border-top: 1px solid #ffffff12; color: #94a3b8; background: #07110f; font-size: 9px; }
.artifact-proof-strip span { display: inline-flex; align-items: center; gap: 6px; color: #6ee7b7; font-weight: 800; }
.artifact-proof-strip span i, .verified-dot { width: 6px; height: 6px; border-radius: 50%; background: #34d399; box-shadow: 0 0 12px #34d399; }
.artifact-proof-strip strong { overflow: hidden; font-weight: 500; text-overflow: ellipsis; white-space: nowrap; }
.artifact-proof-strip code { color: #64748b; font-size: 8px; }
.delivery-switcher { display: grid; grid-template-columns: repeat(6,minmax(0,1fr)); gap: 1px; margin: 0 0 18px; overflow: hidden; border: 1px solid #dbe5df; border-top: 0; border-radius: 0 0 14px 14px; background: #dbe5df; }
.delivery-switcher button { position: relative; min-width: 0; min-height: 68px; display: grid; grid-template-columns: 28px minmax(0,1fr) 7px; align-items: center; gap: 8px; padding: 9px 10px; border: 0; color: #64748b; background: #f8faf9; text-align: left; cursor: pointer; transition: background .18s,color .18s; }
.delivery-switcher button:hover, .delivery-switcher button.active { color: #064e3b; background: #ecfdf5; }
.delivery-kind { display: grid; place-items: center; width: 28px; height: 28px; border-radius: 8px; color: #047857; background: #d1fae5; }
.delivery-switcher button > span:nth-child(2) { min-width: 0; display: flex; flex-direction: column; }
.delivery-switcher b { font-size: 9px; }
.delivery-switcher small { margin-top: 3px; overflow: hidden; font-size: 7px; text-overflow: ellipsis; white-space: nowrap; }
.project-stage-track { position: relative; display: grid; grid-template-columns: repeat(11,minmax(0,1fr)); gap: 3px; margin: 4px 0 12px; }
.project-stage-track::before { position: absolute; z-index: 0; top: 15px; right: 4%; left: 4%; height: 1px; background: #dfe5e1; content: ''; }
.stage-node { position: relative; z-index: 1; display: flex; align-items: center; flex-direction: column; gap: 5px; min-width: 0; padding: 0; border: 0; background: transparent; cursor: pointer; }
.stage-node > span { display: grid; place-items: center; width: 30px; height: 30px; border: 2px solid #dce3de; border-radius: 50%; color: #9aa59e; background: #fbfcfb; }
.stage-node small { overflow: hidden; width: 100%; color: #8a958e; font-size: 8px; text-align: center; text-overflow: ellipsis; white-space: nowrap; }
.stage-node.done > span { border-color: #49b879; color: #0c6f43; background: #eaf8ef; box-shadow: 0 0 0 3px #f4fbf6; }
.stage-node.done small { color: #176b47; font-weight: 800; }
.stage-node.missing { cursor: default; }
.project-gap { display: flex; align-items: flex-start; gap: 6px; margin: 0 0 12px; padding: 8px 10px; border-radius: 8px; color: #98621d; background: #fff7e8; font-size: 9px; line-height: 1.45; }
.project-agents { display: flex; gap: 5px; flex-wrap: wrap; margin: 14px 0; }
.project-agents span { padding: 4px 7px; border-radius: 6px; color: #425048; background: #edf1ee; font: 700 8px ui-monospace,monospace; }
.project-deliveries { margin-bottom: 16px; border-top: 1px solid #e6eae7; border-bottom: 1px solid #e6eae7; }
.project-deliveries summary { padding: 10px 0; color: #58655e; font-size: 10px; cursor: pointer; }
.project-deliveries summary b { float: right; color: #176b47; }
.project-deliveries > button { display: grid; grid-template-columns: 80px minmax(0,1fr) 34px; gap: 8px; width: 100%; padding: 7px 0; border: 0; border-top: 1px solid #edf0ee; color: #839087; background: transparent; font-size: 9px; text-align: left; cursor: pointer; }
.project-deliveries > button:hover { color: #176b47; }
.project-deliveries code { overflow: hidden; color: #526158; font-size: 8px; text-overflow: ellipsis; white-space: nowrap; }
.project-deliveries em { color: #16824f; font-size: 8px; font-style: normal; text-align: right; }
.project-actions { display: grid; grid-template-columns: .75fr 1.25fr; gap: 7px; margin-top: auto; }
.project-actions button { display: inline-flex; align-items: center; justify-content: center; gap: 6px; min-height: 40px; border-radius: 9px; font-size: 10px; font-weight: 800; cursor: pointer; }
.reject-project { border: 1px solid #dbe1dd; color: #657169; background: #fff; }
.approve-project { border: 1px solid #17291f; color: #fff; background: #17291f; }
.approve-project:hover { background: #214431; }
.approve-project:disabled { border-color: #d9dfdb; color: #97a19b; background: #edf0ee; cursor: not-allowed; }

/* 公司长页面：弱化系统默认滚动条，同时保留清晰的当前位置反馈。 */
.company-view,.trace-list,.meeting-minutes,.project-deliveries { scrollbar-width: thin; scrollbar-color: #5b9b7d transparent; }
.company-view::-webkit-scrollbar,.trace-list::-webkit-scrollbar,.meeting-minutes::-webkit-scrollbar,.project-deliveries::-webkit-scrollbar { width: 8px; height: 8px; }
.company-view::-webkit-scrollbar-track,.trace-list::-webkit-scrollbar-track,.meeting-minutes::-webkit-scrollbar-track,.project-deliveries::-webkit-scrollbar-track { margin-block: 10px; border-radius: 999px; background: rgba(15,70,49,.055); }
.company-view::-webkit-scrollbar-thumb,.trace-list::-webkit-scrollbar-thumb,.meeting-minutes::-webkit-scrollbar-thumb,.project-deliveries::-webkit-scrollbar-thumb { min-height: 42px; border: 2px solid transparent; border-radius: 999px; background: linear-gradient(#66b68f,#39775d) padding-box; }
.company-view::-webkit-scrollbar-thumb:hover,.trace-list::-webkit-scrollbar-thumb:hover,.meeting-minutes::-webkit-scrollbar-thumb:hover,.project-deliveries::-webkit-scrollbar-thumb:hover { background: linear-gradient(#4da477,#245f47) padding-box; }

@media (max-width: 1180px) {
  .pipeline { grid-template-columns: repeat(3,1fr); }
  .department-progress { grid-template-columns: repeat(3,minmax(0,1fr)); row-gap: 18px; }
  .pipeline-arrow { display: none; }
  .war-grid { grid-template-columns: 1fr; }
  .trace-panel { height: auto; }
  .project-approval-grid { grid-template-columns: 1fr; }
}
@media (max-width: 760px) {
  .company-view { padding: 12px 12px 76px; }
  .command-copy { grid-template-columns: 1fr; gap: 24px; }
  .zero-cost-seal { width: auto; }
  .command-metrics { grid-template-columns: repeat(2,1fr); }
  .command-metric:nth-child(2) { border-right: 0; }
  .pipeline { grid-template-columns: 1fr 1fr; }
  .production-overview { align-items: stretch; flex-direction: column; gap: 16px; }
  .production-score { grid-template-columns: auto auto 1fr; align-items: end; padding: 14px 0 0; border-top: 1px solid #e5e9e6; border-left: 0; }
  .production-score small { grid-column: auto; margin: 0 0 1px 9px; }
  .department-progress { grid-template-columns: repeat(2,minmax(0,1fr)); }
  .agent-row { grid-template-columns: 34px 90px 1fr 28px; }
  .agent-handoff, .agent-artifacts { display: none; }
  .panel-heading { flex-direction: column; }
  .dept-switcher { justify-content: flex-start; }
  .approval-desk { padding: 18px; }
  .approval-hero { align-items: flex-start; flex-direction: column; }
  .approval-total { text-align: left; }
  .delivery-switcher { grid-template-columns: repeat(2,minmax(0,1fr)); }
    .artifact-proof-strip { grid-template-columns: 1fr; padding: 10px 12px; }
    .project-stage-track { grid-template-columns: repeat(6,minmax(0,1fr)); row-gap: 12px; }
  }

  /* ===== 下达指令（考题）区 ===== */
  .directive-zone {
    margin-top: 16px;
    padding: 14px 16px;
    border-radius: 12px;
    background: linear-gradient(135deg, rgba(23, 41, 31, .06), rgba(23, 41, 31, .02));
    border: 1px solid rgba(23, 41, 31, .12);
  }
  .directive-head { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; flex-wrap: wrap; }
  .directive-head h3 { margin: 0; font-size: 14px; color: #17291f; }
  .directive-tip { font-size: 12px; color: #8a938c; }
  .directive-input-row { display: flex; gap: 8px; }
  .directive-input {
    flex: 1;
    padding: 9px 12px;
    border-radius: 8px;
    border: 1px solid #dbe1dd;
    background: #fff;
    font-size: 13px;
    color: #17291f;
    outline: none;
  }
  .directive-input:focus { border-color: #5b9b7d; box-shadow: 0 0 0 2px rgba(91, 155, 125, .15); }
  .directive-model-select {
    padding: 9px 10px;
    border-radius: 8px;
    border: 1px solid #dbe1dd;
    background: #fff;
    font-size: 13px;
    color: #17291f;
    outline: none;
    max-width: 200px;
    white-space: nowrap;
  }
  .directive-model-select:focus { border-color: #5b9b7d; }
  .directive-current em { font-style: normal; color: #1d6b45; }
  .directive-save-btn {
    padding: 9px 16px;
    border-radius: 8px;
    border: none;
    background: #17291f;
    color: #fff;
    font-size: 13px;
    cursor: pointer;
    white-space: nowrap;
  }
  .directive-save-btn:hover { background: #214431; }
  .directive-save-btn:disabled { opacity: .5; cursor: not-allowed; }
  .directive-clear-btn {
    padding: 9px 14px;
    border-radius: 8px;
    border: 1px solid #dbe1dd;
    background: #fff;
    color: #657169;
    font-size: 13px;
    cursor: pointer;
    white-space: nowrap;
  }
  .directive-clear-btn:hover { background: #f4f6f5; }
  /* 产出输出 token 数设置行 */
  .directive-output-row { display: flex; align-items: center; gap: 8px; margin-top: 10px; flex-wrap: wrap; }
  .directive-output-row label { font-size: 12px; color: #657169; white-space: nowrap; }
  .directive-output-input {
    width: 110px;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid #dbe1dd;
    background: #fff;
    font-size: 13px;
    color: #17291f;
    outline: none;
  }
  .directive-output-input:focus { border-color: #5b9b7d; box-shadow: 0 0 0 2px rgba(91, 155, 125, .15); }
  .directive-output-save {
    padding: 8px 14px;
    border-radius: 8px;
    border: none;
    background: #17291f;
    color: #fff;
    font-size: 12px;
    cursor: pointer;
    white-space: nowrap;
  }
  .directive-output-save:hover { background: #214431; }
  .directive-output-save:disabled { opacity: .5; cursor: not-allowed; }
  .directive-output-hint { font-size: 11px; color: #8a938c; flex: 1 1 100%; }
  .directive-current {
    margin-top: 10px;
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: #17291f;
    background: rgba(23, 41, 31, .05);
    padding: 8px 12px;
    border-radius: 8px;
  }
  .directive-current strong { color: #1d6b45; }
  .directive-error {
    margin-top: 8px; padding: 9px 12px; border: 1px solid #fecaca; border-radius: 10px;
    background: #fff1f2; color: #b42318; font-size: 12px; font-weight: 700;
  }
  /* ===== 发行评测 ===== */
  .reviews-room { margin-top: 28px; }
  .reviews-total { font-size: 12px; color: #8a938c; }
  .reviews-list { display: flex; flex-direction: column; gap: 14px; }
  .review-card {
    border: 1px solid rgba(23, 41, 31, .12);
    border-radius: 14px;
    background: #fff;
    padding: 16px 18px;
    box-shadow: 0 1px 3px rgba(15, 70, 49, .05);
  }
  .review-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .review-title { display: flex; align-items: baseline; gap: 8px; flex-wrap: wrap; }
  .review-title strong { font-size: 16px; color: #17291f; }
  .review-agent { font-size: 12px; color: #657169; background: #eef3f0; padding: 2px 8px; border-radius: 999px; }
  .review-award { font-size: 12px; font-weight: 700; padding: 2px 10px; border-radius: 999px; }
  .review-award.award-god { color: #92400e; background: #fef3c7; border: 1px solid #f59e0b; }
  .review-award.award-good { color: #1d6b45; background: #e6f4ec; border: 1px solid #34d399; }
  .review-award.award-new { color: #1e40af; background: #e0e7ff; border: 1px solid #818cf8; }
  .review-score { display: flex; align-items: baseline; gap: 3px; }
  .review-score strong { font-size: 24px; }
  .review-score.good strong { color: #1d6b45; }
  .review-score.mid strong { color: #b45309; }
  .review-score small { font-size: 12px; color: #97a19b; }
  .review-users { display: flex; flex-direction: column; gap: 10px; }
  .review-user { display: flex; gap: 10px; padding: 10px 12px; background: #f7faf8; border-radius: 10px; }
  .ru-avatar { font-size: 22px; line-height: 1; }
  .ru-body { flex: 1; min-width: 0; }
  .ru-body header { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .ru-body header strong { font-size: 13px; color: #17291f; }
  .ru-stars { color: #f0b429; font-size: 12px; letter-spacing: 1px; }
  .ru-stars i { font-style: normal; color: #657169; margin-left: 4px; }
  .ru-model { font-style: normal; font-size: 11px; color: #1d6b45; background: #e6f4ec; padding: 1px 8px; border-radius: 999px; }
  .ru-body p { margin: 4px 0 0; font-size: 13px; color: #374151; line-height: 1.55; }
  .review-form { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-top: 12px; padding-top: 12px; border-top: 1px dashed rgba(23, 41, 31, .15); }
  .rf-nick { width: 96px; padding: 6px 10px; border: 1px solid rgba(23, 41, 31, .2); border-radius: 8px; font-size: 12px; background: #fff; }
  .rf-comment { flex: 1; min-width: 160px; padding: 6px 10px; border: 1px solid rgba(23, 41, 31, .2); border-radius: 8px; font-size: 12px; background: #fff; }
  .rf-stars { display: flex; align-items: center; gap: 2px; }
  .rf-star { border: none; background: none; font-size: 18px; color: #d8dfda; cursor: pointer; padding: 0 1px; line-height: 1; }
  .rf-star.on { color: #f0b429; }
  .rf-score-num { font-size: 12px; color: #b45309; font-weight: 700; margin-left: 4px; }
  .rf-submit { padding: 6px 14px; border: none; border-radius: 8px; background: #1d6b45; color: #fff; font-size: 12px; font-weight: 700; cursor: pointer; }
  .rf-submit:disabled { opacity: .5; cursor: wait; }
  .rf-msg { font-size: 12px; color: #1d6b45; font-weight: 700; }
  .reviews-empty {
    border: 1px dashed rgba(23, 41, 31, .2);
    border-radius: 12px;
    padding: 28px;
    text-align: center;
    color: #97a19b;
    display: flex; flex-direction: column; align-items: center; gap: 8px;
  }

  /* ===== 应用大厅（B站式信息流） ===== */
  .market-room { margin-top: 28px; }
  .market-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 14px; }
  .market-card {
    border: 1px solid rgba(23, 41, 31, .12);
    border-radius: 14px;
    background: #fff;
    overflow: hidden;
    box-shadow: 0 1px 3px rgba(15, 70, 49, .05);
    transition: transform .15s ease;
  }
  .market-card:hover { transform: translateY(-2px); box-shadow: 0 4px 12px rgba(15, 70, 49, .1); }
  .market-card.promoted { border-color: #eab308; box-shadow: 0 0 0 1px #eab308, 0 4px 14px rgba(234, 179, 8, .15); }
  .market-card.unreviewed { opacity: .85; }
  .mc-cover {
    position: relative;
    height: 84px;
    display: grid; place-items: center;
    background: linear-gradient(135deg, #eef3f0, #dce8e1);
    font-size: 40px;
  }
  .market-card.promoted .mc-cover { background: linear-gradient(135deg, #fef9c3, #fde68a); }
  .mc-promo {
    position: absolute; top: 8px; right: 8px;
    background: #eab308; color: #fff;
    font-size: 10px; font-weight: 800;
    padding: 2px 8px; border-radius: 999px;
    letter-spacing: .08em;
  }
  .mc-award {
    position: absolute; bottom: 8px; left: 8px;
    font-size: 11px; font-weight: 700;
    padding: 2px 9px; border-radius: 999px;
    background: #fff;
  }
  .mc-award.award-god { color: #92400e; background: #fef3c7; border: 1px solid #f59e0b; }
  .mc-award.award-good { color: #1d6b45; background: #e6f4ec; border: 1px solid #34d399; }
  .mc-award.award-new { color: #1e40af; background: #e0e7ff; border: 1px solid #818cf8; }
  .mc-body { padding: 12px 14px 14px; }
  .mc-body h3 { margin: 0; font-size: 14px; color: #17291f; }
  .mc-agent { margin: 3px 0 8px; font-size: 11px; color: #8a938c; }
  .mc-meta { display: flex; align-items: baseline; gap: 8px; }
  .mc-score { display: flex; align-items: baseline; gap: 2px; }
  .mc-score b { font-size: 22px; }
  .mc-score.good b { color: #1d6b45; }
  .mc-score.mid b { color: #b45309; }
  .mc-score small { font-size: 11px; color: #97a19b; }
  .mc-count { font-size: 11px; color: #8a938c; }
  .mc-wait { font-size: 12px; color: #97a19b; font-weight: 700; }
  .mc-promo-desc { margin: 8px 0 0; font-size: 11px; color: #a16207; background: #fef9c3; padding: 4px 8px; border-radius: 8px; }

  /* ===== 开罗式办公室场景（游戏发展国：俯视公司 + 员工走动 + 头顶飘字）===== */
  .office-scene { margin-top: 28px; }
  .office-money {
    display: flex; flex-direction: column; align-items: flex-end; gap: 2px;
    background: linear-gradient(135deg, #fef9c3, #fde68a);
    border: 1px solid #eab308;
    border-radius: 12px;
    padding: 6px 14px;
  }
  .office-money b { font-size: 18px; color: #854d0e; }
  .office-money small { font-size: 11px; color: #a16207; }
  .office-map {
    position: relative;
    height: 640px;
    border-radius: 16px;
    border: 1px solid rgba(120, 90, 50, .25);
    background:
      linear-gradient(rgba(120, 90, 50, .07) 1px, transparent 1px),
      linear-gradient(90deg, rgba(120, 90, 50, .07) 1px, transparent 1px),
      #f6efdd;
    background-size: 30px 30px;
    overflow: hidden;
    box-shadow: inset 0 0 60px rgba(120, 90, 50, .12);
  }
  .office-dept {
    position: absolute;
    border-radius: 12px;
    background: rgba(255, 255, 255, .78);
    border: 1px solid rgba(120, 90, 50, .22);
    box-shadow: 0 2px 8px rgba(120, 90, 50, .12);
  }
  .dept-ceo { background: rgba(255, 250, 235, .9); border-color: rgba(200, 160, 40, .5); }
  .dept-sign {
    position: absolute;
    top: 5px; left: 8px; right: 8px;
    font-size: 11px; font-weight: 800; color: #7a5c2e;
    letter-spacing: .04em;
    z-index: 2;
  }
  .dept-sign b { color: #b08a3e; font-size: 10px; margin-left: 3px; }
  .office-person {
    position: absolute;
    transform: translate(-50%, -50%);
    display: flex; flex-direction: column; align-items: center;
    z-index: 3;
    pointer-events: none;
  }
  .op-face {
    font-size: 22px; line-height: 1;
    background: #fff;
    border: 1.5px solid #d8c9a8;
    border-radius: 50%;
    width: 30px; height: 30px;
    display: grid; place-items: center;
    box-shadow: 0 2px 5px rgba(90, 70, 40, .25);
  }
  .op-work .op-face { border-color: #34d399; box-shadow: 0 0 0 3px rgba(52, 211, 153, .2), 0 2px 5px rgba(90, 70, 40, .25); }
  .op-idle .op-face { border-color: #f0b429; }
  .op-dead .op-face { filter: grayscale(.85); opacity: .55; }
  .op-dead { opacity: .6; }
  .op-bubble {
    position: absolute;
    bottom: 30px;
    white-space: nowrap;
    background: rgba(255, 255, 255, .96);
    border: 1px solid rgba(120, 90, 50, .25);
    border-radius: 999px;
    padding: 2px 8px;
    font-size: 10px; color: #4a5a4e; font-weight: 700;
    box-shadow: 0 2px 6px rgba(90, 70, 40, .15);
    display: flex; align-items: center; gap: 4px;
  }
  .op-bubble i { width: 7px; height: 7px; border-radius: 50%; display: inline-block; }
  .dot-green { background: #22c55e; }
  .dot-yellow { background: #f0b429; }
  .dot-red { background: #ef4444; }
  .op-name {
    margin-top: 2px;
    font-size: 9px; color: #7a5c2e; font-weight: 800;
    background: rgba(255, 255, 255, .85);
    border-radius: 6px;
    padding: 0 5px;
  }
  .office-prop { position: absolute; font-size: 22px; opacity: .8; z-index: 1; }
  .prop-water { left: 4.5%; top: 4%; }
  .prop-desk1 { right: 4%; top: 6%; font-size: 26px; }
  .prop-desk2 { left: 49.5%; top: 6%; }
  .prop-desk3 { right: 12%; top: 96%; }
  </style>
