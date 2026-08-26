package handler

// company_market.go — 应用大厅（B站式信息流）：公司产出的所有应用/游戏 + 真实评分 + 推广位
//
// 推荐分 = 真实评分(均分×人数) + 推广加权（花钱买的曝光靠前）
// 推广记录从 finance.json 的 spend_promo 流水提取（真实花钱 → 真实曝光）

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// marketGame 大厅里的一张应用卡
type marketGame struct {
	Project   string      `json:"project"`
	Agent     string      `json:"agent"`   // 负责的 coder
	AvgScore  float64     `json:"avg_score"`
	Count     int         `json:"count"`   // 真实评分人数
	Award     *reviewAward `json:"award"`  // 🏆神作/⭐好评/🌱新秀
	Promoted  bool        `json:"promoted"` // 花钱买的推广位
	PromoDesc string      `json:"promo_desc,omitempty"`
	Score     float64     `json:"score"`   // 推荐分（排序用）
	HasReview bool        `json:"has_review"`
}

// HandleCompanyMarket GET /api/company/market — 游戏大厅信息流
// 返回所有 coder 项目；有真实评分的展示评分/奖项，无评分的标记"待评测"。
// 排序：推广位优先 → 推荐分（均分×人数+时间衰减）→ 新发布的排前。
func HandleCompanyMarket(c *gin.Context) {
	companyRoot := companyDir()
	var games []marketGame
	entries, err := os.ReadDir(companyRoot)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"games": []marketGame{}})
		return
	}
	// 推广记录：finance.json 里 spend_promo 的 target → 推广描述
	promos := map[string]string{}
	if lg := loadFinance(); lg.Entries != nil {
		for _, e := range lg.Entries {
			if e.Type == "spend_promo" && e.Project != "" {
				promos[e.Project] = e.Desc
			}
		}
	}
	now := time.Now()
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "coder-") {
			continue
		}
		projDir := filepath.Join(companyRoot, e.Name(), "projects")
		projs, err := os.ReadDir(projDir)
		if err != nil {
			continue
		}
		for _, p := range projs {
			if !p.IsDir() {
				continue
			}
			rb := loadRealReviews(filepath.Join(projDir, p.Name()), p.Name())
			g := marketGame{Project: p.Name(), Agent: e.Name()}
			if len(rb.Reviews) > 0 {
				avg := 0.0
				for _, r := range rb.Reviews {
					avg += float64(r.Score)
				}
				avg /= float64(len(rb.Reviews))
				g.AvgScore = avg
				g.Count = len(rb.Reviews)
				g.Award = reviewAwardFor(avg, len(rb.Reviews))
				g.HasReview = true
				// 推荐分：均分×人数（评分越高人越多越热）+ 时间衰减
				g.Score = avg*float64(len(rb.Reviews))*10 + 1.0
			} else {
				g.Score = 0.2 // 无评分新游垫底，靠推广位上曝光
			}
			// 推广位：花钱买的直接置顶
			if desc, ok := promos[p.Name()]; ok {
				g.Promoted = true
				g.PromoDesc = desc
				g.Score += 1000
			}
			_ = now
			games = append(games, g)
		}
	}
	// 排序：推广>推荐分
	sort.SliceStable(games, func(i, j int) bool {
		return games[i].Score > games[j].Score
	})
	c.JSON(http.StatusOK, gin.H{"games": games, "promoted_count": len(promos)})
}
