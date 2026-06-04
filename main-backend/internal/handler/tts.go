package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type TTSRequest struct {
	Model string `json:"model"`
	Input struct {
		Text         string `json:"text"`
		Voice        string `json:"voice"`
		LanguageType string `json:"language_type"`
	} `json:"input"`
}

type TTSResponse struct {
	Output struct {
		Audio struct {
			URL string `json:"url"`
		} `json:"audio"`
	} `json:"output"`
}

func SynthesizeSpeech(text string) ([]byte, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("缺少 DASHSCOPE_API_KEY")
	}

	// 新版请求体格式
	reqBody := TTSRequest{
		Model: "qwen3-tts-flash",
	}
	reqBody.Input.Text = text
	reqBody.Input.Voice = "Maia"
	reqBody.Input.LanguageType = "Chinese"

	reqBytes, _ := json.Marshal(reqBody)

	// 新版多模态生成端点
	req, _ := http.NewRequest("POST",
		"https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
		bytes.NewBuffer(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := new(http.Client).Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求TTS API失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errMsg bytes.Buffer
		errMsg.ReadFrom(resp.Body)
		return nil, fmt.Errorf("API返回非200: %d, body: %s", resp.StatusCode, errMsg.String())
	}

	// 解析响应，获取音频URL
	var ttsResp TTSResponse
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &ttsResp); err != nil {
		return nil, fmt.Errorf("解析TTS响应失败: %w", err)
	}

	if ttsResp.Output.Audio.URL == "" {
		return nil, fmt.Errorf("未获取到音频URL")
	}

	// 下载音频文件
	audioResp, err := http.Get(ttsResp.Output.Audio.URL)
	if err != nil {
		return nil, fmt.Errorf("下载音频失败: %w", err)
	}
	defer audioResp.Body.Close()

	var audioBuf bytes.Buffer
	audioBuf.ReadFrom(audioResp.Body)
	return audioBuf.Bytes(), nil
}
