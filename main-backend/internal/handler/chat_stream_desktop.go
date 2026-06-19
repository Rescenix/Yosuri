package handler

import "github.com/gin-gonic/gin"

func (h *ChatHandler) StreamChatDesktop(c *gin.Context) {
	// 电脑端专用的聊天处理
	// 工具集：DesktopChatTools（包含 search_codebase、codegraph_query）
	// 记忆：阿里云向量检索
	// 网络：标准 Transport
}
