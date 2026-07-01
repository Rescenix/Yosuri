<template>
  <div class="diff-panel">
    <div class="diff-branch-row">
      <span>main</span>
      <span class="arrow">→</span>
      <span class="worktree">working tree</span>
      <span class="diff-totals">{{ totals }}</span>
    </div>
    <div class="diff-body">
      <div class="diff-file-card" v-for="df in files" :key="df.name">
        <div class="diff-file-head" @click="$emit('toggle-file', df.name)">
          <span class="diff-chev" :class="{ open: !!expandedDiffs[df.name] }">›</span>
          <span class="diff-file-name">{{ df.name }}</span>
          <span class="diff-adds">{{ df.adds }}</span>
          <span class="diff-dels">{{ df.dels }}</span>
        </div>
        <div v-if="expandedDiffs[df.name]" class="diff-rows">
          <template v-for="(r, i) in df.rows" :key="i">
            <div v-if="r.gap" class="diff-gap">⋯ {{ r.gap }} ⋯</div>
            <div v-else class="diff-line" :class="'t-' + r.t">
              <span class="diff-lineno">{{ r.n }}</span>
              <span class="diff-sign" :class="'t-' + r.t">{{ r.t === 'add' ? '+' : (r.t === 'del' ? '−' : '') }}</span>
              <span class="diff-bar" :class="'t-' + r.t" :style="{ maxWidth: r.w + '%' }"></span>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  files: { type: Array, default: () => [] },
  expandedDiffs: { type: Object, default: () => ({}) },
  totals: { type: String, default: '' }
})
defineEmits(['toggle-file'])
</script>

<style scoped>
.diff-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.diff-branch-row {
  padding: 9px 14px;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
  border-bottom: 1px solid #e4dfd4;
  flex-shrink: 0;
}
.diff-branch-row .arrow { color: #a39c8f; font-weight: 400; }
.diff-branch-row .worktree { font-weight: 400; color: #696259; }
.diff-totals {
  margin-left: auto;
  font-weight: 600;
  font-size: 11.5px;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  color: #696259;
}

.diff-body {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  min-height: 0;
  padding: 10px 10px 14px;
}

.diff-file-card {
  border: 1px solid #e4dfd4;
  border-radius: 10px;
  margin-bottom: 10px;
  overflow: hidden;
  background: #ffffff;
}
.diff-file-head {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 8px 12px;
  cursor: pointer;
  background: #f4f2ec;
}
.diff-chev {
  display: inline-block;
  font-size: 12px;
  color: #a39c8f;
  transition: transform 0.15s ease;
}
.diff-chev.open { transform: rotate(90deg); }
.diff-file-name {
  flex: 1;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 12px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.diff-adds, .diff-dels {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 11.5px;
  font-weight: 600;
  flex-shrink: 0;
}
.diff-adds { color: #12b76a; }
.diff-dels { color: #d94834; }

.diff-rows { border-top: 1px solid #e4dfd4; }
.diff-gap {
  padding: 5px 12px;
  font-size: 10.5px;
  color: #a39c8f;
  text-align: center;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  background: #f4f2ec;
}
.diff-line {
  display: flex;
  align-items: center;
  padding: 3.5px 0;
}
.diff-line.t-add { background: rgba(18, 183, 106, 0.10); }
.diff-line.t-del { background: rgba(217, 72, 52, 0.08); }
.diff-lineno {
  width: 30px;
  flex-shrink: 0;
  text-align: right;
  font-size: 10.5px;
  color: #a39c8f;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
}
.diff-sign {
  width: 16px;
  flex-shrink: 0;
  text-align: center;
  font-size: 11px;
  font-weight: 700;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
}
.diff-sign.t-add { color: #12b76a; }
.diff-sign.t-del { color: #d94834; }
.diff-bar {
  display: block;
  height: 8px;
  border-radius: 3px;
  flex-grow: 1;
  margin: 0 14px 0 10px;
}
.diff-bar.t-add { background: rgba(18, 183, 106, 0.55); }
.diff-bar.t-del { background: rgba(217, 72, 52, 0.5); }
.diff-bar.t-ctx { background: rgba(163, 156, 143, 0.22); }
</style>
