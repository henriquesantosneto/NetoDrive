package cachelru_test

import (
	"testing"
	"time"

	"github.com/netodrive/server/internal/cachelru"
)

func TestLRUEvictsOldestWhenOverBudget(t *testing.T) {
	dir := t.TempDir()
	d := &cachelru.Disk{Root: dir, Budget: 30}
	if err := d.Put("a", make([]byte, 20)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := d.Put("b", make([]byte, 20)); err != nil {
		t.Fatal(err)
	}
	if d.Has("a") {
		t.Fatal("expected oldest entry a to be evicted")
	}
	if !d.Has("b") {
		t.Fatal("expected newest entry b to remain")
	}
	if d.Usage() > d.Budget {
		t.Fatalf("usage %d over budget %d", d.Usage(), d.Budget)
	}
}

func TestTouchProtectsFromEviction(t *testing.T) {
	dir := t.TempDir()
	d := &cachelru.Disk{Root: dir, Budget: 25}
	_ = d.Put("keep", make([]byte, 10))
	time.Sleep(20 * time.Millisecond)
	_ = d.Put("drop", make([]byte, 10))
	time.Sleep(20 * time.Millisecond)
	d.Touch("keep")
	time.Sleep(20 * time.Millisecond)
	_ = d.Put("new", make([]byte, 10))
	// keep(10)+drop(10)+new(10)=30 > 25 → drop is oldest by mtime after touch
	if d.Has("drop") {
		t.Fatal("expected untouched mid entry to be evicted")
	}
	if !d.Has("keep") {
		t.Fatal("touched entry should survive")
	}
	if !d.Has("new") {
		t.Fatal("newest entry should remain")
	}
}
