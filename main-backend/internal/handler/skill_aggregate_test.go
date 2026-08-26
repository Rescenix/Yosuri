package handler

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAggregateTestSkill(t *testing.T, root, rel, name, description, body string) string {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func setupAggregateTestDirs(t *testing.T) (string, string, string, string) {
	t.Helper()
	root := t.TempDir()
	hermes := filepath.Join(root, "hermes")
	claude := filepath.Join(root, "claude")
	codex := filepath.Join(root, "codex")
	agents := filepath.Join(root, "agents")
	t.Setenv("RESCENE_HERMES_SKILLS_DIR", hermes)
	t.Setenv("RESCENE_CLAUDE_SKILLS_DIR", claude)
	t.Setenv("RESCENE_CODEX_SKILLS_DIR", codex)
	t.Setenv("RESCENE_AGENTS_SKILLS_DIR", agents)
	t.Setenv("RESCENE_DATA_DIR", filepath.Join(root, "rescene"))
	return hermes, claude, codex, agents
}

func TestScanAggregateSkillsMergesPlatformsAndDetectsConflict(t *testing.T) {
	hermes, claude, _, agents := setupAggregateTestDirs(t)
	writeAggregateTestSkill(t, hermes, "writing/shared-skill", "shared-skill", "统一描述", "Hermes body")
	writeAggregateTestSkill(t, claude, "shared-skill", "shared-skill", "统一描述", "Claude body")
	writeAggregateTestSkill(t, agents, "only-codex", "only-codex", "Codex 兼容目录", "same")

	skills, platforms := scanAggregateSkills()
	if len(skills) != 2 || len(platforms) != 3 {
		t.Fatalf("unexpected inventory: skills=%d platforms=%d", len(skills), len(platforms))
	}
	for _, skill := range skills {
		if skill.Name == "shared-skill" && (!skill.Conflict || len(skill.Locations) != 2) {
			t.Fatalf("shared-skill conflict was not detected: %#v", skill)
		}
		if skill.Name == "only-codex" && skill.Locations[0].Platform != "codex" {
			t.Fatalf(".agents skill should belong to codex: %#v", skill)
		}
	}
}

func TestSyncSkillPackageCopiesAssetsAndBacksUpConflict(t *testing.T) {
	hermes, _, codex, _ := setupAggregateTestDirs(t)
	source := writeAggregateTestSkill(t, hermes, "frontend/demo", "demo-skill", "source", "new body")
	if err := os.MkdirAll(filepath.Join(source, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "references", "guide.md"), []byte("guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeAggregateTestSkill(t, codex, "demo-skill", "demo-skill", "old", "old body")

	destination, backup, err := syncSkillPackage(source, codex, "demo-skill", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if backup == "" {
		t.Fatal("expected backup for existing target")
	}
	for _, path := range []string{
		filepath.Join(destination, "SKILL.md"),
		filepath.Join(destination, "references", "guide.md"),
		filepath.Join(backup, "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected copied file %s: %v", path, err)
		}
	}
}

func TestSkillPackageChecksumIncludesSupportingFiles(t *testing.T) {
	hermes, claude, _, _ := setupAggregateTestDirs(t)
	first := writeAggregateTestSkill(t, hermes, "demo", "demo", "same", "same body")
	second := writeAggregateTestSkill(t, claude, "demo", "demo", "same", "same body")
	for _, item := range []struct{ root, content string }{{first, "one"}, {second, "two"}} {
		if err := os.MkdirAll(filepath.Join(item.root, "scripts"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(item.root, "scripts", "run.txt"), []byte(item.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	skills, _ := scanAggregateSkills()
	if len(skills) != 1 || !skills[0].Conflict {
		t.Fatalf("supporting-file difference should be a conflict: %#v", skills)
	}
}
