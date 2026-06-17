// handler/balance.go
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

type BalanceResponse struct {
	IsAvailable  bool `json:"is_available"`
	BalanceInfos []struct {
		Currency        string `json:"currency"`
		TotalBalance    string `json:"total_balance"`
		GrantedBalance  string `json:"granted_balance"`
		ToppedUpBalance string `json:"topped_up_balance"`
	} `json:"balance_infos"`
}

func GetBalance(c *gin.Context) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "缺少API Key"})
		return
	}

	req, _ := http.NewRequest("GET", "https://api.deepseek.com/user/balance", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: DeepSeekTransport, // 使用平台特化的传输层
	}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("请求失败: %v", err)})
		return
	}
	defer resp.Body.Close()

	var balance BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&balance); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析余额数据失败"})
		return
	}

	c.JSON(http.StatusOK, balance)
}
