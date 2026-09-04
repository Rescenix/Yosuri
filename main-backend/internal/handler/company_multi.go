package handler

// company_multi.go —— 多文件项目交付（2026-09-04）。
//
// 单文件 HTML 只能承载无状态小工具（番茄钟就是天花板）。
// 凡是要「登录 + 数据 + 多页面 + 后端」的产品，交付链改为多文件结构：
//   index.html  +  app.js  +  styles.css  +  data.js（localStorage 伪后端）
// 页面有状态、有结构，不再是 hello world。

import "strings"

// companyDeliveryPlan 一次交付的工程选型。
type companyDeliveryPlan struct {
	MultiFile bool   // 是否多文件项目
	Reason    string // 选型理由（写进项目身份，供复盘）
}

// companyDecidePlan 按指令特征选型：
//   - 提及数据/账户/列表/管理/多页面 → 多文件
//   - 单一小工具（计时/计算/转换）      → 单文件快车道
func companyDecidePlan(brief string) companyDeliveryPlan {
	low := strings.ToLower(brief)
	multi := []string{"登录", "注册", "账", "数据", "存储", "列表", "管理", "多页面", "设置", "历史", "记录", "收藏", "账号", "统计"}
	for _, kw := range multi {
		if strings.Contains(low, kw) {
			return companyDeliveryPlan{MultiFile: true, Reason: "指令含「" + kw + "」→ 需要状态与存储，走多文件项目"}
		}
	}
	single := []string{"计时", "番茄", "倒计时", "计算", "换算", "转换", "生成器", "工具"}
	for _, kw := range single {
		if strings.Contains(low, kw) {
			return companyDeliveryPlan{MultiFile: false, Reason: "指令含「" + kw + "」→ 无状态小工具，单文件快车道"}
		}
	}
	// 兜底：含「应用/软件/系统/平台」这类大词默认多文件，否则单文件。
	if strings.Contains(low, "应用") || strings.Contains(low, "软件") || strings.Contains(low, "系统") || strings.Contains(low, "平台") {
		return companyDeliveryPlan{MultiFile: true, Reason: "指令含「应用/系统/平台」→ 默认多文件项目"}
	}
	return companyDeliveryPlan{MultiFile: false, Reason: "默认单文件小工具"}
}
