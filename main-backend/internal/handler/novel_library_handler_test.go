package handler

import (
	"testing"
	"time"
)

func TestNovelLibraryPersistsAndSortsRecentBook(t *testing.T) {
	t.Setenv("RESCENE_NOVELS_DIR", t.TempDir())
	old := novelBook{ID: "old", Title: "旧书", Chapters: []novelChapter{}, LastOpenedAt: time.Now().Add(-time.Hour)}
	recent := novelBook{ID: "recent", Title: "最近的书", Chapters: []novelChapter{{ID: "c1", Title: "第一章", Content: "正文", Status: "draft"}}, LastOpenedAt: time.Now()}
	if err := saveNovelBooks([]novelBook{old, recent}); err != nil {
		t.Fatal(err)
	}
	books, err := loadNovelBooks()
	if err != nil {
		t.Fatal(err)
	}
	sortNovelBooks(books)
	if len(books) != 2 || books[0].ID != "recent" {
		t.Fatalf("recent book was not selected first: %#v", books)
	}
	if len(books[0].Chapters) != 1 || books[0].Chapters[0].Status != "draft" {
		t.Fatalf("chapter draft did not persist: %#v", books[0].Chapters)
	}
}
