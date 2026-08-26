package handler

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCallNativeVideoGenerate(t *testing.T) {
	// 实测默认路径：agnes-video-2.5-flash（$0/秒）+ anime_live 风格模板
	args := map[string]interface{}{
		"prompt": "a Japanese anime schoolgirl with silver hair and violet eyes, standing under cherry blossom trees, Tokyo street, gentle smile",
		"style":  "anime_live",
		"seed":   42,
	}
	raw, _ := json.Marshal(args)
	res, err := callNativeVideoGenerate(context.Background(), string(raw))
	if err != nil {
		t.Fatalf("生视频失败: %v", err)
	}
	t.Logf("结果: %s", res.Text)
}
