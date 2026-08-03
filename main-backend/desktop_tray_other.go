//go:build !windows && !bindings

package main

func (a *DesktopApp) startTray()  {}
func (a *DesktopApp) showWindow() {}
func (a *DesktopApp) stopTray()   {}
