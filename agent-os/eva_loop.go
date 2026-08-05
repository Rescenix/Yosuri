package main

// eva_loop.go — Eva 世界循环：编排角色、推动事件、管理世界

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// AddCharacter 添加角色到世界
func (e *Eva) AddCharacter(id, name, role, prompt string) {
	e.Characters = append(e.Characters, EvaCharacter{
		ID: id, Name: name, Role: role, Status: "active",
		LastSeen: time.Now().Format("15:04"), Prompt: prompt,
	})
	e.save()
}

// LogEvent 记录世界事件
func (e *Eva) LogEvent(etype, content string) {
	e.Events = append(e.Events, EvaEvent{
		Time: time.Now().Format("15:04"), Type: etype, Content: content,
	})
	if len(e.Events) > 50 {
		e.Events = e.Events[len(e.Events)-50:]
	}
	e.save()
}

// RunEvaLoop 世界主循环（Eva 驱动一切）
func (e *Eva) RunEvaLoop(d *Daughter) {
	liveLog := filepath.Join(d.Home, "live.log")
	logLive(liveLog, fmt.Sprintf("🎬 Eva 世界系统启动 · 第 %d 天 · 种子 %d", e.Day, e.Seed))

	round := 0
	for {
		round++
		time.Sleep(5 * time.Second)

		// Eva 决策：当前世界状态 → 决定做什么
		action := e.think(round)
		logLive(liveLog, fmt.Sprintf("[%s] 🧠 Eva 第 %d 轮 · %s", time.Now().Format("15:04"), round, action))

		// 执行 Eva 的决策
		e.execute(action, d, liveLog)
	}
}

// think Eva 思考世界状态，决定下一步行动
func (e *Eva) think(round int) string {
	// 根据轮次和世界状态决定行动
	switch round % 5 {
	case 0:
		return "推动世界时间"
	case 1:
		return "检查角色状态"
	case 2:
		return "生成随机事件"
	case 3:
		return "更新角色行为"
	default:
		return "维持世界稳态"
	}
}

// execute 执行 Eva 的决策
func (e *Eva) execute(action string, d *Daughter, liveLog string) {
	switch {
	case strings.Contains(action, "世界时间"):
		e.Time = time.Now().Format("2006-01-02 15:04")
		e.save()
		logLive(liveLog, fmt.Sprintf("⏰ Eva 更新时间 → %s", e.Time))

	case strings.Contains(action, "角色状态"):
		for i := range e.Characters {
			e.Characters[i].LastSeen = time.Now().Format("15:04")
		}
		e.save()
		logLive(liveLog, fmt.Sprintf("👥 Eva 检查了 %d 个角色", len(e.Characters)))

	case strings.Contains(action, "随机事件"):
		events := []string{"天气变化", "新消息到来", "角色相遇", "发现新事物"}
		evt := events[time.Now().Second()%len(events)]
		e.LogEvent("world", evt)
		logLive(liveLog, fmt.Sprintf("🌍 Eva 触发事件: %s", evt))

	case strings.Contains(action, "角色行为"):
		logLive(liveLog, "🔄 Eva 更新角色行为模式")

	default:
		logLive(liveLog, "🌐 Eva 维持世界稳态")
	}
}

// CharacterStatus 返回角色状态报告
func (e *Eva) CharacterStatus() string {
	var sb strings.Builder
	sb.WriteString("📋 世界角色状态：\n")
	for _, c := range e.Characters {
		sb.WriteString(fmt.Sprintf("  %s (%s) - %s [%s]\n", c.Name, c.Role, c.Status, c.LastSeen))
	}
	return sb.String()
}

// WorldReport 世界报告
func (e *Eva) WorldReport() string {
	data, _ := json.MarshalIndent(e, "", "  ")
	return string(data)
}