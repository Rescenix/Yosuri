#!/bin/bash
# Rescene 发版后清 Cloudflare CDN 缓存（上传新包后必跑，否则粉丝下到旧缓存）
# 用法: CLOUDFLARE_API_TOKEN=<token> bash purge-cache.sh [zone]
# 或:   bash purge-cache.sh          # 从环境变量读 token
# zone 默认 shanca.me，可用参数覆盖
set -euo pipefail

ZONE="${1:-shanca.me}"
TOKEN="${CLOUDFLARE_API_TOKEN:-}"
if [ -z "$TOKEN" ]; then
  echo "❌ 缺少 CLOUDFLARE_API_TOKEN 环境变量" >&2
  echo "用法: CLOUDFLARE_API_TOKEN=<token> bash purge-cache.sh" >&2
  exit 1
fi

echo "🔍 解析 zone: $ZONE"
ZONE_ID=$(curl -s "https://api.cloudflare.com/client/v4/zones?name=$ZONE" \
  -H "Authorization: Bearer $TOKEN" --max-time 30 \
  | python -c "import json,sys; d=json.load(sys.stdin); print(d['result'][0]['id'] if d.get('success') else '')")
if [ -z "$ZONE_ID" ]; then
  echo "❌ zone 解析失败（token 权限不足或 zone 名不对）" >&2
  exit 1
fi
echo "✅ zone ID: $ZONE_ID"

echo "🧹 清空 CDN 缓存 (Purge Everything)..."
RESP=$(curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/purge_cache" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"purge_everything": true}' --max-time 60)
echo "$RESP" | python -c "import json,sys; d=json.load(sys.stdin); print('✅ 缓存已清' if d.get('success') else '❌ 失败: '+json.dumps(d.get('errors',[]),ensure_ascii=False))"

echo "🧪 验证下载链接（应为 200 + 新大小）:"
curl -s -o /dev/null -w "  zip: %{http_code} %{size_download}B\n" \
  "https://download.shanca.me/Rescene-windows-amd64-portable.zip" --max-time 120 || true
