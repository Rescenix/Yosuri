// 演示模式（零 token 流式沙盒）：开启后，聊天框发任何消息都不再调后端，
// 而是本地用一段预置长对话「模拟」四态机 agentflow 的流式输出——思考块 +
// 富 markdown 回答按字符级联追加，直接复用 AgentWorkflowPanel 渲染，从而让
// streamFade 瀑布渐变被真实触发，方便在不花 token 的情况下肉眼验收动画。
// 与 streamFadeConfig 同模式：reactive + localStorage 持久化，设置面板直接读写。
import { reactive, watch } from 'vue'

const KEY = 'demoMode'

function loadPersisted() {
  try { return JSON.parse(localStorage.getItem(KEY) || '{}') } catch (e) { return {} }
}

export const demoMode = reactive({ enabled: false, ...loadPersisted() })

watch(demoMode, () => {
  try { localStorage.setItem(KEY, JSON.stringify(demoMode)) } catch (e) {}
}, { deep: true })

// 预置「长对话」样本：思考过程 + 一段**尽量覆盖所有 markdown/LaTeX 渲染路径**的富文本，
// 用来在零 token 下验收 streamFade 瀑布渐变对每种格式都不崩。覆盖：标题 / 有序无序列表 /
// 行内代码 / 代码围栏 / 引用 / 表格 / 行内公式 $...$ / 块级公式 $$...$$（\boxed \frac \sum）。
// 注意：模板串里的 ``` 必须写成 \`\`\` 以免提前结束模板；$ 公式无需转义，renderMarkdown 直接吃。
export const DEMO_THINKING =
`用户这条消息触发了演示模式。我先在心里拆解一下：
1. 他想要一个不花 token 就能看到的流式效果，所以必须本地模拟，不触网。
2. 复用现有 agentflow 渲染链路最稳——思考块走 flow-thinking-text，回答走 flow-intent markdown-body，
   这两个 class 已经被 streamFadePass 的扩展选择器命中，渐变会自动接上。
3. 字符级联追加的节奏要慢一点，让 fadeMs / staggerMs 的级联尾巴肉眼可辨。
4. 下面那段回答故意塞满标题、列表、代码、表格、行内公式和块级公式，验证每种格式在瀑布下都不炸。`

export const DEMO_INTENT =
`# 演示模式已就绪 ✨

这是一段**本地生成**的富文本，用来验收流式瀑布渐变——它**完全不消耗 token**，也不连后端。

## 你正在看的格式

- 标题 / 列表 / 行内代码 \`streamFadeConfig\` 同款级联淡入
- 引用块、表格、代码围栏都走同一套 \`renderMarkdown\`
- 行内公式如 $E = mc^2$ 与块级公式如下也都正常渲染

## 行内与块级公式

爱因斯坦质能方程写成行内是 $E = mc^2$，写成块级是：

$$
E = mc^2
$$

二次方程求根公式，用 \`\boxed\` 强调结果：

$$
x = \\frac{-b \\pm \\sqrt{b^2 - 4ac}}{2a}
$$

求和符号与分数同框：

$$
\\sum_{i=1}^{n} i = \\frac{n(n+1)}{2}
$$

## 代码示例

\`\`\`js
// 演示模式就靠这个逐字追加触发瀑布
async function tick(block, text) {
  for (let i = 0; i < text.length; i++) {
    block.text += text[i]   // 每 tick 加一个字
    await sleep(14)         // 节奏放慢，让模糊尾巴可辨
  }
}
\`\`\`

> 引用块也会跟着淡入。把「模糊强度」拉到 4px 再看，尾巴更像水流。

## 参数表

| 参数 | 含义 | 建议值 | 是否即时 |
| --- | --- | --- | --- |
| fadeMs | 单字淡入时长 | 500ms | 是 |
| staggerMs | 字间级联 | 14ms | 是 |
| blurPx | 起始模糊 | 2px | 是 |
| maxSweepMs | 大块扫过上限 | 350ms | 是 |

1. 有序列表第一项
2. 有序列表第二项，含行内公式 $\\alpha + \\beta = \\gamma$
3. 有序列表第三项

关掉右上角「演示模式」开关，聊天即恢复正常联网调用。祝你验收愉快 (｡•ᴗ•｡)`
