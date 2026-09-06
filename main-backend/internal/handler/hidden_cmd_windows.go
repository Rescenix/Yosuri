//go:build windows

package handler

import (
	"context"
	"os/exec"
	"syscall"
)

// createNoWindow = windows.CREATE_NO_WINDOW. HideWindow alone sets the
// *initial* show-state in STARTUPINFO, but PowerShell (and other shells)
// spawning a native console app like python.exe as a grandchild can still
// allocate a fresh, visible console for it — this is what actually caused
// the black terminal box flashing on every run_command call (reported by
// user while driving the Pokémon demo via run_command). CREATE_NO_WINDOW
// tells the OS not to create a console for the whole process tree at all,
// which is the reliable fix; HideWindow is kept too for older edge cases.
const createNoWindow = 0x08000000

// hiddenCommand wraps exec.Command with HideWindow: true on Windows,
// so child console processes (git, powershell, python, node, etc.)
// don't flash a visible terminal window when spawned from a GUI app.
func hiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}

// hiddenCommandContext wraps exec.CommandContext with HideWindow: true on Windows.
func hiddenCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}
