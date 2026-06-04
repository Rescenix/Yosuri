package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"time"
)

type EmbeddingRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"` // 兼容模式直接传字符串
	EncodingFormat string `json:"encoding_format,omitempty"`
	Dimensions     int    `json:"dimensions,omitempty"` // 维度参数
}

// 兼容模式响应结构
type EmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func getEmbedding(text string) ([]float64, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("缺少 DASHSCOPE_API_KEY")
	}

	if text == "" {
		return nil, fmt.Errorf("输入文本为空")
	}

	// 构造请求体，Input 直接赋值字符串
	reqBody := EmbeddingRequest{
		Model:      "text-embedding-v4",
		Input:      text,
		Dimensions: 128,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化失败: %v", err)
	}
	fmt.Printf("📤 Embedding请求: %s\n", string(reqBytes))

	req, err := http.NewRequest("POST", "https://dashscope.aliyuncs.com/compatible-mode/v1/embeddings", bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API返回非200: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var embResp EmbeddingResponse
	if err := json.Unmarshal(bodyBytes, &embResp); err != nil {
		return nil, fmt.Errorf("解析失败: %v", err)
	}
	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("空向量")
	}
	return embResp.Data[0].Embedding, nil
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func (m *MemoryStore) SearchSimilar(query string, topK int) []MemoryRecord {
	if len(m.records) == 0 {
		return nil
	}

	var candidates []MemoryRecord
	for _, rec := range m.records {
		if len(rec.Embedding) > 0 {
			candidates = append(candidates, rec)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	queryEmb, err := getEmbedding(query)
	if err != nil {
		fmt.Printf("⚠️ 生成查询向量失败: %v\n", err)
		return nil
	}

	type scored struct {
		rec   MemoryRecord
		score float64
	}
	var scores []scored
	for _, rec := range candidates {
		scores = append(scores, scored{rec, cosineSimilarity(queryEmb, rec.Embedding)})
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].score > scores[j].score })

	var results []MemoryRecord
	for i := 0; i < topK && i < len(scores); i++ {
		results = append(results, scores[i].rec)
	}
	return results
}
