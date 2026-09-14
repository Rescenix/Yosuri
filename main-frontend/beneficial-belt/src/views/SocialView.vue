<template>
  <div class="so-page">
    <div class="so-topbar">
      <button class="so-back" @click="$router.push('/chat')"><Icon icon="mdi:arrow-left" /> {{ tr('返回') }}</button>
      <span class="so-title">{{ tr('搭子') }}</span>
      <span class="so-tip">{{ tr('好友 · 私聊 · 云端权威') }}</span>
    </div>

    <div v-if="!authed" class="so-login">
      <Icon icon="mdi:account-circle-outline" width="42" />
      <p>{{ tr('登录 Rescene 账号后即可添加好友、私聊搭子') }}</p>
    </div>

    <div v-else class="so-body">
      <!-- 左栏：会话 / 好友 -->
      <aside class="so-side">
        <div class="so-tabs">
          <button :class="{ on: tab === 'chats' }" @click="switchTab('chats')">
            {{ tr('消息') }}<span v-if="totalUnread" class="so-badge">{{ totalUnread }}</span>
          </button>
          <button :class="{ on: tab === 'friends' }" @click="switchTab('friends')">{{ tr('好友') }}</button>
          <button :class="{ on: tab === 'add' }" @click="switchTab('add')">{{ tr('添加') }}</button>
        </div>

        <div class="so-list">
          <div v-if="loading" class="so-empty">{{ tr('加载中…') }}</div>
          <div v-else-if="err" class="so-empty so-err">{{ err }}</div>

          <template v-else-if="tab === 'chats'">
            <div v-if="!conversations.length" class="so-empty">{{ tr('还没有聊天，去「好友」里找个搭子吧') }}</div>
            <div v-for="cv in conversations" :key="cv.uid" class="so-row" :class="{ on: peer && peer.uid === cv.uid }" @click="openChat(cv)">
              <span class="so-avatar"><Icon icon="mdi:account-circle-outline" width="26" /></span>
              <div class="so-info">
                <span class="so-name">{{ cv.name || cv.username || ('uid ' + cv.uid) }}</span>
                <span class="so-sub">{{ cv.last_text || tr('还没有消息') }}</span>
              </div>
              <span v-if="cv.unread" class="so-badge">{{ cv.unread }}</span>
            </div>
          </template>

          <template v-else-if="tab === 'friends'">
            <div v-if="incoming.length" class="so-sec">{{ tr('待处理申请') }}</div>
            <div v-for="f in incoming" :key="'in' + f.uid" class="so-row">
              <span class="so-avatar"><Icon icon="mdi:account-plus-outline" width="26" /></span>
              <div class="so-info">
                <span class="so-name">{{ f.name || f.username }}</span>
                <span class="so-sub">{{ tr('想加你为好友') }}</span>
              </div>
              <button class="so-btn ok" @click.stop="respond(f, true)">{{ tr('同意') }}</button>
              <button class="so-btn" @click.stop="respond(f, false)">{{ tr('拒绝') }}</button>
            </div>
            <div v-if="friends.length" class="so-sec">{{ tr('我的好友') }}</div>
            <div v-if="!friends.length && !incoming.length" class="so-empty">{{ tr('还没有好友，去「添加」里搜用户名') }}</div>
            <div v-for="f in friends" :key="'fr' + f.uid" class="so-row" :class="{ on: peer && peer.uid === f.uid }" @click="openChat(f)">
              <span class="so-avatar"><Icon icon="mdi:account-circle-outline" width="26" /></span>
              <div class="so-info">
                <span class="so-name">{{ f.name || f.username || ('uid ' + f.uid) }}</span>
                <span class="so-sub">@{{ f.username || '—' }} · {{ tr('装等') }} {{ f.gear_score }}</span>
              </div>
              <button class="so-btn" @click.stop="delFriend(f)">{{ tr('删除') }}</button>
            </div>
          </template>

          <template v-else>
            <div class="so-search">
              <input v-model="q" class="so-input" :placeholder="tr('输入用户名或 uid，精确搜索')" @keyup.enter="doSearch" />
              <button class="so-btn primary" @click="doSearch"><Icon icon="mdi:magnify" /> {{ tr('搜索') }}</button>
            </div>
            <div v-if="searching" class="so-empty">{{ tr('搜索中…') }}</div>
            <div v-else-if="searchErr" class="so-empty so-err">{{ searchErr }}</div>
            <div v-else-if="found" class="so-row">
              <span class="so-avatar"><Icon icon="mdi:account-circle-outline" width="26" /></span>
              <div class="so-info">
                <span class="so-name">{{ found.name || found.username }}</span>
                <span class="so-sub">@{{ found.username || ('uid ' + found.uid) }} · {{ tr('装等') }} {{ found.gear_score }}</span>
              </div>
              <button v-if="found.friend_status === 'none'" class="so-btn primary" @click="addFriend(found)">{{ tr('加好友') }}</button>
              <span v-else-if="found.friend_status === 'self'" class="so-sub">{{ tr('这是你自己') }}</span>
              <span v-else class="so-sub">{{ found.friend_status === 'accepted' ? tr('已是好友') : tr('申请已发出') }}</span>
            </div>

            <div class="so-sec">{{ tr('最近上线') }}</div>
            <div v-if="!suggest.length" class="so-empty">{{ tr('暂时没有找到活跃的搭子，过会儿再来看看') }}</div>
            <div v-for="s in suggest" :key="'sg' + s.uid" class="so-row">
              <span class="so-avatar"><Icon icon="mdi:account-circle-outline" width="26" /></span>
              <div class="so-info">
                <span class="so-name">{{ s.name || s.username || ('uid ' + s.uid) }}</span>
                <span class="so-sub">{{ tr('装等') }} {{ s.gear_score }} · {{ seenLabel(s.last_seen) }}</span>
              </div>
              <button v-if="s.friend_status === 'none' || !s.friend_status" class="so-btn primary" @click="addSuggest(s)">{{ tr('加好友') }}</button>
              <span v-else class="so-sub">{{ s.friend_status === 'accepted' ? tr('已是好友') : tr('已申请') }}</span>
            </div>
          </template>
        </div>
      </aside>

      <!-- 右栏：聊天窗 -->
      <section class="so-chat">
        <template v-if="peer">
          <div class="so-chat-head">
            <span class="so-name">{{ peer.name || peer.username || ('uid ' + peer.uid) }}</span>
            <button class="so-btn" @click="peer = null"><Icon icon="mdi:close" /></button>
          </div>
          <div ref="msgBox" class="so-msgs">
            <div v-if="hasMore" class="so-more"><button class="so-btn" @click="loadOlder">{{ tr('加载更早消息') }}</button></div>
            <div v-for="m in messages" :key="m.id" class="so-msg" :class="{ mine: m.from_uid === myUid }">
              <span class="so-bubble">{{ m.text }}</span>
            </div>
            <div v-if="!messages.length && !loadingMsgs" class="so-empty">{{ tr('打个招呼，认识一下你的搭子吧') }}</div>
          </div>
          <div class="so-compose">
            <textarea v-model="draft" class="so-input so-textarea" rows="2" :placeholder="tr('发消息…（Enter 发送）')" @keydown.enter.exact.prevent="send" />
            <button class="so-btn primary" :disabled="sending || !draft.trim()" @click="send">{{ tr('发送') }}</button>
          </div>
        </template>
        <div v-else class="so-chat-empty">
          <Icon icon="mdi:chat-outline" width="46" />
          <p>{{ tr('选择一个会话开始聊天') }}</p>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { Icon } from '@iconify/vue'
import { tr } from '../composables/useI18n.js'
import { useAuth } from '../composables/useAuth.js'

const { isLoggedIn, uid: myUidRef } = useAuth()
const authed = computed(() => isLoggedIn.value)
const myUid = computed(() => Number(myUidRef.value) || 0)

// 社交接口一律走 re0 薄代理转发到 ResceneCloud，身份取 JWT（前端只负责带上 token）
function tk() {
  try { return localStorage.getItem('token') || '' } catch { return '' }
}
function hd(extra) {
  return Object.assign({ 'Content-Type': 'application/json' }, extra, tk() ? { Authorization: 'Bearer ' + tk() } : {})
}
async function api(method, path, body) {
  const res = await fetch(path, { method, headers: hd(), body: body ? JSON.stringify(body) : undefined })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.error || ('HTTP ' + res.status))
  return data
}

const tab = ref('chats')
const loading = ref(false)
const err = ref('')
const conversations = ref([])
const friends = ref([])
const incoming = ref([])
const totalUnread = computed(() => conversations.value.reduce((s, c) => s + (c.unread || 0), 0))

// 聊天窗
const peer = ref(null)
const messages = ref([])
const hasMore = ref(false)
const loadingMsgs = ref(false)
const draft = ref('')
const sending = ref(false)
const msgBox = ref(null)

// 搜索 / 添加
const q = ref('')
const searching = ref(false)
const searchErr = ref('')
const found = ref(null)
const suggest = ref([])

async function loadConversations() {
  if (!authed.value) return
  try {
    const d = await api('GET', '/api/dm/conversations')
    conversations.value = d.conversations || []
  } catch (e) { /* 轮询失败静默，下次再来 */ }
}
async function loadFriends() {
  if (!authed.value) return
  loading.value = true; err.value = ''
  try {
    const d = await api('GET', '/api/friends')
    friends.value = d.friends || []
    incoming.value = d.incoming || []
  } catch (e) { err.value = e.message }
  finally { loading.value = false }
}
function switchTab(t) {
  tab.value = t
  if (t === 'friends') loadFriends()
  if (t === 'chats') loadConversations()
  if (t === 'add') loadSuggest()
}

async function loadSuggest() {
  if (!authed.value) return
  try {
    const d = await api('GET', '/api/friends/suggest')
    suggest.value = d.suggestions || []
  } catch (e) { /* 推荐失败静默，不挡搜索 */ }
}

// 云端 last_seen 是 UTC 自然日，展示按北京时间（+8）折算，避免「今天」显示成昨天。
function seenLabel(dateStr) {
  if (!dateStr) return tr('最近上线')
  const today = new Date(Date.now() + 8 * 3600 * 1000)
  const y = today.toISOString().slice(0, 10)
  const yest = new Date(today.getTime() - 86400 * 1000).toISOString().slice(0, 10)
  if (dateStr >= y) return tr('今天上线')
  if (dateStr === yest) return tr('昨天上线')
  const days = Math.max(0, Math.round((today - new Date(dateStr + 'T00:00:00Z')) / 86400000))
  return tr(days + ' 天前上线')
}

async function openChat(p) {
  peer.value = { uid: p.uid, name: p.name, username: p.username }
  messages.value = []
  hasMore.value = false
  loadingMsgs.value = true
  try {
    const d = await api('GET', `/api/dm/history?peer=${p.uid}&limit=50`)
    messages.value = (d.messages || []).slice().reverse() // 接口新→旧，界面要旧→新
    hasMore.value = !!d.has_more
    api('POST', '/api/dm/read', { peer: p.uid }).then(loadConversations).catch(() => {})
    await nextTick(); scrollBottom()
  } catch (e) { err.value = e.message }
  finally { loadingMsgs.value = false }
}
async function loadOlder() {
  if (!messages.value.length) return
  const oldest = messages.value[0].id
  const d = await api('GET', `/api/dm/history?peer=${peer.value.uid}&limit=50&before=${oldest}`)
  messages.value = (d.messages || []).slice().reverse().concat(messages.value)
  hasMore.value = !!d.has_more
}
function scrollBottom() {
  if (msgBox.value) msgBox.value.scrollTop = msgBox.value.scrollHeight
}
async function send() {
  const text = draft.value.trim()
  if (!text || sending.value || !peer.value) return
  sending.value = true
  try {
    const d = await api('POST', '/api/dm/send', { to: peer.value.uid, text })
    messages.value.push({ id: d.id, from_uid: myUid.value, text, created_at: Date.now() * 1e6 })
    draft.value = ''
    await nextTick(); scrollBottom()
    loadConversations()
  } catch (e) { err.value = e.message }
  finally { sending.value = false }
}

async function respond(f, accept) {
  try { await api('POST', '/api/friends/respond', { uid: f.uid, accept }); loadFriends() }
  catch (e) { err.value = e.message }
}
async function delFriend(f) {
  try {
    await api('POST', '/api/friends/delete', { uid: f.uid })
    if (peer.value && peer.value.uid === f.uid) peer.value = null
    loadFriends(); loadConversations()
  } catch (e) { err.value = e.message }
}
async function doSearch() {
  const name = q.value.trim()
  if (!name) return
  searching.value = true; searchErr.value = ''; found.value = null
  try { found.value = await api('GET', '/api/friends/search?username=' + encodeURIComponent(name)) }
  catch (e) { searchErr.value = e.message }
  finally { searching.value = false }
}
async function addFriend(u) {
  try {
    await api('POST', '/api/friends/request', { uid: u.uid })
    found.value = Object.assign({}, u, { friend_status: 'pending' })
  } catch (e) { searchErr.value = e.message }
}
async function addSuggest(s) {
  try {
    await api('POST', '/api/friends/request', { uid: s.uid })
    s.friend_status = 'pending'
  } catch (e) { err.value = e.message }
}

let poll = null
onMounted(() => {
  if (!authed.value) return
  loadConversations(); loadFriends()
  poll = setInterval(() => { if (authed.value) loadConversations() }, 12000)
})
onUnmounted(() => { if (poll) clearInterval(poll) })
</script>

<style scoped>
.so-page { height: 100vh; display: flex; flex-direction: column; background: #F5F8FF; }
.so-topbar { display: flex; align-items: center; gap: 14px; padding: 8px 14px; border-bottom: 1px solid #d6e4ff; background: #fff; flex-shrink: 0; }
.so-back { display: flex; align-items: center; gap: 4px; background: none; border: 1px solid #1950BE; color: #1950BE; border-radius: 8px; padding: 4px 12px; font-size: 13px; cursor: pointer; }
.so-title { font-size: 15px; color: #1a2b4c; font-weight: 600; }
.so-tip { margin-left: auto; font-size: 12px; color: #6b7a99; }

.so-login { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px; color: #6b7a99; }

.so-body { flex: 1; display: flex; min-height: 0; }
.so-side { width: 320px; flex-shrink: 0; display: flex; flex-direction: column; border-right: 1px solid #d6e4ff; background: #fff; min-height: 0; }
.so-tabs { display: flex; gap: 4px; padding: 8px 10px 4px; }
.so-tabs button { position: relative; flex: 1; border: none; background: none; padding: 6px 0; font-size: 13px; color: #6b7a99; border-bottom: 2px solid transparent; cursor: pointer; }
.so-tabs button.on { color: #1950BE; font-weight: 600; border-bottom-color: #1950BE; }
.so-badge { display: inline-block; min-width: 16px; padding: 0 5px; margin-left: 4px; border-radius: 9px; background: #e6496e; color: #fff; font-size: 11px; line-height: 16px; text-align: center; }

.so-list { flex: 1; overflow-y: auto; min-height: 0; }
.so-sec { padding: 8px 12px 2px; font-size: 11px; color: #8ba0c4; }
.so-row { display: flex; align-items: center; gap: 8px; padding: 8px 12px; cursor: pointer; }
.so-row:hover { background: #eef4ff; }
.so-row.on { background: #e3edff; }
.so-avatar { color: #1950BE; display: flex; }
.so-info { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.so-name { font-size: 13px; color: #1a2b4c; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.so-sub { font-size: 11px; color: #8ba0c4; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.so-empty { padding: 24px 14px; text-align: center; font-size: 12px; color: #8ba0c4; }
.so-err { color: #e6496e; }
.so-search { display: flex; gap: 6px; padding: 10px 12px; }
.so-input { flex: 1; border: 1px solid #d6e4ff; border-radius: 8px; padding: 6px 10px; font-size: 13px; color: #1a2b4c; background: #fff; outline: none; }
.so-input:focus { border-color: #1950BE; }
.so-btn { display: inline-flex; align-items: center; gap: 3px; border: 1px solid #d6e4ff; background: #fff; color: #4a5f85; border-radius: 8px; padding: 4px 10px; font-size: 12px; cursor: pointer; flex-shrink: 0; }
.so-btn:hover { border-color: #1950BE; color: #1950BE; }
.so-btn.primary { background: #1950BE; border-color: #1950BE; color: #fff; }
.so-btn.primary:disabled { opacity: .5; cursor: default; }
.so-btn.ok { background: #e8f7ee; border-color: #34a853; color: #1e7e34; }

.so-chat { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.so-chat-head { display: flex; align-items: center; padding: 10px 14px; border-bottom: 1px solid #d6e4ff; background: #fff; }
.so-chat-head .so-name { flex: 1; }
.so-msgs { flex: 1; overflow-y: auto; padding: 12px 16px; display: flex; flex-direction: column; gap: 8px; min-height: 0; }
.so-more { text-align: center; }
.so-msg { display: flex; }
.so-msg.mine { justify-content: flex-end; }
.so-bubble { max-width: 62%; padding: 7px 12px; border-radius: 12px; font-size: 13px; line-height: 1.5; color: #1a2b4c; background: #fff; border: 1px solid #d6e4ff; word-break: break-word; white-space: pre-wrap; }
.so-msg.mine .so-bubble { background: #1950BE; border-color: #1950BE; color: #fff; }
.so-compose { display: flex; gap: 8px; padding: 10px 14px; border-top: 1px solid #d6e4ff; background: #fff; align-items: flex-end; }
.so-textarea { resize: none; font-family: inherit; }
.so-chat-empty { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px; color: #8ba0c4; }
</style>

