package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type VisionResponse struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
}

// VisionQA 是一轮历史问答，供多轮追问时把上文带回去（见 AnalyzeImage）。
type VisionQA struct {
	Q string `json:"q"`
	A string `json:"a"`
}

// AnalyzeImage 调用阿里云 qwen-vl-max 分析图片。
// history 非空时把之前的问答对铺在图片这一轮前面，支持"先问整体、再问细节"的连续追问——
// DashScope 的 multimodal-generation 走标准多轮 messages 数组，image 只挂在最后一条
// user 消息上即可，之前的图不需要重复携带。
func AnalyzeImage(imageBase64 string, question string, history []VisionQA) (string, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")

	// 智能清理可能存在的 base64 前缀
	cleanBase64 := imageBase64
	if idx := strings.Index(cleanBase64, "base64,"); idx != -1 {
		cleanBase64 = cleanBase64[idx+7:] // 截取 "base64," 之后的部分
	}

	var messages []map[string]interface{}
	for _, h := range history {
		if h.Q == "" && h.A == "" {
			continue
		}
		messages = append(messages,
			map[string]interface{}{"role": "user", "content": []map[string]interface{}{{"text": h.Q}}},
			map[string]interface{}{"role": "assistant", "content": []map[string]interface{}{{"text": h.A}}},
		)
	}
	messages = append(messages, map[string]interface{}{
		"role": "user",
		"content": []map[string]interface{}{
			{"image": "data:image/jpeg;base64," + cleanBase64},
			{"text": question},
		},
	})

	reqBody := map[string]interface{}{
		"model": "qwen-vl-max",
		"input": map[string]interface{}{
			"messages": messages,
		},
	}

	reqBytes, _ := json.Marshal(reqBody)
	fmt.Printf("📸 发送视觉请求，大小: %.2f KB\n", float64(len(reqBytes))/1024)

	req, _ := http.NewRequest("POST",
		"https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
		bytes.NewBuffer(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: AliyunTransport, // 走神权代理（手机端）或默认传输（电脑端）
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fmt.Printf("📸 视觉API响应状态: %d\n", resp.StatusCode)

	if resp.StatusCode != 200 {
		var errMsg bytes.Buffer
		errMsg.ReadFrom(resp.Body)
		return "", fmt.Errorf("视觉API返回非200: %d, body: %s", resp.StatusCode, errMsg.String())
	}

	var visionResp VisionResponse
	json.NewDecoder(resp.Body).Decode(&visionResp)

	if len(visionResp.Output.Choices) == 0 {
		return "图片分析未返回结果", nil
	}

	// 提取文本描述
	var description string
	for _, content := range visionResp.Output.Choices[0].Message.Content {
		if content.Text != "" {
			description += content.Text
		}
	}
	if description == "" {
		return "图片分析未返回结果", nil
	}
	return description, nil
}

// VisionAnalyzeRequest 是 HandleVisionAnalyze 的请求体。
// image_url / image_base64 二选一；history 用于多轮追问（见 AnalyzeImage 注释）。
type VisionAnalyzeRequest struct {
	ImageURL    string     `json:"image_url"`
	ImageBase64 string     `json:"image_base64"`
	Question    string     `json:"question"`
	History     []VisionQA `json:"history"`
}

// HandleVisionAnalyze POST /api/vision/analyze —— 看图分析的唯一入口。
// Key 和视觉模型调用都留在这一处：view_image MCP server（main-backend/mcp/
// view_image_server.py）只是把 stdio JSON-RPC 转成对这个 HTTP 端点的调用，
// 不在 Python 侧重复读一遍 DASHSCOPE_API_KEY，也不用两边分别适配 DashScope 的请求格式。
func HandleVisionAnalyze(c *gin.Context) {
	var req VisionAnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}

	imgB64 := req.ImageBase64
	if imgB64 == "" && req.ImageURL != "" {
		// 不少图床（含 Wikimedia）拿 Go 默认 UA 直接 403；没有 UA 装成浏览器，
		// 拿到的错误页正文会被当成"图片"一路 base64 送进视觉模型，对方只会报
		// "image format is illegal"，看不出真实原因是下载环节被拦了。
		imgReq, err := http.NewRequest("GET", req.ImageURL, nil)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "图片 URL 无效: " + err.Error()})
			return
		}
		imgReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		resp, err := http.DefaultClient.Do(imgReq)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "下载图片失败: " + err.Error()})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("下载图片返回非200: %d, body: %s", resp.StatusCode, string(body))})
			return
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "读取图片失败: " + err.Error()})
			return
		}
		imgB64 = base64.StdEncoding.EncodeToString(data)
	}
	if imgB64 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_url 和 image_base64 至少提供一个"})
		return
	}

	question := req.Question
	if question == "" {
		question = "请详细描述这张图片的内容"
	}

	text, err := AnalyzeImage(imgB64, question, req.History)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "视觉分析失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"text": text})
}
