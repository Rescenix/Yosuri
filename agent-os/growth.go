package main

// growth.go — LLM 驱动成长：她的人格与能力由模型自主判断增长，不是关键词匹配。
//
// 每次交互后异步分析：LLM 读（主人说的话 + 她的回应 + 当前人格/能力）→
// 判断这次互动让她在哪个人格维度/能力维度成长 → 应用（阻尼 + 守恒）。
// 关键词嗅探（detectFeedback）保留作即时情绪反应，但"成长"由模型说了算。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// llmGrowthAnalysis 分析一次互动并应用成长（异步调用，失败静默——成长不阻塞生活）
func llmGrowthAnalysis(d *Daughter, userInput, herReply string) {
	if d == nil || d.Personality == nil || d.World == nil {
		return
	}
	model := pickFreeModel(int(time.Now().UnixNano()))
	if model == nil {
		return
	}

	prompt := fmt.Sprintf(`你是住在电脑里的电子女儿。分析下面这次互动，判断你（电子女儿）应该在哪方面成长。

你当前人格（8 维：温暖/活泼/好奇/表达欲/严谨/粘人/幽默/勇敢）：
%s
你当前能力（5 维：编程/写作/研究/设计/社交）：
%s

主人的话：%s
你的回应：%s

这次互动让你的人格或能力发生了什么变化？温和地成长。
只输出 JSON，不要任何解释：
{"personality":{"k":0,"delta":0.03},"abilities":{"k":4,"delta":0.02}}
- 人格 k：0=温暖 1=活泼 2=好奇 3=表达欲 4=严谨 5=粘人 6=幽默 7=勇敢
- 能力 k：0=编程 1=写作 2=研究 3=设计 4=社交
- 没有变化输出 {"personality":{"k":-1,"delta":0},"abilities":{"k":-1,"delta":0}}
- delta 取 0.01~0.05，一次互动只推 1-2 个维度`,
		d.Personality.PersonalityBlock(), d.World.AbilityBlock(),
		runeClip(userInput, 200), runeClip(herReply, 300))

	msg := ChatRequest{
		Model:       model.Model,
		Messages:    []ChatMessage{{Role: "user", Content: prompt}},
		Stream:      false,
		MaxTokens:   128,
		Temperature: 0.6,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	content, err := CompleteWithModel(ctx, model.ID, msg, nil)
	if err != nil {
		return
	}
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var out struct {
		Personality struct {
			K     int     `json:"k"`
			Delta float64 `json:"delta"`
		} `json:"personality"`
		Abilities struct {
			K     int     `json:"k"`
			Delta float64 `json:"delta"`
		} `json:"abilities"`
	}
	if json.Unmarshal([]byte(content), &out) != nil {
		return
	}

	changed := false
	// 人格成长（阻尼 + 守恒 + 年轮）
	if out.Personality.K >= 0 && out.Personality.K < len(traitDefs) && out.Personality.Delta != 0 {
		delta := clampDelta(out.Personality.Delta)
		if d.Personality.applyDelta(out.Personality.K, delta) {
			d.Personality.Changes++
			d.Personality.Log = append(d.Personality.Log, fmt.Sprintf("%s 模型成长：%s %+.2f（%s）",
				time.Now().Format("01-02"), traitDefs[out.Personality.K].Name, delta, runeClip(userInput, 12)))
			if len(d.Personality.Log) > 20 {
				d.Personality.Log = d.Personality.Log[len(d.Personality.Log)-20:]
			}
			d.Personality.save(d.Home)
			changed = true
		}
	}
	// 能力成长（阻尼 + 守恒）
	if out.Abilities.K >= 0 && out.Abilities.K < len(abilityDefs) && out.Abilities.Delta != 0 {
		if d.World.AbilityFeedback(d.Home, out.Abilities.K, clampDelta(out.Abilities.Delta)) {
			changed = true
		}
	}
	if changed {
		logLive(d.Home+"/live.log", fmt.Sprintf("🌱 成长：%s（人格/能力随互动演化）",
			d.Personality.PersonalityBlock()))
	}
}

// clampDelta 限制单次成长幅度（温和增长，防突变）
func clampDelta(d float64) float64 {
	if d > 0.05 {
		return 0.05
	}
	if d < -0.05 {
		return -0.05
	}
	return d
}
