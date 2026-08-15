//go:build !windows

package handler

import "fmt"

func launchUpdateScript(string) error {
	return fmt.Errorf("hot update is only supported on Windows")
}
