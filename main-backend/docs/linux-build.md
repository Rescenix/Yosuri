# Rescene Agent 桌面版 — Linux 构建指南

> 面向开源贡献者：在 Linux 上编译 Rescene 桌面客户端（Wails v2 + Go + Vue3）。
> Windows 构建走 `scripts/package-windows.ps1`，本指南只讲 Linux。

## 1. 系统依赖

### Debian / Ubuntu（含 WSL2 + GUI）

```bash
sudo apt update
sudo apt install -y build-essential pkg-config \
    libgtk-3-dev libwebkit2gtk-4.1-dev \
    libayatana-appindicator3-dev librsvg2-dev \
    libglib2.0-dev libsoup-3.0-dev libjavascriptcoregtk-4.1-dev \
    libgbm-dev
```

> WebKitGTK 4.1 是 Wails v2.13 推荐版本；老发行版没有 4.1 时装 `libwebkit2gtk-4.0-dev` 也可以（脚本自动识别 4.0/4.1）。

### Fedora

```bash
sudo dnf install gcc-c++ pkg-config gtk3-devel webkit2gtk4.1-devel \
    libappindicator-gtk3-devel librsvg2-devel glib2-devel \
    libsoup3-devel javascriptcoregtk4.1-devel
```

### Arch

```bash
sudo pacman -S --needed gcc pkg-config gtk3 webkit2gtk-4.1 \
    libappindicator-gtk3 librsvg glib2 libsoup3 javascriptcoregtk
```

## 2. 安装 Go 与 Wails CLI

```bash
# Go 1.22+（https://go.dev/dl/）
go version

# Wails CLI（与 wails.json / go.mod 锁定的 v2.13.0 保持一致）
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
export PATH="$PATH:$(go env GOPATH)/bin"
```

## 3. 构建

一键脚本（推荐）：

```bash
cd ResceneAgent
bash scripts/build-linux.sh amd64        # 或 arm64
```

产物：

```
dist/Rescene-linux-amd64/
├── rescene          # 主程序（wails build 产物）
└── assets/          # 资源（如有）
dist/Rescene-linux-amd64.tar.gz
```

启动：

```bash
./dist/Rescene-linux-amd64/rescene
```

### 手动构建（不想用脚本）

```bash
cd main-backend

# 1. 前端
cd frontend
npm install
node ./node_modules/vite/bin/vite.js build
node ./copy-dist.mjs
cd ..

# 2. 桌面程序
export GOCACHE="$PWD/.codex-go-cache"
mkdir -p "$GOCACHE/tmp"
export GOTMPDIR="$GOCACHE/tmp"

APP_VERSION="$(grep -oP '"productVersion"\s*:\s*"\K[^"]+' wails.json)"
wails build -platform linux/amd64 \
    -o rescene \
    -ldflags "-X backend/internal/handler.AppVersion=$APP_VERSION"

# 产物在 build/bin/rescene
```

## 4. 已知平台差异（桌面版在 Linux 上的行为）

| 能力 | Windows | Linux |
|------|---------|-------|
| 桌面托盘 | ✅ `desktop_tray_windows.go` | ⚠️ `desktop_tray_other.go` 空实现，托盘不可用 |
| 开机自启 | ✅ `auto_start_windows.go` | ⚠️ `auto_start_other.go` 空实现 |
| DHS 插件自动重启 | ✅ taskkill + PowerShell + SysProcAttr | ⚠️ 降级为提示手动重启（`dhs_restart_other.go`） |
| Computer Use（截图/键鼠） | ✅ 完整实现 | ⚠️ 1×1 占位图 stub（`computer_use_stub.go`） |
| 更新器（热更新/一键安装） | ✅ 完整链路 | ⚠️ 未验证，`updater_process_other.go` 兜底 |

这些 `_other.go` 兜底保证 **Linux 能编译能启动**，但托盘/自启/更新等 Windows 专属功能在 Linux 上是空操作或未实机回归。欢迎贡献者补齐对应实现。

## 5. 常见问题

- **`pkg-config: not found`** → 缺 `pkg-config` 包，装 build-essential。
- **`WebKitGTK not found`** → 装 `libwebkit2gtk-4.1-dev`；Arch 用户注意 webkit2gtk-4.1 与 webkit2gtk 是两个包。
- **`undefined: syscall.SysProcAttr` 字段错误** → 用了 Windows 专属字段，必须拆到 `_windows.go`（`//go:build windows`）文件，参考 `dhs_restart_windows.go` 模式。
- **托盘/图标不显示** → Linux 上 `libayatana-appindicator3` 未装；GNOME 需扩展 `AppIndicator and KStatusNotifierItem Support`。
- **`go vet ./...` 只验证当前平台**：要验证 Linux 编译请直接 `wails build`，或 `GOOS=linux go vet ./...` 做类型检查（不带 CGO 依赖的包可用）。
