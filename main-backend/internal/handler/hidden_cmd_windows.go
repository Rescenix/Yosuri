//go:build windows

package handler

import (
	"context"
	"os/exec"
	"syscall"
)

// hiddenCommand wraps exec.Command with HideWindow: true on Windows,
// so child console processes (git, powershell, python, node, etc.)
// don't flash a visible terminal window when spawned from a GUI app.
func hiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

// hiddenCommandContext wraps exec.CommandContext with HideWindow: true on Windows.
func hiddenCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}
