package main

// tools_mcp.go — MCP 客户端（从 re0 main-backend/internal/handler/mcp_client.go 移植，轻量版）
// 支持通过 mcp.json 配置外部 MCP server（浏览器自动化、数据库等）。
// 当前版本：读取配置 + 工具定义列表；stdio JSON-RPC 调用后续补齐。

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// mcpServerConfig MCP server 配置
type mcpServerConfig struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Args    []string `json:"args"`
	Enabled bool   `json:"enabled"`
}

// loadMCPToolDefs 从 mcp.json 读取外部工具定义（当前返回空，MCP stdio 调用待接）
func loadMCPToolDefs() []ToolDefinition {
	// 读取 ~/rescene_data/mcp.json（存在才加载）
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(home, "rescene_data", "mcp.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfgs []mcpServerConfig
	if json.Unmarshal(data, &cfgs) != nil {
		return nil
	}
	// TODO: 连接 stdio server 并 tools/list。当前先返回空，避免误报工具可用。
	return nil
}
