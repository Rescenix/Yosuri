package main

// user_reviews.go — 发行评测：产品发布后，用户 Agent 打分评论
//
// 公司产品产出 delivery.manifest.json（status=published）后，一组模拟用户
// Agent（学生/社畜/主播/独立开发/抠门党…各有口味）用各自的免费模型读取
// 产品实际产出 → 打分 + 写玩家向评论 → 落盘 08-用户评测.json。
// 模拟用户带模型名（如“DeepSeek V4 Flash·商汤评测”），秀模型池又真实。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UserAgent 一个模拟用户
type UserAgent struct {
	Name     string `json:"name"`     // 用户 Agent 名
	Emoji    string `json:"emoji"`    // 头像
	Profile  string `json:"profile"`  // 画像（口味/需求/场景）
	ModelID  string `json:"model_id"` // 用哪个免费模型评测
	ModelTag string `json:"model_tag"` // 展示用模型名（秀池子）
}

// UserReview 单个用户的打分+评论
type UserReview struct {
	UserAgent
	Score   int    `json:"score"`   // 1-10
	Comment string `json:"comment"` // 玩家向评论（≤120字）
}

// UserReviewsBundle 发行评测包
type UserReviewsBundle struct {
	Project   string       `json:"project"`
	GeneratedAt string     `json:"generated_at"`
	AvgScore  float64      `json:"avg_score"`
	Reviews   []UserReview `json:"reviews"`
	Summary   string       `json:"summary"` // CEO 汇总：总评
}

// MockUserPool 模拟用户池
var MockUserPool = []UserAgent{
	{Name: "阿伟·高三生", Emoji: "🎒", Profile: "高中学生，手机流量党，只看免费的，爱肝游戏、爱分享到班群", ModelID: ""},
	{Name: "小林·社畜", Emoji: "💼", Profile: "996 上班族，通勤路上刷 B 站，想要能直接用的工具，讨厌看说明书", ModelID: ""},
	{Name: "苏苏·主播", Emoji: "🎤", Profile: "游戏区女主播，对画面和互动性挑剔，粉丝多，说好能带量", ModelID: ""},
	{Name: "老 K·独立开发者", Emoji: "🧑‍💻", Profile: "独自创业的开发者，看重技术实现和可扩展性，爱挑代码毛病", ModelID: ""},
	{Name: "王姐·抠门党", Emoji: "🫰", Profile: "家庭主妇，精打细算，免费的东西先怀疑有没有坑，被安利后很能传播", ModelID: ""},
	{Name: "Yuki·二次元", Emoji: "🌸", Profile: "二次元爱好者，对 UI 颜值要求极高，丑的绝不碰，好看的天天用", ModelID: ""},
}

// userReviewsOnce 每项目只评测一次（避免重复烧模型）
var userReviewsOnce sync.Map

// releaseUserReviews 产品发售后触发：用户打分评论，落盘 08-用户评测.json
func releaseUserReviews(projectDir, project, brief string) (UserReviewsBundle, error) {
	manifest := UserReviewsBundle{
		Project:   project,
		GeneratedAt: time.Now().Format("2006-01-02 15:04"),
	}
	// 只评测一次
	if _, done := userReviewsOnce.LoadOrStore(projectDir, true); done {
		return manifest, fmt.Errorf("【%s】已评测过，跳过", project)
	}

	models := snapshotWorkingModels()
	if len(models) == 0 {
		models = workingModelsSnapshotFallback()
	}
	if len(models) == 0 {
		return manifest, fmt.Errorf("【%s】免费模型池为空，无法评测", project)
	}

	var wg sync.WaitGroup
	reviews := make([]UserReview, len(MockUserPool))
	for i, ua := range MockUserPool {
		wg.Add(1)
		go func(i int, ua UserAgent) {
			defer wg.Done()
			// 每个用户绑定池子里一个模型（round-robin，秀模型多样性）
			m := models[i%len(models)]
			ua.ModelID = m.ID
			ua.ModelTag = m.Name
			score, comment := userJudge(ua, m, project, brief)
			reviews[i] = UserReview{UserAgent: ua, Score: score, Comment: comment}
		}(i, ua)
	}
	wg.Wait()

	// 平均分
	total := 0
	for _, r := range reviews {
		total += r.Score
	}
	if len(reviews) > 0 {
		manifest.AvgScore = float64(total) / float64(len(reviews))
	}
	manifest.Reviews = reviews
	manifest.Summary = summarizeReviews(project, manifest)

	// 落盘
	out := filepath.Join(projectDir, "08-用户评测.json")
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(out, data, 0644); err != nil {
		return manifest, err
	}
	return manifest, nil
}

// snapshotWorkingModels 拷贝当前可用模型池（并发安全）
func snapshotWorkingModels() []FreeModel {
	wmMu.RLock()
	defer wmMu.RUnlock()
	out := make([]FreeModel, 0, len(workingModels))
	for _, m := range workingModels {
		if m.Keyless { // 免 key 的优先（免费算力铁律）
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		for _, m := range workingModels {
			out = append(out, m)
		}
	}
	return out
}

// workingModelsSnapshotFallback 兜底：workingModels 未初始化时用目录轮换
func workingModelsSnapshotFallback() []FreeModel {
	wmMu.RLock()
	defer wmMu.RUnlock()
	out := make([]FreeModel, len(workingModels))
	copy(out, workingModels)
	return out
}

// userJudge 单个用户 Agent 用自己模型评测产品 → (打分, 评论)
func userJudge(ua UserAgent, m FreeModel, project, brief string) (int, string) {
	ctx := brief
	if len(ctx) > 800 {
		ctx = ctx[:800]
	}
	prompt := fmt.Sprintf(`你是【%s%s】，一个真实用户，刚刚试用了 AI 公司 Rescene 发布的新产品《%s》。

你的身份：%s
你用 %s（模型：%s）体验了这个产品。

产品简介：
%s

请像真实用户一样给产品打分和写评论：
1. 打分 1-10 分（10 分制，别都打 8/9，真实用户口味不同、各有挑剔理由）
2. 写一条 60 字以内的玩家向评论：说出你真实的感受、适合谁用、会不会推荐给别人
3. 口吻符合你的身份和画像（学生党/社畜/主播/开发者/抠门党/二次元）

输出严格 JSON：{"score": 数字, "comment": "评论文字"}`,
		ua.Emoji, ua.Name, project, ua.Profile, m.Name, m.Model, ctx)

	out, err := callWithRetry(&m, prompt, 1, 10*time.Second)
	if err != nil {
		return 5, "模型评测中死机了…下次再来 [捂脸]" // 失败给中性分，不阻塞
	}
	score, comment := parseJudgeResult(out)
	return score, comment
}

// parseJudgeResult 从模型输出解析 {score, comment}
func parseJudgeResult(out string) (int, string) {
	score := 5
	comment := strings.TrimSpace(out)
	if m := scoreRe.FindStringSubmatch(out); m != nil {
		if v, err := strconv.Atoi(strings.TrimSpace(m[1])); err == nil && v >= 1 && v <= 10 {
			score = v
		}
	}
	if m := commentRe.FindStringSubmatch(out); m != nil {
		comment = strings.TrimSpace(m[1])
	}
	return score, runeClip(comment, 120)
}

var scoreRe = regexp.MustCompile(`"score"\s*:\s*(\d+)`)
var commentRe = regexp.MustCompile(`"comment"\s*:\s*"([^"]+)"`)

// summarizeReviews 生成总评（自己算平均分，不依赖调用方先算）
func summarizeReviews(project string, b UserReviewsBundle) string {
	n := len(b.Reviews)
	if n == 0 {
		return fmt.Sprintf("%s 暂无用户评测", project)
	}
	total, pos := 0, 0
	for _, r := range b.Reviews {
		total += r.Score
		if r.Score >= 7 {
			pos++
		}
	}
	avg := float64(total) / float64(n)
	return fmt.Sprintf("《%s》%d 位用户评测 · 平均 %.1f 分 · %d 位推荐", project, n, avg, pos)
}