package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
)

func envPresent(keys ...string) bool {
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return false
		}
	}
	return true
}

func findNavicat() string {
	if explicit := strings.TrimSpace(os.Getenv("NAVICAT_PATH")); explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
	}
	if runtime.GOOS != "windows" {
		return ""
	}
	for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if root == "" {
			continue
		}
		for _, name := range []string{"Navicat Premium 17", "Navicat Premium 16", "Navicat Premium"} {
			candidate := filepath.Join(root, "PremiumSoft", name, "navicat.exe")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return ""
}

// HandleCompanyIntegrations 只报告真实连接准备度，不返回密钥，也不把“装了客户端”冒充数据库已连接。
func HandleCompanyIntegrations(c *gin.Context) {
	microsoftConfigured := envPresent("MICROSOFT_TENANT_ID", "MICROSOFT_CLIENT_ID") &&
		(strings.TrimSpace(os.Getenv("MICROSOFT_CLIENT_SECRET")) != "" || strings.TrimSpace(os.Getenv("MICROSOFT_CLIENT_CERTIFICATE")) != "")
	databaseConfigured := strings.TrimSpace(os.Getenv("RESCENE_DATABASE_URL")) != "" || strings.TrimSpace(os.Getenv("DATABASE_URL")) != ""
	navicatPath := findNavicat()
	c.JSON(http.StatusOK, gin.H{
		"microsoft": gin.H{
			"configured":   microsoftConfigured,
			"provider":     "Microsoft Graph",
			"capabilities": []string{"OneDrive", "SharePoint", "Outlook", "Teams", "Excel", "PowerPoint"},
			"missing": func() []string {
				if microsoftConfigured {
					return []string{}
				}
				return []string{"MICROSOFT_TENANT_ID", "MICROSOFT_CLIENT_ID", "MICROSOFT_CLIENT_SECRET 或证书"}
			}(),
		},
		"database": gin.H{
			"configured":   databaseConfigured,
			"provider":     "原生数据库连接",
			"capabilities": []string{"PostgreSQL", "MySQL", "MongoDB"},
		},
		"navicat": gin.H{
			"installed": navicatPath != "",
			"path":      navicatPath,
			"role":      "数据库运维客户端（不是数据库）",
		},
	})
}
