package handler

// company_finance.go — 公司资金闭环（开罗经济：真实评分→销量→入账→买推广/设施）
//
// 公式：产品收入 = 平均分 × 评分人数 × 单价（¥500/人分）
// 增量同步：每次真实用户提交评分后自动补差额入账（见 HandleCompanyReviewSubmit）
// 资金用途：买推广位（游戏大厅曝光）/ 买办公室设施（部门升级）

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// financeUnitPrice 每"人分"单价：1 人打 1 分 = ¥500（开罗式定价，人数少单价高才有手感）
const financeUnitPrice = 500

// financeEntry 一条资金流水
type financeEntry struct {
	Type    string `json:"type"`    // earn_launch(发售收入) / spend_promo(推广) / spend_facility(设施)
	Amount  int64  `json:"amount"`  // 正=入账 负=支出
	Project string `json:"project,omitempty"`
	Desc    string `json:"desc"`
	At      string `json:"at"`
}

// financeLedger 公司资金账本
type financeLedger struct {
	Balance  int64            `json:"balance"`  // 当前资金
	TotalIn  int64            `json:"total_in"` // 累计收入
	Products map[string]int64 `json:"products"` // 各产品累计收入（增量计算依据）
	Entries  []financeEntry   `json:"entries"`  // 流水（新→旧）
}

func financePath() string { return filepath.Join(companyDir(), "finance.json") }

func loadFinance() financeLedger {
	lg := financeLedger{Products: map[string]int64{}}
	data, err := os.ReadFile(financePath())
	if err != nil {
		return lg
	}
	json.Unmarshal(data, &lg)
	if lg.Products == nil {
		lg.Products = map[string]int64{}
	}
	return lg
}

func saveFinance(lg financeLedger) error {
	data, _ := json.MarshalIndent(lg, "", "  ")
	return os.WriteFile(financePath(), data, 0644)
}

// productRevenue 计算产品累计收入（真实评分驱动）
func productRevenue(rb realReviewsFile) int64 {
	if len(rb.Reviews) == 0 {
		return 0
	}
	avg := 0.0
	for _, r := range rb.Reviews {
		avg += float64(r.Score)
	}
	avg /= float64(len(rb.Reviews))
	return int64(math.Round(avg*float64(len(rb.Reviews)) * financeUnitPrice))
}

// syncProductRevenue 增量入账：产品收入上涨的部分进公司账户
func syncProductRevenue(projDir, project string) error {
	rb := loadRealReviews(projDir, project)
	rev := productRevenue(rb)
	if rev <= 0 {
		return nil
	}
	lg := loadFinance()
	old := lg.Products[project]
	if rev <= old {
		return nil
	}
	gain := rev - old
	lg.Balance += gain
	lg.TotalIn += gain
	lg.Products[project] = rev
	lg.Entries = append([]financeEntry{{
		Type:    "earn_launch",
		Amount:  gain,
		Project: project,
		Desc:    fmt.Sprintf("《%s》真实销量入账（%d 位用户 · 平均 %.1f 分）", project, len(rb.Reviews), float64(productRevenue(rb))/float64(len(rb.Reviews))/financeUnitPrice),
		At:      time.Now().Format("2006-01-02 15:04"),
	}}, lg.Entries...)
	return saveFinance(lg)
}

// HandleCompanyFinance GET /api/company/finance — 资金账本
func HandleCompanyFinance(c *gin.Context) {
	lg := loadFinance()
	c.JSON(http.StatusOK, gin.H{
		"balance":  lg.Balance,
		"total_in": lg.TotalIn,
		"products": lg.Products,
		"entries":  lg.Entries,
	})
}

// financeSpendReq POST /api/company/finance/spend 请求体
type financeSpendReq struct {
	Type    string `json:"type"`    // promo(推广位) / facility(设施)
	Amount  int64  `json:"amount"`  // 花费（正数）
	Target  string `json:"target"`  // promo→项目名；facility→部门 key
	Desc    string `json:"desc"`
}

// HandleCompanyFinanceSpend POST /api/company/finance/spend — 花钱（推广/设施）
func HandleCompanyFinanceSpend(c *gin.Context) {
	var req financeSpendReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Type != "promo" && req.Type != "facility" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type 只能是 promo 或 facility"})
		return
	}
	lg := loadFinance()
	if lg.Balance < req.Amount {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "公司资金不足"})
		return
	}
	lg.Balance -= req.Amount
	label := req.Type
	desc := req.Desc
	switch req.Type {
	case "promo":
		label = "推广位"
		if desc == "" {
			desc = fmt.Sprintf("《%s》游戏大厅推广位", req.Target)
		}
	case "facility":
		label = "设施"
		if desc == "" {
			desc = fmt.Sprintf("升级 %s 部门设施", req.Target)
		}
	}
	lg.Entries = append([]financeEntry{{
		Type:    "spend_" + req.Type,
		Amount:  -req.Amount,
		Project: req.Target,
		Desc:    desc,
		At:      time.Now().Format("2006-01-02 15:04"),
	}}, lg.Entries...)
	if err := saveFinance(lg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "落盘失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"balance": lg.Balance, "entry": lg.Entries[0], "type_label": label})
}
