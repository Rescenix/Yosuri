package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// maskKey Hermes 同款密钥脱敏：只露前 4 + 后 4，中间全星号。空串返回空。
// 绝不返回明文 key；诊断端点/doctor skill 输出、截图里都只出现脱敏形式。
func maskKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 8 {
		return strings.Repeat("*", len(k))
	}
	return k[:4] + "..." + k[len(k)-4:]
}

// HandleDiagnose GET /api/diagnose
// 本地健康诊断快照：给 agent（doctor skill）自动检测 Rescene 配置/运行问题用。
// 安全边界：key 一律走 maskKey 脱敏（sk-...abcd），绝不回明文；
// 不返回用户记忆/会话内容，只返回配置状态与运行健康。
func HandleDiagnose(c *gin.Context) {
	// 1. 聚合池 auto 链健康（与 /api/aggregate/health 同源）
	autoChain := make([]gin.H, 0, 8)
	dead := 0
	for i, b := range aggAutoChain() {
		m := aggModelHealth(b)
		if m.Disabled || m.Signal <= 0 {
			dead++
		}
		lastUsed := ""
		if !m.LastUsed.IsZero() {
			lastUsed = m.LastUsed.Format(time.RFC3339)
		}
		autoChain = append(autoChain, gin.H{
			"order":     i + 1,
			"id":        m.ID,
			"name":      m.Name,
			"vendor":    aggBackendVendor(b),
			"model":     m.Model,
			"keyless":   m.Keyless,
			"signal":    m.Signal,
			"probe_ms":  m.ProbeMs,
			"real_ms":   m.RealMs,
			"last_used": lastUsed,
			"disabled":  m.Disabled,
		})
	}

	// 2. 免费池探活统计
	freeTotal, freeDead := 0, 0
	for _, f := range freeModelCatalog {
		freeTotal++
		if probeSignalByDef(f) <= 0 {
			freeDead++
		}
	}

	// 3. 提供方 key 状态（脱敏）
	entries, _ := loadModelConfigs("")
	providers := make([]gin.H, 0, len(entries))
	keyed := 0
	for _, e := range entries {
		masked := maskKey(e.APIKey)
		if masked != "" {
			keyed++
		}
		providers = append(providers, gin.H{
			"id":         e.ID,
			"name":       e.Name,
			"endpoint":   e.Endpoint,
			"key_masked": masked, // 如 sk-...abcd；空 = 未配置
			"keyless":    e.Keyless,
		})
	}

	// 4. 设置开关
	memSync := memorySyncEnabled()
	lanOn := lanSync != nil && lanSync.Running()

	// 5. 自动问题清单（doctor 直接读，不用自己算）
	problems := make([]string, 0, 6)
	if len(autoChain) > 0 && dead == len(autoChain) {
		problems = append(problems, "聚合 auto 链全部不可用：模型池疑似全挂，检查各厂商探活/额度")
	} else if dead > 0 {
		problems = append(problems, fmt.Sprintf("auto 链有不可用候选：%d/%d 挂了，auto 会自动跳过（见 disabled 明细）", dead, len(autoChain)))
	}
	if freeTotal > 0 && freeDead == freeTotal {
		problems = append(problems, "免费模型池探活全灭：所有免费模型 signal<=0，可能整体断网或厂商下架")
	}
	if len(providers) > 0 && keyed == 0 {
		problems = append(problems, "没有任何提供方配置 API key（全靠免 key 池兜底）")
	}
	if !memSync {
		problems = append(problems, "云端记忆同步已关闭：换设备不会自动恢复记忆")
	}
	if !lanOn {
		problems = append(problems, "局域网同步未开启：手机无法内网同步对话（按需开启，非异常）")
	}
	if len(problems) == 0 {
		problems = append(problems, "未发现明显问题")
	}

	c.JSON(http.StatusOK, gin.H{
		"app": gin.H{"name": "re0", "diagnose_at": time.Now().Format(time.RFC3339)},
		"summary": gin.H{
			"auto_chain_total":   len(autoChain),
			"auto_chain_healthy": len(autoChain) - dead,
			"free_pool_total":    freeTotal,
			"free_pool_dead":     freeDead,
			"providers_total":    len(providers),
			"providers_with_key": keyed,
			"memory_sync":        memSync,
			"lan_sync":           lanOn,
			"problems":           problems,
		},
		"auto_chain": autoChain,
		"providers":  providers,
	})
}
