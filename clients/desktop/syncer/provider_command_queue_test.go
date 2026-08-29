package syncer

import (
	"os"
	"testing"
	"time"
)

func TestEnqueueProviderCommandAllowsRepeat(t *testing.T) {
	root := t.TempDir()
	if err := enqueueProviderCommand(root, "dehydrate", "a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := enqueueProviderCommand(root, "dehydrate", "a.txt"); err != nil {
		t.Fatal(err)
	}
	path := providerCommandsPath(root)
	b, err := readProviderCommandLines(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 2 {
		t.Fatalf("expected 2 commands at %s, got %d", path, len(b))
	}
}

func TestWaitForProviderCommandCompletesWhenQueueDrains(t *testing.T) {
	root := t.TempDir()
	if err := enqueueProviderCommand(root, "pin", "doc.txt"); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = os.Remove(providerCommandsPath(root))
	}()
	if err := waitForProviderCommand(root, "pin", "doc.txt", 2*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForProviderCommandTimesOut(t *testing.T) {
	root := t.TempDir()
	if err := enqueueProviderCommand(root, "dehydrate", "slow.txt"); err != nil {
		t.Fatal(err)
	}
	err := waitForProviderCommand(root, "dehydrate", "slow.txt", 400*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout")
	}
}
