// backend/internal/handler/image_generate.go
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

type ImageGenerateRequest struct {
	Prompt string `json:"prompt"`
}

type ImageGenerateResponse struct {
	URL string `json:"url"`
}

// GenerateImage 调用阿里云千问图像模型（异步模式），转存图片并返回本地可访问URL
func GenerateImage(c *gin.Context) {
	var req ImageGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	// 创建异步任务
	taskID, err := createImageTask(req.Prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建图片任务失败: %v", err)})
		return
	}

	// 轮询任务，直到完成（最多等 60 秒）
	imageURL, err := waitForTaskResult(taskID, 60*time.Second)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("等待图片生成失败: %v", err)})
		return
	}

	// 转存图片到本地，返回本地URL
	localURL, err := downloadAndSaveImage(imageURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存图片失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, ImageGenerateResponse{URL: localURL})
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
