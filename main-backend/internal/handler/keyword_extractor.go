package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func extractKeywordsWithDS(text string) []string {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return nil
	}

	prompt := fmt.Sprintf(`请为以下对话内容提取3-5个关键词，用逗号分隔。
只输出关键词，不要任何解释。

对话内容：
%s`, text)

	reqBody := DSReq{
		Model: os.Getenv("DEEPSEEK_MODEL"),
		Messages: []DSMessage{
			{Role: "user", Content: prompt},
		},
	}
	reqBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "https://api.deepseek.com/v1/chat/completions", bytes.NewBuffer(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := new(http.Client).Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var dsResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.NewDecoder(resp.Body).Decode(&dsResp)

	if len(dsResp.Choices) == 0 {
		return nil
	}

	// 确保正确处理逗号分隔的返回结果
	raw := strings.TrimSpace(dsResp.Choices[0].Message.Content)
	parts := strings.Split(raw, ",")
	var keywords []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// 过滤掉空字符串和单字符
		if len([]rune(p)) >= 2 {
			keywords = append(keywords, p)
		}
	}
	return keywords
}
