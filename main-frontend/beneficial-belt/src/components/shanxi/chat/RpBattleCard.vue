<template>
  <div class="rp-battle-card">
    <!-- 战场：左我右敌 -->
    <div class="rp-battle-field">
      <div class="rp-battle-side allies">
        <div v-for="u in myUnits" :key="u.id" class="rp-battle-unit" :class="{ dead: !u.alive }">
          <span class="rp-battle-unit-name">{{ u.name }}</span>
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
        <div v-for="u in enemyUnits" :key="u.id" class="rp-battle-unit" :class="{ dead: !u.alive }">
          <span class="rp-battle-unit-name">{{ u.name }}</span>
          <div class="rp-battle-hpbar">
            <div v-if="u.shield > 0" class="rp-battle-shield" :style="{ width: pct(u.shield, u.maxHp) + '%' }"></div>
            <div class="rp-battle-hpfill" :style="{ width: pct(u.hp, u.maxHp) + '%' }"></div>
            <span class="rp-battle-hptext">{{ Math.max(0, u.hp) }}/{{ u.maxHp }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 战斗日志（最新几条，回合结算后滚动） -->
    <div v-if="shownLog.length" class="rp-battle-log">
      <div v-for="(ev, i) in shownLog" :key="i" class="rp-battle-logline" :class="ev.action">
        {{ ev.message }}
      </div>
    </div>

    <!-- 终局结算：主角倒下 → 世界线终局结算面板 -->
    <div v-if="battle.gameOver" class="rp-battle-ending">
      <div class="rp-ending-title">💀 世界线终局</div>
      <div class="rp-ending-row" v-if="battle.turn"><span>战斗回合</span><b>{{ battle.turn - 1 }}</b></div>
      <div class="rp-ending-row" v-if="battle.goldGain"><span>金币战利</span><b>{{ battle.goldGain }}</b></div>
      <div class="rp-ending-sub">{{ tr('主角倒下了，这条世界线到此为止。') }}</div>
      <button type="button" class="rp-battle-close" @click="$emit('close')">{{ tr('收起结算') }}</button>
    </div>

    <!-- 普通结果：胜利（含战利品）或战败 -->
    <div v-else-if="battle.over" class="rp-battle-result" :class="{ win: battle.victory }">
      <span v-if="battle.victory">🎉 胜利！</span>
      <span v-else>💀 战败…</span>
      <span v-if="battle.victory && battle.loot" class="rp-battle-gold">获得战利品 {{ battle.loot.icon }} {{ battle.loot.name }} ×{{ battle.loot.count }}</span>
      <button type="button" class="rp-battle-close" @click="$emit('close')">{{ tr('收起战场') }}</button>
    </div>

    <!-- 战斗进行中：逐格操作（每人一行攻击/技能/防御，点完结束回合统一结算） -->
    <div v-else class="rp-battle-controls">
      <div v-for="u in myUnits" :key="u.id" class="rp-battle-actor-row" :class="{ dead: !u.alive }">
        <span class="rp-battle-actor">{{ u.name }}</span>
        <span v-if="!u.alive" class="rp-battle-wait">{{ tr('已倒下') }}</span>
        <template v-else>
          <button type="button" class="rp-battle-btn atk" :class="{ picked: picked(u.id) === 'attack' }" @click="pick(u.id, 'attack')">⚔️ {{ tr('攻击') }}</button>
          <button type="button" class="rp-battle-btn def" :class="{ picked: picked(u.id) === 'defend' }" @click="pick(u.id, 'defend')">🛡️ {{ tr('防御') }}</button>
          <button type="button" class="rp-battle-btn skill" :class="{ picked: picked(u.id) === 'skill' }" :disabled="u.mp < 8" @click="pick(u.id, 'skill')">✨ {{ tr('技能') }}<span v-if="u.maxMp > 0">({{ u.mp }}/8)</span></button>
        </template>
      </div>
      <div class="rp-battle-turnbar">
        <span class="rp-battle-wait">{{ tr('第') }} {{ battle.turn }} {{ tr('回合') }}</span>
        <button type="button" class="rp-battle-close rp-battle-endturn" :disabled="submitting" @click="submitTurn">
          {{ submitting ? tr('结算中…') : tr('结束回合 ▶') }}
        </button>
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

import { ref, computed } from 'vue'
import { tr } from '../../../composables/useI18n.js'

const props = defineProps({
  battle: { type: Object, required: true },
  events: { type: Array, default: () => [] }, // 初始事件（出现战等）
})
const emit = defineEmits(['close', 'settled'])

const battle = ref(props.battle)
const shownLog = ref([])
const picks = ref({}) // unitID -> action
const submitting = ref(false)

const myUnits = computed(() => battle.value.units.filter(u => u.side === 'ally'))
const enemyUnits = computed(() => battle.value.units.filter(u => u.side === 'enemy'))

function pct(v, max) {
  return Math.max(0, Math.min(100, Math.round(((v || 0) / (max || 1)) * 100)))
}

// 初始事件（现身）先进日志
for (const ev of (props.events || [])) shownLog.value.unshift(ev)

function picked(id) { return picks.value[id] || '' }
function pick(id, action) {
  picks.value = { ...picks.value, [id]: picks.value[id] === action ? '' : action }
}

// 结束回合：把本回合选择提交给后端统一结算，事件追加进日志，血条按快照刷新。
async function submitTurn() {
  if (submitting.value || battle.value.over) return
  submitting.value = true
  try {
    const res = await fetch('/api/rp/battle/turn', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ battle_id: battle.value.id, actions: picks.value }),
    })
    const d = await res.json()
    if (!d.ok) throw new Error(d.error || '结算失败')
    for (const ev of (d.events || [])) shownLog.value.unshift(ev)
    battle.value = d.battle
    picks.value = {}
    if (d.battle.over) emit('settled', d.battle)
  } catch (e) {
    shownLog.value.unshift({ action: 'miss', message: String(e.message || e) })
  } finally {
    submitting.value = false
  }
}
</script>
