package syncer

import "testing"

func TestLocalRelFromRemote(t *testing.T) {
	tests := []struct {
		in      string
		wantRel string
		legacy  bool
	}{
		{"PC/note.txt", "note.txt", true},
		{"Android/pic.jpg", "pic.jpg", true},
		{"docs/a.txt", "docs/a.txt", false},
		{"PC", "", true},
	}
	for _, tc := range tests {
		rel, legacy := localRelFromRemote(tc.in)
		if rel != tc.wantRel {
			t.Fatalf("%q rel=%q want %q", tc.in, rel, tc.wantRel)
		}
		if (legacy != "") != tc.legacy {
			t.Fatalf("%q legacy=%q want legacy=%v", tc.in, legacy, tc.legacy)
		}
	}
}

func TestLocalRelFromLocal(t *testing.T) {
	if got := localRelFromLocal("PC/readme.txt"); got != "readme.txt" {
		t.Fatalf("got %q", got)
	}
	if got := localRelFromLocal("notes/a.txt"); got != "notes/a.txt" {
		t.Fatalf("got %q", got)
	}
}
