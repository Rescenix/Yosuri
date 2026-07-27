// backend/internal/handler/image_generate.go
package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)
type ImageGenerateRequest struct {
	Prompt   string `json:"prompt"`
	Provider string `json:"provider,omitempty"` // 可选：agnes / dashscope；不填则按 key 可用性自动选
}

type ImageGenerateResponse struct {
	URL      string `json:"url"`
	Provider string `json:"provider,omitempty"`
}

// GenerateImage 调用图像模型生成图片，转存到本地公开目录并返回可访问 URL。
// 提供商选择：显式指定 provider 优先；否则 Agnes_API_KEY 存在则走 Agnes（免费多模态网关），
// 否则回退阿里云 DASHSCOPE（qwen-image-plus）。两者都缺则报错提示。
func GenerateImage(c *gin.Context) {
	var req ImageGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt 不能为空"})
		return
	}

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		if os.Getenv("Agnes_API_KEY") != "" {
			provider = "agnes"
		} else if os.Getenv("DASHSCOPE_API_KEY") != "" {
			provider = "dashscope"
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "缺少 API Key：请在环境变量或 .env 配置 Agnes_API_KEY 或 DASHSCOPE_API_KEY"})
			return
		}
	}

	var (
		imageURL  string
		usedProv  string
		err       error
	)
	switch provider {
	case "agnes":
		imageURL, err = generateImageViaAgnes(req.Prompt)
		usedProv = "agnes"
	case "dashscope":
		imageURL, err = createImageTask(req.Prompt) // 现有 DASHSCOPE 异步通道
		usedProv = "dashscope"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "未知 provider: " + provider})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建图片任务失败: %v", err)})
		return
	}

	localURL, err := downloadAndSaveImage(imageURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存图片失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, ImageGenerateResponse{URL: localURL, Provider: usedProv})
}

// generateImageViaAgnes 调用 Agnes AI 免费图像网关（agnes-image-2.1-flash）。
// 同步返回图片 URL（文档标明当前 $0/image，单次生成一般数秒~数十秒）。
func generateImageViaAgnes(prompt string) (string, error) {
	apiKey := os.Getenv("Agnes_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("缺少 Agnes_API_KEY")
	}
	reqBody := map[string]interface{}{
		"model":  "agnes-image-2.1-flash",
		"prompt": prompt,
		"size":   "2K",
		"ratio":  "16:9",
	}
	reqBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", "https://apihub.agnes-ai.com/v1/images/generations", bytes.NewBuffer(reqBytes))
	if err != nil {
		return "", fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Agnes 返回非200: %d, body: %s", resp.StatusCode, string(body))
	}

	// 响应可能含 data[].url 或 data[].b64_json（取其一）
	var parsed struct {
		Data []struct {
			URL    string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("解析 Agnes 响应失败: %w", err)
	}
	if len(parsed.Data) == 0 {
		return "", fmt.Errorf("Agnes 未返回图片数据: %s", string(body))
	}
	if parsed.Data[0].URL != "" {
		return parsed.Data[0].URL, nil
	}
	if parsed.Data[0].B64JSON != "" {
		// base64 直接落盘
		dec, decErr := decodeB64ToTemp(parsed.Data[0].B64JSON)
		if decErr != nil {
			return "", decErr
		}
		return dec, nil
	}
	return "", fmt.Errorf("Agnes 返回数据既无 url 也无 b64_json")
}

// decodeB64ToTemp 把 base64 图片写入 public/images 并返回本地相对 URL。
func decodeB64ToTemp(b64 string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("base64 解码失败: %w", err)
	}
	saveDir := filepath.Join(".", "public", "images")
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return "", fmt.Errorf("创建保存目录失败: %w", err)
	}
	fileName := fmt.Sprintf("generated_%d.png", time.Now().UnixNano())
	savePath := filepath.Join(saveDir, fileName)
	if err := os.WriteFile(savePath, data, 0644); err != nil {
		return "", fmt.Errorf("保存图片失败: %w", err)
	}
	return "/images/" + fileName, nil
}

// createImageTask 创建图片生成异步任务，返回 task_id
func createImageTask(prompt string) (string, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("缺少 DASHSCOPE_API_KEY")
	}

	reqBody := map[string]interface{}{
		"model": "qwen-image-plus",
		"input": map[string]interface{}{
			"prompt": prompt,
		},
		"parameters": map[string]interface{}{
			"size":          "1664*928",
			"n":             1,
			"prompt_extend": true,
			"watermark":     false,
		},
	}
	reqBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis", bytes.NewBuffer(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-DashScope-Async", "enable") // 关键：启用异步模式

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("创建任务返回非200: %d, body: %s", resp.StatusCode, string(body))
	}

	var createResp struct {
		Output struct {
			TaskID string `json:"task_id"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &createResp); err != nil {
		return "", fmt.Errorf("解析创建任务响应失败: %w", err)
	}

	if createResp.Output.TaskID == "" {
		return "", fmt.Errorf("未获取到 task_id")
	}

	return createResp.Output.TaskID, nil
}

// waitForTaskResult 轮询任务结果，直到成功或超时
func waitForTaskResult(taskID string, timeout time.Duration) (string, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("任务超时")
		}

		req, _ := http.NewRequest("GET", fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/tasks/%s", taskID), nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := new(http.Client).Do(req)
		if err != nil {
			return "", fmt.Errorf("查询任务失败: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var taskResp struct {
			Output struct {
				TaskStatus string `json:"task_status"`
				Results    []struct {
					URL string `json:"url"`
				} `json:"results"`
			} `json:"output"`
		}
		if err := json.Unmarshal(body, &taskResp); err != nil {
			return "", fmt.Errorf("解析任务结果失败: %w", err)
		}

		switch taskResp.Output.TaskStatus {
		case "SUCCEEDED":
			if len(taskResp.Output.Results) > 0 && taskResp.Output.Results[0].URL != "" {
				return taskResp.Output.Results[0].URL, nil
			}
			return "", fmt.Errorf("任务成功但未返回图片URL")
		case "FAILED", "CANCELED", "UNKNOWN":
			return "", fmt.Errorf("任务状态异常: %s", taskResp.Output.TaskStatus)
		default:
			// PENDING 或 RUNNING，等待后重试
			time.Sleep(3 * time.Second)
		}
	}
}

// downloadAndSaveImage 下载图片并保存到本地公开目录，返回本地可访问的URL
func downloadAndSaveImage(imageURL string) (string, error) {
	// 1. 下载图片
	resp, err := http.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("下载图片返回非200: %d", resp.StatusCode)
	}

	// 2. 确定保存路径（通过环境变量控制）
	saveDir := filepath.Join(".", "public", "images")
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return "", fmt.Errorf("创建保存目录失败: %w", err)
	}

	fileName := fmt.Sprintf("generated_%d.png", time.Now().UnixNano())
	savePath := filepath.Join(saveDir, fileName)

	// 确保目录存在
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return "", fmt.Errorf("创建保存目录失败: %w", err)
	}

	// 生成唯一文件名

	// 创建文件
	file, err := os.Create(savePath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	// 将下载的图片内容拷贝到新文件
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("保存图片失败: %w", err)
	}

	// 3. 返回可公开访问的URL（通过环境变量控制）
	baseURL := os.Getenv("IMAGE_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:4321" // 默认本地开发环境
	}

	fmt.Printf("[DEBUG] 图片保存目录: %s\n", saveDir)
	fmt.Printf("[DEBUG] 图片完整路径: %s\n", savePath)
	return "/images/" + fileName, nil
}
