// backend/internal/handler/search.go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
)

type SearchRequest struct {
	Model string `json:"model"`
	Input struct {
		Messages []SearchMessage `json:"messages"`
	} `json:"input"`
	Parameters struct {
		EnableSearch bool   `json:"enable_search"`
		ResultFormat string `json:"result_format"`
	} `json:"parameters"`
}

type SearchMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SearchResponse struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
}

// WebSearch 调用阿里云 qwen-plus 内置联网搜索
func WebSearch(query string) (string, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")

	reqBody := SearchRequest{
		Model: "qwen-plus",
	}
	reqBody.Input.Messages = []SearchMessage{
		{Role: "user", Content: query},
	}
	reqBody.Parameters.EnableSearch = true
	reqBody.Parameters.ResultFormat = "message"

	reqBytes, _ := json.Marshal(reqBody)

	// 正确的API端点
	req, _ := http.NewRequest("POST",
		"https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation",
		bytes.NewBuffer(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := new(http.Client).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var searchResp SearchResponse
	json.NewDecoder(resp.Body).Decode(&searchResp)

	if len(searchResp.Output.Choices) == 0 {
		return "", nil
	}
	return searchResp.Output.Choices[0].Message.Content, nil
}
