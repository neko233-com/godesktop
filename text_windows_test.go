//go:build windows && cgo

package godesktop

import (
	"os/exec"
	"strings"
	"testing"
)

func TestNativeTextGridMetrics(t *testing.T) {
	output, err := exec.Command("go", "run", "-race", "./internal/textmetric").CombinedOutput()
	if err != nil || !strings.Contains(string(output), "native text metrics passed") || strings.Contains(string(output), "DATA RACE") {
		t.Fatalf("native metrics %v %s", err, output)
	}
}
