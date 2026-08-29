package handler

// capability_config.go — 联网搜索（web_search）与生图（image_generate）的「来源」配置。
//
// 与 firecrawl/agnes 同一机制：存放在 user_configs 的特殊条目里，
// 由设置面板「模型」tab 的能力区读写：
//
//	id=websearch: 联网来源。mode = firecrawl（默认）/ custom / mcp
//	  - firecrawl: 走 Firecrawl /v1/search（免费额度 500 次/月）
//	  - custom:    走自定义 OpenAI 兼容 Endpoint 的 /v1/responses（内置 web_search 工具）
//	  - mcp:       走用户指定的已装 MCP 工具（mcp__server__tool）
//	  Extra: {mode, mcp_tool}; Endpoint/APIKey/DefaultModel 存标准字段
//
//	id=image: 生图来源。mode = pollinations（默认）/ custom / mcp
//	  - pollinations: 免费无 key 直连
//	  - custom:       走自定义 OpenAI 兼容 Endpoint 的 /v1/images/generations
//	  - mcp:          走用户指定的已装 MCP 工具
//	  Extra: {mode, mcp_tool}; Endpoint/APIKey/DefaultModel 存标准字段

import "strings"

const (
	websearchCapabilityID = "websearch"
	imageCapabilityID     = "image"
)

// isCapabilityConfigID 是否是能力条目（联网/生图），它们在 GET 里即使
// Endpoint 为空也不该被当「目录外残留」丢弃。
func isCapabilityConfigID(id string) bool {
	return id == websearchCapabilityID || id == imageCapabilityID
}

// capabilityMode 读取能力条目的 Extra.mode，缺失/未知时回退默认值。
func capabilityMode(entry ModelConfigEntry, fallback string) string {
	m := strings.ToLower(strings.TrimSpace(entry.Extra["mode"]))
	if m == "" {
		return fallback
	}
	return m
}

// capabilityEntry 读用户配置里的能力条目（固定 default 用户，与 firecrawlAPIKey 一致）。
func capabilityEntry(id string) (ModelConfigEntry, bool) {
	entries, err := loadModelConfigs("")
	if err != nil {
		return ModelConfigEntry{}, false
	}
	for _, e := range entries {
		if e.ID == id {
			return e, true
		}
	}
	return ModelConfigEntry{}, false
}
