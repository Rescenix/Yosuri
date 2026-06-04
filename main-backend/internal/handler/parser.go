// backend/internal/handler/parser.go
package handler

import (
	"regexp"
	"strings"
)

func parseEmotion(reply string) (string, string) {
	re := regexp.MustCompile(`\[emotion:(\w+)\]`)
	matches := re.FindStringSubmatch(reply)
	if len(matches) >= 2 {
		emotion := matches[1]
		cleanReply := re.ReplaceAllString(reply, "")
		return strings.TrimSpace(cleanReply), emotion
	}
	return reply, "calm"
}

func parseAction(reply string) (string, string) {
	re := regexp.MustCompile(`\[action:([^\]]+)\]`)
	matches := re.FindStringSubmatch(reply)
	if len(matches) >= 2 {
		action := matches[1]
		cleanReply := re.ReplaceAllString(reply, "")
		return strings.TrimSpace(cleanReply), action
	}
	return reply, ""
}

func cleanInvalidChars(s string) string {
	var result strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' || r >= ' ' {
			result.WriteRune(r)
		}
	}
	return result.String()
}
