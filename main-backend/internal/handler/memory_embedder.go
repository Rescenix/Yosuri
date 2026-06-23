package handler

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
)

type EmbeddingRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	EncodingFormat string `json:"encoding_format,omitempty"`
	Dimensions     int    `json:"dimensions,omitempty"`
}

type EmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func getEmbedding(text string) ([]float64, error) {
	resp, err := http.Post("http://localhost:6752/", "text/plain", strings.NewReader(text))
	if err != nil {
		return nil, fmt.Errorf("本地BGE请求失败: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	vec := make([]float64, len(data)/4)
	for i := range vec {
		bits := binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
		vec[i] = float64(math.Float32frombits(bits))
	}
	return vec, nil
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
