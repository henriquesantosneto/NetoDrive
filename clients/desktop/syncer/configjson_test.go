package syncer

import (
	"encoding/json"
	"testing"
)

func TestFixJSONWindowsPaths(t *testing.T) {
	raw := []byte(`{"local_folder":"C:\Users\henri\NetoDrive","server_url":"http://127.0.0.1:8080"}`)
	fixed := FixJSONWindowsPaths(raw)
	var doc struct {
		LocalFolder string `json:"local_folder"`
	}
	if err := json.Unmarshal(fixed, &doc); err != nil {
		t.Fatalf("unmarshal fixed: %v", err)
	}
	want := `C:\Users\henri\NetoDrive`
	if doc.LocalFolder != want {
		t.Fatalf("got %q want %q", doc.LocalFolder, want)
	}
}

func TestExtractLocalFolderFromBrokenJSON(t *testing.T) {
	raw := []byte(`{"local_folder":"D:\sync\data"}`)
	got, ok := ExtractLocalFolderFromBrokenJSON(raw)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != `D:\sync\data` {
		t.Fatalf("got %q", got)
	}
}
