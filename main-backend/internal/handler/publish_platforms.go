package handler

// publish_platforms.go — 多平台发布器：平台注册表（GUI 发布面板用）
// 10 个网文平台 + 发布规则（题材/字数/特点）

type PubPlatform struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	CreateURL string `json:"createUrl"`
	Genre     string `json:"genre"`
	MinLen    int    `json:"minLen"`
	Notes     string `json:"notes"`
}

var PubPlatforms = []PubPlatform{
	{"jjwxc", "晋江文学城", "jjwxc.net", "https://my.jjwxc.net/backend/managenovel.php", "女频·言情", 0, "IP 改编能力强，榜单公平，读者粘性高"},
	{"fanqie", "番茄小说", "fanqienovel.com", "https://fanqienovel.com/writer", "爽文·快节奏", 4000, "字节流量巨大，算法推荐精准，日更 4000+ 是基本要求"},
	{"zongheng", "纵横中文网", "zongheng.com", "https://www.zongheng.com/writer", "男频·玄幻都市", 0, "百度旗下老牌男频，买断/分成签约"},
	{"17k", "17K小说网", "17k.com", "https://www.17k.com/writer", "综合", 0, "老牌综合门户，作者扶持计划"},
	{"qimao", "七猫中文网", "qimao.com", "https://www.qimao.com/writer", "免费阅读", 0, "保底千字 20 元起，稳定收入"},
	{"faloo", "飞卢小说网", "faloo.com", "https://b.faloo.com/", "爽文·脑洞", 0, "更新节奏快，创意著称"},
	{"migu", "咪咕阅读", "cmread.com", "https://www.cmread.com/writer", "综合", 0, "中国移动旗下，全勤奖稳定"},
	{"heiyan", "黑岩网", "heiyan.com", "https://www.heiyan.com/writer", "悬疑推理", 0, "悬疑推理特色，IP 化成熟"},
	{"zhangyue", "掌阅科技", "ireader.com", "https://www.ireader.com/writer", "综合", 0, "自有 App 用户庞大，分发渠道"},
	{"douban", "豆瓣阅读", "douban.com", "https://read.douban.com/writer", "文艺严肃", 0, "文艺/严肃/类型文学，文学追求"},
}

// FindPubPlatform 按 ID 或名称找平台
func FindPubPlatform(key string) *PubPlatform {
	for i := range PubPlatforms {
		if PubPlatforms[i].ID == key || PubPlatforms[i].Name == key {
			return &PubPlatforms[i]
		}
	}
	return nil
}
