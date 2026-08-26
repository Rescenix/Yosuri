#!/usr/bin/env bash
# Rescene Agent 桌面版 — Linux 一键构建脚本
# 用法: bash scripts/build-linux.sh [arch]
#   arch: amd64 (默认) | arm64
#
# 产物:
#   dist/Rescene-linux-<arch>/          —— 运行目录（含 binary + assets）
#   dist/Rescene-linux-<arch>.tar.gz    —— 打包压缩包
#   dist/SHA256SUMS.txt                 —— 校验和（追加 Linux 行）
#
# 前置依赖（Debian/Ubuntu）:
#   sudo apt install -y build-essential pkg-config libgtk-3-dev \
#     libwebkit2gtk-4.1-dev libayatana-appindicator3-dev librsvg2-dev \
#     libglib2.0-dev libsoup-3.0-dev libjavascriptcoregtk-4.1-dev
#   go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0

set -euo pipefail

ARCH="${1:-amd64}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BACKEND="$ROOT/main-backend"
DIST="$ROOT/dist"
OUT_DIR="$DIST/Rescene-linux-$ARCH"
BIN_NAME="rescene"

echo "==> 目标: linux/$ARCH"

# 1. 检查 wails CLI
if ! command -v wails >/dev/null 2>&1; then
    GO_BIN="$(go env GOPATH)/bin"
    if [ -x "$GO_BIN/wails" ]; then
        export PATH="$GO_BIN:$PATH"
    else
        echo "❌ 未找到 wails CLI，请先: go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0" >&2
        exit 1
    fi
fi

# 2. 检查 Linux 桌面构建依赖（WebKitGTK 等）
echo "==> 检查系统依赖..."
pkg-config --exists gtk+-3.0 || { echo "❌ 缺少 GTK3 开发库，请安装 libgtk-3-dev" >&2; exit 1; }
if pkg-config --exists webkit2gtk-4.1; then
    echo "    WebKitGTK 4.1 ✓"
elif pkg-config --exists webkit2gtk-4.0; then
    echo "    WebKitGTK 4.0 ✓ (建议升级到 4.1)"
else
    echo "❌ 缺少 WebKitGTK 开发库，请安装 libwebkit2gtk-4.1-dev (或 4.0)" >&2
    exit 1
fi

# 3. 前端构建 + 同步到 embed
echo "==> 构建前端 (vite)..."
pushd "$BACKEND/frontend" >/dev/null
npm run install:app >/dev/null 2>&1 || npm install --no-audit --no-fund
node ./node_modules/vite/bin/vite.js build
node ./copy-dist.mjs
popd >/dev/null

# 4. wails build（Linux 原生交叉编译）
echo "==> wails build -platform linux/$ARCH ..."
pushd "$BACKEND" >/dev/null
# 版本从 wails.json info.productVersion 注入，与 Windows 流程一致
APP_VERSION="$(grep -oP '"productVersion"\s*:\s*"\K[^"]+' wails.json)"
GOCACHE="$BACKEND/.codex-go-cache" GOTMPDIR="$BACKEND/.codex-go-cache/tmp" \
    wails build \
    -platform "linux/$ARCH" \
    -o "$BIN_NAME" \
    -ldflags "-X backend/internal/handler.AppVersion=$APP_VERSION"
popd >/dev/null

# 5. 组装运行目录
echo "==> 组装 $OUT_DIR ..."
rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"
cp "$BACKEND/build/bin/$BIN_NAME" "$OUT_DIR/$BIN_NAME"
# 非 Linux 专属 assets 一并带上（若存在）
[ -d "$BACKEND/assets" ] && cp -r "$BACKEND/assets" "$OUT_DIR/"

# 6. 打包 + 校验和
echo "==> 打包 tar.gz ..."
tar -C "$DIST" -czf "$DIST/Rescene-linux-$ARCH.tar.gz" "Rescene-linux-$ARCH"

# 追加 Linux 行到 SHA256SUMS（不覆盖已有 Windows 行）
SHA256SUMS="$DIST/SHA256SUMS.txt"
{
    echo "$(sha256sum "$DIST/Rescene-linux-$ARCH.tar.gz" | cut -d' ' -f1) *Rescene-linux-$ARCH.tar.gz"
} >> "$SHA256SUMS"

echo ""
echo "✅ 完成！"
echo "    运行目录: dist/Rescene-linux-$ARCH/"
echo "    压缩包:   dist/Rescene-linux-$ARCH.tar.gz"
echo "    启动:     ./dist/Rescene-linux-$ARCH/rescene"
