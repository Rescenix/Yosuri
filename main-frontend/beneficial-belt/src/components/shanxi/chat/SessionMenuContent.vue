<template>
  <div class="smc-root" :class="{ fill }">
    <!-- 顶部操作：搜索和定时任务已收进侧栏右上角 -->
    <div class="smc-nav">
      <button class="smc-nav-item primary" type="button" @click="onClickNewSession">
        <Icon icon="mdi:plus" width="18" />
        <span>新建会话</span>
      </button>
      <router-link class="smc-nav-item" to="/sites" title="发布并分享 Agent 写好的网页">
        <Icon icon="mdi:web" width="18" />
        <span>站点</span>
      </router-link>
    </div>

    <!-- 会话列表区 -->
    <div class="smc-session-area">

      <!-- 置顶项目文件夹 -->
      <div v-if="pinnedFolders.length" class="smc-section">
        <div class="smc-section-label">
          <Icon icon="mdi:pin" width="14" color="var(--app-accent)" />
          <span>置顶</span>
        </div>
        <div v-for="f in pinnedFolders" :key="'pin_' + f.name" class="smc-folder">
          <div class="smc-folder-head" @click="togglePinnedFolder(f.name)">
            <span class="smc-folder-chevron" :class="{ open: expandedPinned[f.name] }">›</span>
            <Icon icon="mdi:folder-outline" width="15" color="var(--app-accent)" />
            <span class="smc-folder-name">{{ f.name }}</span>
            <button
              v-if="projectForName(f.name)"
              class="smc-project-new-chat"
              type="button"
              title="在此项目中新建对话"
              aria-label="在此项目中新建对话"
              @click.stop="newSessionForProject(f.name)"
            >
              <Icon icon="mdi:message-plus-outline" width="17" />
            </button>
          </div>
          <div v-if="expandedPinned[f.name]" class="smc-folder-children">
            <div
              v-for="s in f.sessions"
              :key="s.id"
              class="smc-session-row"
              :class="{ active: s.id === activeSession, running: s.id === runningSession }"
              @mouseenter="hoveredId = s.id"
              @mouseleave="onRowLeave(s.id)"
              @click="onRowClick(s)"
            >
                          <RunningRing
                            v-if="s.id === runningSession"
                            class="smc-running-ring"
                          />
                          <span v-if="bulkMode" class="smc-bulk-check" @click.stop="toggleBulkSelect(s)">
                <Icon :icon="bulkSelected.has(s.id) ? 'mdi:checkbox-marked' : 'mdi:checkbox-blank-outline'" width="16" color="var(--app-accent)" />
              </span>
              <span v-else class="smc-session-dot" :class="dotClass(s)"></span>
              <input
                v-if="editingId === s.id"
                ref="renameInputRef"
                v-model="editingValue"
                class="smc-name-input"
                @click.stop
                @keydown.enter="commitRename"
                @keydown.esc="cancelRename"
                @blur="commitRename"
              />
              <Transition name="smc-title-swap" mode="out-in"><span v-if="editingId !== s.id" :key="s.name" class="smc-session-name">{{ s.name }}</span></Transition>
              <div v-if="!bulkMode && editingId !== s.id && (hoveredId === s.id || openMenuId === s.id)" class="smc-row-menu-wrap">
                <button class="smc-row-menu-btn" @click.stop="toggleMenu(s, $event)" title="更多">
                  <Icon icon="mdi:dots-horizontal" width="16" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 项目：按工作目录分组 -->
      <div class="smc-section">
        <div class="smc-section-label">
          <span>项目</span>
          <button class="smc-project-bulk" type="button" :title="bulkMode ? '退出批量管理' : '批量管理会话'" :class="{ active: bulkMode }" @click="toggleBulkMode">
            <Icon icon="mdi:playlist-edit" width="18" />
          </button>
          <button class="smc-project-add" type="button" title="创建项目" @click="openCreateProject">
            <Icon icon="mdi:plus" width="18" />
          </button>
        </div>
        <div v-for="grp in taskGroups" :key="'wd_' + grp.name" class="smc-folder">
          <div class="smc-folder-head" @click="toggleGroup(grp.name)">
            <span v-if="bulkMode" class="smc-bulk-check smc-group-check" @click.stop="toggleGroupSelect(grp.name)">
              <Icon :icon="groupAllSelected(grp.name) ? 'mdi:checkbox-marked' : 'mdi:checkbox-blank-outline'" width="16" color="var(--app-accent)" />
            </span>
            <span class="smc-folder-chevron" :class="{ open: isGroupOpen(grp.name) }">›</span>
            <Icon icon="mdi:folder-outline" width="15" color="var(--app-accent)" />
            <span class="smc-folder-name">{{ grp.name }}</span>
            <button
              v-if="projectForName(grp.name) && !bulkMode"
              class="smc-project-new-chat"
              type="button"
              title="在此项目中新建对话"
              aria-label="在此项目中新建对话"
              @click.stop="newSessionForProject(grp.name)"
            >
              <Icon icon="mdi:message-plus-outline" width="17" />
            </button>
            <button v-if="bulkMode" class="smc-group-delete" type="button" title="删除项目（含其下所有会话）" @click.stop="onDeleteProject(grp.name)">
              <Icon icon="mdi:trash-can-outline" width="15" />
            </button>
          </div>
          <div v-if="isGroupOpen(grp.name)" class="smc-folder-children">
            <div
              v-for="s in grp.sessions"
              :key="s.id"
              class="smc-session-row"
              :class="{ active: s.id === activeSession, running: s.id === runningSession }"
              @mouseenter="hoveredId = s.id"
              @mouseleave="onRowLeave(s.id)"
              @click="onRowClick(s)"
            >
                          <RunningRing
                            v-if="s.id === runningSession"
                            class="smc-running-ring"
                          />
                          <span v-if="bulkMode" class="smc-bulk-check" @click.stop="toggleBulkSelect(s)">
                <Icon :icon="bulkSelected.has(s.id) ? 'mdi:checkbox-marked' : 'mdi:checkbox-blank-outline'" width="16" color="var(--app-accent)" />
              </span>
              <span v-else class="smc-session-dot" :class="dotClass(s)"></span>
              <input
                v-if="editingId === s.id"
                ref="renameInputRef"
                v-model="editingValue"
                class="smc-name-input"
                @click.stop
                @keydown.enter="commitRename"
                @keydown.esc="cancelRename"
                @blur="commitRename"
              />
              <Transition name="smc-title-swap" mode="out-in"><span v-if="editingId !== s.id" :key="s.name" class="smc-session-name">{{ s.name }}</span></Transition>
              <div v-if="!bulkMode && editingId !== s.id && (hoveredId === s.id || openMenuId === s.id)" class="smc-row-menu-wrap">
                <button class="smc-row-menu-btn" @click.stop="toggleMenu(s, $event)" title="更多">
                  <Icon icon="mdi:dots-horizontal" width="16" />
                </button>
              </div>
            </div>
          </div>
        </div>
        <!-- 未分组 -->
        <div v-if="orphanSessions.length" class="smc-folder">
          <div class="smc-folder-head" @click="toggleOrphan">
            <span class="smc-folder-chevron" :class="{ open: showOrphan }">›</span>
            <Icon icon="mdi:folder-outline" width="15" color="var(--app-text-faint)" />
            <span class="smc-folder-name" style="color:var(--app-text-faint)">未分组</span>
          </div>
          <div v-if="showOrphan" class="smc-folder-children">
            <div
              v-for="s in orphanSessions"
              :key="s.id"
              class="smc-session-row"
              :class="{ active: s.id === activeSession, running: s.id === runningSession }"
              @mouseenter="hoveredId = s.id"
              @mouseleave="onRowLeave(s.id)"
              @click="onRowClick(s)"
            >
                          <RunningRing
                            v-if="s.id === runningSession"
                            class="smc-running-ring"
                          />
                          <span v-if="bulkMode" class="smc-bulk-check" @click.stop="toggleBulkSelect(s)">
                <Icon :icon="bulkSelected.has(s.id) ? 'mdi:checkbox-marked' : 'mdi:checkbox-blank-outline'" width="16" color="var(--app-accent)" />
              </span>
              <span v-else class="smc-session-dot" :class="dotClass(s)"></span>
              <input
                v-if="editingId === s.id"
                ref="renameInputRef"
                v-model="editingValue"
                class="smc-name-input"
                @click.stop
                @keydown.enter="commitRename"
                @keydown.esc="cancelRename"
                @blur="commitRename"
              />
              <Transition name="smc-title-swap" mode="out-in"><span v-if="editingId !== s.id" :key="s.name" class="smc-session-name">{{ s.name }}</span></Transition>
              <div v-if="!bulkMode && editingId !== s.id && (hoveredId === s.id || openMenuId === s.id)" class="smc-row-menu-wrap">
                <button class="smc-row-menu-btn" @click.stop="toggleMenu(s, $event)" title="更多">
                  <Icon icon="mdi:dots-horizontal" width="16" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

    </div>

    <!-- 批量管理操作条 -->
    <div v-if="bulkMode" class="smc-bulk-bar">
      <button class="smc-bulk-action" type="button" @click="toggleSelectAllBulk">
        {{ allBulkSelected ? '取消全选' : '全选' }}
      </button>
      <span class="smc-bulk-count">{{ bulkSelected.size }} 个已选</span>
      <button class="smc-bulk-action danger" type="button" :disabled="!bulkSelected.size" @click="onBulkDelete">删除</button>
      <button class="smc-bulk-action" type="button" @click="toggleBulkMode">完成</button>
    </div>

    <!-- footer -->
    <div class="fm-footer" ref="footerRef">
      <div class="fm-user" ref="userRef" @click.stop="toggleUserMenu" title="点击查看账户">
        <img v-if="auth.displayAvatar.value" :src="auth.displayAvatar.value" class="fm-user-avatar" alt="avatar" />
        <span v-else class="fm-user-avatar fm-user-avatar-fallback">{{ avatarFallback }}</span>
        <span class="fm-user-id">
          <span class="fm-user-name">{{ auth.isLoggedIn.value ? auth.displayName.value : '未登录' }}</span>
        </span>
      </div>
      <button class="fm-footer-mail" type="button" title="通知" @click.stop="$emit('open-mail')">
              <Icon icon="mdi:email-outline" width="18" />
              <span v-if="notifCount > 0" class="fm-mail-badge">{{ notifCount > 99 ? '99+' : notifCount }}</span>
            </button>
      <button class="fm-footer-settings" type="button" title="设置" @click.stop="$emit('open-settings')">
        <Icon icon="mdi:cog-outline" width="18" />
      </button>
    </div>

    <!-- 用户卡片菜单 -->
    <Teleport to="body">
      <template v-if="showUserMenu">
        <div class="smc-card-backdrop" @click="showUserMenu = false"></div>
        <div ref="userCardRef" class="smc-user-card is-profile" :style="userMenuStyle" @click.stop>
          <header class="smc-profile-hero">
            <div class="smc-profile-aura" aria-hidden="true"></div>
            <div class="smc-avatar-shell">
              <img v-if="auth.displayAvatar.value" :src="auth.displayAvatar.value" class="smc-user-avatar" alt="avatar" />
              <span v-else class="smc-user-avatar-fallback">{{ avatarFallback }}</span>
              <i class="smc-online-dot" title="在线"></i>
            </div>
            <div class="smc-user-card-name">
              <span class="smc-profile-kicker">RESCENE IDENTITY</span>
              <strong>{{ auth.name.value || auth.login.value || '本地访客' }}</strong>
              <div class="smc-profile-meta">
                <span v-if="auth.uid.value">UID {{ auth.uid.value }}</span>
                <span>本地 AI 档案</span>
              </div>
            </div>
            <button class="smc-profile-share" type="button" aria-label="分享角色卡" @click="shareCard"><Icon icon="mdi:share-variant-outline" width="18" /></button>
            <button class="smc-profile-close" type="button" aria-label="关闭角色卡" @click="showUserMenu = false"><Icon icon="mdi:close" width="18" /></button>
          </header>
          <div v-if="auth.authError.value" class="smc-auth-warn"><Icon icon="mdi:cloud-alert-outline" width="15" />{{ auth.authError.value }}</div>
          <div v-if="evolve" class="smc-evolve">
            <div class="smc-evolve-summary">
              <div>
                <span class="smc-evolve-stage">{{ evolve.stage }}</span>
                <span class="smc-evolve-lv" :class="{ bump: lvBump }">LV {{ evolve.level || 1 }}</span>
              </div>
              <strong>{{ evolveNum().xp }} <small>XP</small></strong>
            </div>
            <div class="smc-xp-track" role="progressbar" :aria-valuenow="xpProgress" aria-valuemin="0" aria-valuemax="100">
              <i :style="{ width: xpProgress + '%' }"></i>
            </div>
            <div class="smc-xp-copy"><span>本级进度 {{ xpProgress }}%</span><span>距 LV {{ (evolve.level || 1) + 1 }} 还需 {{ xpRemaining }} XP</span></div>

            <div class="smc-profile-dashboard">
              <div class="smc-radar-wrap">
                <span class="smc-panel-label">能力轮廓</span>
                <svg viewBox="0 0 200 200" class="smc-radar">
                  <polygon :points="evoPoints(1)" class="smc-radar-grid smc-radar-grid-1" />
                  <polygon :points="evoPoints(0.66)" class="smc-radar-grid smc-radar-grid-2" />
                  <polygon :points="evoPoints(0.33)" class="smc-radar-grid smc-radar-grid-3" />
                  <polygon :points="evoHexPoints()" class="smc-radar-fill" />
                  <g v-for="(a, i) in EVO_AXES" :key="a.label">
                    <line :x1="100" :y1="100" :x2="100 + Math.cos(labelAng(i)) * 82" :y2="100 - Math.sin(labelAng(i)) * 82" :stroke="a.color" stroke-opacity="0.4" stroke-width="1.4" class="smc-radar-line" />
                    <circle :cx="evoPoint(i, evolveNum()[a.key]).x" :cy="evoPoint(i, evolveNum()[a.key]).y" r="3.2" :fill="a.color" stroke="#fff" stroke-width="1.4" class="smc-radar-point" />
                    <text :x="100 + Math.cos(labelAng(i)) * 93" :y="100 - Math.sin(labelAng(i)) * 93" class="smc-radar-label" text-anchor="middle" dominant-baseline="middle">{{ a.label }}</text>
                  </g>
                </svg>
              </div>
              <div class="smc-ability-list">
                <span class="smc-panel-label">实时能力值</span>
                <div v-for="a in EVO_AXES" :key="a.key" class="smc-ability-row">
                  <div><i :style="{ background: a.color }"></i><span>{{ a.label }}</span><b>{{ evolveNum()[a.key] || 0 }}<small v-if="a.key === 'success'">%</small></b></div>
                  <span class="smc-ability-track"><i :style="{ width: Math.max(2, evolveNum()[a.key] || 0) + '%', background: a.color }"></i></span>
                </div>
              </div>
            </div>

            <div class="smc-career-stats">
              <div><Icon icon="mdi:file-check-outline" width="17" /><span><b>{{ evolve.outputs || 0 }}</b><small>真实产出</small></span></div>
              <div><Icon icon="mdi:puzzle-outline" width="17" /><span><b>{{ evolve.skills || 0 }}</b><small>已学技能</small></span></div>
              <div><Icon icon="mdi:brain" width="17" /><span><b>{{ evolve.memories || 0 }}</b><small>记忆沉淀</small></span></div>
              <div><Icon icon="mdi:chart-timeline-variant-shimmer" width="17" /><span><b>{{ evolve.refines || 0 }}</b><small>自我迭代</small></span></div>
            </div>
          </div>
          <div v-else class="smc-profile-loading"><i></i><span>正在读取成长档案…</span></div>
          <footer class="smc-profile-actions">
            <span><Icon icon="mdi:shield-check-outline" width="15" /> 数据来自真实使用记录</span>
            <button v-if="isLoggedIn" class="smc-user-card-item danger" @click="logout"><Icon icon="mdi:logout-variant" width="16" />退出登录</button>
          </footer>
          <div v-if="!isLoggedIn" class="smc-login-panel">
          <div class="smc-rc-login">
            <input v-model="rcUser" class="smc-rc-input" :placeholder="rcMode === 'login' ? 'Rescene Cloud 账号' : '用户名（3-32 字符）'" @keyup.enter="rcMode === 'login' ? loginResceneCloud() : registerResceneCloud()" />
            <input v-model="rcPwd" type="password" class="smc-rc-input" :placeholder="rcMode === 'login' ? '密码' : '密码（6-64 字符）'" @keyup.enter="rcMode === 'login' ? loginResceneCloud() : registerResceneCloud()" />
            <button class="smc-rc-btn" :disabled="rcLoading" @click="rcMode === 'login' ? loginResceneCloud() : registerResceneCloud()">{{ rcLoading ? '处理中…' : (rcMode === 'login' ? '登录' : '注册') }}</button>
            <div v-if="rcError" class="smc-rc-err">{{ rcError }}</div>
            <div class="smc-rc-hint">
              <template v-if="rcMode === 'login'">没有账号？<a class="smc-rc-link" @click="rcMode = 'register'; rcError = ''">注册一个</a></template>
              <template v-else>已有账号？<a class="smc-rc-link" @click="rcMode = 'login'; rcError = ''">去登录</a></template>
            </div>
          </div>
          </div>
      </div>
      </template>
    </Teleport>

    <!-- 会话三点菜单 -->
    <Teleport to="body">
      <div v-if="openMenuId" class="smc-row-dropdown" :style="dropdownStyle" @click.stop>
        <div class="smc-dropdown-item" @click="startRename(openMenuSession)">重命名</div>
        <div class="smc-dropdown-item danger" @click="onDelete(openMenuSession)">删除</div>
      </div>
    </Teleport>

    <!-- 创建项目：项目名不可编辑，直接取自所选文件夹名，避免和已有项目撞名后互相顶替 -->
    <Teleport to="body">
      <Transition name="smc-modal">
        <div v-if="showCreateProject" class="smc-modal-backdrop" @click.self="closeCreateProject">
          <form class="smc-create-project" @submit.prevent="createProject">
            <div class="smc-create-project-head">
              <h2>创建项目</h2>
              <button type="button" class="smc-modal-close" title="关闭" @click="closeCreateProject">
                <Icon icon="mdi:close" width="20" />
              </button>
            </div>
            <div class="smc-source-label">源文件夹（项目名称自动取文件夹名）</div>
            <button type="button" class="smc-source-picker" @click="pickSourceFolder">
              <Icon icon="mdi:folder-plus-outline" width="25" />
              <span v-if="selectedSourceFolder">{{ selectedSourceFolder.name }}</span>
              <span v-else>添加可读取和编辑的文件夹</span>
            </button>
            <p v-if="sourcePickerError" class="smc-source-picker-error" role="alert">{{ sourcePickerError }}</p>
            <label v-if="recentProjectOptions.length" class="smc-recent-project-picker">
              <span>最近选择项目</span>
              <select v-model="recentProjectPath" @change="pickRecentProject">
                <option value="" disabled>选择一个最近项目</option>
                <option v-for="project in recentProjectOptions" :key="project.path" :value="project.path">
                  {{ project.name }}
                </option>
              </select>
            </label>
            <div class="smc-create-project-actions">
              <button type="button" class="smc-cancel-btn" @click="closeCreateProject">取消</button>
              <button type="submit" class="smc-create-btn" :disabled="!selectedSourceFolder">创建项目</button>
            </div>
          </form>
        </div>
      </Transition>
    </Teleport>

    <!-- 新建会话但当前没有选中项目：像 Claude Code 一样强制先选项目，选中已有项目后不再弹出 -->
    <Teleport to="body">
      <Transition name="smc-modal">
        <div v-if="showSelectProject" class="smc-modal-backdrop" @click.self="closeSelectProject">
          <div class="smc-create-project">
            <div class="smc-create-project-head">
              <h2>选择项目</h2>
              <button type="button" class="smc-modal-close" title="关闭" @click="closeSelectProject">
                <Icon icon="mdi:close" width="20" />
              </button>
            </div>
            <div class="smc-source-label">新对话需要先归属到一个项目</div>
            <div v-if="projects.length" class="smc-select-project-list">
              <button
                v-for="p in projects"
                :key="p.path"
                type="button"
                class="smc-source-picker"
                @click="pickExistingProject(p)"
              >
                <Icon icon="mdi:folder-outline" width="20" />
                <span>{{ p.name }}</span>
              </button>
            </div>
            <div class="smc-create-project-actions">
              <button type="button" class="smc-cancel-btn" @click="closeSelectProject">取消</button>
              <button type="button" class="smc-create-btn" @click="openCreateProjectForSession">新建项目</button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, reactive, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import RunningRing from './RunningRing.vue'
import { useAuth } from '../../../composables/useAuth.js'
import { computeHardwareFingerprint } from '../../../utils/hardwareFingerprint.js'

const auth = useAuth()
const avatarFallback = computed(() => {
  const label = String(auth.displayName.value || 'Rescene').trim()
  return (label.charAt(0) || 'R').toUpperCase()
})

// Rescene Cloud 账号登录在下方 rc-login 面板（用户名+密码），登录态经 useAuth 统一管理。

// UID 与亲密等级已移到「设置 → 我的」tab 展示，侧栏 footer 只保留头像和名字。

const PIN_KEY = 'shanxi_pinned_projects'

const props = defineProps({
  sessions: { type: Array, default: () => [] },
  projects: { type: Array, default: () => [] },
  recentProjects: { type: Array, default: () => [] },
  activeSession: { type: String, default: '' },
  runningSession: { type: String, default: '' },
  completedSessions: { type: Set, default: () => new Set() },
  questionSession: { type: String, default: '' },
    fill: { type: Boolean, default: false },
    notifCount: { type: Number, default: 0 },
    currentWorkdir: { type: String, default: '' }
  })
const emit = defineEmits(['select-session', 'new-session', 'rename-session', 'delete-session', 'delete-sessions', 'delete-project', 'open-settings', 'open-search', 'open-plugins', 'create-project', 'open-scheduled-tasks', 'open-mail'])

// ========== 创建项目 ==========
// 项目名不可编辑：只能来自所选文件夹的名字，避免用户手改后跟已有项目撞名、
// 导致 rememberProject 按名字去重时把旧项目的会话"过继"给新项目。
const showCreateProject = ref(false)
const selectedSourceFolder = ref(null)
const sourcePickerError = ref('')
const recentProjectPath = ref('')
const recentProjectOptions = computed(() => {
  const seen = new Set()
  return props.recentProjects.filter(project => {
    const path = project?.path
    if (!project?.name || !path || seen.has(path)) return false
    seen.add(path)
    return true
  })
})
// 从"选择项目"弹窗里点了"新建项目"进来的，创建完要接着建会话
const pendingSessionAfterCreate = ref(false)

function openCreateProject() {
  showCreateProject.value = true
  selectedSourceFolder.value = null
  sourcePickerError.value = ''
  recentProjectPath.value = ''
}
function openCreateProjectForSession() {
  pendingSessionAfterCreate.value = true
  showSelectProject.value = false
  openCreateProject()
}
function closeCreateProject() {
  showCreateProject.value = false
  pendingSessionAfterCreate.value = false
}
async function pickSourceFolder() {
  sourcePickerError.value = ''
  try {
    const res = await fetch('/api/workdir/pick', { method: 'POST' })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || '无法打开文件夹选择器')
    if (!data.cancelled && data.path) selectedSourceFolder.value = { name: data.name || data.path, path: data.path }
    else if (!data.cancelled) throw new Error('未收到所选文件夹路径')
  } catch (error) {
    sourcePickerError.value = error?.message || '无法打开文件夹选择器'
  }
}
function pickRecentProject() {
  const project = recentProjectOptions.value.find(item => item.path === recentProjectPath.value)
  if (!project) return
  selectedSourceFolder.value = { name: project.name, path: project.path }
  sourcePickerError.value = ''
}
function createProject() {
  const folder = selectedSourceFolder.value
  if (!folder?.path) return
  emit('create-project', { name: folder.name, sourceFolder: folder, thenNewSession: pendingSessionAfterCreate.value })
  pendingSessionAfterCreate.value = false
  showCreateProject.value = false
}
defineExpose({ openCreateProject })

// ========== 新建会话：没有已选项目时强制先选项目 ==========
const showSelectProject = ref(false)
function onClickNewSession() {
  if (props.currentWorkdir?.trim()) emit('new-session')
  else showSelectProject.value = true
}
function closeSelectProject() { showSelectProject.value = false }
function pickExistingProject(project) {
  showSelectProject.value = false
  emit('new-session', project)
}
function projectForName(name) {
  return props.projects.find(project => project?.name === name && project?.path) || null
}
function newSessionForProject(name) {
  const project = projectForName(name)
  if (project) emit('new-session', project)
}

// ========== 置顶项目 ==========
const pinnedProjectNames = ref([])
function loadPinned() {
  try { pinnedProjectNames.value = JSON.parse(localStorage.getItem(PIN_KEY) || '[]') } catch { pinnedProjectNames.value = [] }
}
function savePinned() {
  try { localStorage.setItem(PIN_KEY, JSON.stringify(pinnedProjectNames.value)) } catch {}
}
function isPinned(name) { return pinnedProjectNames.value.includes(name) }
function togglePinFolder(name) {
  if (isPinned(name)) pinnedProjectNames.value = pinnedProjectNames.value.filter(f => f !== name)
  else pinnedProjectNames.value.push(name)
  savePinned()
}

// ========== 文件夹展开 ==========
const expandedPinned = reactive({})
const expandedGroups = reactive({})
const showOrphan = ref(true)

function togglePinnedFolder(name) { expandedPinned[name] = !expandedPinned[name] }
function isGroupOpen(name) {
  if (expandedGroups[name] !== undefined) return expandedGroups[name]
  return true
}
function toggleGroup(name) { expandedGroups[name] = !isGroupOpen(name) }
function toggleOrphan() { showOrphan.value = !showOrphan.value }

// ========== 按 workdir 分组 ==========
const workdirMap = computed(() => {
  const map = new Map()
  // 项目是独立实体，不能只靠“恰好有会话”来反推；新建项目即使暂时没有
  // 会话也必须立刻出现在侧栏。
  for (const project of props.projects) {
    const name = project?.name?.trim()
    if (name && !map.has(name)) map.set(name, [])
  }
  for (const s of props.sessions) {
    const wd = s.workdir || ''
    if (!wd) continue
    if (!map.has(wd)) map.set(wd, [])
    map.get(wd).push(s)
  }
  return map
})

const taskGroups = computed(() => {
  const map = workdirMap.value
  return Array.from(map.entries())
    .sort((a, b) => {
      const aP = isPinned(a[0]) ? 0 : 1
      const bP = isPinned(b[0]) ? 0 : 1
      if (aP !== bP) return aP - bP
      return b[1].length - a[1].length
    })
    .map(([name, sessions]) => ({ name, sessions }))
})

const pinnedFolders = computed(() => {
  const map = workdirMap.value
  return pinnedProjectNames.value
    .filter(name => map.has(name))
    .map(name => ({ name, sessions: map.get(name) }))
})

const orphanSessions = computed(() => {
  const have = new Set()
  for (const [, sess] of workdirMap.value) {
    for (const s of sess) have.add(s.id)
  }
  return props.sessions.filter(s => !have.has(s.id))
})

// ========== 用户卡片 ==========
const footerRef = ref(null)
const userRef = ref(null)
const userCardRef = ref(null)
const showUserMenu = ref(false)
const userMenuStyle = ref({})
const isLoggedIn = auth.isLoggedIn
function refreshLoginState() { auth.refresh() }

// ===== 进化雷达（聊天界面看 AI 成长，真实数据 /api/evolve/me）=====
const evolve = ref(null)
// 经验增长动画：evolveAnim 从 0 滚动逼近真实值（XP 数字滚动 + 雷达从中心展开）
const evolveAnim = ref(null)
let evolveRaf = 0
const lvBump = ref(false)  // XP 到达后 Lv 跳动
function startEvolveAnim(target) {
  cancelAnimationFrame(evolveRaf)
  const keys = ['output', 'skill', 'collab', 'memory', 'success', 'intimacy', 'xp']
  const t0 = performance.now()
  const DUR = 1600 // 1.6s 成长动画
  function tick(t) {
    const p = Math.min(1, (t - t0) / DUR)
    const e = 1 - Math.pow(1 - p, 3) // easeOutCubic：先快后慢，像升级冲刺
    const cur = {}
    for (const k of keys) cur[k] = Math.round((target[k] || 0) * e)
    if (p < 1) {
      evolveAnim.value = cur
      evolveRaf = requestAnimationFrame(tick)
    } else {
          evolveAnim.value = { ...target }
          // 经验到位，Lv 徽章跳一下
          lvBump.value = true
          setTimeout(() => { lvBump.value = false }, 900)
        }
  }
  evolveRaf = requestAnimationFrame(tick)
}
const EVO_AXES = [
  { label: '产出', key: 'output', color: '#f59e0b' },
  { label: '技能', key: 'skill', color: '#6366f1' },
  { label: '协作', key: 'collab', color: '#0ea5e9' },
  { label: '记忆', key: 'memory', color: '#8b5cf6' },
  { label: '成功率', key: 'success', color: '#10b981' },
  { label: '亲密度', key: 'intimacy', color: '#ec4899' },
]
const xpLevelStart = computed(() => Math.max(0, ((evolve.value?.level || 1) * ((evolve.value?.level || 1) - 1) / 2) * 100))
const xpLevelEnd = computed(() => (((evolve.value?.level || 1) * ((evolve.value?.level || 1) + 1) / 2) * 100))
const xpProgress = computed(() => Math.max(0, Math.min(100, Math.round(((evolve.value?.xp || 0) - xpLevelStart.value) / Math.max(1, xpLevelEnd.value - xpLevelStart.value) * 100))))
const xpRemaining = computed(() => Math.max(0, xpLevelEnd.value - (evolve.value?.xp || 0)))
function labelAng(i) { return Math.PI / 2 - i * 2 * Math.PI / EVO_AXES.length }
function evoPoint(i, val) {
  const ang = labelAng(i)
  const v = Math.max(0, Math.min(100, val || 0)) / 100 * 78
  return { x: 100 + Math.cos(ang) * v, y: 100 - Math.sin(ang) * v }
}
function evoPoints(pct) {
  return EVO_AXES.map((_, i) => { const p = evoPoint(i, pct * 100); return p.x.toFixed(1) + ',' + p.y.toFixed(1) }).join(' ')
}
function evoHexPoints() {
  // 动画期间用滚动值，动画结束用真实值
  const src = evolveAnim.value || evolve.value
  return EVO_AXES.map((a, i) => { const p = evoPoint(i, src ? src[a.key] : 0); return p.x.toFixed(1) + ',' + p.y.toFixed(1) }).join(' ')
}
function evolveNum() { return evolveAnim.value || evolve.value || {} }
async function loadEvolve() {
  try {
    const r = await fetch('/api/evolve/me')
    if (r.ok) {
      const data = await r.json()
      evolve.value = data
      startEvolveAnim(data)
    }
  } catch (e) { /* 后端没起就静默，不打扰聊天 */ }
}

// ===== 分享角色卡：canvas 生成成长档案图并下载 =====
function shareCard() {
  if (!evolve.value) return
  const ev = evolve.value
  const lv = ev.level || 1
  const stage = ev.stage || '微光'
  const name = auth.name.value || auth.login.value || '本地访客'
  const W = 720, H = 640
  const c = document.createElement('canvas')
  c.width = W; c.height = H
  const g = c.getContext('2d')
  // 顶栏
  const grd = g.createLinearGradient(0, 0, W, 0)
  grd.addColorStop(0, '#111827'); grd.addColorStop(0.55, '#1e293b'); grd.addColorStop(1, '#172554')
  g.fillStyle = grd; g.fillRect(0, 0, W, 150)
  g.fillStyle = '#93c5fd'; g.font = '700 13px sans-serif'; g.fillText('RESCENE IDENTITY', 36, 44)
  g.fillStyle = '#ffffff'; g.font = '800 30px sans-serif'; g.fillText(name, 36, 84)
  g.fillStyle = '#cbd5e1'; g.font = '600 14px sans-serif'
  g.fillText(auth.uid.value ? 'UID ' + auth.uid.value : '本地 AI 档案', 36, 112)
  g.textAlign = 'right'
  g.fillStyle = '#a5b4fc'; g.font = '800 22px sans-serif'; g.fillText(stage, W - 36, 66)
  g.fillStyle = '#eef2ff'; g.font = '800 42px sans-serif'; g.fillText('LV ' + lv, W - 36, 114)
  g.textAlign = 'left'
  // 雷达
  const cx = W / 2, cy = 340, R = 140
  const pts = (i, r) => { const ang = Math.PI / 2 - i * 2 * Math.PI / 6; return [cx + Math.cos(ang) * r, cy - Math.sin(ang) * r] }
  for (const pct of [1, 0.66, 0.33]) {
    g.strokeStyle = '#dbe3ef'; g.lineWidth = 1; g.beginPath()
    for (let i = 0; i <= 6; i++) { const [x, y] = pts(i % 6, R * pct); i === 0 ? g.moveTo(x, y) : g.lineTo(x, y) }
    g.stroke()
  }
  EVO_AXES.forEach((a, i) => {
    const val = Math.max(0, Math.min(100, ev[a.key] || 0))
    const [ex, ey] = pts(i, R)
    const [px, py] = pts(i, R * val / 100)
    g.strokeStyle = a.color; g.globalAlpha = 0.35; g.lineWidth = 1.6
    g.beginPath(); g.moveTo(cx, cy); g.lineTo(ex, ey); g.stroke()
    g.globalAlpha = 1
    g.fillStyle = a.color
    g.beginPath(); g.arc(px, py, 5, 0, Math.PI * 2); g.fill()
    g.strokeStyle = '#fff'; g.lineWidth = 2; g.stroke()
    g.fillStyle = '#64748b'; g.font = '700 15px sans-serif'; g.textAlign = 'center'
    g.fillText(a.label, cx + Math.cos(Math.PI / 2 - i * 2 * Math.PI / 6) * (R + 26), cy - Math.sin(Math.PI / 2 - i * 2 * Math.PI / 6) * (R + 26) + 5)
    g.textAlign = 'left'
  })
  g.beginPath()
  EVO_AXES.forEach((a, i) => {
    const val = Math.max(0, Math.min(100, ev[a.key] || 0))
    const [x, y] = pts(i, R * val / 100)
    i === 0 ? g.moveTo(x, y) : g.lineTo(x, y)
  })
  g.closePath()
  g.fillStyle = 'rgba(99,102,241,0.14)'; g.fill()
  g.strokeStyle = '#6366f1'; g.lineWidth = 2; g.stroke()
  // 底部数值卡 3x2
  const cardW = 200, cardH = 46, gap = 14, x0 = (W - cardW * 3 - gap * 2) / 2, y0 = 512
  EVO_AXES.forEach((a, i) => {
    const col = i % 3, row = Math.floor(i / 3)
    const x = x0 + col * (cardW + gap), y = y0 + row * (cardH + 10)
    g.fillStyle = '#f8fafc'; g.strokeStyle = '#e2e8f0'; g.lineWidth = 1
    g.beginPath(); g.roundRect ? g.roundRect(x, y, cardW, cardH, 10) : g.rect(x, y, cardW, cardH); g.fill(); g.stroke()
    g.fillStyle = a.color
    g.fillRect(x + 14, y + cardH / 2 - 3.5, 7, 7)
    g.fillStyle = '#64748b'; g.font = '600 13px sans-serif'; g.fillText(a.label, x + 28, y + 19)
    g.fillStyle = '#1e293b'; g.font = '800 20px sans-serif'; g.textAlign = 'right'
    g.fillText(String(ev[a.key] || 0) + (a.key === 'success' ? '%' : ''), x + cardW - 14, y + 29)
    g.textAlign = 'left'
  })
  // 页脚
  g.fillStyle = '#94a3b8'; g.font = '600 12px sans-serif'; g.textAlign = 'center'
  g.fillText('数据来自真实使用记录 · Rescene', W / 2, H - 18)
  g.textAlign = 'left'
  // 下载
  const a = document.createElement('a')
  a.download = '成长档案-LV' + lv + '-' + stage + '.png'
  a.href = c.toDataURL('image/png')
  a.click()
}

// ===== Rescene Cloud 账号登录/注册（国内无需 GitHub/代理）=====
// 开放注册（2026-08-17 用户定稿：邀请码已废弃），注册即签发 JWT 直接登录。
const rcUser = ref('')
const rcPwd = ref('')
const rcLoading = ref(false)
const rcError = ref('')
const rcMode = ref('login') // 'login' | 'register'

// 登录/注册成功后广播「欢迎回来」：App.vue 顶部轻量横幅监听这个事件来显示，
// 和更新完成横幅同一套样式/时序（2026-08-20 用户定稿）。用输入框里的用户名即可——
// Rescene Cloud 账号登录的 login/name 就是这个用户名（account.go signAndReturn），
// 不用等 refresh() 异步拉回 /api/auth/me 才能拿到显示名。
function announceWelcome(username) {
  window.dispatchEvent(new CustomEvent('auth-welcome', { detail: { name: username } }))
}

async function loginResceneCloud() {
  rcError.value = ''
  if (!rcUser.value.trim() || !rcPwd.value) { rcError.value = '请输入账号和密码'; return }
  rcLoading.value = true
  try {
    const username = rcUser.value.trim()
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password: rcPwd.value, fingerprint: computeHardwareFingerprint() })
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok || !data.token) { rcError.value = data.error || '登录失败，请检查账号密码'; return }
    localStorage.setItem('token', data.token)
    window.dispatchEvent(new Event('auth-change'))
    announceWelcome(username)
    rcUser.value = ''; rcPwd.value = ''
    showUserMenu.value = false
  } catch (e) { rcError.value = '网络错误，请稍后再试' } finally { rcLoading.value = false }
}

async function registerResceneCloud() {
  rcError.value = ''
  if (!rcUser.value.trim() || !rcPwd.value) { rcError.value = '请输入用户名和密码'; return }
  rcLoading.value = true
  try {
    const username = rcUser.value.trim()
    const res = await fetch('/api/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password: rcPwd.value, fingerprint: computeHardwareFingerprint() })
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok || !data.token) { rcError.value = data.error || '注册失败，请稍后再试'; return }
    localStorage.setItem('token', data.token)
    window.dispatchEvent(new Event('auth-change'))
    announceWelcome(username)
    rcUser.value = ''; rcPwd.value = ''
    rcMode.value = 'login'
    showUserMenu.value = false
  } catch (e) { rcError.value = '网络错误，请稍后再试' } finally { rcLoading.value = false }
}

function toggleUserMenu() {
  if (showUserMenu.value) { showUserMenu.value = false; return }
  showUserMenu.value = true
  loadEvolve()
  refreshLoginState()
  // 居中弹窗：位置交给 CSS（.smc-user-card fixed 居中），无需按触发点计算
  userMenuStyle.value = {}
}
function logout() {
  auth.logout()
  showUserMenu.value = false
  refreshLoginState()
}

// ========== 会话交互 ==========
const hoveredId = ref(null)
const openMenuId = ref(null)
const openMenuSession = ref(null)
const dropdownStyle = ref({})
const editingId = ref(null)
const editingValue = ref('')
const renameInputRef = ref(null)

function dotClass(s) {
  if (s.id === props.runningSession) return 'running'
  if (props.completedSessions.has(s.id)) return 'completed'
  if (s.id === props.questionSession) return 'question'
  return ''
}

function toggleMenu(s, ev) {
  if (openMenuId.value === s.id) { openMenuId.value = null; return }
  openMenuId.value = s.id
  openMenuSession.value = s
  const rect = ev.currentTarget.getBoundingClientRect()
  const menuW = 140
  let left = rect.right - menuW
  let top = rect.bottom + 6
  if (top + 116 > window.innerHeight) top = rect.top - 116 - 6
  if (left < 8) left = 8
  dropdownStyle.value = { position: 'fixed', left: left + 'px', top: top + 'px', width: menuW + 'px' }
}
function onRowLeave(id) { if (openMenuId.value !== id) hoveredId.value = null }
function onRowClick(s) {
  if (editingId.value === s.id) return
  if (bulkMode.value) { toggleBulkSelect(s); return }
  emit('select-session', s.id)
}
function startRename(s) {
  openMenuId.value = null
  editingId.value = s.id
  editingValue.value = s.name
  nextTick(() => {
    const el = Array.isArray(renameInputRef.value) ? renameInputRef.value[0] : renameInputRef.value
    el?.focus(); el?.select()
  })
}
function commitRename() {
  if (editingId.value) {
    const name = editingValue.value.trim()
    if (name) emit('rename-session', { id: editingId.value, name })
  }
  editingId.value = null
}
function cancelRename() { editingId.value = null }
function onDelete(s) {
  openMenuId.value = null
  hoveredId.value = null
  emit('delete-session', s.id)
}

// ========== 批量管理 ==========
const bulkMode = ref(false)
const bulkSelected = ref(new Set())

function toggleBulkMode() {
  bulkMode.value = !bulkMode.value
  bulkSelected.value = new Set()
  openMenuId.value = null
  hoveredId.value = null
  editingId.value = null
}
function toggleBulkSelect(s) {
  const set = new Set(bulkSelected.value)
  if (set.has(s.id)) set.delete(s.id)
  else set.add(s.id)
  bulkSelected.value = set
}
const allBulkSelected = computed(() => props.sessions.length > 0 && bulkSelected.value.size === props.sessions.length)
function toggleSelectAllBulk() {
  bulkSelected.value = allBulkSelected.value ? new Set() : new Set(props.sessions.map(s => s.id))
}
function onBulkDelete() {
  const ids = [...bulkSelected.value]
  if (!ids.length) return
  emit('delete-sessions', ids)
  toggleBulkMode()
}
// 项目级批量选择：勾选/取消该项目下的全部会话（配合批量删除删会话）
function toggleGroupSelect(name) {
  const grp = taskGroups.value.find(g => g.name === name)
  if (!grp || !grp.sessions.length) return
  const ids = grp.sessions.map(s => s.id)
  const set = new Set(bulkSelected.value)
  if (groupAllSelected(name)) ids.forEach(id => set.delete(id))
  else ids.forEach(id => set.add(id))
  bulkSelected.value = set
}
function groupAllSelected(name) {
  const grp = taskGroups.value.find(g => g.name === name)
  if (!grp || !grp.sessions.length) return false
  return grp.sessions.every(s => bulkSelected.value.has(s.id))
}
// 删除整个项目：项目实体 + 其下所有会话 + 归属映射（由 ChatWidget 落地）
function onDeleteProject(name) {
  if (!window.confirm(`删除项目「${name}」？其下所有会话将一并删除，无法恢复。`)) return
  emit('delete-project', name)
  toggleBulkMode()
}
function onDocClick() { openMenuId.value = null; showUserMenu.value = false }

onMounted(() => { loadPinned(); document.addEventListener('click', onDocClick); window.addEventListener('auth-change', refreshLoginState) })
onUnmounted(() => { document.removeEventListener('click', onDocClick); window.removeEventListener('auth-change', refreshLoginState) })
</script>

<style scoped>
.smc-root {
  display: flex;
  flex-direction: column;
  color: var(--app-text);
}
.smc-root.fill { height: 100%; min-height: 0; }

/* ===== Nav ===== */
.smc-nav {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 0;
  padding: 6px 8px 3px;
}
.smc-nav-item {
  width: 100%;
  min-height: 40px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 12px;
  border: 0;
  border-radius: 0;
  background: transparent;
  color: var(--app-text);
  font: inherit;
  font-size: 13.5px;
  font-weight: 500;
  line-height: 1.2;
  cursor: pointer;
  text-align: left;
  text-decoration: none;
  transition: background .16s ease, color .16s ease;
}
.smc-nav-item:hover {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 94%);
}
.smc-nav-item.primary {
  background: #fff;
  color: #202124;
  font-weight: 620;
  box-shadow: none;
}
.smc-nav-item.primary:hover {
  background: #f5f5f5;
}
.smc-nav-item .iconify {
  flex: 0 0 auto;
  color: var(--app-text-soft);
}
.smc-nav-item.primary .iconify { color: var(--app-accent); }

/* ===== Section labels ===== */
.smc-section { flex-shrink: 0; }
.smc-section-label {
  min-height: 32px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 16px 16px 6px;
  color: var(--app-text-soft);
  font-size: 12px;
  font-weight: 650;
  letter-spacing: .015em;
}
.smc-project-add {
  width: 28px;
  height: 28px;
  margin-right: -6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--app-text-soft);
  cursor: pointer;
  transition: background .15s ease, color .15s ease, transform .15s ease;
}
.smc-project-add:hover {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 93%);
  color: var(--app-text);
  transform: rotate(90deg);
}
.smc-project-bulk {
  width: 28px;
  height: 28px;
  margin-left: auto;
  margin-right: -5px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--app-text-soft);
  cursor: pointer;
  transition: background .15s ease, color .15s ease;
}
.smc-project-bulk:hover {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 93%);
  color: var(--app-text);
}
.smc-project-bulk.active {
  background: color-mix(in srgb, var(--app-accent), transparent 88%);
  color: var(--app-accent);
}

/* ===== 批量管理 ===== */
.smc-bulk-bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 2px 8px 6px;
  padding: 8px 10px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--app-accent), transparent 62%);
  background: color-mix(in srgb, var(--app-accent), transparent 92%);
}
.smc-bulk-count {
  flex: 1;
  min-width: 0;
  color: var(--app-text);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.smc-bulk-action {
  flex-shrink: 0;
  border: 0;
  border-radius: 7px;
  padding: 5px 10px;
  background: transparent;
  color: var(--app-accent);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: background .15s ease;
}
.smc-bulk-action:hover { background: color-mix(in srgb, var(--app-accent), transparent 86%); }
.smc-bulk-action.danger { color: #d94834; }
.smc-bulk-action.danger:hover { background: rgba(217, 72, 52, 0.1); }
.smc-bulk-action:disabled { color: var(--app-text-faint); cursor: default; background: transparent; }
.smc-bulk-check {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: var(--app-text-soft);
}
/* 项目头删除按钮：bulkMode 下显示，悬停变红 */
.smc-group-delete {
  margin-left: auto;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--app-text-faint);
  cursor: pointer;
  transition: background .15s ease, color .15s ease;
}
.smc-group-delete:hover {
  background: rgba(217, 72, 52, 0.12);
  color: #d94834;
}

/* ===== Folder：IDE 列表式分组，不再做圆角卡片 ===== */
.smc-folder { margin-bottom: 0; }
.smc-folder-head {
  min-height: 36px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 0 9px;
  border-radius: 0;
  cursor: pointer;
  transition: background .15s ease;
}
.smc-folder-head:hover {
  background: color-mix(in srgb, var(--app-text, #202124), transparent 94%);
}
.smc-folder-chevron {
  width: 14px;
  flex-shrink: 0;
  color: var(--app-text-faint);
  font-size: 14px;
  text-align: center;
  transition: transform .18s ease;
}
.smc-folder-chevron.open { transform: rotate(90deg); }
.smc-folder-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  font-weight: 620;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.smc-project-new-chat {
  width: 28px;
  height: 28px;
  display: inline-grid;
  flex: 0 0 auto;
  place-items: center;
  margin: 0 -4px 0 2px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--app-text-soft);
  background: transparent;
  cursor: pointer;
  transition: background .15s ease, border-color .15s ease, color .15s ease, transform .15s ease;
}
.smc-project-new-chat:hover {
  border-color: color-mix(in srgb, var(--app-border, #d7dce5), transparent 15%);
  color: var(--app-accent);
  background: var(--app-surface, #fff);
  transform: translateY(-1px);
}
.smc-project-new-chat:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--app-accent), transparent 35%);
  outline-offset: 2px;
}
.smc-folder-children {
  /* 不要缩进 —— 会话与文件夹平级。整栏右边界的描边+阴影统一挪到 .gem-sidebar（chat-window.css），
     不在这里单独做，避免只有展开的文件夹局部带框、和其余会话列表割裂。 */
  margin: 1px 2px 7px 0;
  padding: 1px 3px 4px 0;
}

/* ===== Session list area ===== */
.smc-session-area {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  min-height: 0;
  padding: 2px 6px 10px;
  /* 紧贴右缘的半透明细滚轴：内容较多时才出现，滚动中更容易抓取。 */
  scrollbar-width: thin;
  scrollbar-color: color-mix(in srgb, var(--app-accent, #c96442), transparent 48%) transparent;
  scrollbar-gutter: stable;
}
.smc-session-area::-webkit-scrollbar { width: 8px; }
.smc-session-area::-webkit-scrollbar-track { margin: 8px 0; background: transparent; }
.smc-session-area::-webkit-scrollbar-thumb {
  min-height: 36px;
  border: 2px solid transparent;
  border-radius: 999px;
  background: color-mix(in srgb, var(--app-accent, #c96442), transparent 55%);
  background-clip: padding-box;
}
.smc-session-area::-webkit-scrollbar-thumb:hover {
  background: color-mix(in srgb, var(--app-accent, #c96442), transparent 26%);
  background-clip: padding-box;
}

/* ===== Session row ===== */
.smc-session-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 10px;
  border-radius: 0;
  margin: 0;
  cursor: pointer;
  transition: background 0.15s ease;
}
.smc-session-row:hover { background: color-mix(in srgb, var(--app-text, #202124), transparent 95%); }
/* 恢复原来的会话层级：列表保留皮肤纸色，当前会话回到主表面色和细描边。 */
.smc-session-row.active {
  background: var(--app-surface);
  font-weight: 600;
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--app-text), transparent 88%);
}
/* 运行中会话行：保留高亮边框（波浪环覆盖在边框上做动态提示，圆点状态灯照常保留） */
.smc-session-row.running {
  background: var(--app-surface);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--app-accent), transparent 70%);
}
/* RunningRing(波浪环) 覆盖整行边框 */
.smc-running-ring {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

/* 运行指示灯：灰色空闲 → running(accent 电流行) / completed(绿) / question(橙) */
.smc-session-dot {
  flex-shrink: 0;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #c4c4c4;
  transition: background 0.2s ease;
}
/* 运行圆点：收敛的 accent 实心点 + 轻光晕。动态感交给整行电流环，这里不叠公转。 */
.smc-session-dot.running {
  background: var(--app-accent);
  box-shadow: 0 0 6px color-mix(in srgb, var(--app-accent), transparent 35%);
}
.smc-session-dot.completed {
  background: #22c55e;
  box-shadow: 0 0 0 0 #22c55e;
}
.smc-session-dot.question {
  background: #f59e0b;
  box-shadow: 0 0 0 0 #f59e0b;
}

.smc-session-name {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 400;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--app-text);
}
/* 标题渐变替换：旧标题淡出、新标题淡入（AI 生成标题替换默认标题时）。
   纯透明度渐变，不加位移——之前的 translateY 会让标题看起来在跳 */
.smc-title-swap-enter-active,
.smc-title-swap-leave-active {
  transition: opacity 0.2s ease;
}
.smc-title-swap-enter-from,
.smc-title-swap-leave-to {
  opacity: 0;
}
.smc-name-input {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 400;
  color: var(--app-text);
  font-family: inherit;
  background: var(--app-surface);
  border: 1px solid #3b82f6;
  border-radius: 6px;
  padding: 2px 6px;
  outline: none;
}

.smc-row-menu-wrap { position: relative; flex-shrink: 0; display: flex; }
.smc-row-menu-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  border-radius: 7px;
  color: var(--app-text-soft);
  cursor: pointer;
}
.smc-row-menu-btn:hover {
  color: var(--app-text);
  background: color-mix(in srgb, var(--app-text, #1a1a1a), transparent 91%);
}

/* ===== Dropdown ===== */
.smc-row-dropdown {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 10px;
  box-shadow: 0 12px 28px rgba(0,0,0,0.16);
  padding: 6px 0;
  z-index: 9999;
}
.smc-dropdown-item {
  padding: 7px 14px;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--app-text);
  cursor: pointer;
}
.smc-dropdown-item:hover { background: var(--app-surface-3); }
.smc-dropdown-item.danger { color: #d94834; }

/* ===== Footer ===== */
.fm-footer {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  margin: auto 8px 8px;
  padding: 8px;
  border-top: 1px solid color-mix(in srgb, var(--app-border), transparent 25%);
  color: var(--app-text);
  font-size: 13px;
  font-weight: 500;
}
.fm-footer-settings {
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border: 0;
  border-radius: 10px;
  color: var(--app-text-soft);
  background: transparent;
  cursor: pointer;
  transition: background .15s ease, color .15s ease;
}
.fm-footer-settings:hover,
.fm-footer-mail:hover {
  color: var(--app-text);
  background: color-mix(in srgb, var(--app-text, #202124), transparent 94%);
}
.fm-footer-mail {
  width: 36px;
  height: 36px;
  margin-left: auto;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border: 0;
  border-radius: 10px;
  color: var(--app-text-soft);
  background: transparent;
  cursor: pointer;
  position: relative;
  transition: background .15s ease, color .15s ease;
}
.fm-mail-badge {
  position: absolute;
  top: 0;
  right: 0;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: #ef4444;
  color: #fff;
  font-size: 10px;
  font-weight: 700;
  line-height: 1;
  pointer-events: none;
}
.fm-user {
  min-width: 0;
  min-height: 36px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 6px;
  border-radius: 10px;
  cursor: pointer;
}
.fm-user:hover { background: color-mix(in srgb, var(--app-text, #202124), transparent 95%); }
.fm-user span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fm-user-id {
  display: flex;
  flex-direction: column;
  justify-content: center;
  min-width: 0;
  line-height: 1.3;
}
.fm-user-name { font-size: 13px; color: var(--app-text); }
.fm-user-avatar {
  width: 24px;
  height: 24px;
  flex-shrink: 0;
  border: 1px solid color-mix(in srgb, var(--app-accent), transparent 42%);
  border-radius: 50%;
  object-fit: cover;
}
.fm-user-avatar-fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  background: var(--app-accent);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
}

/* ===== User card ===== */
.smc-card-backdrop {
  position: fixed; inset: 0; z-index: 9990;
  background: rgba(15, 23, 42, 0.32);
  backdrop-filter: blur(3px);
  animation: smcBackdropIn 0.18s ease-out;
}
.smc-user-card {
  position: fixed; left: 50%; top: 50%;
  transform: translate(-50%, -50%);
  width: min(460px, calc(100vw - 40px));
  max-height: min(760px, calc(100vh - 32px));
  overflow: auto;
  box-sizing: border-box;
  background: #fff;
  border: 1px solid var(--app-border);
  border-radius: 16px;
  box-shadow: 0 32px 90px rgba(15, 23, 42, 0.28), 0 2px 8px rgba(15, 23, 42, .08);
  padding: 14px;
  z-index: 10000;
  animation: smcCardIn 0.26s cubic-bezier(0.22, 1, 0.36, 1);
}
.smc-user-card.is-profile { width: min(720px, calc(100vw - 32px)); padding: 0; border-color: rgba(148,163,184,.24); border-radius: 24px; background: #f8fafc; scrollbar-width: none; }
.smc-user-card.is-profile::-webkit-scrollbar { display: none; }
@keyframes smcBackdropIn { from { opacity: 0; } to { opacity: 1; } }
@keyframes smcCardIn {
  from { opacity: 0; transform: translate(-50%, -48%) scale(0.92); }
  to   { opacity: 1; transform: translate(-50%, -50%) scale(1); }
}
.smc-profile-hero { position: relative; display: flex; align-items: center; gap: 16px; min-height: 116px; overflow: hidden; padding: 22px 24px; color: #fff; background: linear-gradient(125deg,#111827 0%,#1e293b 54%,#172554 100%); }
.smc-profile-aura { position: absolute; right: -70px; top: -100px; width: 310px; height: 310px; border: 1px solid rgba(129,140,248,.2); border-radius: 50%; box-shadow: 0 0 0 42px rgba(56,189,248,.045),0 0 0 82px rgba(99,102,241,.035); }
.smc-avatar-shell { position: relative; z-index: 1; display: grid; width: 62px; height: 62px; flex: 0 0 auto; place-items: center; border: 1px solid rgba(255,255,255,.42); border-radius: 18px; background: linear-gradient(145deg,#fb7185,#c2415d); box-shadow: 0 12px 25px rgba(190,24,93,.24); }
.smc-user-avatar {
  width: 100%; height: 100%; border-radius: 18px;
  object-fit: cover; flex-shrink: 0;
  border: 0;
}
.smc-user-avatar-fallback {
  color: #fff;
  font-size: 24px;
  font-weight: 750;
}
.smc-online-dot { position: absolute; right: -4px; bottom: -4px; width: 11px; height: 11px; border: 3px solid #172033; border-radius: 50%; background: #34d399; box-shadow: 0 0 12px #34d399; }
.smc-user-card-name { position: relative; z-index: 1; min-width: 0; color: #fff; }
.smc-profile-kicker { display: block; margin-bottom: 5px; color: #93c5fd; font-size: 9px; font-weight: 800; letter-spacing: .18em; }
.smc-user-card-name strong { display: block; overflow: hidden; font-size: 21px; line-height: 1.25; letter-spacing: -.025em; text-overflow: ellipsis; white-space: nowrap; }
.smc-profile-meta { display: flex; gap: 8px; margin-top: 8px; }
.smc-profile-meta span { padding: 4px 7px; border: 1px solid rgba(255,255,255,.14); border-radius: 6px; color: #cbd5e1; background: rgba(255,255,255,.06); font-size: 9px; font-weight: 700; }
.smc-profile-close { position: absolute; z-index: 2; right: 16px; top: 16px; display: grid; width: 34px; height: 34px; place-items: center; border: 0; border-radius: 9px; color: #94a3b8; background: rgba(255,255,255,.06); cursor: pointer; }
.smc-profile-close:hover { color: #fff; background: rgba(255,255,255,.13); }
.smc-profile-share { position: absolute; z-index: 2; right: 58px; top: 16px; display: grid; width: 34px; height: 34px; place-items: center; border: 0; border-radius: 9px; color: #94a3b8; background: rgba(255,255,255,.06); cursor: pointer; }
.smc-profile-share:hover { color: #fff; background: rgba(255,255,255,.13); }
.smc-radar-val { font-size: 10px; font-weight: 800; fill: #94a3b8; }
.smc-user-card-item {
  display: block; width: 100%; box-sizing: border-box;
  padding: 9px 12px; border: none; border-radius: 8px;
  background: transparent; color: var(--app-text);
  font-size: 13px; font-weight: 500; text-align: left;
  text-decoration: none; cursor: pointer;
  transition: background 0.12s ease;
}
.smc-user-card-item:hover { background: var(--app-surface-3); }
.smc-user-card-item.danger { color: #d94834; }
.smc-user-card-item.danger:hover { background: rgba(217, 72, 52, 0.08); }

/* 角色成长档案 */
.smc-evolve { padding: 22px 24px 18px; background: #f8fafc; }
.smc-evolve-summary { display: flex; align-items: center; justify-content: space-between; }
.smc-evolve-summary > div { display: flex; align-items: center; gap: 9px; }
.smc-evolve-summary > strong { color: #172033; font-size: 26px; letter-spacing: -.04em; }
.smc-evolve-summary > strong small { color: #94a3b8; font-size: 10px; letter-spacing: .12em; }
.smc-evolve-stage { color: #334155; font-size: 14px; font-weight: 800; }
.smc-evolve-lv { padding: 4px 8px; border: 1px solid #c7d2fe; border-radius: 7px; background: #eef2ff; color: #4338ca; font-size: 9px; font-weight: 900; letter-spacing: .08em; animation: smcLvPulse 1.6s ease-in-out 0.5s 2; }
.smc-evolve-lv.bump { animation: smcLvBump 0.6s cubic-bezier(0.34,1.56,0.64,1) 1; }
.smc-xp-track { height: 7px; margin-top: 12px; overflow: hidden; border-radius: 999px; background: #e2e8f0; }
.smc-xp-track i { display: block; height: 100%; border-radius: inherit; background: linear-gradient(90deg,#6366f1,#38bdf8); box-shadow: 0 0 14px rgba(56,189,248,.5); transition: width .5s ease; }
.smc-xp-copy { display: flex; justify-content: space-between; margin-top: 6px; color: #94a3b8; font-size: 9px; font-weight: 650; }
.smc-profile-dashboard { display: grid; grid-template-columns: minmax(0,.88fr) minmax(220px,1.12fr); gap: 14px; margin-top: 18px; }
.smc-radar-wrap,.smc-ability-list { min-height: 242px; padding: 15px; border: 1px solid #e2e8f0; border-radius: 15px; background: #fff; }
.smc-panel-label { display: block; color: #94a3b8; font-size: 8px; font-weight: 900; letter-spacing: .15em; }
.smc-radar { width: 100%; max-width: 225px; height: auto; display: block; margin: 2px auto -4px; }
.smc-radar-grid { fill: none; stroke: #dbe3ef; stroke-width: 1; }
.smc-radar-grid-1 { animation: smcRadarDraw 0.7s ease-out 0.05s both; }
.smc-radar-grid-2 { animation: smcRadarDraw 0.7s ease-out 0.15s both; }
.smc-radar-grid-3 { animation: smcRadarDraw 0.7s ease-out 0.25s both; }
.smc-radar-fill {
  fill: rgba(99, 102, 241, 0.16); stroke: #6366f1; stroke-width: 1.8;
  stroke-linejoin: round;
  transform-box: fill-box; transform-origin: center;
  animation: smcRadarGrow 0.55s cubic-bezier(0.34, 1.56, 0.64, 1) 0.35s both;
}
.smc-radar-line { opacity: 0; animation: smcFadeIn 0.3s ease-out 0.5s both; }
.smc-radar-label { font-size: 10.5px; font-weight: 750; fill: #64748b; opacity: 0; animation: smcFadeIn 0.3s ease-out 0.55s both; }
.smc-radar-point { opacity: 0; animation: smcFadeIn 0.3s ease-out 0.55s both; }
.smc-ability-list { display: flex; flex-direction: column; gap: 11px; }
.smc-ability-list > .smc-panel-label { margin-bottom: 1px; }
.smc-ability-row > div { display: grid; grid-template-columns: 7px 1fr auto; align-items: center; gap: 7px; color: #64748b; font-size: 10px; }
.smc-ability-row > div > i { width: 7px; height: 7px; border-radius: 2px; }
.smc-ability-row b { margin-left: auto; justify-self: end; color: #1e293b; font-size: 14px; font-weight: 800; letter-spacing: -.02em; }
.smc-ability-row b small { font-size: 9px; }
.smc-ability-track { display: block; height: 4px; margin-top: 5px; overflow: hidden; border-radius: 99px; background: #eef2f7; }
.smc-ability-track i { display: block; height: 100%; border-radius: inherit; transition: width .45s ease; }
.smc-career-stats { display: grid; grid-template-columns: repeat(4,1fr); gap: 8px; margin-top: 12px; }
.smc-career-stats > div { display: flex; align-items: center; gap: 9px; min-width: 0; padding: 11px; border: 1px solid #e2e8f0; border-radius: 12px; color: #6366f1; background: #fff; }
.smc-career-stats span { min-width: 0; }
.smc-career-stats b,.smc-career-stats small { display: block; }
.smc-career-stats b { color: #1e293b; font-size: 14px; }
.smc-career-stats small { margin-top: 1px; overflow: hidden; color: #94a3b8; font-size: 8px; text-overflow: ellipsis; white-space: nowrap; }
.smc-profile-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 13px 24px; border-top: 1px solid #e2e8f0; background: #fff; }
.smc-profile-actions > span { display: flex; align-items: center; gap: 5px; color: #94a3b8; font-size: 9px; }
.smc-profile-actions .smc-user-card-item { display: flex; align-items: center; gap: 6px; width: auto; min-height: 36px; padding: 7px 10px; }
.smc-login-panel { padding: 14px 24px 20px; border-top: 1px solid #e2e8f0; background: #fff; }
.smc-login-panel .smc-user-card-item { width: 100%; box-sizing: border-box; text-align: center; color: #1e293b; font-weight: 650; text-decoration: none; }
.smc-login-panel .smc-rc-sep { margin: 12px 0; }
.smc-profile-loading { display: flex; align-items: center; justify-content: center; gap: 9px; min-height: 270px; color: #64748b; font-size: 12px; }
.smc-profile-loading i { width: 16px; height: 16px; border: 2px solid #cbd5e1; border-top-color: #6366f1; border-radius: 50%; animation: smcSpin .7s linear infinite; }
@keyframes smcSpin { to { transform: rotate(360deg); } }
@keyframes smcRadarDraw { from { stroke-dasharray: 640; stroke-dashoffset: 640; } to { stroke-dasharray: 640; stroke-dashoffset: 0; } }
@keyframes smcRadarGrow { from { transform: scale(0); opacity: 0.2; } to { transform: scale(1); opacity: 1; } }
@keyframes smcFadeIn { from { opacity: 0; } to { opacity: 1; } }
@keyframes smcLvPulse { 0%, 100% { transform: scale(1); } 50% { transform: scale(1.12); } }
@keyframes smcLvBump { 0% { transform: scale(1); } 50% { transform: scale(1.35); } 100% { transform: scale(1); } }

@media (max-width: 620px) {
  .smc-user-card.is-profile { width: calc(100vw - 20px); max-height: calc(100vh - 20px); border-radius: 18px; }
  .smc-profile-hero { min-height: 92px; padding: 18px; }
  .smc-profile-dashboard { grid-template-columns: 1fr; }
  .smc-career-stats { grid-template-columns: repeat(2,1fr); }
  .smc-profile-actions { padding: 12px 18px; }
}

@media (prefers-reduced-motion: reduce) {
  .smc-user-card,.smc-radar-grid,.smc-radar-fill,.smc-radar-line,.smc-radar-label,.smc-evolve-lv { animation: none; }
}

/* ===== Create project modal ===== */
.smc-modal-backdrop {
  position: fixed; inset: 0; z-index: 10020;
  display: flex; align-items: center; justify-content: center;
  padding: 24px; background: rgba(15, 23, 42, 0.28); backdrop-filter: blur(3px);
}
.smc-create-project {
  width: min(100%, 620px); box-sizing: border-box; padding: 28px;
  border: 1px solid rgba(255, 255, 255, 0.7); border-radius: 22px;
  background: var(--app-surface, #fff); color: var(--app-text, #202124);
  box-shadow: 0 24px 64px rgba(15, 23, 42, 0.2);
}
.smc-create-project-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.smc-create-project-head h2 { margin: 0; font-size: 24px; line-height: 1.2; letter-spacing: -0.02em; }
.smc-modal-close {
  display: inline-flex; align-items: center; justify-content: center; width: 32px; height: 32px;
  border: 0; border-radius: 8px; background: transparent; color: var(--app-text-soft); cursor: pointer;
}
.smc-modal-close:hover { background: var(--app-surface-3); color: var(--app-text); }
.smc-source-label { margin: 20px 0 10px; font-size: 14px; font-weight: 650; }
.smc-source-picker {
  display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px;
  width: 100%; min-height: 120px; padding: 18px; border: 1px solid var(--app-border); border-radius: 14px;
  background: var(--app-surface); color: var(--app-text); font: inherit; font-size: 14px; cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}
.smc-source-picker:hover { border-color: var(--app-accent); background: var(--app-surface-2); }
.smc-source-picker .iconify { color: var(--app-text-soft); }
.smc-source-picker-error { margin: 8px 2px 0; color: #c62828; font-size: 13px; line-height: 1.45; }
.smc-recent-project-picker { display: grid; gap: 6px; margin-top: 16px; color: var(--app-text-soft); font-size: 13px; font-weight: 600; }
.smc-recent-project-picker select { width: 100%; min-height: 42px; padding: 0 12px; border: 1px solid var(--app-border); border-radius: 10px; background: var(--app-surface); color: var(--app-text); font: inherit; font-weight: 500; cursor: pointer; }
.smc-recent-project-picker select:focus-visible { outline: 2px solid color-mix(in srgb, var(--app-accent), transparent 45%); outline-offset: 2px; border-color: var(--app-accent); }
.smc-select-project-list {
  display: flex; flex-direction: column; gap: 8px; max-height: 320px; overflow-y: auto;
}
.smc-select-project-list .smc-source-picker {
  flex-direction: row; justify-content: flex-start; min-height: 44px; padding: 10px 14px;
}
.smc-create-project-actions { display: flex; justify-content: flex-end; gap: 12px; margin-top: 24px; }
.smc-cancel-btn, .smc-create-btn {
  min-height: 40px; padding: 0 16px; border: 0; border-radius: 11px; font: inherit; font-weight: 600; cursor: pointer;
}
.smc-cancel-btn { background: transparent; color: var(--app-text-soft); }
.smc-cancel-btn:hover { background: var(--app-surface-3); color: var(--app-text); }
.smc-create-btn { background: #202124; color: #fff; }
.smc-create-btn:disabled { opacity: 0.45; cursor: not-allowed; }
.smc-create-btn:not(:disabled):hover { background: #000; }
.smc-modal-enter-active, .smc-modal-leave-active { transition: opacity 0.18s ease; }
.smc-modal-enter-active .smc-create-project, .smc-modal-leave-active .smc-create-project { transition: transform 0.18s ease, opacity 0.18s ease; }
.smc-modal-enter-from, .smc-modal-leave-to { opacity: 0; }
.smc-modal-enter-from .smc-create-project, .smc-modal-leave-to .smc-create-project { transform: translateY(10px) scale(0.98); opacity: 0; }


/* ===== Rescene Cloud 账号登录 ===== */
.smc-rc-sep { margin: 8px 12px 4px; font-size: 11px; color: #9aa3b2; text-align: center; }
.smc-rc-login { padding: 0 12px 12px; display: flex; flex-direction: column; gap: 6px; }
.smc-rc-input { width: 100%; box-sizing: border-box; padding: 7px 10px; border: 1px solid #e3e9f2; border-radius: 8px; font-size: 12.5px; outline: none; background: #fff; }
.smc-rc-input:focus { border-color: #4f7cff; }
.smc-rc-btn { padding: 7px 0; border: 0; border-radius: 8px; background: #4f7cff; color: #fff; font-size: 12.5px; font-weight: 700; cursor: pointer; }
.smc-rc-btn:disabled { opacity: 0.6; }
.smc-rc-link { color: #4f7cff; cursor: pointer; text-decoration: underline; }
.smc-rc-link:hover { color: #3a63e8; }
.smc-rc-err { font-size: 11px; color: #e5484d; }
.smc-rc-hint { font-size: 10.5px; color: #9aa3b2; text-align: center; }
.smc-auth-warn { display: flex; align-items: center; gap: 6px; margin: 0 16px 10px; padding: 7px 10px; border-radius: 8px; background: rgba(245,158,11,.12); color: #b45309; font-size: 11.5px; line-height: 1.4; }
</style>
