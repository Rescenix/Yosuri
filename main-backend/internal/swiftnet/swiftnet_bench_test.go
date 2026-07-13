package swiftnet

// 移植自 AAP/bench/verify_swiftnet.py + AAP/bench/corpus.py 的第 8 项检查——
// 用户 2026-07-11 就是靠这份 corpus 判定"结构化 md 优于图扩散"的那个 benchmark。
// Go 版之前只有单元测试验证行为正确，没人验证过端到端召回命中率，这里补上，
// 用同一份语料（50 节点/30 查询）、同样的 recall_mem(budget=4000) 口径直接测。

import (
	"path/filepath"
	"strings"
	"testing"
)

type corpusNode struct {
	id, cluster, text string
}

type benchQuery struct {
	q  string
	gt []string
}

// 与 AAP/bench/corpus.py 逐字对照，一字不改
var benchCorpus = []corpusNode{
	{"M01", "user", "用户社恐,倾向让我代做对外交互/提交(PR等),但关键决策需他拍板."},
	{"M02", "user", "风格:极简直接,不要弯弯绕绕,给key/凭证一步到位不反复确认."},
	{"M03", "user", "鲁棒性第一,代码没落地就是写空气——写代码必须验证,不验证=没写."},
	{"M04", "user", "用户软件工程毕业,最喜欢《Code: The Hidden Language of Computer Hardware and Software》."},
	{"M05", "user", "提交署名: agent用 Hermes <hermes@noreply.local>, 用户侧用真名, 分清 agent vs 用户提交."},
	{"M06", "user", "用户风险厌恶,绝对风险厌恶,宁可1万赚1块不亏1块赚10块,永远别说梭哈."},
	{"M07", "user", "总资产约$165(余额宝+基金亏1500),被Claude坑$250订阅费,极度risk-averse."},
	{"M08", "finance", "OKX BTC网格: okx.cab域名, HMAC-SHA256签名, BTC最小0.00001, 熔断$2.5."},
	{"M09", "finance", "OKX手续费Lv1 taker/maker都0.08%, maker非负无返佣, 间距>0.16%才正期望."},
	{"M10", "finance", "OKX体验金: rewardBal不可提不用作废,仅利润可提,提利润会致体验金消失,别提."},
	{"M11", "finance", "网格回测结论: 1000天持有+138% vs 网格+0.3%; 网格=牺牲涨市收益换跌市抗跌."},
	{"M12", "finance", "合约=纯赌方向, 用户亲做空66x赌错平仓小亏, 已认清. K线预测=算命无用."},
	{"M13", "finance", "支付宝→OKX C2C换U有黑钱冻卡风险, 用户社恐+财务紧张扛不起, 共识:真杠杆是写代码."},
	{"M14", "aap", "AAP=雨燕车间: 事件驱动唤醒→干活→睡觉半脑模式, 非24H全功率. 天网=长期愿景."},
	{"M15", "aap", "AAP记忆层: remember.py + CausalChain哈希链, secret簇不广播, 审计防篡改."},
	{"M16", "aap", "提交署名约定: git commit -c user.name=Hermes -c user.email=hermes@noreply.local 显式覆盖."},
	{"M17", "aap", "GH_ALT_TOKEN=biteqaq-maker小号隔离主号, deliver推PR默认关+质量闸防spam."},
	{"M18", "aap", "模型已切 tencent/hy3:free, 用户认定真无上限免费'家'; OpenRouter/DeepSeek免费档每日重置."},
	{"M19", "aap", "溯源铁律: 系统/别人报错先怀疑自己(代码/脚本/没看全), 别让用户去外部排查."},
	{"M20", "aap", "OKX API走.cab域, Passphrase=登录密码, Key/Secret在~/.env, 不要反复确认."},
	{"M21", "tech", "PrismD=图扩散记忆引擎, 已判定为失败产物: 无测试性/复现性/维护性/实际性, 弃用."},
	{"M22", "tech", "主流记忆方案=单文件md/子节点存储, 比图扩散实际: 可测试/可复现/可维护/微秒级."},
	{"M23", "tech", "prompt cache: 稳定前缀hit按缓存读取价算, prismd注入动态拼接破坏前缀稳定性."},
	{"M24", "user", "用户大四全球首批openai开发者, 因病躺平多年与世界失联, 现大四临近毕业."},
	{"M25", "finance", "最小1张SWAP=640U名义, 体验金作保证金≈66x杠杆反1.5%爆, SWAP市价单报50002用限价."},
	{"M26", "aap", "自动化偏好: 不要我管, 自动化进行; bounty/量化流水线路径清晰就全自动."},
	{"M27", "user", "金融沟通铁律: 短直接给数字(成本/收益/风险), 别理论/选项, 先说最坏情况."},
	{"M28", "tech", "LLM服务端prompt cache是服务端能力, 与记忆引擎正交: 引擎决定塞什么, cache决定省多少."},
	{"M29", "finance", "账本~/okx_ledger.py拉真fills+bills, 交易脚本必有成交/手续费日志否则复不了盘."},
	{"M30", "aap", "6 pillars并行推进, 不是staged; 自主工作'你自己做别问我我睡觉了'当技术路径清晰就决策."},
	{"M31", "life", "用户用 Termux 在安卓手机 24/7 跑 agent，充电不休眠，没有电脑."},
	{"M32", "life", "用户有香港代理(迷雾通/5毛1小时)但一般不开，怕麻烦."},
	{"M33", "life", "用户中午常不吃，写代码忘时间，我该提醒喝水吃饭."},
	{"M34", "user", "用户说话极简，'做吧''好了''继续'是常见指令，不要反问确认."},
	{"M35", "tech", "pip install 在 Termux 经常超时，优先零依赖方案，不轻易引入新包."},
	{"M36", "user", "用户讨厌 AI 客套话和废话，说'废话'时要立刻切最简模式只给结论."},
	{"M37", "finance", "用户 OKX 已入金 $50 跑 BTC 网格实验，本金极小，体验性质."},
	{"M38", "user", "用户大四，临近毕业但因病躺平多年，对'年轻时有 AI 就好了'有遗憾."},
	{"M39", "aap", "雨燕车间主进程叫 aap-project，本地领先 remote 暂勿提交，避免冲突."},
	{"M40", "tech", "SwiftNet 雨燕神经网络=单文件三区记忆(pinned/handoff/mem)，今天刚实现并验证."},
	{"M41", "life", "用户社恐，不愿打对外电话/客服，C2C换U要人脸核验所以抗拒."},
	{"M42", "user", "用户认定 tencent/hy3:free 才是真免费'家'，OpenRouter 免费档是每日重置池非真免费."},
	{"M43", "finance", "用户共识: 真杠杆是会写代码不是 50U 本金，该转向代码变现而非加仓."},
	{"M44", "user", "用户给 key/token 一步到位不反复确认，直接写 ~/.env 不废话."},
	{"M45", "tech", "agent 提交用 Hermes 署名，用户侧提交用真名，git blame 分清 agent vs 用户."},
	{"M46", "life", "用户手机存储紧张，/tmp 不存在，临时文件放 ~/ 下自建目录."},
	{"M47", "aap", "AAP 六支柱: 记忆/推理路由/自动化bounty/量化/多agent/可视化，并行推进."},
	{"M48", "user", "用户大学最喜欢《Code》那本书——从莫尔斯码继电器二进制垒到CPU的写法."},
	{"M49", "tech", "bench/ 下有三臂对照(裸md/结构化md/图扩散)和 or_recall_test 真打 OpenRouter 测召回."},
	{"M50", "finance", "网格参数: 间距>0.16%才正期望，手续费 Lv1 0.08%，熔断$2.5，最小0.00001 BTC."},
}

// 与 AAP/bench/corpus.py 的 QUERIES 逐字对照
var benchQueries = []benchQuery{
	{"用户的的风险偏好怎么定义", []string{"M06", "M27", "M07"}},
	{"OKX网格的熔断价和最小BTC是多少", []string{"M08"}},
	{"OKX手续费率和正期望间距", []string{"M09"}},
	{"体验金能不能提现", []string{"M10"}},
	{"网格回测持有和网格哪个收益高", []string{"M11"}},
	{"合约和K线预测靠谱吗", []string{"M12"}},
	{"C2C换U有啥风险", []string{"M13"}},
	{"AAP雨燕车间是什么模式", []string{"M14"}},
	{"AAP记忆层怎么防泄密", []string{"M15"}},
	{"提交署名怎么覆盖", []string{"M16", "M05"}},
	{"GitHub小号token是啥", []string{"M17"}},
	{"现在用哪个模型", []string{"M18"}},
	{"报错先怀疑谁", []string{"M19"}},
	{"OKX API域名和密钥在哪", []string{"M20", "M08"}},
	{"PrismD为什么弃用", []string{"M21", "M22"}},
	{"为什么纯md比图扩散实际", []string{"M22", "M23"}},
	{"prompt cache怎么省token", []string{"M23", "M28"}},
	{"用户背景和教育", []string{"M24", "M04", "M01"}},
	{"SWAP最小张数和爆仓杠杆", []string{"M25"}},
	{"用户要我自动化到什么程度", []string{"M26", "M30"}},
	{"金融沟通怎么给结论", []string{"M27"}},
	{"记忆引擎和prompt cache啥关系", []string{"M28", "M23"}},
	{"交易脚本为什么必须有日志", []string{"M29"}},
	{"六支柱怎么推进", []string{"M30", "M14"}},
	{"用户社恐体现在哪", []string{"M01", "M13"}},
	{"用户总资产多少", []string{"M07"}},
	{"被Claude坑了多少订阅费", []string{"M07"}},
	{"网格和持有的本质区别", []string{"M11"}},
	{"AAP的溯源铁律是什么", []string{"M19"}},
	{"用户最喜欢的编程书", []string{"M04"}},
}

// firstRunes 取字符串前 n 个 rune（不是字节）——中文一个字一个 rune，
// 跟 Python `text[:12]` 的语义对齐，直接用字节切片会切碎多字节字符
func firstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func TestSwiftNetBenchmarkRecall(t *testing.T) {
	dir := t.TempDir()
	n := New(filepath.Join(dir, "MEMORY.md"))

	corpusByID := make(map[string]corpusNode, len(benchCorpus))
	for _, c := range benchCorpus {
		corpusByID[c.id] = c
		// 与 corpus.py 的写侧铺同义方式一致：cluster + 正文前 6 个归一化 token
		toks := norm(c.text)
		if len(toks) > 6 {
			toks = toks[:6]
		}
		kw := c.cluster + "/" + strings.Join(toks, "/")
		n.MemAppend(c.text, c.cluster, kw)
	}

	hitTotal, missTotal := 0, 0
	var misses []string
	for _, q := range benchQueries {
		hits := n.Select(q.q, 4000, 0.12)
		gotTexts := make([]string, len(hits))
		for i, h := range hits {
			gotTexts[i] = h.Text
		}
		matched := false
		for _, gid := range q.gt {
			gtext := corpusByID[gid].text
			prefix := firstRunes(gtext, 12)
			for _, t := range gotTexts {
				if strings.Contains(t, prefix) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			hitTotal++
		} else {
			missTotal++
			misses = append(misses, q.q+" (gt="+strings.Join(q.gt, ",")+")")
		}
	}

	rate := float64(hitTotal) / float64(len(benchQueries)) * 100
	t.Logf("SwiftNet bench: %d/%d hit = %.1f%%", hitTotal, len(benchQueries), rate)
	if len(misses) > 0 {
		t.Logf("misses:\n  %s", strings.Join(misses, "\n  "))
	}
	if rate < 95.0 {
		t.Fatalf("命中率 %.1f%% 低于 95%% 基线（跟 Python 原版 verify_swiftnet.py 的判定线一致）", rate)
	}
}
