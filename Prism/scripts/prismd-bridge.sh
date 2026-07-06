#!/usr/bin/env bash
# prismd-bridge.sh — Claude Code memory/ ↔ PrismD ENGRAM 桥接
# 用法:
#   ./prismd-bridge.sh create path/to/memory/xxx.md
#   ./prismd-bridge.sh update path/to/memory/xxx.md
#   ./prismd-bridge.sh query "关键词"
#
# 依赖: PrismD 服务在 localhost:5666 运行

set -euo pipefail

PORT="${PRISMD_PORT:-5666}"
DOMAIN="${PRISMD_DOMAIN:-Claude}"
PRISMD="http://127.0.0.1:${PORT}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TRACK_DIR="${SCRIPT_DIR}/../data/bridge_tracking"
mkdir -p "$TRACK_DIR"

# ── 发送 PrimQL ──
send() {
    curl -s --max-time 3 -X POST "$PRISMD" -d "$1" 2>/dev/null || {
        echo "[bridge] ✗ PrismD 不可达 ($PORT)" >&2
        return 1
    }
}

# ── 追踪：每个 memory name 一个文件，内容是 nodeId
track_get() {
    local name="$1"
    local f="${TRACK_DIR}/${name}.txt"
    [ -f "$f" ] && cat "$f" || echo "0"
}

track_set() {
    local name="$1" node_id="$2"
    echo "$node_id" > "${TRACK_DIR}/${name}.txt"
}

track_del() {
    local name="$1"
    rm -f "${TRACK_DIR}/${name}.txt"
}

# ── 解析 frontmatter ──
parse_fm() {
    local file="$1"
    local name desc typ body

    name=$(grep -m1 '^name:' "$file" 2>/dev/null | sed 's/^name:\s*//;s/^["'"'"']//;s/["'"'"']$//' | xargs)
    [ -z "$name" ] && name=$(basename "$file" .md)

    desc=$(grep -m1 '^description:' "$file" 2>/dev/null | sed 's/^description:\s*//;s/^["'"'"']//;s/["'"'"']$//' | xargs)
    [ -z "$desc" ] && desc="$name"

    typ=$(grep -m1 'type:' "$file" 2>/dev/null | sed 's/.*type:\s*//;s/^["'"'"']//;s/["'"'"']$//' | xargs)
    [ -z "$typ" ] && typ="reference"

    # body: frontmatter 之后前 5 行
    body=$(awk '/^---$/{if(++c==2){next}}c>=2{print}' "$file" \
        | grep -v '^\s*$' \
        | grep -v '^\s*#' \
        | grep -v '^```' \
        | head -5 \
        | tr '\n' ' ' \
        | cut -c1-300)

    echo "${name}|${desc}|${typ}|${body}"
}

# ── 主逻辑 ──

ACTION="${1:-}"
ARG="${2:-}"

send "DOMAIN USE ${DOMAIN}" > /dev/null

case "$ACTION" in
    query)
        [ -z "$ARG" ] && { echo "用法: $0 query <关键词>"; exit 1; }
        send "LOOM ${ARG}"
        ;;

    create)
        FILE="$ARG"
        [ ! -f "$FILE" ] && { echo "[bridge] ✗ 文件不存在: $FILE"; exit 1; }

        IFS='|' read -r name desc typ body <<< "$(parse_fm "$FILE")"
        role="${typ}-memory"
        text="${desc}: ${body}"

        echo "[bridge] ENGRAM $role → $DOMAIN 域"
        resp=$(send "ENGRAM $role $text")

        if echo "$resp" | grep -q 'OK [0-9]'; then
            node_id=$(echo "$resp" | sed 's/OK //;s/[^0-9]//g')
            track_set "$name" "$node_id"
            echo "[bridge] ✓ 节点 #${node_id} 已创建 [$name]"
        else
            echo "[bridge] ✗ ENGRAM 失败: $resp" >&2
        fi
        ;;

    update)
        FILE="$ARG"
        [ ! -f "$FILE" ] && { echo "[bridge] ✗ 文件不存在: $FILE"; exit 1; }

        IFS='|' read -r name desc typ body <<< "$(parse_fm "$FILE")"
        node_id=$(track_get "$name")

        if [ "$node_id" = "0" ]; then
            echo "[bridge] ⚠ 未找到 $name 的已有映射, 改为创建"
            "$0" create "$FILE"
            exit $?
        fi

        role="${typ}-memory"
        text="${desc}: ${body}"
        echo "[bridge] REFRACT #${node_id} → $DOMAIN 域 [$name]"
        json="{\"id\":$node_id,\"content\":\"$text\",\"role\":\"$role\"}"
        resp=$(send "REFRACT $json")

        if echo "$resp" | grep -q 'OK'; then
            echo "[bridge] ✓ 节点 #${node_id} 已更新"
        else
            echo "[bridge] ✗ REFRACT 失败: $resp" >&2
        fi
        ;;

    delete)
        FILE="$ARG"
        [ ! -f "$FILE" ] && { echo "[bridge] ✗ 文件不存在: $FILE"; exit 1; }
        name=$(basename "$FILE" .md)
        node_id=$(track_get "$name")

        if [ "$node_id" = "0" ]; then
            echo "[bridge] ⚠ 未找到映射, 跳过"
            exit 0
        fi

        echo "[bridge] PRUNE #${node_id} [$name]"
        send "PRUNE $node_id" > /dev/null
        track_del "$name"
        echo "[bridge] ✓ 已遗忘"
        ;;

    list-tracking)
        echo "=== PrismD → memory 映射 ==="
        for f in "$TRACK_DIR"/*.txt; do
            [ -f "$f" ] || continue
            name=$(basename "$f" .txt)
            nid=$(cat "$f")
            echo "  $name → 节点 #$nid"
        done
        ;;

    *)
        echo "用法: $0 {create|update|delete|query|list-tracking} <参数>"
        exit 1
        ;;
esac

# 切回 Atri
send "DOMAIN USE Atri" > /dev/null
