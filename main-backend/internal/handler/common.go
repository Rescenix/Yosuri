package handler

import (
	"os"
	"path/filepath"
)

func GetBooksDir() string {
	if dir := os.Getenv("BOOKS_DIR"); dir != "" {
		return dir
	}
	return filepath.Join("public", "books")
}
