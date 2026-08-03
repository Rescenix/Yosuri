#!/bin/bash
# Rescene Agent OS — 一键安装脚本 (Linux / macOS / git-bash)
# 用法: curl -fsSL https://raw.githubusercontent.com/Rescenix/ResceneAgent/main/agent-os/install.sh | sh
#
# 自动检测系统架构，下载对应二进制

set -e

GREEN='\033[32m'
BLUE='\033[34m'
RED='\033[31m'
NC='\033[0m'

echo ""
echo "╭──────────────────────────────────╮"
echo "│  Rescene Agent OS — 一键安装     │"
echo "╰──────────────────────────────────╯"
echo ""

# 检测系统
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo -e "${RED}❌${NC} 不支持的架构: $ARCH"; exit 1 ;;
esac

# 确定二进制名和下载地址
if [ "$OS" = "linux" ]; then
    BINARY="rescene-linux"
elif [ "$OS" = "darwin" ]; then
    BINARY="rescene-darwin"
else
    BINARY="rescene.exe"
fi

BASE_URL="https://raw.githubusercontent.com/Rescenix/ResceneAgent/main/agent-os"
INSTALL_DIR="$HOME/.rescene"
mkdir -p "$INSTALL_DIR"

# 下载二进制
echo -e "${BLUE}ℹ️${NC} 下载 rescene (${OS}/${ARCH})..."
if command -v curl &>/dev/null; then
    curl -fsSL "$BASE_URL/$BINARY" -o "$INSTALL_DIR/rescene"
    curl -fsSL "$BASE_URL/rescene-mascot.png" -o "$INSTALL_DIR/rescene-mascot.png" 2>/dev/null || true
elif command -v wget &>/dev/null; then
    wget -q "$BASE_URL/$BINARY" -O "$INSTALL_DIR/rescene"
    wget -q "$BASE_URL/rescene-mascot.png" -O "$INSTALL_DIR/rescene-mascot.png" 2>/dev/null || true
else
    echo -e "${RED}❌${NC} 需要 curl 或 wget"
    exit 1
fi
chmod +x "$INSTALL_DIR/rescene"
echo -e "${GREEN}✅${NC} 下载完成"

# 加入 PATH
SHELL_RC="$HOME/.bashrc"
if [ -f "$HOME/.zshrc" ]; then
    SHELL_RC="$HOME/.zshrc"
fi
if ! grep -q "\.rescene" "$SHELL_RC" 2>/dev/null; then
    echo "export PATH=\"\$PATH:\$HOME/.rescene\"" >> "$SHELL_RC"
    echo -e "${GREEN}✅${NC} 已加入 PATH ($SHELL_RC)"
    echo -e "${BLUE}ℹ️${NC} 运行 source $SHELL_RC 或重启终端"
else
    echo -e "${BLUE}ℹ️${NC} 已在 PATH 中"
fi
export PATH="$PATH:$INSTALL_DIR"

# 安装 chafa（看板娘渲染）
echo -e "${BLUE}ℹ️${NC} 安装 chafa..."
if command -v brew &>/dev/null; then
    brew install chafa 2>/dev/null || true
elif command -v apt &>/dev/null; then
    sudo apt install -y chafa 2>/dev/null || true
elif command -v pacman &>/dev/null; then
    sudo pacman -S --noconfirm chafa 2>/dev/null || true
else
    echo -e "${BLUE}ℹ️${NC} 提示: chafa 未安装，不影响使用"
fi

# 验证
echo ""
echo "╭──────────────────────────────────╮"
echo "│  ✅ 安装完成！                    │"
echo "╰──────────────────────────────────╯"
echo ""
echo "  现在输入:"
echo ""
echo "    rescene"
echo ""
echo "  或一键执行:"
echo ""
echo "    rescene exec '帮我查下系统信息'"
echo ""
echo "  免费模型已内置（无需 API Key）:"
echo "    - DeepSeek V4 Flash (OpenCode Zen)"
echo "    - Mimo 2.5"
echo "    - North Mini Code"
echo ""

"$INSTALL_DIR/rescene" version 2>/dev/null || echo "✅ 安装完成，请重启终端后运行 rescene"