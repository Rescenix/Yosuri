<template>
  <section class="question-bar" role="group" :aria-label="question.question">
    <div class="question-bar-head">
      <span class="question-bar-mark">?</span>
      <span class="question-bar-title">{{ question.question }}</span>
      <span v-if="question.multi" class="question-bar-hint">可多选</span>
      <button class="question-bar-close" type="button" title="跳过" aria-label="跳过此问题" @click="onCancel">×</button>
    </div>

    <div v-if="question.options.length" class="question-bar-options">
      <button
        v-for="(opt, i) in question.options"
        :key="opt.value || i"
        class="question-option"
        :class="{ checked: isChecked(opt.value) }"
        type="button"
        :disabled="question.submitting"
        @click="onToggle(opt.value)"
      >
        <span class="question-option-key">{{ optionKey(i) }}</span>
        <span class="question-option-label">{{ optionLabel(opt.label, i) }}</span>
      </button>
      <form v-if="question.allowOther" class="question-other-row" @submit.prevent="onConfirm">
        <span class="question-option-key">{{ optionKey(question.options.length) }}</span>
        <input
          ref="freeInput"
          v-model="freeText"
          class="question-free-input"
          placeholder="其他（输入你的答案）"
        />
        <button class="question-submit" type="submit" :disabled="question.submitting || !freeText.trim()">发送</button>
      </form>
      <button
        v-if="question.multi"
        class="question-submit"
        type="button"
        :disabled="question.submitting || (!selected.length && !freeText.trim())"
        @click="onConfirm"
      >确认</button>
    </div>

    <form
      v-if="!question.options.length"
      class="question-bar-free"
      @submit.prevent="onConfirm"
    >
      <input
        ref="freeInput"
        v-model="freeText"
        class="question-free-input"
        :placeholder="question.options.length ? '输入其他回答' : '输入你的回答'"
      />
      <button class="question-submit" type="submit" :disabled="question.submitting || !freeText.trim()">发送</button>
    </form>
    <div v-if="question.error" class="question-error">{{ question.error }}</div>
  </section>
</template>

<script setup>
import { nextTick, onMounted, onUnmounted, ref } from 'vue'

const props = defineProps({
  question: { type: Object, required: true }
})
const emit = defineEmits(['answer'])

const selected = ref([])
const freeText = ref('')
const freeInput = ref(null)

function optionKey(i) {
  return i < 26 ? String.fromCharCode(65 + i) : String(i + 1)
}
function optionLabel(label, i) {
  const key = optionKey(i)
  return String(label || '').replace(new RegExp(`^${key}[.、:：)）\\s-]+`, 'i'), '')
}
function isChecked(value) {
  return selected.value.includes(value)
}
function onToggle(value) {
  if (!props.question.multi) {
    emit('answer', { id: props.question.id, answer: value, selected: [value] })
    return
  }
  const i = selected.value.indexOf(value)
  if (i >= 0) selected.value.splice(i, 1)
  else selected.value.push(value)
}
function onConfirm() {
  const text = freeText.value.trim()
  const values = selected.value.slice()
  const answer = text || values.join('、')
  if (!answer) return
  emit('answer', { id: props.question.id, answer, selected: text ? [] : values })
}
function onCancel() {
  emit('answer', { id: props.question.id, answer: '', selected: [] })
}
function onKey(event) {
  if (event.key === 'Escape') {
    onCancel()
    return
  }
  if (event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement) return
  const i = event.key.toUpperCase().charCodeAt(0) - 65
  if (i >= 0 && i < props.question.options.length) {
    event.preventDefault()
    onToggle(props.question.options[i].value)
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKey)
  if (!props.question.options.length) nextTick(() => freeInput.value?.focus())
})
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<style scoped>
.question-bar {
  display: grid;
  gap: 8px;
  margin: 0 0 8px;
  padding: 10px 12px;
  box-sizing: border-box;
  max-width: 100%;
  color: var(--app-text);
  background: var(--app-surface);
  border: 1px solid var(--app-accent-soft);
  border-left: 3px solid var(--app-accent);
  border-radius: 10px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.05);
  animation: question-bar-in 0.14s ease-out;
}
.question-error {
  padding: 7px 9px;
  border-radius: 7px;
  background: color-mix(in srgb, #d94834 8%, transparent);
  color: #c43d32;
  font-size: 12px;
}
@keyframes question-bar-in {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: none; }
}
.question-bar-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.question-bar-mark {
  display: grid;
  flex: 0 0 22px;
  height: 22px;
  place-items: center;
  border-radius: 999px;
  color: var(--app-accent);
  background: var(--app-accent-soft);
  font-size: 12px;
  font-weight: 750;
}
.question-bar-title {
  min-width: 0;
  font-size: 12.5px;
  font-weight: 600;
  line-height: 1.45;
}
.question-bar-hint {
  flex: none;
  color: var(--app-text-faint);
  font-size: 10.5px;
}
.question-bar-close {
  flex: none;
  margin-left: auto;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  color: var(--app-text-faint);
  background: transparent;
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
}
.question-bar-close:hover { background: var(--app-surface-3); color: var(--app-text); }
.question-bar-options {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 4px;
  padding-left: 30px;
}
.question-option,
.question-submit {
  min-height: 30px;
  border-radius: 8px;
  cursor: pointer;
  font: inherit;
}
.question-option {
  display: inline-flex;
  align-items: center;
  justify-content: flex-start;
  gap: 6px;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
  padding: 4px 9px 4px 5px;
  color: var(--app-text-soft);
  background: var(--app-surface-2);
  border: 1px solid var(--app-border);
  font-size: 12px;
  transition: border-color .14s ease, background .14s ease, color .14s ease;
}
.question-other-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 30px;
  width: 100%;
  box-sizing: border-box;
}
.question-option:hover,
.question-option.checked {
  color: var(--app-text);
  border-color: var(--app-accent);
  background: var(--app-accent-soft);
}
.question-option-key {
  display: grid;
  min-width: 20px;
  height: 20px;
  padding: 0 3px;
  place-items: center;
  box-sizing: border-box;
  border-radius: 5px;
  color: var(--app-accent);
  background: var(--app-surface);
  border: 1px solid var(--app-border-soft);
  font-size: 10px;
  font-weight: 750;
}
.question-option-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.question-bar-free {
  display: flex;
  gap: 6px;
  padding-left: 30px;
}
.question-free-input {
  flex: 1;
  min-width: 120px;
  height: 30px;
  box-sizing: border-box;
  padding: 4px 9px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  outline: none;
  color: var(--app-text);
  background: var(--app-surface-2);
  font: inherit;
  font-size: 12px;
}
.question-free-input:focus { border-color: var(--app-accent); }
.question-submit {
  flex: none;
  padding: 4px 12px;
  color: #fff;
  background: var(--app-accent);
  border: 1px solid var(--app-accent);
  font-size: 12px;
  font-weight: 600;
}
.question-submit:disabled { cursor: default; opacity: .45; }
@media (max-width: 640px) {
  .question-bar-options,
  .question-bar-free { padding-left: 0; }
}
</style>
