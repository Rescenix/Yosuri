//go:build bindings

package main

// Wails executes the application with the bindings build tag while generating
// frontend bindings. That helper process has no Explorer notification area.
func (a *DesktopApp) startTray()  {}
func (a *DesktopApp) showWindow() {}
func (a *DesktopApp) stopTray()   {}
