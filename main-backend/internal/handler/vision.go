package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
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

// AnalyzeImage 调用阿里云 qwen-vl-max 分析图片
func AnalyzeImage(imageBase64 string, question string) (string, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")

	// 智能清理可能存在的 base64 前缀
	cleanBase64 := imageBase64
	if idx := strings.Index(cleanBase64, "base64,"); idx != -1 {
		cleanBase64 = cleanBase64[idx+7:] // 截取 "base64," 之后的部分
	}

	reqBody := map[string]interface{}{
		"model": "qwen-vl-max",
		"input": map[string]interface{}{
			"messages": []map[string]interface{}{
				{
					"role": "user",
					"content": []map[string]interface{}{
						{"image": "data:image/jpeg;base64," + cleanBase64},
						{"text": question},
					},
				},
			},
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
