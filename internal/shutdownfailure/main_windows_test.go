//go:build windows && cgo

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neko233-com/godesktop/testing/winprobe"
)

func TestShutdownEvidenceResetKeepsUnrelatedFiles(t *testing.T) {
	output := t.TempDir()
	for _, name := range []string{"current.json", "failed.json", "shutdown-native.exe", "unrelated.txt"} {
		if err := os.WriteFile(filepath.Join(output, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for range 3 {
		if err := resetEvidence(output); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 2 {
		t.Fatalf("reset damaged unrelated output: %v %v", entries, err)
	}
	for _, name := range []string{"shutdown-native.exe", "unrelated.txt"} {
		body, err := os.ReadFile(filepath.Join(output, name))
		if err != nil || string(body) != name {
			t.Fatalf("unrelated output %s changed: %q %v", name, body, err)
		}
	}
}

func TestShutdownOutputRejectsExternalDirectoryBeforeCreation(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "not-owned")
	if _, err := prepareOutput(outside); err == nil {
		t.Fatal("external output accepted")
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("external output created: %v", err)
	}
}

func TestShutdownPropertyRejectsUnownedIdentityBeforeNativeRead(t *testing.T) {
	for _, owner := range []struct {
		window winprobe.Window
		pid    uint32
	}{{0, uint32(os.Getpid())}, {1, 0}, {1, uint32(os.Getpid()) + 1}} {
		if accepted, err := actualShutdownPending(owner.window, owner.pid); accepted || err == nil {
			t.Fatal("unowned shutdown property identity admitted")
		}
	}
}
