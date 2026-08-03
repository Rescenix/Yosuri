package main

func hasBackgroundFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--background" {
			return true
		}
	}
	return false
}
