//go:build !windows

package handler

import (
	"context"
	"os/exec"
)

// hiddenCommand is a no-op wrapper on non-Windows (no HideWindow needed).
func hiddenCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// hiddenCommandContext is a no-op wrapper on non-Windows.
func hiddenCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// detachedCommand is a no-op wrapper on non-Windows.
func detachedCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
