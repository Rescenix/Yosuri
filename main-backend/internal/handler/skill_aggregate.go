package handler

// Skills 聚合管理器 —— 在 Hermes、Claude 与 Codex 的本地技能目录之间
// 发现并同步 Claude 风格的 SKILL.md 包。它只管理用户技能目录，不扫描插件缓存；
// 覆盖目标前会把旧目录备份到 Rescene 数据目录，便于人工恢复。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type skillPlatform struct {
	ID       string
	Label    string
	ScanDirs []string
	WriteDir string
}

type aggregateSkillLocation struct {
	Platform     string `json:"platform"`
	PlatformName string `json:"platform_name"`
	Path         string `json:"path"`
	RelativePath string `json:"relative_path"`
	Checksum     string `json:"checksum"`
}

type aggregateSkill struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Locations   []aggregateSkillLocation `json:"locations"`
	Conflict    bool                     `json:"conflict"`
}

type aggregatePlatformView struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Dirs      []string `json:"dirs"`
	WriteDir  string   `json:"write_dir"`
	Available bool     `json:"available"`
	Count     int      `json:"count"`
}

func aggregateSkillPlatforms() []skillPlatform {
	home, _ := os.UserHomeDir()
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" && home != "" {
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	hermesDir := envOrDefault("RESCENE_HERMES_SKILLS_DIR", filepath.Join(localAppData, "hermes", "skills"))
	claudeDir := envOrDefault("RESCENE_CLAUDE_SKILLS_DIR", filepath.Join(home, ".claude", "skills"))
	codexDir := envOrDefault("RESCENE_CODEX_SKILLS_DIR", filepath.Join(home, ".codex", "skills"))
	agentsDir := envOrDefault("RESCENE_AGENTS_SKILLS_DIR", filepath.Join(home, ".agents", "skills"))
	return []skillPlatform{
		{ID: "hermes", Label: "Hermes", ScanDirs: []string{hermesDir}, WriteDir: hermesDir},
		{ID: "claude", Label: "Claude", ScanDirs: []string{claudeDir}, WriteDir: claudeDir},
		{ID: "codex", Label: "Codex", ScanDirs: uniqueCleanPaths([]string{codexDir, agentsDir}), WriteDir: codexDir},
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func uniqueCleanPaths(paths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		clean := filepath.Clean(path)
		key := strings.ToLower(clean)
		if path == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, clean)
	}
	return out
}

func scanAggregateSkills() ([]aggregateSkill, []aggregatePlatformView) {
	byName := map[string]*aggregateSkill{}
	platforms := aggregateSkillPlatforms()
	views := make([]aggregatePlatformView, 0, len(platforms))
	for _, platform := range platforms {
		view := aggregatePlatformView{ID: platform.ID, Label: platform.Label, Dirs: platform.ScanDirs, WriteDir: platform.WriteDir}
		for _, root := range platform.ScanDirs {
			if info, err := os.Stat(root); err == nil && info.IsDir() {
				view.Available = true
			}
			_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					if entry != nil && entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if entry.IsDir() || !strings.EqualFold(entry.Name(), "SKILL.md") {
					return nil
				}
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					return nil
				}
				name, description, _ := parseSkillMD(string(data))
				if strings.TrimSpace(name) == "" {
					name = filepath.Base(filepath.Dir(path))
				}
				name = strings.TrimSpace(name)
				if name == "" {
					return nil
				}
				rel, _ := filepath.Rel(root, filepath.Dir(path))
				checksum, checksumErr := skillPackageChecksum(filepath.Dir(path))
				if checksumErr != nil {
					sum := sha256.Sum256(data)
					checksum = hex.EncodeToString(sum[:8])
				}
				location := aggregateSkillLocation{
					Platform: platform.ID, PlatformName: platform.Label, Path: filepath.Dir(path),
					RelativePath: filepath.ToSlash(rel), Checksum: checksum,
				}
				key := strings.ToLower(name)
				skill := byName[key]
				if skill == nil {
					skill = &aggregateSkill{Name: name, Description: description, Locations: []aggregateSkillLocation{}}
					byName[key] = skill
				}
				if skill.Description == "" {
					skill.Description = description
				}
				skill.Locations = append(skill.Locations, location)
				view.Count++
				return nil
			})
		}
		views = append(views, view)
	}

	skills := make([]aggregateSkill, 0, len(byName))
	for _, skill := range byName {
		checksums := map[string]bool{}
		for _, location := range skill.Locations {
			checksums[location.Checksum] = true
		}
		skill.Conflict = len(checksums) > 1
		sort.Slice(skill.Locations, func(i, j int) bool {
			if skill.Locations[i].Platform == skill.Locations[j].Platform {
				return skill.Locations[i].RelativePath < skill.Locations[j].RelativePath
			}
			return skill.Locations[i].Platform < skill.Locations[j].Platform
		})
		skills = append(skills, *skill)
	}
	sort.Slice(skills, func(i, j int) bool { return strings.ToLower(skills[i].Name) < strings.ToLower(skills[j].Name) })
	return skills, views
}

// skillPackageChecksum 把 SKILL.md 及其 references/scripts/assets 等附属文件一起
// 纳入版本判断；仅正文相同但脚本不同，也应在聚合页显示为冲突。
func skillPackageChecksum(root string) (string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("技能包包含符号链接")
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	hash := sha256.New()
	for _, path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(hash, filepath.ToSlash(rel))
		_, _ = hash.Write([]byte{0})
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)[:8]), nil
}

func HandleAggregateSkills(c *gin.Context) {
	skills, platforms := scanAggregateSkills()
	c.JSON(http.StatusOK, gin.H{"skills": skills, "platforms": platforms})
}

type aggregateSkillSyncRequest struct {
	Name       string   `json:"name"`
	Source     string   `json:"source"`
	SourcePath string   `json:"source_path"`
	Targets    []string `json:"targets"`
}

func HandleSyncAggregateSkill(c *gin.Context) {
	var body aggregateSkillSyncRequest
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" || body.Source == "" || len(body.Targets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供技能名、来源端和至少一个目标端"})
		return
	}
	skills, _ := scanAggregateSkills()
	var source *aggregateSkillLocation
	var selectedSkill *aggregateSkill
	for _, skill := range skills {
		if !strings.EqualFold(skill.Name, body.Name) {
			continue
		}
		selectedSkill = &skill
		for i := range skill.Locations {
			pathMatches := body.SourcePath == "" || filepath.Clean(skill.Locations[i].Path) == filepath.Clean(body.SourcePath)
			if skill.Locations[i].Platform == body.Source && pathMatches {
				source = &skill.Locations[i]
				break
			}
		}
	}
	if source == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "没有找到指定来源的技能"})
		return
	}
	platformByID := map[string]skillPlatform{}
	for _, platform := range aggregateSkillPlatforms() {
		platformByID[platform.ID] = platform
	}
	results := make([]gin.H, 0, len(body.Targets))
	seen := map[string]bool{}
	for _, targetID := range body.Targets {
		if targetID == body.Source || seen[targetID] {
			continue
		}
		seen[targetID] = true
		target, ok := platformByID[targetID]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的目标端：" + targetID})
			return
		}
		destinationPath := filepath.Join(target.WriteDir, sanitizeAggregateSkillDir(body.Name))
		if selectedSkill != nil {
			for _, location := range selectedSkill.Locations {
				if location.Platform == targetID {
					destinationPath = location.Path
					break
				}
			}
		}
		destination, backup, err := syncSkillPackageTo(source.Path, destinationPath, body.Name, targetID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("同步到 %s 失败：%v", target.Label, err)})
			return
		}
		results = append(results, gin.H{"platform": targetID, "path": destination, "backup": backup})
	}
	if len(results) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目标端不能只有来源端"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "results": results})
}

func syncSkillPackage(sourceDir, targetRoot, skillName, platform string) (string, string, error) {
	cleanName := sanitizeAggregateSkillDir(skillName)
	if cleanName == "" {
		return "", "", fmt.Errorf("技能名不能安全映射为目录")
	}
	return syncSkillPackageTo(sourceDir, filepath.Join(targetRoot, cleanName), skillName, platform)
}

func syncSkillPackageTo(sourceDir, destination, skillName, platform string) (string, string, error) {
	cleanName := sanitizeAggregateSkillDir(skillName)
	if cleanName == "" {
		return "", "", fmt.Errorf("技能名不能安全映射为目录")
	}
	if err := validateSkillPackage(sourceDir); err != nil {
		return "", "", err
	}
	targetRoot := filepath.Dir(destination)
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return "", "", err
	}
	staging, err := os.MkdirTemp(targetRoot, ".rescene-skill-sync-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(staging)
	stagedSkill := filepath.Join(staging, cleanName)
	if err := copySkillTree(sourceDir, stagedSkill); err != nil {
		return "", "", err
	}

	backup := ""
	if _, err := os.Stat(destination); err == nil {
		stamp := time.Now().Format("20060102-150405.000")
		backup = filepath.Join(resceneUserDataDir(), "skill-switch-backups", stamp, platform, cleanName)
		if err := copySkillTree(destination, backup); err != nil {
			return "", "", fmt.Errorf("备份旧技能失败: %w", err)
		}
		if err := os.RemoveAll(destination); err != nil {
			return "", backup, err
		}
	}
	if err := os.Rename(stagedSkill, destination); err != nil {
		return "", backup, err
	}
	return destination, backup, nil
}

func sanitizeAggregateSkillDir(name string) string {
	clean := skillNameSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	return strings.Trim(clean, "-")
}

func validateSkillPackage(root string) error {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("技能来源目录无效")
	}
	if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err != nil {
		return fmt.Errorf("技能来源缺少 SKILL.md")
	}
	return nil
}

func copySkillTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("技能包不支持符号链接: %s", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("技能包路径越界")
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("技能包包含不支持的文件类型: %s", path)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyRegularFile(path, target, info.Mode().Perm())
	})
}

func copyRegularFile(source, destination string, mode os.FileMode) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	dst, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		src.Close()
		return err
	}
	_, copyErr := io.Copy(dst, src)
	srcCloseErr := src.Close()
	dstCloseErr := dst.Close()
	if copyErr != nil {
		return copyErr
	}
	if srcCloseErr != nil {
		return srcCloseErr
	}
	return dstCloseErr
}
