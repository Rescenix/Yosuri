<template>
  <div class="rp-battle-card">
    <!-- 战场：左我右敌（星迹式大场景：头像 + 状态栏 + 血条） -->
    <div class="rp-battle-field" :class="{ shake: shaking }">
      <div class="rp-battle-side allies">
        <div v-for="u in myUnits" :key="u.id" class="rp-battle-unit" :class="{ dead: !u.alive, 'just-hit': hitMark(u.id), defending: u.defending }">
          <span v-if="popFor(u.id)" class="rp-pop-dmg" :class="{ crit: popFor(u.id).crit }">-{{ popFor(u.id).amount }}</span>
          <div class="rp-battle-avatar-wrap">
            <img v-if="u.avatar" :src="u.avatar" class="rp-battle-avatar" alt="" />
            <span v-else class="rp-battle-avatar rp-battle-avatar-text" :style="{ background: unitColor(u) }">{{ (u.name || '?').charAt(0) }}</span>
            <span class="rp-battle-element" :class="u.element">{{ elementIcon(u.element) }}</span>
          </div>
          <span class="rp-battle-unit-name">{{ u.name }}</span>
          <div class="rp-battle-statbar">
            <span>攻{{ u.atk }}</span><span>防{{ u.def }}</span><span>速{{ u.spd }}</span><span v-if="u.maxMp > 0">魔{{ u.mag }}</span>
          </div>
          <div class="rp-battle-hpbar">
            <div v-if="u.shield > 0" class="rp-battle-shield" :style="{ width: pct(u.shield, u.maxHp) + '%' }"></div>
            <div class="rp-battle-hpfill" :style="{ width: pct(u.hp, u.maxHp) + '%' }"></div>
            <span class="rp-battle-hptext">{{ Math.max(0, u.hp) }}/{{ u.maxHp }}</span>
          </div>
          <div v-if="u.maxMp > 0" class="rp-battle-mpbar">
            <div class="rp-battle-mpfill" :style="{ width: pct(u.mp, u.maxMp) + '%' }"></div>
            <span class="rp-battle-hptext">{{ u.mp }}/{{ u.maxMp }}</span>
          </div>
        </div>
      </div>
      <div class="rp-battle-vs">⚔️</div>
      <div class="rp-battle-side enemies">
        <div v-for="u in enemyUnits" :key="u.id" class="rp-battle-unit" :class="{ dead: !u.alive, 'just-hit': hitMark(u.id), defending: u.defending }">
          <span v-if="popFor(u.id)" class="rp-pop-dmg" :class="{ crit: popFor(u.id).crit }">-{{ popFor(u.id).amount }}</span>
          <div class="rp-battle-avatar-wrap">
            <span v-if="u.avatar && u.avatar.length <= 4" class="rp-battle-avatar rp-battle-avatar-emoji">{{ u.avatar }}</span>
            <img v-else-if="u.avatar" :src="u.avatar" class="rp-battle-avatar" alt="" />
            <span v-else class="rp-battle-avatar rp-battle-avatar-text" :style="{ background: unitColor(u) }">{{ (u.name || '?').charAt(0) }}</span>
            <span class="rp-battle-element" :class="u.element">{{ elementIcon(u.element) }}</span>
          </div>
          <span class="rp-battle-unit-name">{{ u.name }}</span>
          <div class="rp-battle-statbar">
            <span>攻{{ u.atk }}</span><span>防{{ u.def }}</span><span>速{{ u.spd }}</span><span v-if="u.maxMp > 0">魔{{ u.mag }}</span>
          </div>
          <div class="rp-battle-hpbar">
            <div v-if="u.shield > 0" class="rp-battle-shield" :style="{ width: pct(u.shield, u.maxHp) + '%' }"></div>
            <div class="rp-battle-hpfill" :style="{ width: pct(u.hp, u.maxHp) + '%' }"></div>
            <span class="rp-battle-hptext">{{ Math.max(0, u.hp) }}/{{ u.maxHp }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 战斗日志（宝可梦式：逐条浮出，行动行 + 效果行分行） -->
    <div v-if="shownLog.length" class="rp-battle-log" ref="logEl">
      <div v-for="(ev, i) in shownLog" :key="`${i}-${ev.actor}-${ev.amount}`" class="rp-battle-logline" :class="splitEvent(ev).cls">
        <span class="rp-log-actor" :class="unitSide(ev.actor)">{{ ev.actor || '' }}</span>
        <span class="rp-log-action" :class="unitSide(ev.actor)">{{ actionLabel(ev) }}</span>
        <span class="rp-log-arrow" v-if="ev.target">→</span>
        <span class="rp-log-target" :class="unitSide(ev.target)">{{ ev.target || '' }}</span>
        <span v-if="ev.amount > 0" class="rp-log-dmg" :class="splitEvent(ev).cls">-{{ ev.amount }}</span>
        <span v-if="splitEvent(ev).effect" class="rp-log-effect">{{ splitEvent(ev).effect }}</span>
      </div>
    </div>

    <!-- 终局结算：主角倒下 → 世界线终局结算面板 -->
    <div v-if="battle.gameOver" class="rp-battle-ending">
      <div class="rp-ending-title">💀 世界线终局</div>
      <div class="rp-ending-row" v-if="battle.turn"><span>战斗回合</span><b>{{ battle.turn - 1 }}</b></div>
      <div class="rp-ending-row" v-if="battle.goldGain"><span>金币战利</span><b>{{ battle.goldGain }}</b></div>
      <div class="rp-ending-sub">{{ tr('主角倒下了，这条世界线到此为止。') }}</div>
      <button type="button" class="rp-battle-close" @click="closeAndMaybeSettle">{{ tr('收起结算') }}</button>
    </div>

    <!-- 普通结果：胜利（含战利品）或战败 -->
    <div v-else-if="battle.over" class="rp-battle-result" :class="{ win: battle.victory }">
      <span v-if="battle.victory">🎉 胜利！</span>
      <span v-else>💀 战败…</span>
      <span v-if="battle.victory && battle.loot" class="rp-battle-gold">获得战利品 {{ battle.loot.icon }} {{ battle.loot.name }} ×{{ battle.loot.count }}</span>
      <button type="button" class="rp-battle-close" @click="closeAndMaybeSettle">{{ tr('收起战场') }}</button>
    </div>

    <!-- 战斗进行中：逐格操作（每人一行攻击/技能/防御，点完结束回合统一结算） -->
    <div v-else class="rp-battle-controls">
      <div v-for="u in myUnits" :key="u.id" class="rp-battle-actor-row" :class="{ dead: !u.alive }">
        <span class="rp-battle-actor">{{ u.name }}</span>
        <span v-if="!u.alive" class="rp-battle-wait">{{ tr('已倒下') }}</span>
        <template v-else>
          <button type="button" class="rp-battle-btn atk" :class="{ picked: !isSolo && picked(u.id) === 'attack' }" @click="pick(u.id, 'attack')">⚔️ {{ tr('攻击') }}</button>
          <button type="button" class="rp-battle-btn def" :class="{ picked: !isSolo && picked(u.id) === 'defend' }" @click="pick(u.id, 'defend')">🛡️ {{ tr('防御') }}</button>
          <button type="button" class="rp-battle-btn skill" :class="{ picked: !isSolo && picked(u.id) === 'skill' }" :disabled="u.mp < 8" @click="pick(u.id, 'skill')">✨ {{ skillLabel(u) }}<span v-if="u.maxMp > 0">(耗8MP)</span></button>
        </template>
      </div>
      <div class="rp-battle-turnbar">
        <span class="rp-battle-wait">{{ tr('第') }} {{ battle.turn }} {{ tr('回合') }}</span>
        <!-- 单人战斗：选完即结算，不再需要「结束回合」按钮 -->
        <button v-if="!isSolo" type="button" class="rp-battle-close rp-battle-endturn" :disabled="submitting" @click="submitTurn">
          {{ submitting ? tr('结算中…') : tr('结束回合 ▶') }}
        </button>
        <span v-else class="rp-battle-wait">{{ tr('选择行动后自动结算') }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
// RpBattleCard.vue —— 聊天内嵌战斗场景（逐格操作模式）。
// Yosuri 用 rp_battle_start 开战 → 后端只摆战场（带 battle.id），
// 前端给每个存活我方单位一行 攻击/技能/防御 按钮，点选后「结束回合」
// POST /api/rp/battle/turn 整回合结算（我方按选择行动，敌方 AI 自动）。
// 结果自动写回角色卡（HP/金币/背包战利品），主角倒下显示终局结算。

import { ref, computed, watch, nextTick } from 'vue'
import { tr } from '../../../composables/useI18n.js'
const props = defineProps({
  battle: { type: Object, required: true },
  events: { type: Array, default: () => [] }, // 初始事件（出现战等）
})
const emit = defineEmits(['close', 'settled', 'tick'])

const battle = ref(props.battle)
// 父级每回合用后端返回的 battle 覆盖更新
watch(() => props.battle, (v) => { if (v) battle.value = v })
const shownLog = ref([])
const logEl = ref(null)
const settledEmitted = ref(false) // 本场战斗 settled 是否已发（防重复；close 兜底补发用）
const picks = ref({}) // unitID -> action
const submitting = ref(false)
// ── 宝可梦式播报状态：事件逐条浮出 + 受击单位飘伤害数字 ──
const pendingEvents = ref([])  // 待播放事件队列
let playTimer = null
const popDmg = ref(null)       // { unitId, amount, crit, key } 当前飘字
const hitFlash = ref({})       // unitId -> timestamp，受击白闪标记

const myUnits = computed(() => battle.value.units.filter(u => u.side === 'ally'))
const enemyUnits = computed(() => battle.value.units.filter(u => u.side === 'enemy'))

// 头像底色（无头像时的首字母兜底色，按元素/阵营给色）
function unitColor(u) {
  const E = { fire: '#ff5a3c', water: '#3c8dff', wind: '#7fd8a4', earth: '#b07a2e', dark: '#9b6bff', light: '#ffd75e', holy: '#ffd75e', ice: '#6fd3ff', thunder: '#ffc93c' }
  return E[u.element] || (u.side === 'ally' ? '#8b5e7c' : '#e74c3c')
}
// 元素角标图标
function elementIcon(el) {
  return ({ fire: '🔥', water: '💧', wind: '🌪️', earth: '⛰️', dark: '🌑', light: '✨', holy: '✨', ice: '❄️', thunder: '⚡' })[el] || ''
}
// 受击震屏：结算事件里命中时短震一下（星迹 hit-stop 表现，纯视觉不碰数值）
const shaking = ref(false)
let shakeTimer = null
function triggerShake() {
  shaking.value = true
  if (shakeTimer) clearTimeout(shakeTimer)
  shakeTimer = setTimeout(() => { shaking.value = false }, 260)
}

// 技能名：按元素给角色专属招式名（表现层，让技能按钮有名有姓）
const ELEMENT_SKILL_NAMES = {
  fire: '烈焰斩', water: '水刃', wind: '疾风斩', earth: '地裂',
  dark: '暗影击', light: '圣光刃', holy: '圣光刃', ice: '霜华',
  lightning: '雷击', thunder: '雷击',
}
function skillLabel(u) {
  const el = (u.element || '').toLowerCase()
  return ELEMENT_SKILL_NAMES[el] || '元素爆发'
}

function pct(v, max) {
  return Math.max(0, Math.min(100, Math.round(((v || 0) / (max || 1)) * 100)))
}

// ── 宝可梦式播报（照抄星迹回合制演出语言）──────────────────────
// 事件拆成「行动行」+「效果行」：攻击/技能是行动行，暴击/克制/闪避是效果行；
// 伤害数字在受击单位头上飘。只抄显示方式，数值仍是后端权威结果。
function splitEvent(ev) {
  const m = ev.message || ''
  const parts = { head: m, effect: '' }
  if (ev.action === 'error') { parts.cls = 'error'; return parts }
  // 效果后缀拆出来：暴击（！）/属性克制/护盾吸收
  const crit = m.includes('暴击')
  const superHit = m.includes('克制')
  const shield = m.includes('护盾吸收')
  if (crit) parts.effect = '命中了要害！'
  if (superHit) parts.effect = '效果拔群！'
  if (shield) parts.effect = '护盾挡住了伤害'
  parts.cls = crit ? 'crit' : superHit ? 'super' : ev.action || ''
  return parts
}

// 播放队列：每 260ms 出一条事件，更新日志 + 飘伤害数字
function playEvents(events) {
  pendingEvents.value.push(...(events || []))
  if (playTimer) return
  playTimer = setInterval(() => {
    const ev = pendingEvents.value.shift()
    if (!ev) { clearInterval(playTimer); playTimer = null; return }
    shownLog.value.push(ev)
    // 自动滚到底部：宝可梦式播报从上往下逐条出现
    nextTick(() => {
      const el = logEl.value
      if (el) el.scrollTop = el.scrollHeight
    })
    // 受击飘字（有伤害且指向某个单位）+ 震屏（星迹 hit-stop 表现）
    if (ev.amount > 0 && ev.target) {
      popDmg.value = { unitId: ev.target, amount: ev.amount, crit: (ev.message || '').includes('暴击'), key: Date.now() }
      hitFlash.value = { ...hitFlash.value, [ev.target]: Date.now() }
      triggerShake()
      setTimeout(() => {
        if (popDmg.value && ev.target === popDmg.value.unitId) popDmg.value = null
      }, 1100)
    }
    if (shownLog.value.length > 12) shownLog.value.shift()
  }, 260)
}

function hitMark(id) {
  const t = hitFlash.value[id]
  if (!t) return false
  return Date.now() - t < 400
}
function popFor(unitId) {
  return popDmg.value && popDmg.value.unitId === unitId ? popDmg.value : null
}

// 单位阵营（区分染色：我方蓝 / 敌方红）
function unitSide(name) {
  if (!name) return ''
  const u = battle.value.units.find(x => x.name === name)
  return u ? u.side : ''
}
// 动作词（宝可梦式：「使用 攻击」/「使用 技能」）
function actionLabel(ev) {
  switch (ev.action) {
    case 'attack': return '攻击'
    case 'skill': return '技能'
    case 'defend': return '防御'
    case 'hurt': return '受伤'
    case 'miss': return '闪避'
    case 'shield': return '护盾'
    default: return ''
  }
}

// 初始事件（现身）先进日志
playEvents(props.events || [])

function picked(id) { return picks.value[id] || '' }
// 单人战斗（存活我方单位 ≤1）：点击即行动，直接提交结算，不需要「选中」再确认。
// 多人同框才走「先选中、结束回合统一结算」。
const isSolo = computed(() => myUnits.value.filter(u => u.alive).length <= 1)
function pick(id, action) {
  if (submitting.value) return
  if (isSolo.value) {
    // 单人：点一下就是行动，不进选中状态
    picks.value = { [id]: action }
    submitTurn()
    return
  }
  // 多人：先选中（可再点取消），结束回合一起结算
  const next = picks.value[id] === action ? '' : action
  picks.value = { ...picks.value, [id]: next }
}

// 结束回合：把本回合选择提交给后端统一结算，事件追加进日志，血条按快照刷新。
async function submitTurn() {
  if (submitting.value || battle.value.over) return
  submitting.value = true
  // 防卡死：后端限流/挂起时 fetch 会永久 pending，submitting 永远锁着 = 按钮按不了。
  // 加 10s 超时，超时 Abort 解锁并显示错误（不然快死时一点结算就卡死）。
  const ac = new AbortController()
  const timer = setTimeout(() => ac.abort(), 10000)
  try {
    const res = await fetch('/api/rp/battle/turn', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ battle_id: battle.value.id, actions: picks.value }),
      signal: ac.signal,
    })
    const d = await res.json()
    clearTimeout(timer)
    if (!d.ok) throw new Error(d.error || '结算失败')
    playEvents(d.events || [])
    battle.value = d.battle
    picks.value = {}
    // 每回合都通知父级：刷新角色卡状态 + 同步浮层里的 battle 快照
    emit('tick', d.battle)
    if (d.battle.over && !settledEmitted.value) {
      settledEmitted.value = true
      emit('settled', d.battle)
    }
  } catch (e) {
    // 结算失败：真实错误单独染色显示，绝不伪装成「闪避/落空」
    // AbortError = 我们自己的 10s 超时，给个能看懂的中文提示
    const msg = e?.name === 'AbortError' ? '结算超时（10秒）后端没响应，请稍后再试' : String(e.message || e)
    shownLog.value.push({ action: 'error', actor: '', message: '⚠️ ' + msg })
  } finally {
    clearTimeout(timer)
    submitting.value = false
  }
}

// 收起战场（结果面板/终局面板的唯一关闭按钮）：如果战斗已结束但 settled 还没
// 发给父级（比如用户直接点收起、或 settle 时序被吞），这里补发一次，
// 保证「收起后聊天必有战果播报」。close 无论如何都要执行——父级处理 settled
// 时若抛异常，会冒泡回来掐断 emit('close')，浮层就得点两次才关（用户实测的坑）。
function closeAndMaybeSettle() {
  if (battle.value.over && !settledEmitted.value) {
    settledEmitted.value = true
    try {
      emit('settled', battle.value)
    } catch (e) { console.error('settled 处理异常，已忽略以保证关闭', e) }
  }
  emit('close')
}
</script>
