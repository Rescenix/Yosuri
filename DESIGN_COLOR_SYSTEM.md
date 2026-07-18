# 多主题系统规范（Multi-Theme System Spec）

> 目的：支持在设置面板切换**几十种主题皮肤**。先内置 4 套验证架构：蓝 / 粉 / 紫 / 橙。
> 本文件是**执行依据**。改主题系统前先读此文件。
> 最后核对：2026-07-18，基于真实代码（非设想）。

---

## 1. 当前瓶颈（已实查）

- `useTheme.js`：`THEME_OPTIONS` 只有 `light / dark / system` 三态，`data-theme` 只能取 `light`/`dark`。
- `global.css`：`--app-*` 变量只定义了 `:root[data-theme='light']` 和 `:root[data-theme='dark']` 两组，写死在 CSS 里。
- `chat-window.css:1` 的 `--chat-*` 在 scoped 下取不到 `<html>` 的值，换肤**铺不满聊天窗**。
- 想支持几十种主题，靠「CSS 里枚举几十组 `:root[data-theme=xxx]`」会撑爆文件且不可维护。

**结论：必须改为运行时 JS 注入 CSS 变量**，主题数据从代码对象来，不再依赖 CSS 分组。

---

## 2. 目标架构

### 2.1 数据模型
主题 = **色板对象**。加一套主题 = 往 `THEME_PRESETS` 加一个对象，零 CSS 改动。

```js
// useTheme.js
export const THEME_PRESETS = {
  blue:   { label: '蓝',   accent: '#3b82f6', accentHover: '#2563eb', accentSoft: 'rgba(59,130,246,0.12)' },
  pink:   { label: '粉',   accent: '#ec4899', accentHover: '#db2777', accentSoft: 'rgba(236,72,153,0.12)' },
  purple: { label: '紫',   accent: '#a855f7', accentHover: '#9333ea', accentSoft: 'rgba(168,85,247,0.12)' },
  orange: { label: '橙',   accent: '#c96442', accentHover: '#b85737', accentSoft: 'rgba(201,100,66,0.12)' },
  // 后续几十套：在此追加一行即可
}
```

### 2.2 双轴：主题色板 × 明暗
保留「亮 / 暗 / 跟随系统」作为**亮度轴**，与主题色板正交：

```
最终变量 = THEME_PRESETS[theme].accent*   （每主题各一）
         × 中性面（surface/bg/text/border）= 亮度轴提供（light/dark 各一套共享中性）
```

- 中性面（bg/surface/text/border/code-bg/shadow）按亮度轴取，**所有主题共用同一套中性面** → 加主题只需定义 accent 三件套，极易扩展到几十种。
- 亮度轴三种状态：`light` / `dark` / `system`（system 解析为实际 light/dark）。

### 2.3 运行时注入（核心改造）
`applyTheme()` 不再只写 `data-theme` 属性，而是**把解析后的完整变量集写到 `<html>` 的 inline style**，覆盖 CSS 里写死的 `:root[data-theme=...]` 分组（那些分组可删）：

```js
function applyTheme() {
  const preset = THEME_PRESETS[theme.value] || THEME_PRESETS.orange
  const dark = resolvedMode() === 'dark'
  const neutral = dark ? NEUTRAL_DARK : NEUTRAL_LIGHT   // 中性面两套常量
  const root = document.documentElement
  root.setAttribute('data-theme', dark ? 'dark' : 'light') // 保留给个别依赖属性选择器的样式
  const vars = {
    '--app-accent': preset.accent,
    '--app-accent-hover': preset.accentHover,
    '--app-accent-soft': preset.accentSoft,
    '--app-bg': neutral.bg,
    '--app-surface': neutral.surface,
    '--app-surface-2': neutral.surface2,
    '--app-surface-3': neutral.surface3,
    '--app-text': neutral.text,
    '--app-text-soft': neutral.textSoft,
    '--app-text-faint': neutral.textFaint,
    '--app-border': neutral.border,
    '--app-border-soft': neutral.borderSoft,
    '--app-code-bg': neutral.codeBg,
    '--app-shadow': neutral.shadow,
  }
  for (const [k, v] of Object.entries(vars)) root.style.setProperty(k, v)
}
```

- `NEUTRAL_LIGHT` / `NEUTRAL_DARK` 是 JS 常量对象，值取自当前 `global.css` 里两套 `--app-*` 的中性部分（照搬，不丢）。
- `localStorage`：`aurora_theme`（色板名，如 `blue`）+ `aurora_mode`（light/dark/system），各自独立持久化。

---

## 3. 设置面板 UI（仿现有「配色主题」分段控件扩展）

现有 `SettingsModal.vue:121` 的「配色主题」从 3 按钮改成：
- **主题色板**：横排色卡网格（蓝/粉/紫/橙/…），每张显示 accent 色块 + label，当前项高亮边框。
- **亮度**：保留分段控件（亮 / 暗 / 跟随系统）。

切任意一项即时生效（`watch` 触发 `applyTheme`）。

---

## 4. 聊天窗必须跟随（修复 scoped 取不到值）

`chat-window.css:1` 的 `:root { --chat-* }` 在 Vue scoped 下被化成 `:root[data-v-xxx]`，匹配不到 `<html>`，导致聊天窗换肤无效。改造：
- `chat-window.css` 里的 `--chat-accent` / `--chat-user-bg` / `--chat-text-muted` / `--chat-text-soft` / 背景 `#fff` 等，改为引用 `--app-*`（已由 `<html>` inline style 注入，全局可见）。
- 保留纯几何变量 `--chat-radius-*`。
- 删掉 `chat-window.css` 顶部那个取不到值的 `:root` 变量块。

---

## 5. 清理（与多主题不冲突的部分）

- 删 `global.css` 里 `--primary` 蓝三变量（line 7-9）+ 其引用，蓝色调由 `THEME_PRESETS.blue` 提供，不再有游离蓝。
- 落地页 hero 渐变 / island-card / skill-tag / post-card 的写死蓝，改为 `var(--app-accent)`（跟随当前主题）。
- `body` 背景 → `var(--app-bg)`，跟随主题。

---

## 6. 语义辅助色（保留，非主题强调色）

context 横条分类色（紫 `#a78bfa` / 青 `#0f766e` / 蓝灰 `#3b82f6` / 橙 `#fb923c` / 灰 `#98a2b3` / 成功 `#12b76a` / 警告 `#d97706` / 危险 `#ef4444`）是数据可视化配色，**不随主题变**，保留。

---

## 7. 落地清单

- [ ] `useTheme.js`：新增 `THEME_PRESETS`（蓝/粉/紫/橙）、`NEUTRAL_LIGHT/DARK` 常量；`applyTheme` 改为运行时注入 inline style；`theme` 改为色板名、`mode` 独立；双 `watch` 持久化。
- [ ] `global.css`：删 `:root[data-theme=...]` 两组写死变量（改由 JS 注入）；删 `--primary` 蓝；`body` 背景→`var(--app-bg)`。
- [ ] `chat-window.css`：`--chat-*` 引用改 `--app-*`；删取不到值的 `:root` 块。
- [ ] `SettingsModal.vue`：配色主题 UI 改为色卡网格 + 亮度分段。
- [ ] 落地页/博客页蓝 → `var(--app-accent)`。
- [ ] 验收：4 套主题 × 亮/暗 各截图，确认整站（含聊天窗）跟随，无游离硬编码色。

---

## 8. 非目标

- 不开放「用户自定义任意颜色并保存多套」（本轮只内置预设，验证架构）。
- 不重构组件结构，仅改主题取值机制与颜色引用。
- context 横条分类色不随主题。
