<template>
  <div class="editor-overlay" @click.self="$emit('close')">
    <div class="editor-panel">
      <!-- 顶部操作栏 -->
      <div class="editor-header">
        <h3>编辑生命线</h3>
        <div class="header-actions">
          <button class="add-btn" @click="openAddDialog">+ 新增节点</button>
          <button class="save-btn" @click="saveAll">保存</button>
          <button class="tb-btn" @click="$emit('close')">取消</button>
        </div>
      </div>

      <!-- 列表区域（倒序） -->
      <div class="editor-list">
        <div v-for="node in reversedData" :key="node._id" class="editor-node">
          <div class="node-summary" @click="toggleExpand(node._id)">
            <span class="node-name">{{ node.name || '新节点' }}</span>
            <span class="node-date">{{ node.date }}</span>
            <span class="node-type">{{ node.type === 'major' ? '重大' : '小更新' }}</span>
            <button class="tb-btn" @click.stop="removeNode(node._id)">删除</button>
          </div>
          <div v-if="expandedId === node._id" class="node-edit">
            <label>名称 <input v-model="node.name" /></label>
            <label>日期 <input v-model="node.date" type="date" /></label>
            <label>类型
              <select v-model="node.type">
                <option value="major">重大更新</option>
                <option value="minor">小更新</option>
              </select>
            </label>
            <div class="changes-editor">
              <p>更新内容</p>
              <div v-for="(change, ci) in node.changes" :key="ci" class="change-row">
                <input v-model="node.changes[ci]" />
                <button class="tb-btn" @click="removeChange(node._id, ci)">✕</button>
              </div>
              <button class="tb-btn" @click="addChange(node._id)">+ 添加</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 新增节点弹窗 -->
    <div v-if="showAddDialog" class="add-dialog-overlay" @click.self="closeAddDialog">
      <div class="add-dialog-panel">
        <h3>新增版本节点</h3>
        <label>名称 <input v-model="newNode.name" /></label>
        <label>日期 <input v-model="newNode.date" type="date" /></label>
        <label>类型
          <select v-model="newNode.type">
            <option value="major">重大更新</option>
            <option value="minor">小更新</option>
          </select>
        </label>
        <div class="changes-editor">
          <p>更新内容</p>
          <div v-for="(change, ci) in newNode.changes" :key="ci" class="change-row">
            <input v-model="newNode.changes[ci]" />
            <button class="tb-btn" @click="removeNewChange(ci)">✕</button>
          </div>
          <button class="tb-btn" @click="newNode.changes.push('')">+ 添加</button>
        </div>
        <div class="dialog-actions">
          <button class="save-btn" @click="confirmAddNode">确定添加</button>
          <button class="tb-btn" @click="closeAddDialog">取消</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps(['initialData', 'onClose', 'onSaved'])
const editableData = ref([])
const expandedId = ref(null) // 当前展开的节点_id
let idCounter = 0 // 用于给节点生成唯一_id

// 倒序显示
const reversedData = computed(() => {
  return [...editableData.value].reverse()
})

// 新增弹窗状态
const showAddDialog = ref(false)
const newNode = ref({
  name: '',
  date: new Date().toISOString().slice(0, 10),
  type: 'minor',
  changes: ['']
})

onMounted(() => {
  const raw = JSON.parse(JSON.stringify(props.initialData || []))
  // 给每个节点添加唯一_id（如果没有）
  editableData.value = raw.map(node => ({
    ...node,
    _id: `node-${idCounter++}`
  }))
})

// 切换展开
const toggleExpand = (id) => {
  expandedId.value = expandedId.value === id ? null : id
}

// 删除节点
const removeNode = (id) => {
  if (confirm('确定删除？')) {
    editableData.value = editableData.value.filter(n => n._id !== id)
  }
}

// 添加一行变更
const addChange = (nodeId) => {
  const node = editableData.value.find(n => n._id === nodeId)
  if (node) node.changes.push('')
}

// 删除一行变更
const removeChange = (nodeId, changeIndex) => {
  const node = editableData.value.find(n => n._id === nodeId)
  if (node) node.changes.splice(changeIndex, 1)
}

// 新增弹窗操作
const openAddDialog = () => {
  newNode.value = {
    name: '',
    date: new Date().toISOString().slice(0, 10),
    type: 'minor',
    changes: ['']
  }
  showAddDialog.value = true
}

const closeAddDialog = () => {
  showAddDialog.value = false
}

const removeNewChange = (index) => {
  newNode.value.changes.splice(index, 1)
}

const confirmAddNode = () => {
  if (!newNode.value.name.trim()) {
    alert('请填写版本名称')
    return
  }
  const nodeToAdd = {
    ...newNode.value,
    _id: `node-${idCounter++}`
  }
  editableData.value.push(nodeToAdd) // 追加到末尾（倒序后会在最上面）
  showAddDialog.value = false
}

// 保存全部
const saveAll = () => {
  // 传递给父组件的数据要去掉_id字段
  const cleanData = editableData.value.map(({ _id, ...rest }) => rest)
  if (props.onSaved) {
    props.onSaved(cleanData)
  }
  if (props.onClose) {
    props.onClose()
  }
}
</script>

<style scoped>
/* 复用之前的样式，并添加新增弹窗样式 */
.editor-overlay {
  position: fixed; inset: 0;
  background: rgba(0,0,0,0.3);
  z-index: 50;
  display: flex;
  justify-content: center;
  align-items: center;
}
.editor-panel {
  background: #fff;
  width: 90%;
  max-width: 700px;
  max-height: 80vh;
  border-radius: 14px;
  overflow-y: auto;
  padding: 16px;
}
.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  position: sticky;
  top: 0;
  background: #fff;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
  z-index: 1;
}
.header-actions {
  display: flex;
  gap: 8px;
}
.save-btn {
  background: var(--primary);
  color: #fff;
  border: none;
  padding: 6px 16px;
  border-radius: 6px;
  cursor: pointer;
}
.add-btn {
  background: var(--bg-card);
  border: 1px solid var(--primary);
  color: var(--primary);
  padding: 6px 16px;
  border-radius: 6px;
  cursor: pointer;
}
.editor-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.editor-node {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px;
}
.node-summary {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
  font-weight: 600;
}
.node-edit {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.node-edit label {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 0.9rem;
}
.node-edit input, .node-edit select, .add-dialog-panel input, .add-dialog-panel select {
  padding: 6px;
  border: 1px solid var(--border);
  border-radius: 4px;
}
.changes-editor {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.change-row {
  display: flex;
  gap: 6px;
}
.change-row input {
  flex: 1;
}
.tb-btn {
  background: var(--bg-card);
  border: 1px solid var(--border);
  padding: 4px 10px;
  border-radius: 6px;
  cursor: pointer;
}
/* 新增弹窗样式 */
.add-dialog-overlay {
  position: fixed; inset: 0;
  background: rgba(0,0,0,0.4);
  z-index: 60;
  display: flex;
  justify-content: center;
  align-items: center;
}
.add-dialog-panel {
  background: #fff;
  width: 90%;
  max-width: 500px;
  border-radius: 14px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.add-dialog-panel h3 {
  margin: 0 0 8px 0;
}
.add-dialog-panel label {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 0.9rem;
}
.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
}
</style>