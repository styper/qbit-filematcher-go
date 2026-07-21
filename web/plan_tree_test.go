package web

import (
	"testing"
)

func TestBuildPlanTree(t *testing.T) {
	tree := buildPlanTree([]string{
		"Show/S01/E01.mkv",
		"Show/S01/E02.mkv",
		"readme.txt",
		"",
		"Show/extras/notes.txt",
	})
	if len(tree) != 2 {
		t.Fatalf("root children=%d want 2", len(tree))
	}
	if tree[0].Name != "Show" {
		t.Fatalf("want Show first (dir), got %+v", tree[0])
	}
	if tree[1].Name != "readme.txt" || len(tree[1].Children) != 0 {
		t.Fatalf("file=%+v", tree[1])
	}
	show := tree[0]
	if len(show.Children) != 2 {
		t.Fatalf("Show children=%d want 2: %+v", len(show.Children), show.Children)
	}
	// directories first, then alpha: extras, S01
	if show.Children[0].Name != "extras" || show.Children[1].Name != "S01" {
		t.Fatalf("Show children=%v, %v", show.Children[0].Name, show.Children[1].Name)
	}
	s01 := show.Children[1]
	if len(s01.Children) != 2 || s01.Children[0].Name != "E01.mkv" || s01.Children[1].Name != "E02.mkv" {
		t.Fatalf("S01=%+v", s01)
	}
}
