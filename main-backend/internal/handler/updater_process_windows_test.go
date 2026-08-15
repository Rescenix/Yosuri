//go:build windows

package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLaunchUpdateScriptHandlesPathsWithSpaces(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "update scripts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "script-ran.txt")
	scriptPath := filepath.Join(dir, "apply update.cmd")
	script := "@echo off\r\n> \"" + strings.ReplaceAll(marker, "%", "%%") + "\" echo ok\r\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := launchUpdateScript(scriptPath); err != nil {
		t.Fatalf("launchUpdateScript: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if got, err := os.ReadFile(marker); err == nil {
			if strings.TrimSpace(string(got)) != "ok" {
				t.Fatalf("unexpected marker content: %q", got)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("hidden update script did not run")
}
