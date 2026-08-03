#!/bin/bash
# Rescene Agent OS — 一键安装脚本 (bash/git-bash)
# 用法: curl -fsSL https://rescene.dev/install.sh | sh
#
# 这行指令会：
# 1. 下载 rescene 到 ~/.rescene/
# 2. 加入 PATH
# 3. 安装 chafa（如有包管理器）
# 4. 验证安装成功

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

INSTALL_DIR="$HOME/.rescene"
mkdir -p "$INSTALL_DIR"

# 下载 rescene
BINARY_URL="https://github.com/undercurrent-ai/rescene/releases/latest/download/rescene.exe"
echo -e "${BLUE}ℹ️${NC} 下载 rescene.exe..."
if command -v curl &>/dev/null; then
    curl -fsSL "$BINARY_URL" -o "$INSTALL_DIR/rescene.exe"
elif command -v wget &>/dev/null; then
    wget -q "$BINARY_URL" -O "$INSTALL_DIR/rescene.exe"
else
    echo -e "${RED}❌${NC} 需要 curl 或 wget"
    exit 1
fi
chmod +x "$INSTALL_DIR/rescene.exe"
echo -e "${GREEN}✅${NC} 下载完成"

# 下载看板娘
MASCOT_URL="https://raw.githubusercontent.com/undercurrent-ai/rescene/main/agent-os/rescene-mascot.png"
echo -e "${BLUE}ℹ️${NC} 下载看板娘..."
if command -v curl &>/dev/null; then
    curl -fsSL "$MASCOT_URL" -o "$INSTALL_DIR/rescene-mascot.png"
else
    wget -q "$MASCOT_URL" -O "$INSTALL_DIR/rescene-mascot.png"
fi

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

# 安装 chafa（如有包管理器）
echo -e "${BLUE}ℹ️${NC} 安装 chafa..."
if command -v brew &>/dev/null; then
    brew install chafa 2>/dev/null || true
elif command -v apt &>/dev/null; then
    sudo apt install -y chafa 2>/dev/null || true
elif command -v pacman &>/dev/null; then
    sudo pacman -S --noconfirm chafa 2>/dev/null || true
else
    echo -e "${BLUE}ℹ️${NC} 提示: chafa 未安装，看板娘将使用备用渲染"
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
echo "  免费模型已内置（无需配置）:"
echo "    - DeepSeek V4 Flash (OpenCode Zen)"
echo "    - Mimo 2.5 (OpenCode Zen)"
echo "    - North Mini Code (OpenCode Zen)"
echo ""

"$INSTALL_DIR/rescene.exe" version