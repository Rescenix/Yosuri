# 什么是 Agent？从 ChatBox 开始：上下边缘模糊 + 流式渐变瀑布，把 AI 聊天做成本能

> 系列专栏【打造你自己的 Agent】第 1 篇 · 代码全部来自真实项目 [Rescene](https://github.com/Rescenix/ResceneAgent)

先给个一句话答案：**Agent 就是「会自己动手干活的 LLM」**。

普通聊天机器人是「你问一句，它答一句」，像个只会说话的书呆子。Agent 在此基础上多了三样东西：**工具**（能查网页、跑代码、点鼠标）、**记忆**（记得你是谁、上次聊到哪）、**循环**（自己拆任务、干一步看一步、干错了自己修）。一句话：聊天机器人是嘴，Agent 是嘴+手+脑子+记事本。

但很多教程一上来就讲 LangChain、讲 ReAct、讲多智能体编排，把新手直接劝退。我想换个讲法：**从你每天都会看到的 ChatBox 讲起**——因为一个 Agent 产品，用户 90% 的时间都在盯着聊天窗口看。它流不流畅、有没有「活着的感觉」，决定了用户愿不愿意用第二次。

今天这篇就用真实代码做三件事：

1. 用 **SSE 流式** 让 AI 开口说话（说一个字符、显示一个字符，而不是憋 10 秒一次性吐出来）
2. 用 **上下边缘模糊** 做出 Gemini 那种「内容从雾里滑入滑出」的呼吸感
3. 用 **流式渐变瀑布** 让新到的字符像瀑布一样级联淡入

这三件事做完，你的 ChatBox 就从「能用」变成「有生命力」。

---

## 一、先看 Agent 的最小骨架

不管多复杂的 Agent，剥到底都是这个循环：

```
用户输入 → LLM 思考 → 输出"要调用工具X(参数)" → 执行工具 → 结果回填 → LLM 继续
                                                              ↑__________|
```

- **LLM** 是大脑：负责理解、拆解、决策
- **工具** 是手脚：代码执行、浏览器、文件系统、搜索……
- **记忆** 是记事本：短期上下文 + 长期偏好
- **循环** 是心脏：驱动「思考→行动→观察」反复转

我们项目（Rescene，一个 Go + Vue 的二次元 Agent）把整个循环做成了**四态机**：`thinking（思考）→ intent（意图）→ action（行动）→ result（结果）`，每一态都通过 SSE 实时推给前端。这就是你看到的「AI 在干活」的完整过程——不是黑箱，而是每一步都摊开给你看。

而这一切的起点，就是 ChatBox 里那次「发送」。

---

## 二、让 AI 开口说话：SSE 流式

### 为什么不能等结果一次性返回？

LLM 生成 500 字可能要 5~10 秒。如果等完整结果再返回，用户盯着一个空白的「正在输入…」看十秒，99% 的人会关掉页面。流式（streaming）就是：**模型每吐出一个 token，立刻推给浏览器渲染**。用户看到第一个字在 1 秒内就出现，然后像打字机一样源源不断——「它活着」的感觉就出来了。

### 后端：Go 实现 SSE 只需要三行关键代码

SSE（Server-Sent Events）是 HTTP 协议上的单向实时通道，比 WebSocket 轻得多：**浏览器用原生 `EventSource` 就能收，不需要任何库**。服务端要做的只是：把响应头设成 `text/event-stream`，然后不断往连接里写特定格式的文本并 `Flush`。

这是 Rescene 后端（Go + Gin）真实的核心代码：

```go
// 事件写锁：Agent 的子代理 goroutine 会并发发事件，
// 而 gin 的 ResponseWriter 不是并发安全的，必须串行写。
var codeSSEMu sync.Mutex

func writeCodeSSE(c *gin.Context, event string, data map[string]any) {
	codeSSEMu.Lock()
	defer codeSSEMu.Unlock()
	data["type"] = event
	b, _ := json.Marshal(data)
	// SSE 协议格式：event: 事件名 \n data: JSON \n\n（空行结尾）
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, b)
	c.Writer.Flush() // 关键！Flush 把缓冲立即推给浏览器，不等响应结束
}
```

SSE 的响应头设置：

```go
c.Header("Content-Type", "text/event-stream")
c.Header("Cache-Control", "no-cache")   // 禁止缓存，否则浏览器会等连接结束才读
c.Header("Connection", "keep-alive")
```

就这么简单。`Flush()` 是灵魂——没有它，数据会堆在缓冲区直到请求结束才发出去，流式就变成了一次性返回。

### 事件契约：一个 Agent 有十几个「说话频道」

普通聊天只有一个频道（正文），但 Agent 需要同时汇报：思考过程、当前意图、调用哪个工具、工具进度、子代理进度、错误……如果全塞一个通道里，前端要写一堆解析逻辑。

SSE 的 `event:` 字段天然支持多频道——每个事件带一个名字，前端按名字订阅即可。这是 Rescene 的完整事件表（部分）：

| 事件 | 含义 |
| --- | --- |
| `workflow_start` | 工作流启动，下发 workflow_id |
| `model_info` | 当前用的模型 + 上下文占用分布 |
| `thinking` / `intent` | 思考过程、行动意图（文本增量） |
| `action_delta` | 工具参数流式生成中 |
| `action` / `result` | 工具开始执行 / 执行完成 |
| `approval_request` | 危险操作请求用户批准 |
| `subagent_*` | 子代理启动/进度/完成 |
| `flow_error` | 出错 |
| `workflow_done` | 整个工作流结束 |

### 前端：原生 EventSource，零依赖

```js
const es = new EventSource(`/api/code/workflow?task=${encodeURIComponent(task)}&session_id=${sid}`)

// 文本增量事件：追加到同类型的最后一个块，类型切换时开新块
const appendText = (type, text) => {
  if (!text) return
  const last = flow.blocks[flow.blocks.length - 1]
  if (last && last.type === type) last.text += text
  else flow.blocks.push({ type, text })
}

es.addEventListener('thinking', e => appendText('thinking', JSON.parse(e.data).content))
es.addEventListener('intent',    e => appendText('intent',    JSON.parse(e.data).content))
es.addEventListener('workflow_start', e => { flow.workflowId = JSON.parse(e.data).workflow_id })
es.addEventListener('flow_error', e => { /* 展示错误 */ })
es.addEventListener('workflow_done', e => { es.close(); /* 收尾 */ })

// 服务端正常结束响应也会触发 onerror（EventSource 会尝试重连），
// 要用自定义标记区分"正常结束"和"真断线"，避免正常结束还疯狂重连。
es.onerror = () => { /* 判断标记，决定重连还是关闭 */ }
```

到这里，你的 Agent 已经能「说话」了。但还差点味道——字是瞬间蹦出来的，太生硬。接下来给它上妆。

---

## 三、上下边缘模糊：Gemini 的呼吸感

你用过 Gemini 的话会发现一个细节：聊天内容滚到顶部/底部时，边缘不是硬生生的切断，而是**渐隐在雾里**。滚动时内容从雾里滑入、滑出，整个窗口像有生命一样呼吸。

Rescene 用两个 `sticky` 定位的模糊条实现（真实 CSS）：

```css
.msg-edge-blur.top {
  position: sticky;
  top: -24px;               /* 抵消容器的 padding-top:24px，贴住滚动视口顶边 */
  height: 44px;
  margin: 0 auto -44px;     /* 负外边距：不占布局高度，纯覆盖在内容之上 */
  z-index: 5;
  backdrop-filter: blur(6px);  /* 关键：把底下的内容模糊掉 */
  background: linear-gradient(to bottom,
    rgba(var(--app-surface-rgb), 0.85), rgba(var(--app-surface-rgb), 0));
  -webkit-mask-image: linear-gradient(to bottom, black 30%, transparent);
}

.msg-edge-blur.bottom {
  position: sticky;
  bottom: 0;                /* 吸在滚动视口底边，正好压在输入区上沿 */
  height: 40px;
  margin: -40px auto 0;
  z-index: 5;
  backdrop-filter: blur(6px);
  background: linear-gradient(to top,
    rgba(var(--app-surface-rgb), 0.85), rgba(var(--app-surface-rgb), 0));
  -webkit-mask-image: linear-gradient(to top, black 30%, transparent);
}
```

原理一句话：**`position: sticky` 让这个 div 永远吸附在滚动视口的边缘，`backdrop-filter: blur()` 把从它底下滚过的内容模糊掉，渐变 background + mask 让模糊只存在于边缘那一带**。内容滚动时穿过这个「雾带」，就有了滑入滑出的效果。

顶部和底部各一个，用户一眼就知道「上面还有内容」「下面快到输入框了」，比任何提示箭头都自然。

---

## 四、流式渐变瀑布：字符级级联淡入

模糊解决的是「纵向呼吸感」，但横向还有一个问题：**新到的字符是突然蹦出来的**。ChatGPT / Gemini 的做法是让字符按到达顺序级联淡入——像瀑布一样，一个字符淡入，紧接着下一个，形成一条「渐变的尾巴」。Rescene 把参数都开放成了可配置项（fadeMs/staggerMs/blurPx，用户能在设置面板调）。

实现难点在于：**正文是 v-html 整段重渲染的**，每个流式 chunk 都会把上一轮包的动画 span 冲掉。Rescene 的解法（真实代码）：

```js
const STREAM_SEG_CHARS = 2  // 每 2 个字符包一个 span（性能/细腻度折中）
const streamFadeState = new WeakMap()  // el -> { len, pending: [{start, bornAt}] }

function applyStreamFade(el) {
  const { fadeMs, staggerMs, maxSweepMs, blurPx } = streamFadeConfig
  let st = streamFadeState.get(el)
  if (!st) { st = { len: 0, pending: [] }; streamFadeState.set(el, st) }

  const now = performance.now()
  const nodes = collectStreamTextNodes(el)          // 收集纯文本节点
  const total = nodes.reduce((s, n) => s + n.nodeValue.length, 0)

  // 记录"新增了多少字符 + 这批是什么时候到的"
  if (total > st.len) { st.pending.push({ start: st.len, bornAt: now }); st.len = total }

  // 对新增区间逐个字符包 span，用"负 animation-delay"恢复已播进度：
  // 动画本来是从 0 播的，但字符已经出现了一会儿了，
  // 用 delay = 已过时间 就能让动画从中间接着播，视觉上无缝。
  for (const node of nodes) {
    // ... 跳过已包过 span 的节点 ...
    for (let i = plainEnd; i < text.length; i += STREAM_SEG_CHARS) {
      const span = document.createElement('span')
      span.className = 'stream-fade-seg'
      span.style.animationDuration = fadeMs + 'ms'
      span.style.animationDelay = ((pos - range.start) * staggerMs - (now - range.bornAt)).toFixed(1) + 'ms'
      span.style.setProperty('--sf-blur', blurPx + 'px')
      span.textContent = text.slice(i, i + STREAM_SEG_CHARS)
      // 动画播完立刻拆回纯文本：否则成千上万个带 will-change 的 span
      // 永久堆在已完成的消息里，滚动时会卡成"果冻抖动"
      span.addEventListener('animationend', () => {
        const p = span.parentNode
        if (p) p.replaceChild(document.createTextNode(span.textContent), span)
      }, { once: true })
      frag.appendChild(span)
    }
  }
}
```

配合的 CSS：

```css
@keyframes om-stream-fade {
  from { opacity: 0; filter: blur(var(--sf-blur, 2px)); }
  to   { opacity: 1; filter: blur(0); }
}
.stream-fade-seg {
  animation: om-stream-fade 0.5s ease-out both;
  will-change: opacity, filter;
}
```

核心 Trick 是**负的 `animation-delay`**：流式渲染是高频整段重绘，每次重绘我们重新包 span。但字符可能 100ms 前就出现了，重新从 0 播动画会闪一下。用 `delay = 字符已存在的时间`，动画就从「中间进度」接着播——**视觉上完全无缝**，用户根本感觉不到 DOM 被重造了。

另外注意：代码块、表格、公式（pre/code/table/katex）会被整段跳过不包 span——它们边吐边重排会抖，让它们直接整块出现更稳。

---

## 五、踩坑清单（都是真金白银换来的）

1. **`Flush()` 忘了调 = 流式变一次性返回**。SSE 的灵魂就是 Flush，且要配合 `Cache-Control: no-cache`，否则浏览器会缓冲到连接结束。
2. **gin 的 ResponseWriter 不是并发安全的**。Agent 的子代理 goroutine 并发发事件时，必须加锁串行写，否则偶发数据交错、崩溃。
3. **sticky 定位的 padding 抵消**。`sticky` 相对父级 content box 吸附，容器有 `padding-top` 时 blur 条会停在 padding 下沿，漏一条没被淡化的带子。用负 `top` 抵消，两个值必须绑在一起改。
4. **`backdrop-filter` 和 `transform` 不能同元素用**。曾经想用 `left:50% + translateX(-50%)` 居中 blur 条，结果背景采样取的是变换前的位置，左边露出一条没模糊的原图。改用 sticky 后同宽同中线，问题消失。
5. **flex 容器里不写 `flex-shrink: 0`，blur 条会被内容压成 0 高**。
6. **动画播完必须拆 span**。带 `will-change` 的 span 会创建合成层，上万个永久堆积 → 滚动时全量重合成，果冻抖动。`animationend` 里拆回纯文本节点，零图层零开销。
7. **EventSource 正常结束也会触发 `onerror`**（它默认会重连）。要用自定义标记区分「服务端正常收尾」和「真断线」，否则 Agent 干完活浏览器还在疯狂重连。

---

## 六、关于 Rescene（项目介绍）

上面所有代码都来自我手写的开源项目 **Rescene**——一个专攻**前端设计、浏览器自动化、Computer Use** 的二次元 Agent：

- 🎨 **专攻前端设计**：内置 54 个真实设计系统参考（Linear / Vercel / Stripe / Notion…），Agent 写完直接真实渲染给你看
- 🌐 **真实浏览器自动化**：基于 Chromium + CDP，不是截图假装，是真浏览器在跑你的页面
- 🖱️ **Computer Use**：能操作桌面——截图、移动鼠标、点击、键入、拖拽、滚动
- 🔋 **免费模型每日更新**：每天自动探测各厂商免费档模型，免费池永远是真能跑的
- 🧠 **成长中的记忆**：每次工作流完成后自动萃取经验，下次自动融入上下文
- 🔧 **4+4+2 Agent 工作流**：40% 计划 → 40% 验证 → 20% 编码，任务中断恢复、全链路交付验证

> 🔗 官网：https://rescene.shanca.me/ （全速下载最新发行版）
> 🐙 GitHub：https://github.com/Rescenix/ResceneAgent （Issues 区欢迎交流）

本专栏所有文章的代码都将从 Rescene 的真实实现里取材——你看到的每一个技巧，都是在真实产品里跑过的，不是教程 Demo。

---

## 下篇预告

第 2 篇：《LLM 接入与多模型路由：让 Agent 学会「换脑」》——OpenAI 兼容协议怎么做统一抽象、免费模型池怎么搭、上游限流了怎么优雅降级。到时候见～

---

*📌 发布小贴士（对读者）：三平台同步更，CSDN 版标题建议带上关键词（如「Agent 聊天窗口 SSE 流式实现」），掘金版可多贴代码，知乎版开头多放观点。本文所有代码均可在 Rescene 仓库找到完整实现。*
